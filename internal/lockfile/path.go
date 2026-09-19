package lockfile

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func canonicalTarget(value string) (string, error) {
	absolute, err := filepath.Abs(value)
	if err != nil || !localAbsolute(absolute) {
		return "", ErrUnsafeLockArtifact
	}
	base := filepath.Base(absolute)
	if !safeComponent(base) {
		return "", ErrUnsafeLockArtifact
	}
	exists, err := safeEntry(absolute)
	if err != nil {
		return "", err
	}
	if exists {
		resolved, err := filepath.EvalSymlinks(absolute)
		if err != nil {
			return "", ErrUnsafeLockArtifact
		}
		return strings.ToLower(filepath.Clean(resolved)), nil
	}
	parent := filepath.Dir(absolute)
	ok, err := safeEntry(parent)
	if err != nil || !ok {
		return "", ErrUnsafeLockArtifact
	}
	resolved, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", ErrUnsafeLockArtifact
	}
	return strings.ToLower(filepath.Join(resolved, base)), nil
}
func fileResource(value string) (string, error) {
	canonical, err := canonicalTarget(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:]), nil
}
func localAbsolute(value string) bool {
	if len(value) < 4 || value[1] != ':' || value[2] != '\\' || !letter(value[0]) || strings.HasPrefix(value, `\\`) || strings.Contains(value, "/") {
		return false
	}
	for _, part := range strings.Split(value[3:], `\`) {
		if !safeComponent(part) {
			return false
		}
	}
	return true
}
func safeComponent(part string) bool {
	if part == "" || part == "." || part == ".." || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") || strings.ContainsAny(part, `<>:"|?*`) {
		return false
	}
	name := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
	if name == "CON" || name == "PRN" || name == "AUX" || name == "NUL" || name == "CLOCK$" {
		return false
	}
	return !(len(name) == 4 && (strings.HasPrefix(name, "COM") || strings.HasPrefix(name, "LPT")) && name[3] >= '1' && name[3] <= '9')
}
func safeEntry(path string) (bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, ErrLockPersistence
	}
	ptr, e := windows.UTF16PtrFromString(path)
	if e != nil {
		return false, ErrUnsafeLockArtifact
	}
	attrs, e := windows.GetFileAttributes(ptr)
	if e != nil || info.Mode()&os.ModeSymlink != 0 || attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return false, ErrUnsafeLockArtifact
	}
	return true, nil
}
func letter(v byte) bool { return v >= 'A' && v <= 'Z' || v >= 'a' && v <= 'z' }
