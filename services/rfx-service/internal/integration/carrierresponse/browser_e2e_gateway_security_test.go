//go:build integration

package carrierresponse

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
)

func TestBrowserProductionGateway_CarrierResponseSecurityProofs(t *testing.T) {
	if strings.TrimSpace(getEnv("TEST_DATABASE_URL", "")) == "" {
		t.Fatal("TEST_DATABASE_URL is required")
	}
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	deadline := time.Now().UTC().Add(24 * time.Hour)
	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "GW Sec",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-GW-CR-1",
		ResponseDeadline: &deadline,
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	if _, err := env.rfxSvc.AddParticipant(ctx, fix.BuyerA, event.ID, domain.AddRfxParticipantInput{
		TenantID: fix.TenantID, RfxEventID: event.ID, CompanyID: fix.CarrierID, ParticipantType: "CARRIER",
	}); err != nil {
		t.Fatalf("participant: %v", err)
	}
	enableQuestionnaire(t, env, fix.BuyerA, event.ID)
	if _, err := env.rfxSvc.PublishEvent(ctx, fix.BuyerA, event.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}

	rfxURL, rfxSrv := startBrowserCarrierRfxService(t, env)
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = rfxSrv.Shutdown(c)
	})

	identity := startBrowserIdentityStub(t, map[string][]string{
		fix.BuyerA.UserID.String():   {"PROCUREMENT_MANAGER"},
		fix.CarrierAct.UserID.String(): {"CARRIER_DISPATCHER"},
	})
	gatewayURL, gwProc := startBrowserProductionGateway(t, rfxURL, "http://127.0.0.1:3005", identity)
	t.Cleanup(func() { dumpGatewayLogsOnFailure(t, gwProc) })

	path := gatewayURL + "/api/v1/rfx-events/" + event.ID.String() + "/carrier-response"

	t.Run("no_token_deny", func(t *testing.T) {
		resp := gatewayRequest(t, http.MethodGet, path, "", fix.CarrierID, nil, nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401 got %d", resp.StatusCode)
		}
	})
	t.Run("bad_token_deny", func(t *testing.T) {
		resp := gatewayRequest(t, http.MethodGet, path, "bad", fix.CarrierID, nil, nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401 got %d", resp.StatusCode)
		}
	})
	t.Run("buyer_role_deny", func(t *testing.T) {
		token := browserCarrierJWT(fix.BuyerA.UserID, fix.TenantID)
		resp := gatewayRequest(t, http.MethodGet, path, token, fix.CompanyA, nil, nil)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403 got %d body=%s", resp.StatusCode, resp.body)
		}
	})
	t.Run("authorized_carrier_allow", func(t *testing.T) {
		token := browserCarrierJWT(fix.CarrierAct.UserID, fix.TenantID)
		resp := gatewayRequest(t, http.MethodGet, path+"?carrier_company_id="+fix.CarrierID.String(), token, fix.CarrierID, nil, nil)
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected allow got %d body=%s", resp.StatusCode, resp.body)
		}
	})
	t.Run("tenant_header_spoof_ignored", func(t *testing.T) {
		token := browserCarrierJWT(fix.CarrierAct.UserID, fix.TenantID)
		resp := gatewayRequest(t, http.MethodGet, path+"?carrier_company_id="+fix.CarrierID.String(), token, fix.CarrierID, map[string]string{
			"X-Tenant-ID": uuid.New().String(),
			"X-User-ID":   uuid.New().String(),
		}, nil)
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 200 got %d", resp.StatusCode)
		}
		if got := lastBrowserDownstreamHeaders().Get("X-Tenant-ID"); got != fix.TenantID.String() {
			t.Fatalf("downstream tenant=%q want %q", got, fix.TenantID)
		}
	})
}

type gwResp struct {
	StatusCode int
	body       string
}

func gatewayRequest(t *testing.T, method, url, token string, companyID uuid.UUID, extraHeaders map[string]string, body []byte) gwResp {
	t.Helper()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if companyID != uuid.Nil {
		req.Header.Set("X-Company-ID", companyID.String())
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return gwResp{StatusCode: resp.StatusCode, body: string(raw)}
}

func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
