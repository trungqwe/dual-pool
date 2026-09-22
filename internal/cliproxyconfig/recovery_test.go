package cliproxyconfig

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/trungqwe/dual-pool/internal/dataroot"
	"github.com/trungqwe/dual-pool/internal/upstreamlock"
	"github.com/trungqwe/dual-pool/internal/winacl"
	"golang.org/x/crypto/bcrypt"
)

func TestFaultRecoveryMatrix(t *testing.T) {
	points := []Fault{AfterMarkerSync, AfterAttemptDirCreate, AfterConfigSync, AfterAuthDirCreate, AfterLogsDirCreate, AfterAttemptVerify, BeforeInstanceInstall, AfterInstanceInstall, AfterFinalVerify, BeforeMarkerCleanup, AfterCodexBeforeGoogle}
	for _, point := range points {
		t.Run(string(point), func(t *testing.T) {
			g, _ := fixture(t)
			g.WithFault(func(p Fault) error {
				if p == point {
					return errors.New("synthetic crash")
				}
				return nil
			})
			if _, err := g.GeneratePair(); err == nil {
				t.Fatal("fault not reached")
			}
			var codexBefore []byte
			if point == AfterCodexBeforeGoogle {
				codexBefore, _ = os.ReadFile(filepath.Join(g.final(Codex), "config.yaml"))
			}
			g.WithFault(nil)
			r, err := g.GeneratePair()
			if err != nil || !r.Ready {
				t.Fatalf("recovery failed: %v", err)
			}
			for _, id := range []ID{Codex, Google} {
				if exists(g.markerPath(id)) {
					t.Fatal("marker retained")
				}
				if entries, e := os.ReadDir(g.final(id)); e != nil || len(entries) != 3 {
					t.Fatal("partial final")
				}
			}
			if codexBefore != nil {
				after, _ := os.ReadFile(filepath.Join(g.final(Codex), "config.yaml"))
				if !bytes.Equal(codexBefore, after) {
					t.Fatal("completed Codex changed")
				}
				zero(after)
				zero(codexBefore)
			}
		})
	}
}

func TestCrashHelper(t *testing.T) {
	root := os.Getenv("DUALPOOL_CRASH_FIXTURE_ROOT")
	if root == "" {
		t.Skip("subprocess only")
	}
	l, err := dataroot.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	a, err := winacl.New()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join("..", "..", "upstream.lock"))
	if err != nil {
		t.Fatal(err)
	}
	lock, err := upstreamlock.Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	g, err := New(l, a, fixtureValues(), lock)
	if err != nil {
		t.Fatal(err)
	}
	g.bcryptCost = bcrypt.MinCost
	point := Fault(os.Getenv("DUALPOOL_CRASH_POINT"))
	g.WithFault(func(p Fault) error {
		if p == point {
			os.Exit(91)
		}
		return nil
	})
	_, _ = g.GeneratePair()
	os.Exit(92)
}

func TestSubprocessCrashRecovery(t *testing.T) {
	for _, point := range []Fault{AfterMarkerSync, AfterConfigSync, AfterInstanceInstall, AfterCodexBeforeGoogle} {
		t.Run(string(point), func(t *testing.T) {
			g, _ := fixture(t)
			cmd := exec.Command(os.Args[0], "-test.run=^TestCrashHelper$")
			cmd.Env = append(os.Environ(), "DUALPOOL_CRASH_FIXTURE_ROOT="+filepath.Dir(g.layout.Root), "DUALPOOL_CRASH_POINT="+string(point))
			err := cmd.Run()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 91 {
				t.Fatal("child did not stop at requested fault")
			}
			r, err := g.GeneratePair()
			if err != nil || !r.Ready {
				t.Fatalf("recovery failed: %v", err)
			}
			for _, id := range []ID{Codex, Google} {
				if exists(g.markerPath(id)) {
					t.Fatal("marker retained")
				}
			}
		})
	}
}

func TestCorruptMarkerAndUnexpectedFinalPreserved(t *testing.T) {
	g, _ := fixture(t)
	f, err := g.acl.CreateFile(g.markerPath(Codex))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Write([]byte("not-json")); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if _, err = g.GeneratePair(); !errors.Is(err, ErrRecoveryUnresolved) {
		t.Fatalf("%v", err)
	}
	if !exists(g.markerPath(Codex)) || exists(g.final(Codex)) {
		t.Fatal("corrupt marker was changed")
	}
	if err = os.Remove(g.markerPath(Codex)); err != nil {
		t.Fatal(err)
	}
	if _, err = g.GeneratePair(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(g.final(Codex), "config.yaml")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	modified := bytes.Replace(before, []byte("port: 8317"), []byte("port: 9999"), 1)
	if err = os.WriteFile(path, modified, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = g.GeneratePair(); !errors.Is(err, ErrConfigConflict) {
		t.Fatalf("%v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, modified) {
		t.Fatal("conflict overwritten")
	}
	zero(before)
	zero(modified)
	zero(after)
}

func TestOrphanCandidateIsConflict(t *testing.T) {
	g, _ := fixture(t)
	if err := g.acl.Create(filepath.Join(g.layout.Instances, ".codex.init-00000000000000000000000000000000")); err != nil {
		t.Fatal(err)
	}
	if _, err := g.GeneratePair(); !errors.Is(err, ErrConfigConflict) {
		t.Fatalf("%v", err)
	}
}
