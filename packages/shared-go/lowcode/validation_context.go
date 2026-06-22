package lowcode

import "net/http"

const (
	HeaderEntityStatus = "X-Low-Code-Entity-Status"
	HeaderRole         = "X-Low-Code-Role"
)

type ValidationContext struct {
	EntityStatus string
	Role         string
}

func ApplyValidationContextHeaders(h http.Header, ctx ValidationContext) {
	if ctx.EntityStatus != "" {
		h.Set(HeaderEntityStatus, ctx.EntityStatus)
	}
	if ctx.Role != "" {
		h.Set(HeaderRole, ctx.Role)
	}
}

func ValidationContextFromHeaders(h http.Header) ValidationContext {
	return ValidationContext{
		EntityStatus: h.Get(HeaderEntityStatus),
		Role:         h.Get(HeaderRole),
	}
}
