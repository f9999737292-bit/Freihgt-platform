//go:build integration

package freightpaymentscore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/payment-service/internal/config"
	"github.com/freight-platform/payment-service/internal/domain"
	"github.com/freight-platform/payment-service/internal/outbox"
	"github.com/freight-platform/payment-service/internal/repository"
	"github.com/freight-platform/payment-service/internal/service"
)

func TestOutboxInsertFailureRollbackF1(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	err := env.paymentRepo.SimulateOutboxInsertFailureForTest(ctx, domain.CreateAllocationInput{
		TenantID: fix.TenantID, PaymentID: payment.ID, ObligationID: obligation.ID,
		AllocatedAmount: decimal.RequireFromString("100.00"), CreatedBy: fix.BuyerUserID,
		ActorCompanyID: fix.BuyerID, ActorKind: domain.PaymentActorBuyer,
	})
	if err == nil {
		t.Fatal("F1_TX_ROLLBACK_NO_EVENT=FAIL expected outbox insert failure")
	}
	reloaded, _ := env.paymentRepo.GetObligationByID(ctx, fix.TenantID, obligation.ID)
	if reloaded.Status == domain.ObligationStatusPaid {
		t.Fatal("obligation must not be PAID after rollback")
	}
	count, _ := env.outboxRepo.CountOutboxByAggregate(ctx, fix.TenantID, domain.PaymentEventObligationPaid, obligation.ID)
	if count != 0 {
		t.Fatal("outbox row must not exist after rollback")
	}
}

func TestPartialAllocationNoOutboxEvent(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "40.00")
	_, err := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("40.00"),
	}, buyerActor(fix))
	if err != nil {
		t.Fatalf("partial allocate: %v", err)
	}
	count, _ := env.outboxRepo.CountOutboxByAggregate(ctx, fix.TenantID, domain.PaymentEventObligationPaid, obligation.ID)
	if count != 0 {
		t.Fatal("PARTIAL_ALLOCATION_NO_EVENT=FAIL")
	}
}

func TestPaidTransitionCreatesOneOutboxEvent(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	_, err := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("100.00"),
	}, buyerActor(fix))
	if err != nil {
		t.Fatalf("allocate: %v", err)
	}
	count, _ := env.outboxRepo.CountOutboxByAggregate(ctx, fix.TenantID, domain.PaymentEventObligationPaid, obligation.ID)
	if count != 1 {
		t.Fatalf("PAID_TRANSITION_ONE_EVENT=FAIL count=%d", count)
	}
}

func TestDuplicateOutboxInsertBlocked(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payload, _ := domain.BuildObligationPaidOutboxPayload(fix.TenantID, obligation.ID, fix.RegisterID)
	_, err := env.pool.Exec(ctx, `
		INSERT INTO billing.payment_outbox (
			tenant_id, aggregate_type, aggregate_id, event_type, schema_version, payload, status
		) VALUES ($1,$2,$3,$4,$5,$6,'PENDING')`,
		fix.TenantID, domain.AggregatePaymentObligation, obligation.ID, domain.PaymentEventObligationPaid, 1, payload,
	)
	if err != nil {
		t.Fatalf("seed outbox: %v", err)
	}
	_, err = env.pool.Exec(ctx, `
		INSERT INTO billing.payment_outbox (
			tenant_id, aggregate_type, aggregate_id, event_type, schema_version, payload, status
		) VALUES ($1,$2,$3,$4,$5,$6,'PENDING')`,
		fix.TenantID, domain.AggregatePaymentObligation, obligation.ID, domain.PaymentEventObligationPaid, 1, payload,
	)
	if err == nil {
		t.Fatal("DUPLICATE_OUTBOX_INSERT=FAIL expected unique violation")
	}
}

func TestBillingDownLeavesOutboxPendingF2(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	env.payments = service.NewPaymentService(env.paymentRepo, repository.NewBillingRegisterLookupRepository(env.pool), repository.NewMembershipRepository(env.pool), failingBillingSync{}, env.outboxRepo)
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	outcome, err := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("100.00"),
	}, buyerActor(fix))
	if err != nil {
		t.Fatalf("allocate: %v", err)
	}
	if outcome.RegisterPaidProjection == nil || outcome.RegisterPaidProjection.Status != service.RegisterPaidProjectionFailed {
		t.Fatal("F2_BILLING_DOWN_PENDING=FAIL projection failure not visible")
	}
	event, err := env.outboxRepo.GetOutboxByAggregate(ctx, fix.TenantID, domain.PaymentEventObligationPaid, obligation.ID)
	if err != nil || event == nil || event.Status != domain.PaymentOutboxStatusPending {
		t.Fatal("F2_BILLING_DOWN_PENDING=FAIL outbox must remain PENDING")
	}
}

func TestDirectSyncSuccessMarksPublished(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	billingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer billingServer.Close()
	env.payments = service.NewPaymentService(env.paymentRepo, repository.NewBillingRegisterLookupRepository(env.pool), repository.NewMembershipRepository(env.pool), service.NewBillingRegisterHTTPClient(billingServer.URL, "token"), env.outboxRepo)
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	_, err := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("100.00"),
	}, buyerActor(fix))
	if err != nil {
		t.Fatalf("allocate: %v", err)
	}
	event, err := env.outboxRepo.GetOutboxByAggregate(ctx, fix.TenantID, domain.PaymentEventObligationPaid, obligation.ID)
	if err != nil || event == nil || event.Status != domain.PaymentOutboxStatusPublished {
		t.Fatal("DIRECT_SYNC_SUCCESS=FAIL outbox must be PUBLISHED")
	}
}

func TestOnePaymentMultiObligationTwoOutboxRows(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	registerB := uuid.New()
	if _, err := env.pool.Exec(ctx, `INSERT INTO billing.billing_registers (
		id, tenant_id, register_number, customer_company_id, contractor_company_id,
		period_from, period_to, currency_code, status,
		total_without_vat, vat_amount, total_with_vat
	) VALUES ($1,$2,$3,$4,$5,now(),now(),'RUB','SIGNED_BY_COUNTERPARTY','166.67','33.33','200.00')`,
		registerB, fix.TenantID, "REG-B", fix.BuyerID, fix.CarrierID); err != nil {
		t.Fatalf("register b: %v", err)
	}
	obligationA, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	obligationB, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, registerB)
	payment := createManualPayment(t, env, fix, "300.00")
	if _, err := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligationA.ID, AllocatedAmount: decimal.RequireFromString("100.00"),
	}, buyerActor(fix)); err != nil {
		t.Fatalf("allocate A: %v", err)
	}
	if _, err := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligationB.ID, AllocatedAmount: decimal.RequireFromString("200.00"),
	}, buyerActor(fix)); err != nil {
		t.Fatalf("allocate B: %v", err)
	}
	countA, _ := env.outboxRepo.CountOutboxByAggregate(ctx, fix.TenantID, domain.PaymentEventObligationPaid, obligationA.ID)
	countB, _ := env.outboxRepo.CountOutboxByAggregate(ctx, fix.TenantID, domain.PaymentEventObligationPaid, obligationB.ID)
	if countA != 1 || countB != 1 {
		t.Fatalf("ONE_PAYMENT_MULTI_OBLIGATION=FAIL counts A=%d B=%d", countA, countB)
	}
}

func TestWorkerPublishesPendingEvent(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	env.payments = service.NewPaymentService(env.paymentRepo, repository.NewBillingRegisterLookupRepository(env.pool), repository.NewMembershipRepository(env.pool), failingBillingSync{}, env.outboxRepo)
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	if _, err := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("100.00"),
	}, buyerActor(fix)); err != nil {
		t.Fatalf("allocate: %v", err)
	}

	var syncCount atomic.Int32
	billingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		syncCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer billingServer.Close()
	billingClient := service.NewBillingRegisterHTTPClient(billingServer.URL, "token")
	publisher := outbox.NewHTTPPublisher(billingClient)
	worker := outbox.NewWorker(config.OutboxConfig{
		Enabled: true, WorkerID: "test-worker", BatchSize: 10, MaxAttempts: 5,
		PublishTimeout: 5 * time.Second, LeaseTimeout: 10 * time.Second,
	}, env.outboxRepo, publisher, nil, outbox.NewRealClock())

	events, err := env.outboxRepo.ClaimPendingForPublisher(ctx, "test-worker", 10, time.Now().UTC(), 10*time.Second)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	var target *domain.PaymentOutboxEvent
	for i := range events {
		if events[i].AggregateID == obligation.ID && events[i].EventType == domain.PaymentEventObligationPaid {
			target = &events[i]
			break
		}
	}
	if target == nil {
		t.Fatalf("expected claimed event for obligation %s, got %d events", obligation.ID, len(events))
	}
	worker.ProcessEventForIntegration(ctx, *target)
	if syncCount.Load() != 1 {
		t.Fatalf("worker must call billing once, got %d", syncCount.Load())
	}
	event, _ := env.outboxRepo.GetOutboxByAggregate(ctx, fix.TenantID, domain.PaymentEventObligationPaid, obligation.ID)
	if event.Status != domain.PaymentOutboxStatusPublished {
		t.Fatal("worker must mark event PUBLISHED")
	}
}

func TestWorkerClosedAlreadySatisfiedF6(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payload, _ := domain.BuildObligationPaidOutboxPayload(fix.TenantID, obligation.ID, fix.RegisterID)
	if _, err := env.pool.Exec(ctx, `
		UPDATE billing.payment_obligations
		SET status='PAID', paid_amount=original_amount, outstanding_amount=0
		WHERE id=$1`, obligation.ID); err != nil {
		t.Fatalf("set obligation paid: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		UPDATE billing.billing_registers SET status='CLOSED' WHERE id=$1`, fix.RegisterID); err != nil {
		t.Fatalf("set register closed: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO billing.payment_outbox (
			tenant_id, aggregate_type, aggregate_id, event_type, schema_version, payload, status
		) VALUES ($1,$2,$3,$4,$5,$6,'PENDING')`,
		fix.TenantID, domain.AggregatePaymentObligation, obligation.ID, domain.PaymentEventObligationPaid, 1, payload,
	); err != nil {
		t.Fatalf("seed outbox: %v", err)
	}

	billingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"CLOSED"}`))
	}))
	defer billingServer.Close()
	billingClient := service.NewBillingRegisterHTTPClient(billingServer.URL, "token")
	worker := outbox.NewWorker(config.OutboxConfig{
		Enabled: true, WorkerID: "closed-worker", BatchSize: 10, MaxAttempts: 5,
		PublishTimeout: 5 * time.Second, LeaseTimeout: 10 * time.Second,
	}, env.outboxRepo, outbox.NewHTTPPublisher(billingClient), nil, outbox.NewRealClock())
	events, err := env.outboxRepo.ClaimPendingForPublisher(ctx, "closed-worker", 10, time.Now().UTC(), 10*time.Second)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	var target *domain.PaymentOutboxEvent
	for i := range events {
		if events[i].AggregateID == obligation.ID && events[i].EventType == domain.PaymentEventObligationPaid {
			target = &events[i]
			break
		}
	}
	if target == nil {
		t.Fatalf("expected claimed event for obligation %s, got %d events", obligation.ID, len(events))
	}
	worker.ProcessEventForIntegration(ctx, *target)
	event, _ := env.outboxRepo.GetOutboxByAggregate(ctx, fix.TenantID, domain.PaymentEventObligationPaid, obligation.ID)
	if event.Status != domain.PaymentOutboxStatusPublished {
		t.Fatal("F6_ALREADY_CLOSED=FAIL outbox must be PUBLISHED")
	}
	var registerStatus string
	if err := env.pool.QueryRow(ctx, `SELECT status FROM billing.billing_registers WHERE id=$1`, fix.RegisterID).Scan(&registerStatus); err != nil {
		t.Fatalf("register status: %v", err)
	}
	if registerStatus != "CLOSED" {
		t.Fatal("register must remain CLOSED")
	}
}

func TestOutboxPayloadIdentifiersOnly(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	if _, err := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("100.00"),
	}, buyerActor(fix)); err != nil {
		t.Fatalf("allocate: %v", err)
	}
	event, _ := env.outboxRepo.GetOutboxByAggregate(ctx, fix.TenantID, domain.PaymentEventObligationPaid, obligation.ID)
	var payload map[string]string
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("payload decode: %v", err)
	}
	if _, ok := payload["paid_amount"]; ok {
		t.Fatal("payload must not contain paid_amount")
	}
	if payload["obligation_id"] == "" || payload["register_id"] == "" || payload["tenant_id"] == "" {
		t.Fatal("payload must contain identifiers")
	}
}
