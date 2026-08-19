package internalauth

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

const HeaderName = "X-Internal-Service-Token"

type Config struct {
	Token       string
	Environment string
}

func (c Config) Authorize(r *http.Request) bool {
	expected := strings.TrimSpace(c.Token)
	if expected == "" {
		return false
	}
	provided := strings.TrimSpace(r.Header.Get(HeaderName))
	if provided == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

func (c Config) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !c.Authorize(r) {
			http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"internal service authentication failed","details":{}}}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
