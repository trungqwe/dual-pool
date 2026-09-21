package instance

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/trungqwe/dual-pool/internal/state"
)

func TestUpdaterLifecycleRejectsInvalidPoolSetsBeforeMutation(t *testing.T) {
	lifecycle := (&Manager{}).UpdaterLifecycle()
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

func TestInjectedDependenciesSkipDefaultConstruction(t *testing.T) {
	fixture, _ := legacyInstallFixture(t)
	layout := fixture.layout
	layout.Locks = filepath.Join(t.TempDir(), "missing-locks")
	layout.State = filepath.Join(t.TempDir(), "missing-state")
	layout.Bin = filepath.Join(t.TempDir(), "missing-bin")
	manager, err := New(layout, fixture.acl, fixture.lock,
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
}
