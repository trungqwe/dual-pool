package upstreamstage

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/trungqwe/dual-pool/internal/lockfile"
	"github.com/trungqwe/dual-pool/internal/upstreamlock"
)

func TestPinnedUpstreamIntegration(t *testing.T) {
	if os.Getenv("DUALPOOL_RUN_PINNED_UPSTREAM_INTEGRATION") != "1" {
		t.Skip("opt-in pinned network integration")
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "upstream.lock"))
	if err != nil {
		t.Fatal(err)
	}
	l, err := upstreamlock.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	locksDir := filepath.Join(root, "locks")
	stageRoot := filepath.Join(root, "stage")
	if err = os.Mkdir(locksDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(stageRoot, 0700); err != nil {
		t.Fatal(err)
	}
	manager, err := lockfile.NewManager(locksDir)
	if err != nil {
		t.Fatal(err)
	}
	stager, err := New(stageRoot, manager)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	result, err := stager.Stage(ctx, l)
	if err != nil {
		t.Fatal(err)
	}
	if result.Existing || result.DownloadedBytes <= 0 || result.DownloadedBytes > MaxArchiveBytes || !reflect.DeepEqual(result.RedirectHosts, []string{"github.com", "release-assets.githubusercontent.com"}) {
		t.Fatalf("unexpected normalized result: existing=%v bytes=%d hosts=%v", result.Existing, result.DownloadedBytes, result.RedirectHosts)
	}
	if result.Manifest.ArchiveSHA256 != l.Platforms.WindowsAMD64.ArchiveSHA256 || result.Manifest.ExecutableSHA256 != l.Platforms.WindowsAMD64.ExecutableSHA256 || !result.Manifest.BinaryVersionVerified || !result.Manifest.BinaryCommitVerified {
		t.Fatal("manifest identity mismatch")
	}
	if second, err := stager.Stage(ctx, l); err != nil || !second.Existing {
		t.Fatalf("idempotence existing=%v err=%v", second.Existing, err)
	}
}
