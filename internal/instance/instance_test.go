package instance

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/trungqwe/dual-pool/internal/cliproxyconfig"
	"github.com/trungqwe/dual-pool/internal/dataroot"
	"github.com/trungqwe/dual-pool/internal/processidentity"
	"github.com/trungqwe/dual-pool/internal/upstreamlock"
)

type fakeTerminationHandle struct {
	identity                          processidentity.Identity
	inspectErr, terminateErr, waitErr error
	terminated, waited, closed        bool
}

func (h *fakeTerminationHandle) Inspect() (processidentity.Identity, error) {
	return h.identity, h.inspectErr
}
func (h *fakeTerminationHandle) Terminate() error  { h.terminated = true; return h.terminateErr }
func (h *fakeTerminationHandle) Wait(uint32) error { h.waited = true; return h.waitErr }
func (h *fakeTerminationHandle) Close() error      { h.closed = true; return nil }

func TestStopRecordUsesOneVerifiedHandle(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "bin")
	dir := filepath.Join(bin, "cliproxyapi", "7.3.7")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "cliproxyapi.exe")
	if err := os.WriteFile(exe, []byte("fixture executable"), 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("fixture executable"))
	record := ProcessRecord{SchemaVersion: 1, InstanceID: "codex", PID: 42, StartTime: 99, ExecutableSHA256: hex.EncodeToString(sum[:]), ConfigSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Port: cliproxyconfig.CodexPort}
	for name, live := range map[string]processidentity.Identity{
		"PID reuse":      {PID: 42, StartTime: 100, Image: exe},
		"image mismatch": {PID: 42, StartTime: 99, Image: filepath.Join(dir, "foreign.exe")},
	} {
		t.Run(name, func(t *testing.T) {
			h := &fakeTerminationHandle{identity: live}
			opened := 0
			m := &Manager{layout: dataroot.Layout{Bin: bin}, lock: upstreamlock.Lock{Version: "7.3.7"}, opener: func(pid uint32) (terminationHandle, error) {
				opened++
				if pid != record.PID {
					t.Fatal("wrong PID")
				}
				return h, nil
			}}
			if err := m.stopRecord(record); err != ErrIdentityMismatch {
				t.Fatalf("got %v", err)
			}
			if opened != 1 || h.terminated || h.waited || !h.closed {
				t.Fatal("unverified handle was used to terminate")
			}
		})
	}
}

func TestStopRecordTerminatesTheVerifiedHandle(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "bin")
	dir := filepath.Join(bin, "cliproxyapi", "7.3.7")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "cliproxyapi.exe")
	if err := os.WriteFile(exe, []byte("fixture executable"), 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("fixture executable"))
	record := ProcessRecord{SchemaVersion: 1, InstanceID: "codex", PID: 42, StartTime: 99, ExecutableSHA256: hex.EncodeToString(sum[:]), ConfigSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Port: cliproxyconfig.CodexPort}
	h := &fakeTerminationHandle{identity: processidentity.Identity{PID: record.PID, StartTime: record.StartTime, Image: exe}}
	m := &Manager{layout: dataroot.Layout{Bin: bin}, lock: upstreamlock.Lock{Version: "7.3.7"}, opener: func(uint32) (terminationHandle, error) { return h, nil }}
	if err := m.stopRecord(record); err != nil {
		t.Fatal(err)
	}
	if !h.terminated || !h.waited || !h.closed {
		t.Fatal("verified handle lifecycle incomplete")
	}
}

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
