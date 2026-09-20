//go:build windows

// Package processidentity provides one canonical Windows process identity
// implementation. A handle opened for termination can be verified and used
// without a PID reopen race.
package processidentity

import (
	"errors"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

var (
	ErrNotFound     = errors.New("process is not running")
	ErrUnverifiable = errors.New("process identity is unverifiable")
)

type Identity struct {
	PID       uint32
	StartTime uint64
	Image     string
}
type Handle struct {
	value windows.Handle
	pid   uint32
}

func Inspect(pid uint32) (Identity, error) {
	if pid == 0 {
		return Identity{}, ErrNotFound
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			return Identity{}, ErrNotFound
		}
		return Identity{}, ErrUnverifiable
	}
	defer windows.CloseHandle(h)
	return inspectHandle(h, pid)
}

func OpenForTermination(pid uint32) (*Handle, error) {
	if pid == 0 {
		return nil, ErrNotFound
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, pid)
	if err != nil {
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			return nil, ErrNotFound
		}
		return nil, ErrUnverifiable
	}
	return &Handle{value: h, pid: pid}, nil
}
func (h *Handle) Inspect() (Identity, error) {
	if h == nil || h.value == 0 {
		return Identity{}, ErrUnverifiable
	}
	return inspectHandle(h.value, h.pid)
}
func (h *Handle) Terminate() error {
	if h == nil || h.value == 0 {
		return ErrUnverifiable
	}
	if err := windows.TerminateProcess(h.value, 1); err != nil {
		return ErrUnverifiable
	}
	return nil
}
func (h *Handle) Wait(milliseconds uint32) error {
	if h == nil || h.value == 0 {
		return ErrUnverifiable
	}
	r, e := windows.WaitForSingleObject(h.value, milliseconds)
	if e != nil || r != windows.WAIT_OBJECT_0 {
		return ErrUnverifiable
	}
	return nil
}
func (h *Handle) Close() error {
	if h == nil || h.value == 0 {
		return nil
	}
	e := windows.CloseHandle(h.value)
	h.value = 0
	return e
}

func inspectHandle(h windows.Handle, pid uint32) (Identity, error) {
	var code uint32
	if windows.GetExitCodeProcess(h, &code) != nil {
		return Identity{}, ErrUnverifiable
	}
	if code != 259 {
		return Identity{}, ErrNotFound
	}
	var created, exited, kernel, user windows.Filetime
	if windows.GetProcessTimes(h, &created, &exited, &kernel, &user) != nil {
		return Identity{}, ErrUnverifiable
	}
	start := (uint64(created.HighDateTime) << 32) | uint64(created.LowDateTime)
	if start == 0 {
		return Identity{}, ErrUnverifiable
	}
	b := make([]uint16, 32768)
	n := uint32(len(b))
	if windows.QueryFullProcessImageName(h, 0, &b[0], &n) != nil || n == 0 {
		return Identity{}, ErrUnverifiable
	}
	image, err := CanonicalExistingPath(windows.UTF16ToString(b[:n]))
	if err != nil {
		return Identity{}, ErrUnverifiable
	}
	return Identity{pid, start, image}, nil
}
func CanonicalExistingPath(value string) (string, error) {
	absolute, err := filepath.Abs(value)
	if err != nil || !localAbsolute(absolute) {
		return "", ErrUnverifiable
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", ErrUnverifiable
	}
	return strings.ToLower(filepath.Clean(resolved)), nil
}
func localAbsolute(v string) bool {
	return len(v) >= 4 && v[1] == ':' && v[2] == '\\' && !strings.Contains(v, "/")
}
