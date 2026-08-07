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

	"github.com/freight-platform/shipment-service/internal/config"
	"github.com/freight-platform/shipment-service/internal/domain"
)

type fakeClock struct {
	now time.Time
}

func (f *fakeClock) Now() time.Time { return f.now }

type fakeRepo struct {
	mu      sync.Mutex
	events  []domain.ShipmentOutboxEvent
	claimed []domain.ShipmentOutboxEvent
}

func (f *fakeRepo) ClaimPendingForPublisher(_ context.Context, workerID string, batchSize int, now time.Time, _ time.Duration) ([]domain.ShipmentOutboxEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.events) == 0 {
		return nil, nil
	}
	limit := batchSize
	if limit > len(f.events) {
		limit = len(f.events)
	}
	out := make([]domain.ShipmentOutboxEvent, limit)
	copy(out, f.events[:limit])
	for i := range out {
		lockedAt := now
		lockedBy := workerID
		out[i].LockedAt = &lockedAt
		out[i].LockedBy = &lockedBy
		out[i].Attempts++
	}
	f.claimed = append(f.claimed, out...)
	f.events = f.events[limit:]
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
			status := domain.OutboxStatusPublished
			f.claimed[i].Status = status
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
			f.claimed[i].Status = domain.OutboxStatusFailed
			code := errorCode
			f.claimed[i].LastErrorCode = &code
			return nil
		}
	}
	return domain.ErrOutboxPublishStateConflict
}

func (f *fakeRepo) OutboxGaugeSnapshot(context.Context, time.Time) (int64, int64, float64, error) {
	return int64(len(f.events)), 0, 0, nil
}

func (f *fakeRepo) claimedStatus(eventID uuid.UUID) (domain.OutboxStatus, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, event := range f.claimed {
		if event.ID == eventID {
			return event.Status, true
		}
	}
	return "", false
}

func (f *fakeRepo) publishCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.claimed)
}

type fakePublisher struct {
	mu    sync.Mutex
	calls int
	err   error
	panic bool
}

func (f *fakePublisher) Publish(context.Context, domain.ShipmentOutboxEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.panic {
		panic("publisher panic")
	}
	return f.err
}

func sampleOutboxEvent() domain.ShipmentOutboxEvent {
	return domain.ShipmentOutboxEvent{
		ID:               uuid.New(),
		TenantID:         uuid.New(),
		AggregateType:    domain.OutboxAggregateTypeShipment,
		AggregateID:      uuid.New(),
		AggregateVersion: 2,
		EventType:        domain.OutboxEventTypeCreated,
		SchemaVersion:    domain.OutboxSchemaVersion,
		SourceEventID:    uuid.New(),
		Payload:          []byte(`{"eventType":"shipment.created"}`),
		Headers:          []byte(`{}`),
		Status:           domain.OutboxStatusPending,
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

func TestWorkerSuccessfulPublishMarksPublished(t *testing.T) {
	t.Parallel()
	event := sampleOutboxEvent()
	repo := &fakeRepo{events: []domain.ShipmentOutboxEvent{event}}
	publisher := &fakePublisher{}
	clock := &fakeClock{now: time.Now().UTC()}
	worker := NewWorker(testWorkerConfig(), repo, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)), clock)
	ctx, cancel := context.WithCancel(context.Background())
	worker.Start(ctx)
	waitForOutboxPublished(t, repo, event.ID, 2*time.Second)
	cancel()
	_ = worker.Wait(context.Background())

	if publisher.calls != 1 {
		t.Fatalf("publish calls=%d", publisher.calls)
	}
	if len(repo.claimed) != 1 || repo.claimed[0].Status != domain.OutboxStatusPublished {
		t.Fatalf("status=%s", repo.claimed[0].Status)
	}
}

func TestWorkerTransientErrorSchedulesRetry(t *testing.T) {
	t.Parallel()
	event := sampleOutboxEvent()
	repo := &fakeRepo{events: []domain.ShipmentOutboxEvent{event}}
	publisher := &fakePublisher{err: &PublishError{Code: ErrorCodeTransientNetwork, Retryable: true}}
	clock := &fakeClock{now: time.Now().UTC()}
	worker := NewWorker(testWorkerConfig(), repo, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)), clock)
	ctx, cancel := context.WithCancel(context.Background())
	worker.Start(ctx)
	waitForCondition(t, 2*time.Second, "released event for retry", func() bool {
		repo.mu.Lock()
		defer repo.mu.Unlock()
		return len(repo.events) == 1
	})
	cancel()
	_ = worker.Wait(context.Background())

	if len(repo.events) != 1 {
		t.Fatal("event must be released for retry")
	}
}

func TestWorkerPermanentErrorMarksFailed(t *testing.T) {
	t.Parallel()
	event := sampleOutboxEvent()
	repo := &fakeRepo{events: []domain.ShipmentOutboxEvent{event}}
	publisher := &fakePublisher{err: &PublishError{Code: ErrorCodePayloadRejected, Retryable: false}}
	cfg := testWorkerConfig()
	cfg.MaxAttempts = 1
	worker := NewWorker(cfg, repo, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeClock{now: time.Now().UTC()})
	ctx, cancel := context.WithCancel(context.Background())
	worker.Start(ctx)
	waitForCondition(t, 2*time.Second, "failed status", func() bool {
		status, ok := repo.claimedStatus(event.ID)
		return ok && status == domain.OutboxStatusFailed
	})
	cancel()
	_ = worker.Wait(context.Background())

	if repo.claimed[0].Status != domain.OutboxStatusFailed {
		t.Fatalf("status=%s", repo.claimed[0].Status)
	}
}

func TestWorkerDisabledDoesNotStart(t *testing.T) {
	t.Parallel()
	cfg := testWorkerConfig()
	cfg.Enabled = false
	worker := NewWorker(cfg, &fakeRepo{}, &fakePublisher{}, slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeClock{now: time.Now().UTC()})
	ctx := context.Background()
	worker.Start(ctx)
	if err := worker.Wait(context.Background()); err != nil {
		t.Fatalf("wait: %v", err)
	}
}

func TestWorkerPublisherPanicDoesNotMarkPublished(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{events: []domain.ShipmentOutboxEvent{sampleOutboxEvent()}}
	publisher := &fakePublisher{panic: true}
	worker := NewWorker(testWorkerConfig(), repo, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeClock{now: time.Now().UTC()})
	ctx, cancel := context.WithCancel(context.Background())
	worker.Start(ctx)
	waitForCondition(t, 2*time.Second, "worker handled panic", func() bool {
		return repo.publishCallCount() > 0
	})
	cancel()
	_ = worker.Wait(context.Background())
	if len(repo.claimed) > 0 && repo.claimed[0].Status == domain.OutboxStatusPublished {
		t.Fatal("panic must not silently mark published")
	}
}

func TestConfigValidationEnabledRequiresTransport(t *testing.T) {
	t.Parallel()
	cfg := config.OutboxConfig{
		Enabled:        true,
		Transport:      "",
		BatchSize:      1,
		MaxAttempts:    1,
		PublishTimeout: time.Second,
		LeaseTimeout:   2 * time.Second,
		PollInterval:   time.Second,
		WorkerID:       "w1",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestWorkerMaxAttemptsMarksFailed(t *testing.T) {
	t.Parallel()
	event := sampleOutboxEvent()
	event.Attempts = 4
	repo := &fakeRepo{events: []domain.ShipmentOutboxEvent{event}}
	publisher := &fakePublisher{err: errors.New("timeout")}
	cfg := testWorkerConfig()
	cfg.MaxAttempts = 5
	worker := NewWorker(cfg, repo, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeClock{now: time.Now().UTC()})
	ctx, cancel := context.WithCancel(context.Background())
	worker.Start(ctx)
	waitForCondition(t, 2*time.Second, "max attempts failed status", func() bool {
		status, ok := repo.claimedStatus(event.ID)
		return ok && status == domain.OutboxStatusFailed
	})
	cancel()
	_ = worker.Wait(context.Background())
	if repo.claimed[0].Status != domain.OutboxStatusFailed {
		t.Fatalf("status=%s attempts=%d", repo.claimed[0].Status, repo.claimed[0].Attempts)
	}
}

type trackingFakeRepo struct {
	fakeRepo
	claimCalls int
}

func (f *trackingFakeRepo) ClaimPendingForPublisher(ctx context.Context, workerID string, batchSize int, now time.Time, lease time.Duration) ([]domain.ShipmentOutboxEvent, error) {
	f.mu.Lock()
	f.claimCalls++
	f.mu.Unlock()
	return f.fakeRepo.ClaimPendingForPublisher(ctx, workerID, batchSize, now, lease)
}

type blockingPublisher struct {
	mu         sync.Mutex
	calls      int
	release    chan struct{}
	publishErr error
}

func (p *blockingPublisher) Publish(ctx context.Context, _ domain.ShipmentOutboxEvent) error {
	p.mu.Lock()
	p.calls++
	p.mu.Unlock()

	if p.publishErr != nil {
		return p.publishErr
	}

	select {
	case <-p.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *blockingPublisher) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

func TestWorkerImmediatePollBeforeTicker(t *testing.T) {
	t.Parallel()
	event := sampleOutboxEvent()
	repo := &trackingFakeRepo{fakeRepo: fakeRepo{events: []domain.ShipmentOutboxEvent{event}}}
	publisher := &fakePublisher{}
	cfg := testWorkerConfig()
	cfg.PollInterval = time.Second

	worker := NewWorker(cfg, repo, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeClock{now: time.Now().UTC()})
	ctx, cancel := context.WithCancel(context.Background())
	started := time.Now()
	worker.Start(ctx)
	waitForOutboxPublished(t, &repo.fakeRepo, event.ID, 200*time.Millisecond)
	elapsed := time.Since(started)
	cancel()
	_ = worker.Wait(context.Background())

	if elapsed >= cfg.PollInterval {
		t.Fatalf("expected immediate poll before ticker, elapsed=%s poll_interval=%s", elapsed, cfg.PollInterval)
	}
	if repo.claimCalls < 1 {
		t.Fatalf("claim calls=%d", repo.claimCalls)
	}
}

func TestWorkerContinuesTickerAfterImmediatePoll(t *testing.T) {
	t.Parallel()
	event := sampleOutboxEvent()
	repo := &trackingFakeRepo{fakeRepo: fakeRepo{events: []domain.ShipmentOutboxEvent{event}}}
	publisher := &fakePublisher{}
	cfg := testWorkerConfig()
	cfg.PollInterval = 20 * time.Millisecond

	worker := NewWorker(cfg, repo, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeClock{now: time.Now().UTC()})
	ctx, cancel := context.WithCancel(context.Background())
	worker.Start(ctx)
	waitForOutboxPublished(t, &repo.fakeRepo, event.ID, time.Second)
	waitForCondition(t, time.Second, "second poll cycle", func() bool {
		repo.mu.Lock()
		defer repo.mu.Unlock()
		return repo.claimCalls >= 2
	})
	cancel()
	_ = worker.Wait(context.Background())
}

func TestWorkerEmptyImmediatePollContinuesRunning(t *testing.T) {
	t.Parallel()
	repo := &trackingFakeRepo{}
	cfg := testWorkerConfig()
	cfg.PollInterval = 15 * time.Millisecond
	worker := NewWorker(cfg, repo, &fakePublisher{}, slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeClock{now: time.Now().UTC()})

	ctx, cancel := context.WithCancel(context.Background())
	worker.Start(ctx)
	waitForCondition(t, time.Second, "empty poll cycles", func() bool {
		repo.mu.Lock()
		defer repo.mu.Unlock()
		return repo.claimCalls >= 2
	})
	cancel()
	if err := worker.Wait(context.Background()); err != nil {
		t.Fatalf("wait: %v", err)
	}
}

func TestWorkerDisabledSkipsImmediatePoll(t *testing.T) {
	t.Parallel()
	repo := &trackingFakeRepo{fakeRepo: fakeRepo{events: []domain.ShipmentOutboxEvent{sampleOutboxEvent()}}}
	cfg := testWorkerConfig()
	cfg.Enabled = false
	worker := NewWorker(cfg, repo, &fakePublisher{}, slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeClock{now: time.Now().UTC()})

	worker.Start(context.Background())
	if err := worker.Wait(context.Background()); err != nil {
		t.Fatalf("wait: %v", err)
	}
	if repo.claimCalls != 0 {
		t.Fatalf("disabled worker claim calls=%d", repo.claimCalls)
	}
}

func TestWorkerCancelledBeforeStartSkipsClaim(t *testing.T) {
	t.Parallel()
	repo := &trackingFakeRepo{fakeRepo: fakeRepo{events: []domain.ShipmentOutboxEvent{sampleOutboxEvent()}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	worker := NewWorker(testWorkerConfig(), repo, &fakePublisher{}, slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeClock{now: time.Now().UTC()})
	worker.Start(ctx)
	_ = worker.Wait(context.Background())

	if repo.claimCalls != 0 {
		t.Fatalf("cancelled worker claim calls=%d", repo.claimCalls)
	}
}

func TestWorkerNoDoubleClaimImmediateAndTicker(t *testing.T) {
	t.Parallel()
	event := sampleOutboxEvent()
	repo := &trackingFakeRepo{fakeRepo: fakeRepo{events: []domain.ShipmentOutboxEvent{event}}}
	publisher := &fakePublisher{}
	cfg := testWorkerConfig()
	cfg.PollInterval = 5 * time.Millisecond

	worker := NewWorker(cfg, repo, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeClock{now: time.Now().UTC()})
	ctx, cancel := context.WithCancel(context.Background())
	worker.Start(ctx)
	waitForOutboxPublished(t, &repo.fakeRepo, event.ID, time.Second)
	waitForCondition(t, time.Second, "additional poll without second claim", func() bool {
		repo.mu.Lock()
		defer repo.mu.Unlock()
		return repo.claimCalls >= 2
	})
	cancel()
	_ = worker.Wait(context.Background())

	if publisher.calls != 1 {
		t.Fatalf("publish calls=%d", publisher.calls)
	}
	if len(repo.claimed) != 1 {
		t.Fatalf("claimed=%d", len(repo.claimed))
	}
	if repo.claimed[0].Attempts != 1 {
		t.Fatalf("attempts=%d", repo.claimed[0].Attempts)
	}
}

func TestWorkerNoConcurrentPollWithSlowPublisher(t *testing.T) {
	t.Parallel()
	event := sampleOutboxEvent()
	repo := &trackingFakeRepo{fakeRepo: fakeRepo{events: []domain.ShipmentOutboxEvent{event, sampleOutboxEvent()}}}
	publisher := &blockingPublisher{release: make(chan struct{})}
	cfg := testWorkerConfig()
	cfg.PollInterval = 2 * time.Millisecond

	worker := NewWorker(cfg, repo, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeClock{now: time.Now().UTC()})
	ctx, cancel := context.WithCancel(context.Background())
	worker.Start(ctx)

	waitForCondition(t, time.Second, "first publish blocked", func() bool {
		return publisher.callCount() == 1
	})
	for i := 0; i < 10; i++ {
		if publisher.callCount() > 1 {
			t.Fatalf("concurrent publish calls=%d want 1", publisher.callCount())
		}
		time.Sleep(2 * time.Millisecond)
	}

	close(publisher.release)
	waitForOutboxPublished(t, &repo.fakeRepo, event.ID, time.Second)
	cancel()
	_ = worker.Wait(context.Background())
}

func TestWorkerShutdownCancellationDoesNotMarkPublished(t *testing.T) {
	t.Parallel()
	event := sampleOutboxEvent()
	repo := &trackingFakeRepo{fakeRepo: fakeRepo{events: []domain.ShipmentOutboxEvent{event}}}
	publisher := &blockingPublisher{release: make(chan struct{})}
	worker := NewWorker(testWorkerConfig(), repo, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeClock{now: time.Now().UTC()})

	ctx, cancel := context.WithCancel(context.Background())
	worker.Start(ctx)
	waitForCondition(t, time.Second, "publish started", func() bool {
		return publisher.callCount() == 1
	})
	cancel()
	_ = worker.Wait(context.Background())

	status, ok := repo.claimedStatus(event.ID)
	if ok && status == domain.OutboxStatusPublished {
		t.Fatal("shutdown cancellation must not mark published")
	}
	if len(repo.events) == 0 && (!ok || status == domain.OutboxStatusPending) {
		return
	}
	if len(repo.events) == 1 {
		return
	}
	t.Fatalf("unexpected shutdown state status=%v released=%d", status, len(repo.events))
}

func TestWorkerContextCanceledClassifiedTransientTimeout(t *testing.T) {
	t.Parallel()
	classified := ClassifyPublishError(context.Canceled)
	if classified.Code != ErrorCodeTransientTimeout {
		t.Fatalf("code=%s", classified.Code)
	}
}
