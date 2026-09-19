//go:build windows

package dataroot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/trungqwe/dual-pool/internal/apperr"
)

func TestResolveWindowsLayoutIsDeterministicAndPure(t *testing.T) {
	base := t.TempDir()
	first, err := Resolve(base)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Resolve(base)
	if err != nil || first != second {
		t.Fatalf("resolution changed: %#v %#v %v", first, second, err)
	}
	if first.Root != filepath.Join(base, "DualPool") {
		t.Fatalf("unexpected root shape: %q", first.Root)
	}
	for _, child := range []string{first.Bin, first.Instances, first.Config, first.State, first.Backups, first.Evidence, first.Locks} {
		rel, err := filepath.Rel(first.Root, child)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == "." {
			t.Fatalf("child escaped root: %q %v", child, err)
		}
	}
	if _, err := os.Stat(first.Root); !os.IsNotExist(err) {
		t.Fatalf("resolver created root or unexpected stat error: %v", err)
	}
}

func TestResolveRejectsUnsafeWindowsRoots(t *testing.T) {
	for _, input := range []string{"", ".", `relative\path`, `C:\`, `C:\Data\..\Other`, `C:\Data\.\Local`, `C:\Data.\Local`, `C:\CON\Local`, "C:\\Data\x00Local", `\\server\share\Local`} {
		layout, err := Resolve(input)
		if err == nil || layout != (Layout{}) {
			t.Errorf("accepted unsafe root %q: %#v %v", input, layout, err)
			continue
		}
		appErr, ok := err.(*apperr.Error)
		if !ok || appErr.Code() != apperr.CodeDataRootUnavailable || appErr.Category() != apperr.CategoryConfig {
			t.Errorf("unsafe root %q returned wrong error: %v", input, err)
		}
	}
}

func TestResolveCurrentUsesOnlyLocalAppDataWithoutCreatingIt(t *testing.T) {
	base := t.TempDir()
	t.Setenv("LOCALAPPDATA", base)
	got, err := ResolveCurrent()
	if err != nil || got.Root != filepath.Join(base, "DualPool") {
		t.Fatalf("current root: %#v %v", got, err)
	}
	if _, err := os.Stat(got.Root); !os.IsNotExist(err) {
		t.Fatalf("current resolver created root: %v", err)
	}
	t.Setenv("LOCALAPPDATA", "")
	if _, err := ResolveCurrent(); err == nil {
		t.Fatal("missing LOCALAPPDATA used a fallback")
	}
}
