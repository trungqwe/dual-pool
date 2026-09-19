package state

type ConfigFormat string

const (
	FormatTOML ConfigFormat = "toml"
	FormatJSON ConfigFormat = "json"
)

type Encoding string

const EncodingUTF8 Encoding = "utf-8"

type BOM string

const (
	BOMAbsent  BOM = "absent"
	BOMPresent BOM = "present"
)

type Newline string

const (
	NewlineLF   Newline = "lf"
	NewlineCRLF Newline = "crlf"
)

type RollbackStatus string

const (
	RollbackPending    RollbackStatus = "pending"
	RollbackApplied    RollbackStatus = "applied"
	RollbackRolledBack RollbackStatus = "rolled_back"
	RollbackConflict   RollbackStatus = "conflict"
)

type ValueKind string

const (
	ValueAbsent    ValueKind = "absent"
	ValueString    ValueKind = "string"
	ValueBool      ValueKind = "bool"
	ValueInteger   ValueKind = "integer"
	ValueNumber    ValueKind = "number"
	ValueStringMap ValueKind = "string_map"
	ValueSecretRef ValueKind = "secret_ref"
)

type TypedValue struct {
	Kind      ValueKind         `json:"kind"`
	String    *string           `json:"string,omitempty"`
	Bool      *bool             `json:"bool,omitempty"`
	Integer   *int64            `json:"integer,omitempty"`
	Number    *float64          `json:"number,omitempty"`
	StringMap map[string]string `json:"string_map,omitempty"`
	SecretRef *string           `json:"secret_ref,omitempty"`
}
type OwnershipRecord struct {
	TargetPath      string         `json:"target_path"`
	Format          ConfigFormat   `json:"format"`
	Encoding        Encoding       `json:"encoding"`
	BOM             BOM            `json:"bom"`
	Newline         Newline        `json:"newline"`
	FileIdentity    string         `json:"file_identity"`
	PreWriteHash    string         `json:"pre_write_hash"`
	KeyPath         string         `json:"key_path"`
	OriginalExisted bool           `json:"original_existed"`
	OriginalValue   TypedValue     `json:"original_value"`
	AppliedValue    TypedValue     `json:"applied_value"`
	BackupPath      string         `json:"backup_path"`
	BackupHash      string         `json:"backup_hash"`
	AppliedAt       string         `json:"applied_at"`
	ToolVersion     string         `json:"tool_version"`
	PostWriteHash   string         `json:"post_write_hash"`
	RollbackStatus  RollbackStatus `json:"rollback_status"`
}
type Ownership struct {
	SchemaVersion int               `json:"schema_version"`
	Records       []OwnershipRecord `json:"records"`
}
