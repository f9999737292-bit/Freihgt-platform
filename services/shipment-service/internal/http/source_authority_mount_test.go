package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/http/sourceauth"
	"github.com/freight-platform/shipment-service/internal/service"
)

type mountSource struct {
	row     domain.ShipmentSourceRecord
	readErr error
	actor   domain.ShipmentCreatorAuthority
}

func (m mountSource) ReadShipmentSource(context.Context, uuid.UUID) (domain.ShipmentSourceRecord, error) {
	if m.readErr != nil {
		return domain.ShipmentSourceRecord{}, m.readErr
	}
	return m.row, nil
}

func (m mountSource) ReadShipmentCreator(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.ShipmentCreatorAuthority, error) {
	return m.actor, nil
}

func (m mountSource) ActiveCarrierInTenant(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}

func TestShipmentSourceAuthorityMount(t *testing.T) {
	row := domain.ShipmentSourceRecord{
		TenantID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), ID: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Version: 4, ShipperCompanyID: uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		CarrierCompanyID: uuid.MustParse("44444444-4444-4444-4444-444444444444"), CarrierAssigned: true, Live: true,
	}
	fingerprint, err := domain.ShipmentSourceFingerprint(row)
	if err != nil {
		t.Fatal(err)
	}
	const revalidationToken = "company-revalidation-token"
	const internalToken = "generic-internal-token"
	router := WithShipmentSourceAuthority(http.NotFoundHandler(), revalidationToken, service.NewShipmentSourceAuthority(mountSource{
		row: row, actor: domain.ShipmentCreatorAuthority{Active: true, ActiveMembership: true, CompanyRoles: []string{"SHIPPER_ADMIN"}},
	}))
	body := map[string]string{
		"source_type": domain.ShipmentSourceType, "source_id": row.ID.String(), "source_tenant_id": row.TenantID.String(),
		"source_version_or_fingerprint": fingerprint, "shipper_carrier_company_id": row.CarrierCompanyID.String(),
		"requesting_company_id": row.ShipperCompanyID.String(), "requesting_user_id": "55555555-5555-5555-5555-555555555555",
	}
	payload, _ := json.Marshal(body)

	missing := postSource(router, domain.ShipmentSourceConfirmPath, payload, "")
	wrong := postSource(router, domain.ShipmentSourceConfirmPath, payload, internalToken)
	if missing.Code != http.StatusUnauthorized || wrong.Code != http.StatusUnauthorized || missing.Body.String() != "" || wrong.Body.String() != "" {
		t.Fatalf("auth failures must be identical 401s: %d %q / %d %q", missing.Code, missing.Body.String(), wrong.Code, wrong.Body.String())
	}

	spoof := httptest.NewRequest(http.MethodPost, domain.ShipmentSourceConfirmPath, bytes.NewReader(payload))
	spoof.Header.Set("X-Internal-Service-Token", revalidationToken)
	spoof.Header.Set("X-Internal-Service-Name", "company-service")
	spoofed := httptest.NewRecorder()
	router.ServeHTTP(spoofed, spoof)
	if spoofed.Code != http.StatusUnauthorized || spoofed.Body.Len() != 0 {
		t.Fatalf("generic internal headers must not authorize the callback: %d %s", spoofed.Code, spoofed.Body.String())
	}

	ok := postSource(router, domain.ShipmentSourceConfirmPath, payload, revalidationToken)
	if ok.Code != http.StatusOK || !strings.Contains(ok.Body.String(), `"decision":"CONFIRMED"`) || strings.Contains(ok.Body.String(), "tenant_id") {
		t.Fatalf("confirm response = %d %s", ok.Code, ok.Body.String())
	}

	staleBody := map[string]string{
		"source_type": domain.ShipmentSourceType, "source_id": row.ID.String(), "source_tenant_id": row.TenantID.String(),
		"source_version_or_fingerprint": strings.Repeat("cd", 32), "shipper_carrier_company_id": row.CarrierCompanyID.String(),
	}
	stalePayload, _ := json.Marshal(staleBody)
	stale := postSource(router, domain.ShipmentSourceRevalidatePath, stalePayload, revalidationToken)
	if stale.Code != http.StatusOK || !strings.Contains(stale.Body.String(), `"decision":"CHANGED"`) {
		t.Fatalf("stale revalidate = %d %s", stale.Code, stale.Body.String())
	}

	down := WithShipmentSourceAuthority(http.NotFoundHandler(), revalidationToken, service.NewShipmentSourceAuthority(mountSource{readErr: errors.New("db down")}))
	unavailable := postSource(down, domain.ShipmentSourceRevalidatePath, stalePayload, revalidationToken)
	if unavailable.Code != http.StatusServiceUnavailable || !strings.Contains(unavailable.Body.String(), `"decision":"UNAVAILABLE"`) {
		t.Fatalf("unavailable = %d %s", unavailable.Code, unavailable.Body.String())
	}

	closed := WithShipmentSourceAuthority(http.NotFoundHandler(), "", service.NewShipmentSourceAuthority(mountSource{row: row}))
	refused := postSource(closed, domain.ShipmentSourceConfirmPath, payload, revalidationToken)
	if refused.Code != http.StatusUnauthorized {
		t.Fatalf("empty credential = %d", refused.Code)
	}
	if sourceauth.Header() != "X-Company-Relationship-Revalidation-Token" {
		t.Fatal("revalidation header drifted")
	}
}

func postSource(handler http.Handler, path string, payload []byte, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	if token != "" {
		req.Header.Set(sourceauth.Header(), token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
