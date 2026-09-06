//go:build integration

package carrierresponse

import (
	"bytes"
	"context"
	"encoding/json"
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
	event := seedBrowserSecurityEvent(t, env, fix)

	rfxURL, rfxSrv := startBrowserRfxService(t, env)
	t.Cleanup(func() {
		ctx, cancel := withTimeout()
		defer cancel()
		_ = rfxSrv.Shutdown(ctx)
	})

	nonParticipantUser := uuid.New()
	nonParticipantCarrier := uuid.New()
	ctx := context.Background()
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1, $2, $3, $4)`,
		nonParticipantCarrier, fix.TenantID, "Carrier Non Participant", "CARRIER"); err != nil {
		t.Fatalf("seed non-participant carrier: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.users (id, tenant_id, email, full_name) VALUES ($1, $2, $3, $4)`,
		nonParticipantUser, fix.TenantID, "non-participant@test.local", "non-participant@test.local"); err != nil {
		t.Fatalf("seed non-participant user: %v", err)
	}
	var carrierRoleID uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = 'CARRIER_DISPATCHER' LIMIT 1`).Scan(&carrierRoleID); err != nil {
		t.Fatalf("lookup carrier role: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id) VALUES ($1, $2, $3)`,
		fix.TenantID, nonParticipantCarrier, nonParticipantUser); err != nil {
		t.Fatalf("seed non-participant membership: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1, $2, $3, $4)`,
		fix.TenantID, nonParticipantUser, nonParticipantCarrier, carrierRoleID); err != nil {
		t.Fatalf("seed non-participant role: %v", err)
	}

	identity := startBrowserIdentityStub(t, map[string][]string{
		fix.BuyerA.UserID.String():     {"PROCUREMENT_MANAGER"},
		fix.CarrierAct.UserID.String(): {"CARRIER_DISPATCHER"},
		nonParticipantUser.String():    {"CARRIER_DISPATCHER"},
	}, nil)
	companyStub := startBrowserCompanyStub(t, browserCarrierFixture{
		TenantID: fix.TenantID, BuyerCompanyID: fix.CompanyA, CarrierCompanyID: fix.CarrierID,
	})
	gatewayURL, gwProc := startBrowserProductionGateway(t, rfxURL, "http://127.0.0.1:3005", companyStub.URL(), identity)
	t.Cleanup(func() { dumpGatewayLogsOnFailure(t, gwProc) })

	carrierPath := gatewayURL + "/api/v1/rfx-events/" + event.ID.String() + "/carrier-response"
	startPath := carrierPath + "/start"
	answersPath := carrierPath + "/answers?carrier_company_id=" + fix.CarrierID.String()

	t.Run("no_token_deny", func(t *testing.T) {
		resp := gatewayRequest(t, http.MethodGet, carrierPath+"?carrier_company_id="+fix.CarrierID.String(), "", fix.CarrierID, nil, nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401 got %d body=%s", resp.StatusCode, resp.body)
		}
	})

	t.Run("bad_token_deny", func(t *testing.T) {
		resp := gatewayRequest(t, http.MethodGet, carrierPath+"?carrier_company_id="+fix.CarrierID.String(), "not-a-valid-jwt", fix.CarrierID, nil, nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401 got %d body=%s", resp.StatusCode, resp.body)
		}
	})

	t.Run("buyer_deny", func(t *testing.T) {
		token := browserCarrierJWT(fix.BuyerA.UserID, fix.TenantID)
		resp := gatewayRequest(t, http.MethodGet, carrierPath+"?carrier_company_id="+fix.CarrierID.String(), token, fix.CompanyA, nil, nil)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403 got %d body=%s", resp.StatusCode, resp.body)
		}
	})

	t.Run("non_participant_deny", func(t *testing.T) {
		token := browserCarrierJWT(nonParticipantUser, fix.TenantID)
		resp := gatewayRequest(t, http.MethodPost, startPath+"?carrier_company_id="+nonParticipantCarrier.String(), token, nonParticipantCarrier, nil, []byte("{}"))
		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 404/403 for non-participant got %d body=%s", resp.StatusCode, resp.body)
		}
	})

	t.Run("authorized_carrier_allow", func(t *testing.T) {
		token := browserCarrierJWT(fix.CarrierAct.UserID, fix.TenantID)
		resp := gatewayRequest(t, http.MethodPost, startPath+"?carrier_company_id="+fix.CarrierID.String(), token, fix.CarrierID, nil, []byte("{}"))
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 got %d body=%s", resp.StatusCode, resp.body)
		}
	})

	t.Run("header_spoof_ignored", func(t *testing.T) {
		spoofTenant := uuid.New()
		token := browserCarrierJWT(fix.CarrierAct.UserID, fix.TenantID)
		resp := gatewayRequest(t, http.MethodGet, carrierPath+"?carrier_company_id="+fix.CarrierID.String(), token, fix.CarrierID, map[string]string{
			"X-Tenant-ID": spoofTenant.String(),
			"X-User-ID":   uuid.New().String(),
		}, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 got %d body=%s", resp.StatusCode, resp.body)
		}
		headers := lastBrowserDownstreamHeaders()
		if got := headers.Get("X-Tenant-ID"); got != fix.TenantID.String() {
			t.Fatalf("downstream tenant=%q want verified %q", got, fix.TenantID)
		}
		if got := headers.Get("X-User-ID"); got != fix.CarrierAct.UserID.String() {
			t.Fatalf("downstream user=%q want verified %q", got, fix.CarrierAct.UserID)
		}
	})

	t.Run("patch_route_carrier_allow", func(t *testing.T) {
		token := browserCarrierJWT(fix.CarrierAct.UserID, fix.TenantID)
		body := map[string]any{"save_version": 0, "answers": []any{}}
		raw, _ := json.Marshal(body)
		resp := gatewayRequest(t, http.MethodPatch, answersPath, token, fix.CarrierID, nil, raw)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for patch answers got %d body=%s", resp.StatusCode, resp.body)
		}
	})
}

func seedBrowserSecurityEvent(t *testing.T, env *testEnv, fix buyerFixture) *domain.RfxEvent {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().UTC().Add(24 * time.Hour)
	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "Gateway Security",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-CR-GW-SEC-" + uuid.NewString()[:8],
		ResponseDeadline: &deadline,
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	if _, err := env.rfxSvc.AddParticipant(ctx, fix.BuyerA, event.ID, domain.AddRfxParticipantInput{
		TenantID: fix.TenantID, RfxEventID: event.ID, CompanyID: fix.CarrierID, ParticipantType: "CARRIER",
	}); err != nil {
		t.Fatalf("add participant: %v", err)
	}
	version := enableQuestionnaire(t, env, fix.BuyerA, event.ID)
	publishVersion(t, env, version.ID)
	if _, err := env.rfxSvc.PublishEvent(ctx, fix.BuyerA, event.ID); err != nil {
		t.Fatalf("publish event: %v", err)
	}
	return event
}

type gatewayHTTPResponse struct {
	StatusCode int
	body       string
}

func gatewayRequest(t *testing.T, method, url, token string, companyID uuid.UUID, spoofHeaders map[string]string, body []byte) gatewayHTTPResponse {
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
	req.Header.Set("Content-Type", "application/json")
	for k, v := range spoofHeaders {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return gatewayHTTPResponse{StatusCode: resp.StatusCode, body: string(raw)}
}

func withTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 15*time.Second)
}

func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
