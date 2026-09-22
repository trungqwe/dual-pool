package upstreamcatalog

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/trungqwe/dual-pool/internal/upstreamlock"
)

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
