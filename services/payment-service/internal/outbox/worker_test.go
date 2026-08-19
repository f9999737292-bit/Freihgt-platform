package outbox

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/payment-service/internal/config"
	"github.com/freight-platform/payment-service/internal/domain"
)

type fakeClock struct {
	now time.Time
}

func (f *fakeClock) Now() time.Time { return f.now }

type fakeRepo struct {
	mu      sync.Mutex
	events  []domain.PaymentOutboxEvent
	claimed []domain.PaymentOutboxEvent
}

func (f *fakeRepo) ClaimPendingForPublisher(_ context.Context, workerID string, batchSize int, now time.Time, _ time.Duration) ([]domain.PaymentOutboxEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	available := make([]domain.PaymentOutboxEvent, 0, len(f.events))
	for _, event := range f.events {
		if event.Status == domain.PaymentOutboxStatusPending && !event.AvailableAt.After(now) {
			available = append(available, event)
		}
	}
	if len(available) == 0 {
		return nil, nil
	}
	limit := batchSize
	if limit > len(available) {
		limit = len(available)
	}
	out := make([]domain.PaymentOutboxEvent, limit)
	copy(out, available[:limit])
	remaining := make([]domain.PaymentOutboxEvent, 0, len(f.events))
	claimedIDs := map[uuid.UUID]struct{}{}
	for i := range out {
		lockedAt := now
		lockedBy := workerID
		out[i].LockedAt = &lockedAt
		out[i].LockedBy = &lockedBy
		out[i].Attempts++
		claimedIDs[out[i].ID] = struct{}{}
	}
	for _, event := range f.events {
		if _, ok := claimedIDs[event.ID]; !ok {
			remaining = append(remaining, event)
		}
	}
	f.events = remaining
	f.claimed = append(f.claimed, out...)
	return out, nil
}

func (f *fakeRepo) MarkPublished(_ context.Context, eventID uuid.UUID, workerID string, publishedAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.claimed {
		if f.claimed[i].ID == eventID {
			if f.claimed[i].LockedBy == nil || *f.claimed[i].LockedBy != workerID {
				return domain.ErrOutboxPublishStateConflict
			}
			f.claimed[i].Status = domain.PaymentOutboxStatusPublished
			f.claimed[i].PublishedAt = &publishedAt
			return nil
		}
	}
	return domain.ErrOutboxPublishStateConflict
}

func (f *fakeRepo) ReleaseWithRetry(_ context.Context, eventID uuid.UUID, workerID string, availableAt time.Time, errorCode string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.claimed {
		if f.claimed[i].ID == eventID && f.claimed[i].LockedBy != nil && *f.claimed[i].LockedBy == workerID {
			f.claimed[i].AvailableAt = availableAt
			code := errorCode
			f.claimed[i].LastErrorCode = &code
			f.claimed[i].LockedAt = nil
			f.claimed[i].LockedBy = nil
			f.events = append(f.events, f.claimed[i])
			return nil
		}
	}
	return domain.ErrOutboxPublishStateConflict
}

func (f *fakeRepo) MarkFailed(_ context.Context, eventID uuid.UUID, workerID string, errorCode string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.claimed {
		if f.claimed[i].ID == eventID && f.claimed[i].LockedBy != nil && *f.claimed[i].LockedBy == workerID {
			f.claimed[i].Status = domain.PaymentOutboxStatusFailed
			code := errorCode
			f.claimed[i].LastErrorCode = &code
			return nil
		}
	}
	return domain.ErrOutboxPublishStateConflict
}

func (f *fakeRepo) OutboxGaugeSnapshot(context.Context, time.Time) (int64, int64, float64, error) {
	return 0, 0, 0, nil
}

func (f *fakeRepo) claimedStatus(eventID uuid.UUID) (domain.PaymentOutboxStatus, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, event := range f.claimed {
		if event.ID == eventID {
			return event.Status, true
		}
	}
	return "", false
}

func (f *fakeRepo) releasedAvailableAt(eventID uuid.UUID) (time.Time, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, event := range f.events {
		if event.ID == eventID {
			return event.AvailableAt, true
		}
	}
	return time.Time{}, false
}

type fakePublisher struct {
	mu    sync.Mutex
	calls int
	err   error
	panic bool
}

func (f *fakePublisher) Publish(context.Context, domain.PaymentOutboxEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.panic {
		panic("publisher panic")
	}
	return f.err
}

func samplePaymentOutboxEvent() domain.PaymentOutboxEvent {
	return domain.PaymentOutboxEvent{
		ID:          uuid.New(),
		TenantID:    uuid.New(),
		AggregateID: uuid.New(),
		EventType:   domain.PaymentEventObligationPaid,
		Status:      domain.PaymentOutboxStatusPending,
		AvailableAt: time.Now().UTC(),
	}
}

func testWorkerConfig() config.OutboxConfig {
	return config.OutboxConfig{
		Enabled:        true,
		PollInterval:   10 * time.Millisecond,
		BatchSize:      10,
		PublishTimeout: time.Second,
		LeaseTimeout:   2 * time.Second,
		MaxAttempts:    5,
		WorkerID:       "worker-test",
	}
}

func waitForCondition(t *testing.T, timeout time.Duration, label string, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", label)
}

func TestWorkerMaxAttemptsMarksFailed(t *testing.T) {
	t.Parallel()
	event := samplePaymentOutboxEvent()
	event.Attempts = 4
	repo := &fakeRepo{events: []domain.PaymentOutboxEvent{event}}
	publisher := &fakePublisher{err: errors.New("timeout")}
	cfg := testWorkerConfig()
	cfg.MaxAttempts = 5
	worker := NewWorker(cfg, repo, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeClock{now: time.Now().UTC()})
	ctx, cancel := context.WithCancel(context.Background())
	worker.Start(ctx)
	waitForCondition(t, 2*time.Second, "max attempts failed status", func() bool {
		status, ok := repo.claimedStatus(event.ID)
		return ok && status == domain.PaymentOutboxStatusFailed
	})
	cancel()
	_ = worker.Wait(context.Background())
	if repo.claimed[0].Status != domain.PaymentOutboxStatusFailed {
		t.Fatalf("status=%s attempts=%d", repo.claimed[0].Status, repo.claimed[0].Attempts)
	}
	if publisher.calls != 1 {
		t.Fatalf("ACTUAL_PUBLISH_ATTEMPTS_BEFORE_FAILED=%d want 1 publish on 5th attempt", publisher.calls)
	}
	if repo.claimed[0].Attempts != 5 {
		t.Fatalf("attempts=%d want 5", repo.claimed[0].Attempts)
	}
}

func TestWorkerMaxAttemptsOffByOneRetryBeforeLimit(t *testing.T) {
	t.Parallel()
	event := samplePaymentOutboxEvent()
	event.Attempts = 3
	repo := &fakeRepo{events: []domain.PaymentOutboxEvent{event}}
	publisher := &fakePublisher{err: &PublishError{Code: ErrorCodeTransientNetwork, Retryable: true}}
	cfg := testWorkerConfig()
	cfg.MaxAttempts = 5
	worker := NewWorker(cfg, repo, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeClock{now: time.Now().UTC()})
	ctx, cancel := context.WithCancel(context.Background())
	worker.Start(ctx)
	waitForCondition(t, 2*time.Second, "released for retry", func() bool {
		_, ok := repo.releasedAvailableAt(event.ID)
		return ok
	})
	cancel()
	_ = worker.Wait(context.Background())
	status, _ := repo.claimedStatus(event.ID)
	if status == domain.PaymentOutboxStatusFailed {
		t.Fatal("MAX_ATTEMPTS_OFF_BY_ONE=YES event failed before reaching limit")
	}
}

func TestWorkerBackoffSchedulesAvailableAt(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	event := samplePaymentOutboxEvent()
	event.AvailableAt = now
	repo := &fakeRepo{events: []domain.PaymentOutboxEvent{event}}
	publisher := &fakePublisher{err: &PublishError{Code: ErrorCodeBillingUnavailable, Retryable: true}}
	clock := &fakeClock{now: now}
	cfg := testWorkerConfig()
	cfg.MaxAttempts = 5
	worker := NewWorker(cfg, repo, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)), clock)
	ctx, cancel := context.WithCancel(context.Background())
	worker.Start(ctx)
	waitForCondition(t, 2*time.Second, "retry scheduled", func() bool {
		availableAt, ok := repo.releasedAvailableAt(event.ID)
		return ok && availableAt.After(now)
	})
	cancel()
	_ = worker.Wait(context.Background())

	availableAt, ok := repo.releasedAvailableAt(event.ID)
	if !ok {
		t.Fatal("BACKOFF_SCHEDULING=FAIL event not released")
	}
	expected := NextRetryAvailableAt(1, now)
	if !availableAt.Equal(expected) {
		t.Fatalf("available_at=%s want %s", availableAt, expected)
	}
	before, _ := repo.ClaimPendingForPublisher(context.Background(), cfg.WorkerID, 1, expected.Add(-time.Second), cfg.LeaseTimeout)
	if len(before) != 0 {
		t.Fatal("claim before available_at must not succeed")
	}
	after, err := repo.ClaimPendingForPublisher(context.Background(), cfg.WorkerID, 1, expected, cfg.LeaseTimeout)
	if err != nil {
		t.Fatalf("claim at available_at: %v", err)
	}
	if len(after) != 1 {
		t.Fatal("claim at/after available_at must succeed")
	}
}

func TestWorkerPoisonEventMarksFailedAndContinues(t *testing.T) {
	t.Parallel()
	poison := samplePaymentOutboxEvent()
	poison.EventType = "payment.voided"
	valid := samplePaymentOutboxEvent()
	payload, err := domain.BuildObligationPaidOutboxPayload(valid.TenantID, valid.AggregateID, uuid.New())
	if err != nil {
		t.Fatalf("payload: %v", err)
	}
	valid.Payload = payload
	repo := &fakeRepo{events: []domain.PaymentOutboxEvent{poison, valid}}
	publisher := NewHTTPPublisher(stubBillingClient{})
	cfg := testWorkerConfig()
	cfg.BatchSize = 2
	worker := NewWorker(cfg, repo, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeClock{now: time.Now().UTC()})
	ctx, cancel := context.WithCancel(context.Background())
	worker.Start(ctx)
	waitForCondition(t, 2*time.Second, "valid event published", func() bool {
		status, ok := repo.claimedStatus(valid.ID)
		return ok && status == domain.PaymentOutboxStatusPublished
	})
	cancel()
	_ = worker.Wait(context.Background())

	poisonStatus, ok := repo.claimedStatus(poison.ID)
	if !ok || poisonStatus != domain.PaymentOutboxStatusFailed {
		t.Fatalf("F8_POISON_EVENT=FAIL poison status=%v ok=%v", poisonStatus, ok)
	}
	for _, event := range repo.claimed {
		if event.ID == poison.ID {
			if event.LastErrorCode == nil || *event.LastErrorCode == "" {
				t.Fatal("poison event must populate last_error_code")
			}
			break
		}
	}
	validStatus, _ := repo.claimedStatus(valid.ID)
	if validStatus != domain.PaymentOutboxStatusPublished {
		t.Fatal("valid queued event must still process")
	}
}

type captureHandler struct {
	mu     sync.Mutex
	attrs  []slog.Attr
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	r.Attrs(func(a slog.Attr) bool {
		h.attrs = append(h.attrs, a)
		return true
	})
	return nil
}
func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(string) slog.Handler            { return h }

func TestWorkerLogsIncludeTenantID(t *testing.T) {
	t.Parallel()
	event := samplePaymentOutboxEvent()
	repo := &fakeRepo{events: []domain.PaymentOutboxEvent{event}}
	publisher := &fakePublisher{}
	handler := &captureHandler{}
	worker := NewWorker(testWorkerConfig(), repo, publisher, slog.New(handler), &fakeClock{now: time.Now().UTC()})
	ctx, cancel := context.WithCancel(context.Background())
	worker.Start(ctx)
	waitForCondition(t, 2*time.Second, "published", func() bool {
		status, ok := repo.claimedStatus(event.ID)
		return ok && status == domain.PaymentOutboxStatusPublished
	})
	cancel()
	_ = worker.Wait(context.Background())

	handler.mu.Lock()
	defer handler.mu.Unlock()
	found := false
	for _, attr := range handler.attrs {
		if attr.Key == "tenant_id" && attr.Value.String() == event.TenantID.String() {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("STRUCTURED_LOG_TENANT_ID=NO")
	}
}
