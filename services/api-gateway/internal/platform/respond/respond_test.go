package respond_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/platform/respond"
)

func TestServiceUnavailableMapsTo502(t *testing.T) {
	rec := httptest.NewRecorder()
	respond.Error(rec, apperrors.ServiceUnavailable("target service is unavailable", "company-service"))

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status=%d want 502", rec.Code)
	}
	assertErrorCode(t, rec, "SERVICE_UNAVAILABLE")
}

func TestControlTowerShipmentsUnavailableMapsTo503(t *testing.T) {
	rec := httptest.NewRecorder()
	respond.Error(rec, apperrors.ControlTowerShipmentsUnavailable("required shipment data is temporarily unavailable"))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d want 503", rec.Code)
	}
	assertErrorCode(t, rec, "CONTROL_TOWER_SHIPMENTS_UNAVAILABLE")
}

func TestAuthDependencyUnavailableMapsTo503(t *testing.T) {
	rec := httptest.NewRecorder()
	respond.Error(rec, apperrors.AuthDependencyUnavailable("authentication service is temporarily unavailable"))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d want 503", rec.Code)
	}
	assertErrorCode(t, rec, "AUTH_DEPENDENCY_UNAVAILABLE")
}

func assertErrorCode(t *testing.T, rec *httptest.ResponseRecorder, wantCode string) {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error.Code != wantCode {
		t.Fatalf("error code=%q want %q", body.Error.Code, wantCode)
	}
}
