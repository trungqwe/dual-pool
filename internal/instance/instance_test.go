package instance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/trungqwe/dual-pool/internal/cliproxyconfig"
)

func TestProcessRecordRejectsPIDReuseAndUnsafeValues(t *testing.T) {
	valid := ProcessRecord{SchemaVersion: 1, InstanceID: "codex", PID: 42, StartTime: 99, ExecutableSHA256: string(make([]byte, 64)), ExecutableImage: `c:\dualpool\cliproxyapi.exe`, ConfigSHA256: string(make([]byte, 64)), Port: cliproxyconfig.CodexPort}
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

func TestMinimalEnvironmentNeverCopiesSecrets(t *testing.T) {
	t.Setenv("DUALPOOL_TEST_SECRET", "must-not-propagate")
	for _, value := range minimalEnv() {
		if value == "DUALPOOL_TEST_SECRET=must-not-propagate" {
			t.Fatal("child inherited arbitrary environment")
		}
	}
}
