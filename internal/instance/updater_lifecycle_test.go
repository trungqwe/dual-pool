package instance

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/trungqwe/dual-pool/internal/dataroot"
	"github.com/trungqwe/dual-pool/internal/lockfile"
	"github.com/trungqwe/dual-pool/internal/state"
	"github.com/trungqwe/dual-pool/internal/upstreamlock"
)

func TestComposeUpdaterCannotSplitManagerAuthority(t *testing.T) {
	composition := reflect.TypeOf(ComposeUpdater)
	if composition.NumIn() != 1 || composition.In(0) != reflect.TypeOf((*Manager)(nil)) {
		t.Fatalf("ComposeUpdater accepts caller-selected transaction authority: %v", composition)
	}
}

func TestUpdaterLifecycleRejectsInvalidPoolSetsBeforeMutation(t *testing.T) {
	lifecycle := (&Manager{}).updaterLifecycle()
	for _, pools := range [][]state.Pool{{state.Pool("other")}, {state.PoolCodex, state.PoolCodex}, {state.PoolGoogle, state.PoolGoogle}} {
		if err := lifecycle.Stop(context.Background(), pools); !errors.Is(err, ErrUnsafeInstance) {
			t.Fatalf("Stop(%v) = %v", pools, err)
		}
		if err := lifecycle.Start(context.Background(), pools); !errors.Is(err, ErrUnsafeInstance) {
			t.Fatalf("Start(%v) = %v", pools, err)
		}
	}
	if err := lifecycle.Stop(context.Background(), nil); err != nil {
		t.Fatalf("empty Stop = %v", err)
	}
}

func TestWithLockManagerNilFailsClosed(t *testing.T) {
	fixture, _ := legacyInstallFixture(t)
	if manager, err := New(fixture.layout, fixture.acl, fixture.lock, WithLockManager(nil)); err == nil || manager != nil {
		t.Fatalf("nil lock manager accepted: %#v %v", manager, err)
	}
}

func TestInjectedLockManagerMustMatchLayoutLocks(t *testing.T) {
	fixture, _ := legacyInstallFixture(t)
	otherLocks := filepath.Join(fixture.layout.Root, "other-locks")
	if err := fixture.acl.Create(otherLocks); err != nil {
		t.Fatal(err)
	}
	other, err := lockfile.NewManager(otherLocks)
	if err != nil {
		t.Fatal(err)
	}
	if manager, err := New(fixture.layout, fixture.acl, fixture.lock, WithLockManager(other), WithSlotRegistry(fixture.registry), WithStateReader(fixture.state)); err == nil || manager != nil {
		t.Fatalf("mismatched lock root accepted: %#v %v", manager, err)
	}
}

func TestExplicitNilInjectedDependenciesFailClosed(t *testing.T) {
	fixture, _ := legacyInstallFixture(t)
	for name, option := range map[string]Option{
		"registry": WithSlotRegistry(nil),
		"state":    WithStateReader(nil),
	} {
		if manager, err := New(fixture.layout, fixture.acl, fixture.lock, option); err == nil || manager != nil {
			t.Fatalf("explicit nil %s accepted: %#v %v", name, manager, err)
		}
	}
}

func TestInjectedDependenciesSuppressDefaultConstruction(t *testing.T) {
	fixture, _ := legacyInstallFixture(t)
	counts := struct{ locks, registry, state int }{}
	factories := dependencyFactories{
		newLocks: func(string) (*lockfile.Manager, error) {
			counts.locks++
			return nil, errors.New("default lock construction should not run")
		},
		newRegistry: func(dataroot.Layout, ACL, upstreamlock.Lock, *lockfile.Manager) (SlotRegistry, error) {
			counts.registry++
			return nil, errors.New("default registry construction should not run")
		},
		newState: func(string, *lockfile.Manager) (ActiveStateReader, error) {
			counts.state++
			return nil, errors.New("default state construction should not run")
		},
	}
	manager, err := newManager(fixture.layout, fixture.acl, fixture.lock, factories,
		WithLockManager(fixture.locks),
		WithSlotRegistry(fixture.registry),
		WithStateReader(fixture.state),
	)
	if err != nil {
		t.Fatalf("complete injection attempted default construction: %v", err)
	}
	if manager.locks != fixture.locks || manager.registry != fixture.registry || !reflect.DeepEqual(manager.state, fixture.state) {
		t.Fatal("injected dependencies were not retained as construction authority")
	}
	if counts.locks != 0 || counts.registry != 0 || counts.state != 0 {
		t.Fatalf("default construction calls = %+v", counts)
	}
}

func TestInjectedDependenciesDoNotBypassLayoutValidation(t *testing.T) {
	fixture, _ := legacyInstallFixture(t)
	layout := fixture.layout
	layout.State = filepath.Join(t.TempDir(), "missing-state")
	if manager, err := New(layout, fixture.acl, fixture.lock,
		WithLockManager(fixture.locks),
		WithSlotRegistry(fixture.registry),
		WithStateReader(fixture.state),
	); err == nil || manager != nil {
		t.Fatalf("invalid injected layout accepted: %#v %v", manager, err)
	}
}

func TestDefaultDependenciesStillConstruct(t *testing.T) {
	fixture, _ := legacyInstallFixture(t)
	manager, err := New(fixture.layout, fixture.acl, fixture.lock)
	if err != nil || manager == nil || manager.locks == nil || manager.registry == nil || manager.state == nil {
		t.Fatalf("default dependencies failed: %#v %v", manager, err)
	}
}
