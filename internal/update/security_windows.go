package update

import (
	"github.com/trungqwe/dual-pool/internal/winacl"
	"os"
)

type WindowsMarkerSecurity struct{ manager *winacl.Manager }

func NewWindowsMarkerSecurity() (*WindowsMarkerSecurity, error) {
	m, err := winacl.New()
	if err != nil {
		return nil, err
	}
	return &WindowsMarkerSecurity{manager: m}, nil
}
func (s *WindowsMarkerSecurity) InspectDir(path string) error { return s.manager.Inspect(path) }
func (s *WindowsMarkerSecurity) CreateFile(path string) (*os.File, error) {
	return s.manager.CreateFile(path)
}
func (s *WindowsMarkerSecurity) InspectHandle(file *os.File) error {
	return s.manager.InspectHandle(file)
}
