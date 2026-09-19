package state

const (
	StateSchemaVersion        = 1
	OwnershipSchemaVersion    = 1
	MaxStateDocumentBytes     = 1 << 20
	MaxOwnershipDocumentBytes = 4 << 20
)

type Pool string

const (
	PoolCodex  Pool = "codex"
	PoolGoogle Pool = "google"
)

type Eligibility string

const (
	EligibilityUnknown    Eligibility = "unknown"
	EligibilityEligible   Eligibility = "eligible"
	EligibilityIneligible Eligibility = "ineligible"
)

type InstanceStatus string

const InstanceStopped InstanceStatus = "stopped"

type AntigravityMode string

const (
	ModeDisabled    AntigravityMode = "disabled"
	ModeObserve     AntigravityMode = "observe"
	ModePassthrough AntigravityMode = "passthrough"
	ModeOverride    AntigravityMode = "override"
	ModeDegraded    AntigravityMode = "degraded"
)

type Instance struct {
	Port       int            `json:"port"`
	ConfigHash string         `json:"config_hash"`
	Status     InstanceStatus `json:"status"`
}
type Instances struct {
	Codex  Instance `json:"codex"`
	Google Instance `json:"google"`
}
type Antigravity struct {
	Mode               AntigravityMode `json:"mode"`
	BridgePort         int             `json:"bridge_port"`
	DonorModelID       string          `json:"donor_model_id"`
	CatalogFingerprint string          `json:"catalog_fingerprint"`
	AdapterVersion     string          `json:"adapter_version"`
}
type Codex struct {
	AdapterVersion     string `json:"adapter_version"`
	CatalogFingerprint string `json:"catalog_fingerprint"`
}
type Account struct {
	OpaqueID         string      `json:"opaque_id"`
	Pool             Pool        `json:"pool"`
	Nickname         string      `json:"nickname"`
	Enabled          bool        `json:"enabled"`
	Eligibility      Eligibility `json:"eligibility"`
	ReasonCode       string      `json:"reason_code"`
	ModelIDs         []string    `json:"model_ids"`
	LastProbeAt      string      `json:"last_probe_at"`
	LastProbeVersion string      `json:"last_probe_version"`
}
type State struct {
	SchemaVersion         int         `json:"schema_version"`
	InstallID             string      `json:"install_id"`
	ActiveUpstreamVersion string      `json:"active_upstream_version"`
	Instances             Instances   `json:"instances"`
	Antigravity           Antigravity `json:"antigravity"`
	Codex                 Codex       `json:"codex"`
	Accounts              []Account   `json:"accounts"`
}
