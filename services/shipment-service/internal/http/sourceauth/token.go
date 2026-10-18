package sourceauth

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
)

const maxTokenLength = 256

// Middleware accepts only the dedicated company-service revalidation credential.
// An empty configured token, a missing header, and a wrong token are the same 401.
// X-Internal-Service-Token and X-Internal-Service-Name are not credentials here.
func Middleware(configured string) func(http.Handler) http.Handler {
	configured = strings.TrimSpace(configured)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !tokenMatches(configured, r.Header.Get(Header())) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func Header() string {
	return "X-Company-Relationship-Revalidation-Token"
}

func tokenMatches(configured, presented string) bool {
	configured = strings.TrimSpace(configured)
	presented = strings.TrimSpace(presented)
	if configured == "" || presented == "" || len(configured) > maxTokenLength || len(presented) > maxTokenLength {
		return false
	}
	sumConfigured := sha256.Sum256([]byte(configured))
	sumPresented := sha256.Sum256([]byte(presented))
	return subtle.ConstantTimeCompare(sumConfigured[:], sumPresented[:]) == 1
}
