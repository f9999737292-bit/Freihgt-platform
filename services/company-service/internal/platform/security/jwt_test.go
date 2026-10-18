package security

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestParseHS256RejectsMissingAndInvalidCredentials(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()
	token, err := SignHS256("test-secret", userID, tenantID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	principal, err := ParseHS256("test-secret", token, time.Now())
	if err != nil || principal.UserID != userID || principal.TenantID != tenantID {
		t.Fatalf("valid token: %v %+v", err, principal)
	}
	for _, tc := range []struct{ secret, token string }{
		{"", token},
		{"test-secret", ""},
		{"other-secret", token},
		{"test-secret", token + "x"},
	} {
		if _, err := ParseHS256(tc.secret, tc.token, time.Now()); err == nil {
			t.Fatalf("accepted secret=%q token=%q", tc.secret, tc.token)
		}
	}
	expired, err := SignHS256("test-secret", userID, tenantID, time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseHS256("test-secret", expired, time.Now()); err == nil {
		t.Fatal("expired token was accepted")
	}
	parts := strings.Split(token, ".")
	noneHeader := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	unsigned := noneHeader + "." + parts[1] + "."
	if _, err := ParseHS256("test-secret", unsigned, time.Now()); err == nil {
		t.Fatal("unsigned token was accepted")
	}
}
