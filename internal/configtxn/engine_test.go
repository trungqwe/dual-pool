package configtxn

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/trungqwe/dual-pool/internal/lockfile"
	"github.com/trungqwe/dual-pool/internal/state"
)

type fixture struct {
	root, target, journal, backup, stateDir, lockDir string
	locks                                            *lockfile.Manager
	store                                            *state.Store
	engine                                           *Engine
	original                                         []byte
}

func newFixture(t *testing.T, options ...Option) *fixture {
	t.Helper()
	root := t.TempDir()
	f := &fixture{root: root, target: filepath.Join(root, "config.toml"), journal: filepath.Join(root, "journal"), backup: filepath.Join(root, "backup"), stateDir: filepath.Join(root, "state"), lockDir: filepath.Join(root, "locks"), original: []byte("# fixture\nmodel = \"old-model\" # old\nuser_setting = true\n\n[other]\nvalue = \"unchanged\"\n")}
	for _, d := range []string{f.journal, f.backup, f.stateDir, f.lockDir} {
		if err := os.Mkdir(d, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(f.target, f.original, 0600); err != nil {
		t.Fatal(err)
	}
	if !safeLocalAbsolute(f.target) {
		t.Fatal("TEMP target rejected by lexical local-path gate")
	}
	if _, err := safeExistingDir(filepath.Dir(f.target)); err != nil {
		t.Fatal("TEMP target parent rejected by directory gate")
	}
	if _, err := safeTarget(f.target); err != nil {
		t.Fatal("TEMP target rejected by artifact gate")
	}
	if _, err := fileIdentity(f.target); err != nil {
		t.Fatal("TEMP target rejected by identity gate")
	}
	var err error
	f.locks, err = lockfile.NewManager(f.lockDir)
	if err != nil {
		t.Fatal(err)
	}
	f.store, err = state.NewStore(f.stateDir, state.WithLockManager(f.locks))
	if err != nil {
		t.Fatal(err)
	}
	f.engine, err = NewEngine(f.journal, f.backup, f.locks, f.store, options...)
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func (f *fixture) plan() CodexPlan { return CodexPlan{Target: f.target} }

func TestApplyBackupOwnershipIdempotenceAndRollback(t *testing.T) {
	f := newFixture(t)
	if err := f.engine.Apply(f.plan()); err != nil {
		t.Fatal(err)
	}
	applied, err := os.ReadFile(f.target)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(applied, f.original) {
		t.Fatal("target unchanged")
	}
	ownership, err := f.store.LoadOwnership()
	if err != nil {
		t.Fatal(err)
	}
	if len(ownership.Records) != 3 {
		t.Fatalf("records=%d", len(ownership.Records))
	}
	for _, r := range ownership.Records {
		if r.RollbackStatus != state.RollbackApplied {
			t.Fatal("ownership not applied")
		}
		backup, err := os.ReadFile(r.BackupPath)
		if err != nil || !bytes.Equal(backup, f.original) || hash(backup) != r.BackupHash {
			t.Fatal("backup not exact")
		}
	}
	backupsBefore := countFiles(t, f.backup)
	hashBefore := hash(applied)
	if err = f.engine.Apply(f.plan()); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(f.target)
	if hash(again) != hashBefore || countFiles(t, f.backup) != backupsBefore {
		t.Fatal("idempotent apply rewrote artifacts")
	}
	// An unrelated user edit after apply must survive rollback.
	again = bytes.Replace(again, []byte("user_setting = true"), []byte("user_setting = false"), 1)
	if err = os.WriteFile(f.target, again, 0600); err != nil {
		t.Fatal(err)
	}
	if err = f.engine.Rollback(f.plan()); err != nil {
		t.Fatal(err)
	}
	rolled, _ := os.ReadFile(f.target)
	want := bytes.Replace(f.original, []byte("user_setting = true"), []byte("user_setting = false"), 1)
	if !bytes.Equal(rolled, want) {
		t.Fatalf("rollback did not preserve unrelated edit\n%s", rolled)
	}
	if err = f.engine.Rollback(f.plan()); err != nil {
		t.Fatal(err)
	}
}

func TestApplyCASAndRollbackOwnedConflict(t *testing.T) {
	f := newFixture(t)
	engine, err := NewEngine(f.journal, f.backup, f.locks, f.store, WithFaultInjector(func(p FaultPoint) error {
		if p == BeforeTargetCAS {
			b, _ := os.ReadFile(f.target)
			return os.WriteFile(f.target, append(b, []byte("external = true\n")...), 0600)
		}
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	err = engine.Apply(f.plan())
	if !errors.Is(err, ErrConfigConflict) {
		t.Fatalf("CAS=%v", err)
	}
	b, _ := os.ReadFile(f.target)
	if !bytes.Contains(b, []byte("external = true")) {
		t.Fatal("external bytes overwritten")
	}
	f = newFixture(t)
	if err = f.engine.Apply(f.plan()); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(f.target)
	b = bytes.Replace(b, []byte("model_provider = \"dualpool_codex\""), []byte("model_provider = \"user_choice\""), 1)
	if err = os.WriteFile(f.target, b, 0600); err != nil {
		t.Fatal(err)
	}
	err = f.engine.Rollback(f.plan())
	if !errors.Is(err, ErrRollbackConflict) {
		t.Fatalf("rollback conflict=%v", err)
	}
	after, _ := os.ReadFile(f.target)
	if !bytes.Equal(after, b) {
		t.Fatal("rollback overwrote owned user edit")
	}
}

func TestApplyRecoveryPreAndPost(t *testing.T) {
	for _, tc := range []struct {
		name        string
		point       FaultPoint
		wantApplied bool
	}{{"pre", AfterPendingOwnership, false}, {"post", AfterTargetReplace, true}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, WithFaultInjector(func(p FaultPoint) error {
				if p == tc.point {
					return ErrInjectedCrash
				}
				return nil
			}))
			if err := f.engine.Apply(f.plan()); !errors.Is(err, ErrInjectedCrash) {
				t.Fatalf("injection=%v", err)
			}
			fresh, err := NewEngine(f.journal, f.backup, f.locks, f.store)
			if err != nil {
				t.Fatal(err)
			}
			if err = fresh.Recover(f.plan()); err != nil {
				t.Fatal(err)
			}
			if err = fresh.Recover(f.plan()); err != nil {
				t.Fatal(err)
			}
			current, _ := os.ReadFile(f.target)
			if bytes.Equal(current, f.original) == tc.wantApplied {
				t.Fatalf("wrong recovered state")
			}
			o, err := f.store.LoadOwnership()
			if tc.wantApplied {
				if err != nil || len(o.Records) != 3 || o.Records[0].RollbackStatus != state.RollbackApplied {
					t.Fatal("post ownership not finalized")
				}
			} else if !errors.Is(err, state.ErrNotInitialized) && len(o.Records) != 0 {
				t.Fatal("pre ownership retained")
			}
		})
	}
}

func TestRollbackRecoveryPost(t *testing.T) {
	f := newFixture(t)
	if err := f.engine.Apply(f.plan()); err != nil {
		t.Fatal(err)
	}
	crash, err := NewEngine(f.journal, f.backup, f.locks, f.store, WithFaultInjector(func(p FaultPoint) error {
		if p == AfterTargetReplace {
			return ErrInjectedCrash
		}
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	if err = crash.Rollback(f.plan()); !errors.Is(err, ErrInjectedCrash) {
		t.Fatalf("rollback injection=%v", err)
	}
	fresh, _ := NewEngine(f.journal, f.backup, f.locks, f.store)
	if err = fresh.Recover(f.plan()); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(f.target)
	if !bytes.Equal(b, f.original) {
		t.Fatal("rollback recovery did not finalize restored document")
	}
	o, _ := f.store.LoadOwnership()
	for _, r := range o.Records {
		if r.RollbackStatus != state.RollbackRolledBack {
			t.Fatal("rollback status not finalized")
		}
	}
}

func TestAlreadyOwnedDifferentPlanRejected(t *testing.T) {
	f := newFixture(t)
	if err := f.engine.Apply(f.plan()); err != nil {
		t.Fatal(err)
	}
	different := f.plan()
	different.Catalog = AuthorizedCatalogFallback("fixture-catalog.json")
	if err := f.engine.Apply(different); !errors.Is(err, ErrReconfigureUnsupported) {
		t.Fatalf("different plan=%v", err)
	}
}

func countFiles(t *testing.T, path string) int {
	t.Helper()
	x, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	return len(x)
}
