package winacl

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var ErrUnsafeACL = errors.New("unsafe product directory ACL")

const fileAllAccess windows.ACCESS_MASK = 0x001f01ff

type Manager struct{ user string }

func New() (*Manager, error) {
	token := windows.GetCurrentProcessToken()
	u, err := token.GetTokenUser()
	if err != nil || u == nil || u.User.Sid == nil {
		return nil, ErrUnsafeACL
	}
	return &Manager{user: u.User.Sid.String()}, nil
}

func (m *Manager) Create(path string) error {
	if err := validateParent(path); err != nil {
		return err
	}
	sd, err := windows.SecurityDescriptorFromString("O:" + m.user + "D:P(A;OICI;FA;;;" + m.user + ")(A;OICI;FA;;;SY)")
	if err != nil {
		return ErrUnsafeACL
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return ErrUnsafeACL
	}
	sa := &windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	if err = windows.CreateDirectory(p, sa); err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return ErrUnsafeACL
	}
	return m.Inspect(path)
}

func (m *Manager) Inspect(path string) error {
	if !safeDirectory(path) {
		return ErrUnsafeACL
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil || sd == nil {
		return ErrUnsafeACL
	}
	owner, _, err := sd.Owner()
	if err != nil || owner == nil || owner.String() != m.user {
		return ErrUnsafeACL
	}
	control, _, err := sd.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		return ErrUnsafeACL
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil || dacl.AceCount != 2 {
		return ErrUnsafeACL
	}
	allowed := map[string]bool{m.user: false, "S-1-5-18": false}
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if windows.GetAce(dacl, i, &ace) != nil || ace == nil || ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Header.AceFlags&windows.INHERITED_ACE != 0 || ace.Mask != fileAllAccess {
			return ErrUnsafeACL
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		name := sid.String()
		if _, ok := allowed[name]; !ok || allowed[name] {
			return ErrUnsafeACL
		}
		allowed[name] = true
	}
	if !allowed[m.user] || !allowed["S-1-5-18"] {
		return ErrUnsafeACL
	}
	return nil
}

func validateParent(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil || filepath.Clean(path) != abs || len(abs) < 4 || abs[1] != ':' || abs[2] != '\\' || strings.Contains(abs, "/") {
		return ErrUnsafeACL
	}
	root := filepath.VolumeName(abs) + `\`
	p, _ := windows.UTF16PtrFromString(root)
	var flags uint32
	if windows.GetVolumeInformation(p, nil, 0, nil, nil, &flags, nil, 0) != nil || flags&windows.FILE_PERSISTENT_ACLS == 0 {
		return ErrUnsafeACL
	}
	parent := filepath.Dir(abs)
	for current := root; len(current) <= len(parent); {
		if !safeDirectory(current) {
			return ErrUnsafeACL
		}
		if strings.EqualFold(current, parent) {
			break
		}
		rest := strings.TrimPrefix(parent, current)
		rest = strings.TrimPrefix(rest, `\`)
		part := strings.SplitN(rest, `\`, 2)[0]
		if part == "" {
			return ErrUnsafeACL
		}
		current = filepath.Join(current, part)
	}
	return nil
}

func safeDirectory(path string) bool {
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	a, err := windows.GetFileAttributes(p)
	return err == nil && a&windows.FILE_ATTRIBUTE_DIRECTORY != 0 && a&windows.FILE_ATTRIBUTE_REPARSE_POINT == 0
}
