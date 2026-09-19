package safelog

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

// Fingerprint returns the first 16 bytes of HMAC-SHA-256 as 32 lowercase hex
// characters. The caller owns the key lifecycle; this package never persists it.
func Fingerprint(key []byte, rawSession string) (string, error) {
	if len(key) == 0 || rawSession == "" {
		return "", errors.New("session fingerprint input is unavailable")
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(rawSession))
	sum := mac.Sum(nil)
	return hex.EncodeToString(sum[:16]), nil
}
