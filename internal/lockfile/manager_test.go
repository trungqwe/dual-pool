package lockfile

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
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

func TestGlobalLockLiveExpiryReleaseAndStaleRecovery(t *testing.T) {
	identity := fixtureIdentity()
	inspector := &fakeInspector{values: map[uint32]ProcessIdentity{identity.PID: identity}, errs: map[uint32]error{}}
	m := fixtureManager(t, inspector, "00112233445566778899aabbccddeeff", "11223344556677889900aabbccddeeff")
	guard, err := m.AcquireGlobal()
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(guard.path)
	m.now = func() time.Time { return time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC) }
	if _, err = m.AcquireGlobal(); !errors.Is(err, ErrLockHeld) {
		t.Fatalf("expired live owner stolen: %v", err)
	}
	after, _ := os.ReadFile(guard.path)
	if !bytes.Equal(before, after) {
		t.Fatal("live lock changed")
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

func TestReleasePreservesChangedCanonicalLock(t *testing.T) {
	identity := fixtureIdentity()
	inspector := &fakeInspector{values: map[uint32]ProcessIdentity{identity.PID: identity}, errs: map[uint32]error{}}
	m := fixtureManager(t, inspector, "00112233445566778899aabbccddeeff")
	guard, err := m.AcquireGlobal()
	if err != nil {
		t.Fatal(err)
	}
	changed := guard.record
	changed.OperationID = "11223344556677889900aabbccddeeff"
	payload, _ := encodeRecord(changed)
	if err = os.WriteFile(guard.path, payload, 0600); err != nil {
		t.Fatal(err)
	}
	if err = guard.Release(); !errors.Is(err, ErrLockOwnershipLost) {
		t.Fatalf("changed lock released: %v", err)
	}
	after, err := os.ReadFile(guard.path)
	if err != nil || !bytes.Equal(after, payload) {
		t.Fatal("changed canonical lock was removed or altered")
	}
}

func TestPIDReuseImageMismatchAndUnverifiable(t *testing.T) {
	for name, mutate := range map[string]func(*fakeInspector, ProcessIdentity){"pid reused": func(f *fakeInspector, v ProcessIdentity) { v.StartTime++; f.set(v) }, "image mismatch": func(f *fakeInspector, v ProcessIdentity) { v.Image = `c:\fixture\other.exe`; f.set(v) }} {
		t.Run(name, func(t *testing.T) {
			identity := fixtureIdentity()
			f := &fakeInspector{values: map[uint32]ProcessIdentity{identity.PID: identity}, errs: map[uint32]error{}}
			m := fixtureManager(t, f, "00112233445566778899aabbccddeeff", "11223344556677889900aabbccddeeff")
			_, err := m.AcquireGlobal()
			if err != nil {
				t.Fatal(err)
			}
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
		guard, _ := m.AcquireGlobal()
		before, _ := os.ReadFile(guard.path)
		f.errs[identity.PID] = ErrLockOwnerUnverifiable
		if _, err := m.AcquireGlobal(); !errors.Is(err, ErrLockOwnerUnverifiable) {
			t.Fatal(err)
		}
		after, _ := os.ReadFile(guard.path)
		if !bytes.Equal(before, after) {
			t.Fatal("unverifiable lock changed")
		}
	})
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
