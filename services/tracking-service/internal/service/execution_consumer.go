package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/tracking-service/internal/domain"
	"github.com/freight-platform/tracking-service/internal/repository"
)

type StopContext struct {
	ExecutionID           uuid.UUID
	RevisionID            uuid.UUID
	ExecutionStopID       uuid.UUID
	Ordinal               int
	Status                string
	PlannedArrival        *time.Time
	LocationID            *uuid.UUID
	TargetLatitude        *float64
	TargetLongitude       *float64
	DriverID              *uuid.UUID
	VehicleID             *uuid.UUID
	PointKind             string
	StopRole              string
	LiveETAStopID         *uuid.UUID
	LiveETAOrdinal        *int
	LiveETAPlannedArrival *time.Time
	LiveETALocationID     *uuid.UUID
	LiveETALatitude       *float64
	LiveETALongitude      *float64
}

type StopContextClient interface {
	Get(ctx context.Context, operatingTenantID, executionID, stopID uuid.UUID) (StopContext, error)
}

type ContextNotFoundError struct{}

func (ContextNotFoundError) Error() string { return "tracking context not found" }

func IsContextNotFound(err error) bool {
	var missing ContextNotFoundError
	return errors.As(err, &missing)
}

type ExecutionConsumer struct {
	repo    *repository.ExecutionTrackingRepository
	context StopContextClient
}

func NewExecutionConsumer(repo *repository.ExecutionTrackingRepository, contextClient StopContextClient) *ExecutionConsumer {
	return &ExecutionConsumer{repo: repo, context: contextClient}
}

type executionEvent struct {
	EventID           uuid.UUID
	EventType         string
	OperatingTenantID uuid.UUID
	ExecutionID       uuid.UUID
	RevisionID        uuid.UUID
	RevisionVersion   int
	EventSequence     int64
	StopID            uuid.UUID
	OccurredAt        time.Time
}

func (c *ExecutionConsumer) Apply(ctx context.Context, raw []byte) error {
	event, err := parseExecutionEvent(raw)
	if err != nil {
		return err
	}
	var view *StopContext
	if event.EventType == domain.EventRouteStopCurrent {
		loaded, loadErr := c.context.Get(ctx, event.OperatingTenantID, event.ExecutionID, event.StopID)
		if IsContextNotFound(loadErr) {
			view = nil
		} else if loadErr != nil {
			return loadErr
		} else if loaded.RevisionID != event.RevisionID {
			view = nil
		} else {
			view = &loaded
		}
	}
	tx, err := c.repo.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	inserted, err := repository.InsertExecutionInbox(ctx, tx, repository.ExecutionEventRow{
		EventID:           event.EventID,
		OperatingTenantID: event.OperatingTenantID,
		ExecutionID:       event.ExecutionID,
		EventSequence:     event.EventSequence,
		EventType:         event.EventType,
	})
	if err != nil {
		return err
	}
	if !inserted {
		return tx.Commit(ctx)
	}
	state, err := repository.LockExecutionTrackingState(ctx, tx, event.ExecutionID)
	if err != nil {
		return err
	}
	if state != nil && state.OperatingTenantID != event.OperatingTenantID {
		return tx.Commit(ctx)
	}
	if state != nil && event.EventSequence <= state.LastExecutionEventSequence {
		return tx.Commit(ctx)
	}
	switch event.EventType {
	case domain.EventRouteStopCurrent:
		if view == nil {
			return tx.Commit(ctx)
		}
		next := stateFromContext(event, *view)
		if err := repository.UpsertExecutionTrackingState(ctx, tx, next); err != nil {
			return err
		}
	case domain.EventRouteStopCompleted:
		if state == nil {
			return tx.Commit(ctx)
		}
		if err := repository.ClearExecutionTarget(ctx, tx, event.OperatingTenantID, event.ExecutionID, event.StopID, event.EventSequence); err != nil {
			return err
		}
	default:
		if state == nil {
			return tx.Commit(ctx)
		}
		if err := repository.TouchExecutionSequence(ctx, tx, event.OperatingTenantID, event.ExecutionID, event.EventSequence); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func stateFromContext(event executionEvent, view StopContext) repository.ExecutionTrackingState {
	current := event.StopID
	ordinal := view.Ordinal
	state := repository.ExecutionTrackingState{
		OperatingTenantID:          event.OperatingTenantID,
		ExecutionID:                event.ExecutionID,
		RevisionID:                 event.RevisionID,
		CurrentStopID:              &current,
		CurrentStopOrdinal:         &ordinal,
		DriverID:                   view.DriverID,
		VehicleID:                  view.VehicleID,
		LastExecutionEventSequence: event.EventSequence,
		LiveETAStopID:              view.LiveETAStopID,
		LiveETAOrdinal:             view.LiveETAOrdinal,
	}
	if view.LiveETAStopID != nil {
		state.PlannedArrival = view.LiveETAPlannedArrival
		state.LocationID = view.LiveETALocationID
		state.TargetLatitude, state.TargetLongitude = coordinatePair(view.LiveETALatitude, view.LiveETALongitude)
		return state
	}
	if view.PointKind == "POSITION_ANCHOR" || view.StopRole == "START" {
		return state
	}
	state.PlannedArrival = view.PlannedArrival
	state.LocationID = view.LocationID
	state.TargetLatitude, state.TargetLongitude = coordinatePair(view.TargetLatitude, view.TargetLongitude)
	stop := view.ExecutionStopID
	state.LiveETAStopID = &stop
	state.LiveETAOrdinal = &ordinal
	return state
}

func coordinatePair(lat, lon *float64) (*float64, *float64) {
	if lat == nil || lon == nil {
		return nil, nil
	}
	return lat, lon
}

func parseExecutionEvent(raw []byte) (executionEvent, error) {
	var body map[string]json.RawMessage
	if err := json.Unmarshal(raw, &body); err != nil {
		return executionEvent{}, err
	}
	for _, key := range []string{"shipment_tenant_id", "shipmentTenantId", "price", "rate", "capacity_snapshot", "capacitySnapshot"} {
		if _, ok := body[key]; ok {
			return executionEvent{}, errors.New("execution event contains a private field")
		}
	}
	var payload struct {
		EventID           string `json:"event_id"`
		EventType         string `json:"event_type"`
		OperatingTenantID string `json:"operating_tenant_id"`
		ExecutionID       string `json:"execution_id"`
		RevisionID        string `json:"revision_id"`
		RevisionVersion   int    `json:"revision_version"`
		EventSequence     int64  `json:"event_sequence"`
		StopID            string `json:"stop_id"`
		OccurredAt        string `json:"occurred_at"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return executionEvent{}, err
	}
	eventID, err := uuid.Parse(payload.EventID)
	if err != nil {
		return executionEvent{}, err
	}
	operating, err := uuid.Parse(payload.OperatingTenantID)
	if err != nil {
		return executionEvent{}, err
	}
	executionID, err := uuid.Parse(payload.ExecutionID)
	if err != nil {
		return executionEvent{}, err
	}
	revisionID, err := uuid.Parse(payload.RevisionID)
	if err != nil {
		return executionEvent{}, err
	}
	stopID, err := uuid.Parse(payload.StopID)
	if err != nil {
		return executionEvent{}, err
	}
	occurred, err := time.Parse(time.RFC3339Nano, payload.OccurredAt)
	if err != nil {
		occurred, err = time.Parse(time.RFC3339, payload.OccurredAt)
		if err != nil {
			return executionEvent{}, err
		}
	}
	if payload.EventSequence <= 0 || payload.EventType == "" {
		return executionEvent{}, errors.New("execution event is incomplete")
	}
	return executionEvent{
		EventID: eventID, EventType: payload.EventType, OperatingTenantID: operating,
		ExecutionID: executionID, RevisionID: revisionID, RevisionVersion: payload.RevisionVersion,
		EventSequence: payload.EventSequence, StopID: stopID, OccurredAt: occurred.UTC(),
	}, nil
}
