package lowcode

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const (
	HeaderTenantID       = "X-Tenant-ID"
	HeaderUserID         = "X-User-ID"
	HeaderRequestID      = "X-Request-ID"
	HeaderAuthorization  = "Authorization"
)

func TenantIDFromHeader(h http.Header) string {
	return strings.TrimSpace(h.Get(HeaderTenantID))
}

func ActorIDFromHeader(h http.Header) (uuid.UUID, bool) {
	raw := strings.TrimSpace(h.Get(HeaderUserID))
	if raw == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

func RequestIDFromHeader(h http.Header) string {
	return strings.TrimSpace(h.Get(HeaderRequestID))
}
