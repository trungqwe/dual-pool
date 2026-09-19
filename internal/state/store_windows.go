package state

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var replaceFileW = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReplaceFileW")

type windowsReplacer struct{}

func (windowsReplacer) replaceExisting(target, candidate, backup string) error {
	targetPtr, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	candidatePtr, err := windows.UTF16PtrFromString(candidate)
	if err != nil {
		return err
	}
	var backupPtr *uint16
	if backup != "" {
		backupPtr, err = windows.UTF16PtrFromString(backup)
		if err != nil {
			return err
		}
	}
	r1, _, callErr := replaceFileW.Call(uintptr(unsafe.Pointer(targetPtr)), uintptr(unsafe.Pointer(candidatePtr)), uintptr(unsafe.Pointer(backupPtr)), 0, 0, 0)
	if r1 == 0 {
		if callErr != nil && callErr != syscall.Errno(0) {
			return callErr
		}
		return syscall.EINVAL
	}
	return nil
}

func (windowsReplacer) installNew(candidate, target string) error {
	from, err := windows.UTF16PtrFromString(candidate)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(from, to, windows.MOVEFILE_WRITE_THROUGH)
}

func isReparsePoint(path string) bool {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return true
	}
	attributes, err := windows.GetFileAttributes(name)
	return err != nil || attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
}
