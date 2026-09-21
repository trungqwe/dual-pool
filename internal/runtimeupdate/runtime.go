// Package runtimeupdate composes the trusted update transaction graph.
package runtimeupdate

import (
	"errors"
	"path/filepath"
	"reflect"

	"github.com/trungqwe/dual-pool/internal/dataroot"
	"github.com/trungqwe/dual-pool/internal/installedslot"
	"github.com/trungqwe/dual-pool/internal/instance"
	"github.com/trungqwe/dual-pool/internal/lockfile"
	"github.com/trungqwe/dual-pool/internal/state"
	"github.com/trungqwe/dual-pool/internal/update"
	"github.com/trungqwe/dual-pool/internal/upstreamlock"
)

var ErrCompositionInvalid = errors.New("runtime update composition is invalid")

type ACL interface {
	installedslot.ACL
	instance.ACL
}

type Config struct {
	Layout dataroot.Layout
	Lock   upstreamlock.Lock
	ACL    ACL
	Smoke  update.Smoke
	// MarkerSecurity is test-only injection; production callers leave it nil.
	MarkerSecurity update.MarkerSecurity
}

// Runtime exposes the concrete objects for the future command boundary while
// retaining a single Store and Registry as the transaction trust sources.
type Runtime struct {
	Updater   *update.Updater
	Manager   *instance.Manager
	Registry  *installedslot.Registry
	State     *state.Store
	Lifecycle *instance.UpdaterLifecycle
}

func New(c Config) (*Runtime, error) {
	if isNil(c.ACL) || isNil(c.Smoke) || c.Lock.Validate() != nil || c.Layout.Root == "" || c.Layout.State == "" || c.Layout.Locks == "" {
		return nil, ErrCompositionInvalid
	}
	for _, dir := range []string{c.Layout.Root, c.Layout.Bin, c.Layout.Instances, c.Layout.Config, c.Layout.State, c.Layout.Backups, c.Layout.Evidence, c.Layout.Locks} {
		if filepath.Clean(dir) != dir || c.ACL.Inspect(dir) != nil {
			return nil, ErrCompositionInvalid
		}
	}
	locks, err := lockfile.NewManager(c.Layout.Locks)
	if err != nil {
		return nil, ErrCompositionInvalid
	}
	store, err := state.NewStore(c.Layout.State, state.WithLockManager(locks))
	if err != nil {
		return nil, ErrCompositionInvalid
	}
	registry, err := installedslot.New(c.Layout, c.ACL, c.Lock)
	if err != nil {
		return nil, ErrCompositionInvalid
	}
	manager, err := instance.New(c.Layout, c.ACL, c.Lock, instance.WithLockManager(locks), instance.WithSlotRegistry(registry), instance.WithStateReader(store))
	if err != nil {
		return nil, ErrCompositionInvalid
	}
	lifecycle := manager.UpdaterLifecycle()
	security := c.MarkerSecurity
	if isNil(security) {
		security, err = update.NewWindowsMarkerSecurity()
		if err != nil {
			return nil, ErrCompositionInvalid
		}
	}
	updater, err := update.New(update.Config{Locks: locks, State: store, Verifier: registry, Lifecycle: lifecycle, Smoke: c.Smoke, MarkerDir: c.Layout.State, MarkerSecurity: security})
	if err != nil {
		return nil, ErrCompositionInvalid
	}
	return &Runtime{Updater: updater, Manager: manager, Registry: registry, State: store, Lifecycle: lifecycle}, nil
}

func isNil(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	return (v.Kind() == reflect.Chan || v.Kind() == reflect.Func || v.Kind() == reflect.Interface || v.Kind() == reflect.Map || v.Kind() == reflect.Pointer || v.Kind() == reflect.Slice) && v.IsNil()
}
