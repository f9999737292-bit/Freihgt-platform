package lowcode

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestTenantIDFromHeader(t *testing.T) {
	h := make(http.Header)
	h.Set(HeaderTenantID, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	if got := TenantIDFromHeader(h); got != "74519f22-ff9b-4a8b-8fff-a958c689682f" {
		t.Fatalf("unexpected tenant id: %q", got)
	}
}

func TestActorIDFromHeader(t *testing.T) {
	userID := uuid.New()
	h := make(http.Header)
	h.Set(HeaderUserID, userID.String())
	got, ok := ActorIDFromHeader(h)
	if !ok || got != userID {
		t.Fatalf("expected actor id, got %v ok=%v", got, ok)
	}
}

func TestRequestIDFromHeader(t *testing.T) {
	h := make(http.Header)
	h.Set(HeaderRequestID, "req-abc-123")
	if got := RequestIDFromHeader(h); got != "req-abc-123" {
		t.Fatalf("unexpected request id: %q", got)
	}
}
