//go:build windows

package cliproxymgmt

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/trungqwe/dual-pool/internal/cliproxyconfig"
	"github.com/trungqwe/dual-pool/internal/secretstore"
	"github.com/trungqwe/dual-pool/internal/upstreamlock"
)

type testReader struct{ value []byte }

var pinnedVersion = testLock().Version
var pinnedCommit = testLock().Commit

func testLock() upstreamlock.Lock {
	b, err := os.ReadFile(filepath.Join("..", "..", "upstream.lock"))
	if err != nil {
		panic(err)
	}
	v, err := upstreamlock.Decode(b)
	if err != nil {
		panic(err)
	}
	return v
}

func (r *testReader) Get(secretstore.Purpose) ([]byte, error) { return r.value, nil }

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func testClient(body string, status int, version, commit string, r *testReader) *Client {
	return &Client{id: cliproxyconfig.Codex, port: cliproxyconfig.CodexPort, purpose: secretstore.CodexManagementKey, reader: r, lock: testLock(), http: &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		h := make(http.Header)
		h.Set("X-CPA-VERSION", version)
		h.Set("X-CPA-COMMIT", commit)
		return &http.Response{StatusCode: status, Header: h, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	})}}
}

func TestDebugContractAndSecretWipe(t *testing.T) {
	r := &testReader{value: []byte("01234567890123456789012345678901")}
	c := testClient(`{"debug":false}`, 200, pinnedVersion, pinnedCommit, r)
	if err := c.Debug(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, b := range r.value {
		if b != 0 {
			t.Fatal("raw management key was not wiped")
		}
	}
	for name, c := range map[string]*Client{
		"wrong version":   testClient(`{"debug":false}`, 200, "v7.3.7", pinnedCommit, &testReader{value: []byte("01234567890123456789012345678901")}),
		"wrong commit":    testClient(`{"debug":false}`, 200, pinnedVersion, "wrong", &testReader{value: []byte("01234567890123456789012345678901")}),
		"missing version": testClient(`{"debug":false}`, 200, "", pinnedCommit, &testReader{value: []byte("01234567890123456789012345678901")}),
		"missing commit":  testClient(`{"debug":false}`, 200, pinnedVersion, "", &testReader{value: []byte("01234567890123456789012345678901")}),
	} {
		t.Run(name, func(t *testing.T) {
			if !errors.Is(c.Debug(context.Background()), ErrContract) {
				t.Fatal("accepted invalid header")
			}
		})
	}
}

func TestEmptyInventoryStrictSchema(t *testing.T) {
	valid := `{"observed_at":"2026-09-20T00:00:00Z","files":[]}`
	if err := testClient(valid, 200, pinnedVersion, pinnedCommit, &testReader{value: []byte("01234567890123456789012345678901")}).EmptyInventory(context.Background()); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"missing observed": `{"files":[]}`, "invalid observed": `{"observed_at":"bad","files":[]}`, "missing files": `{"observed_at":"2026-09-20T00:00:00Z"}`, "null files": `{"observed_at":"2026-09-20T00:00:00Z","files":null}`, "nonarray": `{"observed_at":"2026-09-20T00:00:00Z","files":{}}`, "unknown": `{"observed_at":"2026-09-20T00:00:00Z","files":[],"x":1}`, "duplicate": `{"observed_at":"2026-09-20T00:00:00Z","files":[],"files":[]}`, "multiple": valid + ` {}`,
	} {
		t.Run(name, func(t *testing.T) {
			if !errors.Is(testClient(body, 200, pinnedVersion, pinnedCommit, &testReader{value: []byte("01234567890123456789012345678901")}).EmptyInventory(context.Background()), ErrContract) {
				t.Fatal("invalid schema accepted")
			}
		})
	}
	if !errors.Is(testClient(`{"observed_at":"2026-09-20T00:00:00Z","files":[{}]}`, 200, pinnedVersion, pinnedCommit, &testReader{value: []byte("01234567890123456789012345678901")}).EmptyInventory(context.Background()), ErrUnexpectedAuthInventory) {
		t.Fatal("non-empty inventory accepted")
	}
}

func TestClientAllowlistIsClosed(t *testing.T) {
	c, err := New(cliproxyconfig.Codex, &testReader{value: make([]byte, 32)}, testLock())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.request(context.Background(), "/v0/management/config.yaml", 10); !errors.Is(err, ErrManagementUnavailable) {
		t.Fatal("forbidden route accepted")
	}
}

func TestRuntimeCommitPrefixContract(t *testing.T) {
	for name, got := range map[string]bool{
		"seven": validRuntimeCommit(pinnedCommit, pinnedCommit[:7]), "long": validRuntimeCommit(pinnedCommit, pinnedCommit[:12]), "full": validRuntimeCommit(pinnedCommit, pinnedCommit),
		"short": validRuntimeCommit(pinnedCommit, pinnedCommit[:6]), "uppercase": validRuntimeCommit(pinnedCommit, strings.ToUpper(pinnedCommit[:7])), "nonhex": validRuntimeCommit(pinnedCommit, "zzzzzzz"), "wrong": validRuntimeCommit(pinnedCommit, "abcdef0"), "longer": validRuntimeCommit(pinnedCommit, pinnedCommit+"a"),
	} {
		t.Run(name, func(t *testing.T) {
			if got != (name == "seven" || name == "long" || name == "full") {
				t.Fatal("commit prefix contract mismatch")
			}
		})
	}
}

func TestStrictEOF(t *testing.T) {
	for _, body := range []string{`{"debug":false} `, "{\"debug\":false}\n\t"} {
		if err := parseDebug([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	for _, body := range []string{`{"debug":false}{}`, `{"debug":false} garbage`, `{"debug":false} {`, `{"debug":false}` + string([]byte{0xff})} {
		if !errors.Is(parseDebug([]byte(body)), ErrContract) {
			t.Fatal("trailing data accepted")
		}
	}
}
