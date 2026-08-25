//go:build integration

package systemwave2

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	gwmiddleware "github.com/freight-platform/api-gateway/internal/http/middleware"
)

// TestW2_HeaderSpoofRegressionOnBusinessEndpoint verifies JWT tenant wins over spoofed headers
// on a sensitive procurement/shipment mutation path (Wave 2 regression of Wave 1).
func TestW2_HeaderSpoofRegressionOnBusinessEndpoint(t *testing.T) {
	secret := "wave2-secret"
	tokenTenantA := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	tokenUserA := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	token := signToken(t, secret, tokenUserA, tokenTenantA, "a-buyer@test.local")

	var gotTenant, gotUser, gotCompany string
	handler := gwmiddleware.Auth(true, secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenant = r.Header.Get("X-Tenant-ID")
		gotUser = r.Header.Get("X-User-ID")
		gotCompany = r.Header.Get("X-Company-ID")
		w.WriteHeader(http.StatusAccepted)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/bids/00000000-0000-0000-0000-000000000001/accept", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "cccccccc-cccc-cccc-cccc-cccccccccccc")
	req.Header.Set("X-Company-ID", "dddddddd-dddd-dddd-dddd-dddddddddddd")
	req.Header.Set("X-User-ID", "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status=%d want 202", rec.Code)
	}
	if gotTenant != tokenTenantA {
		t.Fatalf("spoofed tenant not ignored: got %q want %q", gotTenant, tokenTenantA)
	}
	if gotUser != tokenUserA {
		t.Fatalf("spoofed user not ignored: got %q want %q", gotUser, tokenUserA)
	}
	if gotCompany != "" {
		t.Fatalf("X-Company-ID must be stripped, got %q", gotCompany)
	}
}

func signToken(t *testing.T, secret, userID, tenantID, email string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"tenant_id": tenantID,
		"email":     email,
		"sub":       userID,
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return signed
}
