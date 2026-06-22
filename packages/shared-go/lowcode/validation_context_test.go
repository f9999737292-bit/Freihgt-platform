package lowcode

import (
	"net/http"
	"testing"
)

func TestApplyValidationContextHeaders(t *testing.T) {
	h := make(http.Header)
	ApplyValidationContextHeaders(h, ValidationContext{
		EntityStatus: "DRAFT",
		Role:         "SHIPPER_ADMIN",
	})
	if h.Get(HeaderEntityStatus) != "DRAFT" {
		t.Fatalf("expected entity status header")
	}
	if h.Get(HeaderRole) != "SHIPPER_ADMIN" {
		t.Fatalf("expected role header")
	}
}

func TestValidationContextFromHeaders(t *testing.T) {
	h := make(http.Header)
	h.Set(HeaderEntityStatus, "APPROVED")
	h.Set(HeaderRole, "PLATFORM_ADMIN")
	ctx := ValidationContextFromHeaders(h)
	if ctx.EntityStatus != "APPROVED" || ctx.Role != "PLATFORM_ADMIN" {
		t.Fatalf("unexpected context: %+v", ctx)
	}
}
