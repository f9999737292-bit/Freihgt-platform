package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/currenttrip"
	"github.com/freight-platform/network-optimizer-service/internal/domain"
	apperrors "github.com/freight-platform/network-optimizer-service/internal/platform/errors"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/routeplan"
)

const (
	routePlanAcceptedEvent            = "network.route_plan.accepted"
	routePlanActivationRequestedEvent = "network.route_plan.activation_requested"
)

func (s *Service) AcceptRoutePlan(ctx context.Context, actor Actor, idempotencyKey string, id uuid.UUID, raw []byte) (Result, error) {
	version, err := parseRoutePlanDecision(raw)
	if err != nil {
		return Result{}, err
	}
	if idempotencyKey == "" {
		return Result{}, apperrors.Validation("Idempotency-Key is required", nil)
	}
	storedKey := "accept_route_plan:" + idempotencyKey
	hash := HashBody(raw)
	if replay, ok, err := s.peekIdempotency(ctx, actor.TenantID, storedKey, hash); err != nil || ok {
		return replay, err
	}
	var result Result
	err = s.store.Within(ctx, func(tx repository.Tx) error {
		replayed, err := takeIdempotency(ctx, tx, actor.TenantID, storedKey, hash, &result)
		if err != nil || replayed {
			return err
		}
		graph, err := tx.GetRoutePlan(ctx, actor.TenantID, id)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return apperrors.NotFound("route plan not found")
			}
			return err
		}
		if graph.Plan.Version != version {
			return planStale()
		}
		if _, err := s.ensureRoutePlanFresh(ctx, tx, actor, graph); err != nil {
			return err
		}
		now := s.now()
		if graph.Plan.Status != routeplan.StatusAccepted {
			if graph.Plan.Status != routeplan.StatusEvaluated {
				return apperrors.Validation("route plan cannot be accepted", map[string]any{"reason": routeplan.ReasonActivationNotAccepted})
			}
			if err := tx.MarkRoutePlanAccepted(ctx, actor.TenantID, id, version, now); err != nil {
				return err
			}
			graph, err = tx.GetRoutePlan(ctx, actor.TenantID, id)
			if err != nil {
				return err
			}
			if err := s.audit(ctx, tx, actor, "route_plan", id, "accepted", routeplan.StatusEvaluated, routeplan.StatusAccepted, domain.VisPrivate, graph.Plan.PlanningMode, nil, now); err != nil {
				return err
			}
			if err := s.emit(ctx, tx, routePlanAcceptedEvent, actor.TenantID, id, graph.Plan.Version, routeplan.StatusAccepted, domain.VisPrivate, nil, now); err != nil {
				return err
			}
		}
		body, err := marshalRoutePlan(graph)
		if err != nil {
			return err
		}
		result = Result{Status: http.StatusOK, Body: body, AggregateID: id}
		return saveIdempotency(ctx, tx, actor.TenantID, storedKey, hash, result.Status, result.Body)
	})
	if errors.Is(err, repository.ErrIdempotencyRace) {
		return s.readReplay(ctx, actor.TenantID, storedKey, hash)
	}
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

func (s *Service) ActivateRoutePlan(ctx context.Context, actor Actor, idempotencyKey string, id uuid.UUID, raw []byte) (Result, error) {
	version, err := parseRoutePlanDecision(raw)
	if err != nil {
		return Result{}, err
	}
	if idempotencyKey == "" {
		return Result{}, apperrors.Validation("Idempotency-Key is required", nil)
	}
	storedKey := "activate_route_plan:" + idempotencyKey
	hash := HashBody(raw)
	if replay, ok, err := s.peekIdempotency(ctx, actor.TenantID, storedKey, hash); err != nil || ok {
		return replay, err
	}
	var result Result
	err = s.store.Within(ctx, func(tx repository.Tx) error {
		replayed, err := takeIdempotency(ctx, tx, actor.TenantID, storedKey, hash, &result)
		if err != nil || replayed {
			return err
		}
		if err := tx.LockRoutePlan(ctx, actor.TenantID, id); err != nil {
			return err
		}
		graph, err := tx.GetRoutePlan(ctx, actor.TenantID, id)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return apperrors.NotFound("route plan not found")
			}
			return err
		}
		existing, err := tx.GetRoutePlanActivation(ctx, actor.TenantID, id)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return err
		}
		if err == nil {
			body, err := marshalActivationResult(graph, existing)
			if err != nil {
				return err
			}
			result = Result{Status: http.StatusOK, Body: body, AggregateID: id}
			return saveIdempotency(ctx, tx, actor.TenantID, storedKey, hash, result.Status, result.Body)
		}
		if graph.Plan.Version != version {
			return planStale()
		}
		shipmentStatus, err := s.ensureRoutePlanFresh(ctx, tx, actor, graph)
		if err != nil {
			return err
		}
		if graph.Plan.Status != routeplan.StatusAccepted {
			return apperrors.Validation("route plan must be accepted before activation", map[string]any{"reason": routeplan.ReasonActivationNotAccepted})
		}
		if reason, blocked := activationBlockReason(graph); blocked {
			return activationRefused(reason)
		}
		now := s.now()
		for _, leg := range graph.Legs {
			if !leg.ExpiresAt.After(now) {
				return activationRefused(routeplan.ResultRoutingDown)
			}
		}
		if !routeplan.ActivationStatusAllowed(graph.Plan.PlanningMode, shipmentStatus) {
			return activationRefused(routeplan.ReasonShipmentStatusIneligible)
		}
		row := repository.RoutePlanActivationRow{
			ID: uuid.New(), TenantID: actor.TenantID, RoutePlanID: graph.Plan.ID, PlanVersion: graph.Plan.Version,
			Version: 1, IdempotencyKey: idempotencyKey, ExecutionShipmentID: graph.Plan.ShipmentID,
			Status: routeplan.ActivationPending, CreatedAt: now,
		}
		if err := tx.InsertRoutePlanActivation(ctx, row); err != nil {
			if !errors.Is(err, repository.ErrConflict) {
				return err
			}
			row, err = tx.GetRoutePlanActivation(ctx, actor.TenantID, id)
			if err != nil {
				return err
			}
		} else {
			if err := s.audit(ctx, tx, actor, "route_plan", id, "activation_requested", routeplan.StatusAccepted, routeplan.ActivationPending, domain.VisPrivate, graph.Plan.PlanningMode, graph.Plan.ShipmentID, now); err != nil {
				return err
			}
			if err := s.emit(ctx, tx, routePlanActivationRequestedEvent, actor.TenantID, id, graph.Plan.Version, routeplan.ActivationPending, domain.VisPrivate, nil, now); err != nil {
				return err
			}
		}
		body, err := marshalActivationResult(graph, row)
		if err != nil {
			return err
		}
		result = Result{Status: http.StatusOK, Body: body, AggregateID: id}
		return saveIdempotency(ctx, tx, actor.TenantID, storedKey, hash, result.Status, result.Body)
	})
	if errors.Is(err, repository.ErrIdempotencyRace) {
		return s.readReplay(ctx, actor.TenantID, storedKey, hash)
	}
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

func parseRoutePlanDecision(raw []byte) (int, error) {
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		return 0, apperrors.Validation("request body is invalid", nil)
	}
	for key := range keys {
		if key != "version" {
			return 0, apperrors.Validation("caller supplied execution state is not allowed", nil)
		}
	}
	var body struct {
		Version *int `json:"version"`
	}
	if err := json.Unmarshal(raw, &body); err != nil || body.Version == nil || *body.Version < 1 {
		return 0, apperrors.Validation("version is required", nil)
	}
	return *body.Version, nil
}

func (s *Service) ensureRoutePlanFresh(ctx context.Context, tx repository.Tx, actor Actor, graph repository.RoutePlanGraph) (string, error) {
	byKind := map[string][]repository.RouteDependencyRow{}
	for _, dep := range graph.Dependencies {
		byKind[dep.DependencyKind] = append(byKind[dep.DependencyKind], dep)
	}
	loads := make([]domain.LoadOpportunity, 0, len(byKind["LOAD_OPPORTUNITY"]))
	for _, dep := range byKind["LOAD_OPPORTUNITY"] {
		if dep.SubjectID == nil || dep.SubjectVersion == nil {
			return "", planStale()
		}
		load, err := tx.GetLoad(ctx, *dep.SubjectID)
		if err != nil || !routeLoadVisible(actor, load) || load.Version != *dep.SubjectVersion {
			return "", planStale()
		}
		loads = append(loads, load)
	}
	catalogFP, ruleFP, err := s.referenceFingerprints(ctx, actor, loads)
	if err != nil {
		return "", err
	}
	if !dependencyFingerprint(byKind["CATALOG"], catalogFP) || !dependencyFingerprint(byKind["RULE_SET"], ruleFP) ||
		!dependencyFingerprint(byKind["ROUTING_POLICY"], routeplan.RoutingPolicyVersion) ||
		!dependencyFingerprint(byKind["ALGORITHM_POLICY"], routeplan.AlgorithmPolicyVersion) {
		return "", planStale()
	}
	switch graph.Plan.PlanningMode {
	case routeplan.ModeCurrentTrip:
		if graph.Plan.ShipmentID == nil {
			return "", planStale()
		}
		if s.currentTrip == nil {
			return "", apperrors.ServiceUnavailable("current trip context is unavailable")
		}
		trip, err := s.currentTrip.Build(ctx, actor.TenantID, *graph.Plan.ShipmentID)
		if err != nil {
			if errors.Is(err, currenttrip.ErrNotFound) {
				return "", apperrors.NotFound("shipment not found")
			}
			return "", err
		}
		if !dependencyVersion(byKind["SHIPMENT"], trip.ShipmentID, trip.ShipmentVersion) ||
			!dependencyFingerprint(byKind["CURRENT_TRIP_CONTEXT"], trip.InputFingerprint) {
			return "", planStale()
		}
		if rows := byKind["VEHICLE"]; len(rows) > 0 {
			if trip.VehicleID == nil || !dependencyVersion(rows, *trip.VehicleID, trip.VehicleVersion) {
				return "", planStale()
			}
		}
		if !cargoDependenciesFresh(byKind["SHIPMENT_CARGO"], trip.OnboardCargoUnits) {
			return "", planStale()
		}
		return trip.ShipmentStatus, nil
	case routeplan.ModeDepotStart:
		if graph.Plan.CapacityID == nil || graph.Plan.CapacityVersion == nil {
			return "", planStale()
		}
		capacity, err := s.ownCapacity(ctx, tx, actor.TenantID, *graph.Plan.CapacityID)
		if err != nil {
			if app, ok := err.(*apperrors.AppError); ok && app.Code == apperrors.CodeNotFound {
				return "", planStale()
			}
			return "", err
		}
		if capacity.Version != *graph.Plan.CapacityVersion || !dependencyFingerprint(byKind["CAPACITY"], capacityContextFingerprint(capacity.ID)) {
			return "", planStale()
		}
		return "", nil
	default:
		return "", planStale()
	}
}

func (s *Service) referenceFingerprints(ctx context.Context, actor Actor, loads []domain.LoadOpportunity) (string, string, error) {
	owners := map[uuid.UUID]struct{}{actor.TenantID: {}}
	for _, load := range loads {
		owners[load.OwnerTenantID] = struct{}{}
	}
	if s.catalog == nil || len(owners) > 1 {
		empty := fingerprintLines(nil)
		return empty, empty, nil
	}
	evalCtx, err := s.catalog.Evaluation(ctx, actor.TenantID)
	if err != nil {
		empty := fingerprintLines(nil)
		return empty, empty, nil
	}
	return fingerprintLines(catalogLines(evalCtx.CatalogRefs)), fingerprintLines(ruleLines(evalCtx.RuleSets)), nil
}

func dependencyVersion(rows []repository.RouteDependencyRow, id uuid.UUID, version int) bool {
	if len(rows) != 1 || rows[0].SubjectID == nil || rows[0].SubjectVersion == nil {
		return false
	}
	return *rows[0].SubjectID == id && *rows[0].SubjectVersion == version
}

func dependencyFingerprint(rows []repository.RouteDependencyRow, fingerprint string) bool {
	return len(rows) == 1 && rows[0].Fingerprint == fingerprint
}

func cargoDependenciesFresh(rows []repository.RouteDependencyRow, units []currenttrip.OnboardCargoUnit) bool {
	confirmed := map[uuid.UUID]currenttrip.OnboardCargoUnit{}
	for _, unit := range units {
		if unit.EvidenceState != currenttrip.EvidenceConfirmedOnboard {
			continue
		}
		confirmed[unit.CargoID] = unit
	}
	if len(rows) != len(confirmed) {
		return false
	}
	for _, row := range rows {
		if row.SubjectID == nil || row.SubjectVersion == nil {
			return false
		}
		unit, ok := confirmed[*row.SubjectID]
		if !ok || *row.SubjectVersion != unit.CargoVersion || row.Fingerprint != unit.EvidenceState {
			return false
		}
	}
	return true
}

func activationBlockReason(graph repository.RoutePlanGraph) (string, bool) {
	unknownDuration := planHasReason(graph.Plan.ReasonCodes, routeplan.ReasonServiceDurationUnknown) || cargoStopMissingDuration(graph)
	if routeplan.ProductionActivationRequiresServiceDurationSource && unknownDuration {
		return routeplan.ReasonServiceDurationUnknown, true
	}
	if graph.Plan.ResultStatus != routeplan.ResultFeasible {
		return routeplan.ReasonActivationNotFeasible, true
	}
	for _, snap := range graph.Snapshots {
		if snap.CompatibilityStatus == "INDETERMINATE" {
			return routeplan.ReasonActivationNotFeasible, true
		}
	}
	return "", false
}

func cargoStopMissingDuration(graph repository.RoutePlanGraph) bool {
	counts := map[uuid.UUID]int{}
	for _, action := range graph.Actions {
		counts[action.StopID]++
	}
	for _, stop := range graph.Stops {
		if stop.StopRole == routeplan.RoleCargo && counts[stop.ID] > 0 && stop.ServiceDurationSeconds == nil {
			return true
		}
	}
	return false
}

func planHasReason(codes []string, want string) bool {
	for _, code := range codes {
		if code == want {
			return true
		}
	}
	return false
}

func planStale() error {
	return apperrors.Conflict("route plan is stale", map[string]any{"reason": routeplan.ReasonPlanStale})
}

func activationRefused(reason string) error {
	return apperrors.Validation("route plan cannot be activated", map[string]any{"reason": reason})
}

func marshalActivationResult(graph repository.RoutePlanGraph, row repository.RoutePlanActivationRow) ([]byte, error) {
	plan, err := marshalRoutePlan(graph)
	if err != nil {
		return nil, err
	}
	activation := map[string]any{
		"id": row.ID, "route_plan_id": row.RoutePlanID, "plan_version": row.PlanVersion,
		"version": row.Version, "status": row.Status, "created_at": row.CreatedAt,
	}
	if row.ExecutionShipmentID != nil {
		activation["execution_shipment_id"] = row.ExecutionShipmentID
	}
	if row.ExecutionID != nil {
		activation["execution_id"] = row.ExecutionID
	}
	if row.ExecutionRevisionID != nil {
		activation["execution_revision_id"] = row.ExecutionRevisionID
	}
	return json.Marshal(map[string]any{"plan": json.RawMessage(plan), "activation": activation})
}
