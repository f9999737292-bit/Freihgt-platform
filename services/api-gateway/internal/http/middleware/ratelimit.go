package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/time/rate"

	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/platform/respond"
	"github.com/freight-platform/shared-go/clientip"
)

var rateLimitedTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_rate_limited_total",
		Help: "Total number of rate limited HTTP requests",
	},
	[]string{"service"},
)

func init() {
	prometheus.MustRegister(rateLimitedTotal)
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimit applies an IP-based token bucket limiter to /api/v1/* routes.
func RateLimit(enabled bool, rps float64, burst int, serviceName string, resolver *clientip.Resolver) func(http.Handler) http.Handler {
	if !enabled {
		return func(next http.Handler) http.Handler { return next }
	}
	if resolver == nil {
		resolver = clientip.NewResolver(nil)
	}

	visitors := make(map[string]*visitor)
	var mu sync.Mutex

	go func() {
		for {
			time.Sleep(time.Minute)
			mu.Lock()
			for ip, v := range visitors {
				if time.Since(v.lastSeen) > 3*time.Minute {
					delete(visitors, ip)
				}
			}
			mu.Unlock()
		}
	}()

	limit := rate.Limit(rps)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !shouldRateLimit(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			ip, err := resolver.ClientIP(r)
			if err != nil || ip == "" {
				respond.Error(w, apperrors.Forbidden("access_denied"))
				return
			}
			mu.Lock()
			v, exists := visitors[ip]
			if !exists {
				v = &visitor{limiter: rate.NewLimiter(limit, burst), lastSeen: time.Now()}
				visitors[ip] = v
			}
			v.lastSeen = time.Now()
			allowed := v.limiter.Allow()
			mu.Unlock()

			if !allowed {
				rateLimitedTotal.WithLabelValues(serviceName).Inc()
				respond.Error(w, apperrors.RateLimitExceeded("too many requests"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func shouldRateLimit(path string) bool {
	return strings.HasPrefix(path, "/api/v1/")
}
