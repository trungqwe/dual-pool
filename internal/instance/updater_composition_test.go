package instance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/trungqwe/dual-pool/internal/cliproxyconfig"
	"github.com/trungqwe/dual-pool/internal/dataroot"
	"github.com/trungqwe/dual-pool/internal/installedslot"
	"github.com/trungqwe/dual-pool/internal/lockfile"
	"github.com/trungqwe/dual-pool/internal/state"
	"github.com/trungqwe/dual-pool/internal/update"
	"github.com/trungqwe/dual-pool/internal/upstreamlock"
	"golang.org/x/sys/windows"
)

type composedRegistry struct {
	slots map[string]installedslot.ResolvedSlot
	seen  []string
}

func (r *composedRegistry) VerifyInstalled(_ context.Context, version string) error {
	r.seen = append(r.seen, "verify:"+version)
	if _, ok := r.slots[version]; !ok {
		return installedslot.ErrSlotUnknown
	}
	return nil
}
func (r *composedRegistry) Resolve(version string) (installedslot.ResolvedSlot, error) {
	r.seen = append(r.seen, "resolve:"+version)
	slot, ok := r.slots[version]
	if !ok {
		return installedslot.ResolvedSlot{}, installedslot.ErrSlotUnknown
	}
	return slot, nil
}
func (*composedRegistry) RegisterLocked(context.Context, string) error { return nil }

type composedSmoke struct {
	calls       []string
	failVersion string
}

func (s *composedSmoke) Disposable(_ context.Context, version string) error {
	s.calls = append(s.calls, "disposable:"+version)
	return nil
}
func (s *composedSmoke) Production(_ context.Context, version string) error {
	s.calls = append(s.calls, "production:"+version)
	if version == s.failVersion {
		return errors.New("synthetic smoke failure")
	}
	return nil
}

type composedMarkerSecurity struct{}

func (composedMarkerSecurity) InspectDir(string) error { return nil }
func (composedMarkerSecurity) CreateFile(path string) (*os.File, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE|windows.READ_CONTROL|windows.DELETE, 0, nil, windows.CREATE_NEW, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(h), path), nil
}
func (composedMarkerSecurity) InspectHandle(file *os.File) error {
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 {
		return update.ErrRecoveryUnresolved
	}
	return nil
}

type composedFixture struct {
	t             *testing.T
	manager       *Manager
	lifecycle     *updaterLifecycle
	registry      *composedRegistry
	store         *state.Store
	locks         *lockfile.Manager
	smoke         *composedSmoke
	records       map[cliproxyconfig.ID]ProcessRecord
	starts, stops []string
	startCount    int
	stopCount     int
	failStartCall int
	failStopCall  int
	markerDir     string
}

func newComposedFixture(t *testing.T, running ...state.Pool) *composedFixture {
	t.Helper()
	root := t.TempDir()
	layout := dataroot.Layout{Root: root, Bin: filepath.Join(root, "bin"), Instances: filepath.Join(root, "instances"), State: filepath.Join(root, "state"), Locks: filepath.Join(root, "locks")}
	for _, dir := range []string{layout.Bin, layout.Instances, layout.State, layout.Locks} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	locks, err := lockfile.NewManager(layout.Locks)
	if err != nil {
		t.Fatal(err)
	}
	store, err := state.NewStore(layout.State, state.WithLockManager(locks))
	if err != nil {
		t.Fatal(err)
	}
	if err = store.SaveState(composedState("vA")); err != nil {
		t.Fatal(err)
	}
	registry := &composedRegistry{slots: map[string]installedslot.ResolvedSlot{}}
	for index, version := range []string{"vA", "vB"} {
		body := []byte("synthetic-slot-" + version)
		dir := filepath.Join(layout.Bin, version)
		if err = os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		exe := filepath.Join(dir, "cliproxyapi.exe")
		if err = os.WriteFile(exe, body, 0600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(body)
		registry.slots[version] = installedslot.ResolvedSlot{Version: version, ExecutablePath: exe, ExecutableBasename: "cliproxyapi.exe", ExecutableSHA256: hex.EncodeToString(sum[:]), ManifestSHA256: hex.EncodeToString(bytes32(byte(index + 1))), Platform: "windows_amd64", ConfigAdapterVersion: "adapter-v1"}
	}
	f := &composedFixture{t: t, registry: registry, store: store, locks: locks, smoke: &composedSmoke{}, records: map[cliproxyconfig.ID]ProcessRecord{}, markerDir: layout.State}
	f.manager = &Manager{layout: layout, lock: upstreamlock.Lock{ConfigAdapterVersion: "adapter-v1"}, locks: locks, registry: registry, state: store}
	f.manager.updaterStatus = f.status
	f.manager.updaterStop = f.stop
	f.manager.updaterStart = f.start
	f.manager.updaterPortOccupied = func(int) (bool, error) { return false, nil }
	f.lifecycle = f.manager.updaterLifecycle()
	for _, pool := range running {
		id, _ := lifecyclePoolID(pool)
		f.records[id] = f.record(id, registry.slots["vA"], uint32(len(f.records)+10))
	}
	return f
}

func bytes32(value byte) []byte {
	out := make([]byte, 32)
	for i := range out {
		out[i] = value
	}
	return out
}

func composedState(version string) state.State {
	return state.State{SchemaVersion: 1, InstallID: "install_fixture_01", ActiveUpstreamVersion: version, Instances: state.Instances{Codex: state.Instance{Port: cliproxyconfig.CodexPort, Status: state.InstanceStopped}, Google: state.Instance{Port: cliproxyconfig.GooglePort, Status: state.InstanceStopped}}, Antigravity: state.Antigravity{Mode: state.ModeDisabled, BridgePort: 51074}, Accounts: []state.Account{}}
}

func (f *composedFixture) record(id cliproxyconfig.ID, slot installedslot.ResolvedSlot, pid uint32) ProcessRecord {
	port := cliproxyconfig.CodexPort
	if id == cliproxyconfig.Google {
		port = cliproxyconfig.GooglePort
	}
	return ProcessRecord{SchemaVersion: 2, InstanceID: string(id), PID: pid, StartTime: uint64(pid) + 100, ExecutableSHA256: slot.ExecutableSHA256, ConfigSHA256: hex.EncodeToString(bytes32(9)), Port: port, UpstreamVersion: slot.Version, ManifestSHA256: slot.ManifestSHA256}
}
func (f *composedFixture) status(id cliproxyconfig.ID) (Status, error) {
	record, ok := f.records[id]
	if !ok {
		return Status{ID: id}, ErrNotRunning
	}
	if _, err := f.manager.recordSlot(record); err != nil {
		return Status{ID: id, Record: record}, ErrUnverifiable
	}
	return Status{ID: id, Running: true, Record: record}, nil
}
func (f *composedFixture) stop(id cliproxyconfig.ID) error {
	f.stopCount++
	if f.failStopCall == f.stopCount {
		return errors.New("synthetic stop failure")
	}
	record, ok := f.records[id]
	if !ok {
		return nil
	}
	if _, err := f.manager.recordSlot(record); err != nil {
		return err
	}
	f.stops = append(f.stops, string(id)+":"+record.UpstreamVersion)
	delete(f.records, id)
	return nil
}
func (f *composedFixture) start(ctx context.Context, id cliproxyconfig.ID) (Status, error) {
	f.startCount++
	if f.failStartCall == f.startCount {
		return Status{}, errors.New("synthetic start failure")
	}
	slot, err := f.manager.activeSlot(ctx)
	if err != nil {
		return Status{}, err
	}
	record := f.record(id, slot, uint32(100+f.startCount))
	f.records[id] = record
	f.starts = append(f.starts, string(id)+":"+slot.Version)
	return Status{ID: id, Running: true, Record: record}, nil
}
func (f *composedFixture) updater(fault func(update.FaultPoint) error) *update.Updater {
	f.t.Helper()
	u, err := composeUpdaterForTest(f.manager, f.smoke, composedMarkerSecurity{}, fault, func() (string, error) { return "00112233445566778899aabbccddeeff", nil })
	if err != nil {
		f.t.Fatal(err)
	}
	return u
}

func updaterPointer(updater *update.Updater, name string) uintptr {
	field := reflect.ValueOf(updater).Elem().FieldByName(name)
	if field.Kind() == reflect.Interface {
		field = field.Elem()
	}
	return field.Pointer()
}

func TestComposeUpdaterUsesExactManagerAuthorities(t *testing.T) {
	f := newComposedFixture(t)
	updater := f.updater(nil)
	if updaterPointer(updater, "locks") != reflect.ValueOf(f.manager.locks).Pointer() {
		t.Fatal("Updater locks differ from Manager locks")
	}
	if updaterPointer(updater, "state") != reflect.ValueOf(f.manager.state).Pointer() {
		t.Fatal("Updater state differs from Manager state")
	}
	if updaterPointer(updater, "verifier") != reflect.ValueOf(f.manager.registry).Pointer() {
		t.Fatal("Updater verifier differs from Manager registry")
	}
	markers := reflect.ValueOf(updater).Elem().FieldByName("markers")
	if markers.Elem().FieldByName("dir").String() != f.manager.layout.State {
		t.Fatal("Updater marker directory differs from Manager state directory")
	}
}
func (f *composedFixture) active() string {
	s, err := f.store.LoadState()
	if err != nil {
		f.t.Fatal(err)
	}
	return s.ActiveUpstreamVersion
}
func (f *composedFixture) markerExists() bool {
	_, err := os.Stat(filepath.Join(f.markerDir, ".update-transaction.json"))
	return err == nil
}

func TestUpdaterLifecycleDoesNotReacquireGlobal(t *testing.T) {
	f := newComposedFixture(t)
	guard, err := f.locks.AcquireGlobal()
	if err != nil {
		t.Fatal(err)
	}
	defer guard.Release()
	if _, err = f.lifecycle.CaptureRunning(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = f.lifecycle.Stop(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if err = f.lifecycle.Start(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeCompositionGlobalThenStateFileLock(t *testing.T) {
	f := newComposedFixture(t)
	guard, err := f.locks.AcquireGlobal()
	if err != nil {
		t.Fatal(err)
	}
	if err = f.store.SaveState(composedState("vB")); err != nil {
		_ = guard.Release()
		t.Fatal(err)
	}
	if err = guard.Release(); err != nil {
		t.Fatal(err)
	}
	if f.active() != "vB" {
		t.Fatalf("active=%s", f.active())
	}
}

func TestUpdaterLifecycleCanonicalizesPoolOrderBeforeMutation(t *testing.T) {
	f := newComposedFixture(t)
	if err := f.lifecycle.Start(context.Background(), []state.Pool{state.PoolGoogle, state.PoolCodex}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.starts, []string{"codex:vA", "google:vA"}) {
		t.Fatalf("starts=%v", f.starts)
	}
}

func TestUpdaterLifecycleCaptureRunningExactPools(t *testing.T) {
	for _, want := range [][]state.Pool{nil, {state.PoolCodex}, {state.PoolGoogle}, {state.PoolCodex, state.PoolGoogle}} {
		f := newComposedFixture(t, want...)
		got, err := f.lifecycle.CaptureRunning(context.Background())
		if err != nil || len(got) != len(want) || !reflect.DeepEqual(got, append([]state.Pool{}, want...)) {
			t.Fatalf("got %v %v, want %v", got, err, want)
		}
	}
}

func TestUpdaterLifecycleStaleRecordFreePortIsStopped(t *testing.T) {
	f := newComposedFixture(t, state.PoolCodex)
	record := f.records[cliproxyconfig.Codex]
	f.manager.updaterStatus = func(id cliproxyconfig.ID) (Status, error) {
		if id == cliproxyconfig.Codex {
			return Status{ID: id, Record: record}, ErrNotRunning
		}
		return Status{ID: id}, ErrNotRunning
	}
	got, err := f.lifecycle.CaptureRunning(context.Background())
	if err != nil || len(got) != 0 || len(f.records) != 1 {
		t.Fatalf("got %v %v", got, err)
	}
}

func TestUpdaterLifecycleStaleRecordOccupiedPortFailsClosed(t *testing.T) {
	f := newComposedFixture(t, state.PoolCodex)
	record := f.records[cliproxyconfig.Codex]
	f.manager.updaterStatus = func(id cliproxyconfig.ID) (Status, error) { return Status{ID: id, Record: record}, ErrNotRunning }
	f.manager.updaterPortOccupied = func(int) (bool, error) { return true, nil }
	if _, err := f.lifecycle.CaptureRunning(context.Background()); !errors.Is(err, ErrUnverifiable) {
		t.Fatalf("got %v", err)
	}
}

func TestUpdaterLifecycleCaptureRunningRejectsUnverifiableIdentity(t *testing.T) {
	for _, failure := range []error{ErrIdentityMismatch, ErrUnverifiable, ErrUnsafeInstance, installedslot.ErrSlotUnknown} {
		f := newComposedFixture(t, state.PoolCodex)
		f.manager.updaterStatus = func(cliproxyconfig.ID) (Status, error) { return Status{}, failure }
		if _, err := f.lifecycle.CaptureRunning(context.Background()); err == nil {
			t.Fatalf("accepted %v", failure)
		}
	}
}

func TestComposedPromotionSelectsCandidateThroughManager(t *testing.T) {
	f := newComposedFixture(t, state.PoolCodex, state.PoolGoogle)
	if err := f.updater(nil).Promote(context.Background(), "vB"); err != nil {
		t.Fatal(err)
	}
	if f.active() != "vB" || f.markerExists() {
		t.Fatalf("active=%s marker=%v", f.active(), f.markerExists())
	}
	for _, id := range []cliproxyconfig.ID{cliproxyconfig.Codex, cliproxyconfig.Google} {
		if f.records[id].UpstreamVersion != "vB" {
			t.Fatalf("%s record=%#v", id, f.records[id])
		}
	}
	if !reflect.DeepEqual(f.smoke.calls, []string{"disposable:vB", "production:vB"}) {
		t.Fatalf("smoke=%v", f.smoke.calls)
	}
	if !reflect.DeepEqual(f.starts, []string{"codex:vB", "google:vB"}) {
		t.Fatalf("starts=%v", f.starts)
	}
	if _, err := f.registry.Resolve("vA"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.registry.Resolve("vB"); err != nil {
		t.Fatal(err)
	}
}

func TestComposedPromotionSmokeFailureRollsBackPreviousSlot(t *testing.T) {
	f := newComposedFixture(t, state.PoolCodex, state.PoolGoogle)
	f.smoke.failVersion = "vB"
	err := f.updater(nil).Promote(context.Background(), "vB")
	if !errors.Is(err, update.ErrPromotionRolledBack) {
		t.Fatalf("got %v", err)
	}
	if f.active() != "vA" || f.markerExists() {
		t.Fatalf("active=%s marker=%v", f.active(), f.markerExists())
	}
	for _, id := range []cliproxyconfig.ID{cliproxyconfig.Codex, cliproxyconfig.Google} {
		if f.records[id].UpstreamVersion != "vA" {
			t.Fatalf("%s record=%#v", id, f.records[id])
		}
	}
	if !containsString(f.stops, "codex:vB") || !containsString(f.stops, "google:vB") {
		t.Fatalf("stops=%v", f.stops)
	}
}

func TestComposedPromotionPartialStopFailsClosed(t *testing.T) {
	f := newComposedFixture(t, state.PoolCodex, state.PoolGoogle)
	f.failStopCall = 2
	err := f.updater(nil).Promote(context.Background(), "vB")
	if !errors.Is(err, update.ErrPromotionRolledBack) || f.markerExists() || f.active() != "vA" {
		t.Fatalf("err=%v marker=%v active=%s", err, f.markerExists(), f.active())
	}
	for _, id := range []cliproxyconfig.ID{cliproxyconfig.Codex, cliproxyconfig.Google} {
		if f.records[id].UpstreamVersion != "vA" {
			t.Fatalf("%s not safely restored: %#v", id, f.records[id])
		}
	}
}

func TestComposedCandidatePartialStartRollback(t *testing.T) {
	f := newComposedFixture(t, state.PoolCodex, state.PoolGoogle)
	f.failStartCall = 2
	err := f.updater(nil).Promote(context.Background(), "vB")
	if !errors.Is(err, update.ErrPromotionRolledBack) || f.markerExists() || f.active() != "vA" {
		t.Fatalf("err=%v marker=%v active=%s", err, f.markerExists(), f.active())
	}
	for _, id := range []cliproxyconfig.ID{cliproxyconfig.Codex, cliproxyconfig.Google} {
		if f.records[id].UpstreamVersion != "vA" {
			t.Fatalf("%s record=%#v", id, f.records[id])
		}
	}
}

func TestComposedRecoveryAfterMarkerPublishDoesNotDuplicateProcesses(t *testing.T) {
	f := newComposedFixture(t, state.PoolCodex, state.PoolGoogle)
	initial := map[cliproxyconfig.ID]ProcessRecord{cliproxyconfig.Codex: f.records[cliproxyconfig.Codex], cliproxyconfig.Google: f.records[cliproxyconfig.Google]}
	err := f.updater(func(point update.FaultPoint) error {
		if point == update.AfterMarkerPublish {
			return update.ErrInjectedCrash
		}
		return nil
	}).Promote(context.Background(), "vB")
	if !errors.Is(err, update.ErrInjectedCrash) || !f.markerExists() {
		t.Fatalf("err=%v marker=%v", err, f.markerExists())
	}
	if err = f.updater(nil).Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.starts) != 0 || f.markerExists() || !reflect.DeepEqual(initial, f.records) {
		t.Fatalf("starts=%v records=%v", f.starts, f.records)
	}
}

func TestComposedRecoveryAfterActiveSaveRestoresPrevious(t *testing.T) {
	f := newComposedFixture(t, state.PoolCodex, state.PoolGoogle)
	err := f.updater(func(point update.FaultPoint) error {
		if point == update.AfterActiveSave {
			return update.ErrInjectedCrash
		}
		return nil
	}).Promote(context.Background(), "vB")
	if !errors.Is(err, update.ErrInjectedCrash) || f.active() != "vB" || !f.markerExists() {
		t.Fatalf("err=%v active=%s marker=%v", err, f.active(), f.markerExists())
	}
	if err = f.updater(nil).Recover(context.Background()); !errors.Is(err, update.ErrPromotionRolledBack) {
		t.Fatal(err)
	}
	if f.active() != "vA" || f.markerExists() {
		t.Fatalf("active=%s marker=%v", f.active(), f.markerExists())
	}
	for _, record := range f.records {
		if record.UpstreamVersion != "vA" {
			t.Fatalf("record=%#v", record)
		}
	}
}

func TestComposedRecoveryRejectsPartialRunningSet(t *testing.T) {
	f := newComposedFixture(t, state.PoolCodex, state.PoolGoogle)
	err := f.updater(func(point update.FaultPoint) error {
		if point == update.AfterMarkerPublish {
			return update.ErrInjectedCrash
		}
		return nil
	}).Promote(context.Background(), "vB")
	if !errors.Is(err, update.ErrInjectedCrash) {
		t.Fatal(err)
	}
	delete(f.records, cliproxyconfig.Google)
	starts, stops := len(f.starts), len(f.stops)
	err = f.updater(nil).Recover(context.Background())
	if !errors.Is(err, update.ErrRecoveryUnresolved) || !f.markerExists() || len(f.starts) != starts || len(f.stops) != stops || f.active() != "vA" {
		t.Fatalf("err=%v", err)
	}
}

func TestComposedStateChangeDoesNotRelabelRunningProcess(t *testing.T) {
	f := newComposedFixture(t, state.PoolCodex)
	before := f.records[cliproxyconfig.Codex]
	if err := f.store.SaveState(composedState("vB")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.lifecycle.CaptureRunning(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := f.lifecycle.Stop(context.Background(), []state.Pool{state.PoolCodex}); err != nil {
		t.Fatal(err)
	}
	if before.UpstreamVersion != "vA" || !containsString(f.stops, "codex:vA") {
		t.Fatalf("stops=%v", f.stops)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
