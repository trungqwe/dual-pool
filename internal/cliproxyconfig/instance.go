package cliproxyconfig

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"

	"github.com/trungqwe/dual-pool/internal/dataroot"
	"github.com/trungqwe/dual-pool/internal/keymaterial"
	"github.com/trungqwe/dual-pool/internal/lockfile"
	"github.com/trungqwe/dual-pool/internal/productinit"
	"github.com/trungqwe/dual-pool/internal/secretstore"
	"github.com/trungqwe/dual-pool/internal/upstreamlock"
	"github.com/trungqwe/dual-pool/internal/winacl"
	"golang.org/x/sys/windows"
)

type ACL interface {
	Create(string) error
	Inspect(string) error
	CreateFile(string) (*os.File, error)
	InspectFile(string) error
}
type SecretReader interface {
	Get(secretstore.Purpose) ([]byte, error)
}
type Fault string

const (
	AfterMarkerSync        Fault = "AFTER_MARKER_SYNC"
	AfterAttemptDirCreate  Fault = "AFTER_ATTEMPT_DIR_CREATE"
	AfterConfigSync        Fault = "AFTER_CONFIG_SYNC"
	AfterAuthDirCreate     Fault = "AFTER_AUTH_DIR_CREATE"
	AfterLogsDirCreate     Fault = "AFTER_LOGS_DIR_CREATE"
	AfterAttemptVerify     Fault = "AFTER_ATTEMPT_VERIFY"
	BeforeInstanceInstall  Fault = "BEFORE_INSTANCE_INSTALL"
	AfterInstanceInstall   Fault = "AFTER_INSTANCE_INSTALL"
	AfterFinalVerify       Fault = "AFTER_FINAL_VERIFY"
	BeforeMarkerCleanup    Fault = "BEFORE_MARKER_CLEANUP"
	AfterCodexBeforeGoogle Fault = "AFTER_CODEX_BEFORE_GOOGLE"
)

type Result struct {
	Created          int
	Reused           int
	Recovered        int
	ConfigsRewritten int
	Ready            bool
}
type Inspection struct {
	Present int
	Ready   bool
}
type Generator struct {
	layout dataroot.Layout
	acl    ACL
	reader SecretReader
	lock   upstreamlock.Lock
	fault  func(Fault) error
}

// NewCurrent checks the real product initialization gates without mutation.
func NewCurrent(lock upstreamlock.Lock) (*Generator, error) {
	state, err := productinit.InspectCurrent()
	if err != nil || !state.Ready || state.DirectoriesReady != 7 || state.KeysPresent != 4 {
		return nil, ErrUnsafeInstanceArtifact
	}
	l, err := dataroot.ResolveCurrent()
	if err != nil {
		return nil, ErrUnsafeInstanceArtifact
	}
	a, err := winacl.New()
	if err != nil {
		return nil, ErrUnsafeInstanceArtifact
	}
	return New(l, a, secretstore.New(), lock)
}

func New(l dataroot.Layout, a ACL, reader SecretReader, lock upstreamlock.Lock) (*Generator, error) {
	if lock.Validate() != nil || lock.Version != UpstreamVersion || lock.Commit != UpstreamCommit || lock.ConfigAdapterVersion != AdapterVersion {
		return nil, ErrUnsupportedAdapter
	}
	if a == nil || reader == nil {
		return nil, ErrUnsafeInstanceArtifact
	}
	return &Generator{layout: l, acl: a, reader: reader, lock: lock}, nil
}
func (g *Generator) WithFault(f func(Fault) error) *Generator { g.fault = f; return g }
func (g *Generator) hit(point Fault) error {
	if g.fault != nil {
		return g.fault(point)
	}
	return nil
}

type material struct {
	raw  [4][]byte
	wire [4][]byte
}

func (m *material) wipe() {
	for i := range m.raw {
		secretstore.Zero(m.raw[i])
		secretstore.Zero(m.wire[i])
	}
}
func (g *Generator) keys() (material, error) {
	var m material
	for i, p := range []secretstore.Purpose{secretstore.CodexClientKey, secretstore.CodexManagementKey, secretstore.GoogleClientKey, secretstore.GoogleManagementKey} {
		v, err := g.reader.Get(p)
		if err != nil || len(v) != 32 {
			secretstore.Zero(v)
			m.wipe()
			return material{}, ErrInvalidKeyMaterial
		}
		for j := 0; j < i; j++ {
			if subtle.ConstantTimeCompare(v, m.raw[j]) == 1 {
				secretstore.Zero(v)
				m.wipe()
				return material{}, ErrInvalidKeyMaterial
			}
		}
		m.raw[i] = v
		m.wire[i], err = keymaterial.Encode(v)
		if err != nil {
			m.wipe()
			return material{}, ErrInvalidKeyMaterial
		}
	}
	return m, nil
}

// InspectPair performs a read-only preflight. It never recovers or creates artifacts.
func (g *Generator) InspectPair() (Inspection, error) {
	if g.lock.Validate() != nil || g.lock.ConfigAdapterVersion != AdapterVersion {
		return Inspection{}, ErrUnsupportedAdapter
	}
	for _, dir := range []string{g.layout.Root, g.layout.Instances, g.layout.Locks} {
		if g.acl.Inspect(dir) != nil {
			return Inspection{}, ErrUnsafeInstanceArtifact
		}
	}
	m, err := g.keys()
	if err != nil {
		return Inspection{}, err
	}
	defer m.wipe()
	if err = g.inspectInstancesTop(); err != nil {
		return Inspection{}, err
	}
	state := Inspection{}
	for _, id := range []ID{Codex, Google} {
		if exists(g.markerPath(id)) {
			return Inspection{}, ErrRecoveryUnresolved
		}
		if exists(g.final(id)) {
			if g.inspectInstance(g.final(id), id, &m, false) != nil {
				return Inspection{}, ErrConfigConflict
			}
			state.Present++
		}
	}
	state.Ready = state.Present == 2
	return state, nil
}

func (g *Generator) GeneratePair() (Result, error) {
	if g.lock.Validate() != nil || g.lock.ConfigAdapterVersion != AdapterVersion {
		return Result{}, ErrUnsupportedAdapter
	}
	for _, dir := range []string{g.layout.Root, g.layout.Instances, g.layout.Locks} {
		if g.acl.Inspect(dir) != nil {
			return Result{}, ErrUnsafeInstanceArtifact
		}
	}
	m, err := g.keys()
	if err != nil {
		return Result{}, err
	}
	defer m.wipe()
	locks, err := lockfile.NewManager(g.layout.Locks)
	if err != nil {
		return Result{}, ErrPersistence
	}
	guard, err := locks.AcquireGlobal()
	if err != nil {
		return Result{}, ErrPersistence
	}
	released := false
	defer func() {
		if !released {
			_ = guard.Release()
		}
	}()
	for _, dir := range []string{g.layout.Root, g.layout.Instances, g.layout.Locks} {
		if g.acl.Inspect(dir) != nil {
			return Result{}, ErrUnsafeInstanceArtifact
		}
	}
	current, err := g.keys()
	if err != nil {
		return Result{}, err
	}
	defer current.wipe()
	for i := range m.raw {
		if !equal(m.raw[i], current.raw[i]) {
			return Result{}, ErrInvalidKeyMaterial
		}
	}
	if err = g.inspectInstancesTop(); err != nil {
		return Result{}, err
	}
	result := Result{}
	for _, id := range []ID{Codex, Google} {
		recovered, err := g.recover(id, &m)
		if err != nil {
			return Result{}, err
		}
		if recovered {
			result.Recovered++
		}
		final := g.final(id)
		if exists(final) {
			if g.inspectInstance(final, id, &m, false) != nil {
				return Result{}, ErrConfigConflict
			}
			result.Reused++
		} else {
			if err = g.create(id, &m); err != nil {
				return Result{}, err
			}
			result.Created++
		}
		if id == Codex {
			if err = g.hit(AfterCodexBeforeGoogle); err != nil {
				return Result{}, err
			}
		}
	}
	for _, id := range []ID{Codex, Google} {
		if g.inspectInstance(g.final(id), id, &m, false) != nil {
			return Result{}, ErrConfigConflict
		}
	}
	if err = guard.Release(); err != nil {
		return Result{}, ErrPersistence
	}
	released = true
	result.Ready = true
	return result, nil
}

func (g *Generator) final(id ID) string { return filepath.Join(g.layout.Instances, string(id)) }
func (g *Generator) auth(id ID) string  { return filepath.Join(g.final(id), "auth") }
func (g *Generator) markerPath(id ID) string {
	return filepath.Join(g.layout.Instances, "."+string(id)+".init-marker.json")
}
func exists(path string) bool { _, err := os.Lstat(path); return err == nil }

func (g *Generator) inspectInstance(root string, id ID, m *material, candidate bool) error {
	if !id.valid() || g.acl.Inspect(root) != nil || g.acl.Inspect(filepath.Join(root, "auth")) != nil || g.acl.Inspect(filepath.Join(root, "logs")) != nil || g.acl.InspectFile(filepath.Join(root, "config.yaml")) != nil {
		return ErrUnsafeInstanceArtifact
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) < 3 || len(entries) > 4 {
		return ErrUnsafeInstanceArtifact
	}
	for _, e := range entries {
		switch e.Name() {
		case "config.yaml":
			if e.IsDir() {
				return ErrUnsafeInstanceArtifact
			}
		case "auth", "logs":
			if !e.IsDir() {
				return ErrUnsafeInstanceArtifact
			}
		case "process.json":
			if e.IsDir() || g.acl.InspectFile(filepath.Join(root, e.Name())) != nil {
				return ErrUnsafeInstanceArtifact
			}
		default:
			return ErrUnsafeInstanceArtifact
		}
	}
	if candidate {
		for _, name := range []string{"auth", "logs"} {
			children, e := os.ReadDir(filepath.Join(root, name))
			if e != nil || len(children) != 0 {
				return ErrUnsafeInstanceArtifact
			}
		}
	}
	b, err := os.ReadFile(filepath.Join(root, "config.yaml"))
	if err != nil {
		return ErrConfigInvalid
	}
	defer secretstore.Zero(b)
	for _, raw := range m.raw {
		if bytes.Contains(b, raw) {
			return ErrConfigInvalid
		}
	}
	base := 0
	if id == Google {
		base = 2
	}
	var other [][]byte
	// Reject opposite-pool secrets; the own management plaintext is checked separately.
	if id == Codex {
		other = [][]byte{m.wire[2], m.wire[3]}
	} else {
		other = [][]byte{m.wire[0], m.wire[1]}
	}
	if validateConfig(b, id, g.auth(id), m.wire[base], m.wire[base+1], other) != nil {
		return ErrConfigInvalid
	}
	return nil
}

func moveNoReplace(from, to string) error {
	a, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return ErrPersistence
	}
	b, err := windows.UTF16PtrFromString(to)
	if err != nil {
		return ErrPersistence
	}
	if windows.MoveFileEx(a, b, windows.MOVEFILE_WRITE_THROUGH) != nil {
		return ErrConfigConflict
	}
	return nil
}

func randomTxn() (string, error) {
	b := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", ErrPersistence
	}
	return hex.EncodeToString(b), nil
}

func validateInstanceID(id ID) error {
	if !id.valid() {
		return ErrConfigConflict
	}
	return nil
}
