package keymaterial

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"regexp"
	"testing"
)

func TestWireEncodingRoundTrip(t *testing.T) {
	variants := [][]byte{make([]byte, 32), bytes.Repeat([]byte{0xff}, 32)}
	variants[0][0], variants[0][1], variants[0][2], variants[0][3] = 0, 0x0a, 0x0d, 0x7f
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		t.Fatal(err)
	}
	variants = append(variants, random)
	valid := regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
	for _, raw := range variants {
		wire, err := Encode(raw)
		if err != nil || !valid.Match(wire) {
			t.Fatal("invalid wire encoding")
		}
		decoded, err := base64.RawURLEncoding.DecodeString(string(wire))
		if err != nil || !bytes.Equal(decoded, raw) {
			t.Fatal("round trip failed")
		}
	}
	for _, size := range []int{0, 31, 33} {
		if _, err := Encode(make([]byte, size)); err != ErrInvalidRawKey {
			t.Fatal("wrong size accepted")
		}
	}
}
