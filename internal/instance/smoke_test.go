//go:build windows

package instance

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/trungqwe/dual-pool/internal/cliproxyconfig"
	"github.com/trungqwe/dual-pool/internal/installedslot"
	"github.com/trungqwe/dual-pool/internal/keymaterial"
	"github.com/trungqwe/dual-pool/internal/lockfile"
	"github.com/trungqwe/dual-pool/internal/secretstore"
	"github.com/trungqwe/dual-pool/internal/upstreamcatalog"
)

type smokeInspector struct {
	identity lockfile.ProcessIdentity
	err      error
}

func (i *smokeInspector) Inspect(uint32) (lockfile.ProcessIdentity, error) { return i.identity, i.err }

type fakeSmokeProcess struct {
	pid       uint32
	killed    bool
	killErr   error
	waitErr   error
	killCount int
	waitCount int
}

func (p *fakeSmokeProcess) PID() uint32 { return p.pid }
func (p *fakeSmokeProcess) Kill() error {
	p.killCount++
	if p.killErr != nil {
		return p.killErr
	}
	p.killed = true
	return nil
}
func (p *fakeSmokeProcess) Wait() error { p.waitCount++; return p.waitErr }

type smokeSecretReader struct{ reads int }

func (r *smokeSecretReader) Get(_ secretstore.Purpose) ([]byte, error) {
	r.reads++
	return nil, secretstore.ErrNotFound
}

type productionSmokeSecrets struct {
	values map[secretstore.Purpose][]byte
	reads  int
}

var (
	forcedKillRawOnce      atomic.Bool
	forcedKillClassifyOnce atomic.Bool
)

func (r *productionSmokeSecrets) Get(p secretstore.Purpose) ([]byte, error) {
	r.reads++
	value, ok := r.values[p]
	if !ok {
		return nil, secretstore.ErrNotFound
	}
	return append([]byte(nil), value...), nil
}

type smokeProcessInspector struct {
	identities map[uint32]lockfile.ProcessIdentity
	errors     map[uint32]error
}

func (i *smokeProcessInspector) Inspect(pid uint32) (lockfile.ProcessIdentity, error) {
	if err := i.errors[pid]; err != nil {
		return lockfile.ProcessIdentity{}, err
	}
	identity, ok := i.identities[pid]
	if !ok {
		return lockfile.ProcessIdentity{}, lockfile.ErrProcessNotFound
	}
	return identity, nil
}

type disposableFixture struct {
	manager      *Manager
	smoke        *updaterSmoke
	child        *fakeSmokeProcess
	launches     int
	launchPath   string
	launchArgs   []string
	launchDir    string
	requests     []string
	reader       *smokeSecretReader
	port         int
	requestFn    smokeRequester
	listenerRows func() ([]listener, error)
}

func newDisposableFixture(t *testing.T) *disposableFixture {
	t.Helper()
	manager, _ := legacyInstallFixture(t)
	catalog, err := upstreamcatalog.FromPinnedLock(manager.lock)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := installedslot.NewWithCatalog(manager.layout, manager.acl, catalog, installedslot.WithLockManager(manager.locks), installedslot.WithBinaryVerifier(func(_ context.Context, path string, expected upstreamcatalog.Provenance) error {
		if digest(path) != expected.ExecutableSHA256 {
			return installedslot.ErrUnsafeSlot
		}
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	manager.registry = registry
	if err := registry.Register(context.Background(), manager.lock.Version); err != nil {
		t.Fatalf("register fixture's exact current slot: %v", err)
	}
	fixture := &disposableFixture{manager: manager, child: &fakeSmokeProcess{pid: 4242}, port: 49123, reader: &smokeSecretReader{}}
	manager.reader = fixture.reader
	manager.inspector = &smokeInspector{identity: lockfile.ProcessIdentity{PID: fixture.child.pid, StartTime: 77, Image: manager.executablePath()}}
	random := smokeRandomBytes()
	clientKey, _ := keymaterial.Encode(random[16:48])
	managementKey, _ := keymaterial.Encode(random[48:80])
	fixture.requestFn = func(_ context.Context, _ int, path, header, value string) (int, []byte, error) {
		status := http.StatusUnauthorized
		switch path {
		case smokeHealthPath:
			status = http.StatusOK
		case smokeClientPath:
			if header == "Authorization" && value == "Bearer "+string(clientKey) {
				status = http.StatusOK
			}
		case smokeManagementPath:
			if header == "X-Management-Key" && value == string(managementKey) {
				status = http.StatusOK
			}
		default:
			return 0, nil, errors.New("unexpected smoke endpoint")
		}
		fixture.requests = append(fixture.requests, fmt.Sprintf("%s|%s|%d", path, header, status))
		if status == http.StatusOK && path == smokeHealthPath {
			return status, []byte(`{"status":"ok"}`), nil
		}
		return status, []byte(`{}`), nil
	}
	fixture.listenerRows = func() ([]listener, error) {
		if fixture.child.killed {
			return nil, nil
		}
		return []listener{{address: "127.0.0.1", port: fixture.port, pid: fixture.child.pid}}, nil
	}
	fixture.smoke = newUpdaterSmoke(manager)
	fixture.smoke.deps = smokeDependencies{
		random:     bytes.NewReader(smokeRandomBytes()),
		choosePort: func() (int, error) { return fixture.port, nil },
		launch: func(path string, args []string, dir string) (smokeProcess, error) {
			fixture.launches++
			fixture.launchPath = path
			fixture.launchArgs = append([]string(nil), args...)
			fixture.launchDir = dir
			return fixture.child, nil
		},
		listeners: func() ([]listener, error) { return fixture.listenerRows() },
		request: func(ctx context.Context, port int, path, header, value string) (int, []byte, error) {
			return fixture.requestFn(ctx, port, path, header, value)
		},
	}
	return fixture
}

func smokeRandomBytes() []byte {
	values := make([]byte, 16+4*32)
	for i := range values {
		values[i] = byte(i + 1)
	}
	return values
}

func TestDisposableSmokeUsesExactInstalledTrustedSlot(t *testing.T) {
	f := newDisposableFixture(t)
	if err := f.smoke.Disposable(context.Background(), f.manager.lock.Version); err != nil {
		t.Fatal(err)
	}
	if f.launches != 1 || f.launchPath != f.manager.executablePath() {
		t.Fatalf("launch count=%d path=%q", f.launches, f.launchPath)
	}
	if !reflect.DeepEqual(f.launchArgs, []string{"-config", filepath.Join(f.launchDir, smokeConfigName), "-local-model"}) {
		t.Fatalf("command args=%v", f.launchArgs)
	}
	if !pathWithin(filepath.Join(f.manager.layout.State, smokeRootName), f.launchDir) {
		t.Fatalf("smoke command escaped owned workspace: dir=%q args=%v", f.launchDir, f.launchArgs)
	}
	if !f.child.killed || f.child.killCount != 1 || f.child.waitCount != 1 {
		t.Fatalf("owned process cleanup: %+v", f.child)
	}
}

func TestDisposableSmokeHonorsCanceledContextBeforeWorkspaceCreation(t *testing.T) {
	f := newDisposableFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := f.smoke.Disposable(ctx, f.manager.lock.Version); !errors.Is(err, ErrCompatibilitySmoke) {
		t.Fatalf("canceled Disposable Smoke error=%v", err)
	}
	if f.launches != 0 {
		t.Fatalf("canceled Disposable Smoke launched %d processes", f.launches)
	}
	if _, err := os.Lstat(filepath.Join(f.manager.layout.State, smokeRootName)); !os.IsNotExist(err) {
		t.Fatalf("canceled Smoke created workspace: %v", err)
	}
}
func TestDisposableSmokeRejectsUnknownVersionBeforeLaunch(t *testing.T) {
	f := newDisposableFixture(t)
	if err := f.smoke.Disposable(context.Background(), "7.3.9"); !errors.Is(err, ErrCompatibilitySmoke) {
		t.Fatalf("unknown version error=%v", err)
	}
	if f.launches != 0 {
		t.Fatalf("unknown version launched %d processes", f.launches)
	}
}

func TestDisposableSmokeRejectsHashMismatchBeforeLaunch(t *testing.T) {
	f := newDisposableFixture(t)
	file, err := os.OpenFile(f.manager.executablePath(), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write([]byte("tamper")); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	if err = f.smoke.Disposable(context.Background(), f.manager.lock.Version); !errors.Is(err, ErrCompatibilitySmoke) {
		t.Fatalf("hash mismatch error=%v", err)
	}
	if f.launches != 0 {
		t.Fatalf("hash mismatch launched %d processes", f.launches)
	}
}

func TestDisposableSmokeRejectsUnverifiedChildIdentity(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*disposableFixture)
	}{
		{"wrong-pid", func(f *disposableFixture) {
			f.manager.inspector = &smokeInspector{identity: lockfile.ProcessIdentity{PID: f.child.pid + 1, StartTime: 77, Image: f.manager.executablePath()}}
		}},
		{"zero-start-time", func(f *disposableFixture) {
			f.manager.inspector = &smokeInspector{identity: lockfile.ProcessIdentity{PID: f.child.pid, Image: f.manager.executablePath()}}
		}},
		{"wrong-image", func(f *disposableFixture) {
			f.manager.inspector = &smokeInspector{identity: lockfile.ProcessIdentity{PID: f.child.pid, StartTime: 77, Image: filepath.Join(f.manager.layout.Bin, "foreign.exe")}}
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			f := newDisposableFixture(t)
			if test.name == "changed-image-bytes" {
				f.smoke.deps.launch = func(path string, _ []string, _ string) (smokeProcess, error) {
					if err := os.WriteFile(path, []byte("changed-after-preflight"), 0600); err != nil {
						return nil, err
					}
					return f.child, nil
				}
			} else {
				test.mutate(f)
			}
			if err := f.smoke.Disposable(context.Background(), f.manager.lock.Version); !errors.Is(err, ErrCompatibilitySmoke) {
				t.Fatalf("unverified child identity accepted: %v", err)
			}
			if f.launches != 1 || !f.child.killed || len(f.requests) != 0 {
				t.Fatalf("identity rejection was not before probes or exact cleanup: launches=%d child=%+v requests=%v", f.launches, f.child, f.requests)
			}
		})
	}
}
func TestDisposableSmokeRequiresExactLoopbackListener(t *testing.T) {
	valid := []listener{{address: "127.0.0.1", port: 49123, pid: 4242}}
	ready, err := exactLoopbackListener(valid, 4242, 49123)
	if err != nil || !ready {
		t.Fatalf("exact listener rejected: ready=%v err=%v", ready, err)
	}
	for _, rows := range [][]listener{
		{{address: "0.0.0.0", port: 49123, pid: 4242}},
		{{address: "127.0.0.1", port: 49123, pid: 9}},
		{{address: "127.0.0.1", port: 49123, pid: 4242}, {address: "127.0.0.1", port: 49123, pid: 4242}},
		{{address: "::1", port: 49123, pid: 4242, ipv6: true}},
	} {
		ready, err = exactLoopbackListener(rows, 4242, 49123)
		if err == nil || ready {
			t.Fatalf("unsafe listener topology accepted: rows=%+v ready=%v err=%v", rows, ready, err)
		}
	}
}

func TestDisposableSmokeHealthContract(t *testing.T) {
	keys, err := newSmokeKeys(bytes.NewReader(smokeRandomBytes()[16:]))
	if err != nil {
		t.Fatal(err)
	}
	defer keys.wipe()
	var paths []string
	request := func(_ context.Context, _ int, path, _, _ string) (int, []byte, error) {
		paths = append(paths, path)
		return http.StatusOK, []byte(`{"status":"degraded"}`), nil
	}
	if err = verifySmokeEndpoints(context.Background(), 49123, keys, request); !errors.Is(err, ErrCompatibilitySmoke) {
		t.Fatalf("bad health body accepted: %v", err)
	}
	if !reflect.DeepEqual(paths, []string{smokeHealthPath}) {
		t.Fatalf("endpoint validation continued beyond bad health response: %v", paths)
	}
}
func TestDisposableSmokeAuthenticationContract(t *testing.T) {
	f := newDisposableFixture(t)
	if err := f.smoke.Disposable(context.Background(), f.manager.lock.Version); err != nil {
		t.Fatal(err)
	}
	if len(f.requests) != 7 {
		t.Fatalf("request count=%d requests=%v", len(f.requests), f.requests)
	}
	want := []string{"/healthz||200", "/v1/models||401", "/v1/models|Authorization|200", "/v1/models|Authorization|401", "/v0/management/debug||401", "/v0/management/debug|X-Management-Key|200", "/v0/management/debug|X-Management-Key|401"}
	for i, request := range f.requests {
		if request != want[i] {
			t.Fatalf("request %d contract=%q, want %q", i, request, want[i])
		}
		path := strings.SplitN(request, "|", 2)[0]
		if !isAllowedSmokePath(path) {
			t.Fatalf("unexpected endpoint in smoke allowlist: %v", f.requests)
		}
	}
	if f.reader.reads != 0 {
		t.Fatalf("Disposable read production secret store %d times", f.reader.reads)
	}
}

func isAllowedSmokePath(path string) bool {
	return path == smokeHealthPath || path == smokeClientPath || path == smokeManagementPath
}

func TestDisposableSmokeCleansOwnedProcessAndWorkspace(t *testing.T) {
	f := newDisposableFixture(t)
	if err := f.smoke.Disposable(context.Background(), f.manager.lock.Version); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(f.manager.layout.State, smokeRootName)); err != nil {
		t.Fatalf("protected root should be lazily retained for reuse: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(f.manager.layout.State, smokeRootName))
	if err != nil || len(entries) != 0 {
		t.Fatalf("owned attempt not removed: entries=%v err=%v", entries, err)
	}
}

func TestDisposableSmokePreservesStaleWorkspaceResidue(t *testing.T) {
	f := newDisposableFixture(t)
	root := filepath.Join(f.manager.layout.State, smokeRootName)
	if err := f.manager.acl.Create(root); err != nil {
		t.Fatal(err)
	}
	abandoned := filepath.Join(root, "abandoned-attempt")
	creator, ok := f.manager.acl.(interface{ CreateExclusive(string) error })
	if !ok {
		t.Fatal("production ACL manager has no exclusive directory creation")
	}
	if err := creator.CreateExclusive(abandoned); err != nil {
		t.Fatal(err)
	}
	if err := f.smoke.Disposable(context.Background(), f.manager.lock.Version); !errors.Is(err, ErrCompatibilitySmoke) {
		t.Fatalf("Smoke accepted an earlier incomplete attempt: %v", err)
	}
	if f.launches != 0 {
		t.Fatalf("Smoke launched with stale residue: %d", f.launches)
	}
	if info, err := os.Stat(abandoned); err != nil || !info.IsDir() {
		t.Fatalf("stale attempt was altered or removed: info=%v err=%v", info, err)
	}
}
func TestDisposableSmokeCleanupFailureFailsClosed(t *testing.T) {
	f := newDisposableFixture(t)
	f.child.waitErr = errors.New("synthetic process wait failure")
	err := f.smoke.Disposable(context.Background(), f.manager.lock.Version)
	if !errors.Is(err, ErrCompatibilitySmoke) || !strings.Contains(err.Error(), "synthetic process wait failure") {
		t.Fatalf("cleanup failure result=%v", err)
	}
}

func TestDisposableSmokeAcceptsExpectedForcedTerminationStatus(t *testing.T) {
	f := newDisposableFixture(t)
	f.child.waitErr = &exec.ExitError{}
	if err := f.smoke.Disposable(context.Background(), f.manager.lock.Version); err != nil {
		t.Fatalf("expected deliberate forced termination to be accepted: %v", err)
	}
}

func TestDisposableSmokeRejectsUnexpectedWaitFailure(t *testing.T) {
	f := newDisposableFixture(t)
	f.child.waitErr = errors.New("unexpected wait failure")
	if err := f.smoke.Disposable(context.Background(), f.manager.lock.Version); !errors.Is(err, ErrCompatibilitySmoke) || !strings.Contains(err.Error(), "unexpected wait failure") {
		t.Fatalf("unexpected wait failure classification=%v", err)
	}
}

func TestDisposableSmokeRejectsChildGoneBeforeOwnedTermination(t *testing.T) {
	f := newDisposableFixture(t)
	f.child.killErr = os.ErrProcessDone
	if err := f.smoke.Disposable(context.Background(), f.manager.lock.Version); !errors.Is(err, ErrCompatibilitySmoke) {
		t.Fatalf("already-gone child was accepted: %v", err)
	}
}

func TestSmokeHelperProcess(t *testing.T) {
	if os.Getenv("DUALPOOL_SMOKE_HELPER") != "1" {
		return
	}
	select {}
}

func startSmokeHelper(t *testing.T) *commandSmokeProcess {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestSmokeHelperProcess$")
	cmd.Env = append(os.Environ(), "DUALPOOL_SMOKE_HELPER=1")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	return &commandSmokeProcess{cmd: cmd}
}

func TestCommandSmokeProcessForcedKillProducesExitError(t *testing.T) {
	if forcedKillRawOnce.Swap(true) {
		t.Skip("raw os/exec regression already exercised in this test process")
	}
	child := startSmokeHelper(t)
	if err := child.Kill(); err != nil {
		t.Fatalf("Kill failed: %v", err)
	}
	err := child.Wait()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("raw Wait error=%T %v, want *exec.ExitError after deliberate Kill", err, err)
	}
}

func TestTerminateSmokeProcessAcceptsExpectedForcedTermination(t *testing.T) {
	if forcedKillClassifyOnce.Swap(true) {
		t.Skip("forced-exit classification already exercised in this test process")
	}
	child := startSmokeHelper(t)
	if err := terminateSmokeProcess(child); err != nil {
		t.Fatalf("expected successful deliberate termination, got %v", err)
	}
}

func TestDisposableSmokeRefusesUnexpectedWorkspaceContent(t *testing.T) {
	f := newDisposableFixture(t)
	f.smoke.deps.launch = func(_ string, _ []string, dir string) (smokeProcess, error) {
		if err := os.WriteFile(filepath.Join(dir, "foreign.txt"), []byte("preserve"), 0600); err != nil {
			return nil, err
		}
		return f.child, nil
	}
	if err := f.smoke.Disposable(context.Background(), f.manager.lock.Version); !errors.Is(err, ErrCompatibilitySmoke) {
		t.Fatalf("unexpected workspace content accepted: %v", err)
	}
	if !f.child.killed {
		t.Fatal("smoke process was not stopped after the unexpected file appeared")
	}
	attempts, err := os.ReadDir(filepath.Join(f.manager.layout.State, smokeRootName))
	if err != nil || len(attempts) != 1 {
		t.Fatalf("foreign content was deleted or workspace disappeared: entries=%v err=%v", attempts, err)
	}
	foreign := filepath.Join(f.manager.layout.State, smokeRootName, attempts[0].Name(), "foreign.txt")
	if body, readErr := os.ReadFile(foreign); readErr != nil || string(body) != "preserve" {
		t.Fatalf("unexpected file was not preserved: body=%q err=%v", body, readErr)
	}
}
func TestDisposableSmokeDoesNotMutateStateRegistryOrInstanceRecords(t *testing.T) {
	f := newDisposableFixture(t)
	activeBefore, err := f.manager.state.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	registryPath := filepath.Join(f.manager.layout.Bin, "cliproxyapi", "installed-slots.json")
	registryBefore := digest(registryPath)
	stateBefore := digest(filepath.Join(f.manager.layout.State, "state.json"))
	instancesBefore, err := os.ReadDir(f.manager.layout.Instances)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.smoke.Disposable(context.Background(), f.manager.lock.Version); err != nil {
		t.Fatal(err)
	}
	activeAfter, err := f.manager.state.LoadState()
	if err != nil || activeAfter.ActiveUpstreamVersion != activeBefore.ActiveUpstreamVersion || digest(registryPath) != registryBefore || digest(filepath.Join(f.manager.layout.State, "state.json")) != stateBefore {
		t.Fatalf("state bytes or registry changed: state=%q err=%v", activeAfter.ActiveUpstreamVersion, err)
	}
	instancesAfter, err := os.ReadDir(f.manager.layout.Instances)
	if err != nil || !reflect.DeepEqual(instancesAfter, instancesBefore) {
		t.Fatalf("instance records changed: before=%v after=%v err=%v", instancesBefore, instancesAfter, err)
	}
	if _, err = os.Lstat(filepath.Join(f.manager.layout.State, ".update-transaction.json")); !os.IsNotExist(err) {
		t.Fatalf("Disposable left an update marker: %v", err)
	}
}
func TestConfigPairPreflightIntegration(t *testing.T) {
	f, _, _, _ := newProductionSmokeFixture(t, true)
	if err := f.smoke.Production(context.Background(), f.manager.lock.Version); err != nil {
		t.Fatalf("default managed-pair preflight rejected a valid TEMP pair: %v", err)
	}
}
func TestProductionSmokeRequiresExactActiveVersion(t *testing.T) {
	f := newDisposableFixture(t)
	if err := f.smoke.Production(context.Background(), "7.3.8"); !errors.Is(err, ErrCompatibilitySmoke) {
		t.Fatalf("production smoke accepted a different active version: %v", err)
	}
}

func newProductionSmokeFixture(t *testing.T, managedPair ...bool) (*disposableFixture, *productionSmokeSecrets, map[cliproxyconfig.ID]ProcessRecord, map[cliproxyconfig.ID]uint32) {
	t.Helper()
	f := newDisposableFixture(t)
	secrets := &productionSmokeSecrets{values: map[secretstore.Purpose][]byte{
		secretstore.CodexClientKey: bytes.Repeat([]byte{0x11}, 32), secretstore.CodexManagementKey: bytes.Repeat([]byte{0x22}, 32),
		secretstore.GoogleClientKey: bytes.Repeat([]byte{0x33}, 32), secretstore.GoogleManagementKey: bytes.Repeat([]byte{0x44}, 32),
	}}
	f.manager.reader = secrets
	if len(managedPair) != 0 && managedPair[0] {
		generator, err := cliproxyconfig.New(f.manager.layout, f.manager.acl, secrets, f.manager.lock)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = generator.GeneratePair(); err != nil {
			t.Fatalf("generate TEMP managed pair: %v", err)
		}
		f.smoke.deps.preflightConfigPair = nil
	} else {
		for _, id := range []cliproxyconfig.ID{cliproxyconfig.Codex, cliproxyconfig.Google} {
			root := f.manager.instancePath(id)
			if err := f.manager.acl.Create(root); err != nil {
				t.Fatal(err)
			}
			file, err := f.manager.acl.CreateFile(filepath.Join(root, "config.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err = file.Write([]byte("synthetic managed config fixture")); err != nil {
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
			if err = f.manager.acl.InspectFile(filepath.Join(root, "config.yaml")); err != nil {
				t.Fatal(err)
			}
		}
		f.smoke.deps.preflightConfigPair = func() error { return nil }
	}
	registry := f.manager.registry.(*installedslot.Registry)
	slot, err := registry.Resolve(f.manager.lock.Version)
	if err != nil {
		t.Fatal(err)
	}
	records := map[cliproxyconfig.ID]ProcessRecord{}
	pids := map[cliproxyconfig.ID]uint32{cliproxyconfig.Codex: 7101, cliproxyconfig.Google: 7102}
	identities := map[uint32]lockfile.ProcessIdentity{}
	for id, pid := range pids {
		_, port, rootErr := f.manager.instanceRoot(id)
		if rootErr != nil {
			t.Fatal(rootErr)
		}
		record := ProcessRecord{SchemaVersion: 2, InstanceID: string(id), PID: pid, StartTime: uint64(pid) + 100, ExecutableSHA256: slot.ExecutableSHA256,
			ConfigSHA256: digest(filepath.Join(f.manager.instancePath(id), "config.yaml")), Port: port, UpstreamVersion: slot.Version, ManifestSHA256: slot.ManifestSHA256}
		records[id] = record
		identities[pid] = lockfile.ProcessIdentity{PID: pid, StartTime: record.StartTime, Image: slot.ExecutablePath}
	}
	f.manager.inspector = &smokeProcessInspector{identities: identities, errors: map[uint32]error{}}
	return f, secrets, records, pids
}

func writeProductionSmokeRecords(t *testing.T, f *disposableFixture, records map[cliproxyconfig.ID]ProcessRecord, ids ...cliproxyconfig.ID) {
	t.Helper()
	for _, id := range ids {
		path := filepath.Join(f.manager.instancePath(id), "process.json")
		if err := writeProtectedJSON(f.manager.acl, path, records[id]); err != nil {
			t.Fatalf("write TEMP process record for %s: %v", id, err)
		}
	}
}

func TestProductionSmokeHonorsCanceledContext(t *testing.T) {
	f, _, _, _ := newProductionSmokeFixture(t)
	readsBefore := f.manager.reader.(*productionSmokeSecrets).reads
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := f.smoke.Production(ctx, f.manager.lock.Version); !errors.Is(err, ErrCompatibilitySmoke) {
		t.Fatalf("canceled Production Smoke error=%v", err)
	}
	if f.manager.reader.(*productionSmokeSecrets).reads != readsBefore || len(f.requests) != 0 {
		t.Fatal("canceled Production Smoke performed auth probes")
	}
}
func TestProductionSmokeRejectsUnreadyConfigPairBeforeStatus(t *testing.T) {
	f, _, _, _ := newProductionSmokeFixture(t)
	called := 0
	f.smoke.deps.preflightConfigPair = func() error { called++; return errors.New("synthetic unready config pair") }
	if err := f.smoke.Production(context.Background(), f.manager.lock.Version); !errors.Is(err, ErrCompatibilitySmoke) {
		t.Fatalf("unready config pair accepted: %v", err)
	}
	if called != 1 {
		t.Fatalf("config pair preflight calls=%d", called)
	}
}
func TestProductionSmokeAcceptsHealthyCandidatePools(t *testing.T) {
	f, _, records, _ := newProductionSmokeFixture(t)
	writeProductionSmokeRecords(t, f, records, cliproxyconfig.Codex, cliproxyconfig.Google)
	rows := []listener{}
	for id, record := range records {
		_ = id
		rows = append(rows, listener{address: "127.0.0.1", port: record.Port, pid: record.PID})
	}
	f.smoke.deps.listeners = func() ([]listener, error) { return rows, nil }
	f.smoke.deps.request = productionSmokeRequester(f, f.manager.reader.(*productionSmokeSecrets), records)
	if err := f.smoke.Production(context.Background(), f.manager.lock.Version); err != nil {
		t.Fatalf("healthy running pools rejected: %v", err)
	}
}

func TestProductionSmokeAcceptsIntentionallyStoppedPools(t *testing.T) {
	f, _, _, _ := newProductionSmokeFixture(t)
	if err := f.smoke.Production(context.Background(), f.manager.lock.Version); err != nil {
		t.Fatalf("zero-running-pool state rejected: %v", err)
	}
}

func TestProductionSmokeRejectsStaleProcessRecord(t *testing.T) {
	f, _, records, pids := newProductionSmokeFixture(t)
	writeProductionSmokeRecords(t, f, records, cliproxyconfig.Codex)
	f.manager.inspector = &smokeProcessInspector{identities: map[uint32]lockfile.ProcessIdentity{}, errors: map[uint32]error{pids[cliproxyconfig.Codex]: lockfile.ErrProcessNotFound}}
	if err := f.smoke.Production(context.Background(), f.manager.lock.Version); !errors.Is(err, ErrCompatibilitySmoke) {
		t.Fatalf("stale nonzero-PID record accepted: %v", err)
	}
}

func TestProductionSmokeRejectsWrongRunningVersion(t *testing.T) {
	f, _, records, _ := newProductionSmokeFixture(t)
	record := records[cliproxyconfig.Codex]
	record.UpstreamVersion = "7.3.8"
	records[cliproxyconfig.Codex] = record
	writeProductionSmokeRecords(t, f, records, cliproxyconfig.Codex)
	if err := f.smoke.Production(context.Background(), f.manager.lock.Version); !errors.Is(err, ErrCompatibilitySmoke) {
		t.Fatalf("wrong running version accepted: %v", err)
	}
}

func TestProductionSmokeRejectsConfigHashDrift(t *testing.T) {
	f, _, records, _ := newProductionSmokeFixture(t)
	writeProductionSmokeRecords(t, f, records, cliproxyconfig.Codex)
	path := filepath.Join(f.manager.instancePath(cliproxyconfig.Codex), "config.yaml")
	if err := os.WriteFile(path, []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := f.smoke.Production(context.Background(), f.manager.lock.Version); !errors.Is(err, ErrCompatibilitySmoke) {
		t.Fatalf("config drift accepted: %v", err)
	}
}

func TestProductionSmokeReusesExistingReadinessContract(t *testing.T) {
	f, secrets, records, _ := newProductionSmokeFixture(t)
	writeProductionSmokeRecords(t, f, records, cliproxyconfig.Codex)
	f.smoke.deps.listeners = func() ([]listener, error) {
		return []listener{{address: "127.0.0.1", port: records[cliproxyconfig.Codex].Port, pid: records[cliproxyconfig.Codex].PID}}, nil
	}
	f.smoke.deps.request = productionSmokeRequester(f, secrets, records)
	if err := f.smoke.Production(context.Background(), f.manager.lock.Version); err != nil {
		t.Fatal(err)
	}
	paths := make([]string, len(f.requests))
	for i, request := range f.requests {
		paths[i] = strings.SplitN(request, "|", 2)[0]
	}
	want := []string{smokeHealthPath, smokeClientPath, smokeClientPath, smokeManagementPath, smokeManagementPath, smokeClientPath, smokeManagementPath}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("production readiness did not use L0/L1/L2 allowlist: %v", paths)
	}
}

func TestProductionSmokeDoesNotAcquireGlobalAgain(t *testing.T) {
	f, _, _, _ := newProductionSmokeFixture(t)
	guard, err := f.manager.locks.AcquireGlobal()
	if err != nil {
		t.Fatal(err)
	}
	defer guard.Release()
	if err = f.smoke.Production(context.Background(), f.manager.lock.Version); err != nil {
		t.Fatalf("Production Smoke attempted nested GLOBAL or failed under held lock: %v", err)
	}
}

func productionSmokeRequester(f *disposableFixture, reader *productionSmokeSecrets, records map[cliproxyconfig.ID]ProcessRecord) smokeRequester {
	return func(_ context.Context, port int, path, header, value string) (int, []byte, error) {
		status := http.StatusUnauthorized
		clientPurpose, managementPurpose := secretstore.CodexClientKey, secretstore.CodexManagementKey
		for id, record := range records {
			if record.Port == port {
				clientPurpose, managementPurpose, _, _ = purposes(id)
				break
			}
		}
		if raw, ok := reader.values[clientPurpose]; ok && header == "Authorization" {
			wire, err := keymaterial.Encode(raw)
			if err != nil {
				return 0, nil, err
			}
			if value == "Bearer "+string(wire) {
				status = http.StatusOK
			}
			secretstore.Zero(wire)
		}
		if raw, ok := reader.values[managementPurpose]; ok && header == "X-Management-Key" {
			wire, err := keymaterial.Encode(raw)
			if err != nil {
				return 0, nil, err
			}
			if value == string(wire) {
				status = http.StatusOK
			}
			secretstore.Zero(wire)
		}
		if path == smokeHealthPath {
			status = http.StatusOK
			f.requests = append(f.requests, fmt.Sprintf("%s|%s|%d", path, header, status))
			return status, []byte(`{"status":"ok"}`), nil
		}
		if header == "" {
			status = http.StatusUnauthorized
		}
		f.requests = append(f.requests, fmt.Sprintf("%s|%s|%d", path, header, status))
		return status, []byte(`{}`), nil
	}
}
func TestSmokePortAndEndpointAllowlist(t *testing.T) {
	for _, port := range []int{0, 8317, 8318, 65536} {
		if safeSmokePort(port) {
			t.Errorf("unsafe smoke port %d accepted", port)
		}
	}
	for _, path := range []string{"/v1/chat/completions", "/v1/responses", "/v1beta/models/generateContent", "/unknown"} {
		if isAllowedSmokePath(path) {
			t.Errorf("provider/generation endpoint %q added to allowlist", path)
		}
	}
	if safeSmokePort(49123) != true || cliproxyconfig.CodexPort == 0 {
		t.Fatal("valid loopback test port or production port contract changed")
	}
}
