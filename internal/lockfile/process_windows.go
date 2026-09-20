package lockfile

import (
	"errors"

	"github.com/trungqwe/dual-pool/internal/processidentity"
)

type WindowsProcessInspector struct{}

func (WindowsProcessInspector) Inspect(pid uint32) (ProcessIdentity, error) {
	v, err := processidentity.Inspect(pid)
	if errors.Is(err, processidentity.ErrNotFound) {
		return ProcessIdentity{}, ErrProcessNotFound
	}
	if err != nil {
		return ProcessIdentity{}, ErrLockOwnerUnverifiable
	}
	return ProcessIdentity{PID: v.PID, StartTime: v.StartTime, Image: v.Image}, nil
}
