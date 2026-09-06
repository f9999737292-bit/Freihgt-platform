//go:build integration

package studio

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
)

func TestBrowserProductionGateway_SecurityProofs(t *testing.T) {
	if strings.TrimSpace(getEnv("TEST_DATABASE_URL", "")) == "" {
		t.Fatal("TEST_DATABASE_URL is required")
	}
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-GW-SEC-1")
	enableQuestionnaire(t, env, fix.BuyerA, event.ID)

	rfxURL, rfxSrv := startBrowserRfxService(t, env)
	t.Cleanup(func() {
		ctx, cancel := withTimeout()
		defer cancel()
		_ = rfxSrv.Shutdown(ctx)
	})

	identity := startBrowserIdentityStub(t, map[string][]string{
		fix.BuyerA.UserID.String():       {"PROCUREMENT_MANAGER"},
		fix.CarrierAct.UserID.String():   {"CARRIER_DISPATCHER"},
		fix.NoMembership.UserID.String(): {"PROCUREMENT_MANAGER"},
	})
	gatewayURL, gwProc := startBrowserProductionGateway(t, rfxURL, "http://127.0.0.1:3020", identity)
	t.Cleanup(func() { dumpGatewayLogsOnFailure(t, gwProc) })

	studioPath := gatewayURL + "/api/v1/rfx-events/" + event.ID.String() + "/studio"

	t.Run("no_token_deny", func(t *testing.T) {
		resp := gatewayRequest(t, http.MethodGet, studioPath, "", fix.CompanyA, nil, nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401 got %d body=%s", resp.StatusCode, resp.body)
		}
	})

	t.Run("bad_token_deny", func(t *testing.T) {
		resp := gatewayRequest(t, http.MethodGet, studioPath, "not-a-valid-jwt", fix.CompanyA, nil, nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401 got %d body=%s", resp.StatusCode, resp.body)
		}
	})

	t.Run("wrong_role_deny", func(t *testing.T) {
		token := browserStudioJWT(fix.CarrierAct.UserID, fix.TenantID)
		resp := gatewayRequest(t, http.MethodGet, studioPath, token, fix.CarrierID, nil, nil)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403 got %d body=%s", resp.StatusCode, resp.body)
		}
	})

	t.Run("buyer_allow", func(t *testing.T) {
		token := browserStudioJWT(fix.BuyerA.UserID, fix.TenantID)
		resp := gatewayRequest(t, http.MethodGet, studioPath, token, fix.CompanyA, nil, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 got %d body=%s", resp.StatusCode, resp.body)
		}
	})

	t.Run("tenant_header_spoof_ignored", func(t *testing.T) {
		spoofTenant := uuid.New()
		token := browserStudioJWT(fix.BuyerA.UserID, fix.TenantID)
		resp := gatewayRequest(t, http.MethodGet, studioPath, token, fix.CompanyA, map[string]string{
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
		if got := headers.Get("X-User-ID"); got != fix.BuyerA.UserID.String() {
			t.Fatalf("downstream user=%q want verified %q", got, fix.BuyerA.UserID)
		}
	})

	t.Run("user_header_spoof_ignored", func(t *testing.T) {
		token := browserStudioJWT(fix.BuyerA.UserID, fix.TenantID)
		resp := gatewayRequest(t, http.MethodGet, studioPath, token, fix.CompanyA, map[string]string{
			"X-User-ID": uuid.New().String(),
		}, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 got %d body=%s", resp.StatusCode, resp.body)
		}
		if got := lastBrowserDownstreamHeaders().Get("X-User-ID"); got != fix.BuyerA.UserID.String() {
			t.Fatalf("downstream user=%q want verified JWT user", got)
		}
	})

	t.Run("foreign_company_mutation_denied", func(t *testing.T) {
		token := browserStudioJWT(fix.BuyerB.UserID, fix.TenantID)
		sectionPath := gatewayURL + "/api/v1/rfx-events/" + event.ID.String() + "/sections"
		body := map[string]any{"section_code": "SEC-FGN", "title": "Foreign"}
		raw, _ := json.Marshal(body)
		resp := gatewayRequest(t, http.MethodPost, sectionPath, token, fix.CompanyB, nil, raw)
		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 404/403 for unowned company got %d body=%s", resp.StatusCode, resp.body)
		}
	})

	scorePath := gatewayURL + "/api/v1/rfx-events/" + event.ID.String() + "/score-model"
	t.Run("score_model_carrier_deny", func(t *testing.T) {
		token := browserStudioJWT(fix.CarrierAct.UserID, fix.TenantID)
		resp := gatewayRequest(t, http.MethodGet, scorePath, token, fix.CarrierID, nil, nil)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403 for carrier score-model got %d body=%s", resp.StatusCode, resp.body)
		}
	})
	t.Run("score_model_buyer_allow", func(t *testing.T) {
		token := browserStudioJWT(fix.BuyerA.UserID, fix.TenantID)
		resp := gatewayRequest(t, http.MethodGet, scorePath, token, fix.CompanyA, nil, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for buyer score-model got %d body=%s", resp.StatusCode, resp.body)
		}
	})
	t.Run("score_model_header_spoof_ignored", func(t *testing.T) {
		token := browserStudioJWT(fix.BuyerA.UserID, fix.TenantID)
		resp := gatewayRequest(t, http.MethodGet, scorePath, token, fix.CompanyA, map[string]string{
			"X-Tenant-ID": uuid.New().String(),
			"X-User-ID":   uuid.New().String(),
		}, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 got %d body=%s", resp.StatusCode, resp.body)
		}
		if got := lastBrowserDownstreamHeaders().Get("X-Tenant-ID"); got != fix.TenantID.String() {
			t.Fatalf("score-model downstream tenant=%q want %q", got, fix.TenantID)
		}
		if got := lastBrowserDownstreamHeaders().Get("X-User-ID"); got != fix.BuyerA.UserID.String() {
			t.Fatalf("score-model downstream user=%q want %q", got, fix.BuyerA.UserID)
		}
	})
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
