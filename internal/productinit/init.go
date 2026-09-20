package productinit

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"io"
	"os"

	"github.com/trungqwe/dual-pool/internal/dataroot"
	"github.com/trungqwe/dual-pool/internal/lockfile"
	"github.com/trungqwe/dual-pool/internal/secretstore"
	"github.com/trungqwe/dual-pool/internal/winacl"
)

var (
	ErrConflict    = errors.New("product initialization conflict")
	ErrInvalidKeys = errors.New("invalid product keys")
)

var purposes = []secretstore.Purpose{secretstore.CodexClientKey, secretstore.CodexManagementKey, secretstore.GoogleClientKey, secretstore.GoogleManagementKey}

type ACL interface {
	Create(string) error
	Inspect(string) error
}
type Result struct {
	CreatedDirectories, CreatedKeys int
	Ready                           bool
}
type Inspection struct {
	RootExists       bool
	DirectoriesReady int
	KeysPresent      int
	Ready            bool
}
type Initializer struct {
	layout dataroot.Layout
	acl    ACL
	store  secretstore.Store
	random io.Reader
}

func NewCurrent() (*Initializer, error) {
	l, err := dataroot.ResolveCurrent()
	if err != nil {
		return nil, err
	}
	a, err := winacl.New()
	if err != nil {
		return nil, err
	}
	return New(l, a, secretstore.New(), rand.Reader), nil
}

// InspectCurrent reads the real product layout and exact four credentials without mutation.
func InspectCurrent() (Inspection, error) {
	i, err := NewCurrent()
	if err != nil {
		return Inspection{}, err
	}
	return i.Inspect()
}

func (i *Initializer) Inspect() (Inspection, error) {
	keys, err := i.readKeys()
	if err != nil {
		return Inspection{}, err
	}
	defer zeroMap(keys)
	state := Inspection{KeysPresent: len(keys)}
	_, err = os.Lstat(i.layout.Root)
	if errors.Is(err, os.ErrNotExist) {
		if state.KeysPresent != 0 {
			return Inspection{}, ErrConflict
		}
		return state, nil
	}
	if err != nil {
		return Inspection{}, ErrConflict
	}
	state.RootExists = true
	if err = i.inspectTree(); err != nil {
		return Inspection{}, err
	}
	for _, dir := range i.directories() {
		_, err = os.Lstat(dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || i.acl.Inspect(dir) != nil {
			return Inspection{}, ErrConflict
		}
		if dir != i.layout.Root {
			state.DirectoriesReady++
		}
	}
	state.Ready = state.DirectoriesReady == 7 && state.KeysPresent == 4
	return state, nil
}

func New(l dataroot.Layout, a ACL, s secretstore.Store, r io.Reader) *Initializer {
	return &Initializer{l, a, s, r}
}

func (i *Initializer) Initialize() (Result, error) {
	present, err := i.readKeys()
	if err != nil {
		return Result{}, err
	}
	rootExists := exists(i.layout.Root)
	if !rootExists && len(present) != 0 {
		zeroMap(present)
		return Result{}, ErrConflict
	}
	zeroMap(present)
	if rootExists {
		if err := i.inspectTree(); err != nil {
			return Result{}, err
		}
	}
	result := Result{}
	for _, dir := range i.directories() {
		if exists(dir) {
			if err := i.acl.Inspect(dir); err != nil {
				return Result{}, err
			}
			continue
		}
		if err := i.acl.Create(dir); err != nil {
			return Result{}, err
		}
		result.CreatedDirectories++
	}
	if err := i.inspectTree(); err != nil {
		return Result{}, err
	}
	manager, err := lockfile.NewManager(i.layout.Locks)
	if err != nil {
		return Result{}, err
	}
	guard, err := manager.AcquireGlobal()
	if err != nil {
		return Result{}, err
	}
	released := false
	defer func() {
		if !released {
			_ = guard.Release()
		}
	}()
	for _, dir := range i.directories() {
		if err := i.acl.Inspect(dir); err != nil {
			return Result{}, err
		}
	}
	present, err = i.readKeys()
	if err != nil {
		return Result{}, err
	}
	defer zeroMap(present)
	for _, purpose := range purposes {
		if _, ok := present[purpose]; ok {
			continue
		}
		key, err := i.uniqueKey(present)
		if err != nil {
			return Result{}, err
		}
		if err = i.store.Put(purpose, key); err != nil {
			secretstore.Zero(key)
			return Result{}, err
		}
		present[purpose] = key
		result.CreatedKeys++
	}
	verified, err := i.readKeys()
	if err != nil {
		return Result{}, err
	}
	defer zeroMap(verified)
	if len(verified) != len(purposes) {
		return Result{}, ErrInvalidKeys
	}
	for _, purpose := range purposes {
		if subtle.ConstantTimeCompare(present[purpose], verified[purpose]) != 1 {
			return Result{}, ErrInvalidKeys
		}
	}
	if err := guard.Release(); err != nil {
		return Result{}, err
	}
	released = true
	result.Ready = true
	return result, nil
}

func (i *Initializer) inspectTree() error {
	entries, err := os.ReadDir(i.layout.Root)
	if err != nil {
		return err
	}
	allowed := map[string]bool{}
	for _, path := range i.directories()[1:] {
		allowed[path] = true
	}
	for _, entry := range entries {
		path := i.layout.Root + string(os.PathSeparator) + entry.Name()
		if !entry.IsDir() || !allowed[path] {
			return ErrConflict
		}
	}
	return nil
}

func (i *Initializer) readKeys() (map[secretstore.Purpose][]byte, error) {
	values := map[secretstore.Purpose][]byte{}
	for _, p := range purposes {
		v, err := i.store.Get(p)
		if errors.Is(err, secretstore.ErrNotFound) {
			continue
		}
		if err != nil {
			zeroMap(values)
			return nil, err
		}
		if len(v) != 32 || contains(values, v) {
			secretstore.Zero(v)
			zeroMap(values)
			return nil, ErrInvalidKeys
		}
		values[p] = v
	}
	return values, nil
}

func (i *Initializer) uniqueKey(existing map[secretstore.Purpose][]byte) ([]byte, error) {
	for attempt := 0; attempt < 8; attempt++ {
		v := make([]byte, 32)
		if _, err := io.ReadFull(i.random, v); err != nil {
			secretstore.Zero(v)
			return nil, err
		}
		if !contains(existing, v) {
			return v, nil
		}
		secretstore.Zero(v)
	}
	return nil, ErrInvalidKeys
}

func (i *Initializer) directories() []string {
	return []string{i.layout.Root, i.layout.Bin, i.layout.Instances, i.layout.Config, i.layout.State, i.layout.Backups, i.layout.Evidence, i.layout.Locks}
}
func exists(path string) bool { _, err := os.Lstat(path); return err == nil }
func contains(values map[secretstore.Purpose][]byte, v []byte) bool {
	for _, x := range values {
		if len(x) == len(v) && subtle.ConstantTimeCompare(x, v) == 1 {
			return true
		}
	}
	return false
}
func zeroMap(values map[secretstore.Purpose][]byte) {
	for _, v := range values {
		secretstore.Zero(v)
	}
}
