package installedslot

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/trungqwe/dual-pool/internal/upstreamcatalog"
)

func writeManifestForCatalog(t *testing.T, r *Registry, version string, m Manifest) {
	t.Helper()
	b, err := encodeManifest(m)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(r.slotPath(version), manifestName), b, 0600); err != nil {
		t.Fatal(err)
	}
}

func twoCatalogRegistry(t *testing.T, calls *int) (*Registry, Manifest, Manifest) {
	t.Helper()
	r, _ := fixtureRegistry(t)
	r.binaryVerifier = func(context.Context, string, upstreamcatalog.Provenance) error { *calls++; return nil }
	a := writeFixtureSlot(t, r, "vA", []byte("catalog-slot-a"))
	b := writeFixtureSlot(t, r, "vB", []byte("catalog-slot-b"))
	b.ConfigAdapterVersion = "fixture-adapter-b"
	writeManifestForCatalog(t, r, "vB", b)
	refreshFixtureCatalog(t, r)
	return r, a, b
}

func TestRegistryResolvesTwoCatalogBoundReleases(t *testing.T) {
	calls := 0
	r, a, b := twoCatalogRegistry(t, &calls)
	if err := r.Register(context.Background(), "vA"); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(context.Background(), "vB"); err != nil {
		t.Fatal(err)
	}
	gotA, err := r.Resolve("vA")
	if err != nil || gotA.Commit != a.Commit || gotA.ExecutableSHA256 != a.ExecutableSHA256 || gotA.UpstreamLockSHA256 != a.UpstreamLockSHA256 || gotA.ConfigAdapterVersion != a.ConfigAdapterVersion {
		t.Fatalf("vA=%+v err=%v", gotA, err)
	}
	gotB, err := r.Resolve("vB")
	if err != nil || gotB.Commit != b.Commit || gotB.ExecutableSHA256 != b.ExecutableSHA256 || gotB.UpstreamLockSHA256 != b.UpstreamLockSHA256 || gotB.ConfigAdapterVersion != b.ConfigAdapterVersion {
		t.Fatalf("vB=%+v err=%v", gotB, err)
	}
	if gotA.Commit == gotB.Commit || gotA.ExecutableSHA256 == gotB.ExecutableSHA256 || gotA.UpstreamLockSHA256 == gotB.UpstreamLockSHA256 || gotA.ConfigAdapterVersion == gotB.ConfigAdapterVersion {
		t.Fatal("cross-release identity was borrowed")
	}
}

func TestRegistryRejectsCatalogCrossBindingBeforeVerifier(t *testing.T) {
	for name, copyField := range map[string]func(*Manifest, Manifest){
		"Commit":           func(b *Manifest, a Manifest) { b.Commit = a.Commit },
		"ExecutableHash":   func(b *Manifest, a Manifest) { b.ExecutableSHA256 = a.ExecutableSHA256 },
		"ProvenanceDigest": func(b *Manifest, a Manifest) { b.UpstreamLockSHA256 = a.UpstreamLockSHA256 },
		"Tag":              func(b *Manifest, a Manifest) { b.Tag = a.Tag },
		"Adapter":          func(b *Manifest, a Manifest) { b.ConfigAdapterVersion = a.ConfigAdapterVersion },
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			r, a, b := twoCatalogRegistry(t, &calls)
			copyField(&b, a)
			writeManifestForCatalog(t, r, "vB", b)
			if err := r.Register(context.Background(), "vB"); !errors.Is(err, ErrUnsafeSlot) {
				t.Fatalf("cross binding accepted: %v", err)
			}
			if calls != 0 {
				t.Fatalf("binary verifier called %d times", calls)
			}
		})
	}
}

func TestRegistryRejectsUnknownReleaseBeforeVerifier(t *testing.T) {
	calls := 0
	r, _, _ := twoCatalogRegistry(t, &calls)
	unknown := writeFixtureSlot(t, r, "vC", []byte("untrusted-but-present"))
	// Restore the original two-entry authority after writing an on-disk vC.
	_ = unknown
	entries := make([]upstreamcatalog.Provenance, 0, 2)
	for _, version := range []string{"vA", "vB"} {
		p, err := r.catalog.Resolve(version)
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, p)
	}
	r.catalog, _ = upstreamcatalog.NewVerified(entries...)
	if _, err := r.Resolve("vC"); !errors.Is(err, ErrSlotUnknown) {
		t.Fatalf("unknown release accepted: %v", err)
	}
	if calls != 0 {
		t.Fatal("unknown release reached verifier")
	}
}

func TestRegistryRejectsLogicalVersionRebind(t *testing.T) {
	calls := 0
	r, _, b := twoCatalogRegistry(t, &calls)
	if err := r.Register(context.Background(), "vA"); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(r.registryPath())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(r.slotPath("vA"), "cliproxyapi.exe"), []byte("catalog-slot-b"), 0600); err != nil {
		t.Fatal(err)
	}
	writeManifestForCatalog(t, r, "vA", b)
	if _, err = r.Resolve("vA"); err == nil {
		t.Fatal("logical version rebound")
	}
	after, err := os.ReadFile(r.registryPath())
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("registry changed: %v", err)
	}
}

func TestRegistryDoesNotTrustEntryAbsentFromCatalogAndHashStopsVerifier(t *testing.T) {
	calls := 0
	r, _, _ := twoCatalogRegistry(t, &calls)
	if err := r.Register(context.Background(), "vA"); err != nil {
		t.Fatal(err)
	}
	p, err := r.catalog.Resolve("vB")
	if err != nil {
		t.Fatal(err)
	}
	r.catalog, err = upstreamcatalog.NewVerified(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Resolve("vA"); !errors.Is(err, ErrSlotUnknown) {
		t.Fatalf("inventory bootstrapped trust: %v", err)
	}
	if calls != 1 {
		t.Fatalf("absent catalog reached verifier: %d", calls)
	}
	calls = 0
	r, _, _ = twoCatalogRegistry(t, &calls)
	if err = os.WriteFile(filepath.Join(r.slotPath("vA"), "cliproxyapi.exe"), []byte("wrong-bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = r.Register(context.Background(), "vA"); !errors.Is(err, ErrSlotConflict) {
		t.Fatalf("hash mismatch: %v", err)
	}
	if calls != 0 {
		t.Fatal("hash mismatch reached verifier")
	}
}
