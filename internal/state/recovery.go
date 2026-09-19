package state

import "os"

type recoveryResult string

const (
	recoveryNone       recoveryResult = "NO_OP"
	recoveryCommitted  recoveryResult = "COMMITTED_RECOVERED"
	recoveryAborted    recoveryResult = "ABORTED_RECOVERED"
	recoveryRolledBack recoveryResult = "ROLLED_BACK_RECOVERED"
)

func (store *Store) Recover() error {
	if store.locks == nil {
		return ErrLockProviderRequired
	}
	for _, kind := range []documentKind{documentState, documentOwnership} {
		if _, err := store.recoverOne(kind); err != nil {
			return err
		}
	}
	return nil
}

func (store *Store) recoverOne(kind documentKind) (recoveryResult, error) {
	if store.locks == nil {
		return recoveryNone, ErrLockProviderRequired
	}
	guard, err := store.locks.AcquireFile(store.targetPath(kind))
	if err != nil {
		return recoveryNone, err
	}
	if err := store.safeDirectory(); err != nil {
		_ = guard.Release()
		return recoveryNone, err
	}
	result, recoverErr := store.recoverOneLocked(kind)
	if releaseErr := guard.Release(); recoverErr == nil && releaseErr != nil {
		return recoveryNone, releaseErr
	}
	return result, recoverErr
}

func (store *Store) recoverOneLocked(kind documentKind) (recoveryResult, error) {
	markerPath := store.markerPath(kind)
	exists, err := safeExists(markerPath)
	if err != nil {
		return recoveryNone, err
	}
	if !exists {
		return recoveryNone, store.cleanupOrphanCandidates(kind)
	}
	markerBytes, err := os.ReadFile(markerPath)
	if err != nil {
		return recoveryNone, ErrRecoveryMarkerInvalid
	}
	marker, err := decodeMarker(markerBytes, kind)
	if err != nil {
		return recoveryNone, err
	}
	target := store.targetPath(kind)
	if _, err := safeExists(target); err != nil {
		return recoveryNone, err
	}
	candidate := store.ownedArtifactPath(marker.CandidateBasename)
	if candidate == "" {
		return recoveryNone, ErrRecoveryMarkerInvalid
	}
	backup := ""
	if marker.BackupBasename != "" {
		backup = store.ownedArtifactPath(marker.BackupBasename)
		if backup == "" {
			return recoveryNone, ErrRecoveryMarkerInvalid
		}
	}
	if _, err := safeExists(candidate); err != nil {
		return recoveryNone, err
	}
	if backup != "" {
		if _, err := safeExists(backup); err != nil {
			return recoveryNone, err
		}
	}
	targetExists, targetValid, targetHash := inspectDocument(target, kind)
	if targetExists && targetValid && targetHash == marker.NewSHA256 {
		if marker.OldExists && backup != "" {
			if exists, err := safeExists(backup); err != nil {
				return recoveryNone, err
			} else if exists && !matchingDocument(backup, kind, marker.OldSHA256) {
				return recoveryNone, ErrRecoveryUnresolved
			}
		}
		if err := removeOwned(candidate); err != nil {
			return recoveryNone, err
		}
		if backup != "" {
			if err := removeOwned(backup); err != nil {
				return recoveryNone, err
			}
		}
		if err := os.Remove(markerPath); err != nil {
			return recoveryNone, ErrPersistenceFailed
		}
		return recoveryCommitted, nil
	}
	if marker.OldExists && targetExists && targetValid && targetHash == marker.OldSHA256 {
		if backup != "" {
			if exists, err := safeExists(backup); err != nil {
				return recoveryNone, err
			} else if exists && !matchingDocument(backup, kind, marker.OldSHA256) {
				return recoveryNone, ErrRecoveryUnresolved
			}
		}
		if err := removeOwned(candidate); err != nil {
			return recoveryNone, err
		}
		if backup != "" {
			if err := removeOwned(backup); err != nil {
				return recoveryNone, err
			}
		}
		if err := os.Remove(markerPath); err != nil {
			return recoveryNone, ErrPersistenceFailed
		}
		return recoveryAborted, nil
	}
	if !marker.OldExists && !targetExists {
		if err := removeOwned(candidate); err != nil {
			return recoveryNone, err
		}
		if err := os.Remove(markerPath); err != nil {
			return recoveryNone, ErrPersistenceFailed
		}
		return recoveryAborted, nil
	}
	if targetExists && targetValid {
		return recoveryNone, ErrConcurrentDrift
	}
	if marker.OldExists && backup != "" && matchingDocument(backup, kind, marker.OldSHA256) {
		if targetExists {
			err = store.replacer.replaceExisting(target, backup, "")
		} else {
			err = store.replacer.installNew(backup, target)
		}
		if err != nil {
			return recoveryNone, ErrPersistenceFailed
		}
		data, readErr := os.ReadFile(target)
		if readErr != nil || validateDocument(kind, data) != nil || hashBytes(data) != marker.OldSHA256 {
			return recoveryNone, ErrRecoveryUnresolved
		}
		file, openErr := os.OpenFile(target, os.O_RDWR, 0)
		if openErr != nil {
			return recoveryNone, ErrPersistenceFailed
		}
		syncErr := file.Sync()
		closeErr := file.Close()
		if syncErr != nil || closeErr != nil {
			return recoveryNone, ErrPersistenceFailed
		}
		if err := removeOwned(candidate); err != nil {
			return recoveryNone, err
		}
		if err := os.Remove(markerPath); err != nil {
			return recoveryNone, ErrPersistenceFailed
		}
		return recoveryRolledBack, nil
	}
	return recoveryNone, ErrRecoveryUnresolved
}

func (store *Store) ownedArtifactPath(base string) string {
	if base == "" || base != filepathBase(base) {
		return ""
	}
	return store.dir + string(os.PathSeparator) + base
}
func filepathBase(value string) string {
	for i := len(value) - 1; i >= 0; i-- {
		if value[i] == '/' || value[i] == '\\' {
			return value[i+1:]
		}
	}
	return value
}
func inspectDocument(path string, kind documentKind) (bool, bool, string) {
	exists, err := safeExists(path)
	if err != nil || !exists {
		return exists, false, ""
	}
	data, err := os.ReadFile(path)
	if err != nil || validateDocument(kind, data) != nil {
		return true, false, ""
	}
	return true, true, hashBytes(data)
}
func matchingDocument(path string, kind documentKind, want string) bool {
	exists, valid, hash := inspectDocument(path, kind)
	return exists && valid && hash == want
}
func removeOwned(path string) error {
	if path == "" {
		return nil
	}
	exists, err := safeExists(path)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if err = os.Remove(path); err != nil {
		return ErrPersistenceFailed
	}
	return nil
}

func (store *Store) cleanupOrphanCandidates(kind documentKind) error {
	entries, err := os.ReadDir(store.dir)
	if err != nil {
		return ErrPersistenceFailed
	}
	prefix := "." + documentFilename(kind) + ".tmp-"
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || len(name) != len(prefix)+32 || name[:len(prefix)] != prefix || !transactionPattern.MatchString(name[len(prefix):]) {
			continue
		}
		if err := removeOwned(store.ownedArtifactPath(name)); err != nil {
			return err
		}
	}
	return nil
}
