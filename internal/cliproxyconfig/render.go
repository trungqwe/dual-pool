package cliproxyconfig

import (
	"bytes"
	"crypto/subtle"
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
)

const header = "# managed-by: dualpool\n# config-adapter: " + AdapterVersion + "\n# upstream: CLIProxyAPI v" + UpstreamVersion + " " + UpstreamCommit + "\n"

func render(id ID, authDir string, clientWire, managementWire []byte) ([]byte, error) {
	return renderWithCost(id, authDir, clientWire, managementWire, bcrypt.DefaultCost)
}

func renderWithCost(id ID, authDir string, clientWire, managementWire []byte, cost int) ([]byte, error) {
	if !id.valid() || len(clientWire) != 43 || len(managementWire) != 43 {
		return nil, ErrConfigInvalid
	}
	return renderConfig(expectedConfig(id, authDir, string(clientWire), ""), managementWire, cost)
}

func renderConfig(value config, managementWire []byte, cost int) ([]byte, error) {
	hash, err := bcrypt.GenerateFromPassword(managementWire, cost)
	if err != nil {
		return nil, ErrConfigInvalid
	}
	defer zero(hash)
	value.RemoteManagement.SecretKey = string(hash)
	var node yaml.Node
	if err = yamlNode(value, &node); err != nil {
		return nil, ErrConfigInvalid
	}
	var out bytes.Buffer
	out.WriteString(header)
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err = encoder.Encode(&node); err != nil {
		return nil, ErrConfigInvalid
	}
	if err = encoder.Close(); err != nil {
		return nil, ErrConfigInvalid
	}
	return out.Bytes(), nil
}

// RenderCompatibilitySmoke renders the production adapter schema for an
// isolated loopback compatibility process. Only its ephemeral port, workspace
// (which determines auth-dir), and synthetic keys vary from the instance
// configuration. The upstream and adapter provenance remain fixed in header.
func RenderCompatibilitySmoke(port int, workspace string, clientWire, managementWire []byte) ([]byte, error) {
	if !validCompatibilitySmokePort(port) || !validCompatibilitySmokeWorkspace(workspace) || !validWireKey(clientWire) || !validWireKey(managementWire) || equal(clientWire, managementWire) {
		return nil, ErrConfigInvalid
	}
	value := expectedConfigAt(port, filepath.Join(workspace, "auth"), string(clientWire), "")
	return renderConfig(value, managementWire, bcrypt.DefaultCost)
}

func validCompatibilitySmokePort(port int) bool {
	return port >= 1024 && port <= 65535 && port != CodexPort && port != GooglePort
}

func validCompatibilitySmokeWorkspace(workspace string) bool {
	if workspace == "" || filepath.Clean(workspace) != workspace || !filepath.IsAbs(workspace) || strings.ContainsAny(workspace, "\x00\r\n") {
		return false
	}
	volume := filepath.VolumeName(workspace)
	if volume == "" || strings.HasPrefix(workspace, `\\`) || strings.EqualFold(filepath.Clean(workspace), volume+string(filepath.Separator)) {
		return false
	}
	return true
}

func validWireKey(value []byte) bool {
	if len(value) != 43 {
		return false
	}
	for _, b := range value {
		if !((b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '-' || b == '_') {
			return false
		}
	}
	return true
}

func yamlNode(v config, n *yaml.Node) error {
	b, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	if err = yaml.Unmarshal(b, n); err != nil {
		return err
	}
	if len(n.Content) != 1 {
		return ErrConfigInvalid
	}
	root := n.Content[0]
	for i := 0; i+1 < len(root.Content); i += 2 {
		key, val := root.Content[i], root.Content[i+1]
		if key.Value == "host" || key.Value == "auth-dir" {
			val.Style = yaml.DoubleQuotedStyle
		}
		if key.Value == "remote-management" {
			for j := 0; j+1 < len(val.Content); j += 2 {
				if val.Content[j].Value == "secret-key" {
					val.Content[j+1].Style = yaml.DoubleQuotedStyle
				}
			}
		}
		if key.Value == "api-keys" && len(val.Content) == 1 {
			val.Content[0].Style = yaml.DoubleQuotedStyle
		}
	}
	return nil
}

func validateConfig(data []byte, id ID, authDir string, clientWire, managementWire []byte, otherKeys [][]byte) error {
	return validateConfigWithCost(data, id, authDir, clientWire, managementWire, otherKeys, bcrypt.DefaultCost)
}

func validateConfigWithCost(data []byte, id ID, authDir string, clientWire, managementWire []byte, otherKeys [][]byte, expectedCost int) error {
	if !id.valid() || len(data) == 0 || len(data) > 16*1024 || !utf8.Valid(data) || !bytes.HasPrefix(data, []byte(header)) {
		return ErrConfigInvalid
	}
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil || len(node.Content) != 1 || invalidNode(&node) {
		return ErrConfigInvalid
	}
	if !exactShape(node.Content[0], rootShape) {
		return ErrConfigInvalid
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var actual config
	if err := decoder.Decode(&actual); err != nil {
		return ErrConfigInvalid
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return ErrConfigInvalid
	}
	if len(actual.APIKeys) != 1 || actual.APIKeys[0] != string(clientWire) || actual.RemoteManagement.SecretKey == "" {
		return ErrConfigInvalid
	}
	if !strings.HasPrefix(actual.RemoteManagement.SecretKey, "$2a$") && !strings.HasPrefix(actual.RemoteManagement.SecretKey, "$2b$") && !strings.HasPrefix(actual.RemoteManagement.SecretKey, "$2y$") {
		return ErrConfigInvalid
	}
	cost, err := bcrypt.Cost([]byte(actual.RemoteManagement.SecretKey))
	if err != nil || cost != expectedCost {
		return ErrConfigInvalid
	}
	if bcrypt.CompareHashAndPassword([]byte(actual.RemoteManagement.SecretKey), managementWire) != nil {
		return ErrConfigInvalid
	}
	if bcrypt.CompareHashAndPassword([]byte(actual.RemoteManagement.SecretKey), clientWire) == nil {
		return ErrConfigInvalid
	}
	for _, other := range otherKeys {
		if bcrypt.CompareHashAndPassword([]byte(actual.RemoteManagement.SecretKey), other) == nil {
			return ErrConfigInvalid
		}
	}
	if !reflect.DeepEqual(actual, expectedConfig(id, authDir, string(clientWire), actual.RemoteManagement.SecretKey)) {
		return ErrConfigInvalid
	}
	if bytes.Contains(data, managementWire) {
		return ErrConfigInvalid
	}
	for _, other := range otherKeys {
		if len(other) > 0 && bytes.Contains(data, other) {
			return ErrConfigInvalid
		}
	}
	return nil
}

func expectedConfigAt(port int, authDir, clientWire, managementHash string) config {
	value := expectedConfig(Codex, authDir, clientWire, managementHash)
	value.Port = port
	return value
}

var rootShape = map[string][]string{
	"host": nil, "port": nil, "tls": {"enable"},
	"remote-management": {"allow-remote", "secret-key", "disable-control-panel", "disable-auto-update-panel"},
	"auth-dir":          nil, "api-keys": nil, "debug": nil, "pprof": {"enable"}, "discovery": {"enabled"},
	"request-log": nil, "logging-to-file": nil, "usage-statistics-enabled": nil, "save-cooldown-status": nil,
	"passthrough-headers": nil, "ws-auth": nil,
	"routing": {"strategy", "session-affinity", "session-affinity-ttl", "session-affinity-subagents"},
}

func exactShape(node *yaml.Node, shape map[string][]string) bool {
	if node.Kind != yaml.MappingNode || len(node.Content) != 2*len(shape) {
		return false
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		nested, ok := shape[key.Value]
		if !ok {
			return false
		}
		if nested != nil {
			if value.Kind != yaml.MappingNode || len(value.Content) != 2*len(nested) {
				return false
			}
			for j := 0; j+1 < len(value.Content); j += 2 {
				found := false
				for _, name := range nested {
					if value.Content[j].Value == name {
						found = true
						break
					}
				}
				if !found || value.Content[j+1].Kind != yaml.ScalarNode {
					return false
				}
			}
		} else if key.Value != "api-keys" && value.Kind != yaml.ScalarNode {
			return false
		}
	}
	return true
}

func invalidNode(n *yaml.Node) bool {
	if n.Anchor != "" || n.Alias != nil || n.Kind == yaml.AliasNode {
		return true
	}
	if n.Kind == yaml.ScalarNode {
		switch n.Tag {
		case "!!str", "!!bool", "!!int":
		default:
			return true
		}
	}
	if n.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			k := n.Content[i]
			if k.Kind != yaml.ScalarNode || k.Tag != "!!str" || seen[k.Value] {
				return true
			}
			seen[k.Value] = true
		}
	}
	for _, child := range n.Content {
		if invalidNode(child) {
			return true
		}
	}
	return false
}

func zero(v []byte) {
	for i := range v {
		v[i] = 0
	}
}
func equal(a, b []byte) bool { return len(a) == len(b) && subtle.ConstantTimeCompare(a, b) == 1 }
