//go:build windows

// Package cliproxymgmt exposes the two read-only management probes used by
// the pinned empty-lifecycle contract. It intentionally has no general URL,
// method, or management-route surface.
package cliproxymgmt

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/trungqwe/dual-pool/internal/cliproxyconfig"
	"github.com/trungqwe/dual-pool/internal/keymaterial"
	"github.com/trungqwe/dual-pool/internal/secretstore"
)

var (
	ErrContract                = errors.New("management response violates pinned contract")
	ErrUnexpectedAuthInventory = errors.New("management inventory is unexpectedly non-empty")
	ErrManagementUnavailable   = errors.New("management request unavailable")
)

const (
	pinnedVersion  = "7.3.7"
	pinnedCommit   = "b773607e3e7756dc6020a291825e4eb08899595a"
	debugLimit     = 1024
	inventoryLimit = 64 * 1024
)

type SecretReader interface {
	Get(secretstore.Purpose) ([]byte, error)
}

type Client struct {
	id      cliproxyconfig.ID
	port    int
	purpose secretstore.Purpose
	reader  SecretReader
	http    *http.Client
}

func New(id cliproxyconfig.ID, reader SecretReader) (*Client, error) {
	if reader == nil {
		return nil, ErrManagementUnavailable
	}
	var port int
	var purpose secretstore.Purpose
	switch id {
	case cliproxyconfig.Codex:
		port, purpose = cliproxyconfig.CodexPort, secretstore.CodexManagementKey
	case cliproxyconfig.Google:
		port, purpose = cliproxyconfig.GooglePort, secretstore.GoogleManagementKey
	default:
		return nil, ErrManagementUnavailable
	}
	addr := net.JoinHostPort("127.0.0.1", itoa(port))
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	tr := &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != addr {
			return nil, ErrManagementUnavailable
		}
		return dialer.DialContext(ctx, network, address)
	}, DisableKeepAlives: true, ResponseHeaderTimeout: 3 * time.Second}
	return &Client{id: id, port: port, purpose: purpose, reader: reader, http: &http.Client{Transport: tr, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func itoa(v int) string {
	if v == 8317 {
		return "8317"
	}
	return "8318"
}

func (c *Client) Debug(ctx context.Context) error {
	b, err := c.request(ctx, "/v0/management/debug", debugLimit)
	if err != nil {
		return err
	}
	return parseDebug(b)
}
func (c *Client) EmptyInventory(ctx context.Context) error {
	b, err := c.request(ctx, "/v0/management/auth-files", inventoryLimit)
	if err != nil {
		return err
	}
	return parseEmptyInventory(b)
}

func (c *Client) request(ctx context.Context, path string, limit int64) ([]byte, error) {
	if c == nil || c.reader == nil || c.http == nil || (path != "/v0/management/debug" && path != "/v0/management/auth-files") {
		return nil, ErrManagementUnavailable
	}
	raw, err := c.reader.Get(c.purpose)
	if err != nil || len(raw) != 32 {
		secretstore.Zero(raw)
		return nil, ErrManagementUnavailable
	}
	defer secretstore.Zero(raw)
	wire, err := keymaterial.Encode(raw)
	if err != nil {
		return nil, ErrManagementUnavailable
	}
	defer secretstore.Zero(wire)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+itoa(c.port)+path, nil)
	if err != nil {
		return nil, ErrManagementUnavailable
	}
	req.Header.Set("X-Management-Key", string(wire)) // Header storage requires an immutable Go string; buffers above are still wiped.
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, ErrManagementUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("X-CPA-VERSION") != pinnedVersion || resp.Header.Get("X-CPA-COMMIT") != pinnedCommit {
		return nil, ErrContract
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil || int64(len(b)) > limit || !utf8.Valid(b) {
		secretstore.Zero(b)
		return nil, ErrContract
	}
	return b, nil
}
