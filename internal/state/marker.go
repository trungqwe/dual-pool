package state

import (
	"encoding/json"
	"regexp"
)

const (
	markerSchemaVersion = 1
	maxMarkerBytes      = 4096
)

type documentKind string

const (
	documentState     documentKind = "state"
	documentOwnership documentKind = "ownership"
)

type recoveryMarker struct {
	SchemaVersion     int          `json:"schema_version"`
	DocumentKind      documentKind `json:"document_kind"`
	TransactionID     string       `json:"transaction_id"`
	OldExists         bool         `json:"old_exists"`
	OldSHA256         string       `json:"old_sha256"`
	NewSHA256         string       `json:"new_sha256"`
	CandidateBasename string       `json:"candidate_basename"`
	BackupBasename    string       `json:"backup_basename"`
}

var transactionPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

func encodeMarker(marker recoveryMarker) ([]byte, error) {
	if !validMarker(marker) {
		return nil, ErrRecoveryMarkerInvalid
	}
	payload, err := json.Marshal(marker)
	if err != nil {
		return nil, ErrRecoveryMarkerInvalid
	}
	return append(payload, '\n'), nil
}

func decodeMarker(data []byte, expected documentKind) (recoveryMarker, error) {
	var marker recoveryMarker
	if err := strictDecode(data, maxMarkerBytes, &marker); err != nil {
		return recoveryMarker{}, ErrRecoveryMarkerInvalid
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil || !has(fields, "schema_version", "document_kind", "transaction_id", "old_exists", "old_sha256", "new_sha256", "candidate_basename", "backup_basename") || len(fields) != 8 || marker.DocumentKind != expected || !validMarker(marker) {
		return recoveryMarker{}, ErrRecoveryMarkerInvalid
	}
	return marker, nil
}

func validMarker(marker recoveryMarker) bool {
	if marker.SchemaVersion != markerSchemaVersion || !oneOf(string(marker.DocumentKind), string(documentState), string(documentOwnership)) || !transactionPattern.MatchString(marker.TransactionID) || !hashPattern.MatchString(marker.NewSHA256) {
		return false
	}
	prefix := "." + documentFilename(marker.DocumentKind)
	if marker.CandidateBasename != prefix+".tmp-"+marker.TransactionID {
		return false
	}
	if marker.OldExists {
		return hashPattern.MatchString(marker.OldSHA256) && marker.BackupBasename == prefix+".bak-"+marker.TransactionID
	}
	return marker.OldSHA256 == "" && marker.BackupBasename == ""
}

func mustMarker(v recoveryMarker) []byte { b, _ := encodeMarker(v); return b }
