package cliproxyconfig

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/trungqwe/dual-pool/internal/secretstore"
)

var txnPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

type marker struct {
	SchemaVersion        int    `json:"schema_version"`
	InstanceID           ID     `json:"instance_id"`
	TransactionID        string `json:"transaction_id"`
	CandidateBasename    string `json:"candidate_basename"`
	ConfigAdapterVersion string `json:"config_adapter_version"`
	UpstreamVersion      string `json:"upstream_version"`
	UpstreamCommit       string `json:"upstream_commit"`
}

func makeMarker(id ID, txn string) marker {
	return marker{1, id, txn, "." + string(id) + ".init-" + txn, AdapterVersion, UpstreamVersion, UpstreamCommit}
}
func (m marker) valid(id ID) bool {
	return m == makeMarker(id, m.TransactionID) && txnPattern.MatchString(m.TransactionID)
}
func markerBytes(m marker) []byte { b, _ := json.Marshal(m); return append(b, '\n') }

func (g *Generator) readMarker(id ID) (marker, bool, error) {
	path := g.markerPath(id)
	if !exists(path) {
		return marker{}, false, nil
	}
	if g.acl.InspectFile(path) != nil {
		return marker{}, false, ErrRecoveryUnresolved
	}
	b, err := os.ReadFile(path)
	if err != nil || len(b) > 1024 {
		return marker{}, false, ErrRecoveryUnresolved
	}
	var m marker
	if json.Unmarshal(b, &m) != nil || !m.valid(id) || !bytes.Equal(b, markerBytes(m)) {
		return marker{}, false, ErrRecoveryUnresolved
	}
	return m, true, nil
}

func (g *Generator) writeMarker(m marker) error {
	if !m.valid(m.InstanceID) {
		return ErrRecoveryUnresolved
	}
	f, err := g.acl.CreateFile(g.markerPath(m.InstanceID))
	if err != nil {
		return ErrRecoveryUnresolved
	}
	b := markerBytes(m)
	n, err := f.Write(b)
	if err != nil || n != len(b) {
		_ = f.Close()
		return ErrPersistence
	}
	if f.Sync() != nil {
		_ = f.Close()
		return ErrPersistence
	}
	if f.Close() != nil {
		return ErrPersistence
	}
	return nil
}

func (g *Generator) recover(id ID, keys *material) (bool, error) {
	m, present, err := g.readMarker(id)
	if err != nil || !present {
		return false, err
	}
	final := g.final(id)
	candidate := filepath.Join(g.layout.Instances, m.CandidateBasename)
	finalExists, candidateExists := exists(final), exists(candidate)
	if finalExists && g.inspectInstance(final, id, keys, false) != nil {
		return false, ErrRecoveryUnresolved
	}
	if candidateExists {
		if g.inspectCandidateForCleanup(candidate, id, keys) != nil {
			return false, ErrRecoveryUnresolved
		}
		if err = os.RemoveAll(candidate); err != nil {
			return false, ErrRecoveryUnresolved
		}
	}
	if err = os.Remove(g.markerPath(id)); err != nil {
		return false, ErrRecoveryUnresolved
	}
	return finalExists, nil
}

func (g *Generator) inspectCandidateForCleanup(root string, id ID, keys *material) error {
	if g.acl.Inspect(root) != nil {
		return ErrRecoveryUnresolved
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return ErrRecoveryUnresolved
	}
	seen := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if seen[name] {
			return ErrRecoveryUnresolved
		}
		seen[name] = true
		path := filepath.Join(root, name)
		switch name {
		case "config.yaml":
			if e.IsDir() || g.acl.InspectFile(path) != nil {
				return ErrRecoveryUnresolved
			}
			b, readErr := os.ReadFile(path)
			if readErr != nil {
				return ErrRecoveryUnresolved
			}
			base := 0
			if id == Google {
				base = 2
			}
			var other [][]byte
			if id == Codex {
				other = [][]byte{keys.wire[2], keys.wire[3]}
			} else {
				other = [][]byte{keys.wire[0], keys.wire[1]}
			}
			validation := validateConfigWithCost(b, id, g.auth(id), keys.wire[base], keys.wire[base+1], other, g.bcryptCost)
			for _, raw := range keys.raw {
				if bytes.Contains(b, raw) {
					validation = ErrRecoveryUnresolved
				}
			}
			secretstore.Zero(b)
			if validation != nil {
				return ErrRecoveryUnresolved
			}
		case "auth", "logs":
			if !e.IsDir() || g.acl.Inspect(path) != nil {
				return ErrRecoveryUnresolved
			}
			children, readErr := os.ReadDir(path)
			if readErr != nil || len(children) != 0 {
				return ErrRecoveryUnresolved
			}
		default:
			return ErrRecoveryUnresolved
		}
	}
	return nil
}

func (g *Generator) create(id ID, keys *material) error {
	if validateInstanceID(id) != nil {
		return ErrConfigConflict
	}
	txn, err := randomTxn()
	if err != nil {
		return err
	}
	m := makeMarker(id, txn)
	if err = g.writeMarker(m); err != nil {
		return err
	}
	if err = g.hit(AfterMarkerSync); err != nil {
		return err
	}
	candidate := filepath.Join(g.layout.Instances, m.CandidateBasename)
	if err = g.acl.Create(candidate); err != nil {
		return ErrUnsafeInstanceArtifact
	}
	if err = g.hit(AfterAttemptDirCreate); err != nil {
		return err
	}
	base := 0
	if id == Google {
		base = 2
	}
	b, err := renderWithCost(id, g.auth(id), keys.wire[base], keys.wire[base+1], g.bcryptCost)
	if err != nil {
		return err
	}
	defer secretstore.Zero(b)
	f, err := g.acl.CreateFile(filepath.Join(candidate, "config.yaml"))
	if err != nil {
		return ErrUnsafeInstanceArtifact
	}
	n, err := f.Write(b)
	if err != nil || n != len(b) {
		_ = f.Close()
		return ErrPersistence
	}
	if f.Sync() != nil {
		_ = f.Close()
		return ErrPersistence
	}
	if f.Close() != nil {
		return ErrPersistence
	}
	if err = g.hit(AfterConfigSync); err != nil {
		return err
	}
	if err = g.acl.Create(filepath.Join(candidate, "auth")); err != nil {
		return ErrUnsafeInstanceArtifact
	}
	if err = g.hit(AfterAuthDirCreate); err != nil {
		return err
	}
	if err = g.acl.Create(filepath.Join(candidate, "logs")); err != nil {
		return ErrUnsafeInstanceArtifact
	}
	if err = g.hit(AfterLogsDirCreate); err != nil {
		return err
	}
	if err := g.inspectInstance(candidate, id, keys, true); err != nil {
		return err
	}
	if err = g.hit(AfterAttemptVerify); err != nil {
		return err
	}
	if err = g.hit(BeforeInstanceInstall); err != nil {
		return err
	}
	if err = moveNoReplace(candidate, g.final(id)); err != nil {
		return err
	}
	if err = g.hit(AfterInstanceInstall); err != nil {
		return err
	}
	if err := g.inspectInstance(g.final(id), id, keys, false); err != nil {
		return err
	}
	if err = g.hit(AfterFinalVerify); err != nil {
		return err
	}
	if err = g.hit(BeforeMarkerCleanup); err != nil {
		return err
	}
	if err = os.Remove(g.markerPath(id)); err != nil {
		return ErrRecoveryUnresolved
	}
	return nil
}

func (g *Generator) inspectInstancesTop() error {
	allowed := map[string]bool{"codex": true, "google": true}
	for _, id := range []ID{Codex, Google} {
		m, present, err := g.readMarker(id)
		if err != nil {
			return err
		}
		if present {
			allowed[filepath.Base(g.markerPath(id))] = true
			allowed[m.CandidateBasename] = true
		}
	}
	entries, err := os.ReadDir(g.layout.Instances)
	if err != nil {
		return ErrUnsafeInstanceArtifact
	}
	for _, e := range entries {
		if !allowed[e.Name()] || strings.EqualFold(e.Name(), "codex") && e.Name() != "codex" || strings.EqualFold(e.Name(), "google") && e.Name() != "google" {
			return ErrConfigConflict
		}
	}
	return nil
}
