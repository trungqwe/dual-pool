package configtxn

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"sort"
	"unicode/utf8"
)

const maxMarkerBytes = 16 << 10

var hex64 = regexp.MustCompile(`^[a-f0-9]{64}$`)
var hex32 = regexp.MustCompile(`^[a-f0-9]{32}$`)

type marker struct {
	SchemaVersion    int      `json:"schema_version"`
	Operation        string   `json:"operation"`
	TransactionID    string   `json:"transaction_id"`
	TargetResourceID string   `json:"target_resource_id"`
	PreHash          string   `json:"pre_hash"`
	PostHash         string   `json:"post_hash"`
	BackupBasename   string   `json:"backup_basename"`
	BackupHash       string   `json:"backup_hash"`
	OwnedKeyPaths    []string `json:"owned_key_paths"`
}

func encodeMarker(m marker) ([]byte, error) {
	if !validMarker(m) {
		return nil, ErrRecoveryUnresolved
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, ErrRecoveryUnresolved
	}
	return append(b, '\n'), nil
}
func decodeMarker(data []byte) (marker, error) {
	var m marker
	if len(data) == 0 || len(data) > maxMarkerBytes || !utf8.Valid(data) || rejectDuplicates(data) != nil {
		return m, ErrRecoveryUnresolved
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if d.Decode(&m) != nil {
		return m, ErrRecoveryUnresolved
	}
	var x any
	if !errors.Is(d.Decode(&x), io.EOF) || !validMarker(m) {
		return m, ErrRecoveryUnresolved
	}
	return m, nil
}
func validMarker(m marker) bool {
	if m.SchemaVersion != 1 || !oneOf(m.Operation, "apply", "rollback") || !hex32.MatchString(m.TransactionID) || !hex64.MatchString(m.TargetResourceID) || !hex64.MatchString(m.PreHash) || !hex64.MatchString(m.PostHash) || !hex64.MatchString(m.BackupHash) || !regexp.MustCompile(`^config-[a-f0-9]{32}\.bak$`).MatchString(m.BackupBasename) {
		return false
	}
	want := append([]string(nil), m.OwnedKeyPaths...)
	sort.Strings(want)
	if len(want) < 3 || len(want) > 4 {
		return false
	}
	for i, k := range want {
		if i > 0 && want[i-1] == k {
			return false
		}
		if !oneOf(k, keyModel, keyModelProvider, keyProviderTable, keyCatalog) {
			return false
		}
	}
	return true
}
func oneOf(v string, values ...string) bool {
	for _, x := range values {
		if v == x {
			return true
		}
	}
	return false
}
func rejectDuplicates(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	var stack []map[string]bool
	var expect []bool
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		switch x := tok.(type) {
		case json.Delim:
			switch x {
			case '{':
				stack = append(stack, map[string]bool{})
				expect = append(expect, true)
			case '}':
				stack = stack[:len(stack)-1]
				expect = expect[:len(expect)-1]
				if len(expect) > 0 && stack[len(stack)-1] != nil {
					expect[len(expect)-1] = true
				}
			case '[':
				stack = append(stack, nil)
				expect = append(expect, false)
			case ']':
				stack = stack[:len(stack)-1]
				expect = expect[:len(expect)-1]
				if len(expect) > 0 && stack[len(stack)-1] != nil {
					expect[len(expect)-1] = true
				}
			}
		case string:
			if len(stack) > 0 && stack[len(stack)-1] != nil && expect[len(expect)-1] {
				if stack[len(stack)-1][x] {
					return ErrRecoveryUnresolved
				}
				stack[len(stack)-1][x] = true
				expect[len(expect)-1] = false
			} else if len(stack) > 0 && stack[len(stack)-1] != nil {
				expect[len(expect)-1] = true
			}
		default:
			if len(stack) > 0 && stack[len(stack)-1] != nil {
				expect[len(expect)-1] = true
			}
		}
	}
}
