package clientip

import "net/http"

const (
	HeaderForwarded     = "Forwarded"
	HeaderXForwardedFor = "X-Forwarded-For"
	HeaderXRealIP       = "X-Real-IP"
)

// StripForwardedClientHeaders removes client-supplied forwarded IP headers.
func StripForwardedClientHeaders(h http.Header) {
	h.Del(HeaderForwarded)
	h.Del(HeaderXForwardedFor)
	h.Del(HeaderXRealIP)
}

// SetTrustedForwardedClientIP sets the canonical downstream client IP header.
func SetTrustedForwardedClientIP(h http.Header, clientIP string) {
	StripForwardedClientHeaders(h)
	if clientIP != "" {
		h.Set(HeaderXForwardedFor, clientIP)
	}
}
