package state

import (
	"errors"
	"math"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidDocument          = errors.New("invalid state document")
	ErrUnsupportedSchemaVersion = errors.New("unsupported schema version")
	ErrMigrationUnavailable     = errors.New("migration unavailable")
	ErrMigrationInvalid         = errors.New("invalid migration result")
	opaquePattern               = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{5,127}$`)
	modelPattern                = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
	symbolPattern               = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)
	versionPattern              = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$`)
	hashPattern                 = regexp.MustCompile(`^[a-f0-9]{64}$`)
	fingerprintPattern          = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
	keyPathPattern              = regexp.MustCompile(`^[A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)*$`)
	sensitiveTextPattern        = regexp.MustCompile(`(?i)\b(?:bearer|authorization|cookie|refresh[-_ ]?token|raw[-_ ]?session|password|client[-_ ]?secret)\b|\b(?:sk|ghp)[-_][A-Za-z0-9._-]{16,}\b|[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}|(^|[^A-Za-z])[A-Za-z]:[\\/]`)
)

func ValidateState(s State) error {
	if s.SchemaVersion != StateSchemaVersion {
		return ErrUnsupportedSchemaVersion
	}
	if !opaquePattern.MatchString(s.InstallID) || !safeVersionOrEmpty(s.ActiveUpstreamVersion) {
		return ErrInvalidDocument
	}
	if err := validateInstance(s.Instances.Codex); err != nil {
		return err
	}
	if err := validateInstance(s.Instances.Google); err != nil {
		return err
	}
	if s.Instances.Codex.Port == s.Instances.Google.Port || !validPort(s.Antigravity.BridgePort) || s.Antigravity.BridgePort == s.Instances.Codex.Port || s.Antigravity.BridgePort == s.Instances.Google.Port {
		return ErrInvalidDocument
	}
	if !oneOf(string(s.Antigravity.Mode), "disabled", "observe", "passthrough", "override", "degraded") || !safeModelOrEmpty(s.Antigravity.DonorModelID) || !safeFingerprintOrEmpty(s.Antigravity.CatalogFingerprint) || !safeVersionOrEmpty(s.Antigravity.AdapterVersion) || !safeVersionOrEmpty(s.Codex.AdapterVersion) || !safeFingerprintOrEmpty(s.Codex.CatalogFingerprint) {
		return ErrInvalidDocument
	}
	seen := map[string]bool{}
	for _, a := range s.Accounts {
		if err := validateAccount(a); err != nil {
			return err
		}
		k := string(a.Pool) + "\x00" + a.OpaqueID
		if seen[k] {
			return ErrInvalidDocument
		}
		seen[k] = true
	}
	return nil
}
func validateInstance(i Instance) error {
	if !validPort(i.Port) || (i.ConfigHash != "" && !hashPattern.MatchString(i.ConfigHash)) || i.Status != InstanceStopped {
		return ErrInvalidDocument
	}
	return nil
}
func validateAccount(a Account) error {
	if !opaquePattern.MatchString(a.OpaqueID) || !oneOf(string(a.Pool), "codex", "google") || !oneOf(string(a.Eligibility), "unknown", "eligible", "ineligible") || !validNickname(a.Nickname) || sensitiveTextPattern.MatchString(a.Nickname) || (a.ReasonCode != "" && !symbolPattern.MatchString(a.ReasonCode)) || !safeVersionOrEmpty(a.LastProbeVersion) {
		return ErrInvalidDocument
	}
	if a.LastProbeAt != "" {
		if _, err := time.Parse(time.RFC3339, a.LastProbeAt); err != nil {
			return ErrInvalidDocument
		}
	}
	seen := map[string]bool{}
	for _, m := range a.ModelIDs {
		if !modelPattern.MatchString(m) || seen[m] {
			return ErrInvalidDocument
		}
		seen[m] = true
	}
	return nil
}

func ValidateOwnership(o Ownership) error {
	if o.SchemaVersion != OwnershipSchemaVersion {
		return ErrUnsupportedSchemaVersion
	}
	seen := map[string]bool{}
	for _, r := range o.Records {
		if err := validateRecord(r); err != nil {
			return err
		}
		k := strings.ToLower(r.TargetPath) + "\x00" + r.KeyPath
		if seen[k] {
			return ErrInvalidDocument
		}
		seen[k] = true
	}
	return nil
}
func validateRecord(r OwnershipRecord) error {
	if !validWindowsPath(r.TargetPath) || !validWindowsPath(r.BackupPath) || !oneOf(string(r.Format), "toml", "json") || r.Encoding != EncodingUTF8 || !oneOf(string(r.BOM), "absent", "present") || !oneOf(string(r.Newline), "lf", "crlf") || !opaquePattern.MatchString(r.FileIdentity) || !keyPathPattern.MatchString(r.KeyPath) || !hashPattern.MatchString(r.PreWriteHash) || !hashPattern.MatchString(r.BackupHash) || !hashPattern.MatchString(r.PostWriteHash) || !safeVersionOrEmpty(r.ToolVersion) || r.ToolVersion == "" || !oneOf(string(r.RollbackStatus), "pending", "applied", "rolled_back", "conflict") {
		return ErrInvalidDocument
	}
	if _, err := time.Parse(time.RFC3339, r.AppliedAt); err != nil {
		return ErrInvalidDocument
	}
	if err := validateValue(r.OriginalValue); err != nil {
		return err
	}
	if err := validateValue(r.AppliedValue); err != nil {
		return err
	}
	if (!r.OriginalExisted) != (r.OriginalValue.Kind == ValueAbsent) {
		return ErrInvalidDocument
	}
	return nil
}
func validateValue(v TypedValue) error {
	n := 0
	if v.String != nil {
		n++
	}
	if v.Bool != nil {
		n++
	}
	if v.Integer != nil {
		n++
	}
	if v.Number != nil {
		n++
	}
	if v.StringMap != nil {
		n++
	}
	if v.SecretRef != nil {
		n++
	}
	switch v.Kind {
	case ValueAbsent:
		if n != 0 {
			return ErrInvalidDocument
		}
	case ValueString:
		if n != 1 || v.String == nil || !validText(*v.String, 4096) || sensitiveTextPattern.MatchString(*v.String) {
			return ErrInvalidDocument
		}
	case ValueBool:
		if n != 1 || v.Bool == nil {
			return ErrInvalidDocument
		}
	case ValueInteger:
		if n != 1 || v.Integer == nil {
			return ErrInvalidDocument
		}
	case ValueNumber:
		if n != 1 || v.Number == nil || math.IsInf(*v.Number, 0) || math.IsNaN(*v.Number) {
			return ErrInvalidDocument
		}
	case ValueStringMap:
		if n != 1 || v.StringMap == nil || len(v.StringMap) > 64 {
			return ErrInvalidDocument
		}
		for k, x := range v.StringMap {
			if !keyPathPattern.MatchString(k) || !validText(x, 4096) || sensitiveTextPattern.MatchString(x) {
				return ErrInvalidDocument
			}
		}
	case ValueSecretRef:
		if n != 1 || v.SecretRef == nil || !opaquePattern.MatchString(*v.SecretRef) {
			return ErrInvalidDocument
		}
	default:
		return ErrInvalidDocument
	}
	return nil
}
func validWindowsPath(s string) bool {
	if len(s) < 4 || len(s) > 1024 || s[1] != ':' || s[2] != '\\' || !asciiLetter(s[0]) || strings.ContainsRune(s, 0) || strings.Contains(s, "/") || strings.Contains(s[3:], `\\`) || strings.ContainsAny(s[3:], `<>:"|?*`) || filepath.Clean(s) != s {
		return false
	}
	for _, p := range strings.FieldsFunc(s[3:], func(r rune) bool { return r == '\\' || r == '/' }) {
		if p == "." || p == ".." || p == "" || strings.HasSuffix(p, ".") || strings.HasSuffix(p, " ") || reservedWindowsName(p) {
			return false
		}
	}
	return true
}
func validNickname(s string) bool { return validText(s, 128) }
func validText(s string, max int) bool {
	if len(s) > max || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r == 0 || r == '\r' || r == '\n' || r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}
func validPort(p int) bool                 { return p >= 1 && p <= 65535 }
func safeVersionOrEmpty(s string) bool     { return s == "" || ValidLogicalVersion(s) }
func safeModelOrEmpty(s string) bool       { return s == "" || modelPattern.MatchString(s) }
func safeFingerprintOrEmpty(s string) bool { return s == "" || fingerprintPattern.MatchString(s) }
func oneOf(v string, x ...string) bool {
	for _, s := range x {
		if v == s {
			return true
		}
	}
	return false
}

// ValidLogicalVersion is the closed syntax shared by state selection,
// installed-slot registration and process-record binding. It is deliberately
// stricter than a generic semver parser because the value becomes a Windows
// directory component under the trusted product root.
func ValidLogicalVersion(s string) bool {
	if s == "" || !versionPattern.MatchString(s) || strings.ContainsAny(s, `\\/:*?"<>|`) || strings.ContainsAny(s, " \t\r\n\x00") || s == "." || s == ".." || strings.HasSuffix(s, ".") || strings.HasSuffix(s, " ") {
		return false
	}
	name := strings.ToUpper(strings.SplitN(s, ".", 2)[0])
	if name == "CON" || name == "PRN" || name == "AUX" || name == "NUL" || name == "CLOCK$" || (len(name) == 4 && (strings.HasPrefix(name, "COM") || strings.HasPrefix(name, "LPT")) && name[3] >= '1' && name[3] <= '9') {
		return false
	}
	return true
}
func asciiLetter(b byte) bool { return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' }
func reservedWindowsName(part string) bool {
	name := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
	if name == "CON" || name == "PRN" || name == "AUX" || name == "NUL" || name == "CLOCK$" {
		return true
	}
	return len(name) == 4 && (strings.HasPrefix(name, "COM") || strings.HasPrefix(name, "LPT")) && name[3] >= '1' && name[3] <= '9'
}
