package app

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/trungqwe/dual-pool/internal/buildinfo"
)

func TestVersionVariants(t *testing.T) {
	info := buildinfo.Info{Product: "poolbridge", Version: "0.0.0-test", Commit: "deadbeef", BuildTime: "2026-09-19T14:15:00Z", Dirty: "false"}
	for _, args := range [][]string{{"version"}, {"--version"}} {
		var stdout, stderr bytes.Buffer
		exit := New(info).Run(args, &stdout, &stderr)
		if exit != 0 || stderr.Len() != 0 {
			t.Fatalf("args %v: exit=%d stderr=%q", args, exit, stderr.String())
		}
		if stdout.String() != buildinfo.Format(info) {
			t.Fatalf("args %v: output=%q", args, stdout.String())
		}
	}
}

func TestHelpVariantsAndMissingCommand(t *testing.T) {
	for _, args := range [][]string{nil, {"help"}, {"--help"}, {"-h"}} {
		var stdout, stderr bytes.Buffer
		if exit := New(buildinfo.Current()).Run(args, &stdout, &stderr); exit != 0 {
			t.Fatalf("args %v: exit=%d stderr=%q", args, exit, stderr.String())
		}
		for _, command := range []string{"version", "help"} {
			if !strings.Contains(stdout.String(), command) {
				t.Fatalf("help missing %q: %q", command, stdout.String())
			}
		}
	}
}

func TestUnknownCommandIsStableUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := New(buildinfo.Current()).Run([]string{"start"}, &stdout, &stderr)
	if exit != 2 || stdout.Len() != 0 {
		t.Fatalf("exit=%d stdout=%q", exit, stdout.String())
	}
	if stderr.String() != "INVALID_COMMAND: command is not recognized\n" {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestCommandsRejectExtraArguments(t *testing.T) {
	for _, args := range [][]string{{"version", "unexpected"}, {"--version", "unexpected"}, {"help", "unexpected"}, {"--help", "unexpected"}, {"-h", "unexpected"}} {
		var stdout, stderr bytes.Buffer
		exit := New(buildinfo.Current()).Run(args, &stdout, &stderr)
		if exit != 2 || stdout.Len() != 0 || stderr.String() != "INVALID_ARGUMENT: argument is invalid\n" {
			t.Errorf("args %v: exit=%d stdout=%q stderr=%q", args, exit, stdout.String(), stderr.String())
		}
	}
}

func TestInternalPackagesDoNotCallOSExit(t *testing.T) {
	root := filepath.Clean("..")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if bytes.Contains(content, []byte("os.Exit(")) {
			t.Errorf("internal package calls os.Exit: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
