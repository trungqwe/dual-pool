package keymaterial

import (
	"encoding/base64"
	"errors"
)

var ErrInvalidRawKey = errors.New("invalid product key material")

// Encode is the single wire representation for every 32-byte product key.
// The caller owns and should wipe the returned 43-byte buffer after use.
func Encode(raw []byte) ([]byte, error) {
	if len(raw) != 32 {
		return nil, ErrInvalidRawKey
	}
	out := make([]byte, base64.RawURLEncoding.EncodedLen(len(raw)))
	base64.RawURLEncoding.Encode(out, raw)
	return out, nil
}
