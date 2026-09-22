// Package runtimeupdate composes the trusted update transaction graph.
package runtimeupdate

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"

	"github.com/trungqwe/dual-pool/internal/dataroot"
	"github.com/trungqwe/dual-pool/internal/installedslot"
	"github.com/trungqwe/dual-pool/internal/instance"
	"github.com/trungqwe/dual-pool/internal/lockfile"
	"github.com/trungqwe/dual-pool/internal/state"
	"github.com/trungqwe/dual-pool/internal/update"
	"github.com/trungqwe/dual-pool/internal/upstreamcatalog"
	"github.com/trungqwe/dual-pool/internal/upstreamlock"
	"github.com/trungqwe/dual-pool/internal/upstreamstage"
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
}

// Runtime exposes only the operational command boundary. Mutable state and
// locked lifecycle dependencies remain private transaction authorities.
type Runtime struct {
	Updater *update.Updater
	Manager *instance.Manager

	locks    *lockfile.Manager
	state    *state.Store
	registry *installedslot.Registry
	catalog  *upstreamcatalog.Catalog
	lock     upstreamlock.Lock
	layout   dataroot.Layout
	acl      ACL
}

func New(c Config) (*Runtime, error) {
	if isNil(c.ACL) || c.Lock.Validate() != nil || c.Layout.Root == "" || c.Layout.State == "" || c.Layout.Locks == "" {
		return nil, ErrCompositionInvalid
	}
	for _, dir := range []string{c.Layout.Root, c.Layout.Bin, c.Layout.Instances, c.Layout.Config, c.Layout.State, c.Layout.Backups, c.Layout.Evidence, c.Layout.Locks} {
		if filepath.Clean(dir) != dir || c.ACL.Inspect(dir) != nil {
			return nil, ErrCompositionInvalid
		}
	}
	catalog, err := upstreamcatalog.Production(c.Lock)
	if err != nil {
		return nil, ErrCompositionInvalid
	}
	locks, err := lockfile.NewManager(c.Layout.Locks)
	if err != nil {
		return nil, ErrCompositionInvalid
	}
	store, err := state.NewStore(c.Layout.State, state.WithLockManager(locks))
	if err != nil {
		return nil, ErrCompositionInvalid
	}
	registry, err := installedslot.NewWithCatalog(c.Layout, c.ACL, catalog, installedslot.WithLockManager(locks))
	if err != nil {
		return nil, ErrCompositionInvalid
	}
	manager, err := instance.New(c.Layout, c.ACL, c.Lock, instance.WithLockManager(locks), instance.WithSlotRegistry(registry), instance.WithStateReader(store))
	if err != nil {
		return nil, ErrCompositionInvalid
	}
	updater, err := instance.ComposeUpdater(manager)
	if err != nil {
		return nil, ErrCompositionInvalid
	}
	return &Runtime{Updater: updater, Manager: manager, locks: locks, state: store, registry: registry, catalog: catalog, lock: c.Lock, layout: c.Layout, acl: c.ACL}, nil
}

// StageVerifiedCandidate creates the fixed product-relative stage root under
// GLOBAL, releases the lock, then lets StageCandidate acquire that same lock.
func (r *Runtime) StageVerifiedCandidate(ctx context.Context) (upstreamstage.Result, error) {
	return r.stageVerifiedCandidate(ctx)
}

func (r *Runtime) stageVerifiedCandidate(ctx context.Context, options ...upstreamstage.Option) (upstreamstage.Result, error) {
	if r == nil || r.locks == nil || isNil(r.acl) || r.lock.Validate() != nil {
		return upstreamstage.Result{}, ErrCompositionInvalid
	}
	stageRoot := filepath.Join(r.layout.Bin, "upstream-stage")
	guard, err := r.locks.AcquireGlobal()
	if err != nil {
		return upstreamstage.Result{}, ErrCompositionInvalid
	}
	if err = r.acl.Create(stageRoot); err == nil {
		err = r.acl.Inspect(stageRoot)
	}
	releaseErr := guard.Release()
	if err != nil || releaseErr != nil {
		return upstreamstage.Result{}, ErrCompositionInvalid
	}
	stager, err := r.candidateStager(options...)
	if err != nil {
		return upstreamstage.Result{}, ErrCompositionInvalid
	}
	return stager.StageCandidate(ctx, r.lock)
}

func (r *Runtime) candidateStager(options ...upstreamstage.Option) (*upstreamstage.Stager, error) {
	if r == nil || r.locks == nil {
		return nil, ErrCompositionInvalid
	}
	return upstreamstage.New(filepath.Join(r.layout.Bin, "upstream-stage"), r.locks, options...)
}

// InstallVerifiedCandidate exposes only the source-stage result. Manager
// independently validates it against the one reviewed production candidate.
func (r *Runtime) InstallVerifiedCandidate(ctx context.Context, stage upstreamstage.Result) (string, bool, error) {
	if r == nil || r.Manager == nil {
		return "", false, ErrCompositionInvalid
	}
	return r.Manager.InstallCandidate(ctx, stage)
}

func isNil(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	return (v.Kind() == reflect.Chan || v.Kind() == reflect.Func || v.Kind() == reflect.Interface || v.Kind() == reflect.Map || v.Kind() == reflect.Pointer || v.Kind() == reflect.Slice) && v.IsNil()
}
