// Package token derives opaque, unguessable subscription tokens from subIds so
// proxy UUIDs never appear in the subscription URL.
package token

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
)

const length = 24 // hex chars (96 bits)

// Make returns the deterministic token for a subId under the given secret.
func Make(secret, subID string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(subID))

	return hex.EncodeToString(mac.Sum(nil))[:length]
}

// Match reports whether token corresponds to subID (constant-time).
func Match(secret, subID, token string) bool {
	want := Make(secret, subID)
	return subtle.ConstantTimeCompare([]byte(want), []byte(token)) == 1
}
