package secretstore

import (
	"errors"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	credTypeGeneric         = 1
	credPersistLocalMachine = 2
	credMaxBlobSize         = 5 * 512
)

var (
	advapi32        = windows.NewLazySystemDLL("advapi32.dll")
	procCredWriteW  = advapi32.NewProc("CredWriteW")
	procCredReadW   = advapi32.NewProc("CredReadW")
	procCredDeleteW = advapi32.NewProc("CredDeleteW")
	procCredFree    = advapi32.NewProc("CredFree")
)

type winCredential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

type winCredAPI interface {
	write(target string, secret []byte) error
	read(target string) ([]byte, error)
	delete(target string) error
}

type systemWinCredAPI struct{}

func (systemWinCredAPI) write(target string, secret []byte) error {
	targetPtr, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return windows.ERROR_INVALID_NAME
	}
	credential := winCredential{
		Type:               credTypeGeneric,
		TargetName:         targetPtr,
		CredentialBlobSize: uint32(len(secret)),
		CredentialBlob:     &secret[0],
		Persist:            credPersistLocalMachine,
	}
	result, _, callErr := procCredWriteW.Call(uintptr(unsafe.Pointer(&credential)), 0)
	runtime.KeepAlive(secret)
	runtime.KeepAlive(targetPtr)
	if result == 0 {
		return normalizeCallError(callErr)
	}
	return nil
}

func (systemWinCredAPI) read(target string) ([]byte, error) {
	targetPtr, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return nil, windows.ERROR_INVALID_NAME
	}
	var credential *winCredential
	result, _, callErr := procCredReadW.Call(
		uintptr(unsafe.Pointer(targetPtr)),
		credTypeGeneric,
		0,
		uintptr(unsafe.Pointer(&credential)),
	)
	runtime.KeepAlive(targetPtr)
	if result == 0 {
		return nil, normalizeCallError(callErr)
	}
	if credential == nil {
		return nil, windows.ERROR_INVALID_DATA
	}
	defer func() {
		if credential.CredentialBlob != nil && credential.CredentialBlobSize <= credMaxBlobSize {
			Zero(unsafe.Slice(credential.CredentialBlob, int(credential.CredentialBlobSize)))
		}
		procCredFree.Call(uintptr(unsafe.Pointer(credential)))
	}()
	if credential.Type != credTypeGeneric || credential.CredentialBlobSize == 0 || credential.CredentialBlobSize > credMaxBlobSize || credential.CredentialBlob == nil {
		return nil, windows.ERROR_INVALID_DATA
	}
	secret := append([]byte(nil), unsafe.Slice(credential.CredentialBlob, int(credential.CredentialBlobSize))...)
	return secret, nil
}

func (systemWinCredAPI) delete(target string) error {
	targetPtr, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return windows.ERROR_INVALID_NAME
	}
	result, _, callErr := procCredDeleteW.Call(uintptr(unsafe.Pointer(targetPtr)), credTypeGeneric, 0)
	runtime.KeepAlive(targetPtr)
	if result == 0 {
		return normalizeCallError(callErr)
	}
	return nil
}

func normalizeCallError(err error) error {
	if err == nil || errors.Is(err, windows.ERROR_SUCCESS) {
		return windows.ERROR_GEN_FAILURE
	}
	return err
}
