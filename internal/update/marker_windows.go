package update

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"time"
	"unicode/utf8"
	"unsafe"

	"github.com/trungqwe/dual-pool/internal/state"
	"golang.org/x/sys/windows"
)

const markerName = ".update-transaction.json"
const maxMarkerBytes = 4096

type transactionMarker struct {
	SchemaVersion    int          `json:"schema_version"`
	TransactionID    string       `json:"transaction_id"`
	PreviousVersion  string       `json:"previous_version"`
	CandidateVersion string       `json:"candidate_version"`
	BaseStateSHA256  string       `json:"base_state_sha256"`
	RestartPools     []state.Pool `json:"restart_pools"`
}
type markerStore struct {
	dir      string
	id       func() (string, error)
	security MarkerSecurity
}

type MarkerSecurity interface {
	InspectDir(string) error
	CreateFile(string) (*os.File, error)
	InspectHandle(*os.File) error
}

func newMarkerStore(dir string, id func() (string, error), security MarkerSecurity) (*markerStore, error) {
	abs, err := filepath.Abs(dir)
	if dir == "" || err != nil || filepath.Clean(abs) != abs {
		return nil, ErrRecoveryUnresolved
	}
	info, err := os.Lstat(abs)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || reparse(abs) {
		return nil, ErrRecoveryUnresolved
	}
	if security == nil || security.InspectDir(abs) != nil {
		return nil, ErrRecoveryUnresolved
	}
	if id == nil {
		id = randomID
	}
	return &markerStore{dir: abs, id: id, security: security}, nil
}
func (s *markerStore) path() string { return filepath.Join(s.dir, markerName) }
func (s *markerStore) publish(m transactionMarker) (transactionMarker, error) {
	id, err := s.id()
	if err != nil || !validID(id) {
		return transactionMarker{}, ErrRecoveryUnresolved
	}
	m.TransactionID = id
	b, err := encodeMarker(m)
	if err != nil {
		return transactionMarker{}, err
	}
	candidate := filepath.Join(s.dir, "."+markerName+".candidate-"+id)
	f, err := s.security.CreateFile(candidate)
	if err != nil {
		return transactionMarker{}, markerFailure("marker_create", err)
	}
	ok := false
	defer func() {
		if !ok {
			_ = deleteMarkerByHandle(f)
		}
		_ = f.Close()
	}()
	if _, err = f.Write(b); err != nil {
		return transactionMarker{}, markerFailure("marker_write", err)
	}
	if err = f.Sync(); err != nil {
		return transactionMarker{}, markerFailure("marker_sync", err)
	}
	if err = s.security.InspectHandle(f); err != nil {
		return transactionMarker{}, markerFailure("marker_post_create_acl", err)
	}
	check, err := readMarkerHandle(f)
	if err != nil || !bytes.Equal(check, b) {
		return transactionMarker{}, markerFailure("marker_readback", err)
	}
	if _, err = decodeMarker(check); err != nil {
		return transactionMarker{}, err
	}
	if err = renameMarkerByHandle(f, s.dir, markerName); err != nil {
		return transactionMarker{}, markerFailure("marker_rename", err)
	}
	if err = s.security.InspectHandle(f); err != nil {
		return transactionMarker{}, markerFailure("marker_post_rename_acl", err)
	}
	if err = f.Close(); err != nil {
		return transactionMarker{}, markerFailure("marker_close", err)
	}
	ok = true
	return m, nil
}
func (s *markerStore) load() (transactionMarker, bool, error) {
	file, marker, exists, err := s.loadHandle()
	if file != nil {
		_ = file.Close()
	}
	return marker, exists, err
}
func (s *markerStore) remove(want transactionMarker) error {
	file, got, exists, err := s.loadHandle()
	if err != nil || !exists || !reflect.DeepEqual(got, want) {
		if file != nil {
			_ = file.Close()
		}
		return ErrRecoveryUnresolved
	}
	defer file.Close()
	if err = deleteMarkerByHandle(file); err != nil {
		return ErrRecoveryUnresolved
	}
	return nil
}

func (s *markerStore) loadHandle() (*os.File, transactionMarker, bool, error) {
	if err := s.security.InspectDir(s.dir); err != nil {
		return nil, transactionMarker{}, false, ErrRecoveryUnresolved
	}
	file, err := openMarker(s.path())
	if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND) {
		return nil, transactionMarker{}, false, nil
	}
	if err != nil {
		return nil, transactionMarker{}, false, markerFailure("marker_open", err)
	}
	if err = s.security.InspectHandle(file); err != nil {
		_ = file.Close()
		return nil, transactionMarker{}, false, markerFailure("marker_load_acl", err)
	}
	b, err := readMarkerHandle(file)
	if err != nil {
		_ = file.Close()
		return nil, transactionMarker{}, false, markerFailure("marker_load_read", err)
	}
	marker, err := decodeMarker(b)
	if err != nil {
		_ = file.Close()
		return nil, transactionMarker{}, false, err
	}
	return file, marker, true, nil
}

// markerFailure intentionally includes only a stable operation label and the
// underlying errno. It never includes a filesystem path, SID, marker bytes, or
// other sensitive state, while preserving the public fail-closed contract.
func markerFailure(operation string, cause error) error {
	if cause == nil {
		return fmt.Errorf("%s: %w", operation, ErrRecoveryUnresolved)
	}
	return fmt.Errorf("%s: %w: %w", operation, ErrRecoveryUnresolved, cause)
}

func openMarker(path string) (*os.File, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, ErrRecoveryUnresolved
	}
	deadline := time.Now().Add(250 * time.Millisecond)
	var h windows.Handle
	for {
		h, err = windows.CreateFile(p, windows.GENERIC_READ|windows.READ_CONTROL|windows.DELETE, 0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
		if !errors.Is(err, windows.ERROR_ACCESS_DENIED) || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(h), path), nil
}

func readMarkerHandle(file *os.File) ([]byte, error) {
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maxMarkerBytes {
		return nil, ErrRecoveryUnresolved
	}
	b := make([]byte, info.Size())
	n, err := file.ReadAt(b, 0)
	if (err != nil && !errors.Is(err, io.EOF)) || n != len(b) {
		return nil, ErrRecoveryUnresolved
	}
	return b, nil
}

func deleteMarkerByHandle(file *os.File) error {
	deleteFile := uint32(1)
	return windows.SetFileInformationByHandle(windows.Handle(file.Fd()), windows.FileDispositionInfo, (*byte)(unsafe.Pointer(&deleteFile)), uint32(unsafe.Sizeof(deleteFile)))
}

type fileRenameInformation struct {
	ReplaceIfExists byte
	RootDirectory   windows.Handle
	FileNameLength  uint32
	FileName        [1]uint16
}

func renameMarkerByHandle(file *os.File, directory, targetName string) error {
	directoryName, err := windows.UTF16PtrFromString(directory)
	if err != nil {
		return err
	}
	directoryHandle, err := windows.CreateFile(directoryName, windows.FILE_LIST_DIRECTORY, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(directoryHandle)
	name, err := windows.UTF16FromString(targetName)
	if err != nil {
		return err
	}
	name = name[:len(name)-1]
	size := unsafe.Offsetof(fileRenameInformation{}.FileName) + uintptr(len(name))*unsafe.Sizeof(name[0])
	buffer := make([]byte, size)
	info := (*fileRenameInformation)(unsafe.Pointer(&buffer[0]))
	info.RootDirectory = directoryHandle
	info.FileNameLength = uint32(len(name) * 2)
	copy(unsafe.Slice(&info.FileName[0], len(name)), name)
	deadline := time.Now().Add(250 * time.Millisecond)
	for {
		err = windows.SetFileInformationByHandle(windows.Handle(file.Fd()), windows.FileRenameInfo, &buffer[0], uint32(len(buffer)))
		if !errors.Is(err, windows.ERROR_ACCESS_DENIED) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(time.Millisecond)
	}
}
func encodeMarker(m transactionMarker) ([]byte, error) {
	if !validMarker(m) {
		return nil, ErrRecoveryUnresolved
	}
	return json.Marshal(m)
}
func decodeMarker(b []byte) (transactionMarker, error) {
	if len(b) == 0 || len(b) > maxMarkerBytes || !utf8.Valid(b) {
		return transactionMarker{}, ErrRecoveryUnresolved
	}
	d := json.NewDecoder(bytes.NewReader(b))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return transactionMarker{}, ErrRecoveryUnresolved
	}
	seen := map[string]bool{}
	required := map[string]bool{"schema_version": true, "transaction_id": true, "previous_version": true, "candidate_version": true, "base_state_sha256": true, "restart_pools": true}
	for d.More() {
		name, err := d.Token()
		if err != nil {
			return transactionMarker{}, ErrRecoveryUnresolved
		}
		key, ok := name.(string)
		if !ok || !required[key] || seen[key] {
			return transactionMarker{}, ErrRecoveryUnresolved
		}
		seen[key] = true
		var ignored json.RawMessage
		if err = d.Decode(&ignored); err != nil {
			return transactionMarker{}, ErrRecoveryUnresolved
		}
	}
	if token, err = d.Token(); err != nil || token != json.Delim('}') {
		return transactionMarker{}, ErrRecoveryUnresolved
	}
	if err = ensureEOF(d); err != nil {
		return transactionMarker{}, ErrRecoveryUnresolved
	}
	if len(seen) != len(required) {
		return transactionMarker{}, ErrRecoveryUnresolved
	}
	var m transactionMarker
	if err = json.Unmarshal(b, &m); err != nil || !validMarker(m) {
		return transactionMarker{}, ErrRecoveryUnresolved
	}
	return m, nil
}
func ensureEOF(d *json.Decoder) error {
	var v any
	err := d.Decode(&v)
	if !errors.Is(err, io.EOF) {
		return errors.New("trailing marker data")
	}
	return nil
}
func validMarker(m transactionMarker) bool {
	if m.SchemaVersion != 1 || !validID(m.TransactionID) || !validLogicalVersion(m.PreviousVersion) || !validLogicalVersion(m.CandidateVersion) || m.PreviousVersion == m.CandidateVersion || len(m.BaseStateSHA256) != 64 {
		return false
	}
	for _, r := range m.BaseStateSHA256 {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return len(m.RestartPools) == len(normalizePools(m.RestartPools))
}
func validID(id string) bool {
	if len(id) != 32 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil && id == stringsLower(id)
}
func stringsLower(v string) string {
	for _, r := range v {
		if r >= 'A' && r <= 'F' {
			return ""
		}
	}
	return v
}
func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func reparse(path string) bool {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return true
	}
	a, err := windows.GetFileAttributes(p)
	return err != nil || a&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
}
