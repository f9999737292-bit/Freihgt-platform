//go:build integration

package scoringv3

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestScoreModelGatewaySecurityProofs(t *testing.T) {
	if strings.TrimSpace(os.Getenv("TEST_DATABASE_URL")) == "" {
		t.Fatal("TEST_DATABASE_URL is required")
	}
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	sf := seedScoringFixture(t, env, fix)

	rfxURL, rfxSrv := startScoringHTTPServer(t, env)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = rfxSrv.Shutdown(ctx)
	})

	identity := startScoringIdentityStub(t, map[string][]string{
		fix.BuyerA.UserID.String():     {"PROCUREMENT_MANAGER"},
		fix.BuyerB.UserID.String():     {"PROCUREMENT_MANAGER"},
		fix.CarrierAct.UserID.String(): {"CARRIER_DISPATCHER"},
	})
	gatewayURL, _ := startScoringProductionGateway(t, rfxURL, identity)
	scorePath := gatewayURL + "/api/v1/rfx-events/" + sf.Event.ID.String() + "/score-model"

	t.Run("carrier_deny", func(t *testing.T) {
		token := scoringBuyerJWT(fix.CarrierAct.UserID, fix.TenantID)
		status, body := scoringGatewayRequest(t, http.MethodGet, scorePath, token, fix.CarrierID, nil, nil)
		if status != http.StatusForbidden {
			t.Fatalf("expected 403 got %d body=%s", status, body)
		}
	})

	t.Run("buyer_allow", func(t *testing.T) {
		token := scoringBuyerJWT(fix.BuyerA.UserID, fix.TenantID)
		status, body := scoringGatewayRequest(t, http.MethodGet, scorePath, token, fix.CompanyA, nil, nil)
		if status != http.StatusOK {
			t.Fatalf("expected 200 got %d body=%s", status, body)
		}
	})

	t.Run("foreign_company_deny", func(t *testing.T) {
		token := scoringBuyerJWT(fix.BuyerB.UserID, fix.TenantID)
		status, body := scoringGatewayRequest(t, http.MethodGet, scorePath, token, fix.CompanyB, nil, nil)
		if status != http.StatusNotFound && status != http.StatusForbidden {
			t.Fatalf("expected 404/403 got %d body=%s", status, body)
		}
	})

	t.Run("header_spoof_deny", func(t *testing.T) {
		spoofTenant := uuid.New()
		spoofUser := uuid.New()
		token := scoringBuyerJWT(fix.BuyerA.UserID, fix.TenantID)
		status, body := scoringGatewayRequest(t, http.MethodGet, scorePath, token, fix.CompanyA, map[string]string{
			"X-Tenant-ID":  spoofTenant.String(),
			"X-User-ID":    spoofUser.String(),
			"X-Company-ID": uuid.New().String(),
		}, nil)
		if status != http.StatusOK {
			t.Fatalf("authorized buyer should pass gateway auth, got %d body=%s", status, body)
		}
		headers := lastScoringDownstreamHeaders()
		if got := headers.Get("X-Tenant-ID"); got != fix.TenantID.String() {
			t.Fatalf("HEADER_SPOOF_DENY downstream tenant=%q want JWT tenant %q", got, fix.TenantID)
		}
		if got := headers.Get("X-User-ID"); got != fix.BuyerA.UserID.String() {
			t.Fatalf("HEADER_SPOOF_DENY downstream user=%q want JWT user %q", got, fix.BuyerA.UserID)
		}
	})
}
