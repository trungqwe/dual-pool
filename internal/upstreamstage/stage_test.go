package upstreamstage

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/trungqwe/dual-pool/internal/lockfile"
	"github.com/trungqwe/dual-pool/internal/upstreamlock"
)

func hashBytes(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func makeZIP(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for n, v := range entries {
		w, e := z.Create(n)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = w.Write(v); e != nil {
			t.Fatal(e)
		}
	}
	if e := z.Close(); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func fixtureExecutable(t *testing.T) []byte {
	t.Helper()
	p, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func lockFor(t *testing.T, archive, exe []byte) upstreamlock.Lock {
	t.Helper()
	raw := fmt.Sprintf(`{"schema_version":1,"product":"CLIProxyAPI","status":"candidate","version":"7.3.7","tag":"v7.3.7","commit":"b773607e3e7756dc6020a291825e4eb08899595a","retrieved_at":"2026-09-20T00:00:00Z","release_metadata_url":"https://github.com/router-for-me/CLIProxyAPI/releases/tag/v7.3.7","config_adapter_version":"dualpool-cpa-v7.3.7-config-v1","platforms":{"windows_amd64":{"artifact":"CLIProxyAPI_7.3.7_windows_amd64.zip","download_url":"https://github.com/router-for-me/CLIProxyAPI/releases/download/v7.3.7/CLIProxyAPI_7.3.7_windows_amd64.zip","archive_sha256":"%s","executable_sha256":"%s"}},"verified_capabilities":["published_checksum_matches_archive","github_asset_digest_matches_archive","binary_version_matches_tag","loopback_ipv4_bind","management_key_required","client_key_required","credential_free_start_stop_cleanup"],"unverified_capabilities":["credential_specific_model_inventory","provider_specific_response_shapes","dedicated_health_endpoint"],"evidence":"fixture"}`, hashBytes(archive), hashBytes(exe))
	l, e := upstreamlock.Decode([]byte(raw))
	if e != nil {
		t.Fatal(e)
	}
	return l
}

type fakeDownload struct {
	data  []byte
	count int
	err   error
}

func (f *fakeDownload) Download(_ context.Context, _ upstreamlock.Platform, path string) (DownloadResult, error) {
	f.count++
	if f.err != nil {
		return DownloadResult{}, f.err
	}
	if e := os.WriteFile(path, f.data, 0600); e != nil {
		return DownloadResult{}, e
	}
	return DownloadResult{Bytes: int64(len(f.data)), RedirectHosts: []string{"github.com", "release-assets.githubusercontent.com"}}, nil
}

type fakeVerify struct {
	count    int
	identity Identity
	err      error
}

type fakeIdentityRunner struct {
	stdout, stderr []byte
	err            error
}

func (f fakeIdentityRunner) Run(context.Context, string) ([]byte, []byte, error) {
	return f.stdout, f.stderr, f.err
}

type blockingIdentityRunner struct{}

func (blockingIdentityRunner) Run(ctx context.Context, _ string) ([]byte, []byte, error) {
	<-ctx.Done()
	return nil, nil, ctx.Err()
}

func (f *fakeVerify) Verify(context.Context, string, upstreamlock.Lock) (Identity, error) {
	f.count++
	return f.identity, f.err
}

func newFixtureStager(t *testing.T, d Downloader, v Verifier) (*Stager, string) {
	t.Helper()
	root := t.TempDir()
	locksDir := filepath.Join(root, "locks")
	stageRoot := filepath.Join(root, "stage")
	if e := os.Mkdir(locksDir, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(stageRoot, 0700); e != nil {
		t.Fatal(e)
	}
	m, e := lockfile.NewManager(locksDir)
	if e != nil {
		t.Fatal(e)
	}
	s, e := New(stageRoot, m, WithDownloader(d), WithVerifier(v))
	if e != nil {
		t.Fatal(e)
	}
	return s, stageRoot
}

func TestStageIsIdempotentAndConflictsOnMutation(t *testing.T) {
	exe := fixtureExecutable(t)
	archive := makeZIP(t, map[string][]byte{"bin/tool.exe": exe, "README.md": []byte("fixture")})
	l := lockFor(t, archive, exe)
	d := &fakeDownload{data: archive}
	v := &fakeVerify{identity: Identity{true, true}}
	s, _ := newFixtureStager(t, d, v)
	first, e := s.Stage(context.Background(), l)
	if e != nil {
		t.Fatal(e)
	}
	second, e := s.Stage(context.Background(), l)
	if e != nil || !second.Existing || d.count != 1 {
		t.Fatalf("existing=%v downloads=%d err=%v", second.Existing, d.count, e)
	}
	if first.Manifest.LockSHA256 != l.Digest() {
		t.Fatal("lock digest")
	}
	if e = os.WriteFile(first.Executable, []byte("changed"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Stage(context.Background(), l); !errors.Is(e, ErrStageConflict) {
		t.Fatalf("err=%v", e)
	}
}

func TestNoVerificationBeforeTrustGates(t *testing.T) {
	exe := fixtureExecutable(t)
	good := makeZIP(t, map[string][]byte{"tool.exe": exe})
	cases := []struct {
		name    string
		archive []byte
		lock    upstreamlock.Lock
		want    error
	}{
		{"archive hash", []byte("not zip"), lockFor(t, good, exe), ErrArchiveHashMismatch},
		{"unsafe archive", makeZIP(t, map[string][]byte{"../tool.exe": exe}), lockFor(t, makeZIP(t, map[string][]byte{"../tool.exe": exe}), exe), ErrArchiveUnsafe},
		{"executable hash", good, lockFor(t, good, []byte("different")), ErrExecutableNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := &fakeVerify{identity: Identity{true, true}}
			s, _ := newFixtureStager(t, &fakeDownload{data: tc.archive}, v)
			_, e := s.Stage(context.Background(), tc.lock)
			if !errors.Is(e, tc.want) || v.count != 0 {
				t.Fatalf("err=%v calls=%d", e, v.count)
			}
		})
	}
}

func TestBinaryIdentityFailures(t *testing.T) {
	exe := fixtureExecutable(t)
	archive := makeZIP(t, map[string][]byte{"tool.exe": exe})
	l := lockFor(t, archive, exe)
	line := func(version, commit string) []byte {
		return []byte(fmt.Sprintf("CLIProxyAPI Version: %s, Commit: %s, BuiltAt: fixture\n", version, commit))
	}
	tests := []struct {
		name                    string
		runner                  fakeIdentityRunner
		wantVersion, wantCommit bool
		wantErr                 bool
	}{
		{"match", fakeIdentityRunner{stdout: line("7.3.7", "b773607e")}, true, true, false},
		{"wrong version", fakeIdentityRunner{stdout: line("9.9.9", "b773607e")}, false, true, false},
		{"wrong commit", fakeIdentityRunner{stdout: line("7.3.7", "ffffffff")}, true, false, false},
		{"nonzero", fakeIdentityRunner{stdout: line("7.3.7", "b773607e"), err: errors.New("exit")}, false, false, true},
		{"oversized stdout", fakeIdentityRunner{stdout: bytes.Repeat([]byte("x"), maxProbeBytes+1)}, false, false, true},
		{"oversized stderr", fakeIdentityRunner{stdout: line("7.3.7", "b773607e"), stderr: bytes.Repeat([]byte("x"), maxProbeBytes+1)}, false, false, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := (WindowsVerifier{runner: tc.runner}).Verify(context.Background(), "ignored", l)
			if (err != nil) != tc.wantErr || got.VersionMatch != tc.wantVersion || got.CommitMatch != tc.wantCommit {
				t.Fatalf("got=%+v err=%v", got, err)
			}
		})
	}
}

func TestBinaryIdentityTimeout(t *testing.T) {
	exe := fixtureExecutable(t)
	archive := makeZIP(t, map[string][]byte{"tool.exe": exe})
	l := lockFor(t, archive, exe)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (WindowsVerifier{runner: blockingIdentityRunner{}}).Verify(ctx, "ignored", l); !errors.Is(err, ErrBinaryIdentityMismatch) {
		t.Fatalf("err=%v", err)
	}
}

func TestZIPSafetyAndUniqueHashSelection(t *testing.T) {
	exe := fixtureExecutable(t)
	tests := []struct {
		name    string
		entries map[string][]byte
		want    error
	}{
		{"safe", map[string][]byte{"dir/tool.exe": exe}, nil}, {"parent", map[string][]byte{"../tool.exe": exe}, ErrArchiveUnsafe}, {"absolute", map[string][]byte{"/tool.exe": exe}, ErrArchiveUnsafe}, {"drive", map[string][]byte{"C:/tool.exe": exe}, ErrArchiveUnsafe}, {"backslash", map[string][]byte{"..\\tool.exe": exe}, ErrArchiveUnsafe}, {"device", map[string][]byte{"CON.exe": exe}, ErrArchiveUnsafe}, {"zero", map[string][]byte{"other.exe": []byte("x")}, ErrExecutableNotFound}, {"two", map[string][]byte{"a.exe": exe, "b.exe": exe}, ErrArchiveUnsafe},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			archive := filepath.Join(root, "a.zip")
			if e := os.WriteFile(archive, makeZIP(t, tc.entries), 0600); e != nil {
				t.Fatal(e)
			}
			_, e := (ZIPExtractor{}).Extract(archive, root, hashBytes(exe))
			if !errors.Is(e, tc.want) {
				t.Fatalf("err=%v", e)
			}
		})
	}
}

func TestZIPResourceBoundsAndSymlink(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "a.zip")
	payload := []byte("12345")
	if err := os.WriteFile(archive, makeZIP(t, map[string][]byte{"tool.exe": payload, "second.txt": payload}), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := (ZIPExtractor{MaxEntries: 1}).Extract(archive, root, hashBytes(payload)); !errors.Is(err, ErrArchiveUnsafe) {
		t.Fatalf("entries err=%v", err)
	}
	if _, err := (ZIPExtractor{MaxEntryBytes: 4}).Extract(archive, root, hashBytes(payload)); !errors.Is(err, ErrArchiveUnsafe) {
		t.Fatalf("entry bytes err=%v", err)
	}
	if _, err := (ZIPExtractor{MaxTotalBytes: 7}).Extract(archive, root, hashBytes(payload)); !errors.Is(err, ErrArchiveUnsafe) {
		t.Fatalf("total bytes err=%v", err)
	}
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	h := &zip.FileHeader{Name: "tool.exe", Method: zip.Store}
	h.SetMode(os.ModeSymlink | 0777)
	w, err := zw.CreateHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}
	symlinkArchive := filepath.Join(t.TempDir(), "s.zip")
	if err = os.WriteFile(symlinkArchive, b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := (ZIPExtractor{}).Extract(symlinkArchive, t.TempDir(), hashBytes(payload)); !errors.Is(err, ErrArchiveUnsafe) {
		t.Fatalf("symlink err=%v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(code int, body []byte, length int64) *http.Response {
	return &http.Response{StatusCode: code, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header), ContentLength: length}
}

func TestDownloaderContract(t *testing.T) {
	body := []byte("archive")
	p := upstreamlock.Platform{Artifact: "CLIProxyAPI_7.3.7_windows_amd64.zip", DownloadURL: "https://github.com/router-for-me/CLIProxyAPI/releases/download/v7.3.7/CLIProxyAPI_7.3.7_windows_amd64.zip", ArchiveSHA256: hashBytes(body)}
	calls := 0
	d := NewHTTPDownloaderWithTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != p.DownloadURL || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" || r.Header.Get("Accept") != "application/octet-stream" {
			t.Fatal("request contract")
		}
		return response(200, body, int64(len(body))), nil
	}))
	dest := filepath.Join(t.TempDir(), "a.tmp")
	result, e := d.Download(context.Background(), p, dest)
	if e != nil || result.Bytes != int64(len(body)) || calls != 1 {
		t.Fatalf("%+v %v", result, e)
	}
	bad := p
	bad.DownloadURL = "http://github.com/router-for-me/CLIProxyAPI/releases/download/v7.3.7/" + p.Artifact
	if _, e = d.Download(context.Background(), bad, filepath.Join(t.TempDir(), "b")); !errors.Is(e, ErrDownloadOrigin) || calls != 1 {
		t.Fatalf("origin err=%v calls=%d", e, calls)
	}
}

func TestDownloaderFailures(t *testing.T) {
	p := upstreamlock.Platform{Artifact: "CLIProxyAPI_7.3.7_windows_amd64.zip", DownloadURL: "https://github.com/router-for-me/CLIProxyAPI/releases/download/v7.3.7/CLIProxyAPI_7.3.7_windows_amd64.zip", ArchiveSHA256: strings.Repeat("0", 64)}
	for name, tc := range map[string]struct {
		rt   roundTripFunc
		want error
	}{
		"status":            {func(*http.Request) (*http.Response, error) { return response(500, []byte("error"), 5), nil }, ErrDownloadFailed},
		"declared oversize": {func(*http.Request) (*http.Response, error) { return response(200, nil, MaxArchiveBytes+1), nil }, ErrDownloadTooLarge},
		"hash":              {func(*http.Request) (*http.Response, error) { return response(200, []byte("short"), 5), nil }, ErrArchiveHashMismatch},
	} {
		t.Run(name, func(t *testing.T) {
			d := NewHTTPDownloaderWithTransport(tc.rt)
			_, e := d.Download(context.Background(), p, filepath.Join(t.TempDir(), "x"))
			if !errors.Is(e, tc.want) {
				t.Fatalf("err=%v", e)
			}
		})
	}
}

func TestDownloaderEnforcesStreamingLimit(t *testing.T) {
	body := []byte("123456789")
	p := upstreamlock.Platform{Artifact: "CLIProxyAPI_7.3.7_windows_amd64.zip", DownloadURL: "https://github.com/router-for-me/CLIProxyAPI/releases/download/v7.3.7/CLIProxyAPI_7.3.7_windows_amd64.zip", ArchiveSHA256: hashBytes(body)}
	d := NewHTTPDownloaderWithTransport(roundTripFunc(func(*http.Request) (*http.Response, error) { return response(200, body, -1), nil }))
	d.maxBytes = 8
	if _, err := d.Download(context.Background(), p, filepath.Join(t.TempDir(), "x")); !errors.Is(err, ErrDownloadTooLarge) {
		t.Fatalf("err=%v", err)
	}
}

func TestRedirectPolicyStripsSecrets(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://release-assets.githubusercontent.com/a", nil)
	req.Header.Set("Authorization", "secret")
	req.Header.Set("Cookie", "secret")
	req.Header.Set("Proxy-Authorization", "secret")
	if e := redirect(req, []*http.Request{{}, {}}); e != nil {
		t.Fatal(e)
	}
	for _, h := range []string{"Authorization", "Cookie", "Proxy-Authorization"} {
		if req.Header.Get(h) != "" {
			t.Fatal(h)
		}
	}
	bad, _ := http.NewRequest(http.MethodGet, "https://example.com/a", nil)
	if !errors.Is(redirect(bad, nil), ErrDownloadOrigin) {
		t.Fatal("unexpected host accepted")
	}
	down, _ := http.NewRequest(http.MethodGet, "http://github.com/a", nil)
	if !errors.Is(redirect(down, nil), ErrDownloadOrigin) {
		t.Fatal("downgrade accepted")
	}
	ported, _ := http.NewRequest(http.MethodGet, "https://github.com:443/a", nil)
	if !errors.Is(redirect(ported, nil), ErrDownloadOrigin) {
		t.Fatal("explicit port accepted")
	}
}

func TestStageRejectsReparseRootsAndFinalStages(t *testing.T) {
	base := t.TempDir()
	realRoot := filepath.Join(base, "real")
	junction := filepath.Join(base, "junction")
	if err := os.Mkdir(realRoot, 0700); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", junction, realRoot).CombinedOutput(); err != nil {
		t.Skipf("junction unavailable: %v", string(out))
	}
	locksDir := filepath.Join(base, "locks")
	if err := os.Mkdir(locksDir, 0700); err != nil {
		t.Fatal(err)
	}
	manager, err := lockfile.NewManager(locksDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = New(junction, manager); !errors.Is(err, ErrPersistence) {
		t.Fatalf("junction root err=%v", err)
	}

	exe := fixtureExecutable(t)
	archive := makeZIP(t, map[string][]byte{"tool.exe": exe})
	l := lockFor(t, archive, exe)
	stageRoot := filepath.Join(base, "stage")
	if err = os.Mkdir(stageRoot, 0700); err != nil {
		t.Fatal(err)
	}
	final := filepath.Join(stageRoot, fmt.Sprintf("%s-windows_amd64-%s", l.Version, l.Digest()[:12]))
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", final, realRoot).CombinedOutput(); err != nil {
		t.Skipf("final junction unavailable: %v", string(out))
	}
	s, err := New(stageRoot, manager, WithDownloader(&fakeDownload{data: archive}), WithVerifier(&fakeVerify{identity: Identity{true, true}}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Stage(context.Background(), l); !errors.Is(err, ErrStageConflict) {
		t.Fatalf("final junction err=%v", err)
	}
}

func TestCompletedStageRejectsExtraAndDuplicateManifest(t *testing.T) {
	exe := fixtureExecutable(t)
	archive := makeZIP(t, map[string][]byte{"tool.exe": exe})
	l := lockFor(t, archive, exe)
	d := &fakeDownload{data: archive}
	v := &fakeVerify{identity: Identity{true, true}}
	s, _ := newFixtureStager(t, d, v)
	r, err := s.Stage(context.Background(), l)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(r.Directory, "extra"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Stage(context.Background(), l); !errors.Is(err, ErrStageConflict) {
		t.Fatalf("extra err=%v", err)
	}
	if err = os.Remove(filepath.Join(r.Directory, "extra")); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(r.Directory, "stage-manifest.json")
	b, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	b = bytes.Replace(b, []byte(`"schema_version": 1`), []byte(`"schema_version": 1, "schema_version": 1`), 1)
	if err = os.WriteFile(manifestPath, b, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Stage(context.Background(), l); !errors.Is(err, ErrStageConflict) {
		t.Fatalf("duplicate manifest err=%v", err)
	}
}

func TestProductionSourceHasNoReleaseDiscovery(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		b, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		lower := strings.ToLower(string(b))
		for _, forbidden := range []string{"releases/latest", "latest release", "github api"} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("release discovery token %q in %s", forbidden, entry.Name())
			}
		}
	}
}
