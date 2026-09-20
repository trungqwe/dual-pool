package instance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/trungqwe/dual-pool/internal/cliproxyconfig"
	"github.com/trungqwe/dual-pool/internal/upstreamlock"
)

func TestProcessRecordRejectsPIDReuseAndUnsafeValues(t *testing.T) {
	valid := ProcessRecord{SchemaVersion: 1, InstanceID: "codex", PID: 42, StartTime: 99, ExecutableSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ConfigSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Port: cliproxyconfig.CodexPort}
	// Hash shape alone is insufficient to identify a new process: Status also
	// compares creation FILETIME and canonical executable image.
	for _, mutate := range []func(*ProcessRecord){
		func(v *ProcessRecord) { v.PID = 0 },
		func(v *ProcessRecord) { v.StartTime = 0 },
		func(v *ProcessRecord) { v.InstanceID = "foreign" },
		func(v *ProcessRecord) { v.Port = 1 },
	} {
		v := valid
		mutate(&v)
		if validRecord(v) {
			t.Fatal("unsafe process record accepted")
		}
	}
}

func TestInstallMarkerTransactionIDValidation(t *testing.T) {
	m := &Manager{lock: upstreamlock.Lock{Version: "7.3.7", ConfigAdapterVersion: "dualpool-cpa-v7.3.7-config-v1"}}
	txn, err := randomTransaction()
	if err != nil {
		t.Fatal(err)
	}
	if !validMarker(m.marker(txn, "."+m.lock.Version+".install-"+txn), m) {
		t.Fatal("P2-INSTALL-MARKER-TXN-001: generated transaction rejected")
	}
	for _, bad := range []string{txn[:31], txn + "a", "A" + txn[1:], "z" + txn[1:]} {
		if validMarker(m.marker(bad, "."+m.lock.Version+".install-"+bad), m) {
			t.Fatalf("bad transaction accepted: %q", bad)
		}
	}
}

func TestReadJSONRejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "record.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"unexpected":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	var record ProcessRecord
	if err := readJSON(path, &record); err == nil {
		t.Fatal("unknown record field accepted")
	}
}

func TestReadJSONRejectsDuplicateKeysAndMultipleDocuments(t *testing.T) {
	for _, payload := range []string{
		`{"schema_version":1,"schema_version":2}`,
		`{"schema_version":1} {}`,
	} {
		path := filepath.Join(t.TempDir(), "record.json")
		if err := os.WriteFile(path, []byte(payload), 0600); err != nil {
			t.Fatal(err)
		}
		var record ProcessRecord
		if err := readJSON(path, &record); err == nil {
			t.Fatalf("unsafe JSON accepted: %s", payload)
		}
	}
}

func TestProcessRecordRequiresLowercaseHexDigests(t *testing.T) {
	record := ProcessRecord{SchemaVersion: 1, InstanceID: "google", PID: 1, StartTime: 1, ExecutableSHA256: "Aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ConfigSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Port: cliproxyconfig.GooglePort}
	if validRecord(record) {
		t.Fatal("uppercase digest accepted")
	}
}

func TestMinimalEnvironmentNeverCopiesSecrets(t *testing.T) {
	t.Setenv("DUALPOOL_TEST_SECRET", "must-not-propagate")
	for _, value := range minimalEnv() {
		if value == "DUALPOOL_TEST_SECRET=must-not-propagate" {
			t.Fatal("child inherited arbitrary environment")
		}
	}
}
