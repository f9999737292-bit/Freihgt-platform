package identity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestClientListUserRoleCodes(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/users/"+userID.String()+"/roles" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("tenant_id") != tenantID.String() {
			t.Fatalf("unexpected tenant_id query")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"code":"PLATFORM_ADMIN"},{"code":"SHIPPER_ADMIN"}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	codes, err := client.ListUserRoleCodes(context.Background(), userID, tenantID)
	if err != nil {
		t.Fatalf("ListUserRoleCodes failed: %v", err)
	}
	if len(codes) != 2 || codes[0] != "PLATFORM_ADMIN" {
		t.Fatalf("unexpected codes: %#v", codes)
	}
}
