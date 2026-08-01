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
	repo := &fakeRepo{events: []domain.ShipmentOutboxEvent{sampleOutboxEvent()}}
	publisher := &fakePublisher{}
	clock := &fakeClock{now: time.Now().UTC()}
	worker := NewWorker(testWorkerConfig(), repo, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)), clock)
	ctx, cancel := context.WithCancel(context.Background())
	worker.Start(ctx)
	time.Sleep(30 * time.Millisecond)
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
	repo := &fakeRepo{events: []domain.ShipmentOutboxEvent{sampleOutboxEvent()}}
	publisher := &fakePublisher{err: &PublishError{Code: ErrorCodeTransientNetwork, Retryable: true}}
	clock := &fakeClock{now: time.Now().UTC()}
	worker := NewWorker(testWorkerConfig(), repo, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)), clock)
	ctx, cancel := context.WithCancel(context.Background())
	worker.Start(ctx)
	time.Sleep(30 * time.Millisecond)
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
	time.Sleep(30 * time.Millisecond)
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
	time.Sleep(30 * time.Millisecond)
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
	time.Sleep(30 * time.Millisecond)
	cancel()
	_ = worker.Wait(context.Background())
	if repo.claimed[0].Status != domain.OutboxStatusFailed {
		t.Fatalf("status=%s attempts=%d", repo.claimed[0].Status, repo.claimed[0].Attempts)
	}
}
