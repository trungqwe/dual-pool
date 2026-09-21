package winacl

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestCreateAppliesProtectedExactACL(t *testing.T) {
	m, err := New()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "secured")
	if err = m.Create(path); err != nil {
		t.Fatal(err)
	}
	if err = m.Inspect(path); err != nil {
		t.Fatal(err)
	}
}

func TestCreateFileAppliesProtectedExactACLAtCreation(t *testing.T) {
	m, err := New()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	f, err := m.CreateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.InspectHandle(f); err != nil {
		t.Fatal(err)
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("created object is not a regular file: info=%v err=%v", info, err)
	}
	replacement := filepath.Join(t.TempDir(), "replacement")
	if err = os.WriteFile(replacement, []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(replacement, path); err == nil {
		t.Fatal("exclusive creation handle allowed pathname replacement")
	}
	if _, err = f.Write([]byte("fixture")); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = m.CreateFile(path); err == nil {
		t.Fatal("existing file replaced")
	}
}

func TestInspectRejectsInheritedBroadACL(t *testing.T) {
	m, err := New()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "ordinary")
	if err = os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err = m.Inspect(path); err == nil {
		t.Fatal("ordinary inherited ACL accepted")
	}
}

func TestInspectRejectsReparsePoint(t *testing.T) {
	m, err := New()
	if err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	target := filepath.Join(base, "target")
	link := filepath.Join(base, "link")
	if err = os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	if err = windows.CreateSymbolicLink(windows.StringToUTF16Ptr(link), windows.StringToUTF16Ptr(target), windows.SYMBOLIC_LINK_FLAG_DIRECTORY|0x2); err != nil {
		t.Skip("symlink unavailable")
	}
	if err = m.Inspect(link); err == nil {
		t.Fatal("reparse point accepted")
	}
}
