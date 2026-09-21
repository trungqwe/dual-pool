package installedslot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/trungqwe/dual-pool/internal/dataroot"
	"github.com/trungqwe/dual-pool/internal/lockfile"
)

type fixtureACL struct{}

func (fixtureACL) Create(path string) error { return os.MkdirAll(path, 0700) }
func (fixtureACL) Inspect(path string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return os.ErrPermission
	}
	return nil
}
func (fixtureACL) CreateFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
}
func (fixtureACL) InspectFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil || info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return os.ErrPermission
	}
	return nil
}

func fixtureRegistry(t *testing.T) (*Registry, string) {
	t.Helper()
	root := t.TempDir()
	layout := dataroot.Layout{Bin: filepath.Join(root, "bin"), Locks: filepath.Join(root, "locks")}
	if err := os.MkdirAll(filepath.Join(layout.Bin, "cliproxyapi"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(layout.Locks, 0700); err != nil {
		t.Fatal(err)
	}
	locks, err := lockfile.NewManager(layout.Locks)
	if err != nil {
		t.Fatal(err)
	}
	r := &Registry{layout: layout, acl: fixtureACL{}, locks: locks, expectedPlatform: "windows_amd64", expectedAdapter: "fixture-adapter", expectedProduct: "CLIProxyAPI", binaryVerifier: func(context.Context, string, Manifest) error { return nil }}
	return r, root
}

func fixtureManifest(version, exeHash string) Manifest {
	return Manifest{SchemaVersion: 1, Product: "CLIProxyAPI", Version: version, Tag: "fixture-" + version, Commit: "fixture-commit-" + version, Platform: "windows_amd64", ExecutableSHA256: exeHash, UpstreamLockSHA256: "fixture-lock", ConfigAdapterVersion: "fixture-adapter", ExecutableBasename: "cliproxyapi.exe"}
}

func writeFixtureSlot(t *testing.T, r *Registry, version string, body []byte) Manifest {
	t.Helper()
	dir := filepath.Join(r.layout.Bin, "cliproxyapi", version)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "cliproxyapi.exe")
	if err := os.WriteFile(exe, body, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	m := fixtureManifest(version, hex.EncodeToString(sum[:]))
	data, err := encodeManifest(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, manifestName), data, 0600); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestLogicalVersionRejectsPathAndDeviceForms(t *testing.T) {
	for _, value := range []string{"../x", `..\x`, `C:\x`, `\\server\share`, `\\?\C:\x`, "x/y", `x\y`, "x:", ".", "..", "NUL", "CON", "trailing-dot.", "trailing-space ", ""} {
		if validVersion(value) {
			t.Fatalf("unsafe logical version accepted: %q", value)
		}
	}
}

func TestRegistryResolvesTwoImmutableSlotsAndRejectsRebind(t *testing.T) {
	r, _ := fixtureRegistry(t)
	a := writeFixtureSlot(t, r, "vA", []byte("slot-a"))
	b := writeFixtureSlot(t, r, "vB", []byte("slot-b"))
	if err := r.Register(context.Background(), "vA"); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(context.Background(), "vB"); err != nil {
		t.Fatal(err)
	}
	ra, err := r.Resolve("vA")
	if err != nil || ra.Version != a.Version || ra.ExecutablePath == "" {
		t.Fatalf("resolve vA: %#v %v", ra, err)
	}
	rb, err := r.Resolve("vB")
	if err != nil || rb.Version != b.Version || rb.ExecutablePath == ra.ExecutablePath {
		t.Fatalf("resolve vB: %#v %v", rb, err)
	}
	if err := r.Register(context.Background(), "vA"); err != nil {
		t.Fatalf("identical registration not idempotent: %v", err)
	}
	dir := filepath.Join(r.layout.Bin, "cliproxyapi", "vA")
	if err := os.WriteFile(filepath.Join(dir, "cliproxyapi.exe"), []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(context.Background(), "vA"); err == nil {
		t.Fatal("replaced slot accepted")
	}
}

func TestRegistryRejectsCorruptDocumentAndSlot(t *testing.T) {
	r, _ := fixtureRegistry(t)
	writeFixtureSlot(t, r, "vA", []byte("slot-a"))
	if err := r.Register(context.Background(), "vA"); err != nil {
		t.Fatal(err)
	}
	path := r.registryPath()
	for _, payload := range []string{
		`{"schema_version":1,"slots":[],"slots":[]}`,
		`{"schema_version":1,"slots":[],"extra":true}`,
		`{"schema_version":1,"slots":[]} {}`,
	} {
		if err := os.WriteFile(path, []byte(payload), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := r.Resolve("vA"); err == nil {
			t.Fatalf("corrupt registry accepted: %s", payload)
		}
	}
	_ = os.Remove(path)
	if _, err := r.Resolve("vA"); err == nil {
		t.Fatal("missing registry accepted")
	}
	if err := r.Register(context.Background(), "missing"); err == nil {
		t.Fatal("missing slot registered")
	}
}

func TestRegistryPublicationFaultNeverExposesUnverifiedEntry(t *testing.T) {
	r, _ := fixtureRegistry(t)
	writeFixtureSlot(t, r, "vA", []byte("slot-a"))
	r.fault = func(point FaultPoint) error {
		if point == AfterCandidateSync {
			return ErrPersistence
		}
		return nil
	}
	if err := r.Register(context.Background(), "vA"); err == nil {
		t.Fatal("faulted registration succeeded")
	}
	if _, err := r.Resolve("vA"); !errors.Is(err, ErrRegistryMissing) {
		t.Fatalf("faulted candidate exposed: %v", err)
	}
}
