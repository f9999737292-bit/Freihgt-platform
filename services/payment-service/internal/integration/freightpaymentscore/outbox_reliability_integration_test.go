//go:build integration

package freightpaymentscore

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
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

func seedPendingOutboxEvent(t *testing.T, env *env, fix fixture, obligationID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	payload, err := domain.BuildObligationPaidOutboxPayload(fix.TenantID, obligationID, fix.RegisterID)
	if err != nil {
		t.Fatalf("payload: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO billing.payment_outbox (
			tenant_id, aggregate_type, aggregate_id, event_type, schema_version, payload, status
		) VALUES ($1,$2,$3,$4,$5,$6,'PENDING')`,
		fix.TenantID, domain.AggregatePaymentObligation, obligationID, domain.PaymentEventObligationPaid, 1, payload,
	); err != nil {
		t.Fatalf("seed outbox: %v", err)
	}
}

type markPublishedFailRepo struct {
	*repository.OutboxRepository
	failRemaining int
	mu            sync.Mutex
}

func (r *markPublishedFailRepo) MarkPublished(ctx context.Context, eventID uuid.UUID, workerID string, publishedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failRemaining > 0 {
		r.failRemaining--
		return domain.ErrOutboxPublishStateConflict
	}
	return r.OutboxRepository.MarkPublished(ctx, eventID, workerID, publishedAt)
}

type failingOutboxProjectionStore struct {
	inner    *repository.OutboxRepository
	failOnce bool
	failed   bool
}

func (f *failingOutboxProjectionStore) MarkPublishedByAggregate(ctx context.Context, tenantID uuid.UUID, eventType string, aggregateID uuid.UUID, publishedAt time.Time) error {
	if f.failOnce && !f.failed {
		f.failed = true
		return domain.ErrOutboxPublishStateConflict
	}
	return f.inner.MarkPublishedByAggregate(ctx, tenantID, eventType, aggregateID, publishedAt)
}

func TestDirectSyncMarkPublishedFailureVisible(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	billingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer billingServer.Close()
	failOutbox := &failingOutboxProjectionStore{inner: env.outboxRepo, failOnce: true}
	env.payments = service.NewPaymentService(
		env.paymentRepo,
		repository.NewBillingRegisterLookupRepository(env.pool),
		repository.NewMembershipRepository(env.pool),
		service.NewBillingRegisterHTTPClient(billingServer.URL, "test-token"),
		failOutbox,
	)
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	outcome, err := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("100.00"),
	}, buyerActor(fix))
	if err != nil {
		t.Fatalf("allocate: %v", err)
	}
	if outcome.RegisterPaidProjection == nil || outcome.RegisterPaidProjection.Status != service.RegisterPaidProjectionSynced {
		t.Fatal("DIRECT_SYNC_MARK_PUBLISHED_FAILURE_VISIBLE=FAIL billing sync must succeed")
	}
	if outcome.OutboxProjection == nil || outcome.OutboxProjection.Status != service.OutboxProjectionMarkFailed || !outcome.OutboxProjection.Retryable {
		t.Fatal("DIRECT_SYNC_MARK_PUBLISHED_FAILURE_VISIBLE=FAIL projection warning missing")
	}
	reloaded, _ := env.paymentRepo.GetObligationByID(ctx, fix.TenantID, obligation.ID)
	if reloaded.Status != domain.ObligationStatusPaid {
		t.Fatal("FINANCIAL_STATE_ROLLED_BACK=YES obligation must remain PAID")
	}
	event, _ := env.outboxRepo.GetOutboxByAggregate(ctx, fix.TenantID, domain.PaymentEventObligationPaid, obligation.ID)
	if event == nil || event.Status != domain.PaymentOutboxStatusPending {
		t.Fatal("OUTBOX_REMAINS_RETRYABLE=NO outbox must stay PENDING")
	}
}

func TestCrashAfterBillingSuccessF3(t *testing.T) {
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

	billingServer := httptest.NewServer(billingSyncPaidHandler(env.pool))
	defer billingServer.Close()
	billingClient := service.NewBillingRegisterHTTPClient(billingServer.URL, "test-token")
	publisher := outbox.NewHTTPPublisher(billingClient)
	leaseTimeout := 2 * time.Second
	now := time.Now().UTC()

	failRepo := &markPublishedFailRepo{OutboxRepository: env.outboxRepo, failRemaining: 1}
	workerA := outbox.NewWorker(config.OutboxConfig{
		Enabled: true, WorkerID: "worker-a", BatchSize: 10, MaxAttempts: 5,
		PublishTimeout: 5 * time.Second, LeaseTimeout: leaseTimeout,
	}, failRepo, publisher, nil, outbox.NewRealClock())

	eventsA, err := env.outboxRepo.ClaimPendingForPublisher(ctx, "worker-a", 100, now, leaseTimeout)
	if err != nil {
		t.Fatalf("claim A: %v", err)
	}
	target := findEventByAggregate(eventsA, obligation.ID, domain.PaymentEventObligationPaid)
	if target == nil {
		target = claimAggregateEvent(t, env, ctx, fix.TenantID, "worker-a", now, leaseTimeout, obligation.ID)
	}
	workerA.ProcessEventForIntegration(ctx, *target)

	eventAfterCrash, _ := env.outboxRepo.GetOutboxByAggregate(ctx, fix.TenantID, domain.PaymentEventObligationPaid, obligation.ID)
	if eventAfterCrash.Status != domain.PaymentOutboxStatusPending {
		t.Fatal("F3_CRASH_AFTER_BILLING_SUCCESS=FAIL event must remain PENDING after mark failure")
	}
	auditAfterFirst := countMarkPaidAuditEvents(t, env.pool, fix.RegisterID)
	if auditAfterFirst != 1 {
		t.Fatalf("expected one MARKED_PAID audit after first billing sync, got %d", auditAfterFirst)
	}

	staleAt := now.Add(-leaseTimeout - time.Second)
	if _, err := env.pool.Exec(ctx, `UPDATE billing.payment_outbox SET locked_at=$1 WHERE tenant_id=$2 AND aggregate_id=$3 AND event_type=$4`,
		staleAt, fix.TenantID, obligation.ID, domain.PaymentEventObligationPaid); err != nil {
		t.Fatalf("expire lease: %v", err)
	}

	reclaimAt := now.Add(leaseTimeout + time.Second)
	reclaimed := claimAggregateEvent(t, env, ctx, fix.TenantID, "worker-b", reclaimAt, leaseTimeout, obligation.ID)
	workerB := outbox.NewWorker(config.OutboxConfig{
		Enabled: true, WorkerID: "worker-b", BatchSize: 10, MaxAttempts: 5,
		PublishTimeout: 5 * time.Second, LeaseTimeout: leaseTimeout,
	}, env.outboxRepo, publisher, nil, outbox.NewRealClock())
	workerB.ProcessEventForIntegration(ctx, *reclaimed)

	finalEvent, _ := env.outboxRepo.GetOutboxByAggregate(ctx, fix.TenantID, domain.PaymentEventObligationPaid, obligation.ID)
	if finalEvent.Status != domain.PaymentOutboxStatusPublished {
		t.Fatal("F3_CRASH_AFTER_BILLING_SUCCESS=FAIL event must end PUBLISHED")
	}
	if auditAfterSecond := countMarkPaidAuditEvents(t, env.pool, fix.RegisterID); auditAfterSecond != auditAfterFirst {
		t.Fatalf("DUPLICATE_MARKED_PAID_AUDIT=YES audits before=%d after=%d", auditAfterFirst, auditAfterSecond)
	}
	var registerStatus string
	if err := env.pool.QueryRow(ctx, `SELECT status FROM billing.billing_registers WHERE id=$1`, fix.RegisterID).Scan(&registerStatus); err != nil {
		t.Fatalf("register status: %v", err)
	}
	if registerStatus != "PAID" {
		t.Fatalf("register must remain PAID, got %s", registerStatus)
	}
}

func TestTwoWorkersSingleOwnerF4(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	seedPendingOutboxEvent(t, env, fix, obligation.ID)

	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make([][]domain.PaymentOutboxEvent, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		workerID := fmt.Sprintf("worker-%c", 'A'+i)
		go func(idx int, id string) {
			defer wg.Done()
			<-start
			events, err := env.outboxRepo.ClaimPendingForPublisher(ctx, id, 1, time.Now().UTC(), 10*time.Second)
			if err != nil {
				t.Errorf("claim %s: %v", id, err)
				return
			}
			results[idx] = events
		}(i, workerID)
	}
	close(start)
	wg.Wait()

	claimed := 0
	for _, batch := range results {
		for _, event := range batch {
			if event.AggregateID == obligation.ID {
				claimed++
			}
		}
	}
	if claimed != 1 {
		t.Fatalf("FOR_UPDATE_SKIP_LOCKED=FAIL MULTI_WORKER_SINGLE_OWNER=FAIL claimed=%d", claimed)
	}
}

func TestLeaseRecoveryF4(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	seedPendingOutboxEvent(t, env, fix, obligation.ID)

	leaseTimeout := 3 * time.Second
	now := time.Now().UTC()
	eventsA, err := env.outboxRepo.ClaimPendingForPublisher(ctx, "worker-a", 1, now, leaseTimeout)
	if err != nil {
		t.Fatalf("claim A: %v", err)
	}
	if len(eventsA) != 1 {
		t.Fatalf("expected one claimed event, got %d", len(eventsA))
	}
	staleAt := now.Add(-leaseTimeout - time.Second)
	if _, err := env.pool.Exec(ctx, `UPDATE billing.payment_outbox SET locked_at=$1, locked_by='worker-a' WHERE id=$2`, staleAt, eventsA[0].ID); err != nil {
		t.Fatalf("force stale lease: %v", err)
	}
	reclaimAt := now.Add(leaseTimeout + time.Second)
	eventsB, err := env.outboxRepo.ClaimPendingForPublisher(ctx, "worker-b", 1, reclaimAt, leaseTimeout)
	if err != nil {
		t.Fatalf("claim B: %v", err)
	}
	if len(eventsB) != 1 || eventsB[0].LockedBy == nil || *eventsB[0].LockedBy != "worker-b" {
		t.Fatal("LEASE_RECOVERY=FAIL worker B must reclaim event")
	}
}

func TestInvalidCanonicalObligationF7(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	seedPendingOutboxEvent(t, env, fix, obligation.ID)
	if _, err := env.pool.Exec(ctx, `
		UPDATE billing.payment_obligations
		SET status='PARTIALLY_PAID', paid_amount='40.00', outstanding_amount='60.00'
		WHERE id=$1`, obligation.ID); err != nil {
		t.Fatalf("invalidate obligation: %v", err)
	}

	beforeStatus := "SIGNED_BY_COUNTERPARTY"
	billingServer := httptest.NewServer(billingSyncPaidHandler(env.pool))
	defer billingServer.Close()
	billingClient := service.NewBillingRegisterHTTPClient(billingServer.URL, "test-token")
	worker := outbox.NewWorker(config.OutboxConfig{
		Enabled: true, WorkerID: "f7-worker", BatchSize: 10, MaxAttempts: 5,
		PublishTimeout: 5 * time.Second, LeaseTimeout: 10 * time.Second,
	}, env.outboxRepo, outbox.NewHTTPPublisher(billingClient), nil, outbox.NewRealClock())
	events, err := env.outboxRepo.ClaimPendingForPublisher(ctx, "f7-worker", 10, time.Now().UTC(), 10*time.Second)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	target := findEventByAggregate(events, obligation.ID, domain.PaymentEventObligationPaid)
	if target == nil {
		t.Fatal("expected claimed outbox event")
	}
	worker.ProcessEventForIntegration(ctx, *target)

	event, _ := env.outboxRepo.GetOutboxByAggregate(ctx, fix.TenantID, domain.PaymentEventObligationPaid, obligation.ID)
	if event.Status != domain.PaymentOutboxStatusFailed {
		t.Fatalf("F7_INVALID_OBLIGATION=FAIL status=%s", event.Status)
	}
	if event.LastErrorCode == nil || *event.LastErrorCode != outbox.ErrorCodeIntegrityViolation {
		t.Fatalf("expected integrity error code, got=%v", event.LastErrorCode)
	}
	var status string
	if err := env.pool.QueryRow(ctx, `SELECT status FROM billing.billing_registers WHERE id=$1`, fix.RegisterID).Scan(&status); err != nil {
		t.Fatalf("register status: %v", err)
	}
	if status != beforeStatus {
		t.Fatalf("billing register changed to %s", status)
	}
	if countMarkPaidAuditEvents(t, env.pool, fix.RegisterID) != 0 {
		t.Fatal("no fabricated PAID audit must be created")
	}
}

func TestBackoffSchedulingIntegration(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	seedPendingOutboxEvent(t, env, fix, obligation.ID)

	now := time.Now().UTC()
	events, err := env.outboxRepo.ClaimPendingForPublisher(ctx, "backoff-worker", 1, now, 10*time.Second)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one event, got %d", len(events))
	}
	availableAt := outbox.NextRetryAvailableAt(events[0].Attempts, now)
	if err := env.outboxRepo.ReleaseWithRetry(ctx, events[0].ID, "backoff-worker", availableAt, outbox.ErrorCodeBillingUnavailable); err != nil {
		t.Fatalf("release: %v", err)
	}
	before, err := env.outboxRepo.ClaimPendingForPublisher(ctx, "backoff-worker", 1, availableAt.Add(-time.Second), 10*time.Second)
	if err != nil {
		t.Fatalf("claim before: %v", err)
	}
	for _, event := range before {
		if event.AggregateID == obligation.ID {
			t.Fatal("BACKOFF_SCHEDULING=FAIL claim before available_at must not succeed")
		}
	}
	after, err := env.outboxRepo.ClaimPendingForPublisher(ctx, "backoff-worker", 1, availableAt, 10*time.Second)
	if err != nil {
		t.Fatalf("claim after: %v", err)
	}
	if findEventByAggregate(after, obligation.ID, domain.PaymentEventObligationPaid) == nil {
		t.Fatal("BACKOFF_SCHEDULING=FAIL claim at available_at must succeed")
	}
}

func TestOutboxMigrationUsableInPostgres(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligationID := uuid.New()
	payload, err := domain.BuildObligationPaidOutboxPayload(fix.TenantID, obligationID, fix.RegisterID)
	if err != nil {
		t.Fatalf("payload: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO billing.payment_outbox (
			id, tenant_id, aggregate_type, aggregate_id, event_type, schema_version, payload, status
		) VALUES ($1,$2,$3,$4,$5,$6,$7,'PENDING')`,
		uuid.New(), fix.TenantID, domain.AggregatePaymentObligation, obligationID, domain.PaymentEventObligationPaid, 1, payload,
	); err != nil {
		t.Fatalf("migration table unusable: %v", err)
	}
	count, err := env.outboxRepo.CountOutboxByAggregate(ctx, fix.TenantID, domain.PaymentEventObligationPaid, obligationID)
	if err != nil || count != 1 {
		t.Fatalf("expected readable outbox row, count=%d err=%v", count, err)
	}
}

func findEventByAggregate(events []domain.PaymentOutboxEvent, aggregateID uuid.UUID, eventType string) *domain.PaymentOutboxEvent {
	for i := range events {
		if events[i].AggregateID == aggregateID && events[i].EventType == eventType {
			return &events[i]
		}
	}
	return nil
}

func claimAggregateEvent(t *testing.T, env *env, ctx context.Context, tenantID uuid.UUID, workerID string, at time.Time, lease time.Duration, aggregateID uuid.UUID) *domain.PaymentOutboxEvent {
	t.Helper()
	for attempt := 0; attempt < 8; attempt++ {
		events, err := env.outboxRepo.ClaimPendingForPublisher(ctx, workerID, 100, at, lease)
		if err != nil {
			t.Fatalf("claim %s: %v", workerID, err)
		}
		if target := findEventByAggregate(events, aggregateID, domain.PaymentEventObligationPaid); target != nil {
			return target
		}
		for _, event := range events {
			if err := env.outboxRepo.ReleaseWithRetry(ctx, event.ID, workerID, at, outbox.ErrorCodeUnknownPublishError); err != nil {
				t.Fatalf("release unrelated event: %v", err)
			}
		}
	}
	row, _ := env.outboxRepo.GetOutboxByAggregate(ctx, tenantID, domain.PaymentEventObligationPaid, aggregateID)
	t.Fatalf("could not claim aggregate %s status=%v locked_by=%v", aggregateID, row.Status, row.LockedBy)
	return nil
}
