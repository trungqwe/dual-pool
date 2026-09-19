package buildinfo

import (
	"strings"
	"testing"
)

func TestFormatDeterministicAndPrivate(t *testing.T) {
	info := Info{Product: "poolbridge", Version: "0.0.0-test", Commit: "abc1234", BuildTime: "2026-09-19T14:15:00Z", Dirty: "false"}
	want := "poolbridge\nversion: 0.0.0-test\ncommit: abc1234\nbuild time: 2026-09-19T14:15:00Z\ndirty: false\n"
	if got := Format(info); got != want {
		t.Fatalf("Format() = %q, want %q", got, want)
	}
	for _, forbidden := range []string{`C:\Users\Example`, "machine-name", "HOME=", "USERPROFILE="} {
		if strings.Contains(Format(info), forbidden) {
			t.Fatalf("version output leaked %q", forbidden)
		}
	}
}

func TestCurrentUsesSafeDevelopmentDefaults(t *testing.T) {
	got := Current()
	if got.Product != "poolbridge" || got.Version == "" || got.Commit == "" || got.BuildTime == "" || got.Dirty == "" {
		t.Fatalf("unsafe or empty defaults: %#v", got)
	}
}

func TestFormatRejectsUnsafeInjectedMetadata(t *testing.T) {
	info := Info{Product: "machine-name", Version: "machine-name", Commit: `C:\Users\Example\secret`, BuildTime: "now\nHOME=private", Dirty: "false"}
	output := Format(info)
	if strings.Contains(output, `C:\Users\Example`) || strings.Contains(output, "HOME=") || strings.Contains(output, "private") || strings.Contains(output, "machine-name") {
		t.Fatalf("unsafe metadata reached version output: %q", output)
	}
	if !strings.Contains(output, "version: unknown\n") || !strings.Contains(output, "commit: unknown\n") || !strings.Contains(output, "build time: unknown\n") {
		t.Fatalf("unsafe metadata was not replaced: %q", output)
	}
}
