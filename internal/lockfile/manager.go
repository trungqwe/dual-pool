package lockfile

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

type Option func(*Manager)

func WithInspector(value ProcessInspector) Option { return func(m *Manager) { m.inspector = value } }
func WithClock(value func() time.Time) Option     { return func(m *Manager) { m.now = value } }
func WithOperationIDGenerator(value func() (string, error)) Option {
	return func(m *Manager) { m.operationID = value }
}
func WithDuration(value time.Duration) Option { return func(m *Manager) { m.duration = value } }

type Manager struct {
	root        string
	inspector   ProcessInspector
	now         func() time.Time
	operationID func() (string, error)
	duration    time.Duration
}

func NewManager(lockDir string, options ...Option) (*Manager, error) {
	if lockDir == "" {
		return nil, ErrUnsafeLockArtifact
	}
	absolute, err := filepath.Abs(lockDir)
	if err != nil || filepath.Clean(absolute) != absolute {
		return nil, ErrUnsafeLockArtifact
	}
	info, err := os.Lstat(absolute)
	if err != nil || !info.IsDir() {
		return nil, ErrUnsafeLockArtifact
	}
	if ok, err := safeEntry(absolute); err != nil || !ok {
		return nil, ErrUnsafeLockArtifact
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, ErrUnsafeLockArtifact
	}
	m := &Manager{root: filepath.Clean(resolved), inspector: WindowsProcessInspector{}, now: time.Now, operationID: randomID, duration: 5 * time.Minute}
	for _, option := range options {
		option(m)
	}
	if m.inspector == nil || m.now == nil || m.operationID == nil || m.duration <= 0 || m.duration > 24*time.Hour {
		return nil, ErrLockPersistence
	}
	return m, nil
}
func (m *Manager) AcquireGlobal() (*Guard, error) {
	return m.acquire(kindGlobal, "global", "global.lock")
}
func (m *Manager) AcquireFile(target string) (*Guard, error) {
	resource, err := fileResource(target)
	if err != nil {
		return nil, err
	}
	return m.acquire(kindFile, resource, "file-"+resource+".lock")
}
func (m *Manager) CheckFile(target string) error {
	resource, err := fileResource(target)
	if err != nil {
		return err
	}
	return m.check(kindFile, resource, "file-"+resource+".lock")
}

func (m *Manager) acquire(kind lockKind, resource, name string) (*Guard, error) {
	identity, err := m.inspector.Inspect(uint32(os.Getpid()))
	if err != nil {
		return nil, ErrLockOwnerUnverifiable
	}
	operation, err := m.operationID()
	if err != nil || !hex32.MatchString(operation) {
		return nil, ErrLockPersistence
	}
	created := m.now().UTC()
	value := record{SchemaVersion: 1, Kind: kind, ResourceID: resource, OwnerPID: identity.PID, OwnerStartTime: identity.StartTime, OwnerImage: identity.Image, OperationID: operation, CreatedAt: created.Format(time.RFC3339Nano), ExpiresAt: created.Add(m.duration).Format(time.RFC3339Nano)}
	payload, _ := encodeRecord(value)
	path := filepath.Join(m.root, name)
	for attempt := 0; attempt < 2; attempt++ {
		candidate := filepath.Join(m.root, "."+name+".candidate-"+operation)
		if err := writeCandidate(candidate, payload); err != nil {
			return nil, err
		}
		written, verifyErr := m.read(candidate)
		if verifyErr != nil || !bytes.Equal(written.bytes, payload) || written.record != value {
			_ = os.Remove(candidate)
			return nil, ErrLockPersistence
		}
		err = m.install(candidate, path)
		_ = os.Remove(candidate)
		if err == nil {
			return &Guard{manager: m, path: path, record: value, bytes: payload}, nil
		}
		existing, readErr := m.read(path)
		if readErr != nil {
			return nil, readErr
		}
		if existing.record.Kind != kind || existing.record.ResourceID != resource {
			return nil, ErrLockRecordInvalid
		}
		stale, inspectErr := m.stale(existing)
		if inspectErr != nil {
			return nil, inspectErr
		}
		if !stale {
			return nil, ErrLockHeld
		}
		again, readErr := m.read(path)
		if readErr != nil || !bytes.Equal(existing.bytes, again.bytes) {
			return nil, ErrLockOwnershipLost
		}
		stale, inspectErr = m.stale(again)
		if inspectErr != nil {
			return nil, inspectErr
		}
		if !stale {
			return nil, ErrLockHeld
		}
		if err := os.Remove(path); err != nil {
			return nil, ErrLockPersistence
		}
		if attempt == 1 {
			return nil, ErrLockPersistence
		}
	}
	return nil, ErrLockPersistence
}

type readRecord struct {
	record record
	bytes  []byte
}

func (m *Manager) read(path string) (readRecord, error) {
	exists, err := safeEntry(path)
	if err != nil {
		return readRecord{}, err
	}
	if !exists {
		return readRecord{}, ErrLockOwnershipLost
	}
	info, err := os.Lstat(path)
	if err != nil || info.IsDir() {
		return readRecord{}, ErrUnsafeLockArtifact
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return readRecord{}, ErrLockPersistence
	}
	value, err := decodeRecord(data)
	if err != nil {
		return readRecord{}, err
	}
	return readRecord{value, data}, nil
}
func (m *Manager) stale(value readRecord) (bool, error) {
	identity, err := m.inspector.Inspect(value.record.OwnerPID)
	if err != nil {
		if err == ErrProcessNotFound {
			return true, nil
		}
		return false, ErrLockOwnerUnverifiable
	}
	return !exactOwner(value.record, identity), nil
}
func (m *Manager) check(kind lockKind, resource, name string) error {
	path := filepath.Join(m.root, name)
	exists, err := safeEntry(path)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	value, err := m.read(path)
	if err != nil {
		return err
	}
	if value.record.Kind != kind || value.record.ResourceID != resource {
		return ErrLockRecordInvalid
	}
	stale, err := m.stale(value)
	if err != nil {
		return err
	}
	if stale {
		return nil
	}
	return ErrLockHeld
}
func (m *Manager) install(candidate, target string) error {
	from, err := windows.UTF16PtrFromString(candidate)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(from, to, windows.MOVEFILE_WRITE_THROUGH)
}

type Guard struct {
	manager  *Manager
	path     string
	record   record
	bytes    []byte
	mu       sync.Mutex
	released bool
}

func (g *Guard) Release() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.released {
		return nil
	}
	current, err := g.manager.read(g.path)
	if err != nil {
		return ErrLockOwnershipLost
	}
	if !bytes.Equal(current.bytes, g.bytes) || current.record.OperationID != g.record.OperationID {
		return ErrLockOwnershipLost
	}
	if err = os.Remove(g.path); err != nil {
		return ErrLockPersistence
	}
	g.released = true
	return nil
}

func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func writeCandidate(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return ErrLockPersistence
	}
	ok := false
	defer func() {
		_ = file.Close()
		if !ok {
			_ = os.Remove(path)
		}
	}()
	for len(data) > 0 {
		n, e := file.Write(data)
		if e != nil || n == 0 {
			return ErrLockPersistence
		}
		data = data[n:]
	}
	if file.Sync() != nil || file.Close() != nil {
		return ErrLockPersistence
	}
	ok = true
	return nil
}
