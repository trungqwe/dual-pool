package state

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"
)

func DecodeState(data []byte) (State, error) {
	var out State
	if err := strictDecode(data, MaxStateDocumentBytes, &out); err != nil {
		return State{}, err
	}
	if err := requireStateFields(data); err != nil {
		return State{}, err
	}
	if err := ValidateState(out); err != nil {
		return State{}, err
	}
	return out, nil
}
func DecodeOwnership(data []byte) (Ownership, error) {
	var out Ownership
	if err := strictDecode(data, MaxOwnershipDocumentBytes, &out); err != nil {
		return Ownership{}, err
	}
	if err := requireOwnershipFields(data); err != nil {
		return Ownership{}, err
	}
	if err := ValidateOwnership(out); err != nil {
		return Ownership{}, err
	}
	return out, nil
}
func EncodeState(v State) ([]byte, error) {
	if err := ValidateState(v); err != nil {
		return nil, err
	}
	return encode(v)
}
func EncodeOwnership(v Ownership) ([]byte, error) {
	if err := ValidateOwnership(v); err != nil {
		return nil, err
	}
	return encode(v)
}
func encode(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, ErrInvalidDocument
	}
	return append(b, '\n'), nil
}
func strictDecode(data []byte, limit int, out any) error {
	if len(data) == 0 || len(data) > limit || !utf8.Valid(data) {
		return ErrInvalidDocument
	}
	if err := rejectDuplicateKeys(data); err != nil {
		return ErrInvalidDocument
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return classifyDecode(err)
	}
	var extra any
	if err := d.Decode(&extra); !errors.Is(err, io.EOF) {
		return ErrInvalidDocument
	}
	return nil
}
func classifyDecode(err error) error {
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		return ErrInvalidDocument
	}
	return ErrInvalidDocument
}
func rejectDuplicateKeys(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	stack := []map[string]struct{}{}
	expectKey := []bool{}
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		switch x := tok.(type) {
		case json.Delim:
			switch x {
			case '{':
				stack = append(stack, map[string]struct{}{})
				expectKey = append(expectKey, true)
			case '}':
				stack = stack[:len(stack)-1]
				expectKey = expectKey[:len(expectKey)-1]
				if len(expectKey) > 0 {
					expectKey[len(expectKey)-1] = true
				}
			case '[':
				stack = append(stack, nil)
				expectKey = append(expectKey, false)
			case ']':
				stack = stack[:len(stack)-1]
				expectKey = expectKey[:len(expectKey)-1]
				if len(expectKey) > 0 && stack[len(stack)-1] != nil {
					expectKey[len(expectKey)-1] = true
				}
			}
		case string:
			if len(stack) > 0 && stack[len(stack)-1] != nil && expectKey[len(expectKey)-1] {
				if _, ok := stack[len(stack)-1][x]; ok {
					return ErrInvalidDocument
				}
				stack[len(stack)-1][x] = struct{}{}
				expectKey[len(expectKey)-1] = false
			} else if len(stack) > 0 && stack[len(stack)-1] != nil {
				expectKey[len(expectKey)-1] = true
			}
		default:
			if len(stack) > 0 && stack[len(stack)-1] != nil {
				expectKey[len(expectKey)-1] = true
			}
		}
	}
	return nil
}

func requireStateFields(data []byte) error {
	var root map[string]json.RawMessage
	if json.Unmarshal(data, &root) != nil {
		return ErrInvalidDocument
	}
	if !has(root, "schema_version", "install_id", "active_upstream_version", "instances", "antigravity", "codex", "accounts") {
		return ErrInvalidDocument
	}
	var instances map[string]json.RawMessage
	if json.Unmarshal(root["instances"], &instances) != nil || !has(instances, "codex", "google") {
		return ErrInvalidDocument
	}
	for _, name := range []string{"codex", "google"} {
		var x map[string]json.RawMessage
		if json.Unmarshal(instances[name], &x) != nil || !has(x, "port", "config_hash", "status") {
			return ErrInvalidDocument
		}
	}
	var ag map[string]json.RawMessage
	if json.Unmarshal(root["antigravity"], &ag) != nil || !has(ag, "mode", "bridge_port", "donor_model_id", "catalog_fingerprint", "adapter_version") {
		return ErrInvalidDocument
	}
	var cx map[string]json.RawMessage
	if json.Unmarshal(root["codex"], &cx) != nil || !has(cx, "adapter_version", "catalog_fingerprint") {
		return ErrInvalidDocument
	}
	var accounts []map[string]json.RawMessage
	if json.Unmarshal(root["accounts"], &accounts) != nil {
		return ErrInvalidDocument
	}
	for _, a := range accounts {
		if !has(a, "opaque_id", "pool", "nickname", "enabled", "eligibility", "reason_code", "model_ids", "last_probe_at", "last_probe_version") {
			return ErrInvalidDocument
		}
	}
	return nil
}
func requireOwnershipFields(data []byte) error {
	var root map[string]json.RawMessage
	if json.Unmarshal(data, &root) != nil || !has(root, "schema_version", "records") {
		return ErrInvalidDocument
	}
	var records []map[string]json.RawMessage
	if json.Unmarshal(root["records"], &records) != nil {
		return ErrInvalidDocument
	}
	names := []string{"target_path", "format", "encoding", "bom", "newline", "file_identity", "pre_write_hash", "key_path", "original_existed", "original_value", "applied_value", "backup_path", "backup_hash", "applied_at", "tool_version", "post_write_hash", "rollback_status"}
	for _, r := range records {
		if !has(r, names...) {
			return ErrInvalidDocument
		}
	}
	return nil
}
func has(v map[string]json.RawMessage, names ...string) bool {
	for _, n := range names {
		if _, ok := v[n]; !ok {
			return false
		}
	}
	return true
}
