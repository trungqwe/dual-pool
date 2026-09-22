package upstreamstage

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"debug/pe"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/trungqwe/dual-pool/internal/lockfile"
	"github.com/trungqwe/dual-pool/internal/upstreamlock"
	"golang.org/x/sys/windows"
)

const (
	MaxArchiveBytes = 64 << 20
	maxEntries      = 128
	maxEntryBytes   = 128 << 20
	maxTotalBytes   = 256 << 20
	maxProbeBytes   = 64 << 10
)

var (
	ErrUnsupportedPlatform    = errors.New("unsupported upstream platform")
	ErrDownloadOrigin         = errors.New("upstream download origin rejected")
	ErrDownloadFailed         = errors.New("upstream download failed")
	ErrDownloadTooLarge       = errors.New("upstream download exceeds size limit")
	ErrArchiveHashMismatch    = errors.New("upstream archive hash mismatch")
	ErrArchiveUnsafe          = errors.New("upstream archive is unsafe")
	ErrExecutableNotFound     = errors.New("verified upstream executable not found")
	ErrExecutableHashMismatch = errors.New("upstream executable hash mismatch")
	ErrBinaryIdentityMismatch = errors.New("upstream binary identity mismatch")
	ErrStageConflict          = errors.New("upstream stage conflicts with verified identity")
	ErrStageIncomplete        = errors.New("upstream stage is incomplete")
	ErrPersistence            = errors.New("upstream stage persistence failed")
	versionLine               = regexp.MustCompile(`(?m)^CLIProxyAPI Version: ([^,\r\n]+), Commit: ([0-9a-f]+), BuiltAt: [^\r\n]+$`)
)

type DownloadResult struct {
	Bytes         int64
	RedirectHosts []string
}
type Identity struct {
	VersionMatch bool
	CommitMatch  bool
}
type Result struct {
	Directory, Executable string
	Manifest              Manifest
	DownloadedBytes       int64
	RedirectHosts         []string
	Existing              bool
}

type Downloader interface {
	Download(context.Context, upstreamlock.Platform, string) (DownloadResult, error)
}
type Extractor interface {
	Extract(string, string, string) (string, error)
}
type Verifier interface {
	Verify(context.Context, string, upstreamlock.Lock) (Identity, error)
}

type Stager struct {
	root       string
	locks      *lockfile.Manager
	downloader Downloader
	extractor  Extractor
	verifier   Verifier
	platform   string
}

func New(root string, locks *lockfile.Manager, options ...Option) (*Stager, error) {
	s := &Stager{root: filepath.Clean(root), locks: locks, downloader: NewHTTPDownloader(), extractor: ZIPExtractor{}, verifier: WindowsVerifier{}, platform: "windows_amd64"}
	for _, option := range options {
		option(s)
	}
	if locks == nil || s.downloader == nil || s.extractor == nil || s.verifier == nil || !safeDirectoryHierarchy(s.root) {
		return nil, ErrPersistence
	}
	return s, nil
}

type Option func(*Stager)

func WithDownloader(v Downloader) Option { return func(s *Stager) { s.downloader = v } }
func WithExtractor(v Extractor) Option   { return func(s *Stager) { s.extractor = v } }
func WithVerifier(v Verifier) Option     { return func(s *Stager) { s.verifier = v } }
func WithPlatform(v string) Option       { return func(s *Stager) { s.platform = v } }

func (s *Stager) Stage(ctx context.Context, l upstreamlock.Lock) (Result, error) {
	if s.platform != "windows_amd64" || runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		return Result{}, ErrUnsupportedPlatform
	}
	if err := l.Validate(); err != nil {
		return Result{}, err
	}
	guard, err := s.locks.AcquireGlobal()
	if err != nil {
		return Result{}, err
	}
	defer guard.Release()
	if !safeDirectoryHierarchy(s.root) {
		return Result{}, ErrPersistence
	}
	final := filepath.Join(s.root, fmt.Sprintf("%s-windows_amd64-%s", l.Version, l.Digest()[:12]))
	if _, statErr := os.Lstat(final); statErr == nil {
		r, e := validateFinal(final, l)
		if e != nil {
			return Result{}, ErrStageConflict
		}
		r.Existing = true
		return r, nil
	} else if !os.IsNotExist(statErr) {
		return Result{}, ErrPersistence
	}
	attempt, err := os.MkdirTemp(s.root, ".attempt-")
	if err != nil {
		return Result{}, ErrPersistence
	}
	owned := true
	defer func() {
		if owned {
			_ = os.RemoveAll(attempt)
		}
	}()
	archive := filepath.Join(attempt, "download.tmp")
	dl, err := s.downloader.Download(ctx, l.Platforms.WindowsAMD64, archive)
	if err != nil {
		return Result{}, err
	}
	if digestFile(archive) != l.Platforms.WindowsAMD64.ArchiveSHA256 {
		return Result{}, ErrArchiveHashMismatch
	}
	base, err := s.extractor.Extract(archive, attempt, l.Platforms.WindowsAMD64.ExecutableSHA256)
	if err != nil {
		return Result{}, err
	}
	_ = os.Remove(archive)
	exe := filepath.Join(attempt, base)
	if digestFile(exe) != l.Platforms.WindowsAMD64.ExecutableSHA256 {
		return Result{}, ErrExecutableHashMismatch
	}
	if err = verifyPE(exe); err != nil {
		return Result{}, err
	}
	identity, err := s.verifier.Verify(ctx, exe, l)
	if err != nil || !identity.VersionMatch || !identity.CommitMatch {
		return Result{}, ErrBinaryIdentityMismatch
	}
	m := newManifest(l, base, identity)
	if err = writeManifest(filepath.Join(attempt, "stage-manifest.json"), m); err != nil {
		return Result{}, err
	}
	if err = os.Rename(attempt, final); err != nil {
		if _, e := os.Lstat(final); e == nil {
			return Result{}, ErrStageConflict
		}
		return Result{}, ErrPersistence
	}
	owned = false
	r, err := validateFinal(final, l)
	if err != nil {
		return Result{}, err
	}
	r.DownloadedBytes = dl.Bytes
	r.RedirectHosts = append([]string(nil), dl.RedirectHosts...)
	return r, nil
}

type Manifest struct {
	SchemaVersion         int    `json:"schema_version"`
	Product               string `json:"product"`
	Version               string `json:"version"`
	Tag                   string `json:"tag"`
	Commit                string `json:"commit"`
	Platform              string `json:"platform"`
	Artifact              string `json:"artifact"`
	ArchiveSHA256         string `json:"archive_sha256"`
	ExecutableSHA256      string `json:"executable_sha256"`
	ExecutableBasename    string `json:"executable_basename"`
	LockSHA256            string `json:"lock_sha256"`
	BinaryVersionVerified bool   `json:"binary_version_verified"`
	BinaryCommitVerified  bool   `json:"binary_commit_verified"`
}

func newManifest(l upstreamlock.Lock, b string, i Identity) Manifest {
	p := l.Platforms.WindowsAMD64
	return Manifest{1, l.Product, l.Version, l.Tag, l.Commit, "windows_amd64", p.Artifact, p.ArchiveSHA256, p.ExecutableSHA256, b, l.Digest(), i.VersionMatch, i.CommitMatch}
}
func writeManifest(path string, m Manifest) error {
	b, e := json.MarshalIndent(m, "", "  ")
	if e != nil {
		return ErrPersistence
	}
	b = append(b, '\n')
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return ErrPersistence
	}
	if _, e = f.Write(b); e != nil {
		f.Close()
		return ErrPersistence
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return ErrPersistence
	}
	if e = f.Close(); e != nil {
		return ErrPersistence
	}
	return nil
}
func readManifest(path string) (Manifest, error) {
	var m Manifest
	if !safeRegular(path) {
		return m, ErrStageIncomplete
	}
	b, e := os.ReadFile(path)
	if e != nil || len(b) > 8192 || duplicateJSONKeys(b) {
		return m, ErrStageIncomplete
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&m) != nil {
		return m, ErrStageIncomplete
	}
	var x json.RawMessage
	if !errors.Is(d.Decode(&x), io.EOF) {
		return m, ErrStageIncomplete
	}
	return m, nil
}
func validateFinal(dir string, l upstreamlock.Lock) (Result, error) {
	if !safeDirectoryHierarchy(dir) {
		return Result{}, ErrStageIncomplete
	}
	m, e := readManifest(filepath.Join(dir, "stage-manifest.json"))
	if e != nil {
		return Result{}, e
	}
	expected := newManifest(l, m.ExecutableBasename, Identity{true, true})
	if m != expected || !upstreamlock.SafeBasename(m.ExecutableBasename) {
		return Result{}, ErrStageIncomplete
	}
	exe := filepath.Join(dir, m.ExecutableBasename)
	if !safeRegular(exe) || digestFile(exe) != m.ExecutableSHA256 || verifyPE(exe) != nil {
		return Result{}, ErrStageIncomplete
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		return Result{}, ErrStageIncomplete
	}
	for _, entry := range entries {
		if entry.Name() != "stage-manifest.json" && entry.Name() != m.ExecutableBasename {
			return Result{}, ErrStageIncomplete
		}
	}
	return Result{Directory: dir, Executable: exe, Manifest: m}, nil
}
func digestFile(path string) string {
	f, e := os.Open(path)
	if e != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

type ZIPExtractor struct {
	MaxEntries                   int
	MaxEntryBytes, MaxTotalBytes uint64
}

func (z ZIPExtractor) Extract(archive, dest, expected string) (string, error) {
	zr, e := zip.OpenReader(archive)
	if e != nil {
		return "", ErrArchiveUnsafe
	}
	defer zr.Close()
	entryLimit := z.MaxEntries
	if entryLimit == 0 {
		entryLimit = maxEntries
	}
	entryBytes := z.MaxEntryBytes
	if entryBytes == 0 {
		entryBytes = maxEntryBytes
	}
	totalBytes := z.MaxTotalBytes
	if totalBytes == 0 {
		totalBytes = maxTotalBytes
	}
	if len(zr.File) > entryLimit {
		return "", ErrArchiveUnsafe
	}
	matches := 0
	base := ""
	var total uint64
	for _, z := range zr.File {
		if !safeZIPName(z.Name) || z.Mode()&os.ModeSymlink != 0 || !z.Mode().IsRegular() {
			return "", ErrArchiveUnsafe
		}
		if z.UncompressedSize64 > entryBytes || total+z.UncompressedSize64 > totalBytes {
			return "", ErrArchiveUnsafe
		}
		total += z.UncompressedSize64
		r, e := z.Open()
		if e != nil {
			return "", ErrArchiveUnsafe
		}
		h := sha256.New()
		n, e := io.Copy(h, io.LimitReader(r, int64(entryBytes)+1))
		r.Close()
		if e != nil || uint64(n) > entryBytes || uint64(n) != z.UncompressedSize64 {
			return "", ErrArchiveUnsafe
		}
		if hex.EncodeToString(h.Sum(nil)) == expected {
			matches++
			base = filepath.Base(filepath.FromSlash(z.Name))
			if !upstreamlock.SafeBasename(base) {
				return "", ErrArchiveUnsafe
			}
			r, e = z.Open()
			if e != nil {
				return "", ErrArchiveUnsafe
			}
			out, e := os.OpenFile(filepath.Join(dest, base), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
			if e != nil {
				r.Close()
				return "", ErrPersistence
			}
			written, e := io.Copy(out, io.LimitReader(r, int64(entryBytes)+1))
			r.Close()
			if e != nil || written != n || out.Sync() != nil || out.Close() != nil {
				return "", ErrPersistence
			}
		}
	}
	if matches == 0 {
		return "", ErrExecutableNotFound
	}
	if matches != 1 {
		return "", ErrArchiveUnsafe
	}
	return base, nil
}
func safeZIPName(name string) bool {
	if name == "" || strings.ContainsRune(name, 0) || strings.Contains(name, "\\") || strings.HasPrefix(name, "/") {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(name)))
	if clean != name || clean == "." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return false
	}
	for _, p := range strings.Split(name, "/") {
		if !upstreamlock.SafeBasename(p) {
			return false
		}
	}
	return true
}

func verifyPE(path string) error {
	f, e := pe.Open(path)
	if e != nil {
		return ErrBinaryIdentityMismatch
	}
	defer f.Close()
	if f.FileHeader.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
		return ErrBinaryIdentityMismatch
	}
	return nil
}

type identityRunner interface {
	Run(context.Context, string) ([]byte, []byte, error)
}
type WindowsVerifier struct{ runner identityRunner }

// ExpectedIdentity is the release-specific executable identity authority used
// after an immutable provenance catalog has matched the executable hash.
type ExpectedIdentity struct {
	Version string
	Commit  string
}

func (v WindowsVerifier) Verify(ctx context.Context, path string, l upstreamlock.Lock) (Identity, error) {
	return v.VerifyExpected(ctx, path, ExpectedIdentity{Version: l.Version, Commit: l.Commit})
}

// VerifyExpected retains the existing bounded -h parser and requires a
// non-ambiguous seven-character-or-longer commit prefix.
func (v WindowsVerifier) VerifyExpected(ctx context.Context, path string, expected ExpectedIdentity) (Identity, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	runner := v.runner
	if runner == nil {
		runner = commandIdentityRunner{}
	}
	stdout, stderr, e := runner.Run(ctx, path)
	if ctx.Err() != nil || e != nil || len(stdout) > maxProbeBytes || len(stderr) > maxProbeBytes {
		return Identity{}, ErrBinaryIdentityMismatch
	}
	m := versionLine.FindSubmatch(stdout)
	if len(m) != 3 {
		return Identity{}, ErrBinaryIdentityMismatch
	}
	version := strings.TrimPrefix(string(m[1]), "v")
	commit := string(m[2])
	return Identity{version == strings.TrimPrefix(expected.Version, "v"), strings.HasPrefix(expected.Commit, commit) && len(commit) >= 7}, nil
}

type commandIdentityRunner struct{}

func (commandIdentityRunner) Run(ctx context.Context, path string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, path, "-h")
	cmd.Stdin = nil
	cmd.Env = minimalEnv()
	var out, er limitedBuffer
	out.limit = maxProbeBytes
	er.limit = maxProbeBytes
	cmd.Stdout = &out
	cmd.Stderr = &er
	e := cmd.Run()
	if errors.Is(out.err, errOutputLimit) || errors.Is(er.err, errOutputLimit) {
		e = errOutputLimit
	}
	return out.b, er.b, e
}
func minimalEnv() []string {
	r := []string{}
	for _, k := range []string{"SystemRoot", "WINDIR"} {
		if v := os.Getenv(k); v != "" {
			r = append(r, k+"="+v)
		}
	}
	return r
}

var errOutputLimit = errors.New("output limit")

type limitedBuffer struct {
	b     []byte
	limit int
	err   error
}

func (w *limitedBuffer) Write(p []byte) (int, error) {
	if len(w.b)+len(p) > w.limit {
		w.err = errOutputLimit
		return 0, errOutputLimit
	}
	w.b = append(w.b, p...)
	return len(p), nil
}

type HTTPDownloader struct {
	transport http.RoundTripper
	timeout   time.Duration
	maxBytes  int64
}

func NewHTTPDownloader() *HTTPDownloader {
	tr := &http.Transport{Proxy: http.ProxyFromEnvironment, DialContext: (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 15 * time.Second}).DialContext, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 20 * time.Second, IdleConnTimeout: 30 * time.Second, MaxIdleConns: 2, MaxIdleConnsPerHost: 1}
	return &HTTPDownloader{transport: tr, timeout: 2 * time.Minute, maxBytes: MaxArchiveBytes}
}
func NewHTTPDownloaderWithTransport(v http.RoundTripper) *HTTPDownloader {
	return &HTTPDownloader{transport: v, timeout: 2 * time.Minute, maxBytes: MaxArchiveBytes}
}
func redirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 3 || req.URL.Scheme != "https" || !allowedAuthority(req.URL.Host) || req.URL.User != nil {
		return ErrDownloadOrigin
	}
	for _, h := range []string{"Authorization", "Cookie", "Proxy-Authorization"} {
		req.Header.Del(h)
	}
	return nil
}
func allowedHost(h string) bool {
	return h == "github.com" || h == "release-assets.githubusercontent.com"
}
func allowedAuthority(authority string) bool {
	return authority == "github.com" || authority == "release-assets.githubusercontent.com"
}
func (d *HTTPDownloader) Download(ctx context.Context, p upstreamlock.Platform, dest string) (DownloadResult, error) {
	u := p.DownloadURL
	parsed, e := url.Parse(u)
	expectedSuffix := "/" + p.Artifact
	if e != nil || parsed.Scheme != "https" || parsed.Host != "github.com" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || !strings.HasPrefix(parsed.Path, "/router-for-me/CLIProxyAPI/releases/download/") || !strings.HasSuffix(parsed.Path, expectedSuffix) {
		return DownloadResult{}, ErrDownloadOrigin
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if e != nil {
		return DownloadResult{}, ErrDownloadOrigin
	}
	req.Header.Set("User-Agent", "DualPool-Upstream-Stager/1")
	req.Header.Set("Accept", "application/octet-stream")
	hosts := []string{"github.com"}
	client := &http.Client{Transport: d.transport, Timeout: d.timeout, CheckRedirect: func(r *http.Request, v []*http.Request) error {
		hosts = append(hosts, r.URL.Hostname())
		return redirect(r, v)
	}}
	resp, e := client.Do(req)
	if e != nil {
		return DownloadResult{}, ErrDownloadFailed
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return DownloadResult{}, ErrDownloadFailed
	}
	limit := d.maxBytes
	if limit <= 0 {
		limit = MaxArchiveBytes
	}
	if resp.ContentLength > limit {
		return DownloadResult{}, ErrDownloadTooLarge
	}
	f, e := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return DownloadResult{}, ErrPersistence
	}
	h := sha256.New()
	n, e := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, limit+1))
	if e != nil || n > limit {
		f.Close()
		os.Remove(dest)
		if n > limit {
			return DownloadResult{}, ErrDownloadTooLarge
		}
		return DownloadResult{}, ErrDownloadFailed
	}
	if e = f.Sync(); e != nil {
		f.Close()
		os.Remove(dest)
		return DownloadResult{}, ErrPersistence
	}
	if e = f.Close(); e != nil {
		os.Remove(dest)
		return DownloadResult{}, ErrPersistence
	}
	if hex.EncodeToString(h.Sum(nil)) != p.ArchiveSHA256 {
		os.Remove(dest)
		return DownloadResult{}, ErrArchiveHashMismatch
	}
	return DownloadResult{n, hosts}, nil
}

func safeDirectoryHierarchy(path string) bool {
	abs, e := filepath.Abs(path)
	if e != nil || abs != filepath.Clean(path) || len(abs) < 4 || abs[1] != ':' || abs[2] != '\\' || strings.Contains(abs, "/") {
		return false
	}
	current := filepath.VolumeName(abs) + `\`
	parts := strings.Split(strings.TrimPrefix(abs, current), `\`)
	for _, part := range parts {
		if !upstreamlock.SafeBasename(part) {
			return false
		}
		current = filepath.Join(current, part)
		ptr, e := windows.UTF16PtrFromString(current)
		if e != nil {
			return false
		}
		a, e := windows.GetFileAttributes(ptr)
		if e != nil || a&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || a&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
			return false
		}
	}
	return true
}
func safeRegular(path string) bool {
	if !safeDirectoryHierarchy(filepath.Dir(path)) || !upstreamlock.SafeBasename(filepath.Base(path)) {
		return false
	}
	ptr, e := windows.UTF16PtrFromString(path)
	if e != nil {
		return false
	}
	a, e := windows.GetFileAttributes(ptr)
	return e == nil && a&windows.FILE_ATTRIBUTE_REPARSE_POINT == 0 && a&windows.FILE_ATTRIBUTE_DIRECTORY == 0
}

func duplicateJSONKeys(data []byte) bool {
	d := json.NewDecoder(bytes.NewReader(data))
	objects := []map[string]bool{}
	expectKey := []bool{}
	for {
		token, err := d.Token()
		if errors.Is(err, io.EOF) {
			return false
		}
		if err != nil {
			return true
		}
		switch value := token.(type) {
		case json.Delim:
			switch value {
			case '{':
				objects = append(objects, map[string]bool{})
				expectKey = append(expectKey, true)
			case '[':
				objects = append(objects, nil)
				expectKey = append(expectKey, false)
			case '}', ']':
				objects = objects[:len(objects)-1]
				expectKey = expectKey[:len(expectKey)-1]
				if len(objects) > 0 && objects[len(objects)-1] != nil {
					expectKey[len(expectKey)-1] = true
				}
			}
		case string:
			if len(objects) > 0 && objects[len(objects)-1] != nil && expectKey[len(expectKey)-1] {
				if objects[len(objects)-1][value] {
					return true
				}
				objects[len(objects)-1][value] = true
				expectKey[len(expectKey)-1] = false
			} else if len(objects) > 0 && objects[len(objects)-1] != nil {
				expectKey[len(expectKey)-1] = true
			}
		default:
			if len(objects) > 0 && objects[len(objects)-1] != nil {
				expectKey[len(expectKey)-1] = true
			}
		}
	}
}
