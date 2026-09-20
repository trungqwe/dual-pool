package configtxn

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestConfigPathAncestorSafety(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "config.toml")
	if err := os.WriteFile(target, []byte("model = \"fixture\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := safeTarget(target); err != nil {
		t.Fatalf("ordinary absolute target: %v", err)
	}
	if _, err := safeTarget(strings.ToUpper(target)); err != nil {
		t.Fatalf("case alias: %v", err)
	}

	short := shortPath(t, target)
	if short != "" && !strings.EqualFold(short, target) {
		if _, err := safeTarget(short); err != nil {
			t.Fatalf("short-name alias: %v", err)
		}
	}

	link := filepath.Join(root, "target-link.toml")
	if err := os.Symlink(target, link); err == nil {
		if _, err := safeTarget(link); err != ErrUnsafeConfigArtifact {
			t.Fatalf("target reparse=%v", err)
		}
		_ = os.Remove(link)
	}

	realParent := filepath.Join(root, "real-parent")
	if err := os.Mkdir(realParent, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realParent, "config.toml"), []byte("x=1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	direct := filepath.Join(root, "direct-junction")
	makeJunction(t, direct, realParent)
	if _, err := safeTarget(filepath.Join(direct, "config.toml")); err != ErrUnsafeConfigArtifact {
		t.Fatalf("direct junction=%v", err)
	}

	realAncestor := filepath.Join(root, "real-ancestor")
	if err := os.MkdirAll(filepath.Join(realAncestor, "child"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realAncestor, "child", "config.toml"), []byte("x=1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	ancestor := filepath.Join(root, "ancestor-junction")
	makeJunction(t, ancestor, realAncestor)
	if _, err := safeTarget(filepath.Join(ancestor, "child", "config.toml")); err != ErrUnsafeConfigArtifact {
		t.Fatalf("ancestor junction=%v", err)
	}
}

func TestConfigPathLexicalRejections(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{`relative\config.toml`, `\\server\share\config.toml`, `\\?\C:\config.toml`, `\\.\C:\config.toml`} {
		if safeLocalAbsolute(path) {
			t.Fatalf("unsafe namespace accepted")
		}
	}
	for _, name := range []string{"CON", "prn.txt", "AUX", "NUL.cfg", "CLOCK$", "COM1", "com9.log", "LPT1", "lpt9.txt", "trailing.", "trailing "} {
		if safeLocalAbsolute(filepath.Join(root, name, "config.toml")) {
			t.Fatalf("unsafe component accepted: %s", name)
		}
	}
}

func makeJunction(t *testing.T, link, target string) {
	t.Helper()
	out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput()
	if err != nil {
		t.Skipf("junction unavailable: %v", string(out))
	}
	t.Cleanup(func() { _ = os.Remove(link) })
}

func shortPath(t *testing.T, path string) string {
	t.Helper()
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return ""
	}
	buf := make([]uint16, 32768)
	n, err := windows.GetShortPathName(p, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 || n >= uint32(len(buf)) {
		return ""
	}
	return windows.UTF16ToString(buf[:n])
}
