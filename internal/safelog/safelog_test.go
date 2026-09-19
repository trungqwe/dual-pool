package safelog

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/trungqwe/dual-pool/internal/apperr"
)

var fixedTime = time.Date(2026, 9, 19, 15, 45, 0, 0, time.FixedZone("test", 7*60*60))

func mustString(t *testing.T, name, value string) Field {
	t.Helper()
	field, err := StringField(name, value)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return field
}

func mustNumber(t *testing.T, name string, value int64) Field {
	t.Helper()
	field, err := NumberField(name, value)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return field
}

func TestAllAllowedFieldsEmitOneDeterministicJSONLine(t *testing.T) {
	fingerprint, err := Fingerprint([]byte("synthetic-key"), "synthetic-session")
	if err != nil {
		t.Fatal(err)
	}
	errorField, err := ErrorCodeField(apperr.CodeDataRootUnavailable)
	if err != nil {
		t.Fatal(err)
	}
	streamField, err := BoolField("stream_started", false)
	if err != nil {
		t.Fatal(err)
	}
	fields := []Field{
		mustString(t, "operation_id", "operation_123"),
		mustString(t, "instance_id", "google_1"),
		mustString(t, "provider", "google"),
		mustString(t, "route_template", "/v1/responses"),
		mustString(t, "method", "POST"),
		mustString(t, "model_id", "gpt-6-astra"),
		mustString(t, "credential_opaque_id", "opaque_12345"),
		mustString(t, "session_fingerprint", fingerprint),
		mustNumber(t, "status_code", 200),
		mustNumber(t, "duration_ms", 123),
		mustNumber(t, "retry_count", 0),
		streamField,
		mustString(t, "bytes_class", "small"),
		errorField,
		mustString(t, "version", "1.2.3"),
		mustString(t, "config_hash_prefix", "abcdef123456"),
	}
	var output bytes.Buffer
	logger := New(&output, func() time.Time { return fixedTime })
	if err := logger.Log(Event{Level: LevelInfo, Component: "poolbridge", Name: "process.ready", Fields: fields}); err != nil {
		t.Fatal(err)
	}
	if bytes.Count(output.Bytes(), []byte("\n")) != 1 || !bytes.HasSuffix(output.Bytes(), []byte("\n")) {
		t.Fatalf("expected one JSON line: %q", output.String())
	}
	var record map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &record); err != nil {
		t.Fatal(err)
	}
	if len(record) != 20 || record["timestamp"] != "2026-09-19T08:45:00Z" || record["stream_started"] != false {
		t.Fatalf("unexpected record shape: %#v", record)
	}
	for _, forbidden := range []string{"synthetic-key", "synthetic-session", "SENTINEL_SECRET", "user@example.invalid"} {
		if strings.Contains(output.String(), forbidden) {
			t.Fatalf("output contains %q", forbidden)
		}
	}
}

func TestUnknownAndForbiddenFieldsRejectBeforeWrite(t *testing.T) {
	for _, name := range []string{"unknown", "authorization", "cookie", "set-cookie", "token", "access_token", "refresh_token", "api_key", "management_key", "client_key", "oauth_state", "email", "prompt", "request_body", "response_body", "tool_arguments", "tool_result", "source", "file_content", "session_id", "conversation_id", "command_line", "env", "environment"} {
		if _, err := StringField(name, "safe"); err == nil {
			t.Errorf("constructor accepted %q", name)
		}
		var output bytes.Buffer
		logger := New(&output, func() time.Time { return fixedTime })
		if err := logger.Log(Event{Level: LevelInfo, Component: "poolbridge", Name: "test.rejected", Fields: []Field{{name: name, kind: stringKind, text: "safe"}}}); err == nil || output.Len() != 0 {
			t.Errorf("logger accepted %q or wrote bytes", name)
		}
	}
}

func TestInvalidMetadataRejectsWithoutWriting(t *testing.T) {
	unknownCode := Field{name: "error_code", kind: errorKind, code: apperr.Code("UNKNOWN_CODE")}
	badRoute := Field{name: "route_template", kind: stringKind, text: "/v1/foo?token=secret"}
	duplicate := mustString(t, "provider", "google")
	cases := []Event{
		{Level: LevelInfo, Component: "poolbridge", Name: "other.ready"},
		{Level: Level("trace"), Component: "poolbridge", Name: "process.ready"},
		{Level: LevelInfo, Component: "poolbridge", Name: "process.ready", Fields: []Field{badRoute}},
		{Level: LevelInfo, Component: "poolbridge", Name: "process.ready", Fields: []Field{unknownCode}},
		{Level: LevelInfo, Component: "poolbridge", Name: "process.ready", Fields: []Field{duplicate, duplicate}},
	}
	for _, event := range cases {
		var output bytes.Buffer
		if err := New(&output, func() time.Time { return fixedTime }).Log(event); err == nil || output.Len() != 0 {
			t.Errorf("invalid event wrote output or succeeded: %#v", event)
		}
	}
	if _, err := ErrorCodeField(apperr.Code("UNKNOWN_CODE")); err == nil {
		t.Fatal("unknown error code accepted")
	}
}

func TestSensitiveValuesRejectEntireEvent(t *testing.T) {
	inputs := []struct{ name, value string }{
		{"model_id", "Bearer SENTINEL_SECRET_12345"},
		{"model_id", "refresh-token-sentinel"},
		{"version", "user@example.invalid"},
		{"route_template", `C:\Users\Example\secret`},
		{"operation_id", "raw-session-sentinel"},
		{"component", "authorization-like-content"},
		{"model_id", "sk-" + strings.Repeat("A", 24)},
	}
	for _, input := range inputs {
		var output bytes.Buffer
		logger := New(&output, func() time.Time { return fixedTime })
		event := Event{Level: LevelInfo, Component: "poolbridge", Name: "test.rejected"}
		if input.name == "component" {
			event.Component = input.value
		} else {
			event.Fields = []Field{{name: input.name, kind: stringKind, text: input.value}}
		}
		if err := logger.Log(event); err == nil || output.Len() != 0 {
			t.Errorf("sensitive %s was not rejected before write", input.name)
		}
	}
}

func TestFieldSpecificBoundariesRejectInvalidValues(t *testing.T) {
	for _, input := range []struct{ name, value string }{
		{"credential_opaque_id", "user@example.invalid"},
		{"credential_opaque_id", `C:\Users\Example\secret`},
		{"route_template", "/v1/foo#fragment"},
		{"route_template", "/v1/foo?query=1"},
		{"bytes_class", "12345"},
		{"config_hash_prefix", "abcdef"},
		{"session_fingerprint", "raw-session-sentinel"},
	} {
		if _, err := StringField(input.name, input.value); err == nil {
			t.Errorf("accepted invalid %s", input.name)
		}
	}
	if _, err := NumberField("status_code", 0); err == nil {
		t.Fatal("accepted invalid status code")
	}
}

type failingWriter struct{ err error }

func (writer failingWriter) Write([]byte) (int, error) { return 0, writer.err }

func TestWriterFailureIsReturned(t *testing.T) {
	want := errors.New("synthetic write failure")
	err := New(failingWriter{want}, func() time.Time { return fixedTime }).Log(Event{Level: LevelInfo, Component: "poolbridge", Name: "test.write"})
	if !errors.Is(err, want) {
		t.Fatalf("writer error lost: %v", err)
	}
}

func TestFingerprintIsKeyedAndNeverReturnsRawInput(t *testing.T) {
	key := []byte("synthetic-key-a")
	raw := "raw-session-sentinel"
	first, err := Fingerprint(key, raw)
	if err != nil {
		t.Fatal(err)
	}
	same, _ := Fingerprint(key, raw)
	otherKey, _ := Fingerprint([]byte("synthetic-key-b"), raw)
	otherInput, _ := Fingerprint(key, "different-session")
	if first != same || first == otherKey || first == otherInput || len(first) != 32 || strings.Contains(first, raw) || strings.Contains(first, string(key)) {
		t.Fatalf("invalid fingerprint relation or shape: %q", first)
	}
	for _, char := range first {
		if !strings.ContainsRune("0123456789abcdef", char) {
			t.Fatalf("non-hex fingerprint: %q", first)
		}
	}
	if _, err := Fingerprint(nil, raw); err == nil {
		t.Fatal("empty key accepted")
	}
	if _, err := Fingerprint(key, ""); err == nil {
		t.Fatal("empty session accepted")
	}
}
