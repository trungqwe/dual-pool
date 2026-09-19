package configtxn

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/trungqwe/dual-pool/internal/lockfile"
	"github.com/trungqwe/dual-pool/internal/state"
)

func TestFaultMatrixRecovery(t *testing.T) {
	points := []struct {
		point   FaultPoint
		applied bool
	}{
		{AfterMarkerSync, false}, {AfterPendingOwnership, false}, {BeforeTargetCAS, false},
		{AfterTargetReplace, true}, {AfterTargetVerify, true}, {AfterTargetSync, true},
		{BeforeOwnershipFinalize, true}, {AfterOwnershipFinalize, true}, {BeforeMarkerCleanup, true},
	}
	for _, tc := range points {
		t.Run(string(tc.point), func(t *testing.T) {
			f := newFixture(t)
			crash, err := NewEngine(f.journal, f.backup, f.locks, f.store, WithFaultInjector(func(p FaultPoint) error {
				if p == tc.point {
					return ErrInjectedCrash
				}
				return nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			if err = crash.Apply(f.plan()); !errors.Is(err, ErrInjectedCrash) {
				t.Fatalf("fault=%v", err)
			}
			fresh, _ := NewEngine(f.journal, f.backup, f.locks, f.store)
			if err = fresh.Recover(f.plan()); err != nil {
				t.Fatal(err)
			}
			if err = fresh.Recover(f.plan()); err != nil {
				t.Fatal(err)
			}
			got, _ := os.ReadFile(f.target)
			if bytes.Equal(got, f.original) == tc.applied {
				t.Fatalf("unexpected recovered state")
			}
		})
	}
}

func TestPreCommitFaultsAndRollbackFaultMatrix(t *testing.T) {
	for _, point := range []FaultPoint{AfterBackupSync, AfterCandidateSync} {
		f := newFixture(t)
		e, _ := NewEngine(f.journal, f.backup, f.locks, f.store, WithFaultInjector(func(p FaultPoint) error {
			if p == point {
				return ErrInjectedCrash
			}
			return nil
		}))
		if err := e.Apply(f.plan()); !errors.Is(err, ErrInjectedCrash) {
			t.Fatalf("%s=%v", point, err)
		}
		got, _ := os.ReadFile(f.target)
		if !bytes.Equal(got, f.original) {
			t.Fatalf("%s changed target", point)
		}
	}
	for _, tc := range []struct {
		point  FaultPoint
		rolled bool
	}{
		{AfterMarkerSync, false}, {BeforeTargetCAS, false}, {AfterTargetReplace, true},
		{AfterTargetVerify, true}, {AfterTargetSync, true}, {BeforeOwnershipFinalize, true},
		{AfterOwnershipFinalize, true}, {BeforeMarkerCleanup, true},
	} {
		t.Run("rollback-"+string(tc.point), func(t *testing.T) {
			f := newFixture(t)
			if err := f.engine.Apply(f.plan()); err != nil {
				t.Fatal(err)
			}
			e, _ := NewEngine(f.journal, f.backup, f.locks, f.store, WithFaultInjector(func(p FaultPoint) error {
				if p == tc.point {
					return ErrInjectedCrash
				}
				return nil
			}))
			if err := e.Rollback(f.plan()); !errors.Is(err, ErrInjectedCrash) {
				t.Fatalf("fault=%v", err)
			}
			fresh, _ := NewEngine(f.journal, f.backup, f.locks, f.store)
			if err := fresh.Recover(f.plan()); err != nil {
				t.Fatal(err)
			}
			got, _ := os.ReadFile(f.target)
			if bytes.Equal(got, f.original) != tc.rolled {
				t.Fatal("wrong rollback recovery state")
			}
		})
	}
}

func TestPerTargetLockHeldDuringCAS(t *testing.T) {
	f := newFixture(t)
	observed := false
	e, _ := NewEngine(f.journal, f.backup, f.locks, f.store, WithFaultInjector(func(p FaultPoint) error {
		if p == BeforeTargetCAS {
			_, err := f.locks.AcquireFile(f.target)
			observed = errors.Is(err, lockfile.ErrLockHeld)
		}
		return nil
	}))
	if err := e.Apply(f.plan()); err != nil {
		t.Fatal(err)
	}
	if !observed {
		t.Fatal("per-target lock was not held at CAS")
	}
}

func TestStrictMarkerAndBounds(t *testing.T) {
	m := marker{1, "apply", strings.Repeat("a", 32), strings.Repeat("b", 64), strings.Repeat("c", 64), strings.Repeat("d", 64), "config-" + strings.Repeat("a", 32) + ".bak", strings.Repeat("c", 64), []string{keyModel, keyModelProvider, keyProviderTable}}
	b, err := encodeMarker(m)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = decodeMarker(b); err != nil {
		t.Fatal(err)
	}
	duplicate := bytes.Replace(b, []byte(`"operation":"apply"`), []byte(`"operation":"apply","operation":"apply"`), 1)
	if _, err = decodeMarker(duplicate); !errors.Is(err, ErrRecoveryUnresolved) {
		t.Fatalf("duplicate=%v", err)
	}
	unknown := bytes.Replace(b, []byte(`"operation":"apply"`), []byte(`"unknown":1,"operation":"apply"`), 1)
	if _, err = decodeMarker(unknown); !errors.Is(err, ErrRecoveryUnresolved) {
		t.Fatalf("unknown=%v", err)
	}
	if _, err = decodeMarker(make([]byte, maxMarkerBytes+1)); !errors.Is(err, ErrRecoveryUnresolved) {
		t.Fatalf("oversize marker=%v", err)
	}
	if _, err = parseDocument(append([]byte("model=\"x\"\n"), make([]byte, maxConfigBytes)...)); !errors.Is(err, ErrUnsupportedConfigShape) {
		t.Fatalf("oversize config=%v", err)
	}
}

type failingReplacement struct{}

func (failingReplacement) replace(string, string) error { return ErrPersistence }
func (failingReplacement) install(string, string) error { return ErrPersistence }

func TestReplaceFailurePreservesOriginal(t *testing.T) {
	f := newFixture(t)
	e, err := NewEngine(f.journal, f.backup, f.locks, f.store, WithReplacementAPI(failingReplacement{}))
	if err != nil {
		t.Fatal(err)
	}
	if err = e.Apply(f.plan()); !errors.Is(err, ErrPersistence) {
		t.Fatalf("replace=%v", err)
	}
	got, _ := os.ReadFile(f.target)
	if !bytes.Equal(got, f.original) {
		t.Fatal("failed replacement changed target")
	}
}

func TestUnsafeArtifactsAndCollision(t *testing.T) {
	f := newFixture(t, WithTransactionIDGenerator(func() (string, error) { return strings.Repeat("a", 32), nil }))
	backup := filepath.Join(f.backup, "config-"+strings.Repeat("a", 32)+".bak")
	if err := os.WriteFile(backup, []byte("collision"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := f.engine.Apply(f.plan()); !errors.Is(err, ErrPersistence) {
		t.Fatalf("collision=%v", err)
	}
	got, _ := os.ReadFile(backup)
	if string(got) != "collision" {
		t.Fatal("backup collision overwritten")
	}
	if _, err := NewEngine("relative", f.backup, f.locks, f.store); !errors.Is(err, ErrUnsafeConfigArtifact) {
		t.Fatalf("relative=%v", err)
	}
}

func TestConfigTxnCrashHelper(t *testing.T) {
	if os.Getenv("DUALPOOL_CONFIGTXN_HELPER") != "1" {
		return
	}
	root := os.Getenv("DUALPOOL_CONFIGTXN_ROOT")
	locks, err := lockfile.NewManager(filepath.Join(root, "locks"))
	if err != nil {
		os.Exit(11)
	}
	store, err := state.NewStore(filepath.Join(root, "state"), state.WithLockManager(locks))
	if err != nil {
		os.Exit(12)
	}
	e, err := NewEngine(filepath.Join(root, "journal"), filepath.Join(root, "backup"), locks, store, WithFaultInjector(func(p FaultPoint) error {
		if string(p) == os.Getenv("DUALPOOL_CONFIGTXN_POINT") {
			os.Exit(97)
		}
		return nil
	}))
	if err != nil {
		os.Exit(13)
	}
	plan := CodexPlan{Target: filepath.Join(root, "config.toml")}
	if os.Getenv("DUALPOOL_CONFIGTXN_ACTION") == "rollback" {
		err = e.Rollback(plan)
	} else {
		err = e.Apply(plan)
	}
	if os.Getenv("DUALPOOL_CONFIGTXN_ACTION") == "contend" && errors.Is(err, lockfile.ErrLockHeld) {
		os.Exit(0)
	}
	if err != nil {
		os.Exit(14)
	}
	os.Exit(15)
}

func TestSubprocessCrashRecoveryAndGlobalContention(t *testing.T) {
	for _, tc := range []struct {
		name, action            string
		point                   FaultPoint
		setupApply, wantApplied bool
	}{
		{"apply-pre", "apply", AfterPendingOwnership, false, false},
		{"apply-post", "apply", AfterTargetReplace, false, true},
		{"rollback-post", "rollback", AfterTargetReplace, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			if tc.setupApply {
				if err := f.engine.Apply(f.plan()); err != nil {
					t.Fatal(err)
				}
			}
			runHelper(t, f.root, tc.action, tc.point, 97)
			fresh, _ := NewEngine(f.journal, f.backup, f.locks, f.store)
			if err := fresh.Recover(f.plan()); err != nil {
				t.Fatal(err)
			}
			got, _ := os.ReadFile(f.target)
			if bytes.Equal(got, f.original) == tc.wantApplied {
				t.Fatal("wrong subprocess recovery state")
			}
		})
	}
	f := newFixture(t)
	guard, err := f.locks.AcquireGlobal()
	if err != nil {
		t.Fatal(err)
	}
	runHelper(t, f.root, "contend", "", 0)
	if err = guard.Release(); err != nil {
		t.Fatal(err)
	}
	if countFiles(t, f.backup) != 0 || countFiles(t, f.journal) != 0 {
		t.Fatal("contender prepared transaction artifacts")
	}
}

func runHelper(t *testing.T, root, action string, point FaultPoint, wantExit int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestConfigTxnCrashHelper$")
	cmd.Env = append(os.Environ(), "DUALPOOL_CONFIGTXN_HELPER=1", "DUALPOOL_CONFIGTXN_ROOT="+root, "DUALPOOL_CONFIGTXN_ACTION="+action, "DUALPOOL_CONFIGTXN_POINT="+string(point))
	err := cmd.Run()
	if wantExit == 0 && err == nil {
		return
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != wantExit {
		t.Fatalf("helper exit=%v", err)
	}
}
