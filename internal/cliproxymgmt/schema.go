//go:build windows

package cliproxymgmt

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"time"
)

func parseDebug(b []byte) error {
	if duplicateKeys(b) {
		return ErrContract
	}
	var value struct {
		Debug *bool `json:"debug"`
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&value) != nil || value.Debug == nil || *value.Debug {
		return ErrContract
	}
	var extra any
	if !errors.Is(d.Decode(&extra), io.EOF) {
		return ErrContract
	}
	return nil
}

func parseEmptyInventory(b []byte) error {
	if duplicateKeys(b) {
		return ErrContract
	}
	var value struct {
		ObservedAt *time.Time       `json:"observed_at"`
		Files      *json.RawMessage `json:"files"`
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&value) != nil || value.ObservedAt == nil || value.ObservedAt.IsZero() || value.Files == nil {
		return ErrContract
	}
	var files []json.RawMessage
	if json.Unmarshal(*value.Files, &files) != nil || files == nil {
		return ErrContract
	}
	var extra any
	if !errors.Is(d.Decode(&extra), io.EOF) {
		return ErrContract
	}
	if len(files) != 0 {
		return ErrUnexpectedAuthInventory
	}
	return nil
}

func duplicateKeys(b []byte) bool {
	d := json.NewDecoder(bytes.NewReader(b))
	stack := []map[string]bool{}
	wants := []bool{}
	for {
		tok, err := d.Token()
		if err != nil {
			return false
		}
		switch x := tok.(type) {
		case json.Delim:
			if x == '{' {
				stack = append(stack, map[string]bool{})
				wants = append(wants, true)
			} else if x == '[' {
				stack = append(stack, nil)
				wants = append(wants, false)
			} else {
				stack = stack[:len(stack)-1]
				wants = wants[:len(wants)-1]
				if len(stack) > 0 && stack[len(stack)-1] != nil {
					wants[len(wants)-1] = true
				}
			}
		case string:
			if len(stack) > 0 && stack[len(stack)-1] != nil && wants[len(wants)-1] {
				if stack[len(stack)-1][x] {
					return true
				}
				stack[len(stack)-1][x] = true
				wants[len(wants)-1] = false
			} else if len(stack) > 0 && stack[len(stack)-1] != nil {
				wants[len(wants)-1] = true
			}
		default:
			if len(stack) > 0 && stack[len(stack)-1] != nil {
				wants[len(wants)-1] = true
			}
		}
	}
}
