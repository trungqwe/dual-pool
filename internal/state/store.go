package state

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"

	"github.com/trungqwe/dual-pool/internal/lockfile"
)

var (
	ErrNotInitialized        = errors.New("state document is not initialized")
	ErrRecoveryRequired      = errors.New("state recovery is required")
	ErrRecoveryUnresolved    = errors.New("state recovery is unresolved")
	ErrRecoveryMarkerInvalid = errors.New("state recovery marker is invalid")
	ErrConcurrentDrift       = errors.New("state document changed concurrently")
	ErrPersistenceFailed     = errors.New("state persistence failed")
	ErrUnsafeArtifact        = errors.New("unsafe state artifact")
	ErrInjectedCrash         = errors.New("injected state-store crash")
	ErrLockProviderRequired  = errors.New("state mutation lock provider is required")
	ErrMutationInProgress    = errors.New("state mutation is in progress")
)

type FaultPoint string

const (
	AfterCandidateCreate  FaultPoint = "AFTER_CANDIDATE_CREATE"
	AfterCandidateWrite   FaultPoint = "AFTER_CANDIDATE_WRITE"
	AfterCandidateSync    FaultPoint = "AFTER_CANDIDATE_SYNC"
	AfterMarkerSync       FaultPoint = "AFTER_MARKER_SYNC"
	BeforeCAS             FaultPoint = "BEFORE_CAS"
	AfterCASBeforeReplace FaultPoint = "AFTER_CAS_BEFORE_REPLACE"
	AfterReplace          FaultPoint = "AFTER_REPLACE"
	AfterTargetVerify     FaultPoint = "AFTER_TARGET_VERIFY"
	AfterTargetSync       FaultPoint = "AFTER_TARGET_SYNC"
	BeforeBackupCleanup   FaultPoint = "BEFORE_BACKUP_CLEANUP"
	BeforeMarkerCleanup   FaultPoint = "BEFORE_MARKER_CLEANUP"
)

type faultInjector func(FaultPoint) error
type transactionIDGenerator func() (string, error)
type Option func(*Store)

func WithFaultInjector(injector func(FaultPoint) error) Option {
	return func(store *Store) { store.fault = injector }
}
func WithTransactionIDGenerator(generator func() (string, error)) Option {
	return func(store *Store) { store.transactionID = generator }
}
func WithLockManager(manager *lockfile.Manager) Option {
	return func(store *Store) { store.locks = manager }
}

type replacer interface {
	replaceExisting(target, candidate, backup string) error
	installNew(candidate, target string) error
}

type Store struct {
	dir           string
	replacer      replacer
	fault         faultInjector
	transactionID transactionIDGenerator
	locks         *lockfile.Manager
}

func NewStore(stateDir string, options ...Option) (*Store, error) {
	if stateDir == "" {
		return nil, ErrUnsafeArtifact
	}
	dir, err := filepath.Abs(stateDir)
	if err != nil || filepath.Clean(dir) != dir {
		return nil, ErrUnsafeArtifact
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || isReparsePoint(dir) {
		return nil, ErrUnsafeArtifact
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, ErrUnsafeArtifact
	}
	dir = filepath.Clean(resolved)
	store := &Store{dir: dir, replacer: windowsReplacer{}, transactionID: randomTransactionID}
	for _, option := range options {
		option(store)
	}
	if store.transactionID == nil || store.replacer == nil {
		return nil, ErrPersistenceFailed
	}
	return store, nil
}

func (store *Store) LoadState() (State, error) {
	data, err := store.load(documentState)
	if err != nil {
		return State{}, err
	}
	return DecodeState(data)
}
func (store *Store) LoadOwnership() (Ownership, error) {
	data, err := store.load(documentOwnership)
	if err != nil {
		return Ownership{}, err
	}
	return DecodeOwnership(data)
}
func (store *Store) SaveState(value State) error {
	data, err := EncodeState(value)
	if err != nil {
		return err
	}
	return store.save(documentState, data, func(b []byte) error {
		decoded, e := DecodeState(b)
		if e != nil || !reflect.DeepEqual(decoded, value) {
			return ErrInvalidDocument
		}
		return nil
	})
}
func (store *Store) SaveOwnership(value Ownership) error {
	data, err := EncodeOwnership(value)
	if err != nil {
		return err
	}
	return store.save(documentOwnership, data, func(b []byte) error {
		decoded, e := DecodeOwnership(b)
		if e != nil || !reflect.DeepEqual(decoded, value) {
			return ErrInvalidDocument
		}
		return nil
	})
}

func (store *Store) load(kind documentKind) ([]byte, error) {
	if err := store.safeDirectory(); err != nil {
		return nil, err
	}
	if exists, err := safeExists(store.markerPath(kind)); err != nil {
		return nil, err
	} else if exists {
		if store.locks != nil {
			err := store.locks.CheckFile(store.targetPath(kind))
			if errors.Is(err, lockfile.ErrLockHeld) || errors.Is(err, lockfile.ErrLockOwnerUnverifiable) {
				return nil, ErrMutationInProgress
			}
			if err != nil {
				return nil, err
			}
		}
		return nil, ErrRecoveryRequired
	}
	path := store.targetPath(kind)
	exists, err := safeExists(path)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotInitialized
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, ErrPersistenceFailed
	}
	if err := validateDocument(kind, data); err != nil {
		return nil, err
	}
	return data, nil
}

func (store *Store) save(kind documentKind, data []byte, semantic func([]byte) error) (result error) {
	if store.locks == nil {
		return ErrLockProviderRequired
	}
	guard, err := store.locks.AcquireFile(store.targetPath(kind))
	if err != nil {
		return err
	}
	defer func() {
		if releaseErr := guard.Release(); result == nil && releaseErr != nil {
			result = releaseErr
		}
	}()
	if err := store.safeDirectory(); err != nil {
		return err
	}
	markerPath := store.markerPath(kind)
	if exists, err := safeExists(markerPath); err != nil {
		return err
	} else if exists {
		return ErrRecoveryRequired
	}
	target := store.targetPath(kind)
	oldExists, err := safeExists(target)
	if err != nil {
		return err
	}
	oldHash := ""
	if oldExists {
		old, e := os.ReadFile(target)
		if e != nil {
			return ErrPersistenceFailed
		}
		if e = validateDocument(kind, old); e != nil {
			return e
		}
		oldHash = hashBytes(old)
	}
	id, err := store.transactionID()
	if err != nil || !transactionPattern.MatchString(id) {
		return ErrPersistenceFailed
	}
	prefix := "." + documentFilename(kind)
	candidateBase := prefix + ".tmp-" + id
	backupBase := ""
	if oldExists {
		backupBase = prefix + ".bak-" + id
	}
	candidate := filepath.Join(store.dir, candidateBase)
	backup := ""
	if backupBase != "" {
		backup = filepath.Join(store.dir, backupBase)
	}
	file, err := os.OpenFile(candidate, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return ErrPersistenceFailed
	}
	if err = store.inject(AfterCandidateCreate); err != nil {
		_ = file.Close()
		return err
	}
	if err = writeAll(file, data); err != nil {
		_ = file.Close()
		_ = os.Remove(candidate)
		return ErrPersistenceFailed
	}
	if err = store.inject(AfterCandidateWrite); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(candidate)
		return ErrPersistenceFailed
	}
	if err = store.inject(AfterCandidateSync); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		_ = os.Remove(candidate)
		return ErrPersistenceFailed
	}
	check, err := os.ReadFile(candidate)
	if err != nil || semantic(check) != nil || hashBytes(check) != hashBytes(data) {
		_ = os.Remove(candidate)
		return ErrPersistenceFailed
	}
	marker := recoveryMarker{SchemaVersion: markerSchemaVersion, DocumentKind: kind, TransactionID: id, OldExists: oldExists, OldSHA256: oldHash, NewSHA256: hashBytes(data), CandidateBasename: candidateBase, BackupBasename: backupBase}
	markerData, _ := encodeMarker(marker)
	if err = writeSyncedExclusive(markerPath, markerData); err != nil {
		_ = os.Remove(candidate)
		_ = os.Remove(markerPath)
		return ErrPersistenceFailed
	}
	if err = store.inject(AfterMarkerSync); err != nil {
		return err
	}
	if err = store.inject(BeforeCAS); err != nil {
		return err
	}
	if err = verifyCAS(target, oldExists, oldHash); err != nil {
		return err
	}
	if err = store.inject(AfterCASBeforeReplace); err != nil {
		return err
	}
	if oldExists {
		err = store.replacer.replaceExisting(target, candidate, backup)
	} else {
		err = store.replacer.installNew(candidate, target)
	}
	if err != nil {
		return ErrPersistenceFailed
	}
	if err = store.inject(AfterReplace); err != nil {
		return err
	}
	final, err := os.ReadFile(target)
	if err != nil || hashBytes(final) != marker.NewSHA256 || semantic(final) != nil {
		return ErrRecoveryUnresolved
	}
	if err = store.inject(AfterTargetVerify); err != nil {
		return err
	}
	targetFile, err := os.OpenFile(target, os.O_RDWR, 0)
	if err != nil {
		return ErrPersistenceFailed
	}
	syncErr := targetFile.Sync()
	closeErr := targetFile.Close()
	if syncErr != nil || closeErr != nil {
		return ErrPersistenceFailed
	}
	if err = store.inject(AfterTargetSync); err != nil {
		return err
	}
	if err = store.inject(BeforeBackupCleanup); err != nil {
		return err
	}
	if backup != "" {
		if err = os.Remove(backup); err != nil && !os.IsNotExist(err) {
			return ErrPersistenceFailed
		}
	}
	if err = store.inject(BeforeMarkerCleanup); err != nil {
		return err
	}
	if err = os.Remove(markerPath); err != nil {
		return ErrPersistenceFailed
	}
	return nil
}

func (store *Store) inject(point FaultPoint) error {
	if store.fault == nil {
		return nil
	}
	if err := store.fault(point); err != nil {
		return err
	}
	return nil
}
func (store *Store) targetPath(kind documentKind) string {
	return filepath.Join(store.dir, documentFilename(kind))
}
func (store *Store) markerPath(kind documentKind) string {
	return filepath.Join(store.dir, "."+documentFilename(kind)+".recovery")
}
func documentFilename(kind documentKind) string {
	if kind == documentState {
		return "state.json"
	}
	if kind == documentOwnership {
		return "ownership.json"
	}
	return ""
}
func validateDocument(kind documentKind, data []byte) error {
	if kind == documentState {
		_, err := DecodeState(data)
		return err
	}
	if kind == documentOwnership {
		_, err := DecodeOwnership(data)
		return err
	}
	return ErrInvalidDocument
}
func hashBytes(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func randomTransactionID() (string, error) {
	b := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func writeAll(file *os.File, data []byte) error {
	for len(data) > 0 {
		n, err := file.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}
func writeSyncedExclusive(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if err = writeAll(file, data); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
func verifyCAS(path string, oldExists bool, oldHash string) error {
	exists, err := safeExists(path)
	if err != nil {
		return err
	}
	if !oldExists {
		if exists {
			return ErrConcurrentDrift
		}
		return nil
	}
	if !exists {
		return ErrConcurrentDrift
	}
	data, err := os.ReadFile(path)
	if err != nil || hashBytes(data) != oldHash {
		return ErrConcurrentDrift
	}
	return nil
}
func safeExists(path string) (bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, ErrPersistenceFailed
	}
	if info.Mode()&os.ModeSymlink != 0 || isReparsePoint(path) {
		return false, ErrUnsafeArtifact
	}
	return true, nil
}
func (store *Store) safeDirectory() error {
	info, err := os.Lstat(store.dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || isReparsePoint(store.dir) {
		return ErrUnsafeArtifact
	}
	return nil
}
