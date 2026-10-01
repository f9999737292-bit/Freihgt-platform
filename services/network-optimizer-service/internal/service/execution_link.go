package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	"github.com/freight-platform/network-optimizer-service/internal/executionproj"
	apperrors "github.com/freight-platform/network-optimizer-service/internal/platform/errors"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/routeplan"
)

const routePlanExecutionLinkedEvent = "network.route_plan.execution_linked"

const reasonAckCorrelationMismatch = "ACK_CORRELATION_MISMATCH"

func (s *Service) finishActivationHandoff(ctx context.Context, actor Actor, planID uuid.UUID, storedKey, hash string) (Result, error) {
	var graph repository.RoutePlanGraph
	var row repository.RoutePlanActivationRow
	err := s.store.Within(ctx, func(tx repository.Tx) error {
		var getErr error
		graph, getErr = tx.GetRoutePlan(ctx, actor.TenantID, planID)
		if getErr != nil {
			return getErr
		}
		row, getErr = tx.GetRoutePlanActivation(ctx, actor.TenantID, planID)
		return getErr
	})
	if errors.Is(err, repository.ErrNotFound) {
		return Result{}, apperrors.NotFound("route plan activation not found")
	}
	if err != nil {
		return Result{}, err
	}
	if row.Status != routeplan.ActivationPending {
		return s.activationResult(ctx, actor, storedKey, hash, graph, row)
	}
	cmd, err := s.projectionCommand(ctx, actor, graph, row)
	if err != nil {
		return Result{}, err
	}
	ack, err := s.projection.Project(ctx, actor.TenantID, cmd)
	if err != nil {
		var callErr *executionproj.Error
		if errors.As(err, &callErr) && !callErr.Temporary {
			_ = s.store.Within(ctx, func(tx repository.Tx) error {
				return tx.MarkRoutePlanActivationRejected(ctx, actor.TenantID, planID)
			})
			return Result{}, apperrors.Conflict("execution projection was rejected", map[string]any{"reason": callErr.Reason})
		}
		pending, marshalErr := marshalActivationResult(graph, row)
		if marshalErr != nil {
			return Result{}, marshalErr
		}
		return Result{Status: http.StatusOK, Body: pending, AggregateID: planID}, nil
	}
	if !ackMatches(ack, actor.TenantID, row.ID, graph.Plan.ID) {
		return Result{}, apperrors.Conflict("execution acknowledgement does not match the activation", map[string]any{"reason": reasonAckCorrelationMismatch})
	}
	var linked repository.RoutePlanActivationRow
	err = s.store.Within(ctx, func(tx repository.Tx) error {
		changed, linkErr := tx.LinkRoutePlanActivation(ctx, actor.TenantID, planID, ack.ExecutionID, ack.RevisionID, graph.Plan.ShipmentID)
		if linkErr != nil {
			return linkErr
		}
		current, getErr := tx.GetRoutePlanActivation(ctx, actor.TenantID, planID)
		if getErr != nil {
			return getErr
		}
		if current.ExecutionID == nil || current.ExecutionRevisionID == nil || *current.ExecutionID != ack.ExecutionID || *current.ExecutionRevisionID != ack.RevisionID || current.Status != routeplan.ActivationLinked {
			return apperrors.Conflict("execution acknowledgement does not match the activation", map[string]any{"reason": reasonAckCorrelationMismatch})
		}
		linked = current
		now := s.now()
		if changed {
			if err := s.audit(ctx, tx, actor, "route_plan", planID, "execution_linked", routeplan.ActivationPending, routeplan.ActivationLinked, domain.VisPrivate, graph.Plan.PlanningMode, graph.Plan.ShipmentID, now); err != nil {
				return err
			}
			if err := s.emitExecutionLinked(ctx, tx, actor.TenantID, planID, graph.Plan.Version, row.ID, ack, now); err != nil {
				return err
			}
		}
		body, err := marshalActivationResult(graph, linked)
		if err != nil {
			return err
		}
		return updateIdempotency(ctx, tx, actor.TenantID, storedKey, hash, http.StatusOK, body)
	})
	if err != nil {
		return Result{}, err
	}
	return s.activationResult(ctx, actor, storedKey, hash, graph, linked)
}

func (s *Service) activationResult(ctx context.Context, actor Actor, storedKey, hash string, graph repository.RoutePlanGraph, row repository.RoutePlanActivationRow) (Result, error) {
	body, err := marshalActivationResult(graph, row)
	if err != nil {
		return Result{}, err
	}
	if err := s.store.Within(ctx, func(tx repository.Tx) error {
		return updateIdempotency(ctx, tx, actor.TenantID, storedKey, hash, http.StatusOK, body)
	}); err != nil {
		return Result{}, err
	}
	return Result{Status: http.StatusOK, Body: body, AggregateID: graph.Plan.ID}, nil
}

func (s *Service) projectionCommand(ctx context.Context, actor Actor, graph repository.RoutePlanGraph, row repository.RoutePlanActivationRow) (executionproj.Command, error) {
	if s.currentTrip == nil || graph.Plan.ShipmentID == nil || graph.Plan.ShipmentVersion == nil {
		return executionproj.Command{}, apperrors.Validation("current trip context is required", nil)
	}
	trip, err := s.currentTrip.Build(ctx, actor.TenantID, *graph.Plan.ShipmentID)
	if err != nil || trip.CarrierCompanyID == nil || *trip.CarrierCompanyID == uuid.Nil {
		return executionproj.Command{}, apperrors.Validation("carrier company is required", nil)
	}
	tenant := actor.TenantID
	shipmentID := *graph.Plan.ShipmentID
	shipmentVersion := *graph.Plan.ShipmentVersion
	cmd := executionproj.Command{
		ActivationID: row.ID, ActivationVersion: row.Version, ActivationStatus: routeplan.ActivationPending,
		RoutePlanID: graph.Plan.ID, RoutePlanVersion: graph.Plan.Version, PlanningMode: graph.Plan.PlanningMode,
		OperatingTenantID: tenant, ContextShipmentID: &shipmentID, ContextShipmentTenantID: &tenant,
		ContextShipmentVersion: &shipmentVersion, CarrierCompanyID: *trip.CarrierCompanyID,
		VehicleID: graph.Plan.VehicleID, DriverID: trip.DriverID, EvaluationFingerprint: graph.Plan.EvaluationFingerprint,
	}
	stops := append([]repository.RouteStopRow(nil), graph.Stops...)
	sort.Slice(stops, func(i, j int) bool { return stops[i].Ordinal < stops[j].Ordinal })
	for _, stop := range stops {
		cmd.Stops = append(cmd.Stops, executionproj.Stop{
			RoutePlanStopID: stop.ID, Ordinal: stop.Ordinal, StopRole: stop.StopRole, PointKind: stop.PointKind,
			LocationID: stop.LocationID, Latitude: stop.Latitude, Longitude: stop.Longitude,
			PlannedArrival: stop.PlannedArrival, PlannedDeparture: stop.PlannedDeparture,
			ServiceDurationSeconds: stop.ServiceDurationSeconds,
		})
	}
	actions := append([]repository.RouteActionRow(nil), graph.Actions...)
	sort.Slice(actions, func(i, j int) bool {
		if actions[i].StopID == actions[j].StopID {
			return actions[i].ActionOrdinal < actions[j].ActionOrdinal
		}
		return actions[i].StopID.String() < actions[j].StopID.String()
	})
	seen := map[string]struct{}{}
	for _, action := range actions {
		item := executionproj.Action{
			RoutePlanActionID: action.ID, RoutePlanStopID: action.StopID, ActionOrdinal: action.ActionOrdinal,
			ActionType: action.ActionType, RouteSubjectType: action.SubjectType, RouteSubjectID: action.SubjectID,
			EvidenceState: action.EvidenceState, EvidenceStateVersion: action.EvidenceStateVersion,
		}
		if action.SubjectType == routeplan.SubjectCargo && action.SourceShipmentID != nil && action.SourceShipmentVersion != nil {
			cargoID := action.SubjectID
			cargoVersion := action.SubjectVersion
			item.ExecutionShipmentID = action.SourceShipmentID
			item.ShipmentTenantID = &tenant
			item.ExecutionShipmentVersion = action.SourceShipmentVersion
			item.CargoID = &cargoID
			item.CargoVersion = &cargoVersion
			key := action.SubjectType + action.SubjectID.String()
			if _, ok := seen[key]; !ok {
				seen[key] = struct{}{}
				cmd.ExecutionSubjects = append(cmd.ExecutionSubjects, executionproj.Subject{
					RouteSubjectType: action.SubjectType, RouteSubjectID: action.SubjectID, RouteSubjectVersion: action.SubjectVersion,
					ExecutionShipmentID: action.SourceShipmentID, ShipmentTenantID: &tenant,
					ExecutionShipmentVersion: action.SourceShipmentVersion, CargoID: &cargoID, CargoVersion: &cargoVersion,
				})
			}
		}
		cmd.Actions = append(cmd.Actions, item)
	}
	if cmd.ExecutionSubjects == nil {
		cmd.ExecutionSubjects = []executionproj.Subject{}
	}
	if cmd.Actions == nil {
		cmd.Actions = []executionproj.Action{}
	}
	return cmd, nil
}

func ackMatches(ack executionproj.Ack, tenant, activationID, planID uuid.UUID) bool {
	return ack.OperatingTenantID == tenant && ack.ActivationID == activationID && ack.RoutePlanID == planID && ack.ExecutionID != uuid.Nil && ack.RevisionID != uuid.Nil
}

func (s *Service) emitExecutionLinked(ctx context.Context, tx repository.Tx, tenant, planID uuid.UUID, planVersion int, activationID uuid.UUID, ack executionproj.Ack, now time.Time) error {
	eventID := uuid.New()
	payload, err := json.Marshal(map[string]any{
		"eventId": eventID, "eventName": routePlanExecutionLinkedEvent, "schemaVersion": 1,
		"tenantId": tenant, "aggregateId": planID, "aggregateVersion": planVersion,
		"occurredAt": now.UTC().Format(time.RFC3339Nano), "status": routeplan.ActivationLinked,
		"visibilityScope": domain.VisPrivate, "activationId": activationID,
		"operatingTenantId": ack.OperatingTenantID, "routePlanId": ack.RoutePlanID,
		"executionId": ack.ExecutionID, "executionRevisionId": ack.RevisionID,
	})
	if err != nil {
		return err
	}
	return tx.InsertOutbox(ctx, repository.OutboxEvent{
		ID: eventID, EventName: routePlanExecutionLinkedEvent, SchemaVersion: 1, TenantID: tenant,
		AggregateID: planID, AggregateVersion: planVersion, OccurredAt: now, Payload: payload,
	})
}
