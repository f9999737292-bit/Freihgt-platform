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

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/http/sourceauth"
	"github.com/freight-platform/rfx-service/internal/service"
)

type mountSource struct {
	award   domain.RfxAwardSourceRecord
	link    domain.RfxAwardLinkSourceRecord
	readErr error
	actor   domain.RfxCreatorAuthority
}

func (m mountSource) ReadAwardSource(context.Context, uuid.UUID) (domain.RfxAwardSourceRecord, error) {
	if m.readErr != nil {
		return domain.RfxAwardSourceRecord{}, m.readErr
	}
	return m.award, nil
}

func (m mountSource) ReadAwardLinkSource(context.Context, uuid.UUID) (domain.RfxAwardLinkSourceRecord, error) {
	if m.readErr != nil {
		return domain.RfxAwardLinkSourceRecord{}, m.readErr
	}
	return m.link, nil
}

func (m mountSource) ReadRfxCreator(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.RfxCreatorAuthority, error) {
	return m.actor, nil
}

func (m mountSource) ActiveCarrierInTenant(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}

func TestRfxSourceAuthorityMount(t *testing.T) {
	award := domain.RfxAwardSourceRecord{
		TenantID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), ID: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Version: 4, RfxEventID: uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		CarrierCompanyID: uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		EventPresent:     true, EventTenantID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		EventID: uuid.MustParse("33333333-3333-3333-3333-333333333333"), OwnerCompanyID: uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		EventStatus: domain.RfxStatusAwarded, EventLive: true,
	}
	link := domain.RfxAwardLinkSourceRecord{
		TenantID: award.TenantID, ID: uuid.MustParse("66666666-6666-6666-6666-666666666666"), Version: 2,
		RfxAwardID: award.ID, RfxEventID: award.RfxEventID, TransportOrderID: uuid.MustParse("77777777-7777-7777-7777-777777777777"),
		CarrierCompanyID: award.CarrierCompanyID, BuyerCompanyID: award.OwnerCompanyID,
		AwardPresent: true, AwardTenantID: award.TenantID, AwardEventID: award.RfxEventID, AwardCarrierID: award.CarrierCompanyID,
		EventPresent: true, EventTenantID: award.TenantID, EventID: award.RfxEventID, OwnerCompanyID: award.OwnerCompanyID,
		EventStatus: domain.RfxStatusAwarded, EventLive: true,
	}
	fingerprint, err := domain.RfxAwardSourceFingerprint(award)
	if err != nil {
		t.Fatal(err)
	}
	linkFingerprint, err := domain.RfxAwardLinkSourceFingerprint(link)
	if err != nil {
		t.Fatal(err)
	}
	const revalidationToken = "company-revalidation-token"
	const internalToken = "generic-internal-token"
	router := WithRfxSourceAuthority(http.NotFoundHandler(), revalidationToken, service.NewRfxSourceAuthority(mountSource{
		award: award, link: link, actor: domain.RfxCreatorAuthority{Active: true, ActiveMembership: true, CompanyRoles: []string{"PROCUREMENT_MANAGER"}},
	}))
	body := map[string]string{
		"source_type": domain.RfxSourceTypeAward, "source_id": award.ID.String(), "source_tenant_id": award.TenantID.String(),
		"source_version_or_fingerprint": fingerprint, "shipper_carrier_company_id": award.CarrierCompanyID.String(),
		"requesting_company_id": award.OwnerCompanyID.String(), "requesting_user_id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
	}
	payload, _ := json.Marshal(body)

	missing := postRfxSource(router, domain.RfxSourceConfirmPath, payload, "")
	wrong := postRfxSource(router, domain.RfxSourceConfirmPath, payload, internalToken)
	if missing.Code != http.StatusUnauthorized || wrong.Code != http.StatusUnauthorized || missing.Body.String() != "" || wrong.Body.String() != "" {
		t.Fatalf("auth failures must be identical 401s: %d %q / %d %q", missing.Code, missing.Body.String(), wrong.Code, wrong.Body.String())
	}

	spoof := httptest.NewRequest(http.MethodPost, domain.RfxSourceConfirmPath, bytes.NewReader(payload))
	spoof.Header.Set("X-Internal-Service-Token", revalidationToken)
	spoof.Header.Set("X-Internal-Service-Name", "company-service")
	spoofed := httptest.NewRecorder()
	router.ServeHTTP(spoofed, spoof)
	if spoofed.Code != http.StatusUnauthorized || spoofed.Body.Len() != 0 {
		t.Fatalf("generic internal headers must not authorize the callback: %d %s", spoofed.Code, spoofed.Body.String())
	}

	ok := postRfxSource(router, domain.RfxSourceConfirmPath, payload, revalidationToken)
	if ok.Code != http.StatusOK || !strings.Contains(ok.Body.String(), `"decision":"CONFIRMED"`) || strings.Contains(ok.Body.String(), "tenant_id") || strings.Contains(ok.Body.String(), "transport_order") {
		t.Fatalf("confirm response = %d %s", ok.Code, ok.Body.String())
	}

	linkBody := map[string]string{
		"source_type": domain.RfxSourceTypeAwardLink, "source_id": link.ID.String(), "source_tenant_id": link.TenantID.String(),
		"source_version_or_fingerprint": linkFingerprint, "shipper_carrier_company_id": link.CarrierCompanyID.String(),
		"requesting_company_id": link.OwnerCompanyID.String(), "requesting_user_id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
	}
	linkPayload, _ := json.Marshal(linkBody)
	linkOK := postRfxSource(router, domain.RfxSourceConfirmPath, linkPayload, revalidationToken)
	if linkOK.Code != http.StatusOK || !strings.Contains(linkOK.Body.String(), `"decision":"CONFIRMED"`) {
		t.Fatalf("link confirm = %d %s", linkOK.Code, linkOK.Body.String())
	}

	staleBody := map[string]string{
		"source_type": domain.RfxSourceTypeAward, "source_id": award.ID.String(), "source_tenant_id": award.TenantID.String(),
		"source_version_or_fingerprint": strings.Repeat("cd", 32), "shipper_carrier_company_id": award.CarrierCompanyID.String(),
	}
	stalePayload, _ := json.Marshal(staleBody)
	stale := postRfxSource(router, domain.RfxSourceRevalidatePath, stalePayload, revalidationToken)
	if stale.Code != http.StatusOK || !strings.Contains(stale.Body.String(), `"decision":"CHANGED"`) {
		t.Fatalf("stale revalidate = %d %s", stale.Code, stale.Body.String())
	}

	down := WithRfxSourceAuthority(http.NotFoundHandler(), revalidationToken, service.NewRfxSourceAuthority(mountSource{readErr: errors.New("db down")}))
	unavailable := postRfxSource(down, domain.RfxSourceRevalidatePath, stalePayload, revalidationToken)
	if unavailable.Code != http.StatusServiceUnavailable || !strings.Contains(unavailable.Body.String(), `"decision":"UNAVAILABLE"`) {
		t.Fatalf("unavailable = %d %s", unavailable.Code, unavailable.Body.String())
	}

	closed := WithRfxSourceAuthority(http.NotFoundHandler(), "", service.NewRfxSourceAuthority(mountSource{award: award}))
	refused := postRfxSource(closed, domain.RfxSourceConfirmPath, payload, revalidationToken)
	if refused.Code != http.StatusUnauthorized {
		t.Fatalf("empty credential = %d", refused.Code)
	}
	if sourceauth.Header() != "X-Company-Relationship-Revalidation-Token" {
		t.Fatal("revalidation header drifted")
	}

	other := httptest.NewRequest(http.MethodGet, "/health", nil)
	otherRec := httptest.NewRecorder()
	router.ServeHTTP(otherRec, other)
	if otherRec.Code != http.StatusNotFound {
		t.Fatalf("unrelated route = %d", otherRec.Code)
	}
}

func postRfxSource(handler http.Handler, path string, payload []byte, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	if token != "" {
		req.Header.Set(sourceauth.Header(), token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
