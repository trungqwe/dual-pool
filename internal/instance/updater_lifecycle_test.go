package instance

import (
	"context"
	"errors"
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
