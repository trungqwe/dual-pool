package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"

	"github.com/trungqwe/dual-pool/internal/lockfile"
	"github.com/trungqwe/dual-pool/internal/state"
)

var (
	ErrActiveSelectionMissing = errors.New("active upstream version is required")
	ErrCandidateInvalid       = errors.New("update candidate is invalid")
	ErrRollbackUnresolved     = errors.New("update rollback is unresolved")
	ErrRecoveryUnresolved     = errors.New("update recovery is unresolved")
	ErrPromotionRolledBack    = errors.New("update promotion failed and rollback completed")
	ErrInjectedCrash          = errors.New("injected update crash")
	ErrUpdatePending          = errors.New("update transaction recovery is required")
)

type StateRepository interface {
	LoadState() (state.State, error)
	SaveState(state.State) error
	Recover() error
}

type SlotVerifier interface {
	VerifyInstalled(context.Context, string) error
}
type Lifecycle interface {
	CaptureRunning(context.Context) ([]state.Pool, error)
	Stop(context.Context, []state.Pool) error
	Start(context.Context, []state.Pool) error
}
type Smoke interface {
	Disposable(context.Context, string) error
	Production(context.Context, string) error
}
type FaultPoint string

const (
	AfterCandidatePreSmoke FaultPoint = "AFTER_CANDIDATE_PRE_SMOKE"
	AfterMarkerPublish     FaultPoint = "AFTER_MARKER_PUBLISH"
	AfterProductionStop    FaultPoint = "AFTER_PRODUCTION_STOP"
	AfterActiveSave        FaultPoint = "AFTER_ACTIVE_SAVE"
	AfterCandidateStart    FaultPoint = "AFTER_CANDIDATE_START"
	AfterCandidateSmoke    FaultPoint = "AFTER_CANDIDATE_SMOKE"
	BeforeMarkerCleanup    FaultPoint = "BEFORE_MARKER_CLEANUP"
	BeforeRollbackRestore  FaultPoint = "BEFORE_ROLLBACK_RESTORE"
	AfterRollbackRestore   FaultPoint = "AFTER_ROLLBACK_RESTORE"
	BeforeRollbackCleanup  FaultPoint = "BEFORE_ROLLBACK_CLEANUP"
)

type Config struct {
	Locks          *lockfile.Manager
	State          StateRepository
	Verifier       SlotVerifier
	Lifecycle      Lifecycle
	Smoke          Smoke
	MarkerDir      string
	Fault          func(FaultPoint) error
	TransactionID  func() (string, error)
	MarkerSecurity MarkerSecurity
}
type Updater struct {
	locks     *lockfile.Manager
	state     StateRepository
	verifier  SlotVerifier
	lifecycle Lifecycle
	smoke     Smoke
	markers   *markerStore
	fault     func(FaultPoint) error
}

func New(c Config) (*Updater, error) {
	if c.Locks == nil || c.State == nil || c.Verifier == nil || c.Lifecycle == nil || c.Smoke == nil {
		return nil, ErrCandidateInvalid
	}
	markers, err := newMarkerStore(c.MarkerDir, c.TransactionID, c.MarkerSecurity)
	if err != nil {
		return nil, err
	}
	return &Updater{locks: c.Locks, state: c.State, verifier: c.Verifier, lifecycle: c.Lifecycle, smoke: c.Smoke, markers: markers, fault: c.Fault}, nil
}
func (u *Updater) Promote(ctx context.Context, candidate string) error {
	guard, err := u.locks.AcquireGlobal()
	if err != nil {
		return err
	}
	defer guard.Release()
	if err = u.state.Recover(); err != nil {
		return err
	}
	if _, pending, markerErr := u.markers.load(); markerErr != nil {
		return markerErr
	} else if pending {
		return ErrUpdatePending
	}
	before, err := u.state.LoadState()
	if err != nil {
		return err
	}
	previous := before.ActiveUpstreamVersion
	if previous == "" {
		return ErrActiveSelectionMissing
	}
	if !validLogicalVersion(previous) || !validLogicalVersion(candidate) || candidate == previous {
		return ErrCandidateInvalid
	}
	if err = u.verifier.VerifyInstalled(ctx, previous); err != nil {
		return err
	}
	if err = u.verifier.VerifyInstalled(ctx, candidate); err != nil {
		return err
	}
	if err = u.smoke.Disposable(ctx, candidate); err != nil {
		return err
	}
	if err = u.inject(AfterCandidatePreSmoke); err != nil {
		return err
	}
	running, err := u.lifecycle.CaptureRunning(ctx)
	if err != nil {
		return err
	}
	running, err = validateRunningSet(running)
	if err != nil {
		return err
	}
	marker := transactionMarker{SchemaVersion: 1, PreviousVersion: previous, CandidateVersion: candidate, BaseStateSHA256: stateFingerprint(before), RestartPools: copyPools(running)}
	marker, err = u.markers.publish(marker)
	if err != nil {
		return err
	}
	if err = u.inject(AfterMarkerPublish); err != nil {
		return err
	}
	if err = u.lifecycle.Stop(ctx, copyPools(running)); err != nil {
		return u.rollback(ctx, marker, err)
	}
	if err = u.inject(AfterProductionStop); err != nil {
		return u.rollback(ctx, marker, err)
	}
	after := before
	after.ActiveUpstreamVersion = candidate
	if err = u.state.SaveState(after); err != nil {
		return u.rollback(ctx, marker, err)
	}
	if err = u.inject(AfterActiveSave); err != nil {
		return u.rollback(ctx, marker, err)
	}
	if err = u.lifecycle.Start(ctx, copyPools(running)); err != nil {
		return u.rollback(ctx, marker, err)
	}
	if err = u.inject(AfterCandidateStart); err != nil {
		return u.rollback(ctx, marker, err)
	}
	if err = u.smoke.Production(ctx, candidate); err != nil {
		return u.rollback(ctx, marker, err)
	}
	if err = u.inject(AfterCandidateSmoke); err != nil {
		return u.rollback(ctx, marker, err)
	}
	if err = u.inject(BeforeMarkerCleanup); err != nil {
		return u.rollback(ctx, marker, err)
	}
	return u.markers.remove(marker)
}
func (u *Updater) Recover(ctx context.Context) error {
	guard, err := u.locks.AcquireGlobal()
	if err != nil {
		return err
	}
	defer guard.Release()
	if err = u.state.Recover(); err != nil {
		return err
	}
	marker, exists, err := u.markers.load()
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	current, err := u.state.LoadState()
	if err != nil {
		return err
	}
	if stateFingerprint(current) != marker.BaseStateSHA256 {
		return ErrRecoveryUnresolved
	}
	if err = u.verifier.VerifyInstalled(ctx, marker.PreviousVersion); err != nil {
		return ErrRecoveryUnresolved
	}
	switch current.ActiveUpstreamVersion {
	case marker.PreviousVersion:
		if err = u.lifecycle.Start(ctx, copyPools(marker.RestartPools)); err != nil {
			return ErrRecoveryUnresolved
		}
		if err = u.smoke.Production(ctx, marker.PreviousVersion); err != nil {
			return ErrRecoveryUnresolved
		}
		if err = u.markers.remove(marker); err != nil {
			return ErrRecoveryUnresolved
		}
		return nil
	case marker.CandidateVersion:
		return u.rollback(ctx, marker, errors.New("candidate selection requires rollback"))
	default:
		return ErrRecoveryUnresolved
	}
}
func (u *Updater) rollback(ctx context.Context, marker transactionMarker, cause error) error {
	if errors.Is(cause, ErrInjectedCrash) {
		return cause
	}
	if err := u.lifecycle.Stop(ctx, copyPools(marker.RestartPools)); err != nil {
		return ErrRollbackUnresolved
	}
	if err := u.state.Recover(); err != nil {
		return ErrRollbackUnresolved
	}
	current, err := u.state.LoadState()
	if err != nil || stateFingerprint(current) != marker.BaseStateSHA256 {
		return ErrRollbackUnresolved
	}
	if err = u.inject(BeforeRollbackRestore); err != nil {
		return err
	}
	current.ActiveUpstreamVersion = marker.PreviousVersion
	if err = u.state.SaveState(current); err != nil {
		return ErrRollbackUnresolved
	}
	if err = u.inject(AfterRollbackRestore); err != nil {
		return err
	}
	if err = u.lifecycle.Start(ctx, copyPools(marker.RestartPools)); err != nil {
		return ErrRollbackUnresolved
	}
	if err = u.smoke.Production(ctx, marker.PreviousVersion); err != nil {
		return ErrRollbackUnresolved
	}
	if err = u.inject(BeforeRollbackCleanup); err != nil {
		return err
	}
	if err = u.markers.remove(marker); err != nil {
		return ErrRollbackUnresolved
	}
	return fmt.Errorf("%w: %v", ErrPromotionRolledBack, cause)
}
func (u *Updater) inject(p FaultPoint) error {
	if u.fault == nil {
		return nil
	}
	return u.fault(p)
}
func stateFingerprint(s state.State) string {
	s.ActiveUpstreamVersion = ""
	b, _ := state.EncodeState(s)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func normalizePools(in []state.Pool) []state.Pool {
	out := make([]state.Pool, 0, len(in))
	for _, p := range in {
		if p == state.PoolCodex || p == state.PoolGoogle {
			found := false
			for _, q := range out {
				if p == q {
					found = true
				}
			}
			if !found {
				out = append(out, p)
			}
		}
	}
	return out
}

var logicalVersionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$`)

func validLogicalVersion(value string) bool { return logicalVersionPattern.MatchString(value) }

func validateRunningSet(in []state.Pool) ([]state.Pool, error) {
	seen := map[state.Pool]bool{}
	out := append([]state.Pool(nil), in...)
	for _, pool := range out {
		if (pool != state.PoolCodex && pool != state.PoolGoogle) || seen[pool] {
			return nil, ErrCandidateInvalid
		}
		seen[pool] = true
	}
	return out, nil
}

func copyPools(in []state.Pool) []state.Pool { return append([]state.Pool(nil), in...) }
