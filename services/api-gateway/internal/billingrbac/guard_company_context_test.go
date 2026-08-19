package billingrbac

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/api-gateway/internal/companycontext"
	"github.com/freight-platform/api-gateway/internal/config"
	gwmiddleware "github.com/freight-platform/api-gateway/internal/http/middleware"
)

func serveThroughAuth(t *testing.T, handler http.Handler, req *http.Request, secret string) *httptest.ResponseRecorder {
	t.Helper()
	chain := gwmiddleware.Auth(true, secret)(handler)
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)
	return rec
}

func signTestToken(t *testing.T, secret, userID, tenantID string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"tenant_id": tenantID,
		"email":     "user@example.com",
		"sub":       userID,
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func TestCompanyContextRejectsMissingCompanyID(t *testing.T) {
	tenantID := uuid.New().String()
	userID := uuid.New().String()
	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"CARRIER_ADMIN"}})
	}))
	defer identityServer.Close()

	downstreamCalled := false
	guard := NewGuard(config.Config{
		AuthEnabled:         true,
		ProxyTimeoutSeconds: 5,
		Services:            config.ServiceURLs{Identity: identityServer.URL},
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstreamCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	token := signTestToken(t, "secret", userID, tenantID)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/billing-registers", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := serveThroughAuth(t, guard.WithPolicy(PolicyRead), req, "secret")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", rec.Code)
	}
	if downstreamCalled {
		t.Fatal("downstream should not be called without company_id")
	}
}

func TestCompanyContextRejectsCompanyMembershipSpoof(t *testing.T) {
	tenantID := uuid.New().String()
	userID := uuid.New().String()
	carrierCompany := uuid.New().String()
	buyerCompany := uuid.New().String()

	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/auth/me"):
			_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"CARRIER_ADMIN"}})
		case strings.Contains(r.URL.Path, "/companies"):
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{{
				"company_id":   carrierCompany,
				"company_type": "CARRIER",
				"roles":        []map[string]any{{"code": "CARRIER_ADMIN"}},
			}}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer identityServer.Close()

	downstreamCalled := false
	guard := NewGuard(config.Config{
		AuthEnabled:         true,
		ProxyTimeoutSeconds: 5,
		Services:            config.ServiceURLs{Identity: identityServer.URL},
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstreamCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	token := signTestToken(t, "secret", userID, tenantID)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/billing-registers?company_id="+buyerCompany+"&actor=BUYER", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := serveThroughAuth(t, guard.WithPolicy(PolicyRead), req, "secret")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", rec.Code)
	}
	if downstreamCalled {
		t.Fatal("downstream should not be called for company spoof")
	}
}

func TestCompanyContextRejectsActorKindSpoof(t *testing.T) {
	tenantID := uuid.New().String()
	userID := uuid.New().String()
	carrierCompany := uuid.New().String()

	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/auth/me"):
			_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"CARRIER_ADMIN"}})
		case strings.Contains(r.URL.Path, "/companies"):
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{{
				"company_id":   carrierCompany,
				"company_type": "CARRIER",
				"roles":        []map[string]any{{"code": "CARRIER_ADMIN"}},
			}}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer identityServer.Close()

	downstreamCalled := false
	guard := NewGuard(config.Config{
		AuthEnabled:         true,
		ProxyTimeoutSeconds: 5,
		Services:            config.ServiceURLs{Identity: identityServer.URL},
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstreamCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	token := signTestToken(t, "secret", userID, tenantID)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/billing-registers?company_id="+carrierCompany+"&actor=BUYER", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := serveThroughAuth(t, guard.WithPolicy(PolicyRead), req, "secret")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", rec.Code)
	}
	if downstreamCalled {
		t.Fatal("downstream should not be called for actor spoof")
	}
}

func TestCompanyContextInjectsVerifiedHeadersAndStripsSpoofedHeaders(t *testing.T) {
	tenantID := uuid.New().String()
	userID := uuid.New().String()
	buyerCompany := uuid.New().String()

	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/auth/me"):
			_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"SHIPPER_ADMIN"}})
		case strings.Contains(r.URL.Path, "/companies"):
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{{
				"company_id":   buyerCompany,
				"company_type": "SHIPPER",
				"roles":        []map[string]any{{"code": "SHIPPER_ADMIN"}},
			}}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer identityServer.Close()

	var gotCompany, gotActor string
	guard := NewGuard(config.Config{
		AuthEnabled:         true,
		ProxyTimeoutSeconds: 5,
		Services:            config.ServiceURLs{Identity: identityServer.URL},
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCompany = r.Header.Get(companycontext.HeaderCompanyID)
		gotActor = r.Header.Get(companycontext.HeaderActorKind)
		w.WriteHeader(http.StatusOK)
	}))

	token := signTestToken(t, "secret", userID, tenantID)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/billing-registers?company_id="+buyerCompany, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set(companycontext.HeaderCompanyID, uuid.New().String())
	req.Header.Set(companycontext.HeaderActorKind, companycontext.ActorCarrier)
	rec := serveThroughAuth(t, guard.WithPolicy(PolicyRead), req, "secret")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200 body=%s", rec.Code, rec.Body.String())
	}
	if gotCompany != buyerCompany {
		t.Fatalf("company header=%q want %q", gotCompany, buyerCompany)
	}
	if gotActor != companycontext.ActorBuyer {
		t.Fatalf("actor header=%q want BUYER", gotActor)
	}
}
