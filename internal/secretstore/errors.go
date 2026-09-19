package secretstore

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

var (
	ErrInvalidPurpose     = errors.New("secret store: invalid purpose")
	ErrInvalidSecret      = errors.New("secret store: invalid secret")
	ErrNotFound           = errors.New("secret store: credential not found")
	ErrUnavailable        = errors.New("secret store: credential manager unavailable")
	ErrPersistence        = errors.New("secret store: credential operation failed")
	ErrVerificationFailed = errors.New("secret store: write verification failed")
)

func classifyReadError(err error) error {
	if isNotFound(err) {
		return errors.Join(ErrNotFound, err)
	}
	return classifyPersistenceError(err)
}

func classifyPersistenceError(err error) error {
	if errors.Is(err, windows.ERROR_NO_SUCH_LOGON_SESSION) {
		return errors.Join(ErrUnavailable, err)
	}
	return fmt.Errorf("%w: %w", ErrPersistence, err)
}

func isNotFound(err error) bool {
	return errors.Is(err, windows.ERROR_NOT_FOUND)
}
