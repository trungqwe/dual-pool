// Package secretstore stores only Poolbridge-owned local client and management keys.
package secretstore

import (
	"crypto/subtle"
	"runtime"
)

const MaxSecretSize = 512

// Store exposes exact operations for the closed Poolbridge secret-purpose registry.
type Store interface {
	Put(Purpose, []byte) error
	Get(Purpose) ([]byte, error)
	Delete(Purpose) error
}

type credentialStore struct {
	api     winCredAPI
	targets map[Purpose]string
}

// New returns the production Windows Credential Manager store. Its target registry is fixed.
func New() Store {
	return newCredentialStore(systemWinCredAPI{}, productionTargets())
}

func newCredentialStore(api winCredAPI, targets map[Purpose]string) *credentialStore {
	return &credentialStore{api: api, targets: targets}
}

func (s *credentialStore) Put(purpose Purpose, secret []byte) error {
	target, err := s.target(purpose)
	if err != nil {
		return err
	}
	if len(secret) == 0 || len(secret) > MaxSecretSize {
		return ErrInvalidSecret
	}

	writeCopy := append([]byte(nil), secret...)
	verifyCopy := append([]byte(nil), secret...)
	defer Zero(writeCopy)
	defer Zero(verifyCopy)

	if err := s.api.write(target, writeCopy); err != nil {
		return classifyPersistenceError(err)
	}
	Zero(writeCopy)

	actual, err := s.api.read(target)
	if err != nil {
		return classifyReadError(err)
	}
	defer Zero(actual)
	if len(actual) != len(verifyCopy) || subtle.ConstantTimeCompare(actual, verifyCopy) != 1 {
		return ErrVerificationFailed
	}
	return nil
}

func (s *credentialStore) Get(purpose Purpose) ([]byte, error) {
	target, err := s.target(purpose)
	if err != nil {
		return nil, err
	}
	secret, err := s.api.read(target)
	if err != nil {
		return nil, classifyReadError(err)
	}
	if len(secret) == 0 || len(secret) > MaxSecretSize {
		Zero(secret)
		return nil, ErrInvalidSecret
	}
	return secret, nil
}

func (s *credentialStore) Delete(purpose Purpose) error {
	target, err := s.target(purpose)
	if err != nil {
		return err
	}
	if err := s.api.delete(target); err != nil && !isNotFound(err) {
		return classifyPersistenceError(err)
	}
	return nil
}

func (s *credentialStore) target(purpose Purpose) (string, error) {
	target, ok := s.targets[purpose]
	if !ok {
		return "", ErrInvalidPurpose
	}
	return target, nil
}

// Zero overwrites a byte slice in place. This provides best-effort bounded plaintext lifetime;
// Go does not guarantee erasure of every historical compiler or runtime copy.
func Zero(value []byte) {
	for i := range value {
		value[i] = 0
	}
	runtime.KeepAlive(value)
}
