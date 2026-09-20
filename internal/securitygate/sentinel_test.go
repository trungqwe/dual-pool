package securitygate

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/trungqwe/dual-pool/internal/app"
	"github.com/trungqwe/dual-pool/internal/apperr"
	"github.com/trungqwe/dual-pool/internal/buildinfo"
	"github.com/trungqwe/dual-pool/internal/configtxn"
	"github.com/trungqwe/dual-pool/internal/lockfile"
	"github.com/trungqwe/dual-pool/internal/safelog"
	"github.com/trungqwe/dual-pool/internal/secretstore"
	"github.com/trungqwe/dual-pool/internal/state"
)

func TestPhase1SentinelNonDisclosure(t *testing.T) {
	id := randomSentinel(t)
	sentinels := []string{
		"sk-" + id,
		"management-secret-" + id,
		"Bearer " + id,
		"raw-session-" + id,
		id + "@example.invalid",
		"prompt source content " + id,
		`C:\sensitive\` + id,
	}
	var forbidden bytes.Buffer

	for _, sentinel := range sentinels {
		err := apperr.MustNew(apperr.CodeConfigConflict, errors.New(sentinel))
		forbidden.WriteString(err.Error())
		payload, marshalErr := json.Marshal(err.Envelope())
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		forbidden.Write(payload)

		var stdout, stderr bytes.Buffer
		app.New(buildinfo.Current()).Run([]string{sentinel}, &stdout, &stderr)
		forbidden.Write(stdout.Bytes())
		forbidden.Write(stderr.Bytes())

		var logs bytes.Buffer
		logger := safelog.New(&logs, func() time.Time { return time.Unix(0, 0).UTC() })
		field, fieldErr := safelog.StringField("operation_id", sentinel)
		if fieldErr == nil {
			_ = logger.Log(safelog.Event{Level: safelog.LevelInfo, Component: "test", Name: "test.sentinel", Fields: []safelog.Field{field}})
		}
		forbidden.Write(logs.Bytes())

		if err := secretstore.New().Put(secretstore.Purpose("invalid"), []byte(sentinel)); !errors.Is(err, secretstore.ErrInvalidPurpose) {
			t.Fatalf("secret error=%v", err)
		} else {
			forbidden.WriteString(err.Error())
		}
		forbidden.WriteString(configtxn.ErrConfigConflict.Error())
		forbidden.WriteString(configtxn.ErrRollbackConflict.Error())
		forbidden.WriteString(lockfile.ErrUnsafeLockArtifact.Error())
	}

	root := t.TempDir()
	for _, name := range []string{"state", "locks", "journal", "backup"} {
		if err := os.Mkdir(filepath.Join(root, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	locks, err := lockfile.NewManager(filepath.Join(root, "locks"))
	if err != nil {
		t.Fatal(err)
	}
	store, err := state.NewStore(filepath.Join(root, "state"), state.WithLockManager(locks))
	if err != nil {
		t.Fatal(err)
	}
	valid := state.State{SchemaVersion: state.StateSchemaVersion, InstallID: "fixture_install", Instances: state.Instances{Codex: state.Instance{Port: 8317, Status: state.InstanceStopped}, Google: state.Instance{Port: 8318, Status: state.InstanceStopped}}, Antigravity: state.Antigravity{Mode: state.ModeDisabled, BridgePort: 51074}}
	if err = store.SaveState(valid); err != nil {
		t.Fatal(err)
	}
	invalid := valid
	invalid.Accounts = []state.Account{{OpaqueID: "fixture_account", Pool: state.PoolCodex, Nickname: sentinels[4], Eligibility: state.EligibilityUnknown}}
	if err = store.SaveState(invalid); !errors.Is(err, state.ErrInvalidDocument) {
		t.Fatalf("sensitive state accepted: %v", err)
	}
	sensitive := sentinels[0]
	hash := strings.Repeat("a", 64)
	ownership := state.Ownership{SchemaVersion: state.OwnershipSchemaVersion, Records: []state.OwnershipRecord{{
		TargetPath: filepath.Join(root, "fixture.toml"), Format: state.FormatTOML, Encoding: state.EncodingUTF8,
		BOM: state.BOMAbsent, Newline: state.NewlineLF, FileIdentity: "fixture_identity", PreWriteHash: hash,
		KeyPath: "model", OriginalExisted: true, OriginalValue: state.TypedValue{Kind: state.ValueString, String: &sensitive},
		AppliedValue: state.TypedValue{Kind: state.ValueString, String: &sensitive}, BackupPath: filepath.Join(root, "fixture.bak"),
		BackupHash: hash, AppliedAt: "2026-09-20T00:00:00Z", ToolVersion: "test", PostWriteHash: hash, RollbackStatus: state.RollbackApplied,
	}}}
	if err = store.SaveOwnership(ownership); !errors.Is(err, state.ErrInvalidDocument) {
		t.Fatalf("sensitive ownership accepted: %v", err)
	}

	engine, err := configtxn.NewEngine(filepath.Join(root, "journal"), filepath.Join(root, "backup"), locks, store)
	if err != nil {
		t.Fatal(err)
	}
	if err = engine.Apply(configtxn.CodexPlan{Target: sentinels[6]}); !errors.Is(err, configtxn.ErrUnsafeConfigArtifact) {
		t.Fatalf("unsafe config path=%v", err)
	} else {
		forbidden.WriteString(err.Error())
	}

	for _, path := range []string{filepath.Join(root, "state", "state.json"), filepath.Join(root, "state", "ownership.json")} {
		data, readErr := os.ReadFile(path)
		if readErr == nil {
			forbidden.Write(data)
		}
	}
	for _, sentinel := range sentinels {
		if bytes.Contains(forbidden.Bytes(), []byte(sentinel)) {
			t.Fatal("runtime sentinel disclosed on a forbidden Phase 1 surface")
		}
	}

	repo := filepath.Clean(filepath.Join("..", ".."))
	for _, subtree := range []string{"evidence", "docs"} {
		err = filepath.WalkDir(filepath.Join(repo, subtree), func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			for _, sentinel := range sentinels {
				if strings.Contains(string(data), sentinel) {
					t.Fatalf("runtime sentinel entered committed artifact surface")
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func randomSentinel(t *testing.T) string {
	t.Helper()
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}
