package outboxreplay

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/repository"
)

var (
	ErrTenantRequired         = errors.New("tenant id is required")
	ErrEmptySelection         = errors.New("at least one event id or aggregate id selector is required")
	ErrNoMatchingFailedRows   = errors.New("no matching failed outbox rows")
	ErrWrongTenant            = errors.New("selected rows do not belong to the supplied tenant")
	ErrPublishedProtected     = errors.New("published outbox rows cannot be replayed")
	ErrPendingProtected       = errors.New("pending outbox rows cannot be replayed via failed replay")
	ErrPartialAggregateReplay = errors.New("partial failed replay blocked: earlier failed events for aggregate must be included or already published")
	ErrAffectedCountMismatch  = errors.New("replay affected row count mismatch")
)

// Request selects FAILED outbox rows for operator replay.
type Request struct {
	TenantID     uuid.UUID
	EventIDs     []uuid.UUID
	AggregateIDs []uuid.UUID
	Execute      bool
	Now          time.Time
}

// PreviewRow is safe dry-run metadata (no payload).
type PreviewRow struct {
	EventID       uuid.UUID
	TenantID      uuid.UUID
	AggregateID   uuid.UUID
	EventType     string
	Status        domain.OutboxStatus
	AttemptCount  int
	LastErrorCode string
}

// Result summarizes a dry-run or execute operation.
type Result struct {
	DryRun        bool
	Preview       []PreviewRow
	AffectedCount int64
}

type failedOutboxStore interface {
	ListFailedOutboxForReplay(ctx context.Context, tenantID uuid.UUID, eventIDs []uuid.UUID, aggregateIDs []uuid.UUID) ([]repository.OutboxReplayPreviewRow, error)
	ListOutboxReplayOrdering(ctx context.Context, tenantID uuid.UUID, aggregateID uuid.UUID) ([]repository.OutboxReplayOrderingRow, error)
	ReplayFailedOutboxRows(ctx context.Context, tenantID uuid.UUID, eventIDs []uuid.UUID, availableAt time.Time, expectedCount int) (int64, error)
}

// Service validates and optionally resets FAILED outbox rows to PENDING.
type Service struct {
	repo failedOutboxStore
}

func NewService(repo failedOutboxStore) *Service {
	return &Service{repo: repo}
}

func (s *Service) ReplayFailedOutbox(ctx context.Context, req Request) (Result, error) {
	if req.TenantID == uuid.Nil {
		return Result{}, ErrTenantRequired
	}
	if len(req.EventIDs) == 0 && len(req.AggregateIDs) == 0 {
		return Result{}, ErrEmptySelection
	}

	now := req.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	rows, err := s.repo.ListFailedOutboxForReplay(ctx, req.TenantID, req.EventIDs, req.AggregateIDs)
	if err != nil {
		return Result{}, err
	}
	if len(rows) == 0 {
		return Result{}, ErrNoMatchingFailedRows
	}

	if err := validateTenant(rows, req.TenantID); err != nil {
		return Result{}, err
	}
	if err := validateStatuses(rows); err != nil {
		return Result{}, err
	}
	if err := s.validateOrdering(ctx, req.TenantID, rows); err != nil {
		return Result{}, err
	}

	preview := toPreview(rows)
	if !req.Execute {
		return Result{DryRun: true, Preview: preview}, nil
	}

	eventIDs := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		eventIDs[i] = row.EventID
	}

	affected, err := s.repo.ReplayFailedOutboxRows(ctx, req.TenantID, eventIDs, now, len(eventIDs))
	if err != nil {
		if strings.Contains(err.Error(), "expected") {
			return Result{}, fmt.Errorf("%w: %v", ErrAffectedCountMismatch, err)
		}
		return Result{}, err
	}

	return Result{
		DryRun:        false,
		Preview:       preview,
		AffectedCount: affected,
	}, nil
}

func validateTenant(rows []repository.OutboxReplayPreviewRow, tenantID uuid.UUID) error {
	for _, row := range rows {
		if row.TenantID != tenantID {
			return ErrWrongTenant
		}
	}
	return nil
}

func validateStatuses(rows []repository.OutboxReplayPreviewRow) error {
	for _, row := range rows {
		switch row.Status {
		case domain.OutboxStatusFailed:
			continue
		case domain.OutboxStatusPublished:
			return ErrPublishedProtected
		case domain.OutboxStatusPending:
			return ErrPendingProtected
		default:
			return fmt.Errorf("unsupported outbox status %q", row.Status)
		}
	}
	return nil
}

func (s *Service) validateOrdering(ctx context.Context, tenantID uuid.UUID, selected []repository.OutboxReplayPreviewRow) error {
	selectedIDs := make(map[uuid.UUID]struct{}, len(selected))
	aggregates := make(map[uuid.UUID]struct{})
	for _, row := range selected {
		selectedIDs[row.EventID] = struct{}{}
		aggregates[row.AggregateID] = struct{}{}
	}

	for aggregateID := range aggregates {
		orderRows, err := s.repo.ListOutboxReplayOrdering(ctx, tenantID, aggregateID)
		if err != nil {
			return err
		}
		for _, row := range orderRows {
			if row.Status == domain.OutboxStatusPublished {
				continue
			}
			if row.Status == domain.OutboxStatusFailed {
				if _, ok := selectedIDs[row.ID]; !ok {
					return fmt.Errorf("%w: aggregate %s event %s", ErrPartialAggregateReplay, aggregateID, row.ID)
				}
				continue
			}
			if row.Status == domain.OutboxStatusPending {
				return ErrPendingProtected
			}
		}
	}
	return nil
}

func toPreview(rows []repository.OutboxReplayPreviewRow) []PreviewRow {
	preview := make([]PreviewRow, len(rows))
	for i, row := range rows {
		lastError := ""
		if row.LastErrorCode != nil {
			lastError = *row.LastErrorCode
		}
		preview[i] = PreviewRow{
			EventID:       row.EventID,
			TenantID:      row.TenantID,
			AggregateID:   row.AggregateID,
			EventType:     row.EventType,
			Status:        row.Status,
			AttemptCount:  row.AttemptCount,
			LastErrorCode: lastError,
		}
	}
	return preview
}
