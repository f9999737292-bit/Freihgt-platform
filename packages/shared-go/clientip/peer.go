package clientip

import (
	"context"
	"net"
	"net/http"
)

type peerContextKey struct{}

// CapturePeerMiddleware stores the TCP peer host before any RealIP middleware runs.
// Security-sensitive code must use Resolver.ClientIP, which reads this immutable peer.
func CapturePeerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if peer, err := hostFromAddr(r.RemoteAddr); err == nil && peer != "" {
			r = r.WithContext(context.WithValue(r.Context(), peerContextKey{}, peer))
		}
		next.ServeHTTP(w, r)
	})
}

// PeerHostFromContext returns the socket peer captured before RealIP middleware.
func PeerHostFromContext(ctx context.Context) (string, bool) {
	peer, ok := ctx.Value(peerContextKey{}).(string)
	return peer, ok && peer != ""
}

func peerHostFromRequest(req *http.Request) (string, error) {
	if peer, ok := PeerHostFromContext(req.Context()); ok {
		return peer, nil
	}
	return hostFromAddr(req.RemoteAddr)
}

// StripSpoofableForwardedHeaders removes forwarded client IP headers when the immediate
// peer is not a configured trusted proxy.
func StripSpoofableForwardedHeaders(resolver *Resolver) func(http.Handler) http.Handler {
	if resolver == nil {
		resolver = NewResolver(nil)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			peerIP, err := peerHostFromRequest(r)
			if err != nil {
				StripForwardedClientHeaders(r.Header)
				next.ServeHTTP(w, r)
				return
			}
			peer := net.ParseIP(peerIP)
			if peer == nil || len(resolver.trusted) == 0 || !resolver.contains(peer) {
				StripForwardedClientHeaders(r.Header)
			}
			next.ServeHTTP(w, r)
		})
	}
}
