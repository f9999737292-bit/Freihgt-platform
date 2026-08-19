//go:build integration

package freightbillingclosing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/freight-platform/billing-register-service/internal/domain"
	"github.com/freight-platform/billing-register-service/internal/http/handlers"
	"github.com/freight-platform/billing-register-service/internal/repository"
)

func Test61CarrierActorBuyerSpoofDeniedViaTrustedResolver(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	membershipRepo := repository.NewMembershipRepository(env.pool)
	resolver := handlers.NewSettlementActorResolver(membershipRepo)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/billing-registers/id/calculate", nil)
	req.Header.Set("X-Tenant-ID", fix.TenantID.String())
	req.Header.Set("X-User-ID", fix.UserID.String())
	req.Header.Set(domain.HeaderCompanyID, fix.BuyerID.String())
	req.Header.Set(domain.HeaderActorKind, domain.SettlementActorBuyer)
	q := req.URL.Query()
	q.Set("company_id", fix.BuyerID.String())
	q.Set("actor", domain.SettlementActorBuyer)
	req.URL.RawQuery = q.Encode()

	_, err := resolver.FromRequest(req)
	assertForbidden(t, err)
}

func Test62ValidBuyerMembershipAllowViaTrustedResolver(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	membershipRepo := repository.NewMembershipRepository(env.pool)
	resolver := handlers.NewSettlementActorResolver(membershipRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/billing-registers", nil)
	req.Header.Set("X-Tenant-ID", fix.TenantID.String())
	req.Header.Set("X-User-ID", fix.BuyerUserID.String())
	req.Header.Set(domain.HeaderCompanyID, fix.BuyerID.String())
	req.Header.Set(domain.HeaderActorKind, domain.SettlementActorBuyer)
	q := req.URL.Query()
	q.Set("company_id", fix.BuyerID.String())
	req.URL.RawQuery = q.Encode()

	actor, err := resolver.FromRequest(req)
	if err != nil {
		t.Fatalf("valid buyer membership: %v", err)
	}
	if actor.ActorCompanyID != fix.BuyerID || actor.ActorKind != domain.SettlementActorBuyer {
		t.Fatalf("unexpected actor: %+v", actor)
	}
}

func Test63ValidCarrierReadAllowViaTrustedResolver(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := createRegister(t, env, fix, "BR-BC-63")
	membershipRepo := repository.NewMembershipRepository(env.pool)
	resolver := handlers.NewSettlementActorResolver(membershipRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/billing-registers/"+reg.ID.String(), nil)
	req.Header.Set("X-Tenant-ID", fix.TenantID.String())
	req.Header.Set("X-User-ID", fix.UserID.String())
	req.Header.Set(domain.HeaderCompanyID, fix.CarrierA.String())
	req.Header.Set(domain.HeaderActorKind, domain.SettlementActorCarrier)
	q := req.URL.Query()
	q.Set("company_id", fix.CarrierA.String())
	req.URL.RawQuery = q.Encode()

	actor, err := resolver.FromRequest(req)
	if err != nil {
		t.Fatalf("valid carrier membership: %v", err)
	}
	if _, err := env.registers.GetByID(context.Background(), reg.ID, fix.TenantID, actor); err != nil {
		t.Fatalf("carrier read allow: %v", err)
	}
}

func Test64DeniedSpoofProducesNoRegisterAuditRows(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := createRegister(t, env, fix, "BR-BC-64")
	before := countRegisterAuditEvents(t, env.pool, reg.ID)
	_, err := env.registers.Calculate(context.Background(), reg.ID, carrierActor(fix))
	assertForbidden(t, err)
	if after := countRegisterAuditEvents(t, env.pool, reg.ID); after != before {
		t.Fatalf("audit rows changed on denied spoof: before=%d after=%d", before, after)
	}
}

func Test65BuyerToCarrierSpoofDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	membershipRepo := repository.NewMembershipRepository(env.pool)
	resolver := handlers.NewSettlementActorResolver(membershipRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/billing-registers", nil)
	req.Header.Set("X-Tenant-ID", fix.TenantID.String())
	req.Header.Set("X-User-ID", fix.BuyerUserID.String())
	req.Header.Set(domain.HeaderCompanyID, fix.BuyerID.String())
	req.Header.Set(domain.HeaderActorKind, domain.SettlementActorCarrier)
	q := req.URL.Query()
	q.Set("company_id", fix.BuyerID.String())
	req.URL.RawQuery = q.Encode()

	_, err := resolver.FromRequest(req)
	assertForbidden(t, err)
}

func Test66CompanyHeaderSpoofDeniedWhenMembershipMismatch(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	membershipRepo := repository.NewMembershipRepository(env.pool)
	resolver := handlers.NewSettlementActorResolver(membershipRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/billing-registers", nil)
	req.Header.Set("X-Tenant-ID", fix.TenantID.String())
	req.Header.Set("X-User-ID", fix.UserID.String())
	req.Header.Set(domain.HeaderCompanyID, fix.BuyerID.String())
	req.Header.Set(domain.HeaderActorKind, domain.SettlementActorBuyer)
	q := req.URL.Query()
	q.Set("company_id", fix.BuyerID.String())
	req.URL.RawQuery = q.Encode()

	_, err := resolver.FromRequest(req)
	assertForbidden(t, err)
}
