//go:build windows

// Package instance owns the closed, local lifecycle for the two CLIProxyAPI
// children.  It deliberately has no provider or OAuth surface.
package instance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/trungqwe/dual-pool/internal/cliproxyconfig"
	"github.com/trungqwe/dual-pool/internal/dataroot"
	"github.com/trungqwe/dual-pool/internal/lockfile"
	"github.com/trungqwe/dual-pool/internal/upstreamlock"
	"github.com/trungqwe/dual-pool/internal/upstreamstage"
	"golang.org/x/sys/windows"
)

var (
	ErrBinaryInstallConflict = errors.New("pinned binary installation conflicts with existing artifact")
	ErrUnsafeInstance        = errors.New("unsafe instance lifecycle artifact")
	ErrIdentityMismatch      = errors.New("managed process identity mismatch")
	ErrNotRunning            = errors.New("managed instance is not running")
	ErrPersistence           = errors.New("instance lifecycle persistence failed")
)

const manifestName = "install-manifest.json"

type ACL interface {
	Create(string) error
	Inspect(string) error
	CreateFile(string) (*os.File, error)
	InspectFile(string) error
}

type InstallManifest struct {
	SchemaVersion        int    `json:"schema_version"`
	Product              string `json:"product"`
	Version              string `json:"version"`
	Tag                  string `json:"tag"`
	Commit               string `json:"commit"`
	Platform             string `json:"platform"`
	ExecutableSHA256     string `json:"executable_sha256"`
	UpstreamLockSHA256   string `json:"upstream_lock_sha256"`
	ConfigAdapterVersion string `json:"config_adapter_version"`
	ExecutableBasename   string `json:"executable_basename"`
}

type ProcessRecord struct {
	SchemaVersion    int    `json:"schema_version"`
	InstanceID       string `json:"instance_id"`
	PID              uint32 `json:"pid"`
	StartTime        uint64 `json:"start_time"`
	ExecutableSHA256 string `json:"executable_sha256"`
	ExecutableImage  string `json:"executable_image"`
	ConfigSHA256     string `json:"config_sha256"`
	Port             int    `json:"port"`
}

type Status struct {
	ID      cliproxyconfig.ID
	Running bool
	Record  ProcessRecord
}

type Manager struct {
	layout    dataroot.Layout
	acl       ACL
	lock      upstreamlock.Lock
	locks     *lockfile.Manager
	inspector lockfile.ProcessInspector
}

func New(layout dataroot.Layout, acl ACL, lock upstreamlock.Lock, opts ...Option) (*Manager, error) {
	if acl == nil || lock.Validate() != nil {
		return nil, ErrUnsafeInstance
	}
	locks, err := lockfile.NewManager(layout.Locks)
	if err != nil {
		return nil, ErrUnsafeInstance
	}
	m := &Manager{layout: layout, acl: acl, lock: lock, locks: locks, inspector: lockfile.WindowsProcessInspector{}}
	for _, opt := range opts {
		opt(m)
	}
	if m.inspector == nil {
		return nil, ErrUnsafeInstance
	}
	return m, nil
}

type Option func(*Manager)

func WithInspector(v lockfile.ProcessInspector) Option { return func(m *Manager) { m.inspector = v } }

func (m *Manager) executableDir() string {
	return filepath.Join(m.layout.Bin, "cliproxyapi", m.lock.Version)
}
func (m *Manager) executablePath() string { return filepath.Join(m.executableDir(), "cliproxyapi.exe") }

// Install accepts only bytes already verified by upstreamstage.  It never
// overwrites a product install: a mismatching final directory is a hard stop.
func (m *Manager) Install(ctx context.Context, stage upstreamstage.Result) (string, bool, error) {
	guard, err := m.locks.AcquireGlobal()
	if err != nil {
		return "", false, ErrPersistence
	}
	defer guard.Release()
	if err = m.preflightRoots(); err != nil {
		return "", false, err
	}
	if err = m.validateStage(ctx, stage); err != nil {
		return "", false, err
	}
	final := m.executableDir()
	if _, e := os.Lstat(final); e == nil {
		if m.validateInstall(ctx, final) == nil {
			return m.executablePath(), true, nil
		}
		return "", false, ErrBinaryInstallConflict
	} else if !os.IsNotExist(e) {
		return "", false, ErrPersistence
	}
	if err = m.acl.Create(filepath.Join(m.layout.Bin, "cliproxyapi")); err != nil {
		return "", false, ErrPersistence
	}
	txn := strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
	attempt := filepath.Join(filepath.Dir(final), "."+m.lock.Version+".install-"+txn)
	if err = m.acl.Create(attempt); err != nil {
		return "", false, ErrPersistence
	}
	owned := true
	defer func() {
		if owned {
			_ = os.RemoveAll(attempt)
		}
	}()
	if err = copyProtected(m.acl, stage.Executable, filepath.Join(attempt, "cliproxyapi.exe")); err != nil {
		return "", false, err
	}
	if err = writeProtectedJSON(m.acl, filepath.Join(attempt, manifestName), m.manifest()); err != nil {
		return "", false, err
	}
	if err = m.validateInstall(ctx, attempt); err != nil {
		return "", false, err
	}
	from, _ := windows.UTF16PtrFromString(attempt)
	to, _ := windows.UTF16PtrFromString(final)
	if windows.MoveFileEx(from, to, windows.MOVEFILE_WRITE_THROUGH) != nil {
		return "", false, ErrBinaryInstallConflict
	}
	owned = false
	if err = m.validateInstall(ctx, final); err != nil {
		return "", false, err
	}
	return m.executablePath(), false, nil
}

func (m *Manager) Start(ctx context.Context, id cliproxyconfig.ID) (Status, error) {
	guard, err := m.locks.AcquireGlobal()
	if err != nil {
		return Status{}, ErrPersistence
	}
	defer guard.Release()
	if err = m.preflightRoots(); err != nil {
		return Status{}, err
	}
	if err = m.validateInstall(ctx, m.executableDir()); err != nil {
		return Status{}, err
	}
	root, port, err := m.instanceRoot(id)
	if err != nil {
		return Status{}, err
	}
	if err = m.validateInstanceRoot(root); err != nil {
		return Status{}, err
	}
	if status, e := m.statusLocked(id); e == nil && status.Running {
		return status, nil
	} else if errors.Is(e, ErrNotRunning) {
		// A protected, schema-valid record whose PID no longer exists is stale
		// state from a natural child exit. It contains no authority to affect a
		// process, so remove only that record before creating a new one.
		if removeErr := os.Remove(filepath.Join(root, "process.json")); removeErr != nil && !os.IsNotExist(removeErr) {
			return Status{}, ErrPersistence
		}
	} else if e != nil && !errors.Is(e, ErrNotRunning) {
		return Status{}, e
	}
	cmd := exec.CommandContext(ctx, m.executablePath(), "-config", filepath.Join(root, "config.yaml"), "-local-model")
	cmd.Dir = root
	cmd.Env = minimalEnv()
	cmd.Stdin = nil
	if err = cmd.Start(); err != nil {
		return Status{}, ErrPersistence
	}
	identity, err := m.inspector.Inspect(uint32(cmd.Process.Pid))
	if err != nil || !samePath(identity.Image, m.executablePath()) {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return Status{}, ErrIdentityMismatch
	}
	record := ProcessRecord{1, string(id), identity.PID, identity.StartTime, digest(m.executablePath()), identity.Image, digest(filepath.Join(root, "config.yaml")), port}
	if !validRecord(record) {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return Status{}, ErrIdentityMismatch
	}
	if err = writeProtectedJSON(m.acl, filepath.Join(root, "process.json"), record); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return Status{}, err
	}
	return Status{ID: id, Running: true, Record: record}, nil
}

func (m *Manager) Stop(id cliproxyconfig.ID) error {
	guard, err := m.locks.AcquireGlobal()
	if err != nil {
		return ErrPersistence
	}
	defer guard.Release()
	status, err := m.statusLocked(id)
	if errors.Is(err, ErrNotRunning) {
		return nil
	}
	if err != nil {
		return err
	}
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, status.Record.PID)
	if err != nil {
		return ErrIdentityMismatch
	}
	defer windows.CloseHandle(h)
	if err = windows.TerminateProcess(h, 1); err != nil {
		return ErrPersistence
	}
	if result, err := windows.WaitForSingleObject(h, 5_000); err != nil || result != windows.WAIT_OBJECT_0 {
		return ErrPersistence
	}
	if err = os.Remove(filepath.Join(m.instancePath(id), "process.json")); err != nil && !os.IsNotExist(err) {
		return ErrPersistence
	}
	return nil
}

func (m *Manager) Restart(ctx context.Context, id cliproxyconfig.ID) (Status, error) {
	if err := m.Stop(id); err != nil {
		return Status{}, err
	}
	return m.Start(ctx, id)
}
func (m *Manager) Status(id cliproxyconfig.ID) (Status, error) { return m.statusLocked(id) }
func (m *Manager) instancePath(id cliproxyconfig.ID) string {
	return filepath.Join(m.layout.Instances, string(id))
}
func (m *Manager) instanceRoot(id cliproxyconfig.ID) (string, int, error) {
	if id == cliproxyconfig.Codex {
		return m.instancePath(id), cliproxyconfig.CodexPort, nil
	}
	if id == cliproxyconfig.Google {
		return m.instancePath(id), cliproxyconfig.GooglePort, nil
	}
	return "", 0, ErrUnsafeInstance
}
func (m *Manager) statusLocked(id cliproxyconfig.ID) (Status, error) {
	root, _, err := m.instanceRoot(id)
	if err != nil {
		return Status{}, err
	}
	path := filepath.Join(root, "process.json")
	if _, e := os.Lstat(path); os.IsNotExist(e) {
		return Status{ID: id}, ErrNotRunning
	}
	if e := m.acl.InspectFile(path); e != nil {
		return Status{}, ErrUnsafeInstance
	}
	var r ProcessRecord
	if e := readJSON(path, &r); e != nil || !validRecord(r) || r.InstanceID != string(id) {
		return Status{}, ErrUnsafeInstance
	}
	live, e := m.inspector.Inspect(r.PID)
	if e != nil {
		return Status{ID: id, Record: r}, ErrNotRunning
	}
	if live.PID != r.PID || live.StartTime != r.StartTime || !samePath(live.Image, r.ExecutableImage) || r.ExecutableSHA256 != digest(m.executablePath()) {
		return Status{}, ErrIdentityMismatch
	}
	return Status{ID: id, Running: true, Record: r}, nil
}
func (m *Manager) manifest() InstallManifest {
	p := m.lock.Platforms.WindowsAMD64
	return InstallManifest{1, m.lock.Product, m.lock.Version, m.lock.Tag, m.lock.Commit, "windows_amd64", p.ExecutableSHA256, m.lock.Digest(), m.lock.ConfigAdapterVersion, "cliproxyapi.exe"}
}
func (m *Manager) validateStage(ctx context.Context, s upstreamstage.Result) error {
	if s.Manifest.ExecutableSHA256 != m.lock.Platforms.WindowsAMD64.ExecutableSHA256 || digest(s.Executable) != m.lock.Platforms.WindowsAMD64.ExecutableSHA256 {
		return ErrBinaryInstallConflict
	}
	i, e := (upstreamstage.WindowsVerifier{}).Verify(ctx, s.Executable, m.lock)
	if e != nil || !i.VersionMatch || !i.CommitMatch {
		return ErrBinaryInstallConflict
	}
	return nil
}
func (m *Manager) validateInstall(ctx context.Context, dir string) error {
	if m.acl.Inspect(dir) != nil || m.acl.InspectFile(filepath.Join(dir, "cliproxyapi.exe")) != nil || m.acl.InspectFile(filepath.Join(dir, manifestName)) != nil {
		return ErrUnsafeInstance
	}
	entries, e := os.ReadDir(dir)
	if e != nil || len(entries) != 2 {
		return ErrUnsafeInstance
	}
	var got InstallManifest
	if readJSON(filepath.Join(dir, manifestName), &got) != nil || got != m.manifest() || digest(filepath.Join(dir, "cliproxyapi.exe")) != got.ExecutableSHA256 {
		return ErrBinaryInstallConflict
	}
	i, e := (upstreamstage.WindowsVerifier{}).Verify(ctx, filepath.Join(dir, "cliproxyapi.exe"), m.lock)
	if e != nil || !i.VersionMatch || !i.CommitMatch {
		return ErrBinaryInstallConflict
	}
	return nil
}
func (m *Manager) preflightRoots() error {
	for _, p := range []string{m.layout.Root, m.layout.Bin, m.layout.Instances, m.layout.Locks} {
		if m.acl.Inspect(p) != nil {
			return ErrUnsafeInstance
		}
	}
	return nil
}
func (m *Manager) validateInstanceRoot(root string) error {
	if m.acl.Inspect(root) != nil || m.acl.Inspect(filepath.Join(root, "auth")) != nil || m.acl.Inspect(filepath.Join(root, "logs")) != nil || m.acl.InspectFile(filepath.Join(root, "config.yaml")) != nil {
		return ErrUnsafeInstance
	}
	entries, e := os.ReadDir(root)
	if e != nil || len(entries) < 3 || len(entries) > 4 {
		return ErrUnsafeInstance
	}
	for _, x := range entries {
		if x.Name() != "auth" && x.Name() != "logs" && x.Name() != "config.yaml" && x.Name() != "process.json" {
			return ErrUnsafeInstance
		}
	}
	if _, e = os.Lstat(filepath.Join(root, ".env")); !os.IsNotExist(e) {
		return ErrUnsafeInstance
	}
	return nil
}
func copyProtected(a ACL, from, to string) error {
	in, e := os.Open(from)
	if e != nil {
		return ErrPersistence
	}
	defer in.Close()
	out, e := a.CreateFile(to)
	if e != nil {
		return ErrPersistence
	}
	_, e = io.Copy(out, in)
	if e == nil {
		e = out.Sync()
	}
	if c := out.Close(); e == nil {
		e = c
	}
	if e != nil {
		return ErrPersistence
	}
	return a.InspectFile(to)
}
func writeProtectedJSON(a ACL, path string, v any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return ErrPersistence
	}
	b = append(b, '\n')
	f, e := a.CreateFile(path)
	if e != nil {
		return ErrPersistence
	}
	_, e = f.Write(b)
	if e == nil {
		e = f.Sync()
	}
	if c := f.Close(); e == nil {
		e = c
	}
	if e != nil {
		return ErrPersistence
	}
	return a.InspectFile(path)
}
func readJSON(path string, v any) error {
	b, e := os.ReadFile(path)
	if e != nil || len(b) == 0 || len(b) > 8192 {
		return ErrUnsafeInstance
	}
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		return ErrUnsafeInstance
	}
	var x any
	if !errors.Is(d.Decode(&x), io.EOF) {
		return ErrUnsafeInstance
	}
	return nil
}
func digest(path string) string {
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
func validRecord(r ProcessRecord) bool {
	return r.SchemaVersion == 1 && (r.InstanceID == "codex" || r.InstanceID == "google") && r.PID != 0 && r.StartTime != 0 && len(r.ExecutableSHA256) == 64 && len(r.ConfigSHA256) == 64 && r.ExecutableImage != "" && (r.Port == cliproxyconfig.CodexPort || r.Port == cliproxyconfig.GooglePort)
}
func samePath(a, b string) bool {
	x, e := filepath.Abs(a)
	if e != nil {
		return false
	}
	y, e := filepath.Abs(b)
	return e == nil && strings.EqualFold(filepath.Clean(x), filepath.Clean(y))
}
func minimalEnv() []string {
	var r []string
	for _, k := range []string{"SystemRoot", "WINDIR"} {
		if v := os.Getenv(k); v != "" {
			r = append(r, k+"="+v)
		}
	}
	return r
}
