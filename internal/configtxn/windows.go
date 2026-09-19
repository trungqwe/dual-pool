package configtxn

import (
	"encoding/hex"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var kernel32 = windows.NewLazySystemDLL("kernel32.dll")
var procReplaceFileW = kernel32.NewProc("ReplaceFileW")

type replacementAPI interface {
	replace(target, candidate string) error
	install(candidate, target string) error
}
type windowsReplacement struct{}

func (windowsReplacement) replace(target, candidate string) error {
	t, e := windows.UTF16PtrFromString(target)
	if e != nil {
		return ErrPersistence
	}
	c, e := windows.UTF16PtrFromString(candidate)
	if e != nil {
		return ErrPersistence
	}
	r, _, _ := procReplaceFileW.Call(uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)), 0, 0, 0, 0)
	runtime.KeepAlive(t)
	runtime.KeepAlive(c)
	if r == 0 {
		return ErrPersistence
	}
	return nil
}
func (windowsReplacement) install(candidate, target string) error {
	c, e := windows.UTF16PtrFromString(candidate)
	if e != nil {
		return ErrPersistence
	}
	t, e := windows.UTF16PtrFromString(target)
	if e != nil {
		return ErrPersistence
	}
	return windows.MoveFileEx(c, t, windows.MOVEFILE_WRITE_THROUGH)
}

func fileIdentity(path string) (string, error) {
	p, e := windows.UTF16PtrFromString(path)
	if e != nil {
		return "", ErrUnsafeConfigArtifact
	}
	h, e := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if e != nil {
		return "", ErrPersistence
	}
	defer windows.CloseHandle(h)
	var info windows.ByHandleFileInformation
	if windows.GetFileInformationByHandle(h, &info) != nil || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return "", ErrUnsafeConfigArtifact
	}
	b := make([]byte, 12)
	*(*uint32)(unsafe.Pointer(&b[0])) = info.VolumeSerialNumber
	*(*uint32)(unsafe.Pointer(&b[4])) = info.FileIndexHigh
	*(*uint32)(unsafe.Pointer(&b[8])) = info.FileIndexLow
	return "file_" + hex.EncodeToString(b), nil
}
func isReparse(path string) bool {
	p, e := windows.UTF16PtrFromString(path)
	if e != nil {
		return true
	}
	a, e := windows.GetFileAttributes(p)
	return e != nil || a&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
}
func syncPath(path string) error {
	f, e := os.OpenFile(path, os.O_RDWR, 0)
	if e != nil {
		return ErrPersistence
	}
	se := f.Sync()
	ce := f.Close()
	if se != nil || ce != nil {
		return ErrPersistence
	}
	return nil
}
