package lockfile

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"

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
	if ok, entryErr := safeEntry(absolute); entryErr != nil || !ok {
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
	payload, err := encodeRecord(value)
	if err != nil {
		return nil, ErrLockPersistence
	}
	path := filepath.Join(m.root, name)
	contended := false
	for attempt := 0; attempt < 4; attempt++ {
		candidate := filepath.Join(m.root, "."+name+".candidate-"+operation)
		if err = writeCandidate(candidate, payload); err != nil {
			return nil, err
		}
		written, verifyErr := readPath(candidate)
		if verifyErr != nil || !bytes.Equal(written.bytes, payload) || written.record != value {
			_ = os.Remove(candidate)
			return nil, ErrLockPersistence
		}
		err = m.install(candidate, path)
		_ = os.Remove(candidate) // only this attempt's non-authoritative candidate
		if err == nil {
			file, openErr := openInstalledCanonical(path)
			if errors.Is(openErr, errCanonicalGone) && attempt < 3 {
				continue
			}
			if openErr != nil {
				return nil, openErr
			}
			current, readErr := readHandle(file)
			if readErr != nil || !bytes.Equal(current.bytes, payload) || current.record != value {
				_ = file.Close()
				return nil, ErrLockOwnershipLost
			}
			return &Guard{path: path, record: value, bytes: payload, file: file}, nil
		}
		if !errors.Is(err, windows.ERROR_ALREADY_EXISTS) && !errors.Is(err, windows.ERROR_FILE_EXISTS) {
			return nil, ErrLockPersistence
		}
		contended = true

		file, openErr := openCanonical(path)
		if errors.Is(openErr, errCanonicalGone) {
			if attempt < 3 {
				continue
			}
			return nil, ErrLockHeld
		}
		if openErr != nil {
			return nil, openErr
		}
		existing, readErr := readHandle(file)
		if readErr != nil {
			_ = file.Close()
			return nil, readErr
		}
		if existing.record.Kind != kind || existing.record.ResourceID != resource {
			_ = file.Close()
			return nil, ErrLockRecordInvalid
		}
		stale, inspectErr := m.stale(existing)
		if inspectErr != nil {
			_ = file.Close()
			return nil, inspectErr
		}
		if !stale {
			_ = file.Close()
			return nil, ErrLockHeld
		}
		stale, inspectErr = m.stale(existing)
		if inspectErr != nil {
			_ = file.Close()
			return nil, inspectErr
		}
		if !stale {
			_ = file.Close()
			return nil, ErrLockHeld
		}
		if err = deleteByHandle(file); err != nil {
			_ = file.Close()
			return nil, err
		}
		if err = file.Close(); err != nil {
			return nil, ErrLockPersistence
		}
	}
	if contended {
		return nil, ErrLockHeld
	}
	return nil, ErrLockPersistence
}

type readRecord struct {
	record record
	bytes  []byte
}

func readPath(path string) (readRecord, error) {
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

func readHandle(file *os.File) (readRecord, error) {
	info, err := file.Stat()
	if err != nil || info.IsDir() || info.Size() <= 0 || info.Size() > maxRecordBytes {
		return readRecord{}, ErrLockRecordInvalid
	}
	data := make([]byte, int(info.Size()))
	n, err := file.ReadAt(data, 0)
	if (err != nil && !errors.Is(err, io.EOF)) || n != len(data) {
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
		if errors.Is(err, ErrProcessNotFound) {
			return true, nil
		}
		return false, ErrLockOwnerUnverifiable
	}
	return !exactOwner(value.record, identity), nil
}

func (m *Manager) check(kind lockKind, resource, name string) error {
	file, err := openCanonical(filepath.Join(m.root, name))
	if errors.Is(err, errCanonicalGone) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	value, err := readHandle(file)
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

var errCanonicalGone = errors.New("canonical lock disappeared")

func openCanonical(path string) (*os.File, error) {
	entry, err := os.Lstat(path)
	if os.IsNotExist(err) || errors.Is(err, windows.ERROR_DELETE_PENDING) {
		return nil, errCanonicalGone
	}
	if err != nil {
		return nil, ErrLockPersistence
	}
	if entry.IsDir() || entry.Mode()&os.ModeSymlink != 0 {
		return nil, ErrUnsafeLockArtifact
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, ErrUnsafeLockArtifact
	}
	attributes, err := windows.GetFileAttributes(name)
	if err != nil {
		if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND) || errors.Is(err, windows.ERROR_DELETE_PENDING) {
			return nil, errCanonicalGone
		}
		return nil, ErrLockPersistence
	}
	if attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || attributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
		return nil, ErrUnsafeLockArtifact
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.DELETE, 0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		if errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, ErrLockHeld
		}
		if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND) || errors.Is(err, windows.ERROR_DELETE_PENDING) {
			return nil, errCanonicalGone
		}
		return nil, ErrLockPersistence
	}
	var info windows.ByHandleFileInformation
	if err = windows.GetFileInformationByHandle(handle, &info); err != nil || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
		_ = windows.CloseHandle(handle)
		return nil, ErrUnsafeLockArtifact
	}
	return os.NewFile(uintptr(handle), path), nil
}

func openInstalledCanonical(path string) (*os.File, error) {
	deadline := time.Now().Add(250 * time.Millisecond)
	for {
		file, err := openCanonical(path)
		if !errors.Is(err, ErrLockHeld) || time.Now().After(deadline) {
			return file, err
		}
		time.Sleep(time.Millisecond)
	}
}

func deleteByHandle(file *os.File) error {
	deleteFile := uint32(1)
	if err := windows.SetFileInformationByHandle(windows.Handle(file.Fd()), windows.FileDispositionInfo, (*byte)(unsafe.Pointer(&deleteFile)), uint32(unsafe.Sizeof(deleteFile))); err != nil {
		return ErrLockPersistence
	}
	return nil
}

type Guard struct {
	path     string
	record   record
	bytes    []byte
	file     *os.File
	mu       sync.Mutex
	released bool
}

func (g *Guard) Release() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.released {
		return nil
	}
	current, err := readHandle(g.file)
	if err != nil || !bytes.Equal(current.bytes, g.bytes) || current.record != g.record {
		_ = g.file.Close()
		g.released = true
		return ErrLockOwnershipLost
	}
	if err = deleteByHandle(g.file); err != nil {
		_ = g.file.Close()
		g.released = true
		return err
	}
	if err = g.file.Close(); err != nil {
		g.released = true
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

func writeCandidate(candidate string, data []byte) error {
	file, err := os.OpenFile(candidate, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return ErrLockPersistence
	}
	ok := false
	defer func() {
		_ = file.Close()
		if !ok {
			_ = os.Remove(candidate)
		}
	}()
	for len(data) > 0 {
		n, writeErr := file.Write(data)
		if writeErr != nil || n == 0 {
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
