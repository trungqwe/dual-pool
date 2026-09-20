package instance

import (
	"bytes"
	"context"
	"crypto/subtle"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/trungqwe/dual-pool/internal/cliproxyconfig"
	"github.com/trungqwe/dual-pool/internal/dataroot"
	"github.com/trungqwe/dual-pool/internal/lockfile"
	"github.com/trungqwe/dual-pool/internal/productinit"
	"github.com/trungqwe/dual-pool/internal/secretstore"
	"github.com/trungqwe/dual-pool/internal/upstreamlock"
	"github.com/trungqwe/dual-pool/internal/upstreamstage"
	"github.com/trungqwe/dual-pool/internal/winacl"
	"golang.org/x/sys/windows"
)

// TestRealLifecycle is the only opt-in persistent lifecycle gate. It sends no
// provider, OAuth, model-generation, IDE, or credential-inventory traffic.
func TestRealLifecycle(t *testing.T) {
	if os.Getenv("DUALPOOL_RUN_REAL_LIFECYCLE") != "1" {
		t.Skip("explicit real lifecycle gate is closed")
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "upstream.lock"))
	if err != nil {
		t.Fatal("upstream lock unavailable")
	}
	lock, err := upstreamlock.Decode(raw)
	if err != nil {
		t.Fatal("upstream lock invalid")
	}
	state, err := productinit.InspectCurrent()
	if err != nil || !state.Ready || state.KeysPresent != 4 {
		t.Fatal("product initialization not ready")
	}
	layout, err := dataroot.ResolveCurrent()
	if err != nil {
		t.Fatal("product layout unavailable")
	}
	acl, err := winacl.New()
	if err != nil {
		t.Fatal("ACL unavailable")
	}
	store := secretstore.New()
	configs, err := cliproxyconfig.New(layout, acl, store, lock)
	if err != nil {
		t.Fatal(err)
	}
	ready, err := configs.InspectPair()
	if err != nil || !ready.Ready {
		t.Fatal("instance config pair not ready")
	}
	m, err := New(layout, acl, lock)
	if err != nil {
		t.Fatal(err)
	}
	before := map[cliproxyconfig.ID][]byte{}
	identities := map[cliproxyconfig.ID]realFileIdentity{}
	beforeKeys := map[secretstore.Purpose][]byte{}
	for _, p := range []secretstore.Purpose{secretstore.CodexClientKey, secretstore.CodexManagementKey, secretstore.GoogleClientKey, secretstore.GoogleManagementKey} {
		v, e := store.Get(p)
		if e != nil || len(v) != 32 {
			t.Fatal("product key invalid")
		}
		beforeKeys[p] = v
	}
	defer func() {
		for _, v := range before {
			secretstore.Zero(v)
		}
		for _, v := range beforeKeys {
			secretstore.Zero(v)
		}
	}()
	for _, id := range []cliproxyconfig.ID{cliproxyconfig.Codex, cliproxyconfig.Google} {
		if _, e := os.Lstat(filepath.Join(layout.Instances, string(id), "process.json")); !os.IsNotExist(e) {
			t.Fatal("unexpected process record")
		}
		for _, name := range []string{"auth", "logs"} {
			entries, e := os.ReadDir(filepath.Join(layout.Instances, string(id), name))
			if e != nil || len(entries) != 0 {
				t.Fatal("unexpected instance artifacts")
			}
		}
		b, e := os.ReadFile(filepath.Join(layout.Instances, string(id), "config.yaml"))
		if e != nil {
			t.Fatal(e)
		}
		before[id] = b
		identities[id], e = realIdentity(filepath.Join(layout.Instances, string(id), "config.yaml"))
		if e != nil {
			t.Fatal(e)
		}
	}
	for _, port := range []int{cliproxyconfig.CodexPort, cliproxyconfig.GooglePort} {
		occupied, e := m.portOccupied(port)
		if e != nil || occupied {
			t.Fatal("port unavailable")
		}
	}
	stageLocks, err := lockfile.NewManager(layout.Locks)
	if err != nil {
		t.Fatal(err)
	}
	stager, err := upstreamstage.New(t.TempDir(), stageLocks)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	stage, err := stager.Stage(ctx, lock)
	if err != nil {
		t.Fatal("pinned stage failed")
	}
	if _, _, err = m.Install(ctx, stage); err != nil {
		t.Fatal("pinned install failed")
	}
	owned := map[cliproxyconfig.ID]bool{}
	t.Cleanup(func() {
		for _, id := range []cliproxyconfig.ID{cliproxyconfig.Codex, cliproxyconfig.Google} {
			if owned[id] {
				if e := m.Stop(id); e != nil {
					t.Errorf("P2-REAL-CLEANUP-001: safe stop unresolved: %v", e)
				}
			}
			if _, e := os.Lstat(filepath.Join(layout.Instances, string(id), "process.json")); !os.IsNotExist(e) {
				t.Errorf("P2-REAL-CLEANUP-001: process record unresolved")
			}
		}
		for _, port := range []int{cliproxyconfig.CodexPort, cliproxyconfig.GooglePort} {
			if occupied, e := m.portOccupied(port); e != nil || occupied {
				t.Errorf("P2-REAL-CLEANUP-001: listener unresolved")
			}
		}
	})
	codex, err := m.Start(ctx, cliproxyconfig.Codex)
	if err != nil {
		t.Fatal("codex start failed")
	}
	owned[cliproxyconfig.Codex] = true
	if again, e := m.Start(ctx, cliproxyconfig.Codex); e != nil || again.Record.PID != codex.Record.PID {
		t.Fatal("codex idempotence failed")
	}
	google, err := m.Start(ctx, cliproxyconfig.Google)
	if err != nil {
		t.Fatal("google start failed")
	}
	owned[cliproxyconfig.Google] = true
	if google.Record.PID == codex.Record.PID {
		t.Fatal("shared PID")
	}
	if again, e := m.Start(ctx, cliproxyconfig.Google); e != nil || again.Record.PID != google.Record.PID {
		t.Fatal("google idempotence failed")
	}
	for _, id := range []cliproxyconfig.ID{cliproxyconfig.Codex, cliproxyconfig.Google} {
		if _, e := m.Status(id); e != nil {
			t.Fatal("status failed")
		}
	}
	if _, err = m.Restart(ctx, cliproxyconfig.Codex); err != nil {
		t.Fatal("codex restart failed")
	}
	if _, err = m.Restart(ctx, cliproxyconfig.Google); err != nil {
		t.Fatal("google restart failed")
	}
	for _, id := range []cliproxyconfig.ID{cliproxyconfig.Codex, cliproxyconfig.Google} {
		if err = m.Stop(id); err != nil {
			t.Fatal("stop failed")
		}
		owned[id] = false
		if err = m.Stop(id); err != nil {
			t.Fatal("repeat stop failed")
		}
		after, e := os.ReadFile(filepath.Join(layout.Instances, string(id), "config.yaml"))
		if e != nil || !bytes.Equal(before[id], after) {
			t.Fatal("config changed")
		}
		secretstore.Zero(after)
		if afterID, e := realIdentity(filepath.Join(layout.Instances, string(id), "config.yaml")); e != nil || afterID != identities[id] {
			t.Fatal("config identity changed")
		}
		if _, e := os.Lstat(filepath.Join(layout.Instances, string(id), "process.json")); !os.IsNotExist(e) {
			t.Fatal("process record remains")
		}
		for _, name := range []string{"auth", "logs"} {
			entries, e := os.ReadDir(filepath.Join(layout.Instances, string(id), name))
			if e != nil || len(entries) != 0 {
				t.Fatal("auth/log artifacts remain")
			}
		}
	}
	for _, port := range []int{cliproxyconfig.CodexPort, cliproxyconfig.GooglePort} {
		occupied, e := m.portOccupied(port)
		if e != nil || occupied {
			t.Fatal("listener remains")
		}
	}
	for p, before := range beforeKeys {
		after, e := store.Get(p)
		if e != nil || subtle.ConstantTimeCompare(before, after) != 1 {
			secretstore.Zero(after)
			t.Fatal("product key changed")
		}
		secretstore.Zero(after)
	}
	if err = m.validateInstall(ctx, m.executableDir()); err != nil {
		t.Fatal("binary final validation failed")
	}
}

type realFileIdentity struct{ volume, high, low uint32 }

func realIdentity(path string) (realFileIdentity, error) {
	f, e := os.Open(path)
	if e != nil {
		return realFileIdentity{}, e
	}
	defer f.Close()
	var info windows.ByHandleFileInformation
	if e = windows.GetFileInformationByHandle(windows.Handle(f.Fd()), &info); e != nil {
		return realFileIdentity{}, e
	}
	return realFileIdentity{info.VolumeSerialNumber, info.FileIndexHigh, info.FileIndexLow}, nil
}
