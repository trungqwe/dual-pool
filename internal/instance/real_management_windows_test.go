package instance

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/trungqwe/dual-pool/internal/cliproxyconfig"
	"github.com/trungqwe/dual-pool/internal/cliproxymgmt"
	"github.com/trungqwe/dual-pool/internal/dataroot"
	"github.com/trungqwe/dual-pool/internal/productinit"
	"github.com/trungqwe/dual-pool/internal/secretstore"
	"github.com/trungqwe/dual-pool/internal/upstreamlock"
	"github.com/trungqwe/dual-pool/internal/winacl"
)

func TestRealEmptyManagementInventory(t *testing.T) {
	if os.Getenv("DUALPOOL_RUN_REAL_MGMT_L3") != "1" {
		t.Skip("explicit management L3 gate is closed")
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "upstream.lock"))
	if err != nil {
		t.Fatal(err)
	}
	lock, err := upstreamlock.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	state, err := productinit.InspectCurrent()
	if err != nil || !state.Ready || state.KeysPresent != 4 {
		t.Fatal("product initialization not ready")
	}
	layout, err := dataroot.ResolveCurrent()
	if err != nil {
		t.Fatal(err)
	}
	acl, err := winacl.New()
	if err != nil {
		t.Fatal(err)
	}
	store := secretstore.New()
	configs, err := cliproxyconfig.New(layout, acl, store, lock)
	if err != nil {
		t.Fatal(err)
	}
	if ready, e := configs.InspectPair(); e != nil || !ready.Ready {
		t.Fatal("config pair not ready")
	}
	m, err := New(layout, acl, lock)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.validateInstall(context.Background(), m.executableDir()); err != nil {
		t.Fatal("installed binary invalid")
	}
	before := map[cliproxyconfig.ID][]byte{}
	for _, id := range []cliproxyconfig.ID{cliproxyconfig.Codex, cliproxyconfig.Google} {
		if _, e := os.Lstat(filepath.Join(layout.Instances, string(id), "process.json")); !os.IsNotExist(e) {
			t.Fatal("unexpected process record")
		}
		for _, n := range []string{"auth", "logs"} {
			entries, e := os.ReadDir(filepath.Join(layout.Instances, string(id), n))
			if e != nil || len(entries) != 0 {
				t.Fatal("unexpected auth/log artifact")
			}
		}
		b, e := os.ReadFile(filepath.Join(layout.Instances, string(id), "config.yaml"))
		if e != nil {
			t.Fatal(e)
		}
		before[id] = b
	}
	for _, p := range []int{cliproxyconfig.CodexPort, cliproxyconfig.GooglePort} {
		used, e := m.portOccupied(p)
		if e != nil || used {
			t.Fatal("port unavailable")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	owned := map[cliproxyconfig.ID]bool{}
	t.Cleanup(func() {
		for _, id := range []cliproxyconfig.ID{cliproxyconfig.Codex, cliproxyconfig.Google} {
			if owned[id] {
				if e := m.Stop(id); e != nil {
					t.Errorf("P2-REAL-CLEANUP-001: %v", e)
				}
			}
			if _, e := os.Lstat(filepath.Join(layout.Instances, string(id), "process.json")); !os.IsNotExist(e) {
				t.Errorf("P2-REAL-CLEANUP-001: record remains")
			}
		}
		for _, p := range []int{cliproxyconfig.CodexPort, cliproxyconfig.GooglePort} {
			if used, e := m.portOccupied(p); e != nil || used {
				t.Errorf("P2-REAL-CLEANUP-001: listener remains")
			}
		}
	})
	for _, id := range []cliproxyconfig.ID{cliproxyconfig.Codex, cliproxyconfig.Google} {
		if _, err = m.Start(ctx, id); err != nil {
			t.Fatal("start failed")
		}
		owned[id] = true
		c, e := cliproxymgmt.New(id, store, lock)
		if e != nil || c.Debug(ctx) != nil || c.EmptyInventory(ctx) != nil || c.EmptyInventory(ctx) != nil {
			t.Fatal("L2/L3 management contract failed")
		}
	}
	for _, id := range []cliproxyconfig.ID{cliproxyconfig.Codex, cliproxyconfig.Google} {
		if err = m.Stop(id); err != nil {
			t.Fatal(err)
		}
		owned[id] = false
		after, e := os.ReadFile(filepath.Join(layout.Instances, string(id), "config.yaml"))
		if e != nil || !bytes.Equal(before[id], after) {
			t.Fatal("config changed")
		}
		secretstore.Zero(after)
	}
	for _, b := range before {
		secretstore.Zero(b)
	}
}
