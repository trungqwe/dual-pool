//go:build windows

package instance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/trungqwe/dual-pool/internal/cliproxyconfig"
	"github.com/trungqwe/dual-pool/internal/dataroot"
	"github.com/trungqwe/dual-pool/internal/installedslot"
	"github.com/trungqwe/dual-pool/internal/lockfile"
	"github.com/trungqwe/dual-pool/internal/state"
	"github.com/trungqwe/dual-pool/internal/upstreamcatalog"
	"github.com/trungqwe/dual-pool/internal/upstreamlock"
	"github.com/trungqwe/dual-pool/internal/upstreamstage"
	"github.com/trungqwe/dual-pool/internal/winacl"
)

func candidateInstallLock(t *testing.T) upstreamlock.Lock {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "upstream.lock"))
	if err != nil {
		t.Fatal(err)
	}
	lock, err := upstreamlock.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return lock
}

func syntheticInstallProvenance(executable []byte) upstreamcatalog.Provenance {
	executableSum := sha256.Sum256(executable)
	archiveSum := sha256.Sum256([]byte("synthetic archive B"))
	digestSum := sha256.Sum256([]byte("synthetic provenance B"))
	return upstreamcatalog.Provenance{
		Product: "CLIProxyAPI", Version: "vB", Tag: "vvB", Commit: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Platform: "windows_amd64", Artifact: "CLIProxyAPI_vB_windows_amd64.zip",
		DownloadURL:   "https://github.com/router-for-me/CLIProxyAPI/releases/download/vvB/CLIProxyAPI_vB_windows_amd64.zip",
		ArchiveSHA256: hex.EncodeToString(archiveSum[:]), ExecutableSHA256: hex.EncodeToString(executableSum[:]),
		ConfigAdapterVersion: "dualpool-cpa-vB-config-v1", Digest: hex.EncodeToString(digestSum[:]),
		ReleaseMetadataURL: "https://github.com/router-for-me/CLIProxyAPI/releases/tag/vvB",
	}
}

func candidateManagerFixture(t *testing.T, extra ...upstreamcatalog.Provenance) (*Manager, *installedslot.Registry, dataroot.Layout, *winacl.Manager, upstreamlock.Lock, []byte) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "product")
	layout := dataroot.Layout{Root: root, Bin: filepath.Join(root, "bin"), Instances: filepath.Join(root, "instances"), Locks: filepath.Join(root, "locks"), State: filepath.Join(root, "state")}
	acl, err := winacl.New()
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{layout.Root, layout.Bin, layout.Instances, layout.Locks, layout.State, filepath.Join(layout.Bin, "cliproxyapi")} {
		if err := acl.Create(dir); err != nil {
			t.Fatal(err)
		}
	}
	lock := candidateInstallLock(t)
	currentCatalog, err := upstreamcatalog.FromPinnedLock(lock)
	if err != nil {
		t.Fatal(err)
	}
	current, err := currentCatalog.Resolve(lock.Version)
	if err != nil {
		t.Fatal(err)
	}
	entries := append([]upstreamcatalog.Provenance{current}, extra...)
	catalog, err := upstreamcatalog.NewVerified(entries...)
	if err != nil {
		t.Fatal(err)
	}
	locks, err := lockfile.NewManager(layout.Locks)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := installedslot.NewWithCatalog(layout, acl, catalog,
		installedslot.WithLockManager(locks),
		installedslot.WithBinaryVerifier(func(_ context.Context, path string, p upstreamcatalog.Provenance) error {
			if digest(path) != p.ExecutableSHA256 {
				return errors.New("synthetic registry hash mismatch")
			}
			return nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	active := state.State{SchemaVersion: 1, InstallID: "install_candidate_fixture", ActiveUpstreamVersion: lock.Version,
		Instances:   state.Instances{Codex: state.Instance{Port: cliproxyconfig.CodexPort, Status: state.InstanceStopped}, Google: state.Instance{Port: cliproxyconfig.GooglePort, Status: state.InstanceStopped}},
		Antigravity: state.Antigravity{Mode: state.ModeDisabled, BridgePort: 51074}, Accounts: []state.Account{}}
	manager, err := New(layout, acl, lock, WithLockManager(locks), WithSlotRegistry(registry), WithStateReader(activeStateFixture{value: active}))
	if err != nil {
		t.Fatal(err)
	}
	manager.identityVerifier = func(_ context.Context, _ string, expected upstreamstage.ExpectedIdentity) (upstreamstage.Identity, error) {
		for _, p := range entries {
			if expected.Version == p.Version && expected.Commit == p.Commit {
				return upstreamstage.Identity{VersionMatch: true, CommitMatch: true}, nil
			}
		}
		return upstreamstage.Identity{}, upstreamstage.ErrBinaryIdentityMismatch
	}
	return manager, registry, layout, acl, lock, executable
}

func writeCandidateStage(t *testing.T, acl *winacl.Manager, p upstreamcatalog.Provenance, executable []byte) upstreamstage.Result {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "stage")
	if err := acl.Create(dir); err != nil {
		t.Fatal(err)
	}
	base := "CLIProxyAPI.exe"
	executablePath := filepath.Join(dir, base)
	if err := writeCandidateProtected(acl, executablePath, executable); err != nil {
		t.Fatal(err)
	}
	manifest := upstreamstage.Manifest{SchemaVersion: 1, Product: p.Product, Version: p.Version, Tag: p.Tag, Commit: p.Commit, Platform: p.Platform, Artifact: p.Artifact, ArchiveSHA256: p.ArchiveSHA256, ExecutableSHA256: p.ExecutableSHA256, ExecutableBasename: base, LockSHA256: p.Digest, BinaryVersionVerified: true, BinaryCommitVerified: true}
	if err := writeProtectedJSON(acl, filepath.Join(dir, "stage-manifest.json"), manifest); err != nil {
		t.Fatal(err)
	}
	return upstreamstage.Result{Directory: dir, Executable: executablePath, Manifest: manifest}
}

func writeCandidateProtected(acl *winacl.Manager, path string, data []byte) error {
	file, err := acl.CreateFile(path)
	if err != nil {
		return err
	}
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}

func TestInstallTrustedSyntheticCandidateRegistersImmutableSlot(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	p := syntheticInstallProvenance(executable)
	manager, registry, layout, acl, lock, _ := candidateManagerFixture(t, p)
	stage := writeCandidateStage(t, acl, p, executable)
	before, err := manager.state.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	path, existing, err := manager.installTrusted(context.Background(), p, stage)
	if err != nil || existing || path != filepath.Join(layout.Bin, "cliproxyapi", "vB", "cliproxyapi.exe") {
		t.Fatalf("install path=%q existing=%v err=%v", path, existing, err)
	}
	if err = registry.VerifyInstalled(context.Background(), "vB"); err != nil {
		t.Fatalf("VerifyInstalled: %v", err)
	}
	slot, err := registry.Resolve("vB")
	if err != nil || slot.Version != p.Version || slot.Tag != p.Tag || slot.Commit != p.Commit || slot.Platform != p.Platform || slot.ExecutableSHA256 != p.ExecutableSHA256 || slot.UpstreamLockSHA256 != p.Digest || slot.ConfigAdapterVersion != p.ConfigAdapterVersion || slot.SlotDirectory != filepath.Join(layout.Bin, "cliproxyapi", "vB") {
		t.Fatalf("resolved candidate=%+v err=%v", slot, err)
	}
	manifestBytes, err := os.ReadFile(filepath.Join(slot.SlotDirectory, manifestName))
	if err != nil {
		t.Fatal(err)
	}
	var manifest InstallManifest
	if err = readJSON(filepath.Join(slot.SlotDirectory, manifestName), &manifest); err != nil || manifest != manifestFor(p) {
		t.Fatalf("install manifest=%+v err=%v bytes=%q", manifest, err, manifestBytes)
	}
	firstRegistry, err := os.ReadFile(filepath.Join(layout.Bin, "cliproxyapi", "installed-slots.json"))
	if err != nil {
		t.Fatal(err)
	}
	path, existing, err = manager.installTrusted(context.Background(), p, stage)
	if err != nil || !existing || path != filepath.Join(layout.Bin, "cliproxyapi", "vB", "cliproxyapi.exe") {
		t.Fatalf("idempotent install path=%q existing=%v err=%v", path, existing, err)
	}
	secondRegistry, err := os.ReadFile(filepath.Join(layout.Bin, "cliproxyapi", "installed-slots.json"))
	if err != nil || string(firstRegistry) != string(secondRegistry) {
		t.Fatalf("idempotent registration changed: %v", err)
	}
	after, err := manager.state.LoadState()
	if err != nil || after.ActiveUpstreamVersion != lock.Version || before.ActiveUpstreamVersion != after.ActiveUpstreamVersion {
		t.Fatalf("active state changed: before=%q after=%q err=%v", before.ActiveUpstreamVersion, after.ActiveUpstreamVersion, err)
	}
	if _, err = os.Stat(filepath.Join(layout.Bin, "cliproxyapi", markerName)); !os.IsNotExist(err) {
		t.Fatalf("install marker remains after success: %v", err)
	}
}

func TestInstallTrustedAcceptsActualStagerACLTopology(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	p := syntheticInstallProvenance(executable)
	manager, registry, layout, acl, _, _ := candidateManagerFixture(t, p)
	stageRoot := filepath.Join(layout.Bin, "upstream-stage")
	if err := acl.Create(stageRoot); err != nil {
		t.Fatal(err)
	}
	if err := acl.Inspect(stageRoot); err != nil {
		t.Fatalf("protected stage root: %v", err)
	}
	stageDir, err := os.MkdirTemp(stageRoot, "candidate-")
	if err != nil {
		t.Fatal(err)
	}
	executablePath := filepath.Join(stageDir, "CLIProxyAPI.exe")
	file, err := os.OpenFile(executablePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write(executable); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
	manifest := upstreamstage.Manifest{SchemaVersion: 1, Product: p.Product, Version: p.Version, Tag: p.Tag, Commit: p.Commit, Platform: p.Platform, Artifact: p.Artifact, ArchiveSHA256: p.ArchiveSHA256, ExecutableSHA256: p.ExecutableSHA256, ExecutableBasename: filepath.Base(executablePath), LockSHA256: p.Digest, BinaryVersionVerified: true, BinaryCommitVerified: true}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	manifestBytes = append(manifestBytes, '\n')
	if err = os.WriteFile(filepath.Join(stageDir, "stage-manifest.json"), manifestBytes, 0600); err != nil {
		t.Fatal(err)
	}
	if err = acl.Inspect(stageDir); err == nil {
		t.Fatal("normal os.MkdirTemp child unexpectedly has the protected installed-object ACL")
	}
	stage := upstreamstage.Result{Directory: stageDir, Executable: executablePath, Manifest: manifest}
	installedPath, existing, err := manager.installTrusted(context.Background(), p, stage)
	if err != nil || existing || installedPath != filepath.Join(layout.Bin, "cliproxyapi", p.Version, "cliproxyapi.exe") {
		t.Fatalf("install path=%q existing=%v err=%v", installedPath, existing, err)
	}
	installedDir := filepath.Dir(installedPath)
	if err = acl.Inspect(installedDir); err != nil {
		t.Fatalf("protected installed slot: %v", err)
	}
	if err = acl.InspectFile(installedPath); err != nil {
		t.Fatalf("protected installed executable: %v", err)
	}
	if err = acl.InspectFile(filepath.Join(installedDir, manifestName)); err != nil {
		t.Fatalf("protected installed manifest: %v", err)
	}
	if err = registry.VerifyInstalled(context.Background(), p.Version); err != nil {
		t.Fatalf("registered installed slot: %v", err)
	}
}

func TestInstallCandidateRejectsValidCandidateOutsideProductStageRoot(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	p := syntheticInstallProvenance(executable)
	manager, _, layout, acl, _, _ := candidateManagerFixture(t, p)
	stageRoot := filepath.Join(layout.Bin, "upstream-stage")
	if err = acl.Create(stageRoot); err != nil {
		t.Fatal(err)
	}
	stage := writeCandidateStage(t, acl, p, executable)
	expectedStage := filepath.Join(stageRoot, "vB-windows_amd64-"+p.Digest)
	if stage.Directory == expectedStage {
		t.Fatal("fixture stage unexpectedly resides at the expected product cache path")
	}
	if _, _, err = manager.installTrustedAtStageRoot(context.Background(), p, stage, stageRoot, expectedStage); !errors.Is(err, ErrBinaryInstallConflict) {
		t.Fatalf("outside-root valid synthetic stage error=%v", err)
	}
	for _, path := range []string{
		filepath.Join(layout.Bin, "cliproxyapi", p.Version),
		filepath.Join(layout.Bin, "cliproxyapi", markerName),
		filepath.Join(layout.Bin, "cliproxyapi", "installed-slots.json"),
	} {
		if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
			t.Fatalf("outside-root rejection mutated %q: %v", path, statErr)
		}
	}
}

func TestPublicInstallCandidateRejectsCandidateShapedStageOutsideProductStageRootBeforeMutation(t *testing.T) {
	lock := candidateInstallLock(t)
	candidate, err := upstreamcatalog.ProductionCandidate(lock)
	if err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	manager, _, layout, acl, _, _ := candidateManagerFixture(t, candidate)
	stage := writeCandidateStage(t, acl, candidate, executable)
	before, err := manager.state.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = manager.InstallCandidate(context.Background(), stage); !errors.Is(err, ErrBinaryInstallConflict) {
		t.Fatalf("outside-root candidate install error=%v", err)
	}
	for _, path := range []string{
		filepath.Join(layout.Bin, "cliproxyapi", candidate.Version),
		filepath.Join(layout.Bin, "cliproxyapi", markerName),
		filepath.Join(layout.Bin, "cliproxyapi", "installed-slots.json"),
	} {
		if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
			t.Fatalf("outside-root rejection mutated %q: %v", path, statErr)
		}
	}
	after, err := manager.state.LoadState()
	if err != nil || after.ActiveUpstreamVersion != before.ActiveUpstreamVersion {
		t.Fatalf("active state changed: before=%q after=%q err=%v", before.ActiveUpstreamVersion, after.ActiveUpstreamVersion, err)
	}
}

func TestInstallCandidateRejectsUnprotectedStageRoot(t *testing.T) {
	lock := candidateInstallLock(t)
	candidate, err := upstreamcatalog.ProductionCandidate(lock)
	if err != nil {
		t.Fatal(err)
	}
	manager, _, layout, _, _, _ := candidateManagerFixture(t, candidate)
	stageRoot := filepath.Join(layout.Bin, "upstream-stage")
	if err := os.Mkdir(stageRoot, 0700); err != nil {
		t.Fatal(err)
	}
	expectedStage, err := upstreamstage.CandidateStagePath(stageRoot, lock)
	if err != nil {
		t.Fatal(err)
	}
	if err = manager.validateCandidateStageRoot(stageRoot, expectedStage, expectedStage); !errors.Is(err, ErrBinaryInstallConflict) {
		t.Fatalf("unprotected stage root validation error=%v", err)
	}
}

func TestPublicInstallWrappersRejectCrossReleaseAndForgedStagesBeforeRegistry(t *testing.T) {
	lock := candidateInstallLock(t)
	currentCatalog, err := upstreamcatalog.FromPinnedLock(lock)
	if err != nil {
		t.Fatal(err)
	}
	current, err := currentCatalog.Resolve(lock.Version)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := upstreamcatalog.ProductionCandidate(lock)
	if err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	fake939 := syntheticInstallProvenance(executable)
	fake939.Version = "7.3.9"
	fake939.Tag = "v7.3.9"
	fake939.Artifact = "CLIProxyAPI_7.3.9_windows_amd64.zip"
	fake939.DownloadURL = "https://github.com/router-for-me/CLIProxyAPI/releases/download/v7.3.9/CLIProxyAPI_7.3.9_windows_amd64.zip"
	fake939.ReleaseMetadataURL = "https://github.com/router-for-me/CLIProxyAPI/releases/tag/v7.3.9"

	cases := []struct {
		name       string
		candidate  bool
		provenance upstreamcatalog.Provenance
		mutate     func(*testing.T, *winacl.Manager, *upstreamstage.Result)
	}{
		{name: "current v7.3.7 stage to InstallCandidate", candidate: true, provenance: current},
		{name: "candidate stage to current Install", provenance: candidate},
		{name: "v7.3.8 bytes claimed with current manifest", provenance: current},
		{name: "current bytes claimed with candidate manifest", candidate: true, provenance: candidate},
		{name: "correct candidate manifest with wrong executable bytes", candidate: true, provenance: candidate, mutate: func(t *testing.T, _ *winacl.Manager, stage *upstreamstage.Result) {
			t.Helper()
			path := stage.Executable
			if err := os.WriteFile(path, []byte("wrong executable bytes"), 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "coherent fake v7.3.9 stage", candidate: true, provenance: fake939},
		{name: "candidate with current provenance digest", candidate: true, provenance: candidate, mutate: func(t *testing.T, acl *winacl.Manager, stage *upstreamstage.Result) {
			stage.Manifest.LockSHA256 = lock.Digest()
			if err := os.Remove(filepath.Join(stage.Directory, "stage-manifest.json")); err != nil {
				t.Fatal(err)
			}
			if err := writeProtectedJSON(acl, filepath.Join(stage.Directory, "stage-manifest.json"), stage.Manifest); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manager, _, layout, acl, _, executable := candidateManagerFixture(t, candidate)
			stage := writeCandidateStage(t, acl, tc.provenance, executable)
			if tc.mutate != nil {
				tc.mutate(t, acl, &stage)
			}
			var installErr error
			if tc.candidate {
				_, _, installErr = manager.InstallCandidate(context.Background(), stage)
			} else {
				_, _, installErr = manager.Install(context.Background(), stage)
			}
			if installErr == nil {
				t.Fatal("untrusted cross-release stage was installed")
			}
			registryPath := filepath.Join(layout.Bin, "cliproxyapi", "installed-slots.json")
			if _, err := os.Lstat(registryPath); !os.IsNotExist(err) {
				t.Fatalf("registry was published before stage rejection: %v", err)
			}
			for _, version := range []string{lock.Version, "7.3.8", "7.3.9", "vB"} {
				if _, err := os.Lstat(filepath.Join(layout.Bin, "cliproxyapi", version)); !os.IsNotExist(err) {
					t.Fatalf("slot %s was published before rejection: %v", version, err)
				}
			}
		})
	}
}

func installAttemptName(version, txn string) string { return "." + version + ".install-" + txn }

func writeInstallMarkerForTest(t *testing.T, manager *Manager, p upstreamcatalog.Provenance, txn string) string {
	t.Helper()
	name := installAttemptName(p.Version, txn)
	path := filepath.Join(manager.layout.Bin, "cliproxyapi", markerName)
	if err := writeProtectedJSON(manager.acl, path, markerFor(p, txn, name)); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(filepath.Dir(path), name)
}

func recoverInstallMarkerLocked(t *testing.T, manager *Manager) error {
	t.Helper()
	guard, err := manager.locks.AcquireGlobal()
	if err != nil {
		t.Fatal(err)
	}
	recoverErr := manager.recoverInstallMarker(context.Background())
	if releaseErr := guard.Release(); recoverErr == nil {
		recoverErr = releaseErr
	}
	return recoverErr
}

func recoverInstallMarkerTrustedLocked(t *testing.T, manager *Manager, p upstreamcatalog.Provenance) error {
	t.Helper()
	guard, err := manager.locks.AcquireGlobal()
	if err != nil {
		t.Fatal(err)
	}
	var marker installMarker
	path := filepath.Join(manager.layout.Bin, "cliproxyapi", markerName)
	if err = readJSON(path, &marker); err == nil {
		err = manager.recoverInstallMarkerTrusted(context.Background(), marker, p)
	}
	if releaseErr := guard.Release(); err == nil {
		err = releaseErr
	}
	return err
}

func createInstalledCandidateFixture(t *testing.T, manager *Manager, p upstreamcatalog.Provenance, stage upstreamstage.Result, dir string, includeManifest bool) {
	t.Helper()
	if err := manager.acl.Create(dir); err != nil {
		t.Fatal(err)
	}
	if err := copyProtected(manager.acl, stage.Executable, filepath.Join(dir, "cliproxyapi.exe")); err != nil {
		t.Fatal(err)
	}
	if includeManifest {
		if err := writeProtectedJSON(manager.acl, filepath.Join(dir, manifestName), manifestFor(p)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCandidateInstallMarkerRecoveryConvergesWithoutActiveMutation(t *testing.T) {
	for _, mode := range []string{"marker only", "partial executable", "complete before rename", "final before registration", "registered with marker remaining"} {
		t.Run(mode, func(t *testing.T) {
			self, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			executable, err := os.ReadFile(self)
			if err != nil {
				t.Fatal(err)
			}
			p := syntheticInstallProvenance(executable)
			manager, registry, layout, acl, lock, _ := candidateManagerFixture(t, p)
			stage := writeCandidateStage(t, acl, p, executable)
			const txn = "0123456789abcdef0123456789abcdef"
			attempt := writeInstallMarkerForTest(t, manager, p, txn)
			final := filepath.Join(layout.Bin, "cliproxyapi", p.Version)
			before, err := manager.state.LoadState()
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "marker only":
			case "partial executable":
				createInstalledCandidateFixture(t, manager, p, stage, attempt, false)
			case "complete before rename":
				createInstalledCandidateFixture(t, manager, p, stage, attempt, true)
			case "final before registration":
				createInstalledCandidateFixture(t, manager, p, stage, final, true)
			case "registered with marker remaining":
				createInstalledCandidateFixture(t, manager, p, stage, final, true)
				if err := registry.Register(context.Background(), p.Version); err != nil {
					t.Fatal(err)
				}
			}
			var beforeRegistry []byte
			registryPath := filepath.Join(layout.Bin, "cliproxyapi", "installed-slots.json")
			if mode == "registered with marker remaining" {
				beforeRegistry, err = os.ReadFile(registryPath)
				if err != nil {
					t.Fatal(err)
				}
			}
			if err = recoverInstallMarkerTrustedLocked(t, manager, p); err != nil {
				t.Fatalf("recovery: %v", err)
			}
			if _, err = os.Lstat(filepath.Join(layout.Bin, "cliproxyapi", markerName)); !os.IsNotExist(err) {
				t.Fatalf("marker not removed after coherent recovery: %v", err)
			}
			if mode == "final before registration" || mode == "registered with marker remaining" {
				if err = registry.VerifyInstalled(context.Background(), p.Version); err != nil {
					t.Fatalf("candidate was not registered: %v", err)
				}
				slot, err := registry.Resolve(p.Version)
				if err != nil || slot.ExecutableSHA256 != p.ExecutableSHA256 || slot.UpstreamLockSHA256 != p.Digest {
					t.Fatalf("recovered slot=%+v err=%v", slot, err)
				}
				if mode == "registered with marker remaining" {
					afterRegistry, err := os.ReadFile(registryPath)
					if err != nil || string(afterRegistry) != string(beforeRegistry) {
						t.Fatalf("idempotent recovery changed registry: %v", err)
					}
				}
			} else {
				if _, err = os.Lstat(final); !os.IsNotExist(err) {
					t.Fatalf("recovery published candidate without final slot: %v", err)
				}
				if _, err = os.Lstat(registryPath); !os.IsNotExist(err) {
					t.Fatalf("recovery published incomplete candidate: %v", err)
				}
			}
			if mode != "marker only" && mode != "final before registration" && mode != "registered with marker remaining" {
				if _, err = os.Lstat(attempt); !os.IsNotExist(err) {
					t.Fatalf("owned attempt was not discarded: %v", err)
				}
			}
			after, err := manager.state.LoadState()
			if err != nil || before.ActiveUpstreamVersion != lock.Version || after.ActiveUpstreamVersion != lock.Version {
				t.Fatalf("recovery mutated active state: before=%q after=%q err=%v", before.ActiveUpstreamVersion, after.ActiveUpstreamVersion, err)
			}
		})
	}
}

func TestCandidateInstallMarkerRejectsCrossReleaseAndTamperedArtifacts(t *testing.T) {
	lock := candidateInstallLock(t)
	p, err := upstreamcatalog.ProductionCandidate(lock)
	if err != nil {
		t.Fatal(err)
	}
	const txn = "abcdef0123456789abcdef0123456789"
	for _, tc := range []struct {
		name   string
		marker installMarker
	}{
		{name: "candidate with current digest", marker: installMarker{1, txn, p.Version, installAttemptName(p.Version, txn), lock.Digest(), p.ConfigAdapterVersion}},
		{name: "candidate with wrong adapter", marker: installMarker{1, txn, p.Version, installAttemptName(p.Version, txn), p.Digest, "wrong-adapter"}},
		{name: "current with candidate digest", marker: installMarker{1, txn, lock.Version, installAttemptName(lock.Version, txn), p.Digest, p.ConfigAdapterVersion}},
		{name: "unknown release with valid digest shape", marker: installMarker{1, txn, "7.3.9", installAttemptName("7.3.9", txn), strings.Repeat("c", 64), "future-adapter"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manager, _, layout, acl, _, _ := candidateManagerFixture(t, p)
			markerPath := filepath.Join(layout.Bin, "cliproxyapi", markerName)
			if err := writeProtectedJSON(acl, markerPath, tc.marker); err != nil {
				t.Fatal(err)
			}
			attempt := filepath.Join(filepath.Dir(markerPath), tc.marker.CandidateBasename)
			if err := acl.Create(attempt); err != nil {
				t.Fatal(err)
			}
			sentinel := filepath.Join(attempt, "unowned-preserve.txt")
			if err := writeCandidateProtected(acl, sentinel, []byte("preserve")); err != nil {
				t.Fatal(err)
			}
			if err := recoverInstallMarkerLocked(t, manager); !errors.Is(err, ErrBinaryInstallConflict) {
				t.Fatalf("cross-release marker accepted: %v", err)
			}
			if _, err := os.Stat(sentinel); err != nil {
				t.Fatalf("tampered/unowned attempt was deleted: %v", err)
			}
			if _, err := os.Stat(markerPath); err != nil {
				t.Fatalf("rejected marker was removed: %v", err)
			}
		})
	}

	t.Run("tampered executable under valid marker", func(t *testing.T) {
		manager, _, layout, acl, _, _ := candidateManagerFixture(t, p)
		const id = "1234567890abcdef1234567890abcdef"
		attempt := writeInstallMarkerForTest(t, manager, p, id)
		if err := acl.Create(attempt); err != nil {
			t.Fatal(err)
		}
		if err := writeCandidateProtected(acl, filepath.Join(attempt, "cliproxyapi.exe"), []byte("tampered")); err != nil {
			t.Fatal(err)
		}
		if err := recoverInstallMarkerLocked(t, manager); !errors.Is(err, ErrBinaryInstallConflict) {
			t.Fatalf("tampered candidate accepted: %v", err)
		}
		if _, err := os.Stat(attempt); err != nil {
			t.Fatalf("tampered candidate was removed: %v", err)
		}
		if _, err := os.Stat(filepath.Join(layout.Bin, "cliproxyapi", markerName)); err != nil {
			t.Fatalf("marker was removed despite tampering: %v", err)
		}
	})
}

func TestCandidateInstallFaultsNeverPublishAndRecoverOwnedAttempts(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	p := syntheticInstallProvenance(executable)
	for _, point := range []installFaultPoint{beforeCandidateCreate, beforeExecutableCopy, beforeManifestWrite, beforeInstallValidate} {
		t.Run(string(point), func(t *testing.T) {
			manager, _, layout, acl, _, _ := candidateManagerFixture(t, p)
			stage := writeCandidateStage(t, acl, p, executable)
			injected := errors.New("injected install fault")
			manager.installFault = func(got installFaultPoint) error {
				if got == point {
					return injected
				}
				return nil
			}
			if _, _, err := manager.installTrusted(context.Background(), p, stage); !errors.Is(err, injected) {
				t.Fatalf("fault error=%v", err)
			}
			final := filepath.Join(layout.Bin, "cliproxyapi", p.Version)
			if _, err := os.Lstat(final); !os.IsNotExist(err) {
				t.Fatalf("candidate final slot published at %s: %v", final, err)
			}
			registryPath := filepath.Join(layout.Bin, "cliproxyapi", "installed-slots.json")
			if _, err := os.Lstat(registryPath); !os.IsNotExist(err) {
				t.Fatalf("registry published after fault: %v", err)
			}
			manager.installFault = nil
			if err := recoverInstallMarkerTrustedLocked(t, manager, p); err != nil {
				t.Fatalf("recovery after fault: %v", err)
			}
			if _, err := os.Lstat(filepath.Join(layout.Bin, "cliproxyapi", markerName)); !os.IsNotExist(err) {
				t.Fatalf("marker remains after safe rollback: %v", err)
			}
			if _, err := os.Lstat(registryPath); !os.IsNotExist(err) {
				t.Fatalf("recovery registered absent final candidate: %v", err)
			}
		})
	}
}

type rejectCandidateRegistration struct {
	SlotRegistry
	err error
}

func (r rejectCandidateRegistration) RegisterLocked(context.Context, string) error { return r.err }

func TestRegistryPublicationFailureRetainsMarkerAndRecoveryRegistersFinalCandidate(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	p := syntheticInstallProvenance(executable)
	manager, registry, layout, acl, lock, _ := candidateManagerFixture(t, p)
	stage := writeCandidateStage(t, acl, p, executable)
	registrationErr := errors.New("registry publication failed")
	manager.registry = rejectCandidateRegistration{SlotRegistry: registry, err: registrationErr}
	if _, _, err = manager.installTrusted(context.Background(), p, stage); !errors.Is(err, registrationErr) {
		t.Fatalf("install error=%v", err)
	}
	final := filepath.Join(layout.Bin, "cliproxyapi", p.Version)
	if err = manager.validateInstallFor(context.Background(), final, p); err != nil {
		t.Fatalf("published final candidate invalid: %v", err)
	}
	markerPath := filepath.Join(layout.Bin, "cliproxyapi", markerName)
	if _, err = os.Stat(markerPath); err != nil {
		t.Fatalf("marker removed before successful registry publication: %v", err)
	}
	registryPath := filepath.Join(layout.Bin, "cliproxyapi", "installed-slots.json")
	if _, err = os.Stat(registryPath); !os.IsNotExist(err) {
		t.Fatalf("failing registry published data: %v", err)
	}
	manager.registry = registry
	if err = recoverInstallMarkerTrustedLocked(t, manager, p); err != nil {
		t.Fatalf("recovery registry publication: %v", err)
	}
	if err = registry.VerifyInstalled(context.Background(), p.Version); err != nil {
		t.Fatalf("candidate not registered after recovery: %v", err)
	}
	if _, err = os.Stat(markerPath); !os.IsNotExist(err) {
		t.Fatalf("marker remains after coherent recovery: %v", err)
	}
	active, err := manager.state.LoadState()
	if err != nil || active.ActiveUpstreamVersion != lock.Version {
		t.Fatalf("active state changed: %+v err=%v", active, err)
	}
}

func TestCandidateInstallManifestCrossBindingFailsBeforeRegistry(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	p := syntheticInstallProvenance(executable)
	for _, mutate := range []struct {
		name string
		fn   func(*InstallManifest)
	}{
		{name: "wrong digest", fn: func(m *InstallManifest) { m.UpstreamLockSHA256 = strings.Repeat("c", 64) }},
		{name: "wrong adapter", fn: func(m *InstallManifest) { m.ConfigAdapterVersion = "wrong-adapter" }},
		{name: "current release identity", fn: func(m *InstallManifest) {
			m.Version, m.Tag, m.Commit = "7.3.7", "v7.3.7", "b773607e3e7756dc6020a291825e4eb08899595a"
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			manager, registry, layout, acl, _, executable := candidateManagerFixture(t, p)
			final := filepath.Join(layout.Bin, "cliproxyapi", p.Version)
			if err := acl.Create(final); err != nil {
				t.Fatal(err)
			}
			if err := writeCandidateProtected(acl, filepath.Join(final, "cliproxyapi.exe"), executable); err != nil {
				t.Fatal(err)
			}
			manifest := manifestFor(p)
			mutate.fn(&manifest)
			if err := writeProtectedJSON(acl, filepath.Join(final, manifestName), manifest); err != nil {
				t.Fatal(err)
			}
			if err := manager.validateInstallFor(context.Background(), final, p); err == nil {
				t.Fatal("cross-release install manifest accepted")
			}
			if err := registry.Register(context.Background(), p.Version); err == nil {
				t.Fatal("registry published cross-release manifest")
			}
			if _, err := os.Stat(filepath.Join(layout.Bin, "cliproxyapi", "installed-slots.json")); !os.IsNotExist(err) {
				t.Fatalf("registry file exists after rejected manifest: %v", err)
			}
		})
	}
}

func TestCurrentInstallMarkerRecoveryRemainsCompatible(t *testing.T) {
	manager, _ := legacyInstallFixture(t)
	txn := "fedcba9876543210fedcba9876543210"
	markerPath := filepath.Join(manager.layout.Bin, "cliproxyapi", markerName)
	marker := manager.marker(txn, installAttemptName(manager.lock.Version, txn))
	if err := writeProtectedJSON(manager.acl, markerPath, marker); err != nil {
		t.Fatal(err)
	}
	if err := recoverInstallMarkerLocked(t, manager); err != nil {
		t.Fatalf("current marker recovery: %v", err)
	}
	if err := manager.registry.VerifyInstalled(context.Background(), manager.lock.Version); err != nil {
		t.Fatalf("current installation not registered: %v", err)
	}
	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Fatalf("current marker remains: %v", err)
	}
}
