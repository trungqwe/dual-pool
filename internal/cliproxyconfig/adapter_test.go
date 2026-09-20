package cliproxyconfig

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/trungqwe/dual-pool/internal/dataroot"
	"github.com/trungqwe/dual-pool/internal/keymaterial"
	"github.com/trungqwe/dual-pool/internal/secretstore"
	"github.com/trungqwe/dual-pool/internal/upstreamlock"
	"github.com/trungqwe/dual-pool/internal/winacl"
	"golang.org/x/crypto/bcrypt"
)

type fakeReader struct {
	values map[secretstore.Purpose][]byte
	reads  int
}

func (s *fakeReader) Get(p secretstore.Purpose) ([]byte, error) {
	s.reads++
	v, ok := s.values[p]
	if !ok {
		return nil, secretstore.ErrNotFound
	}
	return append([]byte(nil), v...), nil
}
func fixtureValues() *fakeReader {
	return &fakeReader{values: map[secretstore.Purpose][]byte{
		secretstore.CodexClientKey: bytes.Repeat([]byte{1}, 32), secretstore.CodexManagementKey: bytes.Repeat([]byte{2}, 32),
		secretstore.GoogleClientKey: bytes.Repeat([]byte{3}, 32), secretstore.GoogleManagementKey: bytes.Repeat([]byte{4}, 32),
	}}
}
func fixture(t *testing.T) (*Generator, *fakeReader) {
	t.Helper()
	l, err := dataroot.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a, err := winacl.New()
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{l.Root, l.Bin, l.Instances, l.Config, l.State, l.Backups, l.Evidence, l.Locks} {
		if err = a.Create(dir); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(filepath.Join("..", "..", "upstream.lock"))
	if err != nil {
		t.Fatal(err)
	}
	lock, err := upstreamlock.Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	s := fixtureValues()
	g, err := New(l, a, s, lock)
	if err != nil {
		t.Fatal(err)
	}
	return g, s
}

func TestRenderStrictSemanticAndSecretPlacement(t *testing.T) {
	client, _ := keymaterial.Encode(bytes.Repeat([]byte{1}, 32))
	mgmt, _ := keymaterial.Encode(bytes.Repeat([]byte{2}, 32))
	opposite, _ := keymaterial.Encode(bytes.Repeat([]byte{3}, 32))
	b, err := render(Codex, `C:\fixture\DualPool\instances\codex\auth`, client, mgmt)
	if err != nil {
		t.Fatal(err)
	}
	defer zero(b)
	if validateConfig(b, Codex, `C:\fixture\DualPool\instances\codex\auth`, client, mgmt, [][]byte{opposite}) != nil {
		t.Fatal("strict config rejected")
	}
	if !bytes.Contains(b, client) || bytes.Contains(b, mgmt) || bytes.Contains(b, opposite) {
		t.Fatal("secret placement failed")
	}
	if !bytes.Contains(b, []byte("host: \"127.0.0.1\"")) {
		t.Fatal("host not explicit")
	}
	for _, bad := range [][]byte{
		append(append([]byte(nil), b...), []byte("host: bad\n")...),
		bytes.Replace(b, []byte("host:"), []byte("unknown:"), 1),
		bytes.Replace(b, []byte("host: \"127.0.0.1\""), []byte("host: 42"), 1),
		bytes.Replace(b, []byte("api-keys:\n"), []byte("api-keys:\n  - extra\n"), 1),
		bytes.Replace(b, []byte("remote-management:"), []byte("other-management:"), 1),
		bytes.Replace(b, []byte("routing:"), []byte("routing: [] #"), 1),
		bytes.Replace(b, []byte("request-log: false\n"), nil, 1),
		bytes.Replace(b, []byte("  allow-remote: false\n"), nil, 1),
		bytes.Replace(b, []byte("  session-affinity: true\n"), nil, 1),
		append(append([]byte(nil), b...), []byte("---\nhost: other\n")...),
		append(append([]byte(nil), b...), 0xff),
		bytes.Replace(b, []byte("debug: false"), []byte("debug: &x false"), 1),
		bytes.Replace(b, []byte("debug: false"), []byte("debug: *x"), 1),
	} {
		if validateConfig(bad, Codex, `C:\fixture\DualPool\instances\codex\auth`, client, mgmt, [][]byte{opposite}) == nil {
			t.Fatal("malformed YAML accepted")
		}
	}
	if bcrypt.CompareHashAndPassword([]byte("invalid"), mgmt) == nil {
		t.Fatal("invalid bcrypt accepted")
	}
}

func TestPairGenerationAndIdempotence(t *testing.T) {
	g, s := fixture(t)
	r, err := g.GeneratePair()
	if err != nil || !r.Ready || r.Created != 2 || r.ConfigsRewritten != 0 {
		t.Fatalf("%#v %v", r, err)
	}
	before := map[ID][]byte{}
	for _, id := range []ID{Codex, Google} {
		path := filepath.Join(g.final(id), "config.yaml")
		b, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		before[id] = b
		for _, dir := range []string{"auth", "logs"} {
			entries, e := os.ReadDir(filepath.Join(g.final(id), dir))
			if e != nil || len(entries) != 0 {
				t.Fatal("nonempty instance child")
			}
		}
	}
	r, err = g.GeneratePair()
	if err != nil || !r.Ready || r.Created != 0 || r.Reused != 2 || r.ConfigsRewritten != 0 {
		t.Fatalf("%#v %v", r, err)
	}
	for _, id := range []ID{Codex, Google} {
		b, e := os.ReadFile(filepath.Join(g.final(id), "config.yaml"))
		if e != nil || !bytes.Equal(before[id], b) {
			t.Fatal("config changed")
		}
		zero(b)
		zero(before[id])
	}
	if s.reads == 0 {
		t.Fatal("keys not read")
	}
}

func TestMissingDuplicateAndUnknownInstanceFailBeforeMutation(t *testing.T) {
	g, s := fixture(t)
	delete(s.values, secretstore.GoogleManagementKey)
	if _, err := g.GeneratePair(); !errors.Is(err, ErrInvalidKeyMaterial) {
		t.Fatalf("%v", err)
	}
	if exists(g.final(Codex)) {
		t.Fatal("mutated with missing key")
	}
	s.values[secretstore.GoogleManagementKey] = append([]byte(nil), s.values[secretstore.CodexClientKey]...)
	if _, err := g.GeneratePair(); !errors.Is(err, ErrInvalidKeyMaterial) {
		t.Fatalf("%v", err)
	}
	if validateInstanceID(ID("other")) == nil {
		t.Fatal("unknown id accepted")
	}
}

func TestAdapterMismatchFailsClosed(t *testing.T) {
	g, _ := fixture(t)
	for _, value := range []string{"UNIMPLEMENTED", "other", "dualpool-cpa-v7.3.8-config-v1"} {
		l := g.lock
		l.ConfigAdapterVersion = value
		if _, err := New(g.layout, g.acl, g.reader, l); !errors.Is(err, ErrUnsupportedAdapter) {
			t.Fatalf("%v", err)
		}
	}
	if _, err := os.Stat(g.final(Codex)); !os.IsNotExist(err) {
		t.Fatal("instance created")
	}
}

func TestManagementEnvironmentExcludedFromGenerator(t *testing.T) {
	// Pinned handler's MANAGEMENT_PASSWORD path enables allowRemoteOverride.
	// The generator has no environment lookup and emits only the bcrypt verifier.
	g, _ := fixture(t)
	t.Setenv("MANAGEMENT_PASSWORD", "synthetic-override")
	if _, err := g.GeneratePair(); err != nil {
		t.Fatal(err)
	}
	for _, id := range []ID{Codex, Google} {
		b, err := os.ReadFile(filepath.Join(g.final(id), "config.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "synthetic-override") {
			t.Fatal("environment key entered config")
		}
		zero(b)
	}
}

func TestGeneratorSourceHasNoUpstreamImportsOrRuntimeLaunch(t *testing.T) {
	module, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(module, []byte("github.com/router-for-me/CLIProxyAPI")) {
		t.Fatal("upstream module imported")
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		b, err := os.ReadFile(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		for _, pattern := range []string{`github\.com/router-for-me/CLIProxyAPI`, `net\.Listen\s*\(`, `exec\.Command\s*\(`, `os\.LookupEnv\s*\(\s*"MANAGEMENT_PASSWORD"`} {
			if regexp.MustCompile(pattern).Match(b) {
				t.Fatal("forbidden upstream/runtime dependency")
			}
		}
	}
}
