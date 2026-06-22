package handlers

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	sharedmiddleware "github.com/freight-platform/shared-go/middleware"

	"github.com/freight-platform/low-code-service/internal/domain"
)

const userIDHeader = "X-User-ID"

func auditContextFromRequest(r *http.Request) domain.AuditContext {
	ctx := domain.AuditContext{
		RequestID: strings.TrimSpace(r.Header.Get(sharedmiddleware.RequestIDHeader)),
		IPAddress: strings.TrimSpace(r.RemoteAddr),
		UserAgent: strings.TrimSpace(r.Header.Get("User-Agent")),
	}
	if ctx.RequestID == "" {
		ctx.RequestID = sharedmiddleware.RequestIDFromContext(r.Context())
	}

	if raw := strings.TrimSpace(r.Header.Get(userIDHeader)); raw != "" {
		if userID, err := uuid.Parse(raw); err == nil {
			ctx.ChangedByUserID = &userID
		}
	}
	return ctx
}
