package security

import (
	"testing"

	"github.com/google/uuid"
)

func TestJWTCreateAndParse(t *testing.T) {
	t.Parallel()

	svc := NewJWTService("test-secret", 60)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	tenantID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	token, expiresIn, err := svc.CreateAccessToken(userID, tenantID, "user@example.com", "ru-RU")
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	if token == "" {
		t.Fatalf("expected token")
	}
	if expiresIn != 3600 {
		t.Fatalf("expected expiresIn 3600, got %d", expiresIn)
	}

	claims, err := svc.ParseAccessToken(token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.Subject != userID.String() {
		t.Fatalf("unexpected subject: %s", claims.Subject)
	}
	if claims.TenantID != tenantID.String() {
		t.Fatalf("unexpected tenant_id: %s", claims.TenantID)
	}
	if claims.Email != "user@example.com" {
		t.Fatalf("unexpected email: %s", claims.Email)
	}
}
