package safelog

import (
	"errors"
	"regexp"
	"strings"

	"github.com/trungqwe/dual-pool/internal/apperr"
)

var errInvalidField = errors.New("invalid log metadata field")

var opaqueID = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)
var credentialID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{5,63}$`)
var modelSlug = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
var safeVersion = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$`)
var routeShape = regexp.MustCompile(`^/[A-Za-z0-9_{}./:-]{1,127}$`)
var fingerprintHex = regexp.MustCompile(`^[a-f0-9]{32}$`)
var hashPrefixHex = regexp.MustCompile(`^[a-f0-9]{12}$`)
var emailPattern = regexp.MustCompile(`(?i)[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}`)
var localPathPattern = regexp.MustCompile(`(?i)[a-z]:[\\/]`)
var sensitivePattern = regexp.MustCompile(`(?i)bearer|authorization|cookie|token|secret|password|api[_-]?key|raw[-_]?session|oauth[_-]?state`)
var secretTokenPattern = regexp.MustCompile(`(?i)\b(?:sk|ghp)_[A-Za-z0-9._-]{16,}\b|\bsk-[A-Za-z0-9._-]{16,}\b`)

type fieldKind uint8

const (
	stringKind fieldKind = iota + 1
	numberKind
	boolKind
	errorKind
)

// Field is constructed through typed helpers; its members are private so callers
// cannot bypass the name/type allowlist.
type Field struct {
	name    string
	kind    fieldKind
	text    string
	number  int64
	boolean bool
	code    apperr.Code
}

func StringField(name, value string) (Field, error) {
	field := Field{name: name, kind: stringKind, text: value}
	if err := validateField(field); err != nil {
		return Field{}, err
	}
	return field, nil
}

func NumberField(name string, value int64) (Field, error) {
	field := Field{name: name, kind: numberKind, number: value}
	if err := validateField(field); err != nil {
		return Field{}, err
	}
	return field, nil
}

func BoolField(name string, value bool) (Field, error) {
	field := Field{name: name, kind: boolKind, boolean: value}
	if err := validateField(field); err != nil {
		return Field{}, err
	}
	return field, nil
}

func ErrorCodeField(code apperr.Code) (Field, error) {
	field := Field{name: "error_code", kind: errorKind, code: code}
	if err := validateField(field); err != nil {
		return Field{}, err
	}
	return field, nil
}

func validateField(field Field) error {
	switch field.kind {
	case stringKind:
		if !validStringField(field.name, field.text) {
			return errInvalidField
		}
	case numberKind:
		switch field.name {
		case "status_code":
			if field.number < 100 || field.number > 599 {
				return errInvalidField
			}
		case "duration_ms":
			if field.number < 0 || field.number > 86_400_000 {
				return errInvalidField
			}
		case "retry_count":
			if field.number < 0 || field.number > 100 {
				return errInvalidField
			}
		default:
			return errInvalidField
		}
	case boolKind:
		if field.name != "stream_started" {
			return errInvalidField
		}
	case errorKind:
		if field.name != "error_code" || !apperr.IsRegistered(field.code) {
			return errInvalidField
		}
	default:
		return errInvalidField
	}
	return nil
}

func validStringField(name, value string) bool {
	if value == "" || len(value) > 128 || sensitiveValue(value) {
		return false
	}
	switch name {
	case "operation_id", "instance_id":
		return opaqueID.MatchString(value)
	case "provider":
		return value == "google" || value == "codex"
	case "route_template":
		return routeShape.MatchString(value) && !strings.HasPrefix(value, "//") && !strings.Contains(value, "..")
	case "method":
		return value == "GET" || value == "POST" || value == "PUT" || value == "PATCH" || value == "DELETE"
	case "model_id":
		return modelSlug.MatchString(value)
	case "credential_opaque_id":
		return credentialID.MatchString(value)
	case "session_fingerprint":
		return fingerprintHex.MatchString(value)
	case "bytes_class":
		return value == "empty" || value == "tiny" || value == "small" || value == "medium" || value == "large" || value == "oversize"
	case "version":
		return safeVersion.MatchString(value)
	case "config_hash_prefix":
		return hashPrefixHex.MatchString(value)
	default:
		return false
	}
}

func sensitiveValue(value string) bool {
	return sensitivePattern.MatchString(value) || secretTokenPattern.MatchString(value) || emailPattern.MatchString(value) || localPathPattern.MatchString(value) || strings.Contains(value, `\\`) || strings.ContainsAny(value, "\r\n\x00")
}
