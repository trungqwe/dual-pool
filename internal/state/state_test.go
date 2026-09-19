package state

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func validState() State {
	return State{SchemaVersion: StateSchemaVersion, InstallID: "install_fixture_01", Instances: Instances{Codex: Instance{Port: 8317, Status: InstanceStopped}, Google: Instance{Port: 8318, Status: InstanceStopped}}, Antigravity: Antigravity{Mode: ModeDisabled, BridgePort: 51074}, Accounts: []Account{}}
}
func TestStateRoundTripDeterministic(t *testing.T) {
	s := validState()
	s.Instances.Codex.Port = 18001
	s.Instances.Google.Port = 18002
	s.Antigravity.BridgePort = 18003
	s.Accounts = []Account{{OpaqueID: "opaque_fixture_01", Pool: PoolCodex, Nickname: "Tài khoản thử", Enabled: true, Eligibility: EligibilityEligible, ModelIDs: []string{"gpt-fixture"}, LastProbeAt: "2026-09-19T00:00:00Z", LastProbeVersion: "v1.2.3"}}
	a, e := EncodeState(s)
	if e != nil {
		t.Fatal(e)
	}
	b, e := EncodeState(s)
	if e != nil || !bytes.Equal(a, b) || !bytes.HasSuffix(a, []byte("\n")) {
		t.Fatal("non-deterministic encoding")
	}
	got, e := DecodeState(a)
	if e != nil || !reflect.DeepEqual(s, got) {
		t.Fatalf("round trip: %v %#v", e, got)
	}
}
func TestStateValidationRejectsInvalidVariants(t *testing.T) {
	hash := strings.Repeat("a", 64)
	cases := map[string]func(*State){"duplicate ports": func(s *State) { s.Instances.Google.Port = s.Instances.Codex.Port }, "bridge collision": func(s *State) { s.Antigravity.BridgePort = s.Instances.Codex.Port }, "install id": func(s *State) { s.InstallID = "user@example.invalid" }, "mode": func(s *State) { s.Antigravity.Mode = "magic" }, "hash": func(s *State) { s.Instances.Codex.ConfigHash = "unknown" }, "status": func(s *State) { s.Instances.Codex.Status = "running" }, "pool": func(s *State) {
		s.Accounts = []Account{{OpaqueID: "opaque_fixture_01", Pool: "other", Eligibility: EligibilityUnknown}}
	}, "eligibility": func(s *State) {
		s.Accounts = []Account{{OpaqueID: "opaque_fixture_01", Pool: PoolCodex, Eligibility: "maybe"}}
	}, "duplicate models": func(s *State) {
		s.Accounts = []Account{{OpaqueID: "opaque_fixture_01", Pool: PoolCodex, Eligibility: EligibilityUnknown, ModelIDs: []string{"gpt-fixture", "gpt-fixture"}}}
	}, "bad time": func(s *State) {
		s.Accounts = []Account{{OpaqueID: "opaque_fixture_01", Pool: PoolCodex, Eligibility: EligibilityUnknown, LastProbeAt: "yesterday"}}
	}, "duplicate account": func(s *State) {
		a := Account{OpaqueID: "opaque_fixture_01", Pool: PoolCodex, Eligibility: EligibilityUnknown}
		s.Accounts = []Account{a, a}
	}}
	_ = hash
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			s := validState()
			mutate(&s)
			if _, err := EncodeState(s); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}
func TestStateStrictJSONAndSecretExtraFields(t *testing.T) {
	base, _ := EncodeState(validState())
	var root map[string]any
	_ = json.Unmarshal(base, &root)
	for _, field := range []string{"unknown", "token", "email", "auth_json"} {
		root[field] = "sentinel"
		b, _ := json.Marshal(root)
		if _, err := DecodeState(b); err == nil {
			t.Fatalf("accepted %s", field)
		}
	}
	account := `{"schema_version":1,"install_id":"install_fixture_01","active_upstream_version":"","instances":{"codex":{"port":8317,"config_hash":"","status":"stopped"},"google":{"port":8318,"config_hash":"","status":"stopped"}},"antigravity":{"mode":"disabled","bridge_port":51074,"donor_model_id":"","catalog_fingerprint":"","adapter_version":""},"codex":{"adapter_version":"","catalog_fingerprint":""},"accounts":[{"opaque_id":"opaque_fixture_01","pool":"codex","nickname":"","enabled":true,"eligibility":"unknown","reason_code":"","model_ids":[],"last_probe_at":"","last_probe_version":"","token":"x"}]}`
	if _, err := DecodeState([]byte(account)); err == nil {
		t.Fatal("accepted account token")
	}
}
func TestStateCodecRobustness(t *testing.T) {
	valid, _ := EncodeState(validState())
	cases := [][]byte{nil, {}, []byte("   "), []byte("null"), []byte("[]"), append(append([]byte{}, valid...), valid...), []byte(`{"schema_version":1,"schema_version":2}`), []byte(`{"schema_version":1,"instances":{"codex":{"port":1,"port":2}}}`), {0xff, 0xfe}, valid[:len(valid)/2], []byte(`{"schema_version":"1"}`)}
	for i, b := range cases {
		if _, err := DecodeState(b); err == nil {
			t.Fatalf("case %d accepted", i)
		}
	}
	over := bytes.Repeat([]byte(" "), MaxStateDocumentBytes+1)
	if _, err := DecodeState(over); err == nil {
		t.Fatal("oversize accepted")
	}
	atLimit := append(append([]byte(nil), valid...), bytes.Repeat([]byte(" "), MaxStateDocumentBytes-len(valid))...)
	if _, err := DecodeState(atLimit); err != nil {
		t.Fatalf("valid document at size limit rejected: %v", err)
	}
	missingEnabled := bytes.Replace(valid, []byte(`"accounts":[]`), []byte(`"accounts":[{"opaque_id":"opaque_fixture_01","pool":"codex","nickname":"","eligibility":"unknown","reason_code":"","model_ids":[],"last_probe_at":"","last_probe_version":""}]`), 1)
	if _, err := DecodeState(missingEnabled); err == nil {
		t.Fatal("missing required boolean field accepted")
	}
}
func TestProductVersionFailures(t *testing.T) {
	s := validState()
	for _, v := range []int{0, -1, 2} {
		s.SchemaVersion = v
		b, _ := json.Marshal(s)
		if _, err := DecodeState(b); !errors.Is(err, ErrUnsupportedSchemaVersion) {
			t.Fatalf("version %d: %v", v, err)
		}
	}
}
func TestSchemaTagsExcludeSecretFields(t *testing.T) {
	forbidden := []string{"token", "refresh_token", "access_token", "authorization", "cookie", "api_key", "management_key", "client_key", "oauth_state", "prompt", "response_body", "source_content"}
	types := []reflect.Type{reflect.TypeOf(State{}), reflect.TypeOf(Account{}), reflect.TypeOf(Ownership{}), reflect.TypeOf(OwnershipRecord{}), reflect.TypeOf(TypedValue{})}
	for _, typ := range types {
		for i := 0; i < typ.NumField(); i++ {
			tag := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]
			for _, bad := range forbidden {
				if tag == bad {
					t.Fatalf("forbidden tag %s", tag)
				}
			}
		}
	}
}
