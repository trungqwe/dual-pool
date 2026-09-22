package instance

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/trungqwe/dual-pool/internal/cliproxyconfig"
	"github.com/trungqwe/dual-pool/internal/dataroot"
	"github.com/trungqwe/dual-pool/internal/installedslot"
	"github.com/trungqwe/dual-pool/internal/state"
	"github.com/trungqwe/dual-pool/internal/upstreamlock"
	"github.com/trungqwe/dual-pool/internal/upstreamstage"
	"github.com/trungqwe/dual-pool/internal/winacl"
)

// The TEMP executable is a copy of this test binary, never CLIProxyAPI.
// Default WindowsVerifier runs it with -h. No provider or listener is started.
func TestMain(m *testing.M) {
	if len(os.Args) == 2 && os.Args[1] == "-h" {
		fmt.Println("CLIProxyAPI Version: 7.3.7, Commit: b773607e3e7756dc6020a291825e4eb08899595a, BuiltAt: synthetic-test")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func legacyInstallFixture(t *testing.T) (*Manager, upstreamstage.Result) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "product-fixture")
	layout := dataroot.Layout{Root: root, Bin: filepath.Join(root, "bin"), Instances: filepath.Join(root, "instances"), Locks: filepath.Join(root, "locks"), State: filepath.Join(root, "state")}
	acl, err := winacl.New()
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{root, layout.Bin, layout.Instances, layout.Locks, layout.State, filepath.Join(layout.Bin, "cliproxyapi"), filepath.Join(layout.Bin, "cliproxyapi", "7.3.7")} {
		if err := acl.Create(dir); err != nil {
			t.Fatal(err)
		}
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "upstream.lock"))
	if err != nil {
		t.Fatal(err)
	}
	pin, err := upstreamlock.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	raw = bytes.ReplaceAll(raw, []byte(pin.Platforms.WindowsAMD64.ExecutableSHA256), []byte(digest(self)))
	pin, err = upstreamlock.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	active := state.State{SchemaVersion: 1, InstallID: "install_fixture_01", ActiveUpstreamVersion: pin.Version, Instances: state.Instances{Codex: state.Instance{Port: cliproxyconfig.CodexPort, Status: state.InstanceStopped}, Google: state.Instance{Port: cliproxyconfig.GooglePort, Status: state.InstanceStopped}}, Antigravity: state.Antigravity{Mode: state.ModeDisabled, BridgePort: 51074}}
	m, err := New(layout, acl, pin, WithStateReader(activeStateFixture{value: active}))
	if err != nil {
		t.Fatal(err)
	}
	// The fixture's executable is this test binary, whose identity is already
	// established by the synthetic TestMain -h contract. Avoid spawning a
	// Windows verifier subprocess for every stress iteration.
	m.identityVerifier = func(_ context.Context, _ string, _ upstreamstage.ExpectedIdentity) (upstreamstage.Identity, error) {
		return upstreamstage.Identity{VersionMatch: true, CommitMatch: true}, nil
	}
	if err = copyProtected(acl, self, m.executablePath()); err != nil {
		t.Fatal(err)
	}
	if err = writeProtectedJSON(acl, filepath.Join(m.executableDir(), manifestName), m.manifest()); err != nil {
		t.Fatal(err)
	}
	if err = m.validateInstall(context.Background(), m.executableDir()); err != nil {
		t.Fatal(err)
	}
	return m, upstreamstage.Result{Executable: self, Manifest: upstreamstage.Manifest{ExecutableSHA256: pin.Platforms.WindowsAMD64.ExecutableSHA256}}
}

func TestExistingInstallBootstrapsRegistryBeforeReuseSuccess(t *testing.T) {
	m, stage := legacyInstallFixture(t)
	registryPath := filepath.Join(m.layout.Bin, "cliproxyapi", "installed-slots.json")
	if _, err := os.Stat(registryPath); !os.IsNotExist(err) {
		t.Fatalf("registry unexpectedly exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(registryPath), markerName)); !os.IsNotExist(err) {
		t.Fatalf("marker unexpectedly exists: %v", err)
	}
	beforeManifest, err := os.ReadFile(filepath.Join(m.executableDir(), manifestName))
	if err != nil {
		t.Fatal(err)
	}
	beforeHash := digest(m.executablePath())
	path, reused, err := m.Install(context.Background(), stage)
	if err != nil || !reused || path != m.executablePath() {
		t.Fatalf("reuse: %s %v %v", path, reused, err)
	}
	slot, err := m.activeSlot(context.Background())
	if err != nil || slot.Version != m.lock.Version || slot.ExecutablePath != path {
		t.Fatalf("reused slot unavailable: %+v %v", slot, err)
	}
	afterManifest, err := os.ReadFile(filepath.Join(m.executableDir(), manifestName))
	if err != nil || !bytes.Equal(beforeManifest, afterManifest) || digest(path) != beforeHash {
		t.Fatal("reuse changed installed artifact")
	}
	first, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, reused, err = m.Install(context.Background(), stage); err != nil || !reused {
		t.Fatalf("repeat reuse: %v %v", reused, err)
	}
	second, err := os.ReadFile(registryPath)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatal("idempotent reuse changed registry")
	}
}

func TestExistingInstallCannotReportSuccessForCorruptRegistry(t *testing.T) {
	m, stage := legacyInstallFixture(t)
	path := filepath.Join(m.layout.Bin, "cliproxyapi", "installed-slots.json")
	if err := writeProtectedJSON(m.acl, path, map[string]string{"unexpected": "corrupt"}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, reused, err := m.Install(context.Background(), stage)
	if !errors.Is(err, installedslot.ErrRegistryCorrupt) || reused {
		t.Fatalf("false reuse success: %v %v", reused, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("corrupt registry was overwritten")
	}
}

func TestRegistryRemovedAfterConstructionFailsBeforeProcessHandle(t *testing.T) {
	m, _ := legacyInstallFixture(t)
	WithSlotRegistry(nil)(m)
	opened := false
	m.opener = func(uint32) (terminationHandle, error) { opened = true; return nil, ErrIdentityMismatch }
	record := ProcessRecord{UpstreamVersion: m.lock.Version, ExecutableSHA256: m.lock.Platforms.WindowsAMD64.ExecutableSHA256, ManifestSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", PID: 42}
	if _, err := m.recordSlot(record); !errors.Is(err, ErrIdentityMismatch) {
		t.Fatalf("nil registry bypass: %v", err)
	}
	if err := m.stopRecord(record); !errors.Is(err, ErrIdentityMismatch) || opened {
		t.Fatalf("nil registry opened process: %v %v", opened, err)
	}
}
