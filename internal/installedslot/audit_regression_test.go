package installedslot

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/trungqwe/dual-pool/internal/lockfile"
	"github.com/trungqwe/dual-pool/internal/upstreamlock"
)

func TestRegistryInjectedLockManagerMustMatchLayoutLocks(t *testing.T) {
	fixture, _ := fixtureRegistry(t)
	raw, err := os.ReadFile(filepath.Join("..", "..", "upstream.lock"))
	if err != nil {
		t.Fatal(err)
	}
	pin, err := upstreamlock.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	otherLocks := filepath.Join(filepath.Dir(fixture.layout.Locks), "other-locks")
	if err = os.MkdirAll(otherLocks, 0700); err != nil {
		t.Fatal(err)
	}
	other, err := lockfile.NewManager(otherLocks)
	if err != nil {
		t.Fatal(err)
	}
	if registry, err := New(fixture.layout, fixture.acl, pin, WithLockManager(other)); err == nil || registry != nil {
		t.Fatalf("mismatched lock root accepted: %#v %v", registry, err)
	}
}

func TestRegistryExplicitNilLockManagerFailsClosed(t *testing.T) {
	fixture, _ := fixtureRegistry(t)
	raw, err := os.ReadFile(filepath.Join("..", "..", "upstream.lock"))
	if err != nil {
		t.Fatal(err)
	}
	pin, err := upstreamlock.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if registry, err := New(fixture.layout, fixture.acl, pin, WithLockManager(nil)); err == nil || registry != nil {
		t.Fatalf("nil lock manager accepted: %#v %v", registry, err)
	}
}

func productionPolicyFixture(t *testing.T, body []byte, calls *int) (*Registry, upstreamlock.Lock) {
	t.Helper()
	fixture, _ := fixtureRegistry(t)
	raw, err := os.ReadFile(filepath.Join("..", "..", "upstream.lock"))
	if err != nil {
		t.Fatal(err)
	}
	original, err := upstreamlock.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	// Only fixture artifact hash changes; constructor policy and pinned version,
	// tag, commit, adapter and provenance validation remain enabled.
	raw = bytes.ReplaceAll(raw, []byte(original.Platforms.WindowsAMD64.ExecutableSHA256), []byte(hashBytes(body)))
	pin, err := upstreamlock.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	r, err := New(fixture.layout, fixture.acl, pin, WithBinaryVerifier(func(_ context.Context, path string, _ Manifest) error {
		*calls++
		data, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(data, body) {
			return ErrUnsafeSlot
		}
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	return r, pin
}

func writePolicySlot(t *testing.T, r *Registry, pin upstreamlock.Lock, version string, body []byte) {
	t.Helper()
	m := writeFixtureSlot(t, r, version, body)
	m.Tag, m.Commit, m.UpstreamLockSHA256, m.ConfigAdapterVersion = pin.Tag, pin.Commit, pin.Digest(), pin.ConfigAdapterVersion
	data, err := encodeManifest(m)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(r.slotPath(version), manifestName), data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestProductionPolicyRejectsVersionAliasBeforeVerifier(t *testing.T) {
	calls := 0
	body := []byte("synthetic pinned artifact")
	r, pin := productionPolicyFixture(t, body, &calls)
	writePolicySlot(t, r, pin, "7.3.8", body)
	if err := r.Register(context.Background(), "7.3.8"); !errors.Is(err, ErrUnsafeSlot) {
		t.Fatalf("version alias accepted: %v", err)
	}
	if calls != 0 {
		t.Fatal("aliased version reached binary verifier")
	}
	if _, err := os.Stat(r.registryPath()); !os.IsNotExist(err) {
		t.Fatalf("alias published registry: %v", err)
	}
}

func TestProductionPolicyRejectsCoherentTamperBeforeVerifier(t *testing.T) {
	calls := 0
	body := []byte("synthetic pinned artifact")
	r, pin := productionPolicyFixture(t, body, &calls)
	writePolicySlot(t, r, pin, pin.Version, []byte("coherently replaced executable"))
	if err := r.Register(context.Background(), pin.Version); !errors.Is(err, ErrUnsafeSlot) {
		t.Fatalf("tamper not rejected by policy: %v", err)
	}
	if calls != 0 {
		t.Fatal("unpinned bytes reached executable verifier")
	}
	if _, err := os.Stat(r.registryPath()); !os.IsNotExist(err) {
		t.Fatalf("tamper published registry: %v", err)
	}
}

func TestProductionPolicyAcceptsPinnedIdentity(t *testing.T) {
	calls := 0
	body := []byte("synthetic pinned artifact")
	r, pin := productionPolicyFixture(t, body, &calls)
	writePolicySlot(t, r, pin, pin.Version, body)
	if err := r.Register(context.Background(), pin.Version); err != nil {
		t.Fatal(err)
	}
	got, err := r.Resolve(pin.Version)
	if err != nil || got.Version != pin.Version || got.ExecutableSHA256 != pin.Platforms.WindowsAMD64.ExecutableSHA256 {
		t.Fatalf("pinned identity: %+v %v", got, err)
	}
	if calls != 2 {
		t.Fatalf("expected verification on register and resolve, got %d", calls)
	}
}

func TestProductionPolicyResolveRejectsCoherentTamperingBeforeVerifier(t *testing.T) {
	calls := 0
	body := []byte("synthetic pinned artifact")
	r, pin := productionPolicyFixture(t, body, &calls)
	writePolicySlot(t, r, pin, pin.Version, body)
	if err := r.Register(context.Background(), pin.Version); err != nil {
		t.Fatal(err)
	}
	calls = 0
	replacement := []byte("coherent replacement of all writable metadata")
	writePolicySlot(t, r, pin, pin.Version, replacement)
	manifestBytes, err := os.ReadFile(filepath.Join(r.slotPath(pin.Version), manifestName))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := r.loadDocument()
	if err != nil {
		t.Fatal(err)
	}
	doc.Slots[0].ExecutableSHA256 = hashBytes(replacement)
	doc.Slots[0].ManifestSHA256 = hashBytes(manifestBytes)
	if err := r.writeDocument(doc); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Resolve(pin.Version); !errors.Is(err, ErrUnsafeSlot) {
		t.Fatalf("coherent resolve accepted: %v", err)
	}
	if calls != 0 {
		t.Fatal("resolve executed unpinned bytes")
	}
}

func TestRegistryRejectsCoherentRebindWithoutChangingDocument(t *testing.T) {
	r, _ := fixtureRegistry(t)
	writeFixtureSlot(t, r, "vA", []byte("original"))
	if err := r.Register(context.Background(), "vA"); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(r.registryPath())
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureSlot(t, r, "vA", []byte("new independently valid identity"))
	if err := r.Register(context.Background(), "vA"); !errors.Is(err, ErrSlotConflict) {
		t.Fatalf("coherent rebind: %v", err)
	}
	after, err := os.ReadFile(r.registryPath())
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("registry changed: %v", err)
	}
}

func TestRegistryMalformedRegisteredDocumentReturnsCorrupt(t *testing.T) {
	for _, kind := range []string{"duplicate", "unknown", "trailing"} {
		t.Run(kind, func(t *testing.T) {
			r, _ := fixtureRegistry(t)
			writeFixtureSlot(t, r, "vA", []byte("slot-a"))
			if err := r.Register(context.Background(), "vA"); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(r.registryPath())
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "duplicate":
				data = bytes.Replace(data, []byte(`"schema_version":1`), []byte(`"schema_version":1,"schema_version":1`), 1)
			case "unknown":
				data = bytes.Replace(data, []byte(`"schema_version":1`), []byte(`"schema_version":1,"unknown":true`), 1)
			case "trailing":
				data = append(data, []byte("{}")...)
			}
			if err = os.WriteFile(r.registryPath(), data, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err = r.Resolve("vA"); !errors.Is(err, ErrRegistryCorrupt) {
				t.Fatalf("expected corrupt, got %v", err)
			}
		})
	}
}

func TestRegistryPublicationFaultMatrix(t *testing.T) {
	points := []FaultPoint{AfterCandidateCreate, AfterCandidateWrite, AfterCandidateSync, AfterPublication, AfterPublicationVerification}
	for _, existing := range []bool{false, true} {
		for _, point := range points {
			name := string(point)
			if existing {
				name = "replacement/" + name
			} else {
				name = "initial/" + name
			}
			t.Run(name, func(t *testing.T) {
				r, _ := fixtureRegistry(t)
				writeFixtureSlot(t, r, "vB", []byte("slot-b"))
				var old []byte
				if existing {
					writeFixtureSlot(t, r, "vA", []byte("slot-a"))
					if err := r.Register(context.Background(), "vA"); err != nil {
						t.Fatal(err)
					}
					var err error
					old, err = os.ReadFile(r.registryPath())
					if err != nil {
						t.Fatal(err)
					}
				}
				injected := false
				r.fault = func(p FaultPoint) error {
					if p == point {
						injected = true
						return ErrPersistence
					}
					return nil
				}
				if err := r.Register(context.Background(), "vB"); err == nil || !injected {
					t.Fatalf("fault not reached: %v", err)
				}
				reopened := *r
				reopened.fault = nil
				published := point == AfterPublication || point == AfterPublicationVerification
				if existing {
					if _, err := reopened.Resolve("vA"); err != nil {
						t.Fatalf("old slot lost: %v", err)
					}
					if !published {
						after, err := os.ReadFile(r.registryPath())
						if err != nil || !bytes.Equal(after, old) {
							t.Fatalf("prepublication changed registry: %v", err)
						}
					}
				}
				slot, err := reopened.Resolve("vB")
				if published {
					if err != nil || slot.ExecutableSHA256 != hashBytes([]byte("slot-b")) {
						t.Fatalf("incomplete published slot: %+v %v", slot, err)
					}
				} else if existing {
					if !errors.Is(err, ErrSlotUnknown) {
						t.Fatalf("unexpected candidate: %v", err)
					}
				} else if !errors.Is(err, ErrRegistryMissing) {
					t.Fatalf("unexpected initial document: %v", err)
				}
			})
		}
	}
}

func TestWindowsVersionReservedExtensionsAndControls(t *testing.T) {
	for _, version := range []string{"CON.txt", "nul.foo", "Com1.exe", "lpt9.anything", "x\x00", "x\x1f", "x\x7f", strings.Repeat("x", 65)} {
		if ValidVersion(version) {
			t.Fatalf("unsafe version accepted: %q", version)
		}
	}
}
