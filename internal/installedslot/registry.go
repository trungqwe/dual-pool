//go:build windows

// Package installedslot owns the closed, product-root-relative mapping from a
// logical upstream version to one immutable, verified executable slot.
package installedslot

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
	"unsafe"

	"github.com/trungqwe/dual-pool/internal/dataroot"
	"github.com/trungqwe/dual-pool/internal/lockfile"
	"github.com/trungqwe/dual-pool/internal/state"
	"github.com/trungqwe/dual-pool/internal/upstreamlock"
	"github.com/trungqwe/dual-pool/internal/upstreamstage"
	"golang.org/x/sys/windows"
)

const (
	registrySchemaVersion = 1
	manifestName          = "install-manifest.json"
	registryName          = "installed-slots.json"
	maxRegistryBytes      = 1 << 20
)

var (
	ErrInvalidVersion  = errors.New("installed slot logical version is invalid")
	ErrRegistryMissing = errors.New("installed slot registry is missing")
	ErrRegistryCorrupt = errors.New("installed slot registry is corrupt")
	ErrSlotUnknown     = errors.New("installed slot is not registered")
	ErrSlotConflict    = errors.New("installed slot identity conflicts with registration")
	ErrUnsafeSlot      = errors.New("installed slot is unsafe")
	ErrUnsupported     = errors.New("installed slot metadata is unsupported")
	ErrPersistence     = errors.New("installed slot registry persistence failed")
	digestPattern      = regexp.MustCompile(`^[a-f0-9]{64}$`)
	commitPattern      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+\-]{5,127}$`)
)

// ACL is the minimum protected-object contract used by the registry. The
// production implementation is internal/winacl; tests inject a TEMP fixture.
type ACL interface {
	Create(string) error
	Inspect(string) error
	CreateFile(string) (*os.File, error)
	InspectFile(string) error
}

// Manifest is the immutable metadata written beside an installed executable.
// It is intentionally shared by instance.Manager and this package.
type Manifest struct {
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

type entry struct {
	Version              string `json:"version"`
	SlotDirectory        string `json:"slot_directory"`
	ExecutableBasename   string `json:"executable_basename"`
	ExecutableSHA256     string `json:"executable_sha256"`
	ManifestSHA256       string `json:"manifest_sha256"`
	Tag                  string `json:"tag"`
	Commit               string `json:"commit"`
	Platform             string `json:"platform"`
	UpstreamLockSHA256   string `json:"upstream_lock_sha256"`
	ConfigAdapterVersion string `json:"config_adapter_version"`
}

type document struct {
	SchemaVersion int     `json:"schema_version"`
	Slots         []entry `json:"slots"`
}

// ResolvedSlot exposes a path only after the registry has validated the
// closed metadata, ACL, reparse state, manifest and executable hash.
type ResolvedSlot struct {
	Version              string
	SlotDirectory        string
	ExecutablePath       string
	ExecutableBasename   string
	ExecutableSHA256     string
	ManifestSHA256       string
	Tag                  string
	Commit               string
	Platform             string
	UpstreamLockSHA256   string
	ConfigAdapterVersion string
}

type BinaryVerifier func(context.Context, string, Manifest) error

type FaultPoint string

const (
	AfterCandidateCreate         FaultPoint = "AFTER_CANDIDATE_CREATE"
	AfterCandidateWrite          FaultPoint = "AFTER_CANDIDATE_WRITE"
	AfterCandidateSync           FaultPoint = "AFTER_CANDIDATE_SYNC"
	AfterPublication             FaultPoint = "AFTER_PUBLICATION"
	AfterPublicationVerification FaultPoint = "AFTER_PUBLICATION_VERIFICATION"
)

type Option func(*Registry)

func WithBinaryVerifier(v BinaryVerifier) Option        { return func(r *Registry) { r.binaryVerifier = v } }
func WithFaultInjector(v func(FaultPoint) error) Option { return func(r *Registry) { r.fault = v } }

type Registry struct {
	layout           dataroot.Layout
	acl              ACL
	locks            *lockfile.Manager
	expectedPlatform string
	expectedProduct  string
	expectedAdapter  string
	expectedVersion  string
	expectedTag      string
	expectedCommit   string
	expectedLockHash string
	binaryVerifier   BinaryVerifier
	fault            func(FaultPoint) error
}

// New constructs the production registry policy without creating product
// directories. The pinned upstream lock remains authoritative.
func New(layout dataroot.Layout, acl ACL, lock upstreamlock.Lock, options ...Option) (*Registry, error) {
	if acl == nil || lock.Validate() != nil {
		return nil, ErrUnsupported
	}
	locks, err := lockfile.NewManager(layout.Locks)
	if err != nil {
		return nil, ErrPersistence
	}
	r := &Registry{layout: layout, acl: acl, locks: locks, expectedPlatform: "windows_amd64", expectedProduct: lock.Product, expectedAdapter: lock.ConfigAdapterVersion, expectedVersion: lock.Version, expectedTag: lock.Tag, expectedCommit: lock.Commit, expectedLockHash: lock.Digest(), binaryVerifier: func(ctx context.Context, path string, _ Manifest) error {
		identity, err := (upstreamstage.WindowsVerifier{}).Verify(ctx, path, lock)
		if err != nil || !identity.VersionMatch || !identity.CommitMatch {
			return ErrUnsafeSlot
		}
		return nil
	}}
	for _, option := range options {
		option(r)
	}
	if r.binaryVerifier == nil {
		return nil, ErrUnsupported
	}
	return r, nil
}

func (r *Registry) registryPath() string {
	return filepath.Join(r.layout.Bin, "cliproxyapi", registryName)
}
func (r *Registry) slotRoot() string { return filepath.Join(r.layout.Bin, "cliproxyapi") }
func (r *Registry) slotPath(version string) string {
	return filepath.Join(r.slotRoot(), version)
}

func (r *Registry) Resolve(version string) (ResolvedSlot, error) {
	return r.ResolveContext(context.Background(), version)
}

func (r *Registry) ResolveContext(ctx context.Context, version string) (ResolvedSlot, error) {
	if !validVersion(version) {
		return ResolvedSlot{}, ErrInvalidVersion
	}
	doc, err := r.loadDocument()
	if err != nil {
		return ResolvedSlot{}, err
	}
	for _, item := range doc.Slots {
		if item.Version != version {
			continue
		}
		return r.validateEntry(ctx, item)
	}
	return ResolvedSlot{}, ErrSlotUnknown
}

func (r *Registry) VerifyInstalled(ctx context.Context, version string) error {
	_, err := r.ResolveContext(ctx, version)
	return err
}

// Register derives the slot path exclusively from the validated version and
// serializes the immutable binding under GLOBAL.
func (r *Registry) Register(ctx context.Context, version string) error {
	if r.locks == nil {
		return ErrPersistence
	}
	guard, err := r.locks.AcquireGlobal()
	if err != nil {
		return ErrPersistence
	}
	defer guard.Release()
	return r.RegisterLocked(ctx, version)
}

// RegisterLocked is used by Manager.Install while it already owns GLOBAL.
// Callers must hold the registry's GLOBAL lock.
func (r *Registry) RegisterLocked(ctx context.Context, version string) error {
	if !validVersion(version) {
		return ErrInvalidVersion
	}
	resolved, err := r.validateDirectory(ctx, version, r.slotPath(version), nil)
	if err != nil {
		return err
	}
	item := entryFromResolved(resolved)
	doc, err := r.loadDocument()
	if errors.Is(err, ErrRegistryMissing) {
		doc = document{SchemaVersion: registrySchemaVersion}
	} else if err != nil {
		return err
	}
	for _, old := range doc.Slots {
		if old.Version != version {
			continue
		}
		if old == item {
			return nil
		}
		return ErrSlotConflict
	}
	doc.Slots = append(doc.Slots, item)
	return r.writeDocument(doc)
}

func entryFromResolved(v ResolvedSlot) entry {
	return entry{Version: v.Version, SlotDirectory: filepath.Base(v.SlotDirectory), ExecutableBasename: v.ExecutableBasename, ExecutableSHA256: v.ExecutableSHA256, ManifestSHA256: v.ManifestSHA256, Tag: v.Tag, Commit: v.Commit, Platform: v.Platform, UpstreamLockSHA256: v.UpstreamLockSHA256, ConfigAdapterVersion: v.ConfigAdapterVersion}
}

func (r *Registry) validateEntry(ctx context.Context, item entry) (ResolvedSlot, error) {
	if !validVersion(item.Version) || item.SlotDirectory != item.Version || item.ExecutableBasename != "cliproxyapi.exe" || !digestPattern.MatchString(item.ExecutableSHA256) || !digestPattern.MatchString(item.ManifestSHA256) || !validMetadata(item.Tag, item.Commit, item.Platform, item.UpstreamLockSHA256, item.ConfigAdapterVersion) {
		return ResolvedSlot{}, ErrRegistryCorrupt
	}
	if item.Version != filepath.Base(r.slotPath(item.Version)) || !samePath(r.slotPath(item.Version), filepath.Join(r.slotRoot(), item.SlotDirectory)) {
		return ResolvedSlot{}, ErrRegistryCorrupt
	}
	return r.validateDirectory(ctx, item.Version, r.slotPath(item.Version), &item)
}

func (r *Registry) validateDirectory(ctx context.Context, version, dir string, binding *entry) (ResolvedSlot, error) {
	if !validVersion(version) || !samePath(dir, r.slotPath(version)) || !safeDirectory(r.layout.Bin) || !safeDirectory(r.slotRoot()) || !safeDirectory(dir) || r.acl.Inspect(dir) != nil {
		return ResolvedSlot{}, ErrUnsafeSlot
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		return ResolvedSlot{}, ErrUnsafeSlot
	}
	for _, item := range entries {
		if item.Name() != "cliproxyapi.exe" && item.Name() != manifestName {
			return ResolvedSlot{}, ErrUnsafeSlot
		}
		path := filepath.Join(dir, item.Name())
		if item.IsDir() || item.Type()&os.ModeSymlink != 0 || isReparsePoint(path) || r.acl.InspectFile(path) != nil {
			return ResolvedSlot{}, ErrUnsafeSlot
		}
	}
	manifestPath := filepath.Join(dir, manifestName)
	executablePath := filepath.Join(dir, "cliproxyapi.exe")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil || len(manifestBytes) > 64*1024 {
		return ResolvedSlot{}, ErrUnsafeSlot
	}
	manifest, err := decodeManifest(manifestBytes)
	if err != nil {
		return ResolvedSlot{}, ErrUnsafeSlot
	}
	if err := r.validateManifest(manifest, version); err != nil {
		return ResolvedSlot{}, ErrUnsafeSlot
	}
	info, err := os.Stat(executablePath)
	if err != nil || !info.Mode().IsRegular() || isReparsePoint(executablePath) {
		return ResolvedSlot{}, ErrUnsafeSlot
	}
	exeHash, err := hashFile(executablePath)
	if err != nil || exeHash != manifest.ExecutableSHA256 {
		return ResolvedSlot{}, ErrSlotConflict
	}
	manifestHash := hashBytes(manifestBytes)
	resolved := ResolvedSlot{Version: version, SlotDirectory: dir, ExecutablePath: executablePath, ExecutableBasename: manifest.ExecutableBasename, ExecutableSHA256: manifest.ExecutableSHA256, ManifestSHA256: manifestHash, Tag: manifest.Tag, Commit: manifest.Commit, Platform: manifest.Platform, UpstreamLockSHA256: manifest.UpstreamLockSHA256, ConfigAdapterVersion: manifest.ConfigAdapterVersion}
	if binding != nil && entryFromResolved(resolved) != *binding {
		return ResolvedSlot{}, ErrSlotConflict
	}
	if r.binaryVerifier != nil {
		if err := r.binaryVerifier(ctx, executablePath, manifest); err != nil {
			return ResolvedSlot{}, ErrUnsafeSlot
		}
	}
	return resolved, nil
}

func (r *Registry) validateManifest(m Manifest, version string) error {
	if m.SchemaVersion != 1 || m.Product != r.expectedProduct || m.Version != version || m.ExecutableBasename != "cliproxyapi.exe" || m.Platform != r.expectedPlatform || m.ConfigAdapterVersion != r.expectedAdapter || !digestPattern.MatchString(m.ExecutableSHA256) || m.Tag == "" || m.Commit == "" || m.UpstreamLockSHA256 == "" || !commitPattern.MatchString(m.Commit) || m.Tag != r.expectedTag && r.expectedTag != "" || m.Commit != r.expectedCommit && r.expectedCommit != "" || m.UpstreamLockSHA256 != r.expectedLockHash && r.expectedLockHash != "" {
		return ErrUnsupported
	}
	return nil
}

func validMetadata(tag, commit, platform, lockHash, adapter string) bool {
	return tag != "" && commitPattern.MatchString(commit) && platform == "windows_amd64" && lockHash != "" && adapter != ""
}

func validVersion(value string) bool {
	return state.ValidLogicalVersion(value)
}

// ValidVersion is the single closed logical-version validator shared by
// state-bound process records and the installed-slot registry.
func ValidVersion(value string) bool { return validVersion(value) }

func decodeManifest(data []byte) (Manifest, error) {
	var out Manifest
	if len(data) == 0 || !utf8.Valid(data) || duplicateKeys(data) {
		return out, ErrUnsafeSlot
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&out); err != nil {
		return Manifest{}, ErrUnsafeSlot
	}
	var extra any
	if !errors.Is(d.Decode(&extra), io.EOF) {
		return Manifest{}, ErrUnsafeSlot
	}
	return out, nil
}

func encodeManifest(m Manifest) ([]byte, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func (r *Registry) loadDocument() (document, error) {
	path := r.registryPath()
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return document{}, ErrRegistryMissing
	}
	if err != nil || !info.Mode().IsRegular() || isReparsePoint(path) || r.acl.InspectFile(path) != nil {
		return document{}, ErrRegistryCorrupt
	}
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 || len(b) > maxRegistryBytes || !utf8.Valid(b) || duplicateKeys(b) {
		return document{}, ErrRegistryCorrupt
	}
	var out document
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&out); err != nil {
		return document{}, ErrRegistryCorrupt
	}
	var extra any
	if !errors.Is(d.Decode(&extra), io.EOF) || out.SchemaVersion != registrySchemaVersion {
		return document{}, ErrRegistryCorrupt
	}
	seen := map[string]bool{}
	for _, item := range out.Slots {
		if seen[item.Version] {
			return document{}, ErrRegistryCorrupt
		}
		seen[item.Version] = true
	}
	return out, nil
}

func (r *Registry) writeDocument(doc document) error {
	if doc.SchemaVersion != registrySchemaVersion {
		return ErrRegistryCorrupt
	}
	if r.acl.Inspect(r.slotRoot()) != nil {
		return ErrUnsafeSlot
	}
	id := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, id); err != nil {
		return ErrPersistence
	}
	txn := hex.EncodeToString(id)
	candidate := filepath.Join(filepath.Dir(r.registryPath()), "."+registryName+".tmp-"+txn)
	data, err := json.Marshal(doc)
	if err != nil {
		return ErrPersistence
	}
	data = append(data, '\n')
	f, err := r.acl.CreateFile(candidate)
	if err != nil {
		return ErrPersistence
	}
	if err = r.inject(AfterCandidateCreate); err != nil {
		_ = f.Close()
		_ = os.Remove(candidate)
		return err
	}
	if _, err = f.Write(data); err == nil {
		err = r.inject(AfterCandidateWrite)
	}
	if err == nil {
		err = f.Sync()
	}
	if err == nil {
		err = r.inject(AfterCandidateSync)
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(candidate)
		return ErrPersistence
	}
	if _, err = decodeDocument(data); err != nil || r.acl.InspectFile(candidate) != nil {
		_ = os.Remove(candidate)
		return ErrPersistence
	}
	if _, err = os.Lstat(r.registryPath()); os.IsNotExist(err) {
		from, _ := windows.UTF16PtrFromString(candidate)
		to, _ := windows.UTF16PtrFromString(r.registryPath())
		if err = windows.MoveFileEx(from, to, windows.MOVEFILE_WRITE_THROUGH); err != nil {
			_ = os.Remove(candidate)
			return ErrPersistence
		}
	} else if err == nil {
		if err = replaceExisting(r.registryPath(), candidate); err != nil {
			_ = os.Remove(candidate)
			return ErrPersistence
		}
	} else {
		_ = os.Remove(candidate)
		return ErrPersistence
	}
	if r.acl.InspectFile(r.registryPath()) != nil {
		return ErrPersistence
	}
	if err := r.inject(AfterPublication); err != nil {
		return err
	}
	if _, err := r.loadDocument(); err != nil {
		return ErrPersistence
	}
	if err := r.inject(AfterPublicationVerification); err != nil {
		return err
	}
	return nil
}

func (r *Registry) inject(point FaultPoint) error {
	if r.fault == nil {
		return nil
	}
	if err := r.fault(point); err != nil {
		return err
	}
	return nil
}

func decodeDocument(data []byte) (document, error) {
	var out document
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&out); err != nil {
		return document{}, err
	}
	var extra any
	if !errors.Is(d.Decode(&extra), io.EOF) || out.SchemaVersion != registrySchemaVersion {
		return document{}, ErrRegistryCorrupt
	}
	seen := map[string]bool{}
	for _, item := range out.Slots {
		if seen[item.Version] {
			return document{}, ErrRegistryCorrupt
		}
		seen[item.Version] = true
	}
	return out, nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func hashBytes(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
func samePath(a, b string) bool {
	x, e := filepath.Abs(a)
	if e != nil {
		return false
	}
	y, e := filepath.Abs(b)
	return e == nil && strings.EqualFold(filepath.Clean(x), filepath.Clean(y))
}
func safeDirectory(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 && !isReparsePoint(path)
}

func isReparsePoint(path string) bool {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return true
	}
	attrs, err := windows.GetFileAttributes(p)
	return err != nil || attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
}

func duplicateKeys(data []byte) bool {
	d := json.NewDecoder(bytes.NewReader(data))
	objects := []map[string]bool{}
	expects := []bool{}
	for {
		t, err := d.Token()
		if errors.Is(err, io.EOF) {
			return false
		}
		if err != nil {
			return true
		}
		switch x := t.(type) {
		case json.Delim:
			switch x {
			case '{':
				objects = append(objects, map[string]bool{})
				expects = append(expects, true)
			case '[':
				objects = append(objects, nil)
				expects = append(expects, false)
			case '}', ']':
				objects = objects[:len(objects)-1]
				expects = expects[:len(expects)-1]
				if len(objects) > 0 && objects[len(objects)-1] != nil {
					expects[len(expects)-1] = true
				}
			}
		case string:
			if len(objects) > 0 && objects[len(objects)-1] != nil && expects[len(expects)-1] {
				if objects[len(objects)-1][x] {
					return true
				}
				objects[len(objects)-1][x] = true
				expects[len(expects)-1] = false
			} else if len(objects) > 0 && objects[len(objects)-1] != nil {
				expects[len(expects)-1] = true
			}
		default:
			if len(objects) > 0 && objects[len(objects)-1] != nil {
				expects[len(expects)-1] = true
			}
		}
	}
}

var replaceFileW = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReplaceFileW")

func replaceExisting(target, candidate string) error {
	t, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	c, err := windows.UTF16PtrFromString(candidate)
	if err != nil {
		return err
	}
	r, _, callErr := replaceFileW.Call(uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)), 0, 0, 0, 0)
	if r == 0 {
		if callErr != nil {
			return callErr
		}
		return fmt.Errorf("ReplaceFileW failed")
	}
	return nil
}
