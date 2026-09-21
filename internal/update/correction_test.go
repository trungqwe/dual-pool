package update

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/trungqwe/dual-pool/internal/lockfile"
	"github.com/trungqwe/dual-pool/internal/state"
	"github.com/trungqwe/dual-pool/internal/winacl"
)

type recordingVerifier struct {
	calls []string
	fail  string
}

func (v *recordingVerifier) VerifyInstalled(_ context.Context, version string) error {
	v.calls = append(v.calls, version)
	if version == v.fail {
		return errors.New("verify failed")
	}
	return nil
}

func TestP2UPDPreviousVerify001(t *testing.T) {
	fs := &fakeState{v: testState("7.3.7")}
	life := &fakeLife{}
	smoke := &fakeSmoke{}
	u := testUpdater(t, fs, life, smoke, nil)
	v := &recordingVerifier{fail: "7.3.7"}
	u.verifier = v
	if err := u.Promote(context.Background(), "7.3.8"); err == nil {
		t.Fatal("accepted invalid previous slot")
	}
	if len(v.calls) != 1 || v.calls[0] != "7.3.7" || len(smoke.calls) != 0 || life.captures != 0 || fs.saves != 0 {
		t.Fatalf("side effects: %#v", v.calls)
	}
}

func TestP2UPDRollbackStop001(t *testing.T) {
	fs := &fakeState{v: testState("7.3.7")}
	life := &fakeLife{pools: []state.Pool{state.PoolCodex}, failStopCall: 2}
	smoke := &fakeSmoke{production: errors.New("candidate"), failVersion: "7.3.8"}
	u := testUpdater(t, fs, life, smoke, nil)
	if err := u.Promote(context.Background(), "7.3.8"); !errors.Is(err, ErrRollbackUnresolved) {
		t.Fatalf("got %v", err)
	}
	if fs.v.ActiveUpstreamVersion != "7.3.8" || life.starts != 1 {
		t.Fatal("published split-brain rollback")
	}
	if _, ok, _ := u.markers.load(); !ok {
		t.Fatal("marker removed")
	}
}

type convergingLife struct {
	running        []state.Pool
	duplicateStart bool
}

func (l *convergingLife) CaptureRunning(context.Context) ([]state.Pool, error) {
	return append([]state.Pool(nil), l.running...), nil
}
func (l *convergingLife) Stop(_ context.Context, want []state.Pool) error {
	if !reflect.DeepEqual(l.running, want) {
		return errors.New("unexpected stop set")
	}
	l.running = nil
	return nil
}
func (l *convergingLife) Start(_ context.Context, want []state.Pool) error {
	if len(l.running) != 0 {
		l.duplicateStart = true
		return errors.New("already running")
	}
	l.running = append([]state.Pool(nil), want...)
	return nil
}

func TestP2UPDAfterMarkerPublishRecoveryDoesNotDuplicateRunningPools(t *testing.T) {
	fs := &fakeState{v: testState("7.3.7")}
	life := &convergingLife{running: []state.Pool{state.PoolCodex, state.PoolGoogle}}
	u := testUpdater(t, fs, &fakeLife{}, &fakeSmoke{}, nil)
	u.lifecycle = life
	u.fault = func(point FaultPoint) error {
		if point == AfterMarkerPublish {
			return ErrInjectedCrash
		}
		return nil
	}
	if err := u.Promote(context.Background(), "7.3.8"); !errors.Is(err, ErrInjectedCrash) {
		t.Fatalf("expected marker-publish crash: %v", err)
	}
	u.fault = nil
	if err := u.Recover(context.Background()); err != nil {
		t.Fatalf("recovery failed: %v", err)
	}
	if life.duplicateStart || !reflect.DeepEqual(life.running, []state.Pool{state.PoolCodex, state.PoolGoogle}) {
		t.Fatalf("running set changed or duplicated: %#v", life.running)
	}
	if _, exists, err := u.markers.load(); err != nil || exists {
		t.Fatalf("marker after resolved recovery: exists=%v err=%v", exists, err)
	}
}

func TestP2UPDRecoveryRetainsMarkerForPartialRunningSet(t *testing.T) {
	fs := &fakeState{v: testState("7.3.7")}
	life := &convergingLife{running: []state.Pool{state.PoolCodex}}
	u := testUpdater(t, fs, &fakeLife{}, &fakeSmoke{}, nil)
	u.lifecycle = life
	m := transactionMarker{SchemaVersion: 1, PreviousVersion: "7.3.7", CandidateVersion: "7.3.8", BaseStateSHA256: stateFingerprint(fs.v), RestartPools: []state.Pool{state.PoolCodex, state.PoolGoogle}}
	if _, err := u.markers.publish(m); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(u.Recover(context.Background()), ErrRecoveryUnresolved) {
		t.Fatal("partial set was accepted")
	}
	if life.duplicateStart || !reflect.DeepEqual(life.running, []state.Pool{state.PoolCodex}) {
		t.Fatal("partial state was mutated")
	}
	if _, exists, err := u.markers.load(); err != nil || !exists {
		t.Fatalf("marker lost: exists=%v err=%v", exists, err)
	}
}

func TestP2UPDPendingMarker001(t *testing.T) {
	fs := &fakeState{v: testState("7.3.7")}
	life := &fakeLife{}
	smoke := &fakeSmoke{}
	u := testUpdater(t, fs, life, smoke, nil)
	m := transactionMarker{SchemaVersion: 1, PreviousVersion: "7.3.7", CandidateVersion: "7.3.8", BaseStateSHA256: stateFingerprint(fs.v), RestartPools: []state.Pool{}}
	if _, err := u.markers.publish(m); err != nil {
		t.Fatal(err)
	}
	beforeMarker, err := os.ReadFile(u.markers.path())
	if err != nil {
		t.Fatal(err)
	}
	v := &recordingVerifier{}
	u.verifier = v
	if err := u.Promote(context.Background(), "7.3.9"); !errors.Is(err, ErrUpdatePending) {
		t.Fatalf("got %v", err)
	}
	if len(v.calls) != 0 || len(smoke.calls) != 0 || life.captures != 0 || fs.saves != 0 {
		t.Fatal("pending marker allowed work")
	}
	afterMarker, err := os.ReadFile(u.markers.path())
	if err != nil || !bytes.Equal(beforeMarker, afterMarker) {
		t.Fatal("pending marker changed")
	}
}

func TestP2UPDLogicalVersion001(t *testing.T) {
	bad := []string{"", `C:\x`, "C:/x", "../x", "x:y", "hello world", ".x", "-x", "+x", "x\n", strings.Repeat("a", 65)}
	for _, candidate := range bad {
		if validLogicalVersion(candidate) {
			t.Fatalf("accepted %q", candidate)
		}
		fs := &fakeState{v: testState("7.3.7")}
		life := &fakeLife{}
		smoke := &fakeSmoke{}
		u := testUpdater(t, fs, life, smoke, nil)
		v := &recordingVerifier{}
		u.verifier = v
		if err := u.Promote(context.Background(), candidate); !errors.Is(err, ErrCandidateInvalid) {
			t.Fatalf("%q: %v", candidate, err)
		}
		if len(v.calls) != 0 || len(smoke.calls) != 0 || life.captures != 0 || fs.saves != 0 {
			t.Fatalf("invalid %q caused side effects", candidate)
		}
	}
	fs := &fakeState{v: testState(`C:\bad`)}
	u := testUpdater(t, fs, &fakeLife{}, &fakeSmoke{}, nil)
	v := &recordingVerifier{}
	u.verifier = v
	if err := u.Promote(context.Background(), "7.3.8"); !errors.Is(err, ErrCandidateInvalid) {
		t.Fatal(err)
	}
	if len(v.calls) != 0 {
		t.Fatal("invalid previous verified")
	}
}

func TestP2UPDMarkerExactSchema001(t *testing.T) {
	valid := []byte(`{"schema_version":1,"transaction_id":"00112233445566778899aabbccddeeff","previous_version":"7.3.7","candidate_version":"7.3.8","base_state_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","restart_pools":[]}`)
	cases := [][]byte{[]byte(`{"schema_version":1,"transaction_id":"00112233445566778899aabbccddeeff","previous_version":"7.3.7","candidate_version":"7.3.8","base_state_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","x":true}`), []byte(`{"schema_version":1,"schema_version":1}`), append(append([]byte{}, valid...), []byte(` {}`)...), append(append([]byte{}, valid...), []byte(` x`)...), []byte("null"), []byte("[]"), {0xff}, []byte(`{"schema_version":1`), bytes.Repeat([]byte(" "), maxMarkerBytes+1), bytes.Replace(valid, []byte("00112233445566778899aabbccddeeff"), []byte("00112233445566778899AABBCCDDEEFF"), 1), bytes.Replace(valid, []byte(strings.Repeat("a", 64)), []byte(strings.Repeat("A", 64)), 1), bytes.Replace(valid, []byte(`"candidate_version":"7.3.8"`), []byte(`"candidate_version":"7.3.7"`), 1), bytes.Replace(valid, []byte(`"restart_pools":[]`), []byte(`"restart_pools":["other"]`), 1), bytes.Replace(valid, []byte(`"restart_pools":[]`), []byte(`"restart_pools":["codex","codex"]`), 1)}
	for i, b := range cases {
		if _, err := decodeMarker(b); err == nil {
			t.Fatalf("case %d accepted", i)
		}
	}
	if _, err := decodeMarker(append(valid, []byte(" \r\n\t")...)); err != nil {
		t.Fatalf("trailing whitespace rejected: %v", err)
	}
}

func TestP2UPDRunningSet001(t *testing.T) {
	for _, pools := range [][]state.Pool{{state.PoolCodex, state.PoolCodex}, {"other"}} {
		fs := &fakeState{v: testState("7.3.7")}
		life := &fakeLife{pools: pools}
		u := testUpdater(t, fs, life, &fakeSmoke{}, nil)
		if err := u.Promote(context.Background(), "7.3.8"); !errors.Is(err, ErrCandidateInvalid) {
			t.Fatalf("accepted %#v: %v", pools, err)
		}
		if _, ok, _ := u.markers.load(); ok {
			t.Fatal("marker published")
		}
	}
}

func TestP2UPDStateRecoveryCompose001(t *testing.T) {
	for _, point := range []state.FaultPoint{state.BeforeCAS, state.AfterReplace} {
		t.Run(string(point), func(t *testing.T) {
			root := t.TempDir()
			stateDir := filepath.Join(root, "state")
			lockDir := filepath.Join(root, "locks")
			if err := os.Mkdir(stateDir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(lockDir, 0700); err != nil {
				t.Fatal(err)
			}
			locks, err := lockfile.NewManager(lockDir)
			if err != nil {
				t.Fatal(err)
			}
			baseStore, err := state.NewStore(stateDir, state.WithLockManager(locks))
			if err != nil {
				t.Fatal(err)
			}
			initial := testState("7.3.7")
			initial.Codex.AdapterVersion = "adapter-v1"
			if err = baseStore.SaveState(initial); err != nil {
				t.Fatal(err)
			}
			injected := false
			store, err := state.NewStore(stateDir, state.WithLockManager(locks), state.WithFaultInjector(func(got state.FaultPoint) error {
				if !injected && got == point {
					injected = true
					return state.ErrInjectedCrash
				}
				return nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			markerDir := filepath.Join(root, "markers")
			acl, err := winacl.New()
			if err != nil {
				t.Fatal(err)
			}
			if err = acl.Create(markerDir); err != nil {
				t.Fatal(err)
			}
			security, err := NewWindowsMarkerSecurity()
			if err != nil {
				t.Fatal(err)
			}
			u, err := New(Config{Locks: locks, State: store, Verifier: fakeVerifier{}, Lifecycle: &fakeLife{}, Smoke: &fakeSmoke{}, MarkerDir: markerDir, MarkerSecurity: security})
			if err != nil {
				t.Fatal(err)
			}
			err = u.Promote(context.Background(), "7.3.8")
			if !errors.Is(err, ErrPromotionRolledBack) {
				t.Fatalf("got %v", err)
			}
			got, err := store.LoadState()
			if err != nil {
				t.Fatal(err)
			}
			if got.ActiveUpstreamVersion != "7.3.7" || got.Codex.AdapterVersion != "adapter-v1" {
				t.Fatal("state not restored")
			}
			if _, ok, err := u.markers.load(); err != nil || ok {
				t.Fatal("update marker remains")
			}
			if _, err = os.Stat(filepath.Join(stateDir, ".state.json.recovery")); !os.IsNotExist(err) {
				t.Fatal("state marker remains")
			}
		})
	}
}

func TestP2UPDMarkerACL001(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "markers")
	acl, err := winacl.New()
	if err != nil {
		t.Fatal(err)
	}
	if err = acl.Create(dir); err != nil {
		t.Fatal(err)
	}
	security, err := NewWindowsMarkerSecurity()
	if err != nil {
		t.Fatal(err)
	}
	store, err := newMarkerStore(dir, func() (string, error) { return "00112233445566778899aabbccddeeff", nil }, security)
	if err != nil {
		t.Fatal(err)
	}
	m := transactionMarker{SchemaVersion: 1, PreviousVersion: "7.3.7", CandidateVersion: "7.3.8", BaseStateSHA256: strings.Repeat("a", 64), RestartPools: []state.Pool{}}
	committed, err := store.publish(m)
	if err != nil {
		t.Fatal(err)
	}
	if err = acl.InspectFile(store.path()); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := store.load(); err != nil || !ok {
		t.Fatal("protected marker unreadable")
	}
	if err = store.remove(committed); err != nil {
		t.Fatal(err)
	}
}

type recordingSecurity struct{ dirs, creates, files int }

func (s *recordingSecurity) InspectDir(string) error { s.dirs++; return nil }
func (s *recordingSecurity) CreateFile(path string) (*os.File, error) {
	s.creates++
	return testMarkerSecurity{}.CreateFile(path)
}
func (s *recordingSecurity) InspectHandle(file *os.File) error {
	s.files++
	return testMarkerSecurity{}.InspectHandle(file)
}

func TestMarkerSecurityGuardCoversCreateLoadRemove(t *testing.T) {
	guard := &recordingSecurity{}
	store, err := newMarkerStore(t.TempDir(), func() (string, error) { return "00112233445566778899aabbccddeeff", nil }, guard)
	if err != nil {
		t.Fatal(err)
	}
	m := transactionMarker{SchemaVersion: 1, PreviousVersion: "7.3.7", CandidateVersion: "7.3.8", BaseStateSHA256: strings.Repeat("a", 64), RestartPools: []state.Pool{}}
	committed, err := store.publish(m)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, err := store.load(); err != nil || !ok {
		t.Fatal(err)
	}
	if err = store.remove(committed); err != nil {
		t.Fatal(err)
	}
	if guard.creates != 1 || guard.dirs < 3 || guard.files < 3 {
		t.Fatalf("guard calls: %#v", guard)
	}
}

func TestP2UPDMarkerHandleBlocksPathReplacementUntilRelease(t *testing.T) {
	store, err := newMarkerStore(t.TempDir(), func() (string, error) { return "00112233445566778899aabbccddeeff", nil }, testMarkerSecurity{})
	if err != nil {
		t.Fatal(err)
	}
	m := transactionMarker{SchemaVersion: 1, PreviousVersion: "7.3.7", CandidateVersion: "7.3.8", BaseStateSHA256: strings.Repeat("a", 64), RestartPools: []state.Pool{}}
	if _, err = store.publish(m); err != nil {
		t.Fatal(err)
	}
	file, err := openMarker(store.path())
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	replacement := filepath.Join(store.dir, "replacement")
	if err = os.WriteFile(replacement, []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(replacement, store.path()); err == nil {
		t.Fatal("pathname replacement succeeded while the inspected marker handle was exclusive")
	}
	if _, err = readMarkerHandle(file); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateFaultMatrixRetainsUnresolvedMarker(t *testing.T) {
	forward := []FaultPoint{AfterMarkerPublish, AfterProductionStop, AfterCandidateStart, AfterCandidateSmoke, BeforeMarkerCleanup}
	for _, point := range forward {
		t.Run(string(point), func(t *testing.T) {
			fs := &fakeState{v: testState("7.3.7")}
			u := testUpdater(t, fs, &fakeLife{}, &fakeSmoke{}, func(got FaultPoint) error {
				if got == point {
					return errors.New("fault")
				}
				return nil
			})
			err := u.Promote(context.Background(), "7.3.8")
			if err == nil {
				t.Fatal("false success")
			}
			_, ok, _ := u.markers.load()
			if point == AfterMarkerPublish {
				if !ok {
					t.Fatal("crash boundary marker removed")
				}
			} else if ok || !errors.Is(err, ErrPromotionRolledBack) {
				t.Fatal("healthy rollback not completed")
			}
		})
	}
	rollback := []FaultPoint{BeforeRollbackRestore, AfterRollbackRestore, BeforeRollbackCleanup}
	for _, point := range rollback {
		t.Run(string(point), func(t *testing.T) {
			fs := &fakeState{v: testState("7.3.7")}
			smoke := &fakeSmoke{production: errors.New("candidate"), failVersion: "7.3.8"}
			u := testUpdater(t, fs, &fakeLife{}, smoke, func(got FaultPoint) error {
				if got == point {
					return errors.New("fault")
				}
				return nil
			})
			if err := u.Promote(context.Background(), "7.3.8"); err == nil {
				t.Fatal("false success")
			}
			if _, ok, _ := u.markers.load(); !ok {
				t.Fatal("marker removed")
			}
		})
	}
}

type previousSmokeFails struct{}

func (previousSmokeFails) Disposable(context.Context, string) error { return nil }
func (previousSmokeFails) Production(_ context.Context, version string) error {
	if version == "7.3.8" {
		return errors.New("candidate")
	}
	return errors.New("previous")
}

func TestPreviousStartAndSmokeFailuresRetainMarker(t *testing.T) {
	t.Run("start", func(t *testing.T) {
		fs := &fakeState{v: testState("7.3.7")}
		life := &fakeLife{failStartCall: 2}
		smoke := &fakeSmoke{production: errors.New("candidate"), failVersion: "7.3.8"}
		u := testUpdater(t, fs, life, smoke, nil)
		if !errors.Is(u.Promote(context.Background(), "7.3.8"), ErrRollbackUnresolved) {
			t.Fatal("wrong result")
		}
		if _, ok, _ := u.markers.load(); !ok {
			t.Fatal("marker removed")
		}
	})
	t.Run("smoke", func(t *testing.T) {
		fs := &fakeState{v: testState("7.3.7")}
		u := testUpdater(t, fs, &fakeLife{}, &fakeSmoke{}, nil)
		u.smoke = previousSmokeFails{}
		if !errors.Is(u.Promote(context.Background(), "7.3.8"), ErrRollbackUnresolved) {
			t.Fatal("wrong result")
		}
		if _, ok, _ := u.markers.load(); !ok {
			t.Fatal("marker removed")
		}
	})
}
