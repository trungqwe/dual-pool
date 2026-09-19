package state

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

type synthetic struct {
	Version int    `json:"schema_version"`
	Value   string `json:"value"`
}

func syntheticChain(steps map[int]MigrationStep) MigrationChain {
	return MigrationChain{Steps: steps, Version: func(b []byte) (int, error) { var d synthetic; e := json.Unmarshal(b, &d); return d.Version, e }, Validate: func(b []byte, v int) error {
		var d synthetic
		if json.Unmarshal(b, &d) != nil || d.Version != v || d.Value == "invalid" {
			return ErrInvalidDocument
		}
		return nil
	}}
}
func stepTo(v int, value string) MigrationStep {
	return func([]byte) ([]byte, error) { return json.Marshal(synthetic{Version: v, Value: value}) }
}
func TestMigrationChainSequentialDeterministic(t *testing.T) {
	input, _ := json.Marshal(synthetic{Version: 7, Value: "start"})
	m := syntheticChain(map[int]MigrationStep{7: stepTo(8, "middle"), 8: stepTo(9, "done")})
	a, e := m.Migrate(input, 9)
	if e != nil {
		t.Fatal(e)
	}
	b, e := m.Migrate(input, 9)
	if e != nil || !bytes.Equal(a, b) {
		t.Fatal("not deterministic")
	}
	var got synthetic
	_ = json.Unmarshal(a, &got)
	if got.Version != 9 || got.Value != "done" {
		t.Fatalf("%+v", got)
	}
}
func TestMigrationFailuresAndInputImmutability(t *testing.T) {
	original, _ := json.Marshal(synthetic{Version: 7, Value: "start"})
	tests := map[string]struct {
		m      MigrationChain
		target int
	}{"missing": {syntheticChain(nil), 9}, "same": {syntheticChain(map[int]MigrationStep{7: stepTo(7, "x")}), 8}, "jump": {syntheticChain(map[int]MigrationStep{7: stepTo(9, "x")}), 8}, "downgrade step": {syntheticChain(map[int]MigrationStep{7: stepTo(6, "x")}), 8}, "invalid output": {syntheticChain(map[int]MigrationStep{7: stepTo(8, "invalid")}), 8}, "downgrade target": {syntheticChain(nil), 6}}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			input := append([]byte(nil), original...)
			if _, e := tc.m.Migrate(input, tc.target); e == nil {
				t.Fatal("accepted")
			}
			if !bytes.Equal(input, original) {
				t.Fatal("input mutated")
			}
		})
	}
	for _, v := range []int{0, -1} {
		b, _ := json.Marshal(synthetic{Version: v, Value: "x"})
		if _, e := syntheticChain(nil).Migrate(b, 9); !errors.Is(e, ErrMigrationInvalid) {
			t.Fatalf("version %d: %v", v, e)
		}
	}
	future, _ := json.Marshal(synthetic{Version: 10, Value: "x"})
	if _, e := syntheticChain(nil).Migrate(future, 9); !errors.Is(e, ErrUnsupportedSchemaVersion) {
		t.Fatal(e)
	}
}
