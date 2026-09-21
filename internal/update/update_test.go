package update

import (
	"context"
	"errors"
	"github.com/trungqwe/dual-pool/internal/lockfile"
	"github.com/trungqwe/dual-pool/internal/state"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

type fakeState struct {
	v          state.State
	recoverErr error
	saves      int
}

type testMarkerSecurity struct{}

func (testMarkerSecurity) InspectDir(string) error { return nil }
func (testMarkerSecurity) CreateFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
}
func (testMarkerSecurity) InspectFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || reparse(path) {
		return ErrRecoveryUnresolved
	}
	return nil
}

func (f *fakeState) LoadState() (state.State, error) { return f.v, nil }
func (f *fakeState) SaveState(v state.State) error   { f.saves++; f.v = v; return nil }
func (f *fakeState) Recover() error                  { return f.recoverErr }

type fakeVerifier struct{ err error }

func (f fakeVerifier) VerifyInstalled(context.Context, string) error { return f.err }

type fakeLife struct {
	pools             []state.Pool
	stopErr, startErr error
	stops, starts     int
	captures          int
	failStopCall      int
	failStartCall     int
}

func (f *fakeLife) CaptureRunning(context.Context) ([]state.Pool, error) {
	f.captures++
	return f.pools, nil
}
func (f *fakeLife) Stop(context.Context, []state.Pool) error {
	f.stops++
	if f.failStopCall == f.stops {
		return errors.New("stop failed")
	}
	return f.stopErr
}
func (f *fakeLife) Start(context.Context, []state.Pool) error {
	f.starts++
	if f.failStartCall == f.starts {
		return errors.New("start failed")
	}
	return f.startErr
}

type fakeSmoke struct {
	disposable, production error
	failVersion            string
	calls                  []string
}

func (f *fakeSmoke) Disposable(_ context.Context, v string) error {
	f.calls = append(f.calls, "d:"+v)
	return f.disposable
}
func (f *fakeSmoke) Production(_ context.Context, v string) error {
	f.calls = append(f.calls, "p:"+v)
	if v == f.failVersion {
		return f.production
	}
	return nil
}
func testState(ver string) state.State {
	return state.State{SchemaVersion: 1, InstallID: "install_fixture_01", ActiveUpstreamVersion: ver, Instances: state.Instances{Codex: state.Instance{Port: 8317, Status: state.InstanceStopped}, Google: state.Instance{Port: 8318, Status: state.InstanceStopped}}, Antigravity: state.Antigravity{Mode: state.ModeDisabled, BridgePort: 51074}, Accounts: []state.Account{}}
}
func testUpdater(t *testing.T, fs *fakeState, life *fakeLife, smoke *fakeSmoke, fault func(FaultPoint) error) *Updater {
	t.Helper()
	root := t.TempDir()
	lockdir := filepath.Join(root, "locks")
	if err := os.Mkdir(lockdir, 0700); err != nil {
		t.Fatal(err)
	}
	locks, err := lockfile.NewManager(lockdir)
	if err != nil {
		t.Fatal(err)
	}
	u, err := New(Config{Locks: locks, State: fs, Verifier: fakeVerifier{}, Lifecycle: life, Smoke: smoke, MarkerDir: root, MarkerSecurity: testMarkerSecurity{}, TransactionID: func() (string, error) { return "00112233445566778899aabbccddeeff", nil }, Fault: fault})
	if err != nil {
		t.Fatal(err)
	}
	return u
}
func TestPromotionPreSmokeLeavesStateUntouched(t *testing.T) {
	fs := &fakeState{v: testState("7.3.7")}
	life := &fakeLife{pools: []state.Pool{state.PoolCodex}}
	smoke := &fakeSmoke{disposable: errors.New("schema")}
	u := testUpdater(t, fs, life, smoke, nil)
	if err := u.Promote(context.Background(), "7.3.8"); err == nil || fs.v.ActiveUpstreamVersion != "7.3.7" || life.stops != 0 {
		t.Fatal("pre-smoke promoted")
	}
	if _, ok, err := u.markers.load(); err != nil || ok {
		t.Fatal("marker published")
	}
}
func TestPromotionFailureRollsBack(t *testing.T) {
	fs := &fakeState{v: testState("7.3.7")}
	life := &fakeLife{pools: []state.Pool{state.PoolCodex, state.PoolGoogle}}
	smoke := &fakeSmoke{production: errors.New("candidate smoke"), failVersion: "7.3.8"}
	u := testUpdater(t, fs, life, smoke, nil)
	err := u.Promote(context.Background(), "7.3.8")
	if !errors.Is(err, ErrPromotionRolledBack) || fs.v.ActiveUpstreamVersion != "7.3.7" || life.stops < 2 || life.starts < 2 {
		t.Fatalf("rollback failed: %v", err)
	}
	if _, ok, _ := u.markers.load(); ok {
		t.Fatal("marker retained after healthy rollback")
	}
}
func TestRecoveryCandidateRollsBack(t *testing.T) {
	fs := &fakeState{v: testState("7.3.8")}
	life := &fakeLife{pools: []state.Pool{state.PoolCodex}}
	smoke := &fakeSmoke{}
	u := testUpdater(t, fs, life, smoke, nil)
	m := transactionMarker{SchemaVersion: 1, PreviousVersion: "7.3.7", CandidateVersion: "7.3.8", BaseStateSHA256: stateFingerprint(fs.v), RestartPools: life.pools}
	m.BaseStateSHA256 = stateFingerprint(testState("7.3.7"))
	if _, err := u.markers.publish(m); err != nil {
		t.Fatal(err)
	}
	err := u.Recover(context.Background())
	if !errors.Is(err, ErrPromotionRolledBack) || fs.v.ActiveUpstreamVersion != "7.3.7" {
		t.Fatalf("recovery: %v", err)
	}
}
func TestMarkerStrictRejectsDuplicateAndTrailing(t *testing.T) {
	m := transactionMarker{SchemaVersion: 1, TransactionID: "00112233445566778899aabbccddeeff", PreviousVersion: "7.3.7", CandidateVersion: "7.3.8", BaseStateSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	b, _ := encodeMarker(m)
	for _, bad := range [][]byte{append(b, []byte("{}")...), append(b, []byte("x")...), []byte(`{"schema_version":1,"schema_version":1}`)} {
		if _, err := decodeMarker(bad); err == nil {
			t.Fatal("accepted malformed marker")
		}
	}
}

type diskState struct{ path string }

func (d diskState) LoadState() (state.State, error) {
	b, e := os.ReadFile(d.path)
	if e != nil {
		return state.State{}, e
	}
	return state.DecodeState(b)
}
func (d diskState) SaveState(v state.State) error {
	b, e := state.EncodeState(v)
	if e != nil {
		return e
	}
	return os.WriteFile(d.path, b, 0600)
}
func (d diskState) Recover() error                { return nil }
func cloneExceptActive(s state.State) state.State { s.ActiveUpstreamVersion = ""; return s }
func TestFaultBoundariesLeaveRecoverableMarker(t *testing.T) {
	for _, point := range []FaultPoint{AfterMarkerPublish, AfterActiveSave} {
		t.Run(string(point), func(t *testing.T) {
			fs := &fakeState{v: testState("7.3.7")}
			life := &fakeLife{pools: []state.Pool{state.PoolCodex}}
			u := testUpdater(t, fs, life, &fakeSmoke{}, func(p FaultPoint) error {
				if p == point {
					return ErrInjectedCrash
				}
				return nil
			})
			if !errors.Is(u.Promote(context.Background(), "7.3.8"), ErrInjectedCrash) {
				t.Fatal("fault did not stop")
			}
			if _, ok, _ := u.markers.load(); !ok {
				t.Fatal("marker lost")
			}
			u.fault = nil
			err := u.Recover(context.Background())
			if point == AfterMarkerPublish {
				if err != nil {
					t.Fatal(err)
				}
			} else if !errors.Is(err, ErrPromotionRolledBack) {
				t.Fatal(err)
			}
			if fs.v.ActiveUpstreamVersion != "7.3.7" {
				t.Fatal("prior selection not restored")
			}
			if _, ok, _ := u.markers.load(); ok {
				t.Fatal("marker remains after recovery")
			}
		})
	}
}
func TestRollbackUnresolvedRetainsMarker(t *testing.T) {
	fs := &fakeState{v: testState("7.3.7")}
	life := &fakeLife{pools: []state.Pool{state.PoolCodex}, startErr: errors.New("restart")}
	smoke := &fakeSmoke{production: errors.New("candidate"), failVersion: "7.3.8"}
	u := testUpdater(t, fs, life, smoke, nil)
	if !errors.Is(u.Promote(context.Background(), "7.3.8"), ErrRollbackUnresolved) {
		t.Fatal("false success")
	}
	if _, ok, _ := u.markers.load(); !ok {
		t.Fatal("marker removed")
	}
}
func TestPromotionPreservesUnrelatedState(t *testing.T) {
	fs := &fakeState{v: testState("7.3.7")}
	fs.v.Codex.AdapterVersion = "adapter-v1"
	before := cloneExceptActive(fs.v)
	life := &fakeLife{}
	u := testUpdater(t, fs, life, &fakeSmoke{}, nil)
	if err := u.Promote(context.Background(), "7.3.8"); err != nil {
		t.Fatal(err)
	}
	if fs.v.ActiveUpstreamVersion != "7.3.8" || !reflect.DeepEqual(before, cloneExceptActive(fs.v)) {
		t.Fatal("unrelated state changed")
	}
}
func TestGlobalBlocksConcurrentPromotion(t *testing.T) {
	fs := &fakeState{v: testState("7.3.7")}
	entered := make(chan struct{})
	release := make(chan struct{})
	life := &fakeLife{}
	u := testUpdater(t, fs, life, &fakeSmoke{}, func(p FaultPoint) error {
		if p == AfterMarkerPublish {
			close(entered)
			<-release
		}
		return nil
	})
	done := make(chan error, 1)
	go func() { done <- u.Promote(context.Background(), "7.3.8") }()
	<-entered
	if err := u.Recover(context.Background()); !errors.Is(err, lockfile.ErrLockHeld) {
		close(release)
		t.Fatalf("recovery entered: %v", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
func TestMarkerArtifactSafety(t *testing.T) {
	root := t.TempDir()
	s, err := newMarkerStore(root, nil, testMarkerSecurity{})
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "outside")
	if err = os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	if output, linkErr := exec.Command("cmd", "/c", "mklink", "/J", s.path(), target).CombinedOutput(); linkErr != nil {
		t.Fatalf("mandatory reparse fixture unavailable: %v: %s", linkErr, output)
	}
	defer os.Remove(s.path())
	if _, _, err = s.load(); !errors.Is(err, ErrRecoveryUnresolved) {
		t.Fatal("reparse accepted")
	}
	dirLink := filepath.Join(root, "marker-link")
	if output, linkErr := exec.Command("cmd", "/c", "mklink", "/J", dirLink, target).CombinedOutput(); linkErr != nil {
		t.Fatalf("directory reparse unavailable: %v: %s", linkErr, output)
	}
	defer os.Remove(dirLink)
	if _, err = newMarkerStore(dirLink, nil, testMarkerSecurity{}); !errors.Is(err, ErrRecoveryUnresolved) {
		t.Fatal("reparse directory accepted")
	}
}

func TestSubprocessCrashRecovery(t *testing.T) {
	if point := os.Getenv("UPDATE_CRASH_POINT"); point != "" {
		root := os.Getenv("UPDATE_CRASH_ROOT")
		lockdir := filepath.Join(root, "locks")
		locks, err := lockfile.NewManager(lockdir)
		if err != nil {
			t.Fatal(err)
		}
		u, err := New(Config{Locks: locks, State: diskState{filepath.Join(root, "state.json")}, Verifier: fakeVerifier{}, Lifecycle: &fakeLife{pools: []state.Pool{state.PoolCodex}}, Smoke: &fakeSmoke{}, MarkerDir: root, MarkerSecurity: testMarkerSecurity{}, Fault: func(p FaultPoint) error {
			if string(p) == point {
				os.Exit(42)
			}
			return nil
		}})
		if err != nil {
			t.Fatal(err)
		}
		_ = u.Promote(context.Background(), "7.3.8")
		return
	}
	for _, point := range []FaultPoint{AfterMarkerPublish, AfterActiveSave} {
		t.Run(string(point), func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, "locks"), 0700); err != nil {
				t.Fatal(err)
			}
			d := diskState{filepath.Join(root, "state.json")}
			if err := d.SaveState(testState("7.3.7")); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestSubprocessCrashRecovery$")
			cmd.Env = append(os.Environ(), "UPDATE_CRASH_POINT="+string(point), "UPDATE_CRASH_ROOT="+root)
			if err := cmd.Run(); err == nil {
				t.Fatal("crash helper exited successfully")
			}
			locks, err := lockfile.NewManager(filepath.Join(root, "locks"))
			if err != nil {
				t.Fatal(err)
			}
			u, err := New(Config{Locks: locks, State: d, Verifier: fakeVerifier{}, Lifecycle: &fakeLife{}, Smoke: &fakeSmoke{}, MarkerDir: root, MarkerSecurity: testMarkerSecurity{}})
			if err != nil {
				t.Fatal(err)
			}
			err = u.Recover(context.Background())
			if point == AfterMarkerPublish {
				if err != nil {
					t.Fatal(err)
				}
			} else if !errors.Is(err, ErrPromotionRolledBack) {
				t.Fatal(err)
			}
			got, err := d.LoadState()
			if err != nil || got.ActiveUpstreamVersion != "7.3.7" {
				t.Fatalf("state after crash: %v", err)
			}
			if _, ok, _ := u.markers.load(); ok {
				t.Fatal("marker retained")
			}
		})
	}
}
