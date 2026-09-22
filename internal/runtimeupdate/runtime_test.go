package runtimeupdate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/trungqwe/dual-pool/internal/cliproxyconfig"
	"github.com/trungqwe/dual-pool/internal/dataroot"
	"github.com/trungqwe/dual-pool/internal/installedslot"
	"github.com/trungqwe/dual-pool/internal/lockfile"
	"github.com/trungqwe/dual-pool/internal/state"
	"github.com/trungqwe/dual-pool/internal/upstreamlock"
	"github.com/trungqwe/dual-pool/internal/winacl"
)

func TestMain(m *testing.M) {
	if len(os.Args) == 2 && os.Args[1] == "-h" {
		fmt.Println("CLIProxyAPI Version: 7.3.7, Commit: b773607e3e7756dc6020a291825e4eb08899595a, BuiltAt: synthetic-test")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type recordingSmoke struct{ calls []string }

func (s *recordingSmoke) Disposable(_ context.Context, version string) error {
	s.calls = append(s.calls, "disposable:"+version)
	return nil
}
func (s *recordingSmoke) Production(_ context.Context, version string) error {
	s.calls = append(s.calls, "production:"+version)
	return nil
}

type nilSmoke struct{}

func (*nilSmoke) Disposable(context.Context, string) error { return nil }
func (*nilSmoke) Production(context.Context, string) error { return nil }

type nilACL struct{}

func (*nilACL) Create(string) error                 { return nil }
func (*nilACL) Inspect(string) error                { return nil }
func (*nilACL) CreateFile(string) (*os.File, error) { return nil, nil }
func (*nilACL) InspectFile(string) error            { return nil }

func TestNewRejectsNilAndTypedNilSmoke(t *testing.T) {
	if _, err := New(Config{}); !errors.Is(err, ErrCompositionInvalid) {
		t.Fatalf("nil smoke: %v", err)
	}
	var smoke *nilSmoke
	if !isNil(smoke) {
		t.Fatal("typed nil smoke not detected")
	}
	if _, err := New(Config{Smoke: smoke}); !errors.Is(err, ErrCompositionInvalid) {
		t.Fatalf("typed nil smoke: %v", err)
	}
}

func TestNewRejectsTypedNilACL(t *testing.T) {
	var acl *nilACL
	if !isNil(acl) {
		t.Fatal("typed nil ACL not detected")
	}
	if _, err := New(Config{ACL: acl, Smoke: &recordingSmoke{}}); !errors.Is(err, ErrCompositionInvalid) {
		t.Fatalf("typed nil ACL: %v", err)
	}
}

func productionFixture(t *testing.T) (*Runtime, Config) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "product")
	layout := dataroot.Layout{Root: root, Bin: filepath.Join(root, "bin"), Instances: filepath.Join(root, "instances"), Config: filepath.Join(root, "config"), State: filepath.Join(root, "state"), Backups: filepath.Join(root, "backups"), Evidence: filepath.Join(root, "evidence"), Locks: filepath.Join(root, "locks")}
	acl, err := winacl.New()
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{layout.Root, layout.Bin, layout.Instances, layout.Config, layout.State, layout.Backups, layout.Evidence, layout.Locks} {
		if err = acl.Create(dir); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "upstream.lock"))
	if err != nil {
		t.Fatal(err)
	}
	pin, err := upstreamlock.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	raw = bytes.ReplaceAll(raw, []byte(pin.Platforms.WindowsAMD64.ExecutableSHA256), []byte(hex.EncodeToString(sum[:])))
	pin, err = upstreamlock.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	smoke := &recordingSmoke{}
	seedLocks, err := lockfile.NewManager(layout.Locks)
	if err != nil {
		t.Fatal(err)
	}
	seedStore, err := state.NewStore(layout.State, state.WithLockManager(seedLocks))
	if err != nil {
		t.Fatal(err)
	}
	if err = seedStore.SaveState(testState(pin.Version)); err != nil {
		t.Fatal(err)
	}
	config := Config{Layout: layout, Lock: pin, ACL: acl, Smoke: smoke}
	runtime, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	slotRoot, slotDir := filepath.Join(layout.Bin, "cliproxyapi"), filepath.Join(layout.Bin, "cliproxyapi", pin.Version)
	for _, dir := range []string{slotRoot, slotDir} {
		if err = acl.Create(dir); err != nil {
			t.Fatal(err)
		}
	}
	writeProtected(t, acl, filepath.Join(slotDir, "cliproxyapi.exe"), body)
	manifest := installedslot.Manifest{SchemaVersion: 1, Product: pin.Product, Version: pin.Version, Tag: pin.Tag, Commit: pin.Commit, Platform: "windows_amd64", ExecutableSHA256: pin.Platforms.WindowsAMD64.ExecutableSHA256, UpstreamLockSHA256: pin.Digest(), ConfigAdapterVersion: pin.ConfigAdapterVersion, ExecutableBasename: "cliproxyapi.exe"}
	manifestBytes, _ := json.Marshal(manifest)
	writeProtected(t, acl, filepath.Join(slotDir, "install-manifest.json"), append(manifestBytes, '\n'))
	if err = runtime.registry.Register(context.Background(), pin.Version); err != nil {
		t.Fatal(err)
	}
	return runtime, config
}

func writeProtected(t *testing.T, acl *winacl.Manager, path string, data []byte) {
	t.Helper()
	file, err := acl.CreateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write(data); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err = file.Sync(); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
}
func testState(version string) state.State {
	return state.State{SchemaVersion: 1, InstallID: "install_fixture_01", ActiveUpstreamVersion: version, Instances: state.Instances{Codex: state.Instance{Port: cliproxyconfig.CodexPort, Status: state.InstanceStopped}, Google: state.Instance{Port: cliproxyconfig.GooglePort, Status: state.InstanceStopped}}, Antigravity: state.Antigravity{Mode: state.ModeDisabled, BridgePort: 51074}, Accounts: []state.Account{}}
}
func pointerField(object any, name string) uintptr {
	field := reflect.ValueOf(object).Elem().FieldByName(name)
	if field.Kind() == reflect.Interface {
		field = field.Elem()
	}
	return field.Pointer()
}

func TestRuntimeCompositionSharesExactObjects(t *testing.T) {
	runtime, _ := productionFixture(t)
	if pointerField(runtime.Updater, "state") != reflect.ValueOf(runtime.state).Pointer() || pointerField(runtime.Manager, "state") != reflect.ValueOf(runtime.state).Pointer() {
		t.Fatal("state Store is not shared")
	}
	if pointerField(runtime.Updater, "verifier") != reflect.ValueOf(runtime.registry).Pointer() || pointerField(runtime.Manager, "registry") != reflect.ValueOf(runtime.registry).Pointer() {
		t.Fatal("Registry is not shared")
	}
	updaterLocks := pointerField(runtime.Updater, "locks")
	if updaterLocks == 0 || pointerField(runtime.Manager, "locks") != updaterLocks || pointerField(runtime.state, "locks") != updaterLocks || pointerField(runtime.registry, "locks") != updaterLocks || reflect.ValueOf(runtime.locks).Pointer() != updaterLocks {
		t.Fatal("lock Manager is not shared")
	}
}

func TestRuntimePrivateLifecycleUsesExactManager(t *testing.T) {
	runtime, _ := productionFixture(t)
	lifecycle := reflect.ValueOf(runtime.Updater).Elem().FieldByName("lifecycle")
	if lifecycle.Kind() != reflect.Interface || lifecycle.IsNil() {
		t.Fatal("Updater has no private lifecycle")
	}
	manager := lifecycle.Elem().Elem().FieldByName("manager")
	if manager.Kind() != reflect.Pointer || manager.Pointer() != reflect.ValueOf(runtime.Manager).Pointer() {
		t.Fatal("Updater lifecycle does not reference the Runtime Manager")
	}
}

func TestProductionCompositionResolvesPinnedSlot(t *testing.T) {
	runtime, config := productionFixture(t)
	if runtime.catalog == nil || runtime.catalog.Len() != 2 {
		t.Fatalf("production catalog length=%d", runtime.catalog.Len())
	}
	provenance, err := runtime.catalog.Resolve(config.Lock.Version)
	if err != nil || provenance.Digest != config.Lock.Digest() || provenance.ExecutableSHA256 != config.Lock.Platforms.WindowsAMD64.ExecutableSHA256 {
		t.Fatalf("production provenance=%+v err=%v", provenance, err)
	}
	slot, err := runtime.registry.Resolve(config.Lock.Version)
	if err != nil || slot.Version != config.Lock.Version || slot.ExecutableSHA256 != config.Lock.Platforms.WindowsAMD64.ExecutableSHA256 {
		t.Fatalf("slot=%#v err=%v", slot, err)
	}
	if pointerField(runtime.Manager, "registry") != reflect.ValueOf(runtime.registry).Pointer() {
		t.Fatal("Manager does not use production Registry")
	}
}

func TestProductionCandidateAbsentFailsUpdaterPreflight(t *testing.T) {
	runtime, config := productionFixture(t)
	smoke := config.Smoke.(*recordingSmoke)
	if _, err := runtime.catalog.Resolve("7.3.8"); err != nil {
		t.Fatalf("verified candidate is absent from catalog: %v", err)
	}
	err := runtime.Updater.Promote(context.Background(), "7.3.8")
	if err == nil {
		t.Fatal("absent candidate accepted")
	}
	current, loadErr := runtime.state.LoadState()
	if loadErr != nil || current.ActiveUpstreamVersion != config.Lock.Version {
		t.Fatalf("state=%#v err=%v", current, loadErr)
	}
	if len(smoke.calls) != 0 {
		t.Fatalf("smoke called: %v", smoke.calls)
	}
	if _, statErr := os.Stat(filepath.Join(config.Layout.State, ".update-transaction.json")); !os.IsNotExist(statErr) {
		t.Fatalf("marker exists: %v", statErr)
	}
}

func TestRuntimeCompositionConstructorFailureMatrixAndNoSideEffects(t *testing.T) {
	_, config := productionFixture(t)
	marker := filepath.Join(config.Layout.State, ".update-transaction.json")
	if _, err := New(Config{Layout: config.Layout, Lock: upstreamlock.Lock{}, ACL: config.ACL, Smoke: config.Smoke}); !errors.Is(err, ErrCompositionInvalid) {
		t.Fatalf("invalid lock: %v", err)
	}
	bad := config
	bad.Layout.Locks = filepath.Join(config.Layout.Root, "missing-locks")
	if _, err := New(bad); !errors.Is(err, ErrCompositionInvalid) {
		t.Fatalf("missing locks: %v", err)
	}
	bad = config
	bad.Layout.State = filepath.Join(config.Layout.Root, "missing-state")
	if _, err := New(bad); !errors.Is(err, ErrCompositionInvalid) {
		t.Fatalf("missing state: %v", err)
	}
	bad = config
	bad.Layout.Bin = filepath.Join(config.Layout.Root, "missing-bin")
	if _, err := New(bad); !errors.Is(err, ErrCompositionInvalid) {
		t.Fatalf("registry construction path: %v", err)
	}
	bad = config
	bad.Layout.Instances = filepath.Join(config.Layout.Root, "missing-instances")
	if _, err := New(bad); !errors.Is(err, ErrCompositionInvalid) {
		t.Fatalf("Manager construction path: %v", err)
	}
	bad = config
	bad.Layout.Root = "relative"
	if _, err := New(bad); !errors.Is(err, ErrCompositionInvalid) {
		t.Fatalf("unsafe layout: %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("constructor published marker: %v", err)
	}
}
