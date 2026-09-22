//go:build windows

package instance

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/trungqwe/dual-pool/internal/cliproxyconfig"
	"github.com/trungqwe/dual-pool/internal/installedslot"
	"github.com/trungqwe/dual-pool/internal/keymaterial"
	"github.com/trungqwe/dual-pool/internal/secretstore"
	"github.com/trungqwe/dual-pool/internal/state"
	"github.com/trungqwe/dual-pool/internal/update"
	"github.com/trungqwe/dual-pool/internal/upstreamcatalog"
)

var ErrCompatibilitySmoke = errors.New("CLIProxyAPI compatibility smoke failed")

const (
	smokeRootName       = "compat-smoke"
	smokeAuthName       = "auth"
	smokeConfigName     = "config.yaml"
	smokeHealthPath     = "/healthz"
	smokeClientPath     = "/v1/models"
	smokeManagementPath = "/v0/management/debug"
	smokeTimeout        = 12 * time.Second
	smokeBodyLimit      = 4096
)

type smokeProcess interface {
	PID() uint32
	Kill() error
	Wait() error
}

type smokeRequester func(context.Context, int, string, string, string) (int, []byte, error)

// smokeDependencies are private fixture seams. Production instances use the
// Manager's process inspector, Windows TCP table, cryptographic randomness,
// direct exec launcher, loopback-only HTTP requester, and ephemeral allocator.
type smokeDependencies struct {
	preflightConfigPair func() error
	random              io.Reader
	choosePort          func() (int, error)
	launch              func(string, []string, string) (smokeProcess, error)
	listeners           func() ([]listener, error)
	request             smokeRequester
}

type updaterSmoke struct {
	manager *Manager
	deps    smokeDependencies
}

var _ update.Smoke = (*updaterSmoke)(nil)

func newUpdaterSmoke(manager *Manager) *updaterSmoke {
	return &updaterSmoke{manager: manager, deps: smokeDependencies{
		random: rand.Reader, choosePort: chooseSmokePort, launch: launchSmokeProcess,
		listeners: tcpListeners, request: requestLoopback,
	}}
}

func (s *updaterSmoke) Disposable(ctx context.Context, version string) error {
	if s == nil || s.manager == nil || ctx == nil || ctx.Err() != nil {
		return ErrCompatibilitySmoke
	}
	provenance, slot, err := s.resolveTrustedSlot(ctx, version)
	if err != nil {
		return err
	}
	return s.disposableResolved(ctx, provenance, slot)
}

func (s *updaterSmoke) resolveTrustedSlot(ctx context.Context, version string) (upstreamcatalog.Provenance, installedslot.ResolvedSlot, error) {
	m := s.manager
	provenance, err := m.trustedOperationalProvenance(version)
	if err != nil || m.registry == nil {
		return upstreamcatalog.Provenance{}, installedslot.ResolvedSlot{}, ErrCompatibilitySmoke
	}
	if err = m.registry.VerifyInstalled(ctx, version); err != nil {
		return upstreamcatalog.Provenance{}, installedslot.ResolvedSlot{}, ErrCompatibilitySmoke
	}
	slot, err := m.registry.Resolve(version)
	if err != nil || !slotMatchesProvenance(slot, provenance) {
		return upstreamcatalog.Provenance{}, installedslot.ResolvedSlot{}, ErrCompatibilitySmoke
	}
	expectedPath := filepath.Join(m.layout.Bin, "cliproxyapi", version, "cliproxyapi.exe")
	if !samePath(slot.ExecutablePath, expectedPath) || digest(slot.ExecutablePath) != provenance.ExecutableSHA256 {
		return upstreamcatalog.Provenance{}, installedslot.ResolvedSlot{}, ErrCompatibilitySmoke
	}
	return provenance, slot, nil
}

func slotMatchesProvenance(slot installedslot.ResolvedSlot, provenance upstreamcatalog.Provenance) bool {
	return slot.Version == provenance.Version && slot.Tag == provenance.Tag && slot.Commit == provenance.Commit &&
		slot.Platform == provenance.Platform && slot.ExecutableSHA256 == provenance.ExecutableSHA256 &&
		slot.UpstreamLockSHA256 == provenance.Digest && slot.ConfigAdapterVersion == provenance.ConfigAdapterVersion &&
		slot.ExecutableBasename == "cliproxyapi.exe"
}

func (s *updaterSmoke) disposableResolved(parent context.Context, provenance upstreamcatalog.Provenance, slot installedslot.ResolvedSlot) (resultErr error) {
	m := s.manager
	if parent == nil || m == nil || m.acl == nil || m.inspector == nil || m.layout.State == "" || !slotMatchesProvenance(slot, provenance) {
		return ErrCompatibilitySmoke
	}
	deps := s.dependencies()
	if deps.random == nil || deps.choosePort == nil || deps.launch == nil || deps.listeners == nil || deps.request == nil {
		return ErrCompatibilitySmoke
	}
	ctx, cancel := context.WithTimeout(parent, smokeTimeout)
	defer cancel()

	root := filepath.Join(m.layout.State, smokeRootName)
	if !pathWithin(m.layout.State, root) || m.acl.Create(root) != nil || m.acl.Inspect(root) != nil || isFileReparse(root) {
		return ErrCompatibilitySmoke
	}
	stale, err := os.ReadDir(root)
	if err != nil || len(stale) != 0 {
		return ErrCompatibilitySmoke
	}
	attempt, err := s.createAttempt(deps.random, root)
	if err != nil {
		return ErrCompatibilitySmoke
	}
	ownedAttempt := true
	var child smokeProcess
	var port int
	defer func() {
		cleanupErr := s.cleanupAttempt(child, port, attempt, ownedAttempt, deps)
		if cleanupErr != nil {
			if resultErr == nil {
				resultErr = errors.Join(ErrCompatibilitySmoke, cleanupErr)
			} else {
				resultErr = errors.Join(resultErr, cleanupErr)
			}
		}
	}()

	keys, err := newSmokeKeys(deps.random)
	if err != nil {
		return ErrCompatibilitySmoke
	}
	defer keys.wipe()

	port, err = deps.choosePort()
	if err != nil || !safeSmokePort(port) {
		return ErrCompatibilitySmoke
	}
	if err = m.acl.Create(filepath.Join(attempt, smokeAuthName)); err != nil || m.acl.Inspect(filepath.Join(attempt, smokeAuthName)) != nil || isFileReparse(filepath.Join(attempt, smokeAuthName)) {
		return ErrCompatibilitySmoke
	}
	configPath := filepath.Join(attempt, smokeConfigName)
	config, err := cliproxyconfig.RenderCompatibilitySmoke(port, attempt, keys.clientWire, keys.managementWire)
	if err != nil {
		return ErrCompatibilitySmoke
	}
	if err = writeSmokeConfig(m.acl, configPath, config); err != nil {
		zeroSmokeBytes(config)
		return ErrCompatibilitySmoke
	}
	zeroSmokeBytes(config)

	// The slot was verified at entry; rehash immediately before execution so
	// no caller-supplied path or post-verification replacement is accepted.
	if digest(slot.ExecutablePath) != provenance.ExecutableSHA256 {
		return ErrCompatibilitySmoke
	}
	child, err = deps.launch(slot.ExecutablePath, []string{"-config", configPath, "-local-model"}, attempt)
	if err != nil || child == nil || child.PID() == 0 {
		return ErrCompatibilitySmoke
	}
	if err = s.awaitDisposable(ctx, child, port, provenance, keys, deps); err != nil {
		return err
	}
	return nil
}

func (s *updaterSmoke) dependencies() smokeDependencies {
	defaults := newUpdaterSmoke(s.manager).deps
	if s.deps.random != nil {
		defaults.random = s.deps.random
	}
	if s.deps.choosePort != nil {
		defaults.choosePort = s.deps.choosePort
	}
	if s.deps.launch != nil {
		defaults.launch = s.deps.launch
	}
	if s.deps.listeners != nil {
		defaults.listeners = s.deps.listeners
	}
	if s.deps.request != nil {
		defaults.request = s.deps.request
	}
	if s.deps.preflightConfigPair != nil {
		defaults.preflightConfigPair = s.deps.preflightConfigPair
	}
	return defaults
}

func (s *updaterSmoke) createAttempt(random io.Reader, root string) (string, error) {
	for range 4 {
		id := make([]byte, 16)
		if _, err := io.ReadFull(random, id); err != nil {
			return "", err
		}
		name := hex.EncodeToString(id)
		zeroSmokeBytes(id)
		path := filepath.Join(root, name)
		if !pathWithin(root, path) {
			return "", ErrCompatibilitySmoke
		}
		if _, err := os.Lstat(path); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return "", ErrCompatibilitySmoke
		}
		creator, ok := s.manager.acl.(interface{ CreateExclusive(string) error })
		if !ok || creator.CreateExclusive(path) != nil {
			return "", ErrCompatibilitySmoke
		}
		if !smokeSafeDirectory(path) || s.manager.acl.Inspect(path) != nil {
			_ = os.Remove(path)
			return "", ErrCompatibilitySmoke
		}
		return path, nil
	}
	return "", ErrCompatibilitySmoke
}

func (s *updaterSmoke) awaitDisposable(ctx context.Context, child smokeProcess, port int, provenance upstreamcatalog.Provenance, keys smokeKeys, deps smokeDependencies) error {
	deadline := time.NewTicker(50 * time.Millisecond)
	defer deadline.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return ErrCompatibilitySmoke
		}
		identity, err := s.manager.inspector.Inspect(child.PID())
		if err != nil || identity.PID != child.PID() || identity.StartTime == 0 || !samePath(identity.Image, filepath.Join(s.manager.layout.Bin, "cliproxyapi", provenance.Version, "cliproxyapi.exe")) {
			return ErrCompatibilitySmoke
		}
		if digest(identity.Image) != provenance.ExecutableSHA256 {
			return ErrCompatibilitySmoke
		}
		rows, err := deps.listeners()
		if err != nil {
			return ErrCompatibilitySmoke
		}
		ready, topologyErr := exactLoopbackListener(rows, child.PID(), port)
		if topologyErr != nil {
			return ErrCompatibilitySmoke
		}
		if ready {
			if err = verifySmokeEndpoints(ctx, port, keys, deps.request); err == nil {
				last, inspectErr := s.manager.inspector.Inspect(child.PID())
				if inspectErr == nil && last.PID == child.PID() && last.StartTime == identity.StartTime && samePath(last.Image, identity.Image) && digest(last.Image) == provenance.ExecutableSHA256 {
					return nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return ErrCompatibilitySmoke
		case <-deadline.C:
		}
	}
}

func exactLoopbackListener(rows []listener, pid uint32, port int) (bool, error) {
	owned, onPort := 0, 0
	for _, row := range rows {
		if row.port == port {
			onPort++
			if row.pid != pid || row.ipv6 || row.address != "127.0.0.1" {
				return false, ErrCompatibilitySmoke
			}
		}
		if row.pid == pid {
			owned++
			if row.port != port || row.ipv6 || row.address != "127.0.0.1" {
				return false, ErrCompatibilitySmoke
			}
		}
	}
	if onPort > 1 || owned > 1 {
		return false, ErrCompatibilitySmoke
	}
	return owned == 1 && onPort == 1, nil
}

func verifySmokeEndpoints(ctx context.Context, port int, keys smokeKeys, request smokeRequester) error {
	for _, test := range []struct {
		path, header, value string
		want                int
	}{
		{smokeHealthPath, "", "", http.StatusOK},
		{smokeClientPath, "", "", http.StatusUnauthorized},
		{smokeClientPath, "Authorization", "Bearer " + string(keys.clientWire), http.StatusOK},
		{smokeClientPath, "Authorization", "Bearer " + string(keys.wrongClientWire), http.StatusUnauthorized},
		{smokeManagementPath, "", "", http.StatusUnauthorized},
		{smokeManagementPath, "X-Management-Key", string(keys.managementWire), http.StatusOK},
		{smokeManagementPath, "X-Management-Key", string(keys.wrongManagementWire), http.StatusUnauthorized},
	} {
		status, body, err := request(ctx, port, test.path, test.header, test.value)
		if err != nil || status != test.want || len(body) > smokeBodyLimit || (test.path == smokeHealthPath && !bytes.Equal(body, []byte(`{"status":"ok"}`))) {
			zeroSmokeBytes(body)
			return ErrCompatibilitySmoke
		}
		zeroSmokeBytes(body)
	}
	return nil
}

func (s *updaterSmoke) cleanupAttempt(child smokeProcess, port int, attempt string, owned bool, deps smokeDependencies) error {
	var failures []error
	if child != nil {
		if err := terminateSmokeProcess(child); err != nil {
			failures = append(failures, err)
		}
		if port != 0 {
			rows, err := deps.listeners()
			if err != nil {
				failures = append(failures, err)
			} else {
				for _, row := range rows {
					if row.port == port {
						failures = append(failures, ErrCompatibilitySmoke)
						break
					}
				}
			}
		}
	}
	if owned {
		if err := s.removeAttempt(attempt); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// terminateSmokeProcess accepts the non-zero exit status produced by a
// deliberate Windows TerminateProcess call, but only after our Kill request
// succeeded. An already-dead child and every other wait failure remain errors.
func terminateSmokeProcess(child smokeProcess) error {
	if child == nil {
		return ErrCompatibilitySmoke
	}
	if err := child.Kill(); err != nil {
		return err
	}
	waitErr := child.Wait()
	if waitErr == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		return nil
	}
	return waitErr
}

func (s *updaterSmoke) removeAttempt(attempt string) error {
	root := filepath.Join(s.manager.layout.State, smokeRootName)
	if !pathWithin(root, attempt) || !smokeSafeDirectory(root) || s.manager.acl.Inspect(root) != nil || !smokeSafeDirectory(attempt) || s.manager.acl.Inspect(attempt) != nil {
		return ErrCompatibilitySmoke
	}
	entries, err := os.ReadDir(attempt)
	if err != nil || len(entries) != 2 {
		return ErrCompatibilitySmoke
	}
	configPath := filepath.Join(attempt, smokeConfigName)
	authPath := filepath.Join(attempt, smokeAuthName)
	configSeen, authSeen := false, false
	for _, entry := range entries {
		switch entry.Name() {
		case smokeConfigName:
			configSeen = !entry.IsDir() && entry.Type()&os.ModeSymlink == 0 && !isFileReparse(configPath) && s.manager.acl.InspectFile(configPath) == nil
		case smokeAuthName:
			authSeen = entry.IsDir() && entry.Type()&os.ModeSymlink == 0 && smokeSafeDirectory(authPath) && s.manager.acl.Inspect(authPath) == nil
			if authSeen {
				children, readErr := os.ReadDir(authPath)
				authSeen = readErr == nil && len(children) == 0
			}
		default:
			return ErrCompatibilitySmoke
		}
	}
	if !configSeen || !authSeen {
		return ErrCompatibilitySmoke
	}
	if err = os.Remove(configPath); err != nil {
		return err
	}
	if err = os.Remove(authPath); err != nil {
		return err
	}
	return os.Remove(attempt)
}

func writeSmokeConfig(acl ACL, path string, config []byte) error {
	file, err := acl.CreateFile(path)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(config)
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil || acl.InspectFile(path) != nil {
		return ErrCompatibilitySmoke
	}
	return nil
}

func newSmokeKeys(random io.Reader) (smokeKeys, error) {
	var keys smokeKeys
	keys.raw = make([][]byte, 4)
	keys.wire = make([][]byte, 4)
	for i := range keys.raw {
		keys.raw[i] = make([]byte, 32)
		if _, err := io.ReadFull(random, keys.raw[i]); err != nil {
			keys.wipe()
			return smokeKeys{}, err
		}
		for j := 0; j < i; j++ {
			if subtle.ConstantTimeCompare(keys.raw[i], keys.raw[j]) == 1 {
				keys.wipe()
				return smokeKeys{}, ErrCompatibilitySmoke
			}
		}
		wire, err := keymaterial.Encode(keys.raw[i])
		if err != nil {
			keys.wipe()
			return smokeKeys{}, err
		}
		keys.wire[i] = wire
	}
	keys.clientWire, keys.managementWire = keys.wire[0], keys.wire[1]
	keys.wrongClientWire, keys.wrongManagementWire = keys.wire[2], keys.wire[3]
	return keys, nil
}

type smokeKeys struct {
	raw, wire                            [][]byte
	clientWire, managementWire           []byte
	wrongClientWire, wrongManagementWire []byte
}

func (k *smokeKeys) wipe() {
	for _, group := range [][][]byte{k.raw, k.wire} {
		for _, value := range group {
			secretstore.Zero(value)
		}
	}
}

func chooseSmokePort() (int, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	address, ok := listener.Addr().(*net.TCPAddr)
	closeErr := listener.Close()
	if !ok || closeErr != nil || address.IP.String() != "127.0.0.1" || !safeSmokePort(address.Port) {
		return 0, ErrCompatibilitySmoke
	}
	return address.Port, nil
}

func safeSmokePort(port int) bool {
	return port >= 1024 && port <= 65535 && port != cliproxyconfig.CodexPort && port != cliproxyconfig.GooglePort
}

func launchSmokeProcess(executable string, args []string, workspace string) (smokeProcess, error) {
	if !filepath.IsAbs(executable) || filepath.Base(executable) != "cliproxyapi.exe" || !filepath.IsAbs(workspace) {
		return nil, ErrCompatibilitySmoke
	}
	cmd := exec.Command(executable, args...)
	cmd.Dir = workspace
	cmd.Env = minimalEnv()
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &commandSmokeProcess{cmd: cmd}, nil
}

type commandSmokeProcess struct{ cmd *exec.Cmd }

func (p *commandSmokeProcess) PID() uint32 {
	if p == nil || p.cmd == nil || p.cmd.Process == nil || p.cmd.Process.Pid <= 0 {
		return 0
	}
	return uint32(p.cmd.Process.Pid)
}
func (p *commandSmokeProcess) Kill() error {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return ErrCompatibilitySmoke
	}
	return p.cmd.Process.Kill()
}
func (p *commandSmokeProcess) Wait() error {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return ErrCompatibilitySmoke
	}
	return p.cmd.Wait()
}

func requestLoopback(ctx context.Context, port int, path, header, value string) (int, []byte, error) {
	if port < 1 || port > 65535 || (path != smokeHealthPath && path != smokeClientPath && path != smokeManagementPath) || strings.ContainsAny(value, "\r\n") {
		return 0, nil, ErrCompatibilitySmoke
	}
	transport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d%s", port, path), nil)
	if err != nil {
		return 0, nil, ErrCompatibilitySmoke
	}
	if header != "" {
		if (header != "Authorization" && header != "X-Management-Key") || value == "" {
			return 0, nil, ErrCompatibilitySmoke
		}
		request.Header.Set(header, value)
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, smokeBodyLimit+1))
	if err != nil || len(body) > smokeBodyLimit {
		zeroSmokeBytes(body)
		return 0, nil, ErrCompatibilitySmoke
	}
	return response.StatusCode, body, nil
}

func pathWithin(root, path string) bool {
	if root == "" || path == "" || !filepath.IsAbs(root) || !filepath.IsAbs(path) {
		return false
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && rel != ".." && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func smokeSafeDirectory(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 && !isFileReparse(path)
}

func zeroSmokeBytes(value []byte) {
	for i := range value {
		value[i] = 0
	}
}

func (s *updaterSmoke) Production(ctx context.Context, version string) error {
	if s == nil || s.manager == nil || ctx == nil || ctx.Err() != nil {
		return ErrCompatibilitySmoke
	}
	m := s.manager
	active, err := m.state.LoadState()
	if err != nil || state.ValidateState(active) != nil || active.ActiveUpstreamVersion != version {
		return ErrCompatibilitySmoke
	}
	provenance, slot, err := s.resolveTrustedSlot(ctx, version)
	if err != nil || !slotMatchesProvenance(slot, provenance) {
		return ErrCompatibilitySmoke
	}
	deps := s.dependencies()
	if deps.preflightConfigPair != nil {
		err = deps.preflightConfigPair()
	} else {
		var validator *cliproxyconfig.Generator
		validator, err = cliproxyconfig.New(m.layout, m.acl, m.reader, m.lock)
		if err == nil {
			var pair cliproxyconfig.Inspection
			pair, err = validator.InspectPair()
			if err == nil && (!pair.Ready || pair.Present != 2) {
				err = ErrCompatibilitySmoke
			}
		}
	}
	if err != nil {
		return ErrCompatibilitySmoke
	}
	for _, id := range []cliproxyconfig.ID{cliproxyconfig.Codex, cliproxyconfig.Google} {
		status, statusErr := m.updaterStatusLocked(id)
		if errors.Is(statusErr, ErrNotRunning) {
			if status.Record.PID != 0 {
				return ErrCompatibilitySmoke
			}
			continue
		}
		if statusErr != nil || !status.Running {
			return ErrCompatibilitySmoke
		}
		record := status.Record
		configPath := filepath.Join(m.instancePath(id), "config.yaml")
		if record.UpstreamVersion != version || record.ExecutableSHA256 != slot.ExecutableSHA256 || record.ManifestSHA256 != slot.ManifestSHA256 || digest(configPath) == "" || digest(configPath) != record.ConfigSHA256 {
			return ErrCompatibilitySmoke
		}
		if err = m.awaitReadyWith(ctx, id, record, deps.listeners, deps.request); err != nil {
			return ErrCompatibilitySmoke
		}
	}
	return nil
}
