package productinit

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/trungqwe/dual-pool/internal/dataroot"
	"github.com/trungqwe/dual-pool/internal/lockfile"
	"github.com/trungqwe/dual-pool/internal/secretstore"
	"github.com/trungqwe/dual-pool/internal/winacl"
)

type fixtureACL struct{}

func (fixtureACL) Create(p string) error {
	err := os.Mkdir(p, 0700)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	return err
}
func (fixtureACL) Inspect(p string) error {
	info, err := os.Lstat(p)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return winacl.ErrUnsafeACL
	}
	return nil
}

type failingACL struct{ fixtureACL }

func (failingACL) Create(string) error { return winacl.ErrUnsafeACL }

type failingChildACL struct {
	fixtureACL
	target string
}

func (a failingChildACL) Create(path string) error {
	if path == a.target {
		return winacl.ErrUnsafeACL
	}
	return a.fixtureACL.Create(path)
}

type failingReadStore struct{ *memoryStore }

func (s failingReadStore) Get(secretstore.Purpose) ([]byte, error) {
	return nil, errors.New("read failed")
}

type failingRandom struct{}

func (failingRandom) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

type memoryStore struct {
	mu     sync.Mutex
	values map[secretstore.Purpose][]byte
	writes int
	failAt int
}

func (s *memoryStore) Get(p secretstore.Purpose) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.values[p]
	if !ok {
		return nil, secretstore.ErrNotFound
	}
	return append([]byte(nil), v...), nil
}
func (s *memoryStore) Put(p secretstore.Purpose, v []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writes++
	if s.failAt == s.writes {
		return errors.New("write failed")
	}
	s.values[p] = append([]byte(nil), v...)
	return nil
}
func (s *memoryStore) Delete(p secretstore.Purpose) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.values, p)
	return nil
}

func TestConcurrentInitializersProduceOneStableKeySet(t *testing.T) {
	l := layout(t)
	s := &memoryStore{values: map[secretstore.Purpose][]byte{}}
	start := make(chan struct{})
	results := make(chan error, 2)
	for n := 0; n < 2; n++ {
		go func(seed byte) {
			<-start
			stream := make([]byte, 32*16)
			for i := range stream {
				stream[i] = seed + byte(i)
			}
			_, err := New(l, fixtureACL{}, s, bytes.NewReader(stream)).Initialize()
			results <- err
		}(byte(n + 1))
	}
	close(start)
	err1, err2 := <-results, <-results
	if err1 != nil && !errors.Is(err1, lockfile.ErrLockHeld) {
		t.Fatal(err1)
	}
	if err2 != nil && !errors.Is(err2, lockfile.ErrLockHeld) {
		t.Fatal(err2)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.values) != 4 || s.writes != 4 {
		t.Fatalf("keys=%d writes=%d", len(s.values), s.writes)
	}
}

func layout(t *testing.T) dataroot.Layout {
	t.Helper()
	l, err := dataroot.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return l
}
func randomBytes() []byte {
	b := make([]byte, 32*16)
	for i := range b {
		b[i] = byte(i + 1)
	}
	return b
}

func TestInitializeCreatesLayoutAndFourIndependentKeysIdempotently(t *testing.T) {
	l := layout(t)
	s := &memoryStore{values: map[secretstore.Purpose][]byte{}}
	i := New(l, fixtureACL{}, s, bytes.NewReader(randomBytes()))
	r, err := i.Initialize()
	if err != nil || !r.Ready || r.CreatedDirectories != 8 || r.CreatedKeys != 4 {
		t.Fatalf("first: %#v %v", r, err)
	}
	before := map[secretstore.Purpose][]byte{}
	for p, v := range s.values {
		before[p] = append([]byte(nil), v...)
	}
	r, err = i.Initialize()
	if err != nil || !r.Ready || r.CreatedDirectories != 0 || r.CreatedKeys != 0 {
		t.Fatalf("second: %#v %v", r, err)
	}
	for p, v := range before {
		if !bytes.Equal(v, s.values[p]) {
			t.Fatal("key changed")
		}
	}
}

func TestInitializeReconcilesEveryPartialCount(t *testing.T) {
	for count := 0; count <= 4; count++ {
		t.Run(string(rune('0'+count)), func(t *testing.T) {
			l := layout(t)
			s := &memoryStore{values: map[secretstore.Purpose][]byte{}}
			if err := (fixtureACL{}).Create(l.Root); err != nil {
				t.Fatal(err)
			}
			for _, d := range []string{l.Bin, l.Instances, l.Config, l.State, l.Backups, l.Evidence, l.Locks} {
				if err := (fixtureACL{}).Create(d); err != nil {
					t.Fatal(err)
				}
			}
			for n := 0; n < count; n++ {
				s.values[purposes[n]] = bytes.Repeat([]byte{byte(n + 1)}, 32)
			}
			r, err := New(l, fixtureACL{}, s, bytes.NewReader(randomBytes())).Initialize()
			if err != nil || r.CreatedKeys != 4-count || len(s.values) != 4 {
				t.Fatalf("%#v %v", r, err)
			}
		})
	}
}

func TestInitializeRejectsInvalidOrDuplicateKeysBeforeWrite(t *testing.T) {
	for _, values := range []map[secretstore.Purpose][]byte{
		{secretstore.CodexClientKey: []byte{1}},
		{secretstore.CodexClientKey: bytes.Repeat([]byte{1}, 32), secretstore.GoogleClientKey: bytes.Repeat([]byte{1}, 32)},
	} {
		l := layout(t)
		s := &memoryStore{values: values}
		_, err := New(l, fixtureACL{}, s, bytes.NewReader(randomBytes())).Initialize()
		if !errors.Is(err, ErrConflict) && !errors.Is(err, ErrInvalidKeys) {
			t.Fatalf("%v", err)
		}
		if s.writes != 0 {
			t.Fatal("unexpected write")
		}
	}
}

func TestInitializeRetriesAfterPartialStoreFailure(t *testing.T) {
	l := layout(t)
	s := &memoryStore{values: map[secretstore.Purpose][]byte{}, failAt: 3}
	i := New(l, fixtureACL{}, s, bytes.NewReader(randomBytes()))
	if _, err := i.Initialize(); err == nil || len(s.values) != 2 {
		t.Fatalf("%v %d", err, len(s.values))
	}
	s.failAt = 0
	r, err := i.Initialize()
	if err != nil || r.CreatedKeys != 2 || len(s.values) != 4 {
		t.Fatalf("%#v %v", r, err)
	}
}

func TestInitializeRejectsUnexpectedRootEntry(t *testing.T) {
	l := layout(t)
	s := &memoryStore{values: map[secretstore.Purpose][]byte{}}
	if err := os.Mkdir(l.Root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(l.Root, "foreign"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := New(l, fixtureACL{}, s, bytes.NewReader(randomBytes())).Initialize()
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("%v", err)
	}
	if s.writes != 0 {
		t.Fatal("unexpected write")
	}
}

func TestInitializeACLFailureWritesNoKeys(t *testing.T) {
	l := layout(t)
	s := &memoryStore{values: map[secretstore.Purpose][]byte{}}
	if _, err := New(l, failingACL{}, s, bytes.NewReader(randomBytes())).Initialize(); !errors.Is(err, winacl.ErrUnsafeACL) {
		t.Fatalf("%v", err)
	}
	if s.writes != 0 {
		t.Fatal("unexpected write")
	}
}

func TestChildACLFailureWritesNoKeys(t *testing.T) {
	l := layout(t)
	s := &memoryStore{values: map[secretstore.Purpose][]byte{}}
	_, err := New(l, failingChildACL{target: l.Config}, s, bytes.NewReader(randomBytes())).Initialize()
	if !errors.Is(err, winacl.ErrUnsafeACL) || s.writes != 0 {
		t.Fatalf("%v writes=%d", err, s.writes)
	}
}

func TestSecretReadFailureWritesNoKeys(t *testing.T) {
	l := layout(t)
	s := &memoryStore{values: map[secretstore.Purpose][]byte{}}
	_, err := New(l, fixtureACL{}, failingReadStore{s}, bytes.NewReader(randomBytes())).Initialize()
	if err == nil || s.writes != 0 || exists(l.Root) {
		t.Fatalf("%v writes=%d", err, s.writes)
	}
}

func TestRandomFailureWritesNoKeys(t *testing.T) {
	l := layout(t)
	s := &memoryStore{values: map[secretstore.Purpose][]byte{}}
	_, err := New(l, fixtureACL{}, s, failingRandom{}).Initialize()
	if !errors.Is(err, io.ErrUnexpectedEOF) || s.writes != 0 {
		t.Fatalf("%v writes=%d", err, s.writes)
	}
}

func TestGlobalLockFailureWritesNoKeys(t *testing.T) {
	l := layout(t)
	s := &memoryStore{values: map[secretstore.Purpose][]byte{}}
	for _, d := range []string{l.Root, l.Bin, l.Instances, l.Config, l.State, l.Backups, l.Evidence, l.Locks} {
		if err := (fixtureACL{}).Create(d); err != nil {
			t.Fatal(err)
		}
	}
	m, err := lockfile.NewManager(l.Locks)
	if err != nil {
		t.Fatal(err)
	}
	g, err := m.AcquireGlobal()
	if err != nil {
		t.Fatal(err)
	}
	defer g.Release()
	_, err = New(l, fixtureACL{}, s, bytes.NewReader(randomBytes())).Initialize()
	if !errors.Is(err, lockfile.ErrLockHeld) || s.writes != 0 {
		t.Fatalf("%v writes=%d", err, s.writes)
	}
}

func TestInitializeRejectsExhaustedRandomCollisions(t *testing.T) {
	l := layout(t)
	s := &memoryStore{values: map[secretstore.Purpose][]byte{}}
	seed := bytes.Repeat([]byte{7}, 32)
	s.values[secretstore.CodexClientKey] = append([]byte(nil), seed...)
	if err := (fixtureACL{}).Create(l.Root); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{l.Bin, l.Instances, l.Config, l.State, l.Backups, l.Evidence, l.Locks} {
		if err := (fixtureACL{}).Create(d); err != nil {
			t.Fatal(err)
		}
	}
	_, err := New(l, fixtureACL{}, s, bytes.NewReader(bytes.Repeat(seed, 8))).Initialize()
	if !errors.Is(err, ErrInvalidKeys) {
		t.Fatalf("%v", err)
	}
	if s.writes != 0 {
		t.Fatal("collision was written")
	}
}

func TestInspectIsReadOnlyAndFreshKeyConflict(t *testing.T) {
	l := layout(t)
	s := &memoryStore{values: map[secretstore.Purpose][]byte{}}
	i := New(l, fixtureACL{}, s, bytes.NewReader(randomBytes()))
	state, err := i.Inspect()
	if err != nil || state.RootExists || state.KeysPresent != 0 || exists(l.Root) {
		t.Fatal("fresh inspection changed state")
	}
	s.values[secretstore.CodexClientKey] = bytes.Repeat([]byte{7}, 32)
	if _, err = i.Inspect(); !errors.Is(err, ErrConflict) {
		t.Fatalf("%v", err)
	}
	if _, err = i.Initialize(); !errors.Is(err, ErrConflict) {
		t.Fatalf("%v", err)
	}
	if exists(l.Root) || s.writes != 0 {
		t.Fatal("conflict changed state")
	}
}

type corruptFinalStore struct {
	*memoryStore
	reads int
}

func (s *corruptFinalStore) Get(p secretstore.Purpose) ([]byte, error) {
	s.reads++
	v, err := s.memoryStore.Get(p)
	if s.reads == 9 && err == nil {
		v[0] ^= 0xff
	}
	return v, err
}

func TestFinalKeyVerificationRejectsChangedStoreValue(t *testing.T) {
	l := layout(t)
	base := &memoryStore{values: map[secretstore.Purpose][]byte{}}
	s := &corruptFinalStore{memoryStore: base}
	_, err := New(l, fixtureACL{}, s, bytes.NewReader(randomBytes())).Initialize()
	if !errors.Is(err, ErrInvalidKeys) || base.writes != 4 {
		t.Fatalf("%v writes=%d", err, base.writes)
	}
	r, err := New(l, fixtureACL{}, base, bytes.NewReader(nil)).Initialize()
	if err != nil || !r.Ready || r.CreatedKeys != 0 {
		t.Fatalf("%#v %v", r, err)
	}
}

func TestInitializerDoesNotPersistSyntheticKeySentinel(t *testing.T) {
	l := layout(t)
	s := &memoryStore{values: map[secretstore.Purpose][]byte{}}
	r, err := New(l, fixtureACL{}, s, bytes.NewReader(randomBytes())).Initialize()
	if err != nil || !r.Ready {
		t.Fatalf("%v", err)
	}
	for _, value := range s.values {
		walkErr := filepath.WalkDir(l.Root, func(path string, entry os.DirEntry, readErr error) error {
			if readErr != nil {
				return readErr
			}
			if entry.IsDir() {
				return nil
			}
			data, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			if bytes.Contains(data, value) {
				t.Fatal("file disclosed key")
			}
			return nil
		})
		if walkErr != nil {
			t.Fatal(walkErr)
		}
	}
}
