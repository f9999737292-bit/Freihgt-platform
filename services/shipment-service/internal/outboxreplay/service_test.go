package outboxreplay

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/repository"
)

type fakeFailedOutboxStore struct {
	listRows       []repository.OutboxReplayPreviewRow
	ordering       map[uuid.UUID][]repository.OutboxReplayOrderingRow
	replayCalls    int
	replayEventIDs []uuid.UUID
	replayFn       func(context.Context, uuid.UUID, []uuid.UUID, time.Time, int) (int64, error)
}

func (f *fakeFailedOutboxStore) ListFailedOutboxForReplay(
	_ context.Context,
	_ uuid.UUID,
	eventIDs []uuid.UUID,
	aggregateIDs []uuid.UUID,
) ([]repository.OutboxReplayPreviewRow, error) {
	_ = eventIDs
	_ = aggregateIDs
	return append([]repository.OutboxReplayPreviewRow(nil), f.listRows...), nil
}

func (f *fakeFailedOutboxStore) ListOutboxReplayOrdering(
	_ context.Context,
	_ uuid.UUID,
	aggregateID uuid.UUID,
) ([]repository.OutboxReplayOrderingRow, error) {
	return f.ordering[aggregateID], nil
}

func (f *fakeFailedOutboxStore) ReplayFailedOutboxRows(
	ctx context.Context,
	tenantID uuid.UUID,
	eventIDs []uuid.UUID,
	availableAt time.Time,
	expectedCount int,
) (int64, error) {
	if f.replayFn != nil {
		return f.replayFn(ctx, tenantID, eventIDs, availableAt, expectedCount)
	}
	f.replayCalls++
	f.replayEventIDs = append([]uuid.UUID(nil), eventIDs...)
	return int64(expectedCount), nil
}

func TestReplayDryRunDoesNotMutate(t *testing.T) {
	tenantID := uuid.New()
	aggregateID := uuid.New()
	eventID := uuid.New()
	store := &fakeFailedOutboxStore{
		listRows: []repository.OutboxReplayPreviewRow{{
			EventID:      eventID,
			TenantID:     tenantID,
			AggregateID:  aggregateID,
			EventType:    domain.OutboxEventTypeCreated,
			Status:       domain.OutboxStatusFailed,
			AttemptCount: 5,
		}},
		ordering: map[uuid.UUID][]repository.OutboxReplayOrderingRow{
			aggregateID: {{
				ID:     eventID,
				Status: domain.OutboxStatusFailed,
			}},
		},
	}
	svc := NewService(store)

	result, err := svc.ReplayFailedOutbox(context.Background(), Request{
		TenantID:     tenantID,
		AggregateIDs: []uuid.UUID{aggregateID},
		Execute:      false,
	})
	require.NoError(t, err)
	require.True(t, result.DryRun)
	require.Len(t, result.Preview, 1)
	require.Equal(t, 0, store.replayCalls)
}

func TestReplayEmptySelectionRejected(t *testing.T) {
	svc := NewService(&fakeFailedOutboxStore{})
	_, err := svc.ReplayFailedOutbox(context.Background(), Request{
		TenantID: uuid.New(),
		Execute:  false,
	})
	require.ErrorIs(t, err, ErrEmptySelection)
}

func TestReplayTenantRequired(t *testing.T) {
	svc := NewService(&fakeFailedOutboxStore{})
	_, err := svc.ReplayFailedOutbox(context.Background(), Request{
		AggregateIDs: []uuid.UUID{uuid.New()},
	})
	require.ErrorIs(t, err, ErrTenantRequired)
}

func TestReplayWrongTenantRejected(t *testing.T) {
	tenantID := uuid.New()
	otherTenant := uuid.New()
	aggregateID := uuid.New()
	eventID := uuid.New()
	store := &fakeFailedOutboxStore{
		listRows: []repository.OutboxReplayPreviewRow{{
			EventID:     eventID,
			TenantID:    otherTenant,
			AggregateID: aggregateID,
			Status:      domain.OutboxStatusFailed,
		}},
	}
	svc := NewService(store)
	_, err := svc.ReplayFailedOutbox(context.Background(), Request{
		TenantID:     tenantID,
		AggregateIDs: []uuid.UUID{aggregateID},
	})
	require.ErrorIs(t, err, ErrWrongTenant)
}

func TestReplayPublishedProtected(t *testing.T) {
	tenantID := uuid.New()
	store := &fakeFailedOutboxStore{
		listRows: []repository.OutboxReplayPreviewRow{{
			EventID:  uuid.New(),
			TenantID: tenantID,
			Status:   domain.OutboxStatusPublished,
		}},
	}
	svc := NewService(store)
	_, err := svc.ReplayFailedOutbox(context.Background(), Request{
		TenantID:     tenantID,
		AggregateIDs: []uuid.UUID{uuid.New()},
	})
	require.ErrorIs(t, err, ErrPublishedProtected)
}

func TestReplayPendingProtected(t *testing.T) {
	tenantID := uuid.New()
	store := &fakeFailedOutboxStore{
		listRows: []repository.OutboxReplayPreviewRow{{
			EventID:  uuid.New(),
			TenantID: tenantID,
			Status:   domain.OutboxStatusPending,
		}},
	}
	svc := NewService(store)
	_, err := svc.ReplayFailedOutbox(context.Background(), Request{
		TenantID:     tenantID,
		AggregateIDs: []uuid.UUID{uuid.New()},
	})
	require.ErrorIs(t, err, ErrPendingProtected)
}

func TestReplayExecuteResetsFailedRows(t *testing.T) {
	tenantID := uuid.New()
	aggregateID := uuid.New()
	eventID := uuid.New()
	store := &fakeFailedOutboxStore{
		listRows: []repository.OutboxReplayPreviewRow{{
			EventID:      eventID,
			TenantID:     tenantID,
			AggregateID:  aggregateID,
			EventType:    domain.OutboxEventTypeCreated,
			Status:       domain.OutboxStatusFailed,
			AttemptCount: 5,
		}},
		ordering: map[uuid.UUID][]repository.OutboxReplayOrderingRow{
			aggregateID: {{
				ID:     eventID,
				Status: domain.OutboxStatusFailed,
			}},
		},
	}
	svc := NewService(store)

	result, err := svc.ReplayFailedOutbox(context.Background(), Request{
		TenantID:     tenantID,
		AggregateIDs: []uuid.UUID{aggregateID},
		Execute:      true,
	})
	require.NoError(t, err)
	require.False(t, result.DryRun)
	require.Equal(t, int64(1), result.AffectedCount)
	require.Equal(t, 1, store.replayCalls)
	require.Equal(t, []uuid.UUID{eventID}, store.replayEventIDs)
}

func TestReplayPartialAggregateOrderingBlocked(t *testing.T) {
	tenantID := uuid.New()
	aggregateID := uuid.New()
	first := uuid.New()
	second := uuid.New()
	store := &fakeFailedOutboxStore{
		listRows: []repository.OutboxReplayPreviewRow{{
			EventID:     second,
			TenantID:    tenantID,
			AggregateID: aggregateID,
			Status:      domain.OutboxStatusFailed,
		}},
		ordering: map[uuid.UUID][]repository.OutboxReplayOrderingRow{
			aggregateID: {
				{ID: first, Status: domain.OutboxStatusFailed},
				{ID: second, Status: domain.OutboxStatusFailed},
			},
		},
	}
	svc := NewService(store)
	_, err := svc.ReplayFailedOutbox(context.Background(), Request{
		TenantID:  tenantID,
		EventIDs:  []uuid.UUID{second},
		Execute:   false,
	})
	require.ErrorIs(t, err, ErrPartialAggregateReplay)
}

func TestReplayOrderingAllowsPublishedEarlierEvents(t *testing.T) {
	tenantID := uuid.New()
	aggregateID := uuid.New()
	first := uuid.New()
	second := uuid.New()
	store := &fakeFailedOutboxStore{
		listRows: []repository.OutboxReplayPreviewRow{{
			EventID:     second,
			TenantID:    tenantID,
			AggregateID: aggregateID,
			Status:      domain.OutboxStatusFailed,
		}},
		ordering: map[uuid.UUID][]repository.OutboxReplayOrderingRow{
			aggregateID: {
				{ID: first, Status: domain.OutboxStatusPublished},
				{ID: second, Status: domain.OutboxStatusFailed},
			},
		},
	}
	svc := NewService(store)
	result, err := svc.ReplayFailedOutbox(context.Background(), Request{
		TenantID:  tenantID,
		EventIDs:  []uuid.UUID{second},
		Execute:   false,
	})
	require.NoError(t, err)
	require.Len(t, result.Preview, 1)
}

func TestReplayAffectedCountMismatch(t *testing.T) {
	tenantID := uuid.New()
	aggregateID := uuid.New()
	eventID := uuid.New()
	store := &fakeFailedOutboxStore{
		listRows: []repository.OutboxReplayPreviewRow{{
			EventID:     eventID,
			TenantID:    tenantID,
			AggregateID: aggregateID,
			Status:      domain.OutboxStatusFailed,
		}},
		ordering: map[uuid.UUID][]repository.OutboxReplayOrderingRow{
			aggregateID: {{ID: eventID, Status: domain.OutboxStatusFailed}},
		},
		replayFn: func(context.Context, uuid.UUID, []uuid.UUID, time.Time, int) (int64, error) {
			return 0, errors.New("replay affected 0 rows, expected 1")
		},
	}

	svc := NewService(store)
	_, err := svc.ReplayFailedOutbox(context.Background(), Request{
		TenantID:     tenantID,
		AggregateIDs: []uuid.UUID{aggregateID},
		Execute:      true,
	})
	require.Error(t, err)
}
