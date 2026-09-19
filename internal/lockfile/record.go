package lockfile

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	recordSchemaVersion = 1
	maxRecordBytes      = 8192
)

var (
	ErrLockHeld              = errors.New("mutation lock is held")
	ErrLockRecordInvalid     = errors.New("mutation lock record is invalid")
	ErrLockOwnerUnverifiable = errors.New("mutation lock owner is unverifiable")
	ErrLockOwnershipLost     = errors.New("mutation lock ownership was lost")
	ErrUnsafeLockArtifact    = errors.New("unsafe mutation lock artifact")
	ErrLockPersistence       = errors.New("mutation lock persistence failed")
	ErrProcessNotFound       = errors.New("process is not running")
	hex32                    = regexp.MustCompile(`^[a-f0-9]{32}$`)
	hex64                    = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

type lockKind string

const (
	kindGlobal lockKind = "global"
	kindFile   lockKind = "file"
)

type record struct {
	SchemaVersion  int      `json:"schema_version"`
	Kind           lockKind `json:"kind"`
	ResourceID     string   `json:"resource_id"`
	OwnerPID       uint32   `json:"owner_pid"`
	OwnerStartTime uint64   `json:"owner_start_time"`
	OwnerImage     string   `json:"owner_image"`
	OperationID    string   `json:"operation_id"`
	CreatedAt      string   `json:"created_at"`
	ExpiresAt      string   `json:"expires_at"`
}

func encodeRecord(value record) ([]byte, error) {
	if !validRecord(value) {
		return nil, ErrLockRecordInvalid
	}
	b, err := json.Marshal(value)
	if err != nil {
		return nil, ErrLockRecordInvalid
	}
	return append(b, '\n'), nil
}
func decodeRecord(data []byte) (record, error) {
	var value record
	if len(data) == 0 || len(data) > maxRecordBytes || !utf8.Valid(data) || duplicateKeys(data) {
		return value, ErrLockRecordInvalid
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if d.Decode(&value) != nil {
		return value, ErrLockRecordInvalid
	}
	var extra any
	if !errors.Is(d.Decode(&extra), io.EOF) {
		return value, ErrLockRecordInvalid
	}
	if !validRecord(value) {
		return value, ErrLockRecordInvalid
	}
	return value, nil
}
func validRecord(v record) bool {
	if v.SchemaVersion != recordSchemaVersion || v.OwnerPID == 0 || v.OwnerStartTime == 0 || !validCanonicalImage(v.OwnerImage) || !hex32.MatchString(v.OperationID) {
		return false
	}
	if v.Kind == kindGlobal {
		if v.ResourceID != "global" {
			return false
		}
	} else if v.Kind == kindFile {
		if !hex64.MatchString(v.ResourceID) {
			return false
		}
	} else {
		return false
	}
	created, e1 := time.Parse(time.RFC3339Nano, v.CreatedAt)
	expires, e2 := time.Parse(time.RFC3339Nano, v.ExpiresAt)
	return e1 == nil && e2 == nil && created.Location() == time.UTC && expires.Location() == time.UTC && expires.After(created)
}

func validCanonicalImage(value string) bool {
	return localAbsolute(value) && filepath.Clean(value) == value && strings.ToLower(value) == value
}
func duplicateKeys(data []byte) bool {
	d := json.NewDecoder(bytes.NewReader(data))
	stack := []map[string]bool{}
	keys := []bool{}
	for {
		token, err := d.Token()
		if errors.Is(err, io.EOF) {
			return false
		}
		if err != nil {
			return true
		}
		switch x := token.(type) {
		case json.Delim:
			switch x {
			case '{':
				stack = append(stack, map[string]bool{})
				keys = append(keys, true)
			case '}':
				stack = stack[:len(stack)-1]
				keys = keys[:len(keys)-1]
				if len(keys) > 0 && stack[len(stack)-1] != nil {
					keys[len(keys)-1] = true
				}
			case '[':
				stack = append(stack, nil)
				keys = append(keys, false)
			case ']':
				stack = stack[:len(stack)-1]
				keys = keys[:len(keys)-1]
				if len(keys) > 0 && stack[len(stack)-1] != nil {
					keys[len(keys)-1] = true
				}
			}
		case string:
			if len(stack) > 0 && stack[len(stack)-1] != nil && keys[len(keys)-1] {
				if stack[len(stack)-1][x] {
					return true
				}
				stack[len(stack)-1][x] = true
				keys[len(keys)-1] = false
			} else if len(stack) > 0 && stack[len(stack)-1] != nil {
				keys[len(keys)-1] = true
			}
		default:
			if len(stack) > 0 && stack[len(stack)-1] != nil {
				keys[len(keys)-1] = true
			}
		}
	}
}
