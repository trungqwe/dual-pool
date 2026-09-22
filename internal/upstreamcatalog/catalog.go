// Package upstreamcatalog owns immutable, independently bound release provenance.
package upstreamcatalog

import (
	"errors"
	"net/url"
	"regexp"
	"strings"

	"github.com/trungqwe/dual-pool/internal/upstreamlock"
)

var (
	ErrInvalidCatalog    = errors.New("trusted upstream catalog is invalid")
	ErrUnknownProvenance = errors.New("trusted upstream provenance is unknown")
	versionPattern       = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+\-]{0,63}$`)
	commitPattern        = regexp.MustCompile(`^[a-f0-9]{40}$`)
	digestPattern        = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

// Provenance is the complete immutable identity a logical release must match.
// It contains no mutable state and is returned by value from Catalog.
type Provenance struct {
	Product              string
	Version              string
	Tag                  string
	Commit               string
	Platform             string
	Artifact             string
	DownloadURL          string
	ArchiveSHA256        string
	ExecutableSHA256     string
	ConfigAdapterVersion string
	Digest               string
	ReleaseMetadataURL   string
}

// Catalog is an immutable in-memory trust set. It intentionally offers no
// mutation API; inventory records never establish provenance.
type Catalog struct{ entries map[string]Provenance }

// FromPinnedLock is the sole production trust-source adapter in this slice.
// It preserves the existing exact upstream.lock pin and its digest.
func FromPinnedLock(lock upstreamlock.Lock) (*Catalog, error) {
	if err := lock.Validate(); err != nil {
		return nil, ErrInvalidCatalog
	}
	p := lock.Platforms.WindowsAMD64
	return newVerified([]Provenance{{
		Product: lock.Product, Version: lock.Version, Tag: lock.Tag, Commit: lock.Commit,
		Platform: "windows_amd64", Artifact: p.Artifact, DownloadURL: p.DownloadURL,
		ArchiveSHA256: p.ArchiveSHA256, ExecutableSHA256: p.ExecutableSHA256,
		ConfigAdapterVersion: lock.ConfigAdapterVersion, Digest: lock.Digest(),
		ReleaseMetadataURL: lock.ReleaseMetadataURL,
	}})
}

// NewVerified constructs a catalog only from entries that the caller has
// already independently verified. It performs structural validation only; it
// never turns unverified external metadata into production trust. Runtime
// composition deliberately uses FromPinnedLock, never this constructor.
func NewVerified(entries ...Provenance) (*Catalog, error) { return newVerified(entries) }

func newVerified(entries []Provenance) (*Catalog, error) {
	if len(entries) == 0 {
		return nil, ErrInvalidCatalog
	}
	result := &Catalog{entries: make(map[string]Provenance, len(entries))}
	for _, entry := range entries {
		if !valid(entry) {
			return nil, ErrInvalidCatalog
		}
		if _, exists := result.entries[entry.Version]; exists {
			return nil, ErrInvalidCatalog
		}
		result.entries[entry.Version] = entry
	}
	return result, nil
}

// Resolve returns the exact independently bound provenance for version.
func (c *Catalog) Resolve(version string) (Provenance, error) {
	if c == nil || !versionPattern.MatchString(version) {
		return Provenance{}, ErrUnknownProvenance
	}
	p, ok := c.entries[version]
	if !ok {
		return Provenance{}, ErrUnknownProvenance
	}
	return p, nil
}

// Len exists for composition assertions; callers cannot mutate its result.
func (c *Catalog) Len() int {
	if c == nil {
		return 0
	}
	return len(c.entries)
}

func valid(p Provenance) bool {
	if p.Product != "CLIProxyAPI" || !versionPattern.MatchString(p.Version) || p.Tag != "v"+p.Version || !commitPattern.MatchString(p.Commit) || p.Platform != "windows_amd64" || !upstreamlock.SafeBasename(p.Artifact) || p.Artifact != "CLIProxyAPI_"+p.Version+"_windows_amd64.zip" || !digestPattern.MatchString(p.ArchiveSHA256) || !digestPattern.MatchString(p.ExecutableSHA256) || !digestPattern.MatchString(p.Digest) || strings.TrimSpace(p.ConfigAdapterVersion) == "" {
		return false
	}
	return exactURL(p.ReleaseMetadataURL, "/router-for-me/CLIProxyAPI/releases/tag/"+p.Tag) && exactURL(p.DownloadURL, "/router-for-me/CLIProxyAPI/releases/download/"+p.Tag+"/"+p.Artifact)
}

func exactURL(raw, path string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host == "github.com" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && u.Path == path
}
