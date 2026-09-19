package secretstore

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"regexp"
	"testing"
)

const (
	childModeEnv      = "DUALPOOL_SECRETSTORE_CHILD"
	childNamespaceEnv = "DUALPOOL_SECRETSTORE_NAMESPACE"
	childDigestEnv    = "DUALPOOL_SECRETSTORE_DIGEST"
)

var testNamespacePattern = regexp.MustCompile(`\Adualpool-test:[0-9a-f]{32}:v1:\z`)

func newTestStore(namespace string) (*credentialStore, error) {
	if !testNamespacePattern.MatchString(namespace) {
		return nil, ErrInvalidPurpose
	}
	return newCredentialStore(systemWinCredAPI{}, targetsWithPrefix(namespace)), nil
}

func TestTestNamespaceValidation(t *testing.T) {
	for _, namespace := range []string{"", "dualpool:v1:", "dualpool-test:not-random:v1:", "dualpool-test:0123456789abcdef0123456789abcdef:v2:"} {
		if _, err := newTestStore(namespace); !errors.Is(err, ErrInvalidPurpose) {
			t.Fatalf("namespace accepted")
		}
	}
}

func TestWindowsCredentialManagerIntegration(t *testing.T) {
	if os.Getenv(childModeEnv) == "1" {
		runCredentialChild(t)
		return
	}

	namespace := "dualpool-test:" + hex.EncodeToString(randomBytes(t, 16)) + ":v1:"
	store, err := newTestStore(namespace)
	if err != nil {
		t.Fatal(err)
	}
	purposes := []Purpose{CodexClientKey, CodexManagementKey, GoogleClientKey, GoogleManagementKey}
	t.Cleanup(func() {
		for _, purpose := range purposes {
			_ = store.Delete(purpose)
		}
	})
	for _, purpose := range purposes {
		if err := store.Delete(purpose); err != nil {
			t.Fatalf("pre-test exact cleanup failed: %v", err)
		}
	}

	if _, err := store.Get(CodexClientKey); !errors.Is(err, ErrNotFound) {
		t.Fatalf("never-written Get = %v", err)
	}

	values := make(map[Purpose][]byte, len(purposes))
	for _, purpose := range purposes {
		values[purpose] = randomBytes(t, 48)
		if err := store.Put(purpose, values[purpose]); err != nil {
			t.Fatalf("Put failed: %v", err)
		}
	}
	defer func() {
		for _, value := range values {
			Zero(value)
		}
	}()
	for i, left := range purposes {
		for _, right := range purposes[i+1:] {
			if bytes.Equal(values[left], values[right]) {
				t.Fatal("synthetic purpose values unexpectedly equal")
			}
		}
		got, err := store.Get(left)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if !bytes.Equal(got, values[left]) {
			Zero(got)
			t.Fatal("purpose isolation mismatch")
		}
		Zero(got)
	}

	replacement := randomBytes(t, 48)
	defer Zero(replacement)
	if err := store.Put(CodexClientKey, replacement); err != nil {
		t.Fatalf("replacement Put failed: %v", err)
	}
	got, err := store.Get(CodexClientKey)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, replacement) || bytes.Equal(got, values[CodexClientKey]) {
		Zero(got)
		t.Fatal("replacement verification failed")
	}
	Zero(got)

	digest := sha256.Sum256(replacement)
	cmd := exec.Command(os.Args[0], "-test.run=^TestWindowsCredentialManagerIntegration$")
	cmd.Env = append(os.Environ(),
		childModeEnv+"=1",
		childNamespaceEnv+"="+namespace,
		childDigestEnv+"="+hex.EncodeToString(digest[:]),
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cross-process credential read failed: %v; child output contained %d bytes", err, len(output))
	}

	if err := store.Delete(CodexClientKey); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(CodexClientKey); err != nil {
		t.Fatalf("idempotent delete failed: %v", err)
	}
	for _, purpose := range purposes[1:] {
		remaining, err := store.Get(purpose)
		if err != nil {
			t.Fatalf("selective delete affected another purpose: %v", err)
		}
		if !bytes.Equal(remaining, values[purpose]) {
			Zero(remaining)
			t.Fatal("remaining purpose changed")
		}
		Zero(remaining)
	}
}

func runCredentialChild(t *testing.T) {
	store, err := newTestStore(os.Getenv(childNamespaceEnv))
	if err != nil {
		t.Fatal("invalid child namespace")
	}
	expected, err := hex.DecodeString(os.Getenv(childDigestEnv))
	if err != nil || len(expected) != sha256.Size {
		t.Fatal("invalid child digest")
	}
	secret, err := store.Get(CodexClientKey)
	if err != nil {
		t.Fatal("child credential read failed")
	}
	defer Zero(secret)
	actual := sha256.Sum256(secret)
	if !bytes.Equal(actual[:], expected) {
		t.Fatal("child credential digest mismatch")
	}
}
