package upstreamcatalog

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"io"
	"time"

	"github.com/trungqwe/dual-pool/internal/upstreamlock"
)

const verifiedV738EvidenceID = "phase-2-v7.3.8-verification"
const verifiedV738ReceiptSHA256 = "0e653e4f01e00c05a44c662e7a7b7321916e7705c901db370aec3cb1116a9776"

//go:embed trust/v7.3.8.json
var verifiedV738Receipt []byte

// Production combines the current pinned release with the one separately
// reviewed v7.3.8 receipt. It never accepts caller supplied trust data.
func Production(lock upstreamlock.Lock) (*Catalog, error) {
	current, err := FromPinnedLock(lock)
	if err != nil {
		return nil, ErrInvalidCatalog
	}
	v737, err := current.Resolve(lock.Version)
	if err != nil {
		return nil, ErrInvalidCatalog
	}
	v738, err := parseVerifiedV738Receipt(verifiedV738Receipt)
	if err != nil {
		return nil, ErrInvalidCatalog
	}
	return newVerified([]Provenance{v737, v738})
}

func parseVerifiedV738Receipt(raw []byte) (Provenance, error) {
	if len(raw) == 0 || len(raw) > 4096 || !bytes.Equal(raw, bytes.ToValidUTF8(raw, []byte("?"))) {
		return Provenance{}, ErrInvalidCatalog
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != verifiedV738ReceiptSHA256 {
		return Provenance{}, ErrInvalidCatalog
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return Provenance{}, ErrInvalidCatalog
	}
	fields := map[string]json.RawMessage{}
	for dec.More() {
		key, err := dec.Token()
		name, ok := key.(string)
		if err != nil || !ok || fields[name] != nil {
			return Provenance{}, ErrInvalidCatalog
		}
		var value json.RawMessage
		if dec.Decode(&value) != nil {
			return Provenance{}, ErrInvalidCatalog
		}
		fields[name] = value
	}
	if tok, err = dec.Token(); err != nil || tok != json.Delim('}') || dec.More() {
		return Provenance{}, ErrInvalidCatalog
	}
	var trailing any
	if dec.Decode(&trailing) != io.EOF {
		return Provenance{}, ErrInvalidCatalog
	}
	required := []string{"schema_version", "product", "status", "version", "tag", "commit", "platform", "artifact", "download_url", "archive_sha256", "executable_sha256", "config_adapter_version", "release_metadata_url", "verified_at", "verification_evidence_id"}
	if len(fields) != len(required) {
		return Provenance{}, ErrInvalidCatalog
	}
	for _, name := range required {
		if fields[name] == nil {
			return Provenance{}, ErrInvalidCatalog
		}
	}
	var schema int
	if json.Unmarshal(fields["schema_version"], &schema) != nil || schema != 1 {
		return Provenance{}, ErrInvalidCatalog
	}
	strings := make(map[string]string, len(required)-1)
	for _, name := range required[1:] {
		var value string
		if json.Unmarshal(fields[name], &value) != nil {
			return Provenance{}, ErrInvalidCatalog
		}
		strings[name] = value
	}
	if strings["product"] != "CLIProxyAPI" || strings["status"] != "verified" || strings["version"] != "7.3.8" || strings["tag"] != "v7.3.8" || strings["commit"] != "c93978c4ea2e908255a2a06c37599fda3651554a" || strings["platform"] != "windows_amd64" || strings["artifact"] != "CLIProxyAPI_7.3.8_windows_amd64.zip" || strings["config_adapter_version"] != upstreamlock.ConfigAdapterVersion || strings["verification_evidence_id"] != verifiedV738EvidenceID {
		return Provenance{}, ErrInvalidCatalog
	}
	if _, err := time.Parse(time.RFC3339, strings["verified_at"]); err != nil {
		return Provenance{}, ErrInvalidCatalog
	}
	p := Provenance{Product: strings["product"], Version: strings["version"], Tag: strings["tag"], Commit: strings["commit"], Platform: strings["platform"], Artifact: strings["artifact"], DownloadURL: strings["download_url"], ArchiveSHA256: strings["archive_sha256"], ExecutableSHA256: strings["executable_sha256"], ConfigAdapterVersion: strings["config_adapter_version"], Digest: verifiedV738ReceiptSHA256, ReleaseMetadataURL: strings["release_metadata_url"]}
	if !valid(p) {
		return Provenance{}, ErrInvalidCatalog
	}
	return p, nil
}
