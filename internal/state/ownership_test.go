package state

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func strp(v string) *string { return &v }
func boolp(v bool) *bool    { return &v }
func validOwnership() Ownership {
	h := strings.Repeat("a", 64)
	return Ownership{SchemaVersion: OwnershipSchemaVersion, Records: []OwnershipRecord{{TargetPath: `C:\Fixture\config.toml`, Format: FormatTOML, Encoding: EncodingUTF8, BOM: BOMAbsent, Newline: NewlineCRLF, FileIdentity: "fixture_identity_01", PreWriteHash: h, KeyPath: "model_provider", OriginalExisted: false, OriginalValue: TypedValue{Kind: ValueAbsent}, AppliedValue: TypedValue{Kind: ValueString, String: strp("dualpool_codex")}, BackupPath: `C:\Fixture\backup\config.toml`, BackupHash: h, AppliedAt: "2026-09-19T00:00:00Z", ToolVersion: "1.0.0", PostWriteHash: h, RollbackStatus: RollbackApplied}}}
}
func TestOwnershipRoundTripDeterministic(t *testing.T) {
	o := validOwnership()
	a, e := EncodeOwnership(o)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := EncodeOwnership(o)
	got, e := DecodeOwnership(a)
	if e != nil || !bytes.Equal(a, b) || !reflect.DeepEqual(o, got) {
		t.Fatalf("round trip %v", e)
	}
}
func TestOwnershipRejectsInvalidRecords(t *testing.T) {
	cases := map[string]func(*OwnershipRecord){"relative": func(r *OwnershipRecord) { r.TargetPath = `Fixture\x` }, "unc": func(r *OwnershipRecord) { r.TargetPath = `\\server\share\x` }, "hash": func(r *OwnershipRecord) { r.BackupHash = "unknown" }, "timestamp": func(r *OwnershipRecord) { r.AppliedAt = "now" }, "contradiction": func(r *OwnershipRecord) { r.OriginalExisted = true }, "unsafe secret ref": func(r *OwnershipRecord) {
		r.AppliedValue = TypedValue{Kind: ValueSecretRef, SecretRef: strp("user@example.invalid")}
	}, "union": func(r *OwnershipRecord) {
		r.AppliedValue = TypedValue{Kind: ValueString, String: strp("x"), Bool: boolp(true)}
	}}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			o := validOwnership()
			mutate(&o.Records[0])
			if _, e := EncodeOwnership(o); e == nil {
				t.Fatal("accepted")
			}
		})
	}
	o := validOwnership()
	o.Records = append(o.Records, o.Records[0])
	if _, e := EncodeOwnership(o); e == nil {
		t.Fatal("duplicate accepted")
	}
}
func TestTypedValueKinds(t *testing.T) {
	i := int64(7)
	n := 1.5
	values := []TypedValue{{Kind: ValueAbsent}, {Kind: ValueString, String: strp("x")}, {Kind: ValueBool, Bool: boolp(false)}, {Kind: ValueInteger, Integer: &i}, {Kind: ValueNumber, Number: &n}, {Kind: ValueStringMap, StringMap: map[string]string{"model": "x"}}, {Kind: ValueSecretRef, SecretRef: strp("secret_fixture_01")}}
	for _, v := range values {
		if e := validateValue(v); e != nil {
			t.Fatalf("%s: %v", v.Kind, e)
		}
	}
}
func TestOwnershipStrictUnknownAndFuture(t *testing.T) {
	b, _ := EncodeOwnership(validOwnership())
	b = bytes.Replace(b, []byte(`"records"`), []byte(`"extra":true,"records"`), 1)
	if _, e := DecodeOwnership(b); e == nil {
		t.Fatal("unknown accepted")
	}
	o := validOwnership()
	o.SchemaVersion = 2
	b, _ = encode(o)
	if _, e := DecodeOwnership(b); e == nil {
		t.Fatal("future accepted")
	}
}

func TestOwnershipRejectsReservedWindowsDevicePaths(t *testing.T) {
	for _, component := range []string{"CON", "prn.txt", "AUX", "NUL.cfg", "CLOCK$", "COM1", "com9.log", "LPT1", "lpt9.txt"} {
		t.Run(component, func(t *testing.T) {
			o := validOwnership()
			o.Records[0].TargetPath = `C:\Fixture\` + component + `\config.toml`
			if _, err := EncodeOwnership(o); err == nil {
				t.Fatal("reserved device component accepted")
			}
		})
	}
}
