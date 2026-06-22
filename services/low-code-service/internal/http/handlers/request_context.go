package handlers

import (
	"net/http"
	"strings"

	sharedlowcode "github.com/freight-platform/shared-go/lowcode"
	sharedmiddleware "github.com/freight-platform/shared-go/middleware"

	"github.com/freight-platform/low-code-service/internal/domain"
)

const userIDHeader = sharedlowcode.HeaderUserID

func auditContextFromRequest(r *http.Request) domain.AuditContext {
	ctx := domain.AuditContext{
		RequestID: sharedlowcode.RequestIDFromHeader(r.Header),
		IPAddress: strings.TrimSpace(r.RemoteAddr),
		UserAgent: strings.TrimSpace(r.Header.Get("User-Agent")),
	}
	if ctx.RequestID == "" {
		ctx.RequestID = sharedmiddleware.RequestIDFromContext(r.Context())
	}

	if userID, ok := sharedlowcode.ActorIDFromHeader(r.Header); ok {
		ctx.ChangedByUserID = &userID
	}
	return ctx
}

func validationContextFromRequest(r *http.Request, body *validationContextRequest) domain.ValidationContext {
	ctx := domain.ValidationContext{}
	if body != nil {
		ctx.EntityStatus = strings.TrimSpace(body.EntityStatus)
		ctx.Role = strings.TrimSpace(body.Role)
	}
	headerCtx := sharedlowcode.ValidationContextFromHeaders(r.Header)
	if ctx.EntityStatus == "" {
		ctx.EntityStatus = strings.TrimSpace(headerCtx.EntityStatus)
	}
	if ctx.Role == "" {
		ctx.Role = strings.TrimSpace(headerCtx.Role)
	}
	return ctx
}
