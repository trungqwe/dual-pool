package upstreamlock

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

const MaxBytes = 64 * 1024

var (
	ErrLockInvalid = errors.New("upstream lock is invalid")
	semver         = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	hex40          = regexp.MustCompile(`^[0-9a-f]{40}$`)
	hex64          = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type Lock struct {
	SchemaVersion          int       `json:"schema_version"`
	Product                string    `json:"product"`
	Status                 string    `json:"status"`
	Version                string    `json:"version"`
	Tag                    string    `json:"tag"`
	Commit                 string    `json:"commit"`
	RetrievedAt            string    `json:"retrieved_at"`
	ReleaseMetadataURL     string    `json:"release_metadata_url"`
	ConfigAdapterVersion   string    `json:"config_adapter_version"`
	Platforms              Platforms `json:"platforms"`
	VerifiedCapabilities   []string  `json:"verified_capabilities"`
	UnverifiedCapabilities []string  `json:"unverified_capabilities"`
	Evidence               string    `json:"evidence"`
	raw                    []byte
}

type Platforms struct {
	WindowsAMD64 Platform `json:"windows_amd64"`
}

type Platform struct {
	Artifact         string `json:"artifact"`
	DownloadURL      string `json:"download_url"`
	ArchiveSHA256    string `json:"archive_sha256"`
	ExecutableSHA256 string `json:"executable_sha256"`
}

func (l Lock) Raw() []byte    { return append([]byte(nil), l.raw...) }
func (l Lock) Digest() string { sum := sha256.Sum256(l.raw); return hex.EncodeToString(sum[:]) }

func Decode(data []byte) (Lock, error) {
	var value Lock
	if len(data) == 0 || len(data) > MaxBytes || !utf8.Valid(data) || duplicateKeys(data) {
		return value, ErrLockInvalid
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&value); err != nil {
		return Lock{}, ErrLockInvalid
	}
	var extra json.RawMessage
	if !errors.Is(d.Decode(&extra), io.EOF) {
		return Lock{}, ErrLockInvalid
	}
	value.raw = append([]byte(nil), data...)
	if err := value.Validate(); err != nil {
		return Lock{}, err
	}
	return value, nil
}

func (l Lock) Validate() error {
	p := l.Platforms.WindowsAMD64
	if l.SchemaVersion != 1 || l.Product != "CLIProxyAPI" || l.Status != "candidate" || !semver.MatchString(l.Version) || l.Tag != "v"+l.Version || !hex40.MatchString(l.Commit) || l.ConfigAdapterVersion != "UNIMPLEMENTED" || l.RetrievedAt == "" || l.Evidence == "" || len(l.raw) == 0 {
		return ErrLockInvalid
	}
	if p.Artifact != "CLIProxyAPI_"+l.Version+"_windows_amd64.zip" || !hex64.MatchString(p.ArchiveSHA256) || !hex64.MatchString(p.ExecutableSHA256) {
		return ErrLockInvalid
	}
	if !exactURL(l.ReleaseMetadataURL, "/router-for-me/CLIProxyAPI/releases/tag/"+l.Tag) || !exactURL(p.DownloadURL, "/router-for-me/CLIProxyAPI/releases/download/"+l.Tag+"/"+p.Artifact) {
		return ErrLockInvalid
	}
	verified := map[string]bool{
		"published_checksum_matches_archive": true, "github_asset_digest_matches_archive": true,
		"binary_version_matches_tag": true, "loopback_ipv4_bind": true,
		"management_key_required": true, "client_key_required": true,
		"credential_free_start_stop_cleanup": true,
	}
	seen := map[string]bool{}
	for _, capability := range l.VerifiedCapabilities {
		if !verified[capability] || seen[capability] {
			return ErrLockInvalid
		}
		seen[capability] = true
	}
	if len(seen) != len(verified) {
		return ErrLockInvalid
	}
	required := map[string]bool{"credential_specific_model_inventory": false, "provider_specific_response_shapes": false, "dedicated_health_endpoint": false}
	for _, capability := range l.UnverifiedCapabilities {
		if seen[capability] {
			return ErrLockInvalid
		}
		if _, ok := required[capability]; !ok || required[capability] {
			return ErrLockInvalid
		}
		required[capability] = true
	}
	for _, present := range required {
		if !present {
			return ErrLockInvalid
		}
	}
	return nil
}

func exactURL(raw, path string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host == "github.com" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && u.Path == path
}

func duplicateKeys(data []byte) bool {
	d := json.NewDecoder(bytes.NewReader(data))
	objects := []map[string]bool{}
	expectKey := []bool{}
	for {
		t, err := d.Token()
		if errors.Is(err, io.EOF) {
			return false
		}
		if err != nil {
			return true
		}
		switch x := t.(type) {
		case json.Delim:
			switch x {
			case '{':
				objects = append(objects, map[string]bool{})
				expectKey = append(expectKey, true)
			case '[':
				objects = append(objects, nil)
				expectKey = append(expectKey, false)
			case '}', ']':
				objects = objects[:len(objects)-1]
				expectKey = expectKey[:len(expectKey)-1]
				if len(objects) > 0 && objects[len(objects)-1] != nil {
					expectKey[len(expectKey)-1] = true
				}
			}
		case string:
			if len(objects) > 0 && objects[len(objects)-1] != nil && expectKey[len(expectKey)-1] {
				if objects[len(objects)-1][x] {
					return true
				}
				objects[len(objects)-1][x] = true
				expectKey[len(expectKey)-1] = false
			} else if len(objects) > 0 && objects[len(objects)-1] != nil {
				expectKey[len(expectKey)-1] = true
			}
		default:
			if len(objects) > 0 && objects[len(objects)-1] != nil {
				expectKey[len(expectKey)-1] = true
			}
		}
	}
}

func SafeBasename(value string) bool {
	if value == "" || value == "." || value == ".." || strings.ContainsAny(value, `\\/:*?"<>|`+"\x00") || strings.HasSuffix(value, ".") || strings.HasSuffix(value, " ") {
		return false
	}
	name := strings.ToUpper(strings.SplitN(value, ".", 2)[0])
	if name == "CON" || name == "PRN" || name == "AUX" || name == "NUL" || name == "CLOCK$" {
		return false
	}
	return !(len(name) == 4 && (strings.HasPrefix(name, "COM") || strings.HasPrefix(name, "LPT")) && name[3] >= '1' && name[3] <= '9')
}
