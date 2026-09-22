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
	"github.com/trungqwe/dual-pool/internal/upstreamstage"
	"github.com/trungqwe/dual-pool/internal/winacl"
)

func TestMain(m *testing.M) {
	if len(os.Args) == 2 && os.Args[1] == "-h" {
		fmt.Println("CLIProxyAPI Version: 7.3.7, Commit: b773607e3e7756dc6020a291825e4eb08899595a, BuiltAt: synthetic-test")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type nilACL struct{}

func (*nilACL) Create(string) error                 { return nil }
func (*nilACL) Inspect(string) error                { return nil }
func (*nilACL) CreateFile(string) (*os.File, error) { return nil, nil }
func (*nilACL) InspectFile(string) error            { return nil }

func TestNewRejectsTypedNilACL(t *testing.T) {
	var acl *nilACL
	if !isNil(acl) {
		t.Fatal("typed nil ACL not detected")
	}
	if _, err := New(Config{ACL: acl}); !errors.Is(err, ErrCompositionInvalid) {
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
	config := Config{Layout: layout, Lock: pin, ACL: acl}
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
	if _, err := os.Lstat(filepath.Join(runtime.layout.State, "compat-smoke")); !os.IsNotExist(err) {
		t.Fatalf("Runtime.New eagerly created compatibility Smoke workspace: %v", err)
	}
	updaterLocks := pointerField(runtime.Updater, "locks")
	if updaterLocks == 0 || pointerField(runtime.Manager, "locks") != updaterLocks || pointerField(runtime.state, "locks") != updaterLocks || pointerField(runtime.registry, "locks") != updaterLocks || reflect.ValueOf(runtime.locks).Pointer() != updaterLocks {
		t.Fatal("lock Manager is not shared")
	}
}

func TestRuntimeProductionSmokeUsesExactManager(t *testing.T) {
	runtime, _ := productionFixture(t)
	smoke := reflect.ValueOf(runtime.Updater).Elem().FieldByName("smoke")
	if smoke.Kind() != reflect.Interface || smoke.IsNil() || smoke.Elem().Type().String() != "*instance.updaterSmoke" {
		t.Fatalf("Runtime production Smoke type=%v", smoke)
	}
	manager := smoke.Elem().Elem().FieldByName("manager")
	if manager.Kind() != reflect.Pointer || manager.Pointer() != reflect.ValueOf(runtime.Manager).Pointer() {
		t.Fatal("Runtime production Smoke does not reference the exact Manager")
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

func TestProductionCompositionBindsPinnedV738DigestToRegistry(t *testing.T) {
	const wantReceiptDigest = "0e653e4f01e00c05a44c662e7a7b7321916e7705c901db370aec3cb1116a9776"
	runtime, _ := productionFixture(t)
	provenance, err := runtime.catalog.Resolve("7.3.8")
	if err != nil || provenance.Digest != wantReceiptDigest {
		t.Fatalf("catalog v7.3.8 provenance digest=%q err=%v", provenance.Digest, err)
	}
	if got := pointerField(runtime.registry, "catalog"); got != reflect.ValueOf(runtime.catalog).Pointer() {
		t.Fatalf("Registry catalog pointer=%x, production catalog=%x", got, reflect.ValueOf(runtime.catalog).Pointer())
	}
}

func TestUninstalledProductionCandidateFailsUpdaterPreflight(t *testing.T) {
	runtime, config := productionFixture(t)
	if _, err := runtime.catalog.Resolve("7.3.8"); err != nil {
		t.Fatalf("verified candidate is absent from catalog: %v", err)
	}
	err := runtime.Updater.Promote(context.Background(), "7.3.8")
	if !errors.Is(err, installedslot.ErrSlotUnknown) {
		t.Fatalf("uninstalled candidate error=%v, want registry absence", err)
	}
	current, loadErr := runtime.state.LoadState()
	if loadErr != nil || current.ActiveUpstreamVersion != config.Lock.Version {
		t.Fatalf("state=%#v err=%v", current, loadErr)
	}
	if _, statErr := os.Stat(filepath.Join(config.Layout.State, "compat-smoke")); !os.IsNotExist(statErr) {
		t.Fatalf("preflight failure created the lazy smoke workspace: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(config.Layout.State, ".update-transaction.json")); !os.IsNotExist(statErr) {
		t.Fatalf("marker exists: %v", statErr)
	}
}

type failStageDownload struct {
	count    int
	platform upstreamlock.Platform
	path     string
	err      error
}

func (d *failStageDownload) Download(_ context.Context, platform upstreamlock.Platform, path string) (upstreamstage.DownloadResult, error) {
	d.count++
	d.platform = platform
	d.path = path
	return upstreamstage.DownloadResult{}, d.err
}

func TestRuntimeCandidateStageRootIsLazyAndUsesExactSharedLock(t *testing.T) {
	runtime, config := productionFixture(t)
	stageRoot := filepath.Join(config.Layout.Bin, "upstream-stage")
	if _, err := os.Lstat(stageRoot); !os.IsNotExist(err) {
		t.Fatalf("Runtime.New eagerly created stage root: %v", err)
	}
	if reflect.ValueOf(runtime.locks).Pointer() != pointerField(runtime.Manager, "locks") || pointerField(runtime.state, "locks") != reflect.ValueOf(runtime.locks).Pointer() || pointerField(runtime.registry, "locks") != reflect.ValueOf(runtime.locks).Pointer() || pointerField(runtime.Updater, "locks") != reflect.ValueOf(runtime.locks).Pointer() {
		t.Fatal("runtime composition authorities do not share the lock manager")
	}
	if err := config.ACL.Create(stageRoot); err != nil {
		t.Fatal(err)
	}
	stager, err := runtime.candidateStager()
	if err != nil {
		t.Fatal(err)
	}
	stagerLock := reflect.ValueOf(stager).Elem().FieldByName("locks")
	if stagerLock.Kind() != reflect.Pointer || stagerLock.Pointer() != reflect.ValueOf(runtime.locks).Pointer() {
		t.Fatal("candidate Stager does not share Runtime GLOBAL lock manager by pointer identity")
	}
}

func TestRuntimeCandidateStageCreatesRootOnlyOnRequestAndRequestsExactV738(t *testing.T) {
	runtime, config := productionFixture(t)
	stageRoot := filepath.Join(config.Layout.Bin, "upstream-stage")
	if _, err := os.Lstat(stageRoot); !os.IsNotExist(err) {
		t.Fatalf("candidate stage root exists before request: %v", err)
	}
	failure := errors.New("stop before network")
	downloader := &failStageDownload{err: failure}
	_, err := runtime.stageVerifiedCandidate(context.Background(), upstreamstage.WithDownloader(downloader))
	if !errors.Is(err, failure) {
		t.Fatalf("stage result error=%v", err)
	}
	if downloader.count != 1 || downloader.platform.Artifact != "CLIProxyAPI_7.3.8_windows_amd64.zip" || downloader.platform.DownloadURL != "https://github.com/router-for-me/CLIProxyAPI/releases/download/v7.3.8/CLIProxyAPI_7.3.8_windows_amd64.zip" || downloader.platform.ArchiveSHA256 != "5e3278ac9b57d16df503fd845827a6fdb57ec241f102b35899788287eb431351" || downloader.platform.ExecutableSHA256 != "479da2fb56eb3db11a76e19adeb2e10c2a4069a512ab5e3933ac4c50628360fd" {
		t.Fatalf("downloader received wrong release: %+v calls=%d", downloader.platform, downloader.count)
	}
	if err = config.ACL.Inspect(stageRoot); err != nil {
		t.Fatalf("request did not create/protect lazy stage root: %v", err)
	}
	entries, err := os.ReadDir(stageRoot)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed download left stage residue: %v entries=%v", err, entries)
	}
}

func TestRuntimeCompositionConstructorFailureMatrixAndNoSideEffects(t *testing.T) {
	_, config := productionFixture(t)
	marker := filepath.Join(config.Layout.State, ".update-transaction.json")
	if _, err := New(Config{Layout: config.Layout, Lock: upstreamlock.Lock{}, ACL: config.ACL}); !errors.Is(err, ErrCompositionInvalid) {
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
