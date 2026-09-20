package upstreamlock

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
)

func repositoryLock(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepathFromRepo("upstream.lock"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func filepathFromRepo(name string) string { return "..\\..\\" + name }

func TestDecodeRepositoryLock(t *testing.T) {
	b := repositoryLock(t)
	l, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	if l.Version != "7.3.7" || l.Tag != "v7.3.7" || l.Platforms.WindowsAMD64.Artifact != "CLIProxyAPI_7.3.7_windows_amd64.zip" || l.Digest() == "" || !bytes.Equal(l.Raw(), b) {
		t.Fatal("repository lock identity mismatch")
	}
}

func TestDecodeRejectsMalformedLocks(t *testing.T) {
	base := string(repositoryLock(t))
	tests := map[string][]byte{
		"unknown":          []byte(strings.Replace(base, `"schema_version": 1`, `"unknown": true, "schema_version": 1`, 1)),
		"duplicate nested": []byte(strings.Replace(base, `"artifact":`, `"artifact": "duplicate", "artifact":`, 1)),
		"multiple":         append(repositoryLock(t), []byte(` {}`)...),
		"invalid utf8":     append(repositoryLock(t), 0xff),
		"wrong type":       []byte(strings.Replace(base, `"schema_version": 1`, `"schema_version": "1"`, 1)),
		"missing":          []byte(strings.Replace(base, `"product": "CLIProxyAPI",`, ``, 1)),
		"query":            []byte(strings.Replace(base, `.zip",`, `.zip?x=1",`, 1)),
		"latest":           []byte(strings.Replace(base, `/download/v7.3.7/`, `/download/latest/`, 1)),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Decode(data); !errors.Is(err, ErrLockInvalid) {
				t.Fatalf("err=%v", err)
			}
		})
	}
	tooLarge := bytes.Repeat([]byte(" "), MaxBytes+1)
	if _, err := Decode(tooLarge); !errors.Is(err, ErrLockInvalid) {
		t.Fatal(err)
	}
}

func TestSafeBasename(t *testing.T) {
	for _, good := range []string{"cli-proxy-api.exe", "safe.bin"} {
		if !SafeBasename(good) {
			t.Fatal(good)
		}
	}
	for _, bad := range []string{"", "..", "CON.exe", "LPT1.txt", "a\\b", "a/b", "bad.", "bad "} {
		if SafeBasename(bad) {
			t.Fatal(bad)
		}
	}
}
