//go:build integration

package freightpaymentscore

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/payment-service/internal/config"
	httpserver "github.com/freight-platform/payment-service/internal/http"
	"github.com/freight-platform/payment-service/internal/http/handlers"
	"github.com/freight-platform/payment-service/internal/repository"
	"github.com/freight-platform/payment-service/internal/service"
	"github.com/freight-platform/shared-go/internalauth"
)

func TestFC_B_PAY_READ_001_InternalObligationByBillingRegister(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	registerItemID := uuid.New()
	settlementID := uuid.New()
	transportOrderID := uuid.New()
	shipmentID := uuid.New()
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO billing.freight_settlements (
			id, tenant_id, settlement_number, shipment_id, transport_order_id,
			buyer_company_id, carrier_company_id, currency_code, status,
			base_freight_amount, total_without_vat, version
		) VALUES ($1,$2,'FS-PAY',$3,$4,$5,$6,'RUB','APPROVED',83.33,83.33,1)`,
		settlementID, fix.TenantID, shipmentID, transportOrderID, fix.BuyerID, fix.CarrierID); err != nil {
		t.Fatalf("settlement: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO billing.billing_register_items (
			id, tenant_id, register_id, shipment_id, settlement_id,
			amount_without_vat, amount_with_vat, status
		) VALUES ($1,$2,$3,$4,$5,83.33,100.00,'INCLUDED')`,
		registerItemID, fix.TenantID, fix.RegisterID, shipmentID, settlementID); err != nil {
		t.Fatalf("register item: %v", err)
	}
	obligation, err := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}

	paymentRepo := repository.NewPaymentRepository(env.pool)
	registerLookup := repository.NewBillingRegisterLookupRepository(env.pool)
	membershipRepo := repository.NewMembershipRepository(env.pool)
	outboxRepo := repository.NewOutboxRepository(env.pool)
	paymentSvc := service.NewPaymentService(paymentRepo, registerLookup, membershipRepo, nil, outboxRepo)
	cfg := config.Config{InternalServiceToken: "pay-test-token", Environment: "test"}
	actor := handlers.NewPaymentActorResolver(membershipRepo)
	log := slog.New(slog.DiscardHandler)
	router := httpserver.NewRouter(log, env.pool, cfg, paymentSvc, actor)

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/payment-obligations/by-billing-register/"+fix.RegisterID.String(), nil)
	req.Header.Set(internalauth.HeaderName, "pay-test-token")
	req.Header.Set("X-Tenant-ID", fix.TenantID.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload["obligation_id"] != obligation.ID.String() {
		t.Fatalf("obligation_id = %v", payload["obligation_id"])
	}
	if payload["original_amount"] != "100.00" || payload["paid_amount"] != "0.00" {
		t.Fatalf("amounts = %v / %v", payload["original_amount"], payload["paid_amount"])
	}
	if payload["tax_basis"] != "WITH_VAT" {
		t.Fatalf("tax_basis = %v", payload["tax_basis"])
	}
	if payload["transport_order_id"] != transportOrderID.String() {
		t.Fatalf("transport_order_id = %v", payload["transport_order_id"])
	}
}
