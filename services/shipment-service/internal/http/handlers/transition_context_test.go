package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

const testVerifiedUserID = "22222222-2222-2222-2222-222222222222"
const testVerifiedTenantID = "11111111-1111-1111-1111-111111111111"

func setVerifiedUserHeader(req *http.Request) {
	req.Header.Set("X-User-ID", testVerifiedUserID)
}

func setVerifiedTenantHeader(req *http.Request) {
	req.Header.Set("X-Tenant-ID", testVerifiedTenantID)
}

func TestResolveVerifiedUserFromHeader(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("POST", "/v1/shipments", nil)
	req.Header.Set("X-User-ID", testVerifiedUserID)

	got, err := resolveVerifiedUser(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.String() != testVerifiedUserID {
		t.Fatalf("user=%s", got)
	}
}

func TestResolveVerifiedUserMissingReturnsUnauthorized(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("POST", "/v1/shipments", nil)
	_, err := resolveVerifiedUser(req)
	if err == nil {
		t.Fatal("expected error")
	}
	var appErr *apperrors.AppError
	if !errorsAsAppError(err, &appErr) || appErr.Code != apperrors.CodeUnauthorized {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}

func TestResolveVerifiedUserMalformedReturns400(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("POST", "/v1/shipments", nil)
	req.Header.Set("X-User-ID", "not-a-uuid")
	_, err := resolveVerifiedUser(req)
	if err == nil {
		t.Fatal("expected error")
	}
	var appErr *apperrors.AppError
	if !errorsAsAppError(err, &appErr) || appErr.Code != apperrors.CodeValidation {
		t.Fatalf("expected validation, got %v", err)
	}
}

func TestResolveVerifiedUserZeroUUIDReturns400(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("POST", "/v1/shipments", nil)
	req.Header.Set("X-User-ID", uuid.Nil.String())
	_, err := resolveVerifiedUser(req)
	if err == nil {
		t.Fatal("expected error")
	}
	var appErr *apperrors.AppError
	if !errorsAsAppError(err, &appErr) || appErr.Code != apperrors.CodeValidation {
		t.Fatalf("expected validation, got %v", err)
	}
}

func TestResolveUserStatusTransitionContextUsesVerifiedUser(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("POST", "/v1/shipments/x/status", nil)
	req.Header.Set("X-User-ID", testVerifiedUserID)
	req.Header.Set(sharedmiddleware.RequestIDHeader, "req-abc")

	transition, err := resolveUserStatusTransitionContext(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if transition.ActorType != domain.ActorTypeUser {
		t.Fatalf("actorType=%s", transition.ActorType)
	}
	if transition.ActorID == nil || transition.ActorID.String() != testVerifiedUserID {
		t.Fatalf("actorID=%v", transition.ActorID)
	}
	if transition.CorrelationID == nil || *transition.CorrelationID != "req-abc" {
		t.Fatalf("correlationID=%v", transition.CorrelationID)
	}
}

func TestResolveUserStatusTransitionContextMissingUserReturnsUnauthorized(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("POST", "/v1/shipments/x/status", nil)
	_, err := resolveUserStatusTransitionContext(req)
	if err == nil {
		t.Fatal("expected error")
	}
	var appErr *apperrors.AppError
	if !errorsAsAppError(err, &appErr) || appErr.Code != apperrors.CodeUnauthorized {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}
