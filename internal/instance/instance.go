//go:build windows

// Package instance owns the closed, local lifecycle for the two CLIProxyAPI
// children.  It deliberately has no provider or OAuth surface.
package instance

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
	"unsafe"

	"github.com/trungqwe/dual-pool/internal/cliproxyconfig"
	"github.com/trungqwe/dual-pool/internal/dataroot"
	"github.com/trungqwe/dual-pool/internal/installedslot"
	"github.com/trungqwe/dual-pool/internal/keymaterial"
	"github.com/trungqwe/dual-pool/internal/lockfile"
	"github.com/trungqwe/dual-pool/internal/processidentity"
	"github.com/trungqwe/dual-pool/internal/secretstore"
	"github.com/trungqwe/dual-pool/internal/state"
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
	ErrPortOccupied          = errors.New("instance port is occupied")
	ErrUnverifiable          = errors.New("managed process is unverifiable")
	ErrActiveSelection       = errors.New("active upstream slot selection is invalid")
)

const manifestName = "install-manifest.json"
const markerName = ".install-marker.json"

type ACL interface {
	Create(string) error
	Inspect(string) error
	CreateFile(string) (*os.File, error)
	InspectFile(string) error
}
type SecretReader interface {
	Get(secretstore.Purpose) ([]byte, error)
}
type terminationHandle interface {
	Inspect() (processidentity.Identity, error)
	Terminate() error
	Wait(uint32) error
	Close() error
}
type terminationOpener func(uint32) (terminationHandle, error)

type InstallManifest = installedslot.Manifest
type installMarker struct {
	SchemaVersion     int    `json:"schema_version"`
	TransactionID     string `json:"transaction_id"`
	Version           string `json:"version"`
	CandidateBasename string `json:"candidate_basename"`
	LockSHA256        string `json:"lock_sha256"`
	AdapterVersion    string `json:"adapter_version"`
}

type ProcessRecord struct {
	SchemaVersion    int    `json:"schema_version"`
	InstanceID       string `json:"instance_id"`
	PID              uint32 `json:"pid"`
	StartTime        uint64 `json:"start_time"`
	ExecutableSHA256 string `json:"executable_sha256"`
	ConfigSHA256     string `json:"config_sha256"`
	Port             int    `json:"port"`
	UpstreamVersion  string `json:"upstream_version"`
	ManifestSHA256   string `json:"manifest_sha256"`
}

type Status struct {
	ID      cliproxyconfig.ID
	Running bool
	Record  ProcessRecord
}

type Manager struct {
	layout      dataroot.Layout
	acl         ACL
	lock        upstreamlock.Lock
	locks       *lockfile.Manager
	inspector   lockfile.ProcessInspector
	reader      SecretReader
	opener      terminationOpener
	registry    SlotRegistry
	state       ActiveStateReader
	locksSet    bool
	registrySet bool
	stateSet    bool
	// updater hooks are nil in production. Package tests use them to exercise
	// the real updater adapter and transaction engine without launching a real
	// CLIProxyAPI process.
	updaterStatus       func(cliproxyconfig.ID) (Status, error)
	updaterStop         func(cliproxyconfig.ID) error
	updaterStart        func(context.Context, cliproxyconfig.ID) (Status, error)
	updaterPortOccupied func(int) (bool, error)
}

type SlotRegistry interface {
	Resolve(string) (installedslot.ResolvedSlot, error)
	VerifyInstalled(context.Context, string) error
	RegisterLocked(context.Context, string) error
}

type ActiveStateReader interface {
	LoadState() (state.State, error)
}

// UpdaterLifecycle is the only lifecycle bridge for update.Updater. Its
// methods require the caller to already hold the shared GLOBAL lock.
type UpdaterLifecycle struct{ manager *Manager }

// UpdaterLifecycle returns a bridge that deliberately uses Manager's locked
// lifecycle operations. Public Manager methods acquire GLOBAL themselves and
// must not be used while an update transaction owns it.
func (m *Manager) UpdaterLifecycle() *UpdaterLifecycle { return &UpdaterLifecycle{manager: m} }

func (l *UpdaterLifecycle) CaptureRunning(ctx context.Context) ([]state.Pool, error) {
	if l == nil || l.manager == nil {
		return nil, ErrUnsafeInstance
	}
	out := make([]state.Pool, 0, 2)
	for _, pair := range []struct {
		pool state.Pool
		id   cliproxyconfig.ID
	}{{state.PoolCodex, cliproxyconfig.Codex}, {state.PoolGoogle, cliproxyconfig.Google}} {
		status, err := l.manager.updaterStatusLocked(pair.id)
		if err == nil {
			if !status.Running {
				return nil, ErrUnverifiable
			}
			out = append(out, pair.pool)
			continue
		}
		if !errors.Is(err, ErrNotRunning) {
			return nil, err
		}
		if status.Record.PID != 0 {
			occupied, checkErr := l.manager.updaterPortIsOccupied(status.Record.Port)
			if checkErr != nil || occupied {
				return nil, ErrUnverifiable
			}
		}
	}
	return out, nil
}

func (m *Manager) updaterPortIsOccupied(port int) (bool, error) {
	if m.updaterPortOccupied != nil {
		return m.updaterPortOccupied(port)
	}
	return m.portOccupied(port)
}

func (l *UpdaterLifecycle) Stop(_ context.Context, pools []state.Pool) error {
	if l == nil || l.manager == nil {
		return ErrUnsafeInstance
	}
	pools, err := canonicalLifecyclePools(pools)
	if err != nil {
		return err
	}
	for _, pool := range pools {
		id, err := lifecyclePoolID(pool)
		if err != nil {
			return err
		}
		if err := l.manager.updaterStopLocked(id); err != nil {
			return err
		}
	}
	return nil
}

func (l *UpdaterLifecycle) Start(ctx context.Context, pools []state.Pool) error {
	if l == nil || l.manager == nil {
		return ErrUnsafeInstance
	}
	pools, err := canonicalLifecyclePools(pools)
	if err != nil {
		return err
	}
	for _, pool := range pools {
		id, err := lifecyclePoolID(pool)
		if err != nil {
			return err
		}
		if _, err := l.manager.updaterStartLocked(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) updaterStatusLocked(id cliproxyconfig.ID) (Status, error) {
	if m.updaterStatus != nil {
		return m.updaterStatus(id)
	}
	return m.statusLocked(id)
}

func (m *Manager) updaterStopLocked(id cliproxyconfig.ID) error {
	if m.updaterStop != nil {
		return m.updaterStop(id)
	}
	return m.stopLocked(id)
}

func (m *Manager) updaterStartLocked(ctx context.Context, id cliproxyconfig.ID) (Status, error) {
	if m.updaterStart != nil {
		return m.updaterStart(ctx, id)
	}
	return m.startLocked(ctx, id)
}

func canonicalLifecyclePools(pools []state.Pool) ([]state.Pool, error) {
	seen := map[state.Pool]bool{}
	for _, pool := range pools {
		if _, err := lifecyclePoolID(pool); err != nil || seen[pool] {
			return nil, ErrUnsafeInstance
		}
		seen[pool] = true
	}
	out := make([]state.Pool, 0, len(pools))
	for _, pool := range []state.Pool{state.PoolCodex, state.PoolGoogle} {
		if seen[pool] {
			out = append(out, pool)
		}
	}
	return out, nil
}

func lifecyclePoolID(pool state.Pool) (cliproxyconfig.ID, error) {
	switch pool {
	case state.PoolCodex:
		return cliproxyconfig.Codex, nil
	case state.PoolGoogle:
		return cliproxyconfig.Google, nil
	default:
		return "", ErrUnsafeInstance
	}
}

func New(layout dataroot.Layout, acl ACL, lock upstreamlock.Lock, opts ...Option) (*Manager, error) {
	if acl == nil || lock.Validate() != nil {
		return nil, ErrUnsafeInstance
	}
	m := &Manager{layout: layout, acl: acl, lock: lock, inspector: lockfile.WindowsProcessInspector{}, reader: secretstore.New(), opener: openForTermination}
	for _, opt := range opts {
		opt(m)
	}
	if (m.locksSet && m.locks == nil) || (m.registrySet && m.registry == nil) || (m.stateSet && m.state == nil) {
		return nil, ErrUnsafeInstance
	}
	if !m.locksSet {
		locks, err := lockfile.NewManager(layout.Locks)
		if err != nil {
			return nil, ErrUnsafeInstance
		}
		m.locks = locks
	}
	if !m.registrySet {
		registry, err := installedslot.New(layout, acl, lock)
		if err != nil {
			return nil, ErrUnsafeInstance
		}
		m.registry = registry
	}
	if !m.stateSet {
		stateStore, err := state.NewStore(layout.State, state.WithLockManager(m.locks))
		if err != nil {
			return nil, ErrUnsafeInstance
		}
		m.state = stateStore
	}
	if m.locks == nil || m.inspector == nil || m.reader == nil || m.opener == nil || m.registry == nil || m.state == nil {
		return nil, ErrUnsafeInstance
	}
	return m, nil
}

func openForTermination(pid uint32) (terminationHandle, error) {
	return processidentity.OpenForTermination(pid)
}

type Option func(*Manager)

func WithInspector(v lockfile.ProcessInspector) Option { return func(m *Manager) { m.inspector = v } }
func WithSecretReader(v SecretReader) Option           { return func(m *Manager) { m.reader = v } }
func WithSlotRegistry(v SlotRegistry) Option {
	return func(m *Manager) { m.registrySet, m.registry = true, v }
}
func WithStateReader(v ActiveStateReader) Option {
	return func(m *Manager) { m.stateSet, m.state = true, v }
}
func WithLockManager(v *lockfile.Manager) Option {
	return func(m *Manager) { m.locksSet, m.locks = true, v }
}

func (m *Manager) executableDir() string {
	return filepath.Join(m.layout.Bin, "cliproxyapi", m.lock.Version)
}
func (m *Manager) executablePath() string { return filepath.Join(m.executableDir(), "cliproxyapi.exe") }

func (m *Manager) activeSlot(ctx context.Context) (installedslot.ResolvedSlot, error) {
	if m.state == nil || m.registry == nil {
		return installedslot.ResolvedSlot{}, ErrActiveSelection
	}
	if recoverer, ok := m.state.(interface{ Recover() error }); ok {
		if err := recoverer.Recover(); err != nil {
			return installedslot.ResolvedSlot{}, ErrActiveSelection
		}
	}
	active, err := m.state.LoadState()
	if err != nil || state.ValidateState(active) != nil || active.ActiveUpstreamVersion == "" {
		return installedslot.ResolvedSlot{}, ErrActiveSelection
	}
	if err = m.registry.VerifyInstalled(ctx, active.ActiveUpstreamVersion); err != nil {
		return installedslot.ResolvedSlot{}, err
	}
	slot, err := m.registry.Resolve(active.ActiveUpstreamVersion)
	if err != nil {
		return installedslot.ResolvedSlot{}, err
	}
	if slot.ConfigAdapterVersion != m.lock.ConfigAdapterVersion || slot.Platform != "windows_amd64" || slot.ExecutableBasename != "cliproxyapi.exe" {
		return installedslot.ResolvedSlot{}, ErrActiveSelection
	}
	return slot, nil
}

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
	if err = m.acl.Create(filepath.Join(m.layout.Bin, "cliproxyapi")); err != nil {
		return "", false, ErrPersistence
	}
	if err = m.recoverInstallMarker(ctx); err != nil {
		return "", false, err
	}
	final := m.executableDir()
	if _, e := os.Lstat(final); e == nil {
		if m.validateInstall(ctx, final) == nil {
			if m.registry == nil {
				return "", false, ErrUnsafeInstance
			}
			if err := m.registry.RegisterLocked(ctx, m.lock.Version); err != nil {
				return "", false, err
			}
			return m.executablePath(), true, nil
		}
		return "", false, ErrBinaryInstallConflict
	} else if !os.IsNotExist(e) {
		return "", false, ErrPersistence
	}
	txn, err := randomTransaction()
	if err != nil {
		return "", false, ErrPersistence
	}
	attempt := filepath.Join(filepath.Dir(final), "."+m.lock.Version+".install-"+txn)
	if err = writeProtectedJSON(m.acl, filepath.Join(filepath.Dir(final), markerName), m.marker(txn, filepath.Base(attempt))); err != nil {
		return "", false, err
	}
	if err = m.acl.Create(attempt); err != nil {
		return "", false, ErrPersistence
	}
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
	if err = m.validateInstall(ctx, final); err != nil {
		return "", false, err
	}
	if m.registry == nil {
		return "", false, ErrUnsafeInstance
	}
	if err = m.registry.RegisterLocked(ctx, m.lock.Version); err != nil {
		return "", false, err
	}
	if err = os.Remove(filepath.Join(filepath.Dir(final), markerName)); err != nil {
		return "", false, ErrPersistence
	}
	return m.executablePath(), false, nil
}
func (m *Manager) marker(txn, candidate string) installMarker {
	return installMarker{1, txn, m.lock.Version, candidate, m.lock.Digest(), m.lock.ConfigAdapterVersion}
}
func randomTransaction() (string, error) {
	b := make([]byte, 16)
	if _, e := io.ReadFull(rand.Reader, b); e != nil {
		return "", e
	}
	return hex.EncodeToString(b), nil
}
func (m *Manager) recoverInstallMarker(ctx context.Context) error {
	dir := filepath.Dir(m.executableDir())
	path := filepath.Join(dir, markerName)
	if _, e := os.Lstat(path); os.IsNotExist(e) {
		return nil
	}
	if m.acl.InspectFile(path) != nil {
		return ErrBinaryInstallConflict
	}
	var mark installMarker
	if e := readJSON(path, &mark); e != nil || !validMarker(mark, m) {
		return ErrBinaryInstallConflict
	}
	final := m.executableDir()
	candidate := filepath.Join(dir, mark.CandidateBasename)
	if _, e := os.Lstat(final); e == nil {
		if e = m.validateInstall(ctx, final); e != nil {
			return e
		}
		if m.registry == nil {
			return ErrUnsafeInstance
		}
		if e = m.registry.RegisterLocked(ctx, m.lock.Version); e != nil {
			return e
		}
	}
	if _, e := os.Lstat(candidate); e == nil {
		if mark.CandidateBasename != "."+m.lock.Version+".install-"+mark.TransactionID || m.acl.Inspect(candidate) != nil {
			return ErrBinaryInstallConflict
		}
		if e = m.validatePartialCandidate(ctx, candidate); e != nil {
			return ErrBinaryInstallConflict
		}
		if e = os.RemoveAll(candidate); e != nil {
			return ErrPersistence
		}
	}
	if e := os.Remove(path); e != nil {
		return ErrPersistence
	}
	return nil
}
func (m *Manager) validatePartialCandidate(ctx context.Context, dir string) error {
	if m.acl.Inspect(dir) != nil {
		return ErrUnsafeInstance
	}
	entries, e := os.ReadDir(dir)
	if e != nil || len(entries) > 2 {
		return ErrUnsafeInstance
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || (entry.Name() != "cliproxyapi.exe" && entry.Name() != manifestName) || seen[entry.Name()] || m.acl.InspectFile(filepath.Join(dir, entry.Name())) != nil {
			return ErrUnsafeInstance
		}
		seen[entry.Name()] = true
	}
	if seen[manifestName] && !seen["cliproxyapi.exe"] {
		return ErrUnsafeInstance
	}
	if seen["cliproxyapi.exe"] && seen[manifestName] {
		// A complete candidate is additionally validated. A failed validation is
		// still a safe crash artifact when the marker, topology and filenames
		// prove ownership; recovery discards it rather than promoting it.
		_ = m.validateInstall(ctx, dir)
	}
	return nil
}
func validMarker(v installMarker, m *Manager) bool {
	return v.SchemaVersion == 1 && transactionPattern.MatchString(v.TransactionID) && v.Version == m.lock.Version && v.LockSHA256 == m.lock.Digest() && v.AdapterVersion == m.lock.ConfigAdapterVersion && v.CandidateBasename == "."+v.Version+".install-"+v.TransactionID
}

func (m *Manager) Start(ctx context.Context, id cliproxyconfig.ID) (Status, error) {
	guard, err := m.locks.AcquireGlobal()
	if err != nil {
		return Status{}, ErrPersistence
	}
	defer guard.Release()
	return m.startLocked(ctx, id)
}
func (m *Manager) startLocked(ctx context.Context, id cliproxyconfig.ID) (Status, error) {
	var err error
	if err = m.preflightRoots(); err != nil {
		return Status{}, err
	}
	slot, err := m.activeSlot(ctx)
	if err != nil {
		return Status{}, err
	}
	root, port, err := m.instanceRoot(id)
	if err != nil {
		return Status{}, err
	}
	if err = m.validateInstanceRoot(root); err != nil {
		return Status{}, err
	}
	if validator, validateErr := cliproxyconfig.New(m.layout, m.acl, m.reader, m.lock); validateErr != nil {
		return Status{}, ErrUnsafeInstance
	} else if state, validateErr := validator.InspectPair(); validateErr != nil || !state.Ready {
		return Status{}, ErrUnsafeInstance
	}
	if status, e := m.statusLocked(id); e == nil && status.Running {
		if status.Record.UpstreamVersion != slot.Version {
			return Status{}, ErrActiveSelection
		}
		if e = m.awaitReady(ctx, id, status.Record); e != nil {
			return Status{}, e
		}
		return status, nil
	} else if errors.Is(e, ErrNotRunning) {
		// A protected, schema-valid record whose PID no longer exists is stale
		// state from a natural child exit. It contains no authority to affect a
		// process, so remove only that record before creating a new one.
		if occupied, inspectErr := m.portOccupied(port); inspectErr != nil {
			return Status{}, ErrUnverifiable
		} else if occupied {
			return Status{}, ErrPortOccupied
		}
		if removeErr := os.Remove(filepath.Join(root, "process.json")); removeErr != nil && !os.IsNotExist(removeErr) {
			return Status{}, ErrPersistence
		}
	} else if e != nil && !errors.Is(e, ErrNotRunning) {
		return Status{}, e
	}
	if occupied, inspectErr := m.portOccupied(port); inspectErr != nil {
		return Status{}, ErrUnverifiable
	} else if occupied {
		return Status{}, ErrPortOccupied
	}
	cmd := exec.Command(slot.ExecutablePath, "-config", filepath.Join(root, "config.yaml"), "-local-model")
	cmd.Dir = root
	cmd.Env = minimalEnv()
	cmd.Stdin = nil
	if err = cmd.Start(); err != nil {
		return Status{}, ErrPersistence
	}
	identity, err := m.inspector.Inspect(uint32(cmd.Process.Pid))
	if err != nil || !samePath(identity.Image, slot.ExecutablePath) {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return Status{}, ErrIdentityMismatch
	}
	record := ProcessRecord{SchemaVersion: 2, InstanceID: string(id), PID: identity.PID, StartTime: identity.StartTime, ExecutableSHA256: digest(slot.ExecutablePath), ConfigSHA256: digest(filepath.Join(root, "config.yaml")), Port: port, UpstreamVersion: slot.Version, ManifestSHA256: slot.ManifestSHA256}
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
	if err = m.awaitReady(ctx, id, record); err != nil {
		if cleanupErr := m.cleanupOwned(id, record); cleanupErr != nil {
			return Status{}, cleanupErr
		}
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
	return m.stopLocked(id)
}
func (m *Manager) stopLocked(id cliproxyconfig.ID) error {
	status, err := m.statusLocked(id)
	if errors.Is(err, ErrNotRunning) {
		if status.Record.PID == 0 {
			return nil
		}
		occupied, e := m.portOccupied(status.Record.Port)
		if e != nil {
			return ErrUnverifiable
		}
		if occupied {
			return ErrPortOccupied
		}
		path := filepath.Join(m.instancePath(id), "process.json")
		if m.acl.InspectFile(path) != nil {
			return ErrUnsafeInstance
		}
		var current ProcessRecord
		if e = readJSON(path, &current); e != nil || current != status.Record {
			return ErrIdentityMismatch
		}
		if e = os.Remove(path); e != nil {
			return ErrPersistence
		}
		return nil
	}
	if err != nil {
		return err
	}
	return m.cleanupOwned(id, status.Record)
}
func (m *Manager) cleanupOwned(id cliproxyconfig.ID, record ProcessRecord) error {
	if err := m.stopRecord(record); err != nil {
		return err
	}
	occupied, e := m.portOccupied(record.Port)
	if e != nil {
		return ErrUnverifiable
	}
	if occupied {
		return ErrPersistence
	}
	path := filepath.Join(m.instancePath(id), "process.json")
	if m.acl.InspectFile(path) != nil {
		return ErrUnsafeInstance
	}
	var current ProcessRecord
	if e = readJSON(path, &current); e != nil || current != record {
		return ErrIdentityMismatch
	}
	if e = os.Remove(path); e != nil {
		return ErrPersistence
	}
	return nil
}
func (m *Manager) stopRecord(record ProcessRecord) error {
	slot, err := m.recordSlot(record)
	if err != nil {
		return ErrIdentityMismatch
	}
	h, err := m.opener(record.PID)
	if err != nil {
		return ErrIdentityMismatch
	}
	defer h.Close()
	live, err := h.Inspect()
	if err != nil || !m.matchesLive(record, live, slot.ExecutablePath) {
		return ErrIdentityMismatch
	}
	if err = h.Terminate(); err != nil {
		return ErrPersistence
	}
	if err = h.Wait(5000); err != nil {
		return ErrPersistence
	}
	return nil
}

func (m *Manager) Restart(ctx context.Context, id cliproxyconfig.ID) (Status, error) {
	guard, err := m.locks.AcquireGlobal()
	if err != nil {
		return Status{}, ErrPersistence
	}
	defer guard.Release()
	if err := m.stopLocked(id); err != nil {
		return Status{}, err
	}
	return m.startLocked(ctx, id)
}
func (m *Manager) Status(id cliproxyconfig.ID) (Status, error) {
	if m.locks == nil {
		return m.statusLocked(id)
	}
	guard, err := m.locks.AcquireGlobal()
	if err != nil {
		return Status{}, ErrPersistence
	}
	defer guard.Release()
	return m.statusLocked(id)
}
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
	slot, e := m.recordSlot(r)
	if e != nil {
		return Status{ID: id, Record: r}, ErrUnverifiable
	}
	live, e := m.inspector.Inspect(r.PID)
	if errors.Is(e, lockfile.ErrProcessNotFound) {
		return Status{ID: id, Record: r}, ErrNotRunning
	}
	if e != nil {
		return Status{ID: id, Record: r}, ErrUnverifiable
	}
	if !m.matchesLive(r, processidentity.Identity{PID: live.PID, StartTime: live.StartTime, Image: live.Image}, slot.ExecutablePath) {
		return Status{}, ErrIdentityMismatch
	}
	return Status{ID: id, Running: true, Record: r}, nil
}
func (m *Manager) matchesLive(r ProcessRecord, live processidentity.Identity, expectedPath string) bool {
	return live.PID == r.PID && live.StartTime == r.StartTime && samePath(live.Image, expectedPath) && r.ExecutableSHA256 == digest(expectedPath)
}

func (m *Manager) recordSlot(record ProcessRecord) (installedslot.ResolvedSlot, error) {
	if record.UpstreamVersion == "" || m.registry == nil {
		return installedslot.ResolvedSlot{}, ErrIdentityMismatch
	}
	if err := m.registry.VerifyInstalled(context.Background(), record.UpstreamVersion); err != nil {
		return installedslot.ResolvedSlot{}, err
	}
	slot, err := m.registry.Resolve(record.UpstreamVersion)
	if err != nil || slot.ExecutableSHA256 != record.ExecutableSHA256 || slot.ManifestSHA256 != record.ManifestSHA256 {
		return installedslot.ResolvedSlot{}, ErrIdentityMismatch
	}
	return slot, nil
}
func (m *Manager) manifest() InstallManifest {
	p := m.lock.Platforms.WindowsAMD64
	return InstallManifest{SchemaVersion: 1, Product: m.lock.Product, Version: m.lock.Version, Tag: m.lock.Tag, Commit: m.lock.Commit, Platform: "windows_amd64", ExecutableSHA256: p.ExecutableSHA256, UpstreamLockSHA256: m.lock.Digest(), ConfigAdapterVersion: m.lock.ConfigAdapterVersion, ExecutableBasename: "cliproxyapi.exe"}
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
	if e != nil || len(b) == 0 || len(b) > 8192 || !utf8.Valid(b) || duplicateJSONKeys(b) {
		return ErrUnsafeInstance
	}
	d := json.NewDecoder(bytes.NewReader(b))
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
func duplicateJSONKeys(data []byte) bool {
	d := json.NewDecoder(bytes.NewReader(data))
	stack := []map[string]bool{}
	expects := []bool{}
	for {
		t, e := d.Token()
		if errors.Is(e, io.EOF) {
			return false
		}
		if e != nil {
			return true
		}
		switch x := t.(type) {
		case json.Delim:
			switch x {
			case '{':
				stack = append(stack, map[string]bool{})
				expects = append(expects, true)
			case '[':
				stack = append(stack, nil)
				expects = append(expects, false)
			case '}', ']':
				stack = stack[:len(stack)-1]
				expects = expects[:len(expects)-1]
				if len(stack) > 0 && stack[len(stack)-1] != nil {
					expects[len(expects)-1] = true
				}
			}
		case string:
			if len(stack) > 0 && stack[len(stack)-1] != nil && expects[len(expects)-1] {
				if stack[len(stack)-1][x] {
					return true
				}
				stack[len(stack)-1][x] = true
				expects[len(expects)-1] = false
			} else if len(stack) > 0 && stack[len(stack)-1] != nil {
				expects[len(expects)-1] = true
			}
		default:
			if len(stack) > 0 && stack[len(stack)-1] != nil {
				expects[len(expects)-1] = true
			}
		}
	}
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

var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
var transactionPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

func validRecord(r ProcessRecord) bool {
	return r.SchemaVersion == 2 && r.PID > 0 && r.StartTime > 0 && digestPattern.MatchString(r.ExecutableSHA256) && digestPattern.MatchString(r.ConfigSHA256) && digestPattern.MatchString(r.ManifestSHA256) && installedslot.ValidVersion(r.UpstreamVersion) && ((r.InstanceID == "codex" && r.Port == cliproxyconfig.CodexPort) || (r.InstanceID == "google" && r.Port == cliproxyconfig.GooglePort))
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

type listener struct {
	address string
	port    int
	pid     uint32
	ipv6    bool
}
type tcp4row struct {
	state      uint32
	localAddr  uint32
	localPort  uint32
	remoteAddr uint32
	remotePort uint32
	pid        uint32
}
type tcp4tableLayout struct {
	count uint32
	rows  [1]tcp4row
}
type tcp6tableLayout struct {
	count uint32
	rows  [1]tcp6row
}
type tcp6row struct {
	localAddr     [16]byte
	localScopeID  uint32
	localPort     uint32
	remoteAddr    [16]byte
	remoteScopeID uint32
	remotePort    uint32
	state         uint32
	pid           uint32
}

var iphlpapi = windows.NewLazySystemDLL("iphlpapi.dll")
var getExtendedTCPTable = iphlpapi.NewProc("GetExtendedTcpTable")

const tcpTableOwnerPIDListener = 3

func (m *Manager) portOccupied(port int) (bool, error) {
	all, err := tcpListeners()
	if err != nil {
		return false, err
	}
	for _, l := range all {
		if l.port == port {
			return true, nil
		}
	}
	return false, nil
}
func tcpListeners() ([]listener, error) {
	var out []listener
	v4, e := tcpTable4()
	if e != nil {
		return nil, e
	}
	v6, e := tcpTable6()
	if e != nil {
		return nil, e
	}
	out = append(out, v4...)
	out = append(out, v6...)
	return out, nil
}
func tcpBuffer(af uint32) ([]byte, error) {
	var size uint32
	r, _, _ := getExtendedTCPTable.Call(0, uintptr(unsafe.Pointer(&size)), 1, uintptr(af), tcpTableOwnerPIDListener, 0)
	if r != 122 || size < 4 || size > 1<<20 {
		return nil, ErrUnverifiable
	}
	b := make([]byte, size)
	r, _, _ = getExtendedTCPTable.Call(uintptr(unsafe.Pointer(&b[0])), uintptr(unsafe.Pointer(&size)), 1, uintptr(af), tcpTableOwnerPIDListener, 0)
	if r != 0 || size > uint32(len(b)) {
		return nil, ErrUnverifiable
	}
	return b[:size], nil
}
func tcpTable4() ([]listener, error) {
	b, e := tcpBuffer(2)
	if e != nil {
		return nil, e
	}
	return decodeTCP4(b)
}
func tcpTable6() ([]listener, error) {
	b, e := tcpBuffer(23)
	if e != nil {
		return nil, e
	}
	return decodeTCP6(b)
}
func decodeTCP4(b []byte) ([]listener, error) {
	const first = unsafe.Offsetof(tcp4tableLayout{}.rows)
	const stride = unsafe.Sizeof(tcp4row{})
	if first < 4 || stride != 24 || len(b) < int(first) {
		return nil, ErrUnverifiable
	}
	n := binary.LittleEndian.Uint32(b[:4])
	if uint64(first)+uint64(n)*uint64(stride) > uint64(len(b)) {
		return nil, ErrUnverifiable
	}
	out := make([]listener, 0, n)
	for i := uint32(0); i < n; i++ {
		offset := first + uintptr(i)*stride
		row := (*tcp4row)(unsafe.Pointer(&b[offset]))
		if row.state != 2 {
			continue
		}
		addr := (*[4]byte)(unsafe.Pointer(&row.localAddr))
		port := (*[4]byte)(unsafe.Pointer(&row.localPort))
		out = append(out, listener{fmtIPv4(addr[:]), int(binary.BigEndian.Uint16(port[:2])), row.pid, false})
	}
	return out, nil
}
func decodeTCP6(b []byte) ([]listener, error) {
	const first = unsafe.Offsetof(tcp6tableLayout{}.rows)
	const stride = unsafe.Sizeof(tcp6row{})
	if first < 4 || stride != 56 || len(b) < int(first) {
		return nil, ErrUnverifiable
	}
	n := binary.LittleEndian.Uint32(b[:4])
	if uint64(first)+uint64(n)*uint64(stride) > uint64(len(b)) {
		return nil, ErrUnverifiable
	}
	out := make([]listener, 0, n)
	for i := uint32(0); i < n; i++ {
		offset := first + uintptr(i)*stride
		row := (*tcp6row)(unsafe.Pointer(&b[offset]))
		if row.state != 2 {
			continue
		}
		port := (*[4]byte)(unsafe.Pointer(&row.localPort))
		out = append(out, listener{"ipv6", int(binary.BigEndian.Uint16(port[:2])), row.pid, true})
	}
	return out, nil
}
func fmtIPv4(b []byte) string { return fmt.Sprintf("%d.%d.%d.%d", b[0], b[1], b[2], b[3]) }

func (m *Manager) awaitReady(parent context.Context, id cliproxyconfig.ID, record ProcessRecord) error {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	for delay := 25 * time.Millisecond; ; {
		if m.checkL0(id, record) == nil && m.checkL1(ctx, record) == nil && m.checkL2(ctx, id) == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ErrPersistence
		case <-time.After(delay):
			if delay < 250*time.Millisecond {
				delay *= 2
			}
		}
	}
}
func (m *Manager) checkL0(id cliproxyconfig.ID, r ProcessRecord) error {
	s, e := m.statusLocked(id)
	if e != nil || !s.Running || s.Record != r {
		return ErrIdentityMismatch
	}
	return nil
}
func (m *Manager) checkL1(ctx context.Context, r ProcessRecord) error {
	all, err := tcpListeners()
	if err != nil {
		return ErrUnverifiable
	}
	expectedPortCount := 0
	managedCount := 0
	for _, l := range all {
		if l.port == r.Port {
			expectedPortCount++
			if l.ipv6 || l.address != "127.0.0.1" || l.pid != r.PID {
				return ErrPersistence
			}
		}
		if l.pid != r.PID {
			continue
		}
		managedCount++
		if l.ipv6 || l.address != "127.0.0.1" || l.port != r.Port {
			return ErrPersistence
		}
	}
	if expectedPortCount != 1 || managedCount != 1 {
		return ErrPersistence
	}
	return m.request(ctx, r.Port, "/healthz", "", "", 200)
}
func (m *Manager) checkL2(ctx context.Context, id cliproxyconfig.ID) error {
	ownClient, ownMgmt, otherClient, otherMgmt := purposes(id)
	for _, c := range []struct {
		p            secretstore.Purpose
		path, header string
		status       int
	}{{ownClient, "/v1/models", "Authorization", 200}, {otherClient, "/v1/models", "Authorization", 401}, {ownMgmt, "/v0/management/debug", "X-Management-Key", 200}, {otherMgmt, "/v0/management/debug", "X-Management-Key", 401}} {
		if e := m.requestWithKey(ctx, id, c.p, c.path, c.header, c.status); e != nil {
			return e
		}
	}
	_, port, _ := m.instanceRoot(id)
	if e := m.request(ctx, port, "/v1/models", "", "", 401); e != nil {
		return e
	}
	return m.request(ctx, port, "/v0/management/debug", "", "", 401)
}
func purposes(id cliproxyconfig.ID) (secretstore.Purpose, secretstore.Purpose, secretstore.Purpose, secretstore.Purpose) {
	if id == cliproxyconfig.Codex {
		return secretstore.CodexClientKey, secretstore.CodexManagementKey, secretstore.GoogleClientKey, secretstore.GoogleManagementKey
	}
	return secretstore.GoogleClientKey, secretstore.GoogleManagementKey, secretstore.CodexClientKey, secretstore.CodexManagementKey
}
func (m *Manager) requestWithKey(ctx context.Context, id cliproxyconfig.ID, p secretstore.Purpose, path, header string, status int) error {
	raw, e := m.reader.Get(p)
	if e != nil {
		return ErrPersistence
	}
	wire, e := keymaterial.Encode(raw)
	secretstore.Zero(raw)
	if e != nil {
		return ErrPersistence
	}
	defer secretstore.Zero(wire)
	_, port, _ := m.instanceRoot(id)
	value := string(wire)
	if header == "Authorization" {
		value = "Bearer " + value
	}
	return m.request(ctx, port, path, header, value, status)
}
func (m *Manager) request(ctx context.Context, port int, path, header, value string, want int) error {
	tr := &http.Transport{Proxy: nil}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d%s", port, path), nil)
	if e != nil {
		return ErrPersistence
	}
	if header != "" {
		req.Header.Set(header, value)
	}
	resp, e := client.Do(req)
	if e != nil {
		return ErrPersistence
	}
	defer resp.Body.Close()
	body, e := io.ReadAll(io.LimitReader(resp.Body, 4097))
	if e != nil || len(body) > 4096 || resp.StatusCode != want {
		return ErrPersistence
	}
	if path == "/healthz" && !bytes.Equal(body, []byte(`{"status":"ok"}`)) {
		return ErrPersistence
	}
	return nil
}
