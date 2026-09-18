package integrationauth

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// APIKeyLookupFingerprint returns the stable lookup key for an API key bearer secret.
func APIKeyLookupFingerprint(apiKey string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(apiKey)))
	return hex.EncodeToString(sum[:])
}

func IsAPIKeyBearer(token string) bool {
	return strings.HasPrefix(strings.TrimSpace(token), APIKeyPrefix)
}
