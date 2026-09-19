//go:build windows

package dataroot

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/trungqwe/dual-pool/internal/apperr"
)

// Layout names future product paths. Resolution never creates them.
type Layout struct {
	Root      string
	Bin       string
	Instances string
	Config    string
	State     string
	Backups   string
	Evidence  string
	Locks     string
}

func Resolve(localAppData string) (Layout, error) {
	if !validLocalAppData(localAppData) {
		return Layout{}, apperr.MustNew(apperr.CodeDataRootUnavailable, nil)
	}
	root := filepath.Join(filepath.Clean(localAppData), "DualPool")
	return Layout{
		Root: root, Bin: filepath.Join(root, "bin"), Instances: filepath.Join(root, "instances"),
		Config: filepath.Join(root, "config"), State: filepath.Join(root, "state"),
		Backups: filepath.Join(root, "backups"), Evidence: filepath.Join(root, "evidence"),
		Locks: filepath.Join(root, "locks"),
	}, nil
}

func ResolveCurrent() (Layout, error) {
	return Resolve(os.Getenv("LOCALAPPDATA"))
}

func validLocalAppData(value string) bool {
	if value == "" || strings.TrimSpace(value) != value || strings.ContainsRune(value, 0) || !filepath.IsAbs(value) {
		return false
	}
	volume := filepath.VolumeName(value)
	if len(volume) != 2 || volume[1] != ':' || !asciiLetter(volume[0]) {
		return false // UNC, device and drive-relative paths are outside this local-only V1 contract.
	}
	path := strings.ReplaceAll(value[len(volume):], "/", `\`)
	if !strings.HasPrefix(path, `\`) {
		return false
	}
	parts := strings.Split(path[1:], `\`)
	if len(parts) == 0 {
		return false
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") || strings.ContainsAny(part, `<>:"|?*`) || reservedWindowsName(part) {
			return false
		}
	}
	return true
}

func reservedWindowsName(part string) bool {
	name := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
	if name == "CON" || name == "PRN" || name == "AUX" || name == "NUL" || name == "CLOCK$" {
		return true
	}
	return len(name) == 4 && (strings.HasPrefix(name, "COM") || strings.HasPrefix(name, "LPT")) && name[3] >= '1' && name[3] <= '9'
}

func asciiLetter(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}
