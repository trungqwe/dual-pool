package apperr

import "fmt"

type Category int

const (
	CategorySuccess       Category = 0
	CategoryUsage         Category = 2
	CategoryCompatibility Category = 10
	CategoryConfig        Category = 20
	CategoryAuth          Category = 30
	CategoryService       Category = 40
	CategorySecurity      Category = 50
	CategoryTest          Category = 60
)

func (category Category) ExitCode() int { return int(category) }

func (category Category) String() string {
	switch category {
	case CategorySuccess:
		return "success"
	case CategoryUsage:
		return "usage"
	case CategoryCompatibility:
		return "compatibility"
	case CategoryConfig:
		return "config"
	case CategoryAuth:
		return "auth"
	case CategoryService:
		return "service"
	case CategorySecurity:
		return "security"
	case CategoryTest:
		return "test"
	default:
		return "unknown"
	}
}

type Code string

const (
	CodePortInUse                Code = "PORT_IN_USE"
	CodeProcessIdentityMismatch  Code = "PROCESS_IDENTITY_MISMATCH"
	CodeUpstreamVersionMismatch  Code = "UPSTREAM_VERSION_MISMATCH"
	CodeManagementUnavailable    Code = "MANAGEMENT_UNAVAILABLE"
	CodeOAuthTimeout             Code = "OAUTH_TIMEOUT"
	CodeAccountIneligible        Code = "ACCOUNT_INELIGIBLE"
	CodeConfigConflict           Code = "CONFIG_CONFLICT"
	CodeAGSchemaUnsupported      Code = "AG_SCHEMA_UNSUPPORTED"
	CodeAGPoolUnavailable        Code = "AG_POOL_UNAVAILABLE"
	CodeCodexPickerRouteMismatch Code = "CODEX_PICKER_ROUTE_MISMATCH"
	CodeCatalogVersionMismatch   Code = "CATALOG_VERSION_MISMATCH"
	CodePoolIsolationViolation   Code = "POOL_ISOLATION_VIOLATION"
	CodeRollbackConflict         Code = "ROLLBACK_CONFLICT"
	CodeSecretLeakDetected       Code = "SECRET_LEAK_DETECTED"
	CodeInvalidCommand           Code = "INVALID_COMMAND"
	CodeInvalidArgument          Code = "INVALID_ARGUMENT"
)

type Definition struct {
	Category Category
	Message  string
}

var registry = map[Code]Definition{
	CodePortInUse:                {CategoryService, "required port is already in use"},
	CodeProcessIdentityMismatch:  {CategorySecurity, "process identity does not match the expected instance"},
	CodeUpstreamVersionMismatch:  {CategoryCompatibility, "upstream binary does not match the pinned version"},
	CodeManagementUnavailable:    {CategoryService, "upstream management interface is unavailable"},
	CodeOAuthTimeout:             {CategoryAuth, "authentication did not complete before the deadline"},
	CodeAccountIneligible:        {CategoryAuth, "account does not provide the required capability"},
	CodeConfigConflict:           {CategoryConfig, "configuration changed since it was inspected"},
	CodeAGSchemaUnsupported:      {CategoryCompatibility, "Antigravity request schema is unsupported"},
	CodeAGPoolUnavailable:        {CategoryService, "Google pool has no usable route"},
	CodeCodexPickerRouteMismatch: {CategoryCompatibility, "Codex picker did not retain the configured provider"},
	CodeCatalogVersionMismatch:   {CategoryCompatibility, "catalog does not match the installed Codex version"},
	CodePoolIsolationViolation:   {CategorySecurity, "provider pool isolation was violated"},
	CodeRollbackConflict:         {CategoryConfig, "owned configuration changed after apply"},
	CodeSecretLeakDetected:       {CategorySecurity, "forbidden sensitive material was detected"},
	CodeInvalidCommand:           {CategoryUsage, "command is not recognized"},
	CodeInvalidArgument:          {CategoryUsage, "argument is invalid"},
}

func Registry() map[Code]Definition {
	result := make(map[Code]Definition, len(registry))
	for code, definition := range registry {
		result[code] = definition
	}
	return result
}

type Error struct {
	code     Code
	message  string
	category Category
	cause    error
}

func New(code Code, cause error) *Error {
	definition, ok := registry[code]
	if !ok {
		definition = registry[CodeInvalidArgument]
		code = CodeInvalidArgument
	}
	return &Error{code: code, message: definition.Message, category: definition.Category, cause: cause}
}

func (err *Error) Error() string      { return fmt.Sprintf("%s: %s", err.code, err.message) }
func (err *Error) Unwrap() error      { return err.cause }
func (err *Error) Code() Code         { return err.code }
func (err *Error) Category() Category { return err.category }

type Envelope struct {
	Code     Code   `json:"code"`
	Message  string `json:"message"`
	ExitCode int    `json:"exit_code"`
}

func (err *Error) Envelope() Envelope {
	return Envelope{Code: err.code, Message: err.message, ExitCode: err.category.ExitCode()}
}
