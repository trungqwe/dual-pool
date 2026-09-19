package lockfile

import (
	"errors"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

type WindowsProcessInspector struct{}

func (WindowsProcessInspector) Inspect(pid uint32) (ProcessIdentity, error) {
	if pid == 0 {
		return ProcessIdentity{}, ErrProcessNotFound
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			return ProcessIdentity{}, ErrProcessNotFound
		}
		return ProcessIdentity{}, ErrLockOwnerUnverifiable
	}
	defer windows.CloseHandle(handle)
	var exitCode uint32
	if windows.GetExitCodeProcess(handle, &exitCode) != nil {
		return ProcessIdentity{}, ErrLockOwnerUnverifiable
	}
	if exitCode != 259 { // STILL_ACTIVE
		return ProcessIdentity{}, ErrProcessNotFound
	}
	var created, exit, kernel, user windows.Filetime
	if windows.GetProcessTimes(handle, &created, &exit, &kernel, &user) != nil {
		return ProcessIdentity{}, ErrLockOwnerUnverifiable
	}
	start := (uint64(created.HighDateTime) << 32) | uint64(created.LowDateTime)
	if start == 0 {
		return ProcessIdentity{}, ErrLockOwnerUnverifiable
	}
	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	if windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size) != nil || size == 0 {
		return ProcessIdentity{}, ErrLockOwnerUnverifiable
	}
	image, err := canonicalExistingPath(windows.UTF16ToString(buffer[:size]))
	if err != nil {
		return ProcessIdentity{}, ErrLockOwnerUnverifiable
	}
	return ProcessIdentity{PID: pid, StartTime: start, Image: image}, nil
}

func canonicalExistingPath(value string) (string, error) {
	absolute, err := filepath.Abs(value)
	if err != nil || !localAbsolute(absolute) {
		return "", ErrUnsafeLockArtifact
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", ErrUnsafeLockArtifact
	}
	return strings.ToLower(filepath.Clean(resolved)), nil
}
