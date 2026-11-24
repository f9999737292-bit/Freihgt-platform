package http

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/service"
	"github.com/freight-platform/shared-go/internalauth"
)

const writeToken = "dedicated-document-token"

func TestDocumentWriteRoutesDenyWithoutAuthority(t *testing.T) {
	t.Setenv("AUTH_ENABLED", "false")
	router := writeRouter(t)
	documentID := uuid.New()
	tenantID := uuid.New()
	otherTenant := uuid.New()
	companyID := uuid.New()
	otherCompany := uuid.New()
	uploadID := uuid.New()

	writes := []string{
		"POST /v1/documents",
		"POST /v1/documents/" + documentID.String() + "/versions",
		"POST /v1/documents/" + documentID.String() + "/files",
		"POST /v1/documents/" + documentID.String() + "/attachments",
		"POST /v1/documents/" + documentID.String() + "/attachments/" + uploadID.String() + "/finalize",
		"POST /v1/documents/" + documentID.String() + "/attachments/" + uploadID.String() + "/signatures",
		"POST /v1/documents/" + documentID.String() + "/ready-for-signing",
		"POST /v1/documents/" + documentID.String() + "/signing-sessions",
		"POST /v1/documents/" + documentID.String() + "/cancel",
		"POST /v1/documents/" + documentID.String() + "/archive",
		"POST /v1/signing-sessions/" + documentID.String() + "/signatures",
		"POST /internal/v1/pod-uploads",
		"PUT /internal/v1/pod-uploads/" + uploadID.String() + "/content",
		"POST /internal/v1/pod-uploads/" + uploadID.String() + "/complete",
	}
	for _, route := range writes {
		method, path, _ := strings.Cut(route, " ")
		t.Run("missing credential "+route, func(t *testing.T) {
			rec := serveWrite(router, method, path, map[string]string{"X-Tenant-ID": tenantID.String()}, `{}`)
			assertWriteStatus(t, rec, http.StatusUnauthorized, "document write credential is required")
		})
		t.Run("invalid bearer "+route, func(t *testing.T) {
			rec := serveWrite(router, method, path, map[string]string{
				"Authorization": "Bearer not-a-jwt",
				"X-Tenant-ID":   tenantID.String(),
			}, `{}`)
			assertWriteStatus(t, rec, http.StatusUnauthorized, "document write credential is not confirmed")
		})
		t.Run("invalid internal token "+route, func(t *testing.T) {
			rec := serveWrite(router, method, path, map[string]string{
				"X-Internal-Service-Token": "wrong-token",
				"X-Tenant-ID":              tenantID.String(),
			}, `{}`)
			assertWriteStatus(t, rec, http.StatusUnauthorized, "document write credential is not confirmed")
		})
	}

	body := `{"tenant_id":"` + otherTenant.String() + `","owner_company_id":"` + otherCompany.String() + `","shipmentId":"` + uuid.NewString() + `"}`
	sameTenantBody := `{"tenant_id":"` + tenantID.String() + `","owner_company_id":"` + otherCompany.String() + `","shipmentId":"` + uuid.NewString() + `"}`

	t.Run("spoofed tenant header", func(t *testing.T) {
		rec := serveWrite(router, http.MethodPost, "/v1/documents", map[string]string{"X-Tenant-ID": otherTenant.String()}, body)
		assertWriteStatus(t, rec, http.StatusUnauthorized, "document write credential is required")
	})
	t.Run("spoofed user and company headers", func(t *testing.T) {
		rec := serveWrite(router, http.MethodPost, "/v1/documents", map[string]string{
			"X-User-ID":    uuid.NewString(),
			"X-Company-ID": companyID.String(),
		}, `{}`)
		assertWriteStatus(t, rec, http.StatusUnauthorized, "document write credential is required")
	})
	t.Run("foreign tenant with confirmed token", func(t *testing.T) {
		rec := serveWrite(router, http.MethodPost, "/v1/documents", map[string]string{
			"X-Internal-Service-Token": writeToken,
			"X-Tenant-ID":              otherTenant.String(),
		}, body)
		assertWriteStatus(t, rec, http.StatusForbidden, "document write tenant is not authorized")
	})
	t.Run("same tenant foreign company", func(t *testing.T) {
		rec := serveWrite(router, http.MethodPost, "/v1/documents", map[string]string{
			"X-Internal-Service-Token": writeToken,
			"X-Company-ID":             otherCompany.String(),
		}, sameTenantBody)
		assertWriteStatus(t, rec, http.StatusForbidden, "document write subject is not authorized")
	})
	t.Run("unexpected internal caller", func(t *testing.T) {
		rec := serveWrite(router, http.MethodPost, "/internal/v1/pod-uploads", map[string]string{
			"X-Internal-Service-Token": writeToken,
			"X-Internal-Service-Name":  "billing-service",
		}, body)
		assertWriteStatus(t, rec, http.StatusForbidden, "document write caller is not allowed")
	})
	t.Run("unsigned caller header alone", func(t *testing.T) {
		rec := serveWrite(router, http.MethodPost, "/internal/v1/pod-uploads", map[string]string{
			"X-Internal-Service-Name": "shipment-service",
			"X-Tenant-ID":             tenantID.String(),
		}, body)
		assertWriteStatus(t, rec, http.StatusUnauthorized, "document write credential is required")
	})
	t.Run("confirmed token still has no resource authority", func(t *testing.T) {
		rec := serveWrite(router, http.MethodPost, "/internal/v1/pod-uploads", map[string]string{
			"X-Internal-Service-Token": writeToken,
		}, `{"shipmentId":"`+uuid.NewString()+`","driverId":"`+uuid.NewString()+`","ownerCompanyId":"`+companyID.String()+`"}`)
		assertWriteStatus(t, rec, http.StatusForbidden, "document write authority is unavailable")
	})
	t.Run("cross shipment pod", func(t *testing.T) {
		rec := serveWrite(router, http.MethodPost, "/internal/v1/pod-uploads", map[string]string{
			"X-Internal-Service-Token": writeToken,
			"X-Tenant-ID":              tenantID.String(),
		}, `{"shipmentId":"`+uuid.NewString()+`"}`)
		assertWriteStatus(t, rec, http.StatusForbidden, "document write tenant is not authorized")
	})
	t.Run("repeated pod upload", func(t *testing.T) {
		header := map[string]string{"X-Internal-Service-Token": writeToken}
		payload := `{"shipmentId":"` + uuid.NewString() + `","idempotencyKey":"pod-repeat"}`
		first := serveWrite(router, http.MethodPost, "/internal/v1/pod-uploads", header, payload)
		second := serveWrite(router, http.MethodPost, "/internal/v1/pod-uploads", header, payload)
		assertWriteStatus(t, first, http.StatusForbidden, "document write authority is unavailable")
		assertWriteStatus(t, second, http.StatusForbidden, "document write authority is unavailable")
	})
	t.Run("auth disabled does not open writes", func(t *testing.T) {
		rec := serveWrite(router, http.MethodPost, "/v1/documents", map[string]string{
			"X-Tenant-ID":    tenantID.String(),
			"X-Auth-Enabled": "false",
		}, `{}`)
		assertWriteStatus(t, rec, http.StatusUnauthorized, "document write credential is required")
	})
	t.Run("direct document write", func(t *testing.T) {
		rec := serveWrite(router, http.MethodPost, "/v1/documents/"+documentID.String()+"/archive", map[string]string{
			"X-Tenant-ID": tenantID.String(),
		}, `{"tenant_id":"`+tenantID.String()+`"}`)
		assertWriteStatus(t, rec, http.StatusUnauthorized, "document write credential is required")
	})
}

func TestDocumentReadAndSigningRecoveryStayClosedOnlyWhereRequired(t *testing.T) {
	router := writeRouter(t)
	tenantID := uuid.New()
	documentID := uuid.New()

	t.Run("shipper list remains a read route", func(t *testing.T) {
		rec := serveWrite(router, http.MethodGet, "/v1/shipper/documents", map[string]string{"X-Tenant-ID": tenantID.String()}, "")
		if rec.Code == http.StatusUnauthorized && strings.Contains(rec.Body.String(), "document write credential is required") {
			t.Fatalf("write guard blocked a read route: %s", rec.Body.String())
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
	t.Run("carrier list remains a read route", func(t *testing.T) {
		rec := serveWrite(router, http.MethodGet, "/v1/carrier/documents", map[string]string{"X-Tenant-ID": tenantID.String()}, "")
		if rec.Code == http.StatusUnauthorized && strings.Contains(rec.Body.String(), "document write credential is required") {
			t.Fatalf("write guard blocked a read route: %s", rec.Body.String())
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
	t.Run("signing recovery remains forbidden", func(t *testing.T) {
		rec := serveWrite(router, http.MethodPost, "/v1/documents/"+documentID.String()+"/signing-recovery", map[string]string{
			"X-Tenant-ID":              tenantID.String(),
			"X-Internal-Service-Token": writeToken,
			"Authorization":            "Bearer eyJhbGciOiJub25lIn0.e30.sig",
		}, `{"signing_session_id":"`+uuid.NewString()+`","expected_version":1}`)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
	t.Run("no public pod upload alias", func(t *testing.T) {
		rec := serveWrite(router, http.MethodPost, "/v1/pod-uploads", nil, `{}`)
		if rec.Code >= 200 && rec.Code < 300 {
			t.Fatalf("public pod upload status=%d", rec.Code)
		}
	})
}

func writeRouter(t *testing.T) http.Handler {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewRouter(
		log, nil, nil, nil, nil, nil,
		service.NewShipperDocumentService(nil, nil),
		service.NewCarrierDocumentService(nil, nil),
		nil,
		internalauth.Config{Token: writeToken, Environment: "test"},
	)
}

func serveWrite(router http.Handler, method, path string, header map[string]string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for key, value := range header {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func assertWriteStatus(t *testing.T, rec *httptest.ResponseRecorder, status int, message string) {
	t.Helper()
	if rec.Code != status || !strings.Contains(rec.Body.String(), message) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
