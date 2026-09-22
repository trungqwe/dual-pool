package cliproxyconfig

import (
	"bytes"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/trungqwe/dual-pool/internal/keymaterial"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
)

func smokeWireKeys(t *testing.T) ([]byte, []byte) {
	t.Helper()
	client, err := keymaterial.Encode(bytes.Repeat([]byte{0x31}, 32))
	if err != nil {
		t.Fatal(err)
	}
	management, err := keymaterial.Encode(bytes.Repeat([]byte{0x72}, 32))
	if err != nil {
		t.Fatal(err)
	}
	return client, management
}

func TestCompatibilitySmokeConfigMatchesProductionAdapterSemantics(t *testing.T) {
	client, management := smokeWireKeys(t)
	workspace := filepath.Join(t.TempDir(), "attempt-0123456789abcdef")
	data, err := RenderCompatibilitySmoke(49123, workspace, client, management)
	if err != nil {
		t.Fatal(err)
	}
	defer zero(data)
	var got config
	if err = yaml.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	want := expectedConfigAt(49123, filepath.Join(workspace, "auth"), string(client), got.RemoteManagement.SecretKey)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("compatibility smoke config diverged from production adapter semantics: got=%+v want=%+v", got, want)
	}
	if !bytes.HasPrefix(data, []byte(header)) {
		t.Fatal("smoke config does not carry current adapter and pinned upstream provenance")
	}
}

func TestCompatibilitySmokeConfigUsesLoopbackOnly(t *testing.T) {
	client, management := smokeWireKeys(t)
	data, err := RenderCompatibilitySmoke(49124, filepath.Join(t.TempDir(), "attempt"), client, management)
	if err != nil {
		t.Fatal(err)
	}
	defer zero(data)
	var got config
	if err = yaml.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Host != "127.0.0.1" || got.TLS.Enable || got.RemoteManagement.AllowRemote {
		t.Fatalf("unsafe smoke bind settings: host=%q tls=%v remote=%v", got.Host, got.TLS.Enable, got.RemoteManagement.AllowRemote)
	}
}

func TestCompatibilitySmokeConfigUsesDefaultBcryptCost(t *testing.T) {
	client, management := smokeWireKeys(t)
	data, err := RenderCompatibilitySmoke(49125, filepath.Join(t.TempDir(), "attempt"), client, management)
	if err != nil {
		t.Fatal(err)
	}
	defer zero(data)
	var got config
	if err = yaml.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	cost, err := bcrypt.Cost([]byte(got.RemoteManagement.SecretKey))
	if err != nil || cost != bcrypt.DefaultCost {
		t.Fatalf("production smoke bcrypt cost=%d err=%v, want %d", cost, err, bcrypt.DefaultCost)
	}
	if err = bcrypt.CompareHashAndPassword([]byte(got.RemoteManagement.SecretKey), management); err != nil {
		t.Fatal("management verifier does not match the synthetic key")
	}
}

func TestCompatibilitySmokeConfigRejectsUnsafePortOrPath(t *testing.T) {
	client, management := smokeWireKeys(t)
	for _, port := range []int{0, -1, 80, 8317, 8318, 65536} {
		if _, err := RenderCompatibilitySmoke(port, filepath.Join(t.TempDir(), "attempt"), client, management); err == nil {
			t.Errorf("unsafe port %d accepted", port)
		}
	}
	for _, workspace := range []string{"relative-attempt", `C:\\`, `C:\\safe\\..\\outside`} {
		if _, err := RenderCompatibilitySmoke(49126, workspace, client, management); err == nil {
			t.Errorf("unsafe workspace %q accepted", workspace)
		}
	}
}
