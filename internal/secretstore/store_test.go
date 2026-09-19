package secretstore

import (
	"bytes"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

type fakeWinCredAPI struct {
	values      map[string][]byte
	writeErr    error
	readErr     error
	deleteErr   error
	readValue   []byte
	writeArg    []byte
	writeCalls  int
	readCalls   int
	deleteCalls int
}

func newFakeAPI() *fakeWinCredAPI {
	return &fakeWinCredAPI{values: make(map[string][]byte)}
}

func (f *fakeWinCredAPI) write(target string, secret []byte) error {
	f.writeCalls++
	f.writeArg = secret
	if f.writeErr != nil {
		return f.writeErr
	}
	f.values[target] = append([]byte(nil), secret...)
	return nil
}

func (f *fakeWinCredAPI) read(target string) ([]byte, error) {
	f.readCalls++
	if f.readErr != nil {
		return nil, f.readErr
	}
	if f.readValue != nil {
		return append([]byte(nil), f.readValue...), nil
	}
	value, ok := f.values[target]
	if !ok {
		return nil, windows.ERROR_NOT_FOUND
	}
	return append([]byte(nil), value...), nil
}

func (f *fakeWinCredAPI) delete(target string) error {
	f.deleteCalls++
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if _, ok := f.values[target]; !ok {
		return windows.ERROR_NOT_FOUND
	}
	delete(f.values, target)
	return nil
}

func TestClosedPurposeRegistry(t *testing.T) {
	targets := productionTargets()
	want := map[Purpose]string{
		CodexClientKey:      "dualpool:v1:codex:client-key",
		CodexManagementKey:  "dualpool:v1:codex:management-key",
		GoogleClientKey:     "dualpool:v1:google:client-key",
		GoogleManagementKey: "dualpool:v1:google:management-key",
	}
	if len(targets) != len(want) {
		t.Fatalf("target count = %d, want %d", len(targets), len(want))
	}
	for purpose, target := range want {
		if targets[purpose] != target {
			t.Fatalf("target for %q differs", purpose)
		}
	}

	api := newFakeAPI()
	store := newCredentialStore(api, targets)
	if err := store.Put(Purpose("unknown"), randomBytes(t, 32)); !errors.Is(err, ErrInvalidPurpose) {
		t.Fatalf("unknown purpose: %v", err)
	}
	if api.writeCalls+api.readCalls+api.deleteCalls != 0 {
		t.Fatal("unknown purpose reached credential API")
	}
}

func TestPutValidatesCopiesWipesAndVerifies(t *testing.T) {
	api := newFakeAPI()
	store := newCredentialStore(api, productionTargets())

	for name, secret := range map[string][]byte{
		"nil":       nil,
		"empty":     {},
		"oversized": make([]byte, MaxSecretSize+1),
	} {
		t.Run(name, func(t *testing.T) {
			if err := store.Put(CodexClientKey, secret); !errors.Is(err, ErrInvalidSecret) {
				t.Fatalf("Put error = %v", err)
			}
		})
	}
	if api.writeCalls != 0 {
		t.Fatal("invalid secret reached credential API")
	}

	secret := randomBytes(t, 48)
	original := append([]byte(nil), secret...)
	if err := store.Put(CodexClientKey, secret); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(secret, original) {
		t.Fatal("Put mutated caller secret")
	}
	if !allZero(api.writeArg) {
		t.Fatal("Put did not wipe internal write copy")
	}

	api.readValue = randomBytes(t, len(secret))
	if bytes.Equal(api.readValue, secret) {
		t.Fatal("random mismatch unexpectedly equal")
	}
	if err := store.Put(CodexClientKey, secret); !errors.Is(err, ErrVerificationFailed) {
		t.Fatalf("mismatch error = %v", err)
	}
}

func TestReplacementGetFreshCopyAndIdempotentDelete(t *testing.T) {
	api := newFakeAPI()
	store := newCredentialStore(api, productionTargets())
	first := randomBytes(t, 32)
	second := randomBytes(t, 32)

	if err := store.Put(GoogleManagementKey, first); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(GoogleManagementKey, second); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(GoogleManagementKey)
	if err != nil {
		t.Fatal(err)
	}
	defer Zero(got)
	if !bytes.Equal(got, second) || bytes.Equal(got, first) {
		t.Fatal("replacement did not retain only the new value")
	}
	got[0] ^= 0xff
	again, err := store.Get(GoogleManagementKey)
	if err != nil {
		t.Fatal(err)
	}
	defer Zero(again)
	if !bytes.Equal(again, second) {
		t.Fatal("Get did not return a fresh copy")
	}
	if err := store.Delete(GoogleManagementKey); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(GoogleManagementKey); err != nil {
		t.Fatalf("idempotent delete: %v", err)
	}
	if _, err := store.Get(GoogleManagementKey); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing Get: %v", err)
	}
}

func TestBackendFailuresAreClassifiedWithoutSecretLeak(t *testing.T) {
	secret := randomBytes(t, 40)
	secretText := string(secret)
	tests := []struct {
		name string
		set  func(*fakeWinCredAPI)
		op   func(*credentialStore) error
		want error
	}{
		{"write", func(api *fakeWinCredAPI) { api.writeErr = windows.ERROR_ACCESS_DENIED }, func(s *credentialStore) error { return s.Put(CodexClientKey, secret) }, ErrPersistence},
		{"write unavailable", func(api *fakeWinCredAPI) { api.writeErr = windows.ERROR_NO_SUCH_LOGON_SESSION }, func(s *credentialStore) error { return s.Put(CodexClientKey, secret) }, ErrUnavailable},
		{"read after write", func(api *fakeWinCredAPI) { api.readErr = windows.ERROR_ACCESS_DENIED }, func(s *credentialStore) error { return s.Put(CodexClientKey, secret) }, ErrPersistence},
		{"read", func(api *fakeWinCredAPI) { api.readErr = windows.ERROR_ACCESS_DENIED }, func(s *credentialStore) error { _, err := s.Get(CodexClientKey); return err }, ErrPersistence},
		{"read not found", func(api *fakeWinCredAPI) { api.readErr = windows.ERROR_NOT_FOUND }, func(s *credentialStore) error { _, err := s.Get(CodexClientKey); return err }, ErrNotFound},
		{"delete", func(api *fakeWinCredAPI) { api.deleteErr = windows.ERROR_ACCESS_DENIED }, func(s *credentialStore) error { return s.Delete(CodexClientKey) }, ErrPersistence},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			api := newFakeAPI()
			tc.set(api)
			err := tc.op(newCredentialStore(api, productionTargets()))
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
			if strings.Contains(err.Error(), secretText) {
				t.Fatal("error contains secret bytes")
			}
		})
	}
}

func TestZero(t *testing.T) {
	value := randomBytes(t, 64)
	Zero(value)
	if !allZero(value) {
		t.Fatal("Zero left nonzero bytes")
	}
}

func TestProductionSourceHasNoBroadCredentialOrProviderAccess(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	forbidden := []string{
		"Cred" + "Enumerate",
		"Cred" + "FindBestCredential",
		"CRYPTPROTECT" + "_LOCAL_MACHINE",
		"auth" + ".json",
		"access" + "_token",
		"refresh" + "_token",
		"Authori" + "zation",
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		for _, term := range forbidden {
			if bytes.Contains(data, []byte(term)) {
				t.Fatalf("production source %s contains forbidden API or provider access", entry.Name())
			}
		}
	}
}

func randomBytes(t *testing.T, size int) []byte {
	t.Helper()
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		t.Fatal(err)
	}
	return value
}

func allZero(value []byte) bool {
	for _, b := range value {
		if b != 0 {
			return false
		}
	}
	return true
}
