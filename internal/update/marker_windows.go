package update

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"unicode/utf8"

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
	InspectFile(string) error
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
		return transactionMarker{}, ErrRecoveryUnresolved
	}
	ok := false
	defer func() {
		_ = f.Close()
		if !ok {
			_ = os.Remove(candidate)
		}
	}()
	if _, err = f.Write(b); err != nil {
		return transactionMarker{}, ErrRecoveryUnresolved
	}
	if err = f.Sync(); err != nil {
		return transactionMarker{}, ErrRecoveryUnresolved
	}
	if err = f.Close(); err != nil {
		return transactionMarker{}, ErrRecoveryUnresolved
	}
	check, err := os.ReadFile(candidate)
	if err != nil || !bytes.Equal(check, b) {
		return transactionMarker{}, ErrRecoveryUnresolved
	}
	if _, err = decodeMarker(check); err != nil {
		return transactionMarker{}, err
	}
	if err = s.security.InspectFile(candidate); err != nil {
		return transactionMarker{}, ErrRecoveryUnresolved
	}
	from, e := windows.UTF16PtrFromString(candidate)
	if e != nil {
		return transactionMarker{}, ErrRecoveryUnresolved
	}
	to, e := windows.UTF16PtrFromString(s.path())
	if e != nil {
		return transactionMarker{}, ErrRecoveryUnresolved
	}
	if e = windows.MoveFileEx(from, to, windows.MOVEFILE_WRITE_THROUGH); e != nil {
		return transactionMarker{}, ErrRecoveryUnresolved
	}
	if e = s.security.InspectFile(s.path()); e != nil {
		return transactionMarker{}, ErrRecoveryUnresolved
	}
	ok = true
	return m, nil
}
func (s *markerStore) load() (transactionMarker, bool, error) {
	p := s.path()
	info, err := os.Lstat(p)
	if os.IsNotExist(err) {
		return transactionMarker{}, false, nil
	}
	if err != nil || info.IsDir() || info.Mode()&os.ModeSymlink != 0 || reparse(p) || info.Size() < 1 || info.Size() > maxMarkerBytes {
		return transactionMarker{}, false, ErrRecoveryUnresolved
	}
	if err = s.security.InspectDir(s.dir); err != nil {
		return transactionMarker{}, false, ErrRecoveryUnresolved
	}
	if err = s.security.InspectFile(p); err != nil {
		return transactionMarker{}, false, ErrRecoveryUnresolved
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return transactionMarker{}, false, ErrRecoveryUnresolved
	}
	m, err := decodeMarker(b)
	return m, true, err
}
func (s *markerStore) remove(want transactionMarker) error {
	got, exists, err := s.load()
	if err != nil || !exists || !reflect.DeepEqual(got, want) {
		return ErrRecoveryUnresolved
	}
	if err = s.security.InspectFile(s.path()); err != nil {
		return ErrRecoveryUnresolved
	}
	if err = os.Remove(s.path()); err != nil {
		return ErrRecoveryUnresolved
	}
	return nil
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
