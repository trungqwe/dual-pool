//go:build windows

package instance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

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

const realV738ArchiveEnv = "DUALPOOL_V738_ARCHIVE"

type localArchiveDownloader struct{ source string }

func (d localArchiveDownloader) Download(ctx context.Context, platform upstreamlock.Platform, destination string) (upstreamstage.DownloadResult, error) {
	if err := ctx.Err(); err != nil {
		return upstreamstage.DownloadResult{}, err
	}
	if platform.Artifact != "CLIProxyAPI_7.3.8_windows_amd64.zip" || platform.DownloadURL != "https://github.com/router-for-me/CLIProxyAPI/releases/download/v7.3.8/CLIProxyAPI_7.3.8_windows_amd64.zip" || platform.ArchiveSHA256 != "5e3278ac9b57d16df503fd845827a6fdb57ec241f102b35899788287eb431351" || platform.ExecutableSHA256 != "479da2fb56eb3db11a76e19adeb2e10c2a4069a512ab5e3933ac4c50628360fd" {
		return upstreamstage.DownloadResult{}, errors.New("candidate authority metadata mismatch")
	}
	source, err := os.Open(d.source)
	if err != nil {
		return upstreamstage.DownloadResult{}, err
	}
	defer source.Close()
	destinationFile, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return upstreamstage.DownloadResult{}, err
	}
	n, copyErr := io.Copy(destinationFile, source)
	syncErr := destinationFile.Sync()
	closeErr := destinationFile.Close()
	if copyErr != nil {
		return upstreamstage.DownloadResult{}, copyErr
	}
	if syncErr != nil {
		return upstreamstage.DownloadResult{}, syncErr
	}
	if closeErr != nil {
		return upstreamstage.DownloadResult{}, closeErr
	}
	return upstreamstage.DownloadResult{Bytes: n}, nil
}

func TestRealV738TemporaryStageInstallOptIn(t *testing.T) {
	archivePath := os.Getenv(realV738ArchiveEnv)
	if archivePath == "" {
		t.Skip(realV738ArchiveEnv + " is unset; real-release TEMP acceptance is opt-in")
	}
	archiveInfo, err := os.Stat(archivePath)
	if err != nil || !archiveInfo.Mode().IsRegular() {
		t.Fatalf("%s must identify an existing regular archive file: %v", realV738ArchiveEnv, err)
	}
	archive, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	archiveSum := sha256.Sum256(archive)
	if got := hex.EncodeToString(archiveSum[:]); got != "5e3278ac9b57d16df503fd845827a6fdb57ec241f102b35899788287eb431351" {
		t.Fatalf("supplied archive digest=%s, want reviewed v7.3.8 digest", got)
	}

	lock := candidateInstallLock(t)
	provenance, err := upstreamcatalog.ProductionCandidate(lock)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "DualPool")
	layout := dataroot.Layout{Root: root, Bin: filepath.Join(root, "bin"), Instances: filepath.Join(root, "instances"), Config: filepath.Join(root, "config"), State: filepath.Join(root, "state"), Backups: filepath.Join(root, "backups"), Evidence: filepath.Join(root, "evidence"), Locks: filepath.Join(root, "locks")}
	acl, err := winacl.New()
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{layout.Root, layout.Bin, layout.Instances, layout.Config, layout.State, layout.Backups, layout.Evidence, layout.Locks, filepath.Join(layout.Bin, "cliproxyapi"), filepath.Join(layout.Bin, "upstream-stage")} {
		if err := acl.Create(dir); err != nil {
			t.Fatalf("create TEMP product directory %s: %v", dir, err)
		}
	}
	locks, err := lockfile.NewManager(layout.Locks)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := upstreamcatalog.Production(lock)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := installedslot.NewWithCatalog(layout, acl, catalog, installedslot.WithLockManager(locks))
	if err != nil {
		t.Fatal(err)
	}
	active := state.State{SchemaVersion: 1, InstallID: "real_temp_acceptance", ActiveUpstreamVersion: lock.Version,
		Instances:   state.Instances{Codex: state.Instance{Port: cliproxyconfig.CodexPort, Status: state.InstanceStopped}, Google: state.Instance{Port: cliproxyconfig.GooglePort, Status: state.InstanceStopped}},
		Antigravity: state.Antigravity{Mode: state.ModeDisabled, BridgePort: 51074}, Accounts: []state.Account{}}
	manager, err := New(layout, acl, lock, WithLockManager(locks), WithSlotRegistry(registry), WithStateReader(activeStateFixture{value: active}))
	if err != nil {
		t.Fatal(err)
	}
	stageRoot := filepath.Join(layout.Bin, "upstream-stage")
	stager, err := upstreamstage.New(stageRoot, locks, upstreamstage.WithDownloader(localArchiveDownloader{source: archivePath}))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	stage, err := stager.StageCandidate(ctx, lock)
	if err != nil {
		t.Fatalf("StageCandidate from local archive: %v", err)
	}
	if stage.Manifest.Version != "7.3.8" || stage.Manifest.Commit != provenance.Commit || stage.Manifest.LockSHA256 != provenance.Digest || stage.Manifest.ExecutableSHA256 != provenance.ExecutableSHA256 {
		t.Fatalf("staged identity does not match reviewed candidate: %+v", stage.Manifest)
	}
	if _, existing, err := manager.InstallCandidate(ctx, stage); err != nil || existing {
		t.Fatalf("InstallCandidate=(existing %v, err %v)", existing, err)
	}
	if err := registry.VerifyInstalled(ctx, "7.3.8"); err != nil {
		t.Fatalf("VerifyInstalled: %v", err)
	}
	registryBefore := digest(filepath.Join(layout.Bin, "cliproxyapi", "installed-slots.json"))
	if err := newUpdaterSmoke(manager).Disposable(ctx, "7.3.8"); err != nil {
		t.Fatalf("Disposable Smoke against exact TEMP candidate: %v", err)
	}
	if active.ActiveUpstreamVersion != lock.Version || digest(filepath.Join(layout.Bin, "cliproxyapi", "installed-slots.json")) != registryBefore {
		t.Fatal("Disposable Smoke changed active state or installed-slot registry")
	}
	if entries, readErr := os.ReadDir(layout.Instances); readErr != nil || len(entries) != 0 {
		t.Fatalf("Disposable Smoke created production process records: entries=%v err=%v", entries, readErr)
	}
	attempts, readErr := os.ReadDir(filepath.Join(layout.State, smokeRootName))
	if readErr != nil || len(attempts) != 0 {
		t.Fatalf("Disposable Smoke left its TEMP attempt workspace: entries=%v err=%v", attempts, readErr)
	}
	resolved, err := registry.Resolve("7.3.8")
	if err != nil || resolved.Version != provenance.Version || resolved.Tag != provenance.Tag || resolved.Commit != provenance.Commit || resolved.Platform != provenance.Platform || resolved.ExecutableSHA256 != provenance.ExecutableSHA256 || resolved.UpstreamLockSHA256 != provenance.Digest || resolved.ConfigAdapterVersion != provenance.ConfigAdapterVersion || resolved.SlotDirectory != filepath.Join(layout.Bin, "cliproxyapi", "7.3.8") {
		t.Fatalf("resolved slot=%+v err=%v", resolved, err)
	}
	if active.ActiveUpstreamVersion != lock.Version {
		t.Fatalf("active state changed: %q", active.ActiveUpstreamVersion)
	}
	if _, err := os.Lstat(filepath.Join(layout.State, ".update-transaction.json")); !os.IsNotExist(err) {
		t.Fatalf("promotion marker exists: %v", err)
	}
}
