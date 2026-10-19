package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestShipmentRelationshipCreateFallsThroughAndRejectsEmptySecret(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	router := WithShipmentRelationshipCreate(next, "", nil)
	other := httptest.NewRequest(http.MethodGet, "/v1/shipments/"+uuid.NewString(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, other)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("fallthrough %d", rec.Code)
	}
	create := httptest.NewRequest(http.MethodPost, "/v1/shipments/"+uuid.NewString()+"/carrier-relationship-requests", nil)
	create.Header.Set("Authorization", "Bearer not-a-token")
	create.Header.Set("Idempotency-Key", "key")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, create)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("empty secret %d", rec.Code)
	}
}
