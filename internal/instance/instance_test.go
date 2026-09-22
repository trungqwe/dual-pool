package instance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/trungqwe/dual-pool/internal/cliproxyconfig"
	"github.com/trungqwe/dual-pool/internal/dataroot"
	"github.com/trungqwe/dual-pool/internal/installedslot"
	"github.com/trungqwe/dual-pool/internal/processidentity"
	"github.com/trungqwe/dual-pool/internal/state"
	"github.com/trungqwe/dual-pool/internal/upstreamlock"
)

type activeStateFixture struct {
	value state.State
	err   error
}

func (f activeStateFixture) LoadState() (state.State, error) { return f.value, f.err }

type slotRegistryFixture struct {
	slots map[string]installedslot.ResolvedSlot
	seen  []string
}

func (r *slotRegistryFixture) Resolve(version string) (installedslot.ResolvedSlot, error) {
	r.seen = append(r.seen, version)
	slot, ok := r.slots[version]
	if !ok {
		return installedslot.ResolvedSlot{}, installedslot.ErrSlotUnknown
	}
	return slot, nil
}
func (r *slotRegistryFixture) VerifyInstalled(_ context.Context, version string) error {
	_, ok := r.slots[version]
	if !ok {
		return installedslot.ErrSlotUnknown
	}
	return nil
}
func (r *slotRegistryFixture) RegisterLocked(context.Context, string) error { return nil }

func recordedRegistryFixture(record ProcessRecord, executable string) SlotRegistry {
	return &slotRegistryFixture{slots: map[string]installedslot.ResolvedSlot{
		record.UpstreamVersion: {Version: record.UpstreamVersion, ExecutablePath: executable, ExecutableSHA256: record.ExecutableSHA256, ManifestSHA256: record.ManifestSHA256},
	}}
}

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
	record := ProcessRecord{SchemaVersion: 2, InstanceID: "codex", PID: 42, StartTime: 99, ExecutableSHA256: hex.EncodeToString(sum[:]), ConfigSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Port: cliproxyconfig.CodexPort}
	record.UpstreamVersion = "7.3.7"
	record.ManifestSHA256 = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
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
			m.registry = recordedRegistryFixture(record, exe)
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
	record := ProcessRecord{SchemaVersion: 2, InstanceID: "codex", PID: 42, StartTime: 99, ExecutableSHA256: hex.EncodeToString(sum[:]), ConfigSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Port: cliproxyconfig.CodexPort}
	record.UpstreamVersion = "7.3.7"
	record.ManifestSHA256 = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	h := &fakeTerminationHandle{identity: processidentity.Identity{PID: record.PID, StartTime: record.StartTime, Image: exe}}
	m := &Manager{layout: dataroot.Layout{Bin: bin}, lock: upstreamlock.Lock{Version: "7.3.7"}, opener: func(uint32) (terminationHandle, error) { return h, nil }}
	m.registry = recordedRegistryFixture(record, exe)
	if err := m.stopRecord(record); err != nil {
		t.Fatal(err)
	}
	if !h.terminated || !h.waited || !h.closed {
		t.Fatal("verified handle lifecycle incomplete")
	}
}

func TestProcessRecordRejectsPIDReuseAndUnsafeValues(t *testing.T) {
	valid := ProcessRecord{SchemaVersion: 2, InstanceID: "codex", PID: 42, StartTime: 99, ExecutableSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ConfigSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Port: cliproxyconfig.CodexPort}
	valid.UpstreamVersion = "7.3.7"
	valid.ManifestSHA256 = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
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
	raw, err := os.ReadFile(filepath.Join("..", "..", "upstream.lock"))
	if err != nil {
		t.Fatal(err)
	}
	lock, err := upstreamlock.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	m := &Manager{lock: lock}
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
	record := ProcessRecord{SchemaVersion: 2, InstanceID: "google", PID: 1, StartTime: 1, ExecutableSHA256: "Aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ConfigSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Port: cliproxyconfig.GooglePort}
	record.UpstreamVersion = "7.3.7"
	record.ManifestSHA256 = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	if validRecord(record) {
		t.Fatal("uppercase digest accepted")
	}
}

func TestOldUnboundProcessRecordFailsClosed(t *testing.T) {
	record := ProcessRecord{SchemaVersion: 1, InstanceID: "codex", PID: 1, StartTime: 1, ExecutableSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ConfigSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Port: cliproxyconfig.CodexPort}
	if validRecord(record) {
		t.Fatal("unbound process record accepted")
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

func TestActiveSelectionPassesOnlyStateVersionToRegistry(t *testing.T) {
	for _, version := range []string{"vA", "vB"} {
		t.Run(version, func(t *testing.T) {
			registry := &slotRegistryFixture{slots: map[string]installedslot.ResolvedSlot{version: {Version: version, ExecutablePath: `C:\trusted\` + version + `\cliproxyapi.exe`, ExecutableBasename: "cliproxyapi.exe", ConfigAdapterVersion: "adapter-v1", Platform: "windows_amd64", ExecutableSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ManifestSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}}}
			m := &Manager{lock: upstreamlock.Lock{ConfigAdapterVersion: "adapter-v1"}, registry: registry, state: activeStateFixture{value: state.State{SchemaVersion: 1, InstallID: "install_fixture_01", ActiveUpstreamVersion: version, Instances: state.Instances{Codex: state.Instance{Port: cliproxyconfig.CodexPort, Status: state.InstanceStopped}, Google: state.Instance{Port: cliproxyconfig.GooglePort, Status: state.InstanceStopped}}, Antigravity: state.Antigravity{Mode: state.ModeDisabled, BridgePort: 51074}}}}
			slot, err := m.activeSlot(context.Background())
			if err != nil || slot.Version != version {
				t.Fatalf("active slot: %#v %v", slot, err)
			}
			if len(registry.seen) != 1 || registry.seen[0] != version {
				t.Fatalf("registry observed unexpected versions: %#v", registry.seen)
			}
		})
	}
}

func TestActiveSelectionFailsClosedBeforeLaunchForUnknownOrCorruptState(t *testing.T) {
	registry := &slotRegistryFixture{slots: map[string]installedslot.ResolvedSlot{}}
	m := &Manager{lock: upstreamlock.Lock{ConfigAdapterVersion: "adapter-v1"}, registry: registry, state: activeStateFixture{value: state.State{SchemaVersion: 1, InstallID: "install_fixture_01", ActiveUpstreamVersion: "../x", Instances: state.Instances{Codex: state.Instance{Port: cliproxyconfig.CodexPort, Status: state.InstanceStopped}, Google: state.Instance{Port: cliproxyconfig.GooglePort, Status: state.InstanceStopped}}, Antigravity: state.Antigravity{Mode: state.ModeDisabled, BridgePort: 51074}}}}
	if _, err := m.activeSlot(context.Background()); err == nil {
		t.Fatal("unsafe active state accepted")
	}
	m.state = activeStateFixture{value: state.State{SchemaVersion: 1, InstallID: "install_fixture_01", ActiveUpstreamVersion: "vMissing", Instances: state.Instances{Codex: state.Instance{Port: cliproxyconfig.CodexPort, Status: state.InstanceStopped}, Google: state.Instance{Port: cliproxyconfig.GooglePort, Status: state.InstanceStopped}}, Antigravity: state.Antigravity{Mode: state.ModeDisabled, BridgePort: 51074}}}
	if _, err := m.activeSlot(context.Background()); err == nil {
		t.Fatal("unknown active slot accepted")
	}
}

func TestProcessRecordBindsRecordedSlotAcrossActiveStateChanges(t *testing.T) {
	registry := &slotRegistryFixture{slots: map[string]installedslot.ResolvedSlot{
		"vA": {Version: "vA", ExecutablePath: `C:\trusted\vA\cliproxyapi.exe`, ExecutableSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ManifestSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
		"vB": {Version: "vB", ExecutablePath: `C:\trusted\vB\cliproxyapi.exe`, ExecutableSHA256: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", ManifestSHA256: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"},
	}}
	m := &Manager{registry: registry}
	record := ProcessRecord{SchemaVersion: 2, InstanceID: "codex", PID: 10, StartTime: 20, ExecutableSHA256: registry.slots["vA"].ExecutableSHA256, ConfigSHA256: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", Port: cliproxyconfig.CodexPort, UpstreamVersion: "vA", ManifestSHA256: registry.slots["vA"].ManifestSHA256}
	slot, err := m.recordSlot(record)
	if err != nil || slot.Version != "vA" {
		t.Fatalf("record slot: %#v %v", slot, err)
	}
	if len(registry.seen) != 1 || registry.seen[0] != "vA" {
		t.Fatalf("record resolved unexpected slot: %#v", registry.seen)
	}
}
