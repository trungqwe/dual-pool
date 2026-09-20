package cliproxyconfig

import (
	"errors"
	"github.com/trungqwe/dual-pool/internal/upstreamlock"
)

const (
	AdapterVersion  = upstreamlock.ConfigAdapterVersion
	UpstreamVersion = "7.3.7"
	UpstreamCommit  = "b773607e3e7756dc6020a291825e4eb08899595a"
	CodexPort       = 8317
	GooglePort      = 8318
)

var (
	ErrUnsupportedAdapter     = errors.New("unsupported CLIProxyAPI config adapter")
	ErrInvalidKeyMaterial     = errors.New("invalid product key material")
	ErrConfigConflict         = errors.New("instance config conflict")
	ErrConfigInvalid          = errors.New("instance config invalid")
	ErrUnsafeInstanceArtifact = errors.New("unsafe instance artifact")
	ErrRecoveryUnresolved     = errors.New("instance recovery unresolved")
	ErrPersistence            = errors.New("instance config persistence failed")
)

type ID string

const (
	Codex  ID = "codex"
	Google ID = "google"
)

func (id ID) valid() bool { return id == Codex || id == Google }
func (id ID) port() int {
	if id == Codex {
		return CodexPort
	}
	return GooglePort
}

type config struct {
	Host                   string           `yaml:"host"`
	Port                   int              `yaml:"port"`
	TLS                    tlsConfig        `yaml:"tls"`
	RemoteManagement       managementConfig `yaml:"remote-management"`
	AuthDir                string           `yaml:"auth-dir"`
	APIKeys                []string         `yaml:"api-keys"`
	Debug                  bool             `yaml:"debug"`
	Pprof                  pprofConfig      `yaml:"pprof"`
	Discovery              discoveryConfig  `yaml:"discovery"`
	RequestLog             bool             `yaml:"request-log"`
	LoggingToFile          bool             `yaml:"logging-to-file"`
	UsageStatisticsEnabled bool             `yaml:"usage-statistics-enabled"`
	SaveCooldownStatus     bool             `yaml:"save-cooldown-status"`
	PassthroughHeaders     bool             `yaml:"passthrough-headers"`
	WSAuth                 bool             `yaml:"ws-auth"`
	Routing                routingConfig    `yaml:"routing"`
}
type tlsConfig struct {
	Enable bool `yaml:"enable"`
}
type managementConfig struct {
	AllowRemote            bool   `yaml:"allow-remote"`
	SecretKey              string `yaml:"secret-key"`
	DisableControlPanel    bool   `yaml:"disable-control-panel"`
	DisableAutoUpdatePanel bool   `yaml:"disable-auto-update-panel"`
}
type pprofConfig struct {
	Enable bool `yaml:"enable"`
}
type discoveryConfig struct {
	Enabled bool `yaml:"enabled"`
}
type routingConfig struct {
	Strategy                 string `yaml:"strategy"`
	SessionAffinity          bool   `yaml:"session-affinity"`
	SessionAffinityTTL       string `yaml:"session-affinity-ttl"`
	SessionAffinitySubagents bool   `yaml:"session-affinity-subagents"`
}

func expectedConfig(id ID, authDir, clientWire, managementHash string) config {
	return config{
		Host: "127.0.0.1", Port: id.port(), TLS: tlsConfig{},
		RemoteManagement: managementConfig{SecretKey: managementHash, DisableControlPanel: true, DisableAutoUpdatePanel: true},
		AuthDir:          authDir, APIKeys: []string{clientWire}, WSAuth: true,
		Routing: routingConfig{Strategy: "round-robin", SessionAffinity: true, SessionAffinityTTL: "1h", SessionAffinitySubagents: true},
	}
}
