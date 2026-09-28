package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/compat"
	"github.com/freight-platform/network-optimizer-service/internal/currenttrip"
	"github.com/freight-platform/network-optimizer-service/internal/domain"
	apperrors "github.com/freight-platform/network-optimizer-service/internal/platform/errors"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/routeplan"
)

const routePlanEvaluatedEvent = "network.route_plan.evaluated"

type RoutePlanCommand struct {
	PlanningMode     string      `json:"planning_mode"`
	ShipmentID       *uuid.UUID  `json:"shipment_id"`
	CapacityID       *uuid.UUID  `json:"capacity_id"`
	CandidateLoadIDs []uuid.UUID `json:"candidate_load_ids"`
	Raw              []byte      `json:"-"`
}

func (s *Service) EvaluateRoutePlan(ctx context.Context, actor Actor, idempotencyKey string, cmd RoutePlanCommand) (Result, error) {
	if idempotencyKey == "" {
		return Result{}, apperrors.Validation("Idempotency-Key is required", nil)
	}
	if err := validateRoutePlanCommand(cmd); err != nil {
		return Result{}, err
	}
	storedKey := "evaluate_route_plan:" + idempotencyKey
	hash := HashBody(cmd.Raw)
	if replay, ok, err := s.peekIdempotency(ctx, actor.TenantID, storedKey, hash); err != nil || ok {
		return replay, err
	}
	planInput, deps, err := s.prepareRoutePlan(ctx, actor, cmd)
	if err != nil {
		return Result{}, err
	}
	outcome, _, err := (routeplan.Planner{}).Plan(ctx, planInput)
	if err != nil {
		return Result{}, mapPlanError(err)
	}
	if outcome.ResultStatus != routeplan.ResultFeasible && outcome.ResultStatus != routeplan.ResultIndeterminate {
		return Result{}, mapPlanError(&routeplan.SearchError{Code: routeplan.ResultNoPlan})
	}
	if deps.evidence != nil {
		deps.catalogFingerprint = fingerprintLines(catalogLines(deps.evidence.catalogs))
		deps.ruleFingerprint = fingerprintLines(ruleLines(deps.evidence.rules))
	} else {
		deps.catalogFingerprint = fingerprintLines(nil)
		deps.ruleFingerprint = fingerprintLines(nil)
	}
	graph, err := buildRoutePlanGraph(actor.TenantID, cmd, outcome, deps, s.now())
	if err != nil {
		return Result{}, err
	}
	body, err := marshalRoutePlan(graph)
	if err != nil {
		return Result{}, err
	}
	result := Result{Status: http.StatusCreated, Body: body, AggregateID: graph.Plan.ID}
	err = s.store.Within(ctx, func(tx repository.Tx) error {
		replayed, err := takeIdempotency(ctx, tx, actor.TenantID, storedKey, hash, &result)
		if err != nil || replayed {
			return err
		}
		if err := tx.InsertRoutePlan(ctx, graph); err != nil {
			return err
		}
		if err := s.audit(ctx, tx, actor, "route_plan", graph.Plan.ID, "evaluated", "", routeplan.StatusEvaluated, domain.VisPrivate, cmd.PlanningMode, nil, graph.Plan.CreatedAt); err != nil {
			return err
		}
		if err := s.emit(ctx, tx, routePlanEvaluatedEvent, actor.TenantID, graph.Plan.ID, graph.Plan.Version, routeplan.StatusEvaluated, domain.VisPrivate, nil, graph.Plan.CreatedAt); err != nil {
			return err
		}
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

func (s *Service) GetRoutePlan(ctx context.Context, actor Actor, id uuid.UUID) (Result, error) {
	var graph repository.RoutePlanGraph
	err := s.store.Within(ctx, func(tx repository.Tx) error {
		var getErr error
		graph, getErr = tx.GetRoutePlan(ctx, actor.TenantID, id)
		return getErr
	})
	if errors.Is(err, repository.ErrNotFound) {
		return Result{}, apperrors.NotFound("route plan not found")
	}
	if err != nil {
		return Result{}, err
	}
	body, err := marshalRoutePlan(graph)
	if err != nil {
		return Result{}, err
	}
	return Result{Status: http.StatusOK, Body: body, AggregateID: graph.Plan.ID}, nil
}

func validateRoutePlanCommand(cmd RoutePlanCommand) error {
	switch cmd.PlanningMode {
	case routeplan.ModeCurrentTrip:
		if cmd.ShipmentID == nil || cmd.CapacityID != nil {
			return apperrors.Validation("CURRENT_TRIP requires shipment_id and forbids capacity_id", nil)
		}
	case routeplan.ModeDepotStart:
		if cmd.CapacityID == nil || cmd.ShipmentID != nil {
			return apperrors.Validation("DEPOT_START requires capacity_id and forbids shipment_id", nil)
		}
	default:
		return apperrors.Validation("planning_mode is invalid", nil)
	}
	if len(cmd.CandidateLoadIDs) < 1 || len(cmd.CandidateLoadIDs) > routeplan.MaxAdditionalLoads {
		return apperrors.Validation("candidate_load_ids must contain 1 or 2 ids", map[string]any{"reason": routeplan.ResultLoadLimit, "budget": routeplan.BudgetLoads})
	}
	seen := map[uuid.UUID]struct{}{}
	for _, id := range cmd.CandidateLoadIDs {
		if id == uuid.Nil {
			return apperrors.Validation("candidate_load_ids must be uuids", nil)
		}
		if _, ok := seen[id]; ok {
			return apperrors.Validation("candidate_load_ids must be distinct", nil)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func (s *Service) peekIdempotency(ctx context.Context, tenant uuid.UUID, key, hash string) (Result, bool, error) {
	var rec repository.IdempotencyRecord
	err := s.store.Within(ctx, func(tx repository.Tx) error {
		var getErr error
		rec, getErr = tx.GetIdempotency(ctx, tenant, key)
		return getErr
	})
	if errors.Is(err, repository.ErrNotFound) {
		return Result{}, false, nil
	}
	if err != nil {
		return Result{}, false, err
	}
	if rec.Hash != hash {
		return Result{}, false, apperrors.Conflict("idempotency key was reused with a different request", nil)
	}
	return Result{Status: rec.Status, Body: append([]byte(nil), rec.Body...), Replay: true}, true, nil
}

type planDeps struct {
	shipmentID         *uuid.UUID
	shipmentVersion    *int
	capacityID         *uuid.UUID
	capacityVersion    *int
	vehicleID          *uuid.UUID
	vehicleVersion     *int
	contextFingerprint string
	catalogFingerprint string
	ruleFingerprint    string
	loads              []domain.LoadOpportunity
	cargos             []currenttrip.OnboardCargoUnit
	snapshots          map[uuid.UUID][]byte
	evidence           *compatEvidence
}

type compatEvidence struct {
	catalogs []compat.CatalogVersionRef
	rules    []compat.RuleSetRef
}

func (s *Service) prepareRoutePlan(ctx context.Context, actor Actor, cmd RoutePlanCommand) (routeplan.Input, planDeps, error) {
	var deps planDeps
	loads := make([]domain.LoadOpportunity, 0, len(cmd.CandidateLoadIDs))
	err := s.store.Within(ctx, func(tx repository.Tx) error {
		for _, id := range cmd.CandidateLoadIDs {
			load, err := tx.GetLoad(ctx, id)
			if err != nil {
				if errors.Is(err, repository.ErrNotFound) {
					return apperrors.NotFound("load opportunity not found")
				}
				return err
			}
			if !routeLoadVisible(actor, load) {
				return apperrors.NotFound("load opportunity not found")
			}
			loads = append(loads, load)
		}
		return nil
	})
	if err != nil {
		return routeplan.Input{}, planDeps{}, err
	}
	deps.loads = loads
	clock := s.now()
	evidence := &compatEvidence{}
	deps.evidence = evidence
	deps.snapshots = map[uuid.UUID][]byte{}
	for _, load := range loads {
		raw, err := marshalRoutePlanPublicLoadSnapshot(load)
		if err != nil {
			return routeplan.Input{}, planDeps{}, err
		}
		deps.snapshots[load.ID] = raw
	}
	var equipment compat.Equipment
	var evalCtx compat.Context
	haveCtx := false
	input := routeplan.Input{Mode: cmd.PlanningMode, Clock: clock, Route: s.routes}
	owners := map[uuid.UUID]struct{}{actor.TenantID: {}}
	for _, load := range loads {
		owners[load.OwnerTenantID] = struct{}{}
	}
	if len(owners) > 1 {
		input.ReferenceUnavailable = s.catalog == nil
	}
	if s.catalog != nil && !input.ReferenceUnavailable {
		ctxEval, err := s.catalog.Evaluation(ctx, actor.TenantID)
		if err != nil {
			input.ReferenceUnavailable = true
		} else if len(owners) > 1 {
			input.ReferenceUnavailable = true
		} else {
			evalCtx = ctxEval
			haveCtx = true
		}
	}
	switch cmd.PlanningMode {
	case routeplan.ModeCurrentTrip:
		if s.currentTrip == nil {
			return routeplan.Input{}, planDeps{}, apperrors.ServiceUnavailable("current trip context is unavailable")
		}
		trip, err := s.currentTrip.Build(ctx, actor.TenantID, *cmd.ShipmentID)
		if err != nil {
			if errors.Is(err, currenttrip.ErrNotFound) {
				return routeplan.Input{}, planDeps{}, apperrors.NotFound("shipment not found")
			}
			return routeplan.Input{}, planDeps{}, err
		}
		if trip.PositionFreshnessStatus != "FRESH" || trip.CurrentPosition == nil || trip.CurrentPosition.Latitude == nil || trip.CurrentPosition.Longitude == nil {
			return routeplan.Input{}, planDeps{}, mapPlanError(&routeplan.SearchError{Code: routeplan.ResultStartUnknown})
		}
		if residualExceeded(trip.ResidualCapacity) {
			return routeplan.Input{}, planDeps{}, mapPlanError(&routeplan.SearchError{Code: routeplan.ResultNoPlan, Detail: routeplan.ReasonCapacityExceeded})
		}
		equipment = equipmentFromVehicle(trip.VehicleCapability)
		observed := trip.PositionObservedAt
		if observed == nil {
			observed = trip.CurrentPosition.RecordedAt
		}
		input.Start = routeplan.Stop{Role: routeplan.RoleStart, Point: routeplan.Point{
			Kind: routeplan.PointAnchor, Latitude: *trip.CurrentPosition.Latitude, Longitude: *trip.CurrentPosition.Longitude,
			Source: routeplan.SourceTracking, ObservedAt: observed,
		}}
		endPoint, err := s.canonicalPoint(ctx, actor.TenantID, trip.DestinationLocationID, nil, nil)
		if err != nil {
			return routeplan.Input{}, planDeps{}, err
		}
		end := routeplan.Stop{Role: routeplan.RoleEnd, Point: endPoint}
		var onboard []compat.GroupageItem
		for _, unit := range trip.OnboardCargoUnits {
			if unit.EvidenceState != currenttrip.EvidenceConfirmedOnboard {
				continue
			}
			version := unit.CargoVersion
			stateVersion := unit.EvidenceStateVersion
			occurred := unit.EvidenceOccurredAt
			shipmentID := trip.ShipmentID
			shipmentVersion := trip.ShipmentVersion
			action := routeplan.Action{
				Type: routeplan.ActionDelivery, SubjectType: routeplan.SubjectCargo, SubjectID: unit.CargoID, SubjectVersion: unit.CargoVersion,
				WeightKg: unit.Profile.WeightKg, VolumeM3: unit.Profile.VolumeM3, LinearMeters: unit.Profile.LinearMeters,
				ShipmentID: &shipmentID, ShipmentVersion: &shipmentVersion, EvidenceState: unit.EvidenceState,
				EvidenceVersion: &stateVersion, EvidenceAt: &occurred, Cargo: cargoFromProfile(unit),
			}
			if unit.Profile.PalletCount != nil {
				pallets := float64(*unit.Profile.PalletCount)
				action.Pallets = &pallets
			}
			if unit.Profile.MaxLoadedHeightMM != nil {
				height := float64(*unit.Profile.MaxLoadedHeightMM)
				action.HeightMM = &height
			}
			end.Actions = append(end.Actions, action)
			onboard = append(onboard, compat.GroupageItem{Cargo: action.Cargo})
			deps.cargos = append(deps.cargos, unit)
			_ = version
		}
		input.End = &end
		input.Onboard = onboard
		input.BaseSubjects = len(onboard)
		input.Initial = capacityFromResidual(trip.ResidualCapacity)
		deps.shipmentID = &trip.ShipmentID
		deps.shipmentVersion = &trip.ShipmentVersion
		deps.vehicleID = trip.VehicleID
		if trip.VehicleVersion > 0 {
			version := trip.VehicleVersion
			deps.vehicleVersion = &version
		}
		deps.contextFingerprint = trip.InputFingerprint
	case routeplan.ModeDepotStart:
		var capacity domain.Capacity
		err := s.store.Within(ctx, func(tx repository.Tx) error {
			var getErr error
			capacity, getErr = s.ownCapacity(ctx, tx, actor.TenantID, *cmd.CapacityID)
			return getErr
		})
		if err != nil {
			return routeplan.Input{}, planDeps{}, err
		}
		start, err := s.depotStart(ctx, actor.TenantID, capacity)
		if err != nil {
			return routeplan.Input{}, planDeps{}, err
		}
		input.Start = routeplan.Stop{Role: routeplan.RoleStart, Point: start}
		input.Initial = capacityFromDepot(capacity)
		equipment = equipmentFromCapacity(capacity, nil)
		version := capacity.Version
		deps.capacityID = &capacity.ID
		deps.capacityVersion = &version
		deps.vehicleID = capacity.VehicleID
		sum := sha256.Sum256([]byte(capacity.ID.String()))
		deps.contextFingerprint = hex.EncodeToString(sum[:])
	}
	input.Groupage = func(items []compat.GroupageItem) compat.Result {
		ctxUsed := compat.Context{}
		if haveCtx && !input.ReferenceUnavailable {
			ctxUsed = evalCtx
		}
		result := compat.EvaluateGroupageItems(equipment, items, ctxUsed)
		evidence.catalogs = append(evidence.catalogs, result.CatalogVersionsUsed...)
		evidence.rules = append(evidence.rules, result.RuleSetsUsed...)
		return result
	}
	planned := make([]routeplan.Load, 0, len(loads))
	for _, load := range loads {
		item, err := s.loadToPlan(ctx, actor.TenantID, load)
		if err != nil {
			return routeplan.Input{}, planDeps{}, err
		}
		planned = append(planned, item)
	}
	input.Loads = planned
	return input, deps, nil
}

func (s *Service) loadToPlan(ctx context.Context, tenant uuid.UUID, load domain.LoadOpportunity) (routeplan.Load, error) {
	if load.Pickup.LocationID == nil || load.Delivery.LocationID == nil {
		return routeplan.Load{}, mapPlanError(&routeplan.SearchError{Code: routeplan.ResultNoPlan, Detail: routeplan.ReasonCanonicalCargo})
	}
	pickup, err := s.canonicalPoint(ctx, tenant, *load.Pickup.LocationID, load.Pickup.Latitude, load.Pickup.Longitude)
	if err != nil {
		return routeplan.Load{}, err
	}
	delivery, err := s.canonicalPoint(ctx, tenant, *load.Delivery.LocationID, load.Delivery.Latitude, load.Delivery.Longitude)
	if err != nil {
		return routeplan.Load{}, err
	}
	return routeplan.Load{
		ID: load.ID, Version: load.Version, OwnerTenantID: load.OwnerTenantID,
		Pickup: pickup, Delivery: delivery,
		PickupAction:   actionFromLoad(load, routeplan.ActionPickup, load.PickupWindow),
		DeliveryAction: actionFromLoad(load, routeplan.ActionDelivery, load.DeliveryWindow),
	}, nil
}

func actionFromLoad(load domain.LoadOpportunity, kind string, window domain.TimeWindow) routeplan.Action {
	cargo := cargoFromLoad(load)
	action := routeplan.Action{
		Type: kind, SubjectType: routeplan.SubjectLoad, SubjectID: load.ID, SubjectVersion: load.Version,
		WeightKg: load.WeightKg, VolumeM3: load.VolumeM3, LinearMeters: load.Cargo.LinearMeters,
		WindowStart: window.Start, WindowEnd: window.End, Cargo: cargo, AccessNeed: accessFromLoad(load),
	}
	if load.Cargo.PalletCount != nil {
		pallets := float64(*load.Cargo.PalletCount)
		action.Pallets = &pallets
	}
	if load.Cargo.MaxLoadedHeightMM != nil {
		height := float64(*load.Cargo.MaxLoadedHeightMM)
		action.HeightMM = &height
	}
	return action
}

func (s *Service) canonicalPoint(ctx context.Context, tenant, location uuid.UUID, lat, lon *float64) (routeplan.Point, error) {
	point := routeplan.Point{Kind: routeplan.PointCanonical, LocationID: &location, Source: routeplan.SourceCanonical}
	if lat != nil && lon != nil {
		point.Latitude = *lat
		point.Longitude = *lon
		return point, nil
	}
	if s.directory == nil {
		return routeplan.Point{}, mapPlanError(&routeplan.SearchError{Code: routeplan.ResultNoPlan, Detail: routeplan.ReasonCanonicalCargo})
	}
	snap, err := s.directory.Projection(ctx, tenant, location)
	if err != nil || snap.Latitude == nil || snap.Longitude == nil {
		return routeplan.Point{}, mapPlanError(&routeplan.SearchError{Code: routeplan.ResultNoPlan, Detail: routeplan.ReasonCanonicalCargo})
	}
	point.Latitude = *snap.Latitude
	point.Longitude = *snap.Longitude
	return point, nil
}

func (s *Service) depotStart(ctx context.Context, tenant uuid.UUID, capacity domain.Capacity) (routeplan.Point, error) {
	if capacity.LocationID != nil {
		return s.canonicalPoint(ctx, tenant, *capacity.LocationID, capacity.Latitude, capacity.Longitude)
	}
	if capacity.Latitude != nil && capacity.Longitude != nil {
		return routeplan.Point{
			Kind: routeplan.PointAnchor, Latitude: *capacity.Latitude, Longitude: *capacity.Longitude, Source: routeplan.SourceCapacity,
		}, nil
	}
	return routeplan.Point{}, mapPlanError(&routeplan.SearchError{Code: routeplan.ResultStartUnknown})
}

func routeLoadVisible(actor Actor, load domain.LoadOpportunity) bool {
	if load.Status != domain.LoadPublished {
		return false
	}
	if load.OwnerTenantID == actor.TenantID {
		return load.ConsolidationAllowed
	}
	if !load.CrossShipperConsolidationAllowed {
		return false
	}
	switch load.VisibilityScope {
	case domain.VisMarketplace, domain.VisAnonymized, domain.VisNetworkOnly:
		return true
	case domain.VisInvited:
		if actor.CompanyID == nil {
			return false
		}
		for _, id := range load.InvitedCarrierCompanyIDs {
			if id == *actor.CompanyID {
				return true
			}
		}
	}
	return false
}

func capacityFromResidual(residual currenttrip.ResidualCapacitySnapshot) routeplan.Capacity {
	return routeplan.Capacity{
		Payload: residualDim(residual.Payload), Volume: residualDim(residual.Volume),
		Pallets: residualDim(residual.PalletPositions), Linear: residualDim(residual.LinearMeters),
		Height: heightDim(residual.Height), Temperature: residual.TemperatureAllocation,
	}
}

func residualDim(dim currenttrip.SubtractiveDimension) routeplan.Dimension {
	if dim.Status == currenttrip.DimensionKnown && dim.Remaining != nil {
		value := *dim.Remaining
		return routeplan.Dimension{Status: routeplan.DimKnown, Value: &value}
	}
	return routeplan.Dimension{Status: routeplan.DimUnknown}
}

func residualExceeded(residual currenttrip.ResidualCapacitySnapshot) bool {
	for _, dim := range []currenttrip.SubtractiveDimension{residual.Payload, residual.Volume, residual.PalletPositions, residual.LinearMeters} {
		if dim.Status == currenttrip.DimensionExceeds {
			return true
		}
	}
	return residual.Height.Status == currenttrip.HeightKnownExceeded
}

func heightDim(height currenttrip.HeightCheck) routeplan.Dimension {
	if height.Status == currenttrip.HeightKnownOK && height.VehicleInternalHeightMM != nil {
		value := float64(*height.VehicleInternalHeightMM)
		return routeplan.Dimension{Status: routeplan.DimKnown, Value: &value}
	}
	return routeplan.Dimension{Status: routeplan.DimUnknown}
}

func capacityFromDepot(capacity domain.Capacity) routeplan.Capacity {
	return routeplan.Capacity{
		Payload: knownOrUnknown(capacity.PayloadRemainingKg), Volume: knownOrUnknown(capacity.VolumeRemainingM3),
		Pallets: routeplan.Dimension{Status: routeplan.DimUnknown}, Linear: routeplan.Dimension{Status: routeplan.DimUnknown},
		Height: routeplan.Dimension{Status: routeplan.DimUnknown}, Temperature: routeplan.DimUnknown,
	}
}

func knownOrUnknown(value *float64) routeplan.Dimension {
	if value == nil {
		return routeplan.Dimension{Status: routeplan.DimUnknown}
	}
	copied := *value
	return routeplan.Dimension{Status: routeplan.DimKnown, Value: &copied}
}

func buildRoutePlanGraph(tenant uuid.UUID, cmd RoutePlanCommand, outcome routeplan.Outcome, deps planDeps, now time.Time) (repository.RoutePlanGraph, error) {
	outcome.ContextFingerprint = deps.contextFingerprint
	outcome.CatalogFingerprint = deps.catalogFingerprint
	outcome.RuleFingerprint = deps.ruleFingerprint
	outcome.ShipmentID = deps.shipmentID
	outcome.ShipmentVersion = deps.shipmentVersion
	outcome.CapacityID = deps.capacityID
	outcome.CapacityVersion = deps.capacityVersion
	outcome.VehicleID = deps.vehicleID
	outcome.VehicleVersion = deps.vehicleVersion
	planID := uuid.New()
	stopIDs := make([]uuid.UUID, len(outcome.Stops))
	stops := make([]repository.RouteStopRow, len(outcome.Stops))
	var actions []repository.RouteActionRow
	for i, stop := range outcome.Stops {
		stopIDs[i] = uuid.New()
		stops[i] = repository.RouteStopRow{
			ID: stopIDs[i], RoutePlanID: planID, Ordinal: i + 1, StopRole: stop.Role, PointKind: stop.Point.Kind,
			LocationID: stop.Point.LocationID, Latitude: stop.Point.Latitude, Longitude: stop.Point.Longitude,
			PointSource: stop.Point.Source, PointObservedAt: stop.Point.ObservedAt, PlannedArrival: stop.Arrival,
			PlannedDeparture: stop.Depart, ServiceDurationSeconds: stop.Service,
		}
		for j, action := range stop.Actions {
			row := repository.RouteActionRow{
				ID: uuid.New(), RoutePlanID: planID, StopID: stopIDs[i], ActionOrdinal: j + 1, ActionType: action.Type,
				SubjectType: action.SubjectType, SubjectID: action.SubjectID, SubjectVersion: action.SubjectVersion,
				WeightDeltaKg: action.WeightKg, VolumeDeltaM3: action.VolumeM3, PalletDelta: action.Pallets,
				LinearMetersDelta: action.LinearMeters, WindowStart: action.WindowStart, WindowEnd: action.WindowEnd,
				SourceShipmentID: action.ShipmentID, SourceShipmentVersion: action.ShipmentVersion,
				EvidenceState: action.EvidenceState, EvidenceStateVersion: action.EvidenceVersion, EvidenceOccurredAt: action.EvidenceAt,
			}
			if action.SubjectType == routeplan.SubjectLoad {
				row.PublicSubjectSnapshot = deps.snapshots[action.SubjectID]
			}
			actions = append(actions, row)
		}
	}
	if len(outcome.Legs) != len(stops)-1 {
		return repository.RoutePlanGraph{}, apperrors.Validation("route leg adjacency is invalid", nil)
	}
	legs := make([]repository.RouteLegRow, len(outcome.Legs))
	for i, leg := range outcome.Legs {
		legs[i] = repository.RouteLegRow{
			ID: uuid.New(), RoutePlanID: planID, Ordinal: i + 1, FromStopID: stopIDs[i], ToStopID: stopIDs[i+1],
			FromPointFingerprint: leg.FromFingerprint, ToPointFingerprint: leg.ToFingerprint,
			DistanceM: leg.DistanceM, DurationSeconds: leg.DurationSeconds, Provider: leg.Provider,
			RequestFingerprint: leg.RequestFingerprint, ResponseFingerprint: leg.ResponseFingerprint,
			VehicleProfileHash: leg.VehicleProfileHash, RouteMode: leg.RouteMode, TrafficMode: leg.TrafficMode,
			DepartureBucket: leg.DepartureBucket, CalculatedAt: leg.CalculatedAt, ExpiresAt: leg.ExpiresAt,
			ProviderDefaultUsed: leg.ProviderDefaultUsed,
		}
		if legs[i].FromStopID != stops[i].ID || legs[i].ToStopID != stops[i+1].ID || stops[i].Ordinal != i+1 || stops[i+1].Ordinal != i+2 {
			return repository.RoutePlanGraph{}, apperrors.Validation("route leg adjacency is invalid", nil)
		}
	}
	snapshots := make([]repository.RouteSnapshotRow, len(outcome.Snapshots))
	for i, snap := range outcome.Snapshots {
		snapshots[i] = repository.RouteSnapshotRow{
			ID: uuid.New(), RoutePlanID: planID, SequenceOrdinal: snap.SequenceOrdinal, AfterStopID: stopIDs[snap.AfterStopIndex],
			AfterActionOrdinal: snap.AfterActionOrdinal,
			PayloadStatus:      snap.Capacity.Payload.Status, PayloadRemainingKg: snap.Capacity.Payload.Value,
			VolumeStatus: snap.Capacity.Volume.Status, VolumeRemainingM3: snap.Capacity.Volume.Value,
			PalletStatus: snap.Capacity.Pallets.Status, PalletPositionsRemaining: snap.Capacity.Pallets.Value,
			LinearStatus: snap.Capacity.Linear.Status, LinearMetersRemaining: snap.Capacity.Linear.Value,
			HeightStatus: snap.Capacity.Height.Status, HeightRemainingMM: snap.Capacity.Height.Value,
			TemperatureAllocationStatus: snap.Capacity.Temperature,
			CompatibilityStatus:         snap.CompatibilityStatus,
			CompatibilityFingerprint:    snap.CompatibilityFingerprint,
			TemperatureCheckStatus:      snap.TemperatureCheck,
			ADRCheckStatus:              snap.ADRCheck,
			FoodGradeCheckStatus:        snap.FoodGradeCheck,
		}
	}
	dependencies := routeDependencies(planID, cmd, deps)
	return repository.RoutePlanGraph{
		Plan: repository.RoutePlanRow{
			ID: planID, TenantID: tenant, Version: 1, Status: routeplan.StatusEvaluated, PlanningMode: cmd.PlanningMode,
			ResultStatus: outcome.ResultStatus, CapacityID: deps.capacityID, CapacityVersion: deps.capacityVersion,
			ShipmentID: deps.shipmentID, ShipmentVersion: deps.shipmentVersion, VehicleID: deps.vehicleID,
			ContextFingerprint: deps.contextFingerprint, EvaluationFingerprint: routeplan.EvaluationFingerprint(outcome),
			AlgorithmPolicyVersion: routeplan.AlgorithmPolicyVersion, RoutingPolicyVersion: routeplan.RoutingPolicyVersion,
			ExecutionSupported: false, ReasonCodes: append([]string(nil), outcome.ReasonCodes...), CreatedAt: now,
		},
		Stops: stops, Actions: actions, Legs: legs, Snapshots: snapshots, Dependencies: dependencies,
	}, nil
}

func routeDependencies(planID uuid.UUID, cmd RoutePlanCommand, deps planDeps) []repository.RouteDependencyRow {
	rows := []repository.RouteDependencyRow{}
	add := func(kind string, id *uuid.UUID, version *int, fingerprint string) {
		rows = append(rows, repository.RouteDependencyRow{
			ID: uuid.New(), RoutePlanID: planID, DependencyKind: kind, SubjectID: id, SubjectVersion: version, Fingerprint: fingerprint,
		})
	}
	if deps.shipmentID != nil {
		add("SHIPMENT", deps.shipmentID, deps.shipmentVersion, "")
		add("CURRENT_TRIP_CONTEXT", deps.shipmentID, deps.shipmentVersion, deps.contextFingerprint)
	}
	if deps.capacityID != nil {
		add("CAPACITY", deps.capacityID, deps.capacityVersion, deps.contextFingerprint)
	}
	if deps.vehicleID != nil {
		add("VEHICLE", deps.vehicleID, deps.vehicleVersion, "")
	}
	loads := append([]domain.LoadOpportunity(nil), deps.loads...)
	sort.Slice(loads, func(i, j int) bool { return loads[i].ID.String() < loads[j].ID.String() })
	for _, load := range loads {
		version := load.Version
		id := load.ID
		add("LOAD_OPPORTUNITY", &id, &version, "")
	}
	cargos := append([]currenttrip.OnboardCargoUnit(nil), deps.cargos...)
	sort.Slice(cargos, func(i, j int) bool { return cargos[i].CargoID.String() < cargos[j].CargoID.String() })
	for _, cargo := range cargos {
		version := cargo.CargoVersion
		id := cargo.CargoID
		add("SHIPMENT_CARGO", &id, &version, cargo.EvidenceState)
	}
	add("ROUTING_POLICY", nil, nil, routeplan.RoutingPolicyVersion)
	add("ALGORITHM_POLICY", nil, nil, routeplan.AlgorithmPolicyVersion)
	add("CATALOG", nil, nil, deps.catalogFingerprint)
	add("RULE_SET", nil, nil, deps.ruleFingerprint)
	return rows
}

// marshalRoutePlanPublicLoadSnapshot is the carrier-visible load projection stored on a route plan.
// It keeps authorized geography and planning characteristics. It never copies owner, commercial, source, or invitation fields.
func marshalRoutePlanPublicLoadSnapshot(load domain.LoadOpportunity) ([]byte, error) {
	view := load.MarketplaceView()
	doc := map[string]any{
		"id":               load.ID,
		"version":          load.Version,
		"status":           load.Status,
		"visibility_scope": load.VisibilityScope,
		"pickup_window":    load.PickupWindow,
		"delivery_window":  load.DeliveryWindow,
	}
	if load.VisibilityScope != domain.VisAnonymized || view.Pickup.HasCoarse() {
		doc["pickup"] = view.Pickup
	}
	if load.VisibilityScope != domain.VisAnonymized || view.Delivery.HasCoarse() {
		doc["delivery"] = view.Delivery
	}
	if load.WeightKg != nil {
		doc["weight_kg"] = load.WeightKg
	}
	if load.VolumeM3 != nil {
		doc["volume_m3"] = load.VolumeM3
	}
	if load.BodyType != "" {
		doc["body_type"] = load.BodyType
	}
	if len(load.Equipment) > 0 {
		doc["equipment"] = append([]string(nil), load.Equipment...)
	}
	if cargo, err := json.Marshal(load.Cargo); err != nil {
		return nil, err
	} else if string(cargo) != "{}" {
		doc["cargo"] = json.RawMessage(cargo)
	}
	return json.Marshal(doc)
}

func marshalRoutePlan(graph repository.RoutePlanGraph) ([]byte, error) {
	stops := make([]map[string]any, len(graph.Stops))
	for i, stop := range graph.Stops {
		actions := publicActions(graph, stop)
		view := map[string]any{
			"id": stop.ID, "ordinal": stop.Ordinal, "stop_role": stop.StopRole,
			"point_kind": stop.PointKind, "point_source": stop.PointSource, "actions": actions,
		}
		if stop.PointObservedAt != nil {
			view["point_observed_at"] = stop.PointObservedAt
		}
		if stop.PlannedArrival != nil {
			view["planned_arrival"] = stop.PlannedArrival
		}
		if stop.PlannedDeparture != nil {
			view["planned_departure"] = stop.PlannedDeparture
		}
		if stop.ServiceDurationSeconds != nil {
			view["service_duration_seconds"] = stop.ServiceDurationSeconds
		}
		if stopShowsExact(stop, actions) {
			view["latitude"] = stop.Latitude
			view["longitude"] = stop.Longitude
			if stop.LocationID != nil {
				view["location_id"] = stop.LocationID
			}
		} else if country, region, city := coarseGeography(actions); country != "" || region != "" || city != "" {
			view["country_code"] = country
			view["region"] = region
			view["city"] = city
		}
		stops[i] = view
	}
	body := map[string]any{
		"id": graph.Plan.ID, "version": graph.Plan.Version, "status": graph.Plan.Status,
		"planning_mode": graph.Plan.PlanningMode, "result_status": graph.Plan.ResultStatus,
		"execution_supported": false, "capacity_id": graph.Plan.CapacityID, "capacity_version": graph.Plan.CapacityVersion,
		"shipment_id": graph.Plan.ShipmentID, "shipment_version": graph.Plan.ShipmentVersion, "vehicle_id": graph.Plan.VehicleID,
		"context_fingerprint": graph.Plan.ContextFingerprint, "evaluation_fingerprint": graph.Plan.EvaluationFingerprint,
		"algorithm_policy_version": graph.Plan.AlgorithmPolicyVersion, "routing_policy_version": graph.Plan.RoutingPolicyVersion,
		"created_at": graph.Plan.CreatedAt, "reason_codes": graph.Plan.ReasonCodes,
		"stops": stops, "legs": graph.Legs, "capacity_snapshots": graph.Snapshots, "dependencies": graph.Dependencies,
	}
	return json.Marshal(body)
}

func publicActions(graph repository.RoutePlanGraph, stop repository.RouteStopRow) []map[string]any {
	out := []map[string]any{}
	for _, action := range graph.Actions {
		if action.StopID != stop.ID {
			continue
		}
		view := map[string]any{
			"id": action.ID, "action_ordinal": action.ActionOrdinal, "action_type": action.ActionType,
			"subject_type": action.SubjectType, "subject_id": action.SubjectID, "subject_version": action.SubjectVersion,
		}
		if action.WeightDeltaKg != nil {
			view["weight_delta_kg"] = action.WeightDeltaKg
		}
		if action.VolumeDeltaM3 != nil {
			view["volume_delta_m3"] = action.VolumeDeltaM3
		}
		if action.PalletDelta != nil {
			view["pallet_delta"] = action.PalletDelta
		}
		if action.LinearMetersDelta != nil {
			view["linear_meters_delta"] = action.LinearMetersDelta
		}
		if action.WindowStart != nil {
			view["window_start"] = action.WindowStart
		}
		if action.WindowEnd != nil {
			view["window_end"] = action.WindowEnd
		}
		if action.SourceShipmentID != nil {
			view["source_shipment_id"] = action.SourceShipmentID
		}
		if action.SourceShipmentVersion != nil {
			view["source_shipment_version"] = action.SourceShipmentVersion
		}
		if action.EvidenceState != "" {
			view["evidence_state"] = action.EvidenceState
		}
		if action.EvidenceStateVersion != nil {
			view["evidence_state_version"] = action.EvidenceStateVersion
		}
		if action.EvidenceOccurredAt != nil {
			view["evidence_occurred_at"] = action.EvidenceOccurredAt
		}
		if len(action.PublicSubjectSnapshot) > 0 {
			view["public_subject_snapshot"] = json.RawMessage(action.PublicSubjectSnapshot)
		}
		out = append(out, view)
	}
	return out
}

func stopShowsExact(stop repository.RouteStopRow, actions []map[string]any) bool {
	if stop.StopRole == routeplan.RoleStart || stop.StopRole == routeplan.RoleEnd {
		return true
	}
	if len(actions) == 0 {
		return true
	}
	for _, action := range actions {
		raw, _ := action["public_subject_snapshot"].(json.RawMessage)
		if snapshotGrantsExact(raw) {
			return true
		}
	}
	return false
}

func snapshotGrantsExact(raw []byte) bool {
	if len(raw) == 0 {
		return true
	}
	var doc struct {
		Visibility string `json:"visibility_scope"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return false
	}
	return doc.Visibility != domain.VisAnonymized
}

func coarseGeography(actions []map[string]any) (string, string, string) {
	for _, action := range actions {
		raw, _ := action["public_subject_snapshot"].(json.RawMessage)
		if len(raw) == 0 {
			continue
		}
		var doc struct {
			Pickup   domain.Place `json:"pickup"`
			Delivery domain.Place `json:"delivery"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			continue
		}
		place := doc.Pickup
		if action["action_type"] == routeplan.ActionDelivery {
			place = doc.Delivery
		}
		if place.CountryCode != "" || place.Region != "" || place.City != "" {
			return place.CountryCode, place.Region, place.City
		}
	}
	return "", "", ""
}

func fingerprintLines(lines []string) string {
	copied := append([]string(nil), lines...)
	sort.Strings(copied)
	sum := sha256.Sum256([]byte(strings.Join(copied, "\n")))
	return hex.EncodeToString(sum[:])
}

func catalogLines(refs []compat.CatalogVersionRef) []string {
	seen := map[string]struct{}{}
	lines := make([]string, 0, len(refs))
	for _, ref := range refs {
		tenant := ""
		if ref.TenantID != nil {
			tenant = *ref.TenantID
		}
		line := ref.ID + "|" + ref.CatalogKind + "|" + ref.Scope + "|" + tenant + "|" + strconv.Itoa(ref.Version)
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		lines = append(lines, line)
	}
	return lines
}

func ruleLines(refs []compat.RuleSetRef) []string {
	seen := map[string]struct{}{}
	lines := make([]string, 0, len(refs))
	for _, ref := range refs {
		tenant := ""
		if ref.TenantID != nil {
			tenant = *ref.TenantID
		}
		line := ref.ID + "|" + ref.Scope + "|" + tenant + "|" + strconv.Itoa(ref.Version)
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		lines = append(lines, line)
	}
	return lines
}

func mapPlanError(err error) error {
	var search *routeplan.SearchError
	if errors.As(err, &search) {
		return apperrors.Validation(search.Code, map[string]any{"reason": search.Code, "budget": search.Budget, "detail": search.Detail})
	}
	return err
}
