package configtxn

import "errors"

var (
	ErrUnsupportedConfigShape = errors.New("config transaction: unsupported config shape")
	ErrConfigConflict         = errors.New("config transaction: concurrent config conflict")
	ErrRollbackConflict       = errors.New("config transaction: owned value changed")
	ErrRecoveryRequired       = errors.New("config transaction: recovery required")
	ErrRecoveryUnresolved     = errors.New("config transaction: recovery unresolved")
	ErrReconfigureUnsupported = errors.New("config transaction: reconfiguration unsupported")
	ErrUnsafeConfigArtifact   = errors.New("config transaction: unsafe artifact")
	ErrPersistence            = errors.New("config transaction: persistence failed")
	ErrInjectedCrash          = errors.New("config transaction: injected crash")
)
