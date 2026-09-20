package configtxn

import (
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func validatePathHierarchy(path string, finalDirectory bool) error {
	volume := filepath.VolumeName(path)
	current := volume + `\`
	parts := strings.Split(path[len(volume)+1:], `\`)
	for i, part := range parts {
		current = filepath.Join(current, part)
		p, err := windows.UTF16PtrFromString(current)
		if err != nil {
			return ErrUnsafeConfigArtifact
		}
		attributes, err := windows.GetFileAttributes(p)
		if err != nil || attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return ErrUnsafeConfigArtifact
		}
		isLast := i == len(parts)-1
		isDirectory := attributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0
		if (!isLast && !isDirectory) || (isLast && finalDirectory != isDirectory) {
			return ErrUnsafeConfigArtifact
		}
	}
	return nil
}

func readSafeRegular(path string) ([]byte, error) {
	if err := validatePathHierarchy(path, false); err != nil {
		if _, statErr := os.Lstat(path); os.IsNotExist(statErr) {
			return nil, statErr
		}
		return nil, ErrUnsafeConfigArtifact
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, ErrUnsafeConfigArtifact
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, ErrPersistence
	}
	f := os.NewFile(uintptr(h), path)
	defer f.Close()
	var info windows.ByHandleFileInformation
	if windows.GetFileInformationByHandle(h, &info) != nil || info.FileAttributes&(windows.FILE_ATTRIBUTE_REPARSE_POINT|windows.FILE_ATTRIBUTE_DIRECTORY) != 0 {
		return nil, ErrUnsafeConfigArtifact
	}
	b, err := io.ReadAll(io.LimitReader(f, maxMarkerBytes+maxConfigBytes+1))
	if err != nil {
		return nil, ErrPersistence
	}
	return b, nil
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
