package cliproxyconfig

import (
	"bytes"
	"crypto/subtle"
	"os"
	"path/filepath"
	"testing"

	"github.com/trungqwe/dual-pool/internal/secretstore"
	"github.com/trungqwe/dual-pool/internal/upstreamlock"
	"golang.org/x/sys/windows"
)

type fileIdentity struct{ volume, high, low uint32 }

func identity(path string) (fileIdentity, error) {
	f, err := os.Open(path)
	if err != nil {
		return fileIdentity{}, err
	}
	defer f.Close()
	var info windows.ByHandleFileInformation
	if err = windows.GetFileInformationByHandle(windows.Handle(f.Fd()), &info); err != nil {
		return fileIdentity{}, err
	}
	return fileIdentity{info.VolumeSerialNumber, info.FileIndexHigh, info.FileIndexLow}, nil
}

func TestRealInstanceConfigGeneration(t *testing.T) {
	if os.Getenv("DUALPOOL_RUN_REAL_INSTANCE_CONFIG") != "1" {
		t.Skip("explicit real-config gate is closed")
	}
	b, err := os.ReadFile(filepath.Join("..", "..", "upstream.lock"))
	if err != nil {
		t.Fatal("lock unavailable")
	}
	lock, err := upstreamlock.Decode(b)
	if err != nil {
		t.Fatal("lock invalid")
	}
	g, err := NewCurrent(lock)
	if err != nil {
		t.Fatal("product preflight failed")
	}
	pre, err := g.InspectPair()
	if err != nil {
		t.Fatal("instance preflight failed")
	}
	if pre.Present < 0 || pre.Present > 2 {
		t.Fatal("unexpected instance state")
	}
	beforeKeys, err := g.keys()
	if err != nil {
		t.Fatal("key preflight failed")
	}
	defer beforeKeys.wipe()
	first, err := g.GeneratePair()
	if err != nil || !first.Ready {
		t.Fatal("first generation failed")
	}
	var firstBytes [2][]byte
	var firstIDs [2]fileIdentity
	for n, id := range []ID{Codex, Google} {
		path := filepath.Join(g.final(id), "config.yaml")
		firstBytes[n], err = os.ReadFile(path)
		if err != nil {
			t.Fatal("config read failed")
		}
		defer secretstore.Zero(firstBytes[n])
		firstIDs[n], err = identity(path)
		if err != nil {
			t.Fatal("file identity failed")
		}
		for _, name := range []string{"auth", "logs"} {
			items, e := os.ReadDir(filepath.Join(g.final(id), name))
			if e != nil || len(items) != 0 {
				t.Fatal("provider/runtime directory not empty")
			}
		}
	}
	if bytes.Count(firstBytes[0], beforeKeys.wire[0]) != 1 || bytes.Count(firstBytes[1], beforeKeys.wire[2]) != 1 {
		t.Fatal("client key locality failed")
	}
	for n := range firstBytes {
		for k := range beforeKeys.raw {
			if bytes.Contains(firstBytes[n], beforeKeys.raw[k]) {
				t.Fatal("raw key entered config")
			}
		}
		for k := range beforeKeys.wire {
			if k == 2*n {
				continue
			}
			if bytes.Contains(firstBytes[n], beforeKeys.wire[k]) {
				t.Fatal("cross/management key entered config")
			}
		}
	}
	second, err := g.GeneratePair()
	if err != nil || !second.Ready || second.Created != 0 || second.ConfigsRewritten != 0 {
		t.Fatal("idempotence failed")
	}
	for n, id := range []ID{Codex, Google} {
		path := filepath.Join(g.final(id), "config.yaml")
		current, e := os.ReadFile(path)
		if e != nil || !bytes.Equal(current, firstBytes[n]) {
			secretstore.Zero(current)
			t.Fatal("config bytes changed")
		}
		secretstore.Zero(current)
		currentID, e := identity(path)
		if e != nil || currentID != firstIDs[n] {
			t.Fatal("config identity changed")
		}
	}
	afterKeys, err := g.keys()
	if err != nil {
		t.Fatal("key recheck failed")
	}
	defer afterKeys.wipe()
	for n := range beforeKeys.raw {
		if subtle.ConstantTimeCompare(beforeKeys.raw[n], afterKeys.raw[n]) != 1 {
			t.Fatal("product key changed")
		}
	}
	if err := filepath.WalkDir(g.layout.Root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return ErrUnsafeInstanceArtifact
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		defer secretstore.Zero(content)
		for _, raw := range beforeKeys.raw {
			if bytes.Contains(content, raw) {
				return ErrConfigInvalid
			}
		}
		for k, wire := range beforeKeys.wire {
			count := bytes.Count(content, wire)
			allowed := (k == 0 && path == filepath.Join(g.final(Codex), "config.yaml")) || (k == 2 && path == filepath.Join(g.final(Google), "config.yaml"))
			if (allowed && count != 1) || (!allowed && count != 0) {
				return ErrConfigInvalid
			}
		}
		return nil
	}); err != nil {
		t.Fatal("product secret locality failed")
	}
	if err := filepath.WalkDir(filepath.Join("..", ".."), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return ErrUnsafeInstanceArtifact
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		defer secretstore.Zero(content)
		for k := range beforeKeys.raw {
			if bytes.Contains(content, beforeKeys.raw[k]) || bytes.Contains(content, beforeKeys.wire[k]) {
				return ErrConfigInvalid
			}
		}
		return nil
	}); err != nil {
		t.Fatal("repository secret locality failed")
	}
	t.Logf("REAL_INSTANCE_CONFIG_PASS instances=%d host_loopback=%t ports_distinct=%t client_config_count=%d management_plaintext_count=%d rewrites=%d key_rotations=%d auth_files=%d", 2, true, true, 2, 0, second.ConfigsRewritten, 0, 0)
}
