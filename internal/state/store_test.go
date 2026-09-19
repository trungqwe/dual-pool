package state

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/trungqwe/dual-pool/internal/lockfile"
)

const fixtureTransactionID = "00112233445566778899aabbccddeeff"

func testStore(t *testing.T, options ...Option) *Store {
	t.Helper()
	dir := t.TempDir()
	assertDisposable(t, dir)
	lockDir := filepath.Join(dir, "locks")
	if err := os.Mkdir(lockDir, 0700); err != nil {
		t.Fatal(err)
	}
	manager, err := lockfile.NewManager(lockDir, lockfile.WithOperationIDGenerator(func() (string, error) { return fixtureTransactionID, nil }))
	if err != nil {
		t.Fatal(err)
	}
	options = append(options, WithLockManager(manager))
	options = append([]Option{WithTransactionIDGenerator(func() (string, error) { return fixtureTransactionID, nil })}, options...)
	store, err := NewStore(dir, options...)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func freshStore(t *testing.T, dir string) *Store {
	t.Helper()
	manager, err := lockfile.NewManager(filepath.Join(dir, "locks"), lockfile.WithOperationIDGenerator(func() (string, error) { return fixtureTransactionID, nil }))
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(dir, WithLockManager(manager))
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func assertDisposable(t *testing.T, dir string) {
	t.Helper()
	root := filepath.Join(os.Getenv("LOCALAPPDATA"), "DualPool")
	abs, _ := filepath.Abs(dir)
	rootAbs, _ := filepath.Abs(root)
	if rootAbs != "" && (strings.EqualFold(abs, rootAbs) || strings.HasPrefix(strings.ToLower(abs), strings.ToLower(rootAbs)+string(os.PathSeparator))) {
		t.Fatal("test uses real product root")
	}
}

func updatedState() State {
	s := validState()
	s.Instances.Codex.Port = 19001
	s.Instances.Google.Port = 19002
	s.Antigravity.BridgePort = 19003
	return s
}
func updatedOwnership() Ownership { o := validOwnership(); o.Records[0].KeyPath = "model"; return o }

func TestStoreFirstCreateAndReplaceBothDocuments(t *testing.T) {
	store := testStore(t)
	if err := store.SaveState(validState()); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveOwnership(validOwnership()); err != nil {
		t.Fatal(err)
	}
	if got, err := store.LoadState(); err != nil || !reflect.DeepEqual(got, validState()) {
		t.Fatalf("state create: %v", err)
	}
	if got, err := store.LoadOwnership(); err != nil || !reflect.DeepEqual(got, validOwnership()) {
		t.Fatalf("ownership create: %v", err)
	}
	oldState, _ := os.ReadFile(store.targetPath(documentState))
	oldOwnership, _ := os.ReadFile(store.targetPath(documentOwnership))
	if err := store.SaveState(updatedState()); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveOwnership(updatedOwnership()); err != nil {
		t.Fatal(err)
	}
	newState, _ := os.ReadFile(store.targetPath(documentState))
	newOwnership, _ := os.ReadFile(store.targetPath(documentOwnership))
	if bytes.Equal(oldState, newState) || bytes.Equal(oldOwnership, newOwnership) {
		t.Fatal("replacement did not change bytes")
	}
	entries, _ := os.ReadDir(store.dir)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			t.Fatalf("transaction artifact remains: %s", entry.Name())
		}
	}
}

func TestWindowsReplaceCreatesVerifiedOldBackup(t *testing.T) {
	store := testStore(t)
	old := mustState(t, validState())
	if err := store.SaveState(validState()); err != nil {
		t.Fatal(err)
	}
	store.fault = func(point FaultPoint) error {
		if point == AfterReplace {
			return ErrInjectedCrash
		}
		return nil
	}
	if err := store.SaveState(updatedState()); !errors.Is(err, ErrInjectedCrash) {
		t.Fatal(err)
	}
	markerBytes, err := os.ReadFile(store.markerPath(documentState))
	if err != nil {
		t.Fatal(err)
	}
	marker, err := decodeMarker(markerBytes, documentState)
	if err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(store.ownedArtifactPath(marker.BackupBasename))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(backup, old) || hashBytes(backup) != marker.OldSHA256 {
		t.Fatal("backup is not exact old bytes")
	}
	if exists, _ := safeExists(store.ownedArtifactPath(marker.CandidateBasename)); exists {
		t.Fatal("candidate was not consumed")
	}
	if err := store.Recover(); err != nil {
		t.Fatal(err)
	}
}

func TestStoreRejectsSymlinkArtifacts(t *testing.T) {
	store := testStore(t)
	outside := filepath.Join(t.TempDir(), "outside.json")
	writeFile(t, outside, mustState(t, validState()))
	if err := os.Symlink(outside, store.targetPath(documentState)); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if _, err := store.LoadState(); !errors.Is(err, ErrUnsafeArtifact) {
		t.Fatalf("load: %v", err)
	}
	if err := store.SaveState(updatedState()); !errors.Is(err, ErrUnsafeArtifact) {
		t.Fatalf("save: %v", err)
	}
	after, _ := os.ReadFile(outside)
	if !bytes.Equal(after, mustState(t, validState())) {
		t.Fatal("outside target changed")
	}
}

func TestStoreRejectsJunctionDirectory(t *testing.T) {
	root := t.TempDir()
	realDir := filepath.Join(root, "real")
	linkDir := filepath.Join(root, "junction")
	if err := os.Mkdir(realDir, 0700); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command("cmd", "/c", "mklink", "/J", linkDir, realDir).CombinedOutput()
	if err != nil {
		t.Fatalf("create junction: %v: %s", err, output)
	}
	defer os.Remove(linkDir)
	if _, err := NewStore(linkDir); !errors.Is(err, ErrUnsafeArtifact) {
		t.Fatalf("junction accepted: %v", err)
	}
	entries, _ := os.ReadDir(realDir)
	if len(entries) != 0 {
		t.Fatal("junction destination changed")
	}
}

func TestStoreRejectsInvalidObjectsWithoutFilesystemChanges(t *testing.T) {
	store := testStore(t)
	s := validState()
	s.InstallID = "bad path"
	if err := store.SaveState(s); err == nil {
		t.Fatal("invalid state accepted")
	}
	entries, _ := os.ReadDir(store.dir)
	if len(entries) != 1 || entries[0].Name() != "locks" {
		t.Fatal("filesystem changed")
	}
}

func TestLoadMissingAndRecoveryRequired(t *testing.T) {
	store := testStore(t)
	if _, err := store.LoadState(); !errors.Is(err, ErrNotInitialized) {
		t.Fatal(err)
	}
	marker := markerFor(t, documentState, false, nil, validState())
	writeFile(t, store.markerPath(documentState), mustMarker(marker))
	if _, err := store.LoadState(); !errors.Is(err, ErrRecoveryRequired) {
		t.Fatal(err)
	}
	if err := store.SaveState(validState()); !errors.Is(err, ErrRecoveryRequired) {
		t.Fatal(err)
	}
}

func TestLoadReportsLiveMutationWithoutRecovering(t *testing.T) {
	store := testStore(t)
	guard, err := store.locks.AcquireFile(store.targetPath(documentState))
	if err != nil {
		t.Fatal(err)
	}
	defer guard.Release()
	marker := markerFor(t, documentState, false, nil, validState())
	writeFile(t, store.markerPath(documentState), mustMarker(marker))
	if _, err = store.LoadState(); !errors.Is(err, ErrMutationInProgress) {
		t.Fatalf("live mutation classification: %v", err)
	}
	if _, err = os.Stat(store.markerPath(documentState)); err != nil {
		t.Fatal("read-only load altered recovery artifacts")
	}
}

func TestStoreRefusesCorruptExistingTarget(t *testing.T) {
	cases := map[string][]byte{"truncated": []byte(`{"schema_version":1`), "utf8": {0xff}, "future": bytes.Replace(mustState(t, validState()), []byte(`"schema_version":1`), []byte(`"schema_version":2`), 1), "duplicate": []byte(`{"schema_version":1,"schema_version":1}`), "oversize": bytes.Repeat([]byte(" "), MaxStateDocumentBytes+1)}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			store := testStore(t)
			writeFile(t, store.targetPath(documentState), data)
			before := append([]byte(nil), data...)
			if _, err := store.LoadState(); err == nil {
				t.Fatal("load accepted")
			}
			if err := store.SaveState(validState()); err == nil {
				t.Fatal("save accepted")
			}
			after, _ := os.ReadFile(store.targetPath(documentState))
			if !bytes.Equal(before, after) {
				t.Fatal("target changed")
			}
		})
	}
}

func TestCASConcurrentDriftRefusesOverwrite(t *testing.T) {
	store := testStore(t)
	if err := store.SaveState(validState()); err != nil {
		t.Fatal(err)
	}
	drift := updatedState()
	drift.Instances.Codex.Port = 20001
	drift.Instances.Google.Port = 20002
	drift.Antigravity.BridgePort = 20003
	driftBytes := mustState(t, drift)
	store.fault = func(point FaultPoint) error {
		if point == BeforeCAS {
			return os.WriteFile(store.targetPath(documentState), driftBytes, 0600)
		}
		return nil
	}
	if err := store.SaveState(updatedState()); !errors.Is(err, ErrConcurrentDrift) {
		t.Fatalf("%v", err)
	}
	got, _ := os.ReadFile(store.targetPath(documentState))
	if !bytes.Equal(got, driftBytes) {
		t.Fatal("drift overwritten")
	}
	fresh := freshStore(t, store.dir)
	if err := fresh.Recover(); !errors.Is(err, ErrConcurrentDrift) {
		t.Fatalf("%v", err)
	}
}

func TestMutationRequiresLockProvider(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.SaveState(validState()); !errors.Is(err, ErrLockProviderRequired) {
		t.Fatalf("save without lock provider: %v", err)
	}
	if err = store.Recover(); !errors.Is(err, ErrLockProviderRequired) {
		t.Fatalf("recover without lock provider: %v", err)
	}
}

func TestSecondStoreCannotEnterCASReplaceWindow(t *testing.T) {
	storeA := testStore(t)
	if err := storeA.SaveState(validState()); err != nil {
		t.Fatal(err)
	}
	storeB := freshStore(t, storeA.dir)
	entered := make(chan struct{})
	release := make(chan struct{})
	storeA.fault = func(point FaultPoint) error {
		if point == AfterCASBeforeReplace {
			close(entered)
			<-release
		}
		return nil
	}
	done := make(chan error, 1)
	go func() { done <- storeA.SaveState(updatedState()) }()
	<-entered
	if err := storeB.SaveState(validState()); !errors.Is(err, lockfile.ErrLockHeld) {
		close(release)
		t.Fatalf("second writer entered mutation: %v", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestRecoverDoesNotDeleteLiveWriterCandidate(t *testing.T) {
	storeA := testStore(t)
	storeB := freshStore(t, storeA.dir)
	entered := make(chan struct{})
	release := make(chan struct{})
	storeA.fault = func(point FaultPoint) error {
		if point == AfterCandidateCreate {
			close(entered)
			<-release
		}
		return nil
	}
	done := make(chan error, 1)
	go func() { done <- storeA.SaveState(validState()) }()
	<-entered
	candidate := storeA.ownedArtifactPath("." + documentFilename(documentState) + ".tmp-" + fixtureTransactionID)
	if err := storeB.Recover(); !errors.Is(err, lockfile.ErrLockHeld) {
		close(release)
		t.Fatalf("recovery entered live mutation: %v", err)
	}
	if exists, err := safeExists(candidate); err != nil || !exists {
		close(release)
		t.Fatalf("live candidate removed: exists=%v err=%v", exists, err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestFaultMatrixExistingAndFirstCreation(t *testing.T) {
	points := []FaultPoint{AfterCandidateCreate, AfterCandidateWrite, AfterCandidateSync, AfterMarkerSync, BeforeCAS, AfterCASBeforeReplace, AfterReplace, AfterTargetVerify, AfterTargetSync, BeforeBackupCleanup, BeforeMarkerCleanup}
	for _, initial := range []string{"old", "absent"} {
		for _, point := range points {
			t.Run(initial+"/"+string(point), func(t *testing.T) {
				store := testStore(t)
				oldHash := ""
				if initial == "old" {
					if err := store.SaveState(validState()); err != nil {
						t.Fatal(err)
					}
					oldHash = hashBytes(mustState(t, validState()))
				}
				store.fault = func(got FaultPoint) error {
					if got == point {
						return ErrInjectedCrash
					}
					return nil
				}
				if err := store.SaveState(updatedState()); !errors.Is(err, ErrInjectedCrash) {
					t.Fatalf("fault not reached: %v", err)
				}
				fresh := freshStore(t, store.dir)
				if err := fresh.Recover(); err != nil {
					t.Fatalf("recover: %v", err)
				}
				exists, valid, finalHash := inspectDocument(fresh.targetPath(documentState), documentState)
				newHash := hashBytes(mustState(t, updatedState()))
				if exists && !valid {
					t.Fatal("partial target")
				}
				if initial == "old" {
					if !exists || (finalHash != oldHash && finalHash != newHash) {
						t.Fatalf("unexpected hash %s", finalHash)
					}
				} else if exists && finalHash != newHash {
					t.Fatalf("unexpected hash %s", finalHash)
				}
				if err := fresh.Recover(); err != nil {
					t.Fatalf("idempotence: %v", err)
				}
			})
		}
	}
}

func TestRecoveryTruthTableAndInvalidArtifacts(t *testing.T) {
	t.Run("invalid marker", func(t *testing.T) {
		store := testStore(t)
		writeFile(t, store.targetPath(documentState), mustState(t, validState()))
		writeFile(t, store.markerPath(documentState), []byte(`{"schema_version":1,"schema_version":1}`))
		before, _ := os.ReadFile(store.targetPath(documentState))
		if err := store.Recover(); !errors.Is(err, ErrRecoveryMarkerInvalid) {
			t.Fatal(err)
		}
		after, _ := os.ReadFile(store.targetPath(documentState))
		if !bytes.Equal(before, after) {
			t.Fatal("target touched")
		}
	})
	t.Run("backup restore", func(t *testing.T) {
		store := testStore(t)
		old := mustState(t, validState())
		marker := markerFor(t, documentState, true, old, updatedState())
		writeFile(t, store.targetPath(documentState), []byte(`{"broken"`))
		writeFile(t, store.ownedArtifactPath(marker.BackupBasename), old)
		writeFile(t, store.ownedArtifactPath(marker.CandidateBasename), mustState(t, updatedState()))
		writeFile(t, store.markerPath(documentState), mustMarker(marker))
		result, err := store.recoverOne(documentState)
		if err != nil || result != recoveryRolledBack {
			t.Fatalf("%s %v", result, err)
		}
		got, _ := os.ReadFile(store.targetPath(documentState))
		if !bytes.Equal(got, old) {
			t.Fatal("old not restored")
		}
	})
	t.Run("invalid backup", func(t *testing.T) {
		store := testStore(t)
		old := mustState(t, validState())
		marker := markerFor(t, documentState, true, old, updatedState())
		writeFile(t, store.targetPath(documentState), []byte(`{"broken"`))
		writeFile(t, store.ownedArtifactPath(marker.BackupBasename), []byte(`{"bad"`))
		writeFile(t, store.markerPath(documentState), mustMarker(marker))
		if _, err := store.recoverOne(documentState); !errors.Is(err, ErrRecoveryUnresolved) {
			t.Fatal(err)
		}
	})
	t.Run("invalid candidate abort", func(t *testing.T) {
		store := testStore(t)
		old := mustState(t, validState())
		marker := markerFor(t, documentState, true, old, updatedState())
		writeFile(t, store.targetPath(documentState), old)
		writeFile(t, store.ownedArtifactPath(marker.CandidateBasename), []byte(`{"bad"`))
		writeFile(t, store.markerPath(documentState), mustMarker(marker))
		result, err := store.recoverOne(documentState)
		if err != nil || result != recoveryAborted {
			t.Fatalf("%s %v", result, err)
		}
	})
	t.Run("unsafe backup preserves all diagnostics", func(t *testing.T) {
		store := testStore(t)
		old := mustState(t, validState())
		marker := markerFor(t, documentState, true, old, updatedState())
		writeFile(t, store.targetPath(documentState), old)
		candidate := store.ownedArtifactPath(marker.CandidateBasename)
		writeFile(t, candidate, mustState(t, updatedState()))
		markerPath := store.markerPath(documentState)
		writeFile(t, markerPath, mustMarker(marker))
		outside := t.TempDir()
		backup := store.ownedArtifactPath(marker.BackupBasename)
		output, err := exec.Command("cmd", "/c", "mklink", "/J", backup, outside).CombinedOutput()
		if err != nil {
			t.Fatalf("create backup junction: %v: %s", err, output)
		}
		defer os.Remove(backup)
		before, _ := os.ReadFile(store.targetPath(documentState))
		if _, err = store.recoverOne(documentState); !errors.Is(err, ErrUnsafeArtifact) {
			t.Fatalf("unsafe backup accepted: %v", err)
		}
		for _, path := range []string{candidate, markerPath} {
			if _, err = os.Lstat(path); err != nil {
				t.Fatalf("diagnostic artifact removed: %v", err)
			}
		}
		after, _ := os.ReadFile(store.targetPath(documentState))
		if !bytes.Equal(before, after) {
			t.Fatal("target changed")
		}
	})
}

func TestSubprocessStoreLockContention(t *testing.T) {
	if os.Getenv("DUALPOOL_STORE_HOLD_CHILD") != "" {
		dir := os.Getenv("DUALPOOL_STORE_DIR")
		manager, _ := lockfile.NewManager(filepath.Join(dir, "locks"))
		store, _ := NewStore(dir, WithLockManager(manager), WithFaultInjector(func(point FaultPoint) error {
			if string(point) == os.Getenv("DUALPOOL_STORE_HOLD_POINT") {
				_ = os.WriteFile(filepath.Join(dir, "ready"), []byte("ready"), 0600)
				time.Sleep(10 * time.Minute)
			}
			return nil
		}))
		_ = store.SaveState(validState())
		os.Exit(0)
	}
	for _, point := range []FaultPoint{AfterCandidateCreate, AfterCASBeforeReplace} {
		t.Run(string(point), func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "locks"), 0700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(os.Args[0], "-test.run=TestSubprocessStoreLockContention")
			cmd.Env = append(os.Environ(), "DUALPOOL_STORE_HOLD_CHILD=1", "DUALPOOL_STORE_DIR="+dir, "DUALPOOL_STORE_HOLD_POINT="+string(point))
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			ready := filepath.Join(dir, "ready")
			for i := 0; i < 200; i++ {
				if _, err := os.Stat(ready); err == nil {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			store := freshStore(t, dir)
			if err := store.SaveState(updatedState()); !errors.Is(err, lockfile.ErrLockHeld) {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
				t.Fatalf("second writer entered: %v", err)
			}
			if point == AfterCandidateCreate {
				if err := store.Recover(); !errors.Is(err, lockfile.ErrLockHeld) {
					t.Fatalf("recover entered: %v", err)
				}
			}
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			_ = os.Remove(ready)
			if err := store.Recover(); err != nil {
				t.Fatalf("stale lock recovery: %v", err)
			}
			if err := store.SaveState(updatedState()); err != nil {
				t.Fatalf("later writer: %v", err)
			}
		})
	}
}

func TestMarkerStrictValidation(t *testing.T) {
	store := testStore(t)
	valid := markerFor(t, documentState, false, nil, validState())
	validBytes := mustMarker(valid)
	cases := map[string][]byte{"truncated": validBytes[:len(validBytes)/2], "invalid utf8": {0xff}, "duplicate": bytes.Replace(validBytes, []byte(`"schema_version":1`), []byte(`"schema_version":1,"schema_version":1`), 1), "unknown": bytes.Replace(validBytes, []byte(`"document_kind"`), []byte(`"extra":true,"document_kind"`), 1), "wrong kind": bytes.Replace(validBytes, []byte(`"document_kind":"state"`), []byte(`"document_kind":"ownership"`), 1), "bad hash": bytes.Replace(validBytes, []byte(valid.NewSHA256), []byte("unknown"), 1), "escape basename": bytes.Replace(validBytes, []byte(valid.CandidateBasename), []byte(`..\\outside`), 1), "oversize": bytes.Repeat([]byte(" "), maxMarkerBytes+1)}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			writeFile(t, store.markerPath(documentState), data)
			if err := store.Recover(); !errors.Is(err, ErrRecoveryMarkerInvalid) {
				t.Fatalf("%v", err)
			}
			_ = os.Remove(store.markerPath(documentState))
		})
	}
}

func TestSubprocessCrashRecovery(t *testing.T) {
	if os.Getenv("DUALPOOL_STORE_CHILD") != "" {
		subprocessChild()
		return
	}
	for _, point := range []FaultPoint{AfterMarkerSync, AfterReplace} {
		t.Run(string(point), func(t *testing.T) {
			dir := t.TempDir()
			assertDisposable(t, dir)
			if err := os.Mkdir(filepath.Join(dir, "locks"), 0700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(os.Args[0], "-test.run=TestSubprocessCrashRecovery")
			cmd.Env = append(os.Environ(), "DUALPOOL_STORE_CHILD=1", "DUALPOOL_STORE_DIR="+dir, "DUALPOOL_STORE_POINT="+string(point))
			if err := cmd.Run(); err == nil {
				t.Fatal("child did not exit")
			}
			store := freshStore(t, dir)
			if err := store.Recover(); err != nil {
				t.Fatal(err)
			}
			exists, valid, _ := inspectDocument(store.targetPath(documentState), documentState)
			if point == AfterMarkerSync && exists {
				t.Fatal("pre-install crash unexpectedly committed")
			}
			if point == AfterReplace && !valid {
				t.Fatal("post-replace target invalid")
			}
		})
	}
}

func subprocessChild() {
	dir := os.Getenv("DUALPOOL_STORE_DIR")
	manager, _ := lockfile.NewManager(filepath.Join(dir, "locks"))
	store, _ := NewStore(dir, WithLockManager(manager), WithTransactionIDGenerator(func() (string, error) { return fixtureTransactionID, nil }), WithFaultInjector(func(point FaultPoint) error {
		if string(point) == os.Getenv("DUALPOOL_STORE_POINT") {
			os.Exit(73)
		}
		return nil
	}))
	_ = store.SaveState(validState())
	os.Exit(0)
}
func markerFor(t *testing.T, kind documentKind, oldExists bool, old []byte, newValue State) recoveryMarker {
	t.Helper()
	m := recoveryMarker{SchemaVersion: 1, DocumentKind: kind, TransactionID: fixtureTransactionID, OldExists: oldExists, NewSHA256: hashBytes(mustState(t, newValue)), CandidateBasename: "." + documentFilename(kind) + ".tmp-" + fixtureTransactionID}
	if oldExists {
		m.OldSHA256 = hashBytes(old)
		m.BackupBasename = "." + documentFilename(kind) + ".bak-" + fixtureTransactionID
	}
	return m
}
func mustState(t *testing.T, value State) []byte {
	t.Helper()
	b, err := EncodeState(value)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}
