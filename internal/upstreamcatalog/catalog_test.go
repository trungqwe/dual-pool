package upstreamcatalog

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/trungqwe/dual-pool/internal/upstreamlock"
)

func TestProductionCatalogContainsCurrentAndVerifiedV738(t *testing.T) {
	c, err := Production(pinned(t))
	if err != nil || c.Len() != 2 {
		t.Fatalf("catalog len=%d err=%v", c.Len(), err)
	}
	for _, version := range []string{"7.3.7", "7.3.8"} {
		if _, err := c.Resolve(version); err != nil {
			t.Fatalf("trusted version %s: %v", version, err)
		}
	}
}

func TestProductionCatalogPreservesCurrentPinnedIdentity(t *testing.T) {
	lock := pinned(t)
	c, err := Production(lock)
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.Resolve("7.3.7")
	if err != nil || p.Digest != lock.Digest() || p.Commit != lock.Commit || p.ExecutableSHA256 != lock.Platforms.WindowsAMD64.ExecutableSHA256 {
		t.Fatalf("current identity=%+v err=%v", p, err)
	}
}

func TestProductionCatalogV738ExactIdentity(t *testing.T) {
	c, err := Production(pinned(t))
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.Resolve("7.3.8")
	if err != nil || p.Tag != "v7.3.8" || p.Commit != "c93978c4ea2e908255a2a06c37599fda3651554a" || p.ArchiveSHA256 != "5e3278ac9b57d16df503fd845827a6fdb57ec241f102b35899788287eb431351" || p.ExecutableSHA256 != "479da2fb56eb3db11a76e19adeb2e10c2a4069a512ab5e3933ac4c50628360fd" || p.ConfigAdapterVersion != "dualpool-cpa-v7.3.7-config-v1" {
		t.Fatalf("v7.3.8 identity=%+v err=%v", p, err)
	}
}

func TestProductionCatalogRejectsUnknownVersion(t *testing.T) {
	c, err := Production(pinned(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"7.3.9", "7.3.12"} {
		if _, err := c.Resolve(version); err == nil {
			t.Fatalf("unknown version accepted: %s", version)
		}
	}
}

func TestVerifiedV738ReceiptDigestStable(t *testing.T) {
	p, err := parseVerifiedV738Receipt(verifiedV738Receipt)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(verifiedV738Receipt)
	if p.Digest != hex.EncodeToString(sum[:]) {
		t.Fatalf("digest=%s", p.Digest)
	}
}

func TestVerifiedV738ReceiptRejectsTampering(t *testing.T) {
	for _, old := range []string{"7.3.8", "v7.3.8", "c93978c4ea2e908255a2a06c37599fda3651554a", "CLIProxyAPI_7.3.8_windows_amd64.zip", "5e3278ac9b57d16df503fd845827a6fdb57ec241f102b35899788287eb431351", "479da2fb56eb3db11a76e19adeb2e10c2a4069a512ab5e3933ac4c50628360fd", "dualpool-cpa-v7.3.7-config-v1", "windows_amd64", "https://github.com/router-for-me/CLIProxyAPI/releases/tag/v7.3.8", "https://github.com/router-for-me/CLIProxyAPI/releases/download/v7.3.8/CLIProxyAPI_7.3.8_windows_amd64.zip"} {
		t.Run(old, func(t *testing.T) {
			bad := []byte(strings.Replace(string(verifiedV738Receipt), old, old+"x", 1))
			if _, err := parseVerifiedV738Receipt(bad); err == nil {
				t.Fatal("tampered receipt accepted")
			}
		})
	}
	for _, ending := range []struct {
		name   string
		ending string
	}{{name: "LF", ending: "\n"}, {name: "CRLF", ending: "\r\n"}} {
		t.Run(ending.name, func(t *testing.T) {
			receipt := strings.ReplaceAll(strings.ReplaceAll(string(verifiedV738Receipt), "\r\n", "\n"), "\n", ending.ending)
			unknown := insertUnknownReceiptField(receipt)
			duplicate := strings.Replace(receipt, `"version": "7.3.8",`, `"version": "7.3.8", "version": "7.3.8",`, 1)
			for name, bad := range map[string]string{"unknown field": unknown, "duplicate version": duplicate} {
				t.Run(name, func(t *testing.T) {
					if bad == receipt {
						t.Fatal("tampering fixture did not modify the receipt")
					}
					if _, err := parseVerifiedV738Receipt([]byte(bad)); err == nil {
						t.Fatal("structurally tampered receipt accepted")
					}
				})
			}
		})
	}
}

func insertUnknownReceiptField(raw string) string {
	trimmed := strings.TrimSpace(raw)
	closing := strings.LastIndexByte(trimmed, '}')
	if closing < 0 {
		return raw
	}
	return trimmed[:closing] + `,"unknown":true` + trimmed[closing:]
}

func TestVerifiedV738ReceiptRejectsDuplicateKeys(t *testing.T) {
	receipt := string(verifiedV738Receipt)
	bad := strings.Replace(receipt, `"tag": "v7.3.8",`, `"tag": "v7.3.8", "tag": "v7.3.8",`, 1)
	if bad == receipt {
		t.Fatal("duplicate-key fixture did not modify the receipt")
	}
	if _, err := parseVerifiedV738Receipt([]byte(bad)); err == nil {
		t.Fatal("duplicate key accepted")
	}
}

func pinned(t *testing.T) upstreamlock.Lock {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "upstream.lock"))
	if err != nil {
		t.Fatal(err)
	}
	l, err := upstreamlock.Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func fixture(version, adapter string, seed byte) Provenance {
	digest := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	archive := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	exe := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	commit := "dddddddddddddddddddddddddddddddddddddddd"
	if seed == 'b' {
		digest = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
		archive = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
		exe = "1111111111111111111111111111111111111111111111111111111111111111"
		commit = "2222222222222222222222222222222222222222"
	}
	tag, artifact := "v"+version, "CLIProxyAPI_"+version+"_windows_amd64.zip"
	return Provenance{Product: "CLIProxyAPI", Version: version, Tag: tag, Commit: commit, Platform: "windows_amd64", Artifact: artifact, DownloadURL: "https://github.com/router-for-me/CLIProxyAPI/releases/download/" + tag + "/" + artifact, ArchiveSHA256: archive, ExecutableSHA256: exe, ConfigAdapterVersion: adapter, Digest: digest, ReleaseMetadataURL: "https://github.com/router-for-me/CLIProxyAPI/releases/tag/" + tag}
}

func TestFromPinnedLockPreservesExactCurrentIdentity(t *testing.T) {
	l := pinned(t)
	c, err := FromPinnedLock(l)
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.Resolve(l.Version)
	if err != nil || p.Tag != l.Tag || p.Commit != l.Commit || p.ExecutableSHA256 != l.Platforms.WindowsAMD64.ExecutableSHA256 || p.ArchiveSHA256 != l.Platforms.WindowsAMD64.ArchiveSHA256 || p.ConfigAdapterVersion != l.ConfigAdapterVersion || p.Digest != l.Digest() {
		t.Fatalf("provenance=%+v err=%v", p, err)
	}
}

func TestCatalogRejectsInvalidAndDuplicateEntries(t *testing.T) {
	base := fixture("vA", "adapter-a", 'a')
	for name, change := range map[string]func(*Provenance){"duplicate": func(*Provenance) {}, "version": func(p *Provenance) { p.Version = "../x" }, "tag": func(p *Provenance) { p.Tag = "wrong" }, "commit": func(p *Provenance) { p.Commit = "short" }, "digest": func(p *Provenance) { p.Digest = "bad" }, "hash": func(p *Provenance) { p.ExecutableSHA256 = "bad" }, "artifact": func(p *Provenance) { p.Artifact = "../x" }, "url": func(p *Provenance) { p.DownloadURL = "https://example.invalid/x" }} {
		t.Run(name, func(t *testing.T) {
			bad := base
			change(&bad)
			entries := []Provenance{bad}
			if name == "duplicate" {
				entries = []Provenance{base, base}
			}
			if _, err := NewVerified(entries...); err == nil {
				t.Fatal("invalid catalog accepted")
			}
		})
	}
}

func TestCatalogUsesSharedWindowsSafeLogicalVersionContract(t *testing.T) {
	for _, version := range []string{"", ".", "..", "../x", `a\b`, "a/b", "CON", "CON.txt", "NUL", "nul.foo", "COM1", "COM1.exe", "LPT9", "v1.", "trailing ", "a\x00b", "a\x1fb", "a\x7fb", strings.Repeat("x", 65)} {
		p := fixture("vA", "adapter-a", 'a')
		p.Version = version
		p.Tag = "v" + version
		p.Artifact = "CLIProxyAPI_" + version + "_windows_amd64.zip"
		p.DownloadURL = "https://github.com/router-for-me/CLIProxyAPI/releases/download/" + p.Tag + "/" + p.Artifact
		p.ReleaseMetadataURL = "https://github.com/router-for-me/CLIProxyAPI/releases/tag/" + p.Tag
		if _, err := NewVerified(p); err == nil {
			t.Fatalf("unsafe version accepted: %q", version)
		}
	}
	for _, version := range []string{"7.3.7", "7.3.8", "1.2.3-rc1", "vA", "vB"} {
		p := fixture(version, "adapter-a", 'a')
		if _, err := NewVerified(p); err != nil {
			t.Fatalf("safe version rejected %q: %v", version, err)
		}
	}
}

func TestCatalogRejectsDuplicateProvenanceDigest(t *testing.T) {
	a := fixture("vA", "adapter-a", 'a')
	b := fixture("vB", "adapter-b", 'b')
	b.Digest = a.Digest
	if _, err := NewVerified(a, b); err == nil {
		t.Fatal("duplicate provenance digest accepted")
	}
}

func TestCatalogUnknownAndConcurrentResolve(t *testing.T) {
	c, err := NewVerified(fixture("vA", "adapter-a", 'a'), fixture("vB", "adapter-b", 'b'))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Resolve("vC"); err == nil {
		t.Fatal("unknown accepted")
	}
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				if _, err := c.Resolve("vA"); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()
}
