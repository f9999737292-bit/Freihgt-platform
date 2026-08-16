package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
)

type deadlineMockStore struct {
	mockRfxStore
	listExpiredFn   func(ctx context.Context, now time.Time, limit int) ([]domain.ExpiredResponseOpenEvent, error)
	updateStatusFn  func(ctx context.Context, id, tenantID uuid.UUID, expected, newStatus string) (*domain.RfxEvent, error)
	updateStatusMu  sync.Mutex
	updateStatusHit int
}

func (m *deadlineMockStore) ListExpiredResponseOpenEvents(ctx context.Context, now time.Time, limit int) ([]domain.ExpiredResponseOpenEvent, error) {
	if m.listExpiredFn != nil {
		return m.listExpiredFn(ctx, now, limit)
	}
	return nil, nil
}

func (m *deadlineMockStore) UpdateEventStatus(ctx context.Context, id, tenantID uuid.UUID, expected, newStatus string) (*domain.RfxEvent, error) {
	m.updateStatusMu.Lock()
	m.updateStatusHit++
	m.updateStatusMu.Unlock()
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, id, tenantID, expected, newStatus)
	}
	return &domain.RfxEvent{ID: id, TenantID: tenantID, Status: newStatus}, nil
}

type recordingAudit struct {
	mu      sync.Mutex
	records []repository.AuditRecord
}

func (r *recordingAudit) Record(_ context.Context, rec repository.AuditRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, rec)
	return nil
}

func (r *recordingAudit) countAction(action string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, rec := range r.records {
		if rec.Action == action {
			n++
		}
	}
	return n
}

func TestProcessExpiredResponseDeadlinesClosesExpiredEvent(t *testing.T) {
	t.Parallel()
	eventID := uuid.New()
	tenantID := uuid.New()
	past := time.Now().UTC().Add(-time.Hour)
	audit := &recordingAudit{}
	store := &deadlineMockStore{
		listExpiredFn: func(_ context.Context, now time.Time, _ int) ([]domain.ExpiredResponseOpenEvent, error) {
			return []domain.ExpiredResponseOpenEvent{{
				ID: eventID, TenantID: tenantID, Status: domain.RfxStatusResponsesOpen, ResponseDeadline: &past,
			}}, nil
		},
	}
	svc := NewRfxService(store, audit, nil)
	examined, closed, failures, err := svc.ProcessExpiredResponseDeadlines(context.Background(), time.Now().UTC(), 10)
	if err != nil || examined != 1 || closed != 1 || failures != 0 {
		t.Fatalf("examined=%d closed=%d failures=%d err=%v", examined, closed, failures, err)
	}
	if audit.countAction("auto_close_responses") != 1 {
		t.Fatal("expected auto-close audit")
	}
}

func TestProcessExpiredResponseDeadlinesSkipsFutureDeadline(t *testing.T) {
	t.Parallel()
	future := time.Now().UTC().Add(time.Hour)
	store := &deadlineMockStore{
		listExpiredFn: func(_ context.Context, _ time.Time, _ int) ([]domain.ExpiredResponseOpenEvent, error) {
			return []domain.ExpiredResponseOpenEvent{{
				ID: uuid.New(), TenantID: uuid.New(), Status: domain.RfxStatusResponsesOpen, ResponseDeadline: &future,
			}}, nil
		},
		updateStatusFn: func(context.Context, uuid.UUID, uuid.UUID, string, string) (*domain.RfxEvent, error) {
			t.Fatal("should not update future deadline event")
			return nil, nil
		},
	}
	svc := NewRfxService(store, nil, nil)
	_, closed, _, err := svc.ProcessExpiredResponseDeadlines(context.Background(), time.Now().UTC(), 10)
	if err != nil || closed != 0 {
		t.Fatalf("closed=%d err=%v", closed, err)
	}
}

func TestProcessExpiredResponseDeadlinesBoundaryEqualNow(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	eventID := uuid.New()
	tenantID := uuid.New()
	store := &deadlineMockStore{
		listExpiredFn: func(_ context.Context, at time.Time, _ int) ([]domain.ExpiredResponseOpenEvent, error) {
			return []domain.ExpiredResponseOpenEvent{{
				ID: eventID, TenantID: tenantID, Status: domain.RfxStatusResponsesOpen, ResponseDeadline: &now,
			}}, nil
		},
	}
	svc := NewRfxService(store, &recordingAudit{}, nil)
	_, closed, _, err := svc.ProcessExpiredResponseDeadlines(context.Background(), now, 10)
	if err != nil || closed != 1 {
		t.Fatalf("closed=%d err=%v", closed, err)
	}
}

func TestProcessExpiredResponseDeadlinesIgnoresIneligibleStatuses(t *testing.T) {
	t.Parallel()
	past := time.Now().UTC().Add(-time.Hour)
	statuses := []string{
		domain.RfxStatusDraft, domain.RfxStatusPublished, domain.RfxStatusResponsesClosed,
		domain.RfxStatusAwarded, domain.RfxStatusCancelled, domain.RfxStatusArchived,
	}
	for _, status := range statuses {
		status := status
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			store := &deadlineMockStore{
				listExpiredFn: func(context.Context, time.Time, int) ([]domain.ExpiredResponseOpenEvent, error) {
					return []domain.ExpiredResponseOpenEvent{{
						ID: uuid.New(), TenantID: uuid.New(), Status: status, ResponseDeadline: &past,
					}}, nil
				},
				updateStatusFn: func(context.Context, uuid.UUID, uuid.UUID, string, string) (*domain.RfxEvent, error) {
					t.Fatalf("status %s must not update", status)
					return nil, nil
				},
			}
			svc := NewRfxService(store, nil, nil)
			_, closed, _, err := svc.ProcessExpiredResponseDeadlines(context.Background(), time.Now().UTC(), 10)
			if err != nil || closed != 0 {
				t.Fatalf("closed=%d err=%v", closed, err)
			}
		})
	}
}

func TestProcessExpiredResponseDeadlinesReplaySafe(t *testing.T) {
	t.Parallel()
	eventID := uuid.New()
	tenantID := uuid.New()
	past := time.Now().UTC().Add(-time.Hour)
	audit := &recordingAudit{}
	store := &deadlineMockStore{
		listExpiredFn: func(_ context.Context, _ time.Time, _ int) ([]domain.ExpiredResponseOpenEvent, error) {
			return []domain.ExpiredResponseOpenEvent{{
				ID: eventID, TenantID: tenantID, Status: domain.RfxStatusResponsesOpen, ResponseDeadline: &past,
			}}, nil
		},
		updateStatusFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID, _, _ string) (*domain.RfxEvent, error) {
			return nil, apperrors.Conflict("already closed", map[string]any{"field": "status"})
		},
	}
	svc := NewRfxService(store, audit, nil)
	_, closed, _, err := svc.ProcessExpiredResponseDeadlines(context.Background(), time.Now().UTC(), 10)
	if err != nil || closed != 0 {
		t.Fatalf("closed=%d err=%v", closed, err)
	}
	if audit.countAction("auto_close_responses") != 0 {
		t.Fatal("expected no duplicate audit on replay")
	}
}

func TestProcessExpiredResponseDeadlinesConcurrentWorkersSafe(t *testing.T) {
	t.Parallel()
	eventID := uuid.New()
	tenantID := uuid.New()
	past := time.Now().UTC().Add(-time.Hour)
	audit := &recordingAudit{}
	var gate sync.Mutex
	successes := 0
	store := &deadlineMockStore{
		listExpiredFn: func(_ context.Context, _ time.Time, _ int) ([]domain.ExpiredResponseOpenEvent, error) {
			return []domain.ExpiredResponseOpenEvent{{
				ID: eventID, TenantID: tenantID, Status: domain.RfxStatusResponsesOpen, ResponseDeadline: &past,
			}}, nil
		},
		updateStatusFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID, _, _ string) (*domain.RfxEvent, error) {
			gate.Lock()
			defer gate.Unlock()
			if successes > 0 {
				return nil, apperrors.Conflict("already closed", map[string]any{"field": "status"})
			}
			successes++
			return &domain.RfxEvent{ID: eventID, TenantID: tenantID, Status: domain.RfxStatusResponsesClosed}, nil
		},
	}
	svc := NewRfxService(store, audit, nil)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, _, _ = svc.ProcessExpiredResponseDeadlines(context.Background(), time.Now().UTC(), 10)
		}()
	}
	wg.Wait()
	if successes != 1 {
		t.Fatalf("successes=%d", successes)
	}
	if audit.countAction("auto_close_responses") != 1 {
		t.Fatal("expected single audit event")
	}
}

func TestProcessExpiredResponseDeadlinesIsolatesEventErrors(t *testing.T) {
	t.Parallel()
	past := time.Now().UTC().Add(-time.Hour)
	attempts := 0
	store := &deadlineMockStore{
		listExpiredFn: func(_ context.Context, _ time.Time, _ int) ([]domain.ExpiredResponseOpenEvent, error) {
			return []domain.ExpiredResponseOpenEvent{
				{ID: uuid.New(), TenantID: uuid.New(), Status: domain.RfxStatusResponsesOpen, ResponseDeadline: &past},
				{ID: uuid.New(), TenantID: uuid.New(), Status: domain.RfxStatusResponsesOpen, ResponseDeadline: &past},
			}, nil
		},
		updateStatusFn: func(_ context.Context, id uuid.UUID, _ uuid.UUID, _, _ string) (*domain.RfxEvent, error) {
			attempts++
			if attempts == 1 {
				return nil, errors.New("simulated failure")
			}
			return &domain.RfxEvent{ID: id, Status: domain.RfxStatusResponsesClosed}, nil
		},
	}
	svc := NewRfxService(store, nil, nil)
	examined, closed, failures, err := svc.ProcessExpiredResponseDeadlines(context.Background(), time.Now().UTC(), 10)
	if err != nil || examined != 2 || closed != 1 || failures != 1 {
		t.Fatalf("examined=%d closed=%d failures=%d err=%v", examined, closed, failures, err)
	}
}

func TestAutoCloseAuditUsesSystemActor(t *testing.T) {
	t.Parallel()
	eventID := uuid.New()
	tenantID := uuid.New()
	past := time.Now().UTC().Add(-time.Hour)
	audit := &recordingAudit{}
	store := &deadlineMockStore{
		listExpiredFn: func(_ context.Context, _ time.Time, _ int) ([]domain.ExpiredResponseOpenEvent, error) {
			return []domain.ExpiredResponseOpenEvent{{
				ID: eventID, TenantID: tenantID, Status: domain.RfxStatusResponsesOpen, ResponseDeadline: &past,
			}}, nil
		},
	}
	svc := NewRfxService(store, audit, nil)
	_, _, _, _ = svc.ProcessExpiredResponseDeadlines(context.Background(), time.Now().UTC(), 10)
	audit.mu.Lock()
	defer audit.mu.Unlock()
	if len(audit.records) != 1 {
		t.Fatalf("records=%d", len(audit.records))
	}
	rec := audit.records[0]
	if rec.ActorUserID != nil {
		t.Fatal("system audit must not use fake user id")
	}
	if rec.Metadata["actor_type"] != domain.AuditActorTypeSystem {
		t.Fatalf("metadata=%v", rec.Metadata)
	}
}
