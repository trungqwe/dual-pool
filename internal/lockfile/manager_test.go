package lockfile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

type fakeInspector struct {
	mu     sync.Mutex
	values map[uint32]ProcessIdentity
	errs   map[uint32]error
}

func (f *fakeInspector) Inspect(pid uint32) (ProcessIdentity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.errs[pid]; err != nil {
		return ProcessIdentity{}, err
	}
	v, ok := f.values[pid]
	if !ok {
		return ProcessIdentity{}, ErrProcessNotFound
	}
	return v, nil
}
func (f *fakeInspector) set(v ProcessIdentity) { f.mu.Lock(); defer f.mu.Unlock(); f.values[v.PID] = v }
func fixtureIdentity() ProcessIdentity {
	return ProcessIdentity{PID: uint32(os.Getpid()), StartTime: 100, Image: `c:\fixture\process.exe`}
}
func fixtureManager(t *testing.T, inspector *fakeInspector, ids ...string) *Manager {
	t.Helper()
	dir := t.TempDir()
	index := 0
	generator := func() (string, error) { value := ids[index%len(ids)]; index++; return value, nil }
	m, err := NewManager(dir, WithInspector(inspector), WithClock(func() time.Time { return time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC) }), WithOperationIDGenerator(generator))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func seedGlobalRecord(t *testing.T, m *Manager, identity ProcessIdentity, operation string) []byte {
	t.Helper()
	created := m.now().UTC()
	payload, err := encodeRecord(record{SchemaVersion: 1, Kind: kindGlobal, ResourceID: "global", OwnerPID: identity.PID, OwnerStartTime: identity.StartTime, OwnerImage: identity.Image, OperationID: operation, CreatedAt: created.Format(time.RFC3339Nano), ExpiresAt: created.Add(m.duration).Format(time.RFC3339Nano)})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(m.root, "global.lock"), payload, 0600); err != nil {
		t.Fatal(err)
	}
	return payload
}

func TestGlobalLockLiveExpiryReleaseAndStaleRecovery(t *testing.T) {
	identity := fixtureIdentity()
	inspector := &fakeInspector{values: map[uint32]ProcessIdentity{identity.PID: identity}, errs: map[uint32]error{}}
	m := fixtureManager(t, inspector, "00112233445566778899aabbccddeeff", "11223344556677889900aabbccddeeff")
	guard, err := m.AcquireGlobal()
	if err != nil {
		t.Fatal(err)
	}
	before := append([]byte(nil), guard.bytes...)
	m.now = func() time.Time { return time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC) }
	if _, err = m.AcquireGlobal(); !errors.Is(err, ErrLockHeld) {
		t.Fatalf("expired live owner stolen: %v", err)
	}
	after, readErr := readHandle(guard.file)
	if readErr != nil || !bytes.Equal(before, after.bytes) {
		t.Fatal("live lock changed")
	}
	if err = guard.Release(); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(guard.path, before, 0600); err != nil {
		t.Fatal(err)
	}
	identity.StartTime++
	inspector.set(identity)
	next, err := m.AcquireGlobal()
	if err != nil {
		t.Fatal(err)
	}
	if err = next.Release(); err != nil {
		t.Fatal(err)
	}
	if err = next.Release(); err != nil {
		t.Fatal("double release")
	}
}

func TestOwnershipHandleBlocksCanonicalMutation(t *testing.T) {
	identity := fixtureIdentity()
	inspector := &fakeInspector{values: map[uint32]ProcessIdentity{identity.PID: identity}, errs: map[uint32]error{}}
	m := fixtureManager(t, inspector, "00112233445566778899aabbccddeeff")
	guard, err := m.AcquireGlobal()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(guard.path, []byte("changed"), 0600); err == nil {
		t.Fatal("canonical lock was writable while Guard was alive")
	}
	if err = guard.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(guard.path); !os.IsNotExist(err) {
		t.Fatalf("handle-backed release did not remove canonical lock: %v", err)
	}
}

func TestReturnedGuardHoldsExclusiveCanonicalHandle(t *testing.T) {
	m, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	guard, err := m.AcquireGlobal()
	if err != nil {
		t.Fatal(err)
	}
	defer guard.Release()
	path, _ := windows.UTF16PtrFromString(guard.path)
	for name, access := range map[string]uint32{"write": windows.GENERIC_WRITE, "delete": windows.DELETE} {
		t.Run(name, func(t *testing.T) {
			handle, openErr := windows.CreateFile(path, access, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
			if openErr == nil {
				windows.CloseHandle(handle)
				t.Fatal("canonical lock accepted an incompatible open while Guard was alive")
			}
			if !errors.Is(openErr, windows.ERROR_SHARING_VIOLATION) {
				t.Fatalf("unexpected open classification: %v", openErr)
			}
		})
	}
}

func TestPIDReuseImageMismatchAndUnverifiable(t *testing.T) {
	for name, mutate := range map[string]func(*fakeInspector, ProcessIdentity){"pid reused": func(f *fakeInspector, v ProcessIdentity) { v.StartTime++; f.set(v) }, "image mismatch": func(f *fakeInspector, v ProcessIdentity) { v.Image = `c:\fixture\other.exe`; f.set(v) }} {
		t.Run(name, func(t *testing.T) {
			identity := fixtureIdentity()
			f := &fakeInspector{values: map[uint32]ProcessIdentity{identity.PID: identity}, errs: map[uint32]error{}}
			m := fixtureManager(t, f, "00112233445566778899aabbccddeeff", "11223344556677889900aabbccddeeff")
			seedGlobalRecord(t, m, identity, "00112233445566778899aabbccddeeff")
			mutate(f, identity)
			guard, err := m.AcquireGlobal()
			if err != nil {
				t.Fatal(err)
			}
			_ = guard.Release()
		})
	}
	t.Run("unverifiable", func(t *testing.T) {
		identity := fixtureIdentity()
		f := &fakeInspector{values: map[uint32]ProcessIdentity{identity.PID: identity}, errs: map[uint32]error{}}
		m := fixtureManager(t, f, "00112233445566778899aabbccddeeff", "11223344556677889900aabbccddeeff")
		before := seedGlobalRecord(t, m, identity, "00112233445566778899aabbccddeeff")
		f.errs[identity.PID] = ErrLockOwnerUnverifiable
		if _, err := m.AcquireGlobal(); !errors.Is(err, ErrLockOwnerUnverifiable) {
			t.Fatal(err)
		}
		after, _ := os.ReadFile(filepath.Join(m.root, "global.lock"))
		if !bytes.Equal(before, after) {
			t.Fatal("unverifiable lock changed")
		}
		if err := os.Remove(filepath.Join(m.root, "global.lock")); err != nil {
			t.Fatalf("failed acquisition leaked claim handle: %v", err)
		}
	})
}

func TestInvalidCanonicalRecordPreservedAndClaimClosed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "global.lock")
	payload := []byte(`{"schema_version":1,"schema_version":1}`)
	if err := os.WriteFile(path, payload, 0600); err != nil {
		t.Fatal(err)
	}
	m, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.AcquireGlobal(); !errors.Is(err, ErrLockRecordInvalid) {
		t.Fatalf("invalid record classification: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, payload) {
		t.Fatal("invalid canonical record changed")
	}
	if err = os.Remove(path); err != nil {
		t.Fatalf("failed acquisition leaked claim handle: %v", err)
	}
}

func TestPerFileIndependenceAndAliases(t *testing.T) {
	identity := fixtureIdentity()
	f := &fakeInspector{values: map[uint32]ProcessIdentity{identity.PID: identity}, errs: map[uint32]error{}}
	m := fixtureManager(t, f, "00112233445566778899aabbccddeeff", "11223344556677889900aabbccddeeff", "22334455667788990011aabbccddeeff")
	dir := t.TempDir()
	a := filepath.Join(dir, "a.json")
	b := filepath.Join(dir, "b.json")
	if err := os.WriteFile(a, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	ga, err := m.AcquireFile(a)
	if err != nil {
		t.Fatal(err)
	}
	gb, err := m.AcquireFile(b)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.AcquireFile(strings.ToUpper(a)); !errors.Is(err, ErrLockHeld) {
		t.Fatalf("alias escaped lock: %v", err)
	}
	_ = gb.Release()
	_ = ga.Release()
}

func TestStrictRecordCodec(t *testing.T) {
	valid := record{SchemaVersion: 1, Kind: kindGlobal, ResourceID: "global", OwnerPID: 1, OwnerStartTime: 2, OwnerImage: `c:\fixture\x.exe`, OperationID: "00112233445566778899aabbccddeeff", CreatedAt: "2026-09-20T00:00:00Z", ExpiresAt: "2026-09-20T00:05:00Z"}
	payload, _ := encodeRecord(valid)
	cases := [][]byte{nil, payload[:len(payload)/2], {0xff}, bytes.Replace(payload, []byte(`"kind"`), []byte(`"extra":true,"kind"`), 1), bytes.Replace(payload, []byte(`"schema_version":1`), []byte(`"schema_version":1,"schema_version":1`), 1), bytes.Repeat([]byte(" "), maxRecordBytes+1)}
	bad := valid
	bad.OwnerPID = 0
	b, _ := jsonMarshal(bad)
	cases = append(cases, b)
	bad = valid
	bad.OwnerStartTime = 0
	b, _ = jsonMarshal(bad)
	cases = append(cases, b)
	bad = valid
	bad.ExpiresAt = bad.CreatedAt
	b, _ = jsonMarshal(bad)
	cases = append(cases, b)
	bad = valid
	bad.OperationID = "predictable"
	b, _ = jsonMarshal(bad)
	cases = append(cases, b)
	bad = valid
	bad.OwnerImage = `relative.exe`
	b, _ = jsonMarshal(bad)
	cases = append(cases, b)
	bad = valid
	bad.ResourceID = "wrong"
	b, _ = jsonMarshal(bad)
	cases = append(cases, b)
	bad = valid
	bad.CreatedAt = "2026-09-20T00:00:00+07:00"
	b, _ = jsonMarshal(bad)
	cases = append(cases, b)
	cases = append(cases,
		bytes.Replace(payload, []byte(`"owner_pid":1`), []byte(`"owner_pid":"1"`), 1),
		bytes.Replace(payload, []byte(`"owner_image":"c:\\fixture\\x.exe",`), nil, 1),
	)
	for i, input := range cases {
		if _, err := decodeRecord(input); !errors.Is(err, ErrLockRecordInvalid) {
			t.Fatalf("case %d: %v", i, err)
		}
	}
}
func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

func TestWindowsProcessIdentityStableAndExit(t *testing.T) {
	inspector := WindowsProcessInspector{}
	first, err := inspector.Inspect(uint32(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	second, err := inspector.Inspect(uint32(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	if first != second || first.StartTime == 0 || first.Image == "" {
		t.Fatal("unstable identity")
	}
	cmd := exec.Command("cmd", "/c", "exit", "0")
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := uint32(cmd.Process.Pid)
	_ = cmd.Wait()
	if _, err = inspector.Inspect(pid); !errors.Is(err, ErrProcessNotFound) {
		t.Fatalf("exited process: %v", err)
	}
}

func TestLockRootJunctionRejected(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	link := filepath.Join(root, "link")
	_ = os.Mkdir(real, 0700)
	out, err := exec.Command("cmd", "/c", "mklink", "/J", link, real).CombinedOutput()
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	defer os.Remove(link)
	if _, err = NewManager(link); !errors.Is(err, ErrUnsafeLockArtifact) {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(real)
	if len(entries) != 0 {
		t.Fatal("junction destination changed")
	}
}

func TestUnsafeCanonicalLockArtifactRejected(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	artifact := filepath.Join(dir, "global.lock")
	output, err := exec.Command("cmd", "/c", "mklink", "/J", artifact, outside).CombinedOutput()
	if err != nil {
		t.Fatalf("create lock junction: %v: %s", err, output)
	}
	defer os.Remove(artifact)
	m, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.AcquireGlobal(); !errors.Is(err, ErrUnsafeLockArtifact) {
		t.Fatalf("unsafe lock artifact accepted: %v", err)
	}
	entries, _ := os.ReadDir(outside)
	if len(entries) != 0 {
		t.Fatal("junction destination changed")
	}
}

func TestGlobalSubprocessContentionAndCrashRecovery(t *testing.T) {
	if os.Getenv("DUALPOOL_LOCK_CHILD") != "" {
		dir := os.Getenv("DUALPOOL_LOCK_DIR")
		m, _ := NewManager(dir)
		_, err := m.AcquireGlobal()
		if err != nil {
			os.Exit(71)
		}
		_ = os.WriteFile(filepath.Join(dir, "ready"), []byte("ready"), 0600)
		time.Sleep(10 * time.Minute)
		os.Exit(0)
	}
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=TestGlobalSubprocessContentionAndCrashRecovery")
	cmd.Env = append(os.Environ(), "DUALPOOL_LOCK_CHILD=1", "DUALPOOL_LOCK_DIR="+dir)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	ready := filepath.Join(dir, "ready")
	readySeen := false
	for i := 0; i < 100; i++ {
		if _, err := os.Stat(ready); err == nil {
			readySeen = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !readySeen {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal("child did not acquire global lock")
	}
	m, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.AcquireGlobal(); !errors.Is(err, ErrLockHeld) {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("contention: %v", err)
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	guard, err := m.AcquireGlobal()
	if err != nil {
		t.Fatalf("stale recovery: %v", err)
	}
	if err = guard.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestSimultaneousStaleReclaim(t *testing.T) {
	if role := os.Getenv("DUALPOOL_RECLAIM_ROLE"); role != "" {
		reclaimChild(role, os.Getenv("DUALPOOL_RECLAIM_CLASS"), os.Getenv("DUALPOOL_RECLAIM_ROOT"), os.Getenv("DUALPOOL_RECLAIM_TARGET"), os.Getenv("DUALPOOL_RECLAIM_PREFIX"), os.Getenv("DUALPOOL_RECLAIM_ID"))
		return
	}
	iterations := 50
	if testing.Short() {
		iterations = 5
	}
	for _, class := range []string{"global", "file"} {
		t.Run(class, func(t *testing.T) {
			root := t.TempDir()
			target := filepath.Join(root, "target.json")
			if err := os.WriteFile(target, []byte("fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			for iteration := 0; iteration < iterations; iteration++ {
				prefix := filepath.Join(root, "sync-"+class+"-"+stringID(iteration))
				runReclaimChild(t, "seed", class, root, target, prefix, "seed")
				commands := []*exec.Cmd{
					startReclaimChild(t, "contender", class, root, target, prefix, "a"),
					startReclaimChild(t, "contender", class, root, target, prefix, "b"),
				}
				waitFiles(t, prefix+"-ready-a", prefix+"-ready-b")
				if err := os.WriteFile(prefix+"-start", []byte("start"), 0600); err != nil {
					t.Fatal(err)
				}
				waitFiles(t, prefix+"-result-a", prefix+"-result-b")
				results := []string{readText(t, prefix+"-result-a"), readText(t, prefix+"-result-b")}
				wins := 0
				unexpected := ""
				for _, result := range results {
					if result == "won" {
						wins++
					} else if result != "held" {
						unexpected = result
					}
				}
				if wins != 1 || unexpected != "" {
					_ = os.WriteFile(prefix+"-release", []byte("release"), 0600)
					for _, cmd := range commands {
						_ = cmd.Wait()
					}
					t.Fatalf("iteration %d acquired guards=%d unexpected=%q", iteration, wins, unexpected)
				}
				manager, err := NewManager(root)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = acquireClass(manager, class, target); !errors.Is(err, ErrLockHeld) {
					t.Fatalf("iteration %d third acquisition: %v", iteration, err)
				}
				if err = os.WriteFile(prefix+"-release", []byte("release"), 0600); err != nil {
					t.Fatal(err)
				}
				for _, cmd := range commands {
					if err = cmd.Wait(); err != nil {
						t.Fatalf("iteration %d child: %v", iteration, err)
					}
				}
				guard, err := acquireClass(manager, class, target)
				if err != nil {
					t.Fatalf("iteration %d post-release: %v", iteration, err)
				}
				if err = guard.Release(); err != nil {
					t.Fatal(err)
				}
				for _, suffix := range []string{"-ready-a", "-ready-b", "-result-a", "-result-b", "-start", "-release"} {
					_ = os.Remove(prefix + suffix)
				}
			}
		})
	}
}

func reclaimChild(role, class, root, target, prefix, id string) {
	manager, err := NewManager(root)
	if err != nil {
		os.Exit(81)
	}
	if role == "seed" {
		if _, err = acquireClass(manager, class, target); err != nil {
			os.Exit(82)
		}
		os.Exit(0)
	}
	_ = os.WriteFile(prefix+"-ready-"+id, []byte("ready"), 0600)
	if !waitFile(prefix+"-start", 10*time.Second) {
		os.Exit(83)
	}
	guard, err := acquireClass(manager, class, target)
	result := "error"
	if err == nil {
		result = "won"
	} else if errors.Is(err, ErrLockHeld) {
		result = "held"
	} else if errors.Is(err, ErrLockPersistence) {
		result = err.Error()
	} else if errors.Is(err, ErrLockOwnershipLost) {
		result = "ownership_lost"
	} else if errors.Is(err, ErrLockOwnerUnverifiable) {
		result = "owner_unverifiable"
	} else if errors.Is(err, ErrLockRecordInvalid) {
		result = "record_invalid"
	} else if errors.Is(err, ErrUnsafeLockArtifact) {
		result = "unsafe_artifact"
	}
	_ = os.WriteFile(prefix+"-result-"+id, []byte(result), 0600)
	if guard != nil {
		if !waitFile(prefix+"-release", 10*time.Second) {
			os.Exit(84)
		}
		if guard.Release() != nil {
			os.Exit(85)
		}
	}
	os.Exit(0)
}

func acquireClass(manager *Manager, class, target string) (*Guard, error) {
	if class == "file" {
		return manager.AcquireFile(target)
	}
	return manager.AcquireGlobal()
}

func runReclaimChild(t *testing.T, role, class, root, target, prefix, id string) {
	t.Helper()
	cmd := reclaimCommand(role, class, root, target, prefix, id)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s child: %v %s", role, err, output)
	}
}

func startReclaimChild(t *testing.T, role, class, root, target, prefix, id string) *exec.Cmd {
	t.Helper()
	cmd := reclaimCommand(role, class, root, target, prefix, id)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	return cmd
}

func reclaimCommand(role, class, root, target, prefix, id string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=TestSimultaneousStaleReclaim$")
	cmd.Env = append(os.Environ(), "DUALPOOL_RECLAIM_ROLE="+role, "DUALPOOL_RECLAIM_CLASS="+class, "DUALPOOL_RECLAIM_ROOT="+root, "DUALPOOL_RECLAIM_TARGET="+target, "DUALPOOL_RECLAIM_PREFIX="+prefix, "DUALPOOL_RECLAIM_ID="+id)
	return cmd
}

func waitFiles(t *testing.T, paths ...string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		all := true
		for _, path := range paths {
			if _, err := os.Stat(path); err != nil {
				all = false
				break
			}
		}
		if all {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for subprocess files")
}

func waitFile(path string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return true
		}
		time.Sleep(2 * time.Millisecond)
	}
	return false
}

func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func stringID(value int) string { return fmt.Sprintf("%03d", value) }
