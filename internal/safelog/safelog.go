package safelog

import (
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/trungqwe/dual-pool/internal/apperr"
)

var errInvalidEvent = errors.New("invalid log metadata event")
var eventName = regexp.MustCompile(`^(process|health|oauth|account|route|affinity|config|update|test)\.[a-z][a-z0-9_]{0,63}$`)
var componentName = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Event contains metadata only. Field constructors and Log both enforce the
// closed schema; raw messages and arbitrary maps are not accepted.
type Event struct {
	Level     Level
	Component string
	Name      string
	Fields    []Field
}

type Logger struct {
	writer io.Writer
	now    func() time.Time
}

func New(writer io.Writer, now func() time.Time) Logger {
	if now == nil {
		now = time.Now
	}
	return Logger{writer: writer, now: now}
}

type record struct {
	Timestamp          string       `json:"timestamp"`
	Level              Level        `json:"level"`
	Component          string       `json:"component"`
	Event              string       `json:"event"`
	OperationID        *string      `json:"operation_id,omitempty"`
	InstanceID         *string      `json:"instance_id,omitempty"`
	Provider           *string      `json:"provider,omitempty"`
	RouteTemplate      *string      `json:"route_template,omitempty"`
	Method             *string      `json:"method,omitempty"`
	ModelID            *string      `json:"model_id,omitempty"`
	CredentialOpaqueID *string      `json:"credential_opaque_id,omitempty"`
	SessionFingerprint *string      `json:"session_fingerprint,omitempty"`
	StatusCode         *int64       `json:"status_code,omitempty"`
	DurationMS         *int64       `json:"duration_ms,omitempty"`
	RetryCount         *int64       `json:"retry_count,omitempty"`
	StreamStarted      *bool        `json:"stream_started,omitempty"`
	BytesClass         *string      `json:"bytes_class,omitempty"`
	ErrorCode          *apperr.Code `json:"error_code,omitempty"`
	Version            *string      `json:"version,omitempty"`
	ConfigHashPrefix   *string      `json:"config_hash_prefix,omitempty"`
}

func (logger Logger) Log(event Event) error {
	if logger.writer == nil || logger.now == nil || !validEvent(event) {
		return errInvalidEvent
	}
	result := record{
		Timestamp: eventTime(logger.now()), Level: event.Level,
		Component: event.Component, Event: event.Name,
	}
	seen := make(map[string]struct{}, len(event.Fields))
	for _, field := range event.Fields {
		if err := validateField(field); err != nil {
			return err
		}
		if _, exists := seen[field.name]; exists {
			return errInvalidField
		}
		seen[field.name] = struct{}{}
		assignField(&result, field)
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return errInvalidEvent
	}
	if serializedSensitive(payload) {
		return errInvalidEvent
	}
	payload = append(payload, '\n')
	n, err := logger.writer.Write(payload)
	if err != nil {
		return err
	}
	if n != len(payload) {
		return io.ErrShortWrite
	}
	return nil
}

func validEvent(event Event) bool {
	if event.Level != LevelDebug && event.Level != LevelInfo && event.Level != LevelWarn && event.Level != LevelError {
		return false
	}
	return componentName.MatchString(event.Component) && eventName.MatchString(event.Name) &&
		!sensitiveValue(event.Component) && !sensitiveValue(event.Name)
}

func eventTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func assignField(result *record, field Field) {
	switch field.name {
	case "operation_id":
		result.OperationID = &field.text
	case "instance_id":
		result.InstanceID = &field.text
	case "provider":
		result.Provider = &field.text
	case "route_template":
		result.RouteTemplate = &field.text
	case "method":
		result.Method = &field.text
	case "model_id":
		result.ModelID = &field.text
	case "credential_opaque_id":
		result.CredentialOpaqueID = &field.text
	case "session_fingerprint":
		result.SessionFingerprint = &field.text
	case "status_code":
		result.StatusCode = &field.number
	case "duration_ms":
		result.DurationMS = &field.number
	case "retry_count":
		result.RetryCount = &field.number
	case "stream_started":
		result.StreamStarted = &field.boolean
	case "bytes_class":
		result.BytesClass = &field.text
	case "error_code":
		result.ErrorCode = &field.code
	case "version":
		result.Version = &field.text
	case "config_hash_prefix":
		result.ConfigHashPrefix = &field.text
	}
}

func serializedSensitive(payload []byte) bool {
	text := strings.ToLower(string(payload))
	return strings.Contains(text, "sentinel_secret") || strings.Contains(text, "bearer") ||
		strings.Contains(text, "refresh-token") || strings.Contains(text, "raw-session") ||
		secretTokenPattern.Match(payload) || emailPattern.Match(payload) || localPathPattern.Match(payload)
}
