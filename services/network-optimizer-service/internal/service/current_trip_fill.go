package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/compat"
	"github.com/freight-platform/network-optimizer-service/internal/currenttrip"
	"github.com/freight-platform/network-optimizer-service/internal/domain"
	apperrors "github.com/freight-platform/network-optimizer-service/internal/platform/errors"
	bnometrics "github.com/freight-platform/network-optimizer-service/internal/platform/metrics"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
)

const (
	MaxAdditionalLoads = 1

	PolicyRehandlingForbidden = "REHANDLING_FORBIDDEN"
	PolicyRehandlingAllowed   = "REHANDLING_ALLOWED"

	PlacementSequenceOK       = "SEQUENCE_OK"
	PlacementSequenceConflict = "SEQUENCE_CONFLICT"
	PlacementRehandleRequired = "REHANDLE_REQUIRED"

	ReasonResidualUnknown      = "RESIDUAL_CAPACITY_UNKNOWN"
	ReasonPositionNotFresh     = "POSITION_NOT_FRESH"
	ReasonETANotFresh          = "ETA_NOT_FRESH"
	ReasonRoutingUnavailable   = "ROUTING_UNAVAILABLE"
	ReasonPickupWindowMiss     = "PICKUP_WINDOW_MISS"
	ReasonDeliveryWindowMiss   = "DELIVERY_WINDOW_MISS"
	ReasonPalletEquivalence    = "PALLET_EQUIVALENCE_UNKNOWN"
	ReasonCompatibilityUnknown = "CARGO_COMPATIBILITY_INDETERMINATE"
	ReasonRehandlingConflict   = "REHANDLING_CONFLICT"
	ReasonPayloadExceeded      = "PAYLOAD_EXCEEDED"
	ReasonVolumeExceeded       = "VOLUME_EXCEEDED"
	ReasonLinearExceeded       = "LINEAR_METERS_EXCEEDED"
	ReasonHeightExceeded       = "HEIGHT_EXCEEDED"
	ReasonPalletExceeded       = "PALLET_POSITIONS_EXCEEDED"
)

type CurrentTripSummary struct {
	ShipmentID        uuid.UUID  `json:"shipment_id"`
	ShipmentVersion   int        `json:"shipment_version"`
	VehicleID         *uuid.UUID `json:"vehicle_id,omitempty"`
	VehicleVersion    int        `json:"vehicle_version,omitempty"`
	PositionFreshness string     `json:"position_freshness"`
	ETAFreshness      string     `json:"eta_freshness"`
	InputFingerprint  string     `json:"input_fingerprint"`
	ConfirmedOnboard  int        `json:"confirmed_onboard"`
	ResidualPayload   string     `json:"residual_payload"`
	ResidualVolume    string     `json:"residual_volume"`
	ResidualPallets   string     `json:"residual_pallets"`
	ResidualLinear    string     `json:"residual_linear_meters"`
	HeightStatus      string     `json:"height_status"`
}

type RouteProof struct {
	Provider              string `json:"provider"`
	DistanceM             int    `json:"distance_m"`
	DurationSeconds       int    `json:"duration_seconds"`
	DetourDistanceM       int    `json:"detour_distance_m"`
	DetourDurationSeconds int    `json:"detour_duration_seconds"`
	RequestFingerprint    string `json:"request_fingerprint"`
}

type fillCandidate struct {
	load                domain.LoadOpportunity
	status              string
	compatibility       string
	hard                []string
	indeterminate       []string
	conditions          []string
	explanation         []string
	placement           string
	policy              string
	routing             *RouteProof
	fingerprint         string
	compatibilityPrints []string
	ruleVersions        []string
	catalogVersions     []string
	compatibilityTrace  []byte
	trace               []byte
	usage               *compat.Usage
}

func (s *Service) searchCurrentTripFill(ctx context.Context, actor Actor, cmd ConsolidationCommand, started time.Time) (Result, error) {
	if cmd.CapacityID != uuid.Nil {
		return Result{}, apperrors.Validation("capacity_id is not accepted for current-trip fill", map[string]any{"field": "capacity_id"})
	}
	if cmd.ShipmentID == uuid.Nil {
		return Result{}, apperrors.Validation("shipment_id is required", map[string]any{"field": "shipment_id"})
	}
	policy := cmd.Policy
	if policy == "" {
		policy = PolicyRehandlingForbidden
	}
	if policy != PolicyRehandlingForbidden && policy != PolicyRehandlingAllowed {
		return Result{}, apperrors.Validation("policy is not supported", map[string]any{"field": "policy"})
	}
	if cmd.CandidateLimit != nil && *cmd.CandidateLimit < 0 {
		return Result{}, apperrors.Validation("candidate_limit must be zero or greater", map[string]any{"field": "candidate_limit"})
	}
	if s.currentTrip == nil {
		return Result{}, apperrors.ServiceUnavailable("current trip context is unavailable")
	}
	if s.consolidations == nil {
		return Result{}, apperrors.ServiceUnavailable("consolidation persistence is unavailable")
	}
	trip, err := s.currentTrip.Build(ctx, actor.TenantID, cmd.ShipmentID)
	if err != nil {
		if errors.Is(err, currenttrip.ErrNotFound) {
			bnometrics.API("consolidation_search", "not_found")
			return Result{}, apperrors.NotFound("shipment is not available")
		}
		return Result{}, apperrors.ServiceUnavailable("current trip context is unavailable")
	}
	var pool []domain.LoadOpportunity
	err = s.store.Within(ctx, func(tx repository.Tx) error {
		rows, listErr := tx.ListPublicConsolidationPool(ctx, actor.TenantID, actor.CompanyID)
		if listErr != nil {
			return listErr
		}
		pool = rows
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	sort.Slice(pool, func(i, j int) bool { return pool[i].ID.String() < pool[j].ID.String() })
	excluded := map[string]int{}
	var assessed []fillCandidate
	for _, load := range pool {
		if !fillOptedIn(actor.TenantID, load) {
			if load.OwnerTenantID == actor.TenantID {
				excluded[ReasonSameOwnerOptInAbsent]++
			} else {
				excluded[ReasonCrossShipperOptInAbsent]++
			}
			continue
		}
		assessed = append(assessed, s.assessFill(ctx, actor.TenantID, policy, trip, load))
	}
	sort.Slice(assessed, func(i, j int) bool {
		if fillRank(assessed[i].status) != fillRank(assessed[j].status) {
			return fillRank(assessed[i].status) < fillRank(assessed[j].status)
		}
		return assessed[i].load.ID.String() < assessed[j].load.ID.String()
	})
	maxLoads := MaxAdditionalLoads
	execution := false
	response := ConsolidationResponse{
		SearchID: uuid.New(), Pattern: PatternCurrentTripFill, PoolLoadCount: len(pool),
		EvaluatedPairCount: len(assessed), ExcludedCountsByReason: nonzero(excluded),
		HardRejectCountsByReason: map[string]int{}, IndeterminateCountsByReason: map[string]int{},
		Candidates: []ConsolidationCandidateView{}, ShipmentID: &trip.ShipmentID, ShipmentVersion: trip.ShipmentVersion,
		MaxAdditionalLoads: &maxLoads, ExecutionSupported: &execution, InputFingerprint: trip.InputFingerprint,
		ContextSummary: summaryOf(trip),
	}
	now := s.now()
	stored := make([]repository.ConsolidationCandidate, 0, len(assessed))
	returnable := make([]ConsolidationCandidateView, 0, len(assessed))
	for _, item := range assessed {
		switch item.status {
		case ConsolidationFeasible:
			response.FeasibleCandidateCount++
		case ConsolidationIndeterminate:
			response.IndeterminateCandidateCount++
			for _, code := range item.indeterminate {
				response.IndeterminateCountsByReason[code]++
			}
		default:
			response.HardRejectCandidateCount++
			for _, code := range item.hard {
				response.HardRejectCountsByReason[code]++
			}
		}
		row := item.stored(response.SearchID, actor.TenantID, trip, now)
		view := item.view()
		view.CandidateID = row.ID
		returnable = append(returnable, view)
		stored = append(stored, row)
	}
	limit := len(returnable)
	if cmd.CandidateLimit != nil && *cmd.CandidateLimit < limit {
		limit = *cmd.CandidateLimit
	}
	response.Candidates = append([]ConsolidationCandidateView(nil), returnable[:limit]...)
	response.ReturnedCandidateCount = len(response.Candidates)
	run := repository.ConsolidationRun{
		ID: response.SearchID, TenantID: actor.TenantID, Pattern: PatternCurrentTripFill,
		StartedAt: started, CompletedAt: now, Status: "COMPLETED", CandidateLimit: cmd.CandidateLimit,
		PoolLoadCount: response.PoolLoadCount, EvaluatedPairCount: response.EvaluatedPairCount, CreatedAt: now,
	}
	if err := s.consolidations.SaveConsolidation(ctx, run, stored); err != nil {
		return Result{}, err
	}
	raw, err := json.Marshal(response)
	if err != nil {
		return Result{}, err
	}
	bnometrics.ConsolidationSearch(time.Since(started), response.EvaluatedPairCount, response.FeasibleCandidateCount, response.IndeterminateCandidateCount, response.HardRejectCandidateCount)
	bnometrics.API("consolidation_search", "ok")
	return Result{Status: http.StatusOK, Body: raw, AggregateID: response.SearchID}, nil
}

func fillOptedIn(searcher uuid.UUID, load domain.LoadOpportunity) bool {
	if load.OwnerTenantID == searcher {
		return load.ConsolidationAllowed
	}
	return load.CrossShipperConsolidationAllowed
}

func fillRank(status string) int {
	switch status {
	case ConsolidationFeasible:
		return 0
	case ConsolidationIndeterminate:
		return 1
	default:
		return 2
	}
}

func summaryOf(trip currenttrip.CurrentTripContext) *CurrentTripSummary {
	confirmed := 0
	for _, unit := range trip.OnboardCargoUnits {
		if unit.EvidenceState == currenttrip.EvidenceConfirmedOnboard {
			confirmed++
		}
	}
	return &CurrentTripSummary{
		ShipmentID: trip.ShipmentID, ShipmentVersion: trip.ShipmentVersion,
		VehicleID: trip.VehicleID, VehicleVersion: trip.VehicleVersion,
		PositionFreshness: trip.PositionFreshnessStatus, ETAFreshness: trip.ETAFreshnessStatus,
		InputFingerprint: trip.InputFingerprint, ConfirmedOnboard: confirmed,
		ResidualPayload: trip.ResidualCapacity.Payload.Status, ResidualVolume: trip.ResidualCapacity.Volume.Status,
		ResidualPallets: trip.ResidualCapacity.PalletPositions.Status, ResidualLinear: trip.ResidualCapacity.LinearMeters.Status,
		HeightStatus: trip.ResidualCapacity.Height.Status,
	}
}

func (s *Service) assessFill(ctx context.Context, searcher uuid.UUID, policy string, trip currenttrip.CurrentTripContext, load domain.LoadOpportunity) fillCandidate {
	out := fillCandidate{
		load: load, status: ConsolidationIndeterminate, compatibility: compat.StatusIndeterminate,
		placement: PlacementNotEvaluated, policy: policy,
	}
	confirmed := confirmedUnits(trip)
	if len(confirmed) == 0 {
		out.indeterminate = []string{currenttrip.ReasonUnproven}
		out.explanation = append([]string(nil), out.indeterminate...)
		out.fingerprint = fillFingerprint(trip, load, out, nil)
		out.trace = fillTrace(trip, load, out, nil)
		return out
	}
	if exceeds(trip.ResidualCapacity.Payload) || exceeds(trip.ResidualCapacity.Volume) || exceeds(trip.ResidualCapacity.PalletPositions) || exceeds(trip.ResidualCapacity.LinearMeters) || trip.ResidualCapacity.Height.Status == currenttrip.HeightKnownExceeded {
		out.hard = append(out.hard, currenttrip.DimensionExceeds)
	}
	addResidual(&out, load.WeightKg, trip.ResidualCapacity.Payload, ReasonPayloadExceeded)
	addResidual(&out, load.VolumeM3, trip.ResidualCapacity.Volume, ReasonVolumeExceeded)
	addResidual(&out, load.Cargo.LinearMeters, trip.ResidualCapacity.LinearMeters, ReasonLinearExceeded)
	s.addPallets(&out, load, trip)
	addHeight(&out, load, trip)
	items := make([]compat.GroupageItem, 0, len(confirmed)+1)
	for _, unit := range confirmed {
		items = append(items, compat.GroupageItem{Cargo: cargoFromProfile(unit)})
	}
	items = append(items, compat.GroupageItem{Cargo: cargoFromLoad(load), AccessNeed: accessFromLoad(load)})
	group := s.fillGroupage(ctx, searcher, load.OwnerTenantID, equipmentFromVehicle(trip.VehicleCapability), items)
	out.compatibility = group.compatibility
	out.usage = group.usage
	out.compatibilityPrints = append([]string(nil), group.fingerprints...)
	out.ruleVersions = append([]string(nil), group.ruleVersions...)
	out.catalogVersions = append([]string(nil), group.catalogVersions...)
	out.compatibilityTrace = append([]byte(nil), group.trace...)
	out.conditions = append(out.conditions, group.conditions...)
	for _, code := range group.hard {
		out.hard = appendUnique(out.hard, code)
	}
	for _, code := range group.indeterminate {
		if code == ReasonCompatibilityUnknown || !containsCode(out.hard, code) {
			out.indeterminate = appendUnique(out.indeterminate, code)
		}
	}
	if group.compatibility == compat.StatusIndeterminate && !containsCode(out.indeterminate, ReasonMultiPartyUnavailable) {
		out.indeterminate = appendUnique(out.indeterminate, ReasonCompatibilityUnknown)
	}
	if !fresh(trip.PositionFreshnessStatus) {
		out.indeterminate = appendUnique(out.indeterminate, ReasonPositionNotFresh)
	}
	if !fresh(trip.ETAFreshnessStatus) || trip.ETA == nil || trip.ETA.EstimatedArrivalAt == nil {
		out.indeterminate = appendUnique(out.indeterminate, ReasonETANotFresh)
	}
	var proof *RouteProof
	if fresh(trip.PositionFreshnessStatus) {
		routed, pickupAt, deliveryAt, routeErr := s.insertRoad(ctx, searcher, trip, load)
		if routeErr != nil || routed == nil {
			out.indeterminate = appendUnique(out.indeterminate, ReasonRoutingUnavailable)
		} else {
			proof = routed
			out.routing = routed
			addWindows(&out, load, pickupAt, deliveryAt)
			out.placement = placementOf(policy, trip, load)
		}
	}
	if out.placement == PlacementNotEvaluated && len(out.hard) == 0 {
		out.indeterminate = appendUnique(out.indeterminate, ReasonCompatibilityUnknown)
	}
	if out.placement == PlacementSequenceConflict {
		out.hard = appendUnique(out.hard, ReasonRehandlingConflict)
	}
	if len(out.hard) > 0 {
		out.status = ConsolidationHardReject
		if out.compatibility == compat.StatusCompatible {
			out.compatibility = compat.StatusIncompatible
		}
	} else if len(out.indeterminate) > 0 || (out.placement != PlacementSequenceOK && out.placement != PlacementRehandleRequired) {
		out.status = ConsolidationIndeterminate
	} else {
		out.status = ConsolidationFeasible
		out.compatibility = compat.StatusCompatible
	}
	out.explanation = append(append([]string{}, out.hard...), out.indeterminate...)
	out.fingerprint = fillFingerprint(trip, load, out, proof)
	out.trace = fillTrace(trip, load, out, proof)
	return out
}

func confirmedUnits(trip currenttrip.CurrentTripContext) []currenttrip.OnboardCargoUnit {
	var units []currenttrip.OnboardCargoUnit
	for _, unit := range trip.OnboardCargoUnits {
		if unit.EvidenceState == currenttrip.EvidenceConfirmedOnboard {
			units = append(units, unit)
		}
	}
	return units
}

func exceeds(dim currenttrip.SubtractiveDimension) bool {
	return dim.Status == currenttrip.DimensionExceeds
}

func addResidual(out *fillCandidate, additional *float64, dim currenttrip.SubtractiveDimension, exceed string) {
	if exceeds(dim) {
		return
	}
	if additional == nil {
		return
	}
	if dim.Status != currenttrip.DimensionKnown || dim.Remaining == nil {
		out.indeterminate = appendUnique(out.indeterminate, ReasonResidualUnknown)
		return
	}
	if *additional > *dim.Remaining {
		out.hard = appendUnique(out.hard, exceed)
	}
}

func (s *Service) addPallets(out *fillCandidate, load domain.LoadOpportunity, trip currenttrip.CurrentTripContext) {
	if load.Cargo.PalletCount == nil {
		return
	}
	if exceeds(trip.ResidualCapacity.PalletPositions) {
		return
	}
	code := ""
	if load.Cargo.PalletTypeCode != nil {
		code = *load.Cargo.PalletTypeCode
	}
	factor, ok := provenPalletFactor(code, s.currentTrip.Equivalences())
	if !ok {
		out.indeterminate = appendUnique(out.indeterminate, ReasonPalletEquivalence)
		return
	}
	if trip.ResidualCapacity.PalletPositions.Status != currenttrip.DimensionKnown || trip.ResidualCapacity.PalletPositions.Remaining == nil {
		out.indeterminate = appendUnique(out.indeterminate, ReasonResidualUnknown)
		return
	}
	used := float64(*load.Cargo.PalletCount) * factor
	if used > *trip.ResidualCapacity.PalletPositions.Remaining {
		out.hard = appendUnique(out.hard, ReasonPalletExceeded)
	}
}

func provenPalletFactor(code string, rows []currenttrip.PalletEquivalence) (float64, bool) {
	if code == "" {
		return 0, false
	}
	for _, row := range rows {
		if row.FromCode == code && row.OwnershipProven && row.PositionsEach > 0 {
			return row.PositionsEach, true
		}
	}
	return 0, false
}

func addHeight(out *fillCandidate, load domain.LoadOpportunity, trip currenttrip.CurrentTripContext) {
	if trip.ResidualCapacity.Height.Status == currenttrip.HeightKnownExceeded {
		out.hard = appendUnique(out.hard, ReasonHeightExceeded)
		return
	}
	if load.Cargo.MaxLoadedHeightMM == nil {
		return
	}
	if trip.VehicleCapability == nil || trip.VehicleCapability.InternalHeightMM == nil {
		out.indeterminate = appendUnique(out.indeterminate, ReasonResidualUnknown)
		return
	}
	if *load.Cargo.MaxLoadedHeightMM > *trip.VehicleCapability.InternalHeightMM {
		out.hard = appendUnique(out.hard, ReasonHeightExceeded)
	}
}

type groupOutcome struct {
	compatibility   string
	hard            []string
	indeterminate   []string
	conditions      []string
	usage           *compat.Usage
	fingerprints    []string
	ruleVersions    []string
	catalogVersions []string
	trace           []byte
}

func (s *Service) fillGroupage(ctx context.Context, searcher, loadOwner uuid.UUID, equipment compat.Equipment, items []compat.GroupageItem) groupOutcome {
	base := assessedPair{compatibility: compat.StatusIndeterminate, crossShipper: searcher != loadOwner}
	base = s.assessSameOwner(ctx, base, equipment, items, "ok", "ok", searcher, loadOwner, nil)
	return groupOutcome{
		compatibility: base.compatibility, hard: base.hard, indeterminate: base.indeterminate,
		conditions: base.conditions, usage: base.usage,
		fingerprints: append([]string(nil), base.fingerprints...), ruleVersions: append([]string(nil), base.ruleVersions...),
		catalogVersions: append([]string(nil), base.catalogVersions...), trace: append([]byte(nil), base.trace...),
	}
}

func (s *Service) insertRoad(ctx context.Context, tenant uuid.UUID, trip currenttrip.CurrentTripContext, load domain.LoadOpportunity) (*RouteProof, time.Time, time.Time, error) {
	if s.routes == nil || trip.CurrentPosition == nil || trip.CurrentPosition.Latitude == nil || trip.CurrentPosition.Longitude == nil || trip.ETA == nil || trip.ETA.EstimatedArrivalAt == nil {
		return nil, time.Time{}, time.Time{}, routing.ErrProviderUnavailable
	}
	position := routing.Point{Latitude: *trip.CurrentPosition.Latitude, Longitude: *trip.CurrentPosition.Longitude}
	pickup, ok := s.pointFor(ctx, tenant, load.Pickup.LocationID, load.Pickup.Latitude, load.Pickup.Longitude)
	if !ok {
		return nil, time.Time{}, time.Time{}, routing.ErrProviderUnavailable
	}
	delivery, ok := s.pointFor(ctx, tenant, load.Delivery.LocationID, load.Delivery.Latitude, load.Delivery.Longitude)
	if !ok {
		return nil, time.Time{}, time.Time{}, routing.ErrProviderUnavailable
	}
	destination, ok := s.pointFor(ctx, tenant, &trip.DestinationLocationID, nil, nil)
	if !ok {
		return nil, time.Time{}, time.Time{}, routing.ErrProviderUnavailable
	}
	direct, err := s.roadLeg(ctx, position, destination)
	if err != nil {
		return nil, time.Time{}, time.Time{}, err
	}
	toPickup, err := s.roadLeg(ctx, position, pickup)
	if err != nil {
		return nil, time.Time{}, time.Time{}, err
	}
	toDelivery, err := s.roadLeg(ctx, pickup, delivery)
	if err != nil {
		return nil, time.Time{}, time.Time{}, err
	}
	toDestination, err := s.roadLeg(ctx, delivery, destination)
	if err != nil {
		return nil, time.Time{}, time.Time{}, err
	}
	insertedDistance := toPickup.DistanceM + toDelivery.DistanceM + toDestination.DistanceM
	insertedDuration := toPickup.DurationSeconds + toDelivery.DurationSeconds + toDestination.DurationSeconds
	positionAt := trip.ETA.EstimatedArrivalAt.Add(-time.Duration(direct.DurationSeconds) * time.Second)
	pickupAt := positionAt.Add(time.Duration(toPickup.DurationSeconds) * time.Second)
	deliveryAt := pickupAt.Add(time.Duration(toDelivery.DurationSeconds) * time.Second)
	return &RouteProof{
		Provider: routingProviderName(s.routes), DistanceM: insertedDistance, DurationSeconds: insertedDuration,
		DetourDistanceM: insertedDistance - direct.DistanceM, DetourDurationSeconds: insertedDuration - direct.DurationSeconds,
		RequestFingerprint: toPickup.RequestFingerprint + "|" + toDelivery.RequestFingerprint + "|" + toDestination.RequestFingerprint + "|" + direct.RequestFingerprint,
	}, pickupAt, deliveryAt, nil
}

func (s *Service) pointFor(ctx context.Context, tenant uuid.UUID, id *uuid.UUID, lat, lon *float64) (routing.Point, bool) {
	if lat != nil && lon != nil {
		return routing.Point{Latitude: *lat, Longitude: *lon}, true
	}
	if s.directory == nil || id == nil || *id == uuid.Nil {
		return routing.Point{}, false
	}
	snap, err := s.directory.Projection(ctx, tenant, *id)
	if err != nil || snap.Latitude == nil || snap.Longitude == nil {
		return routing.Point{}, false
	}
	return routing.Point{Latitude: *snap.Latitude, Longitude: *snap.Longitude}, true
}

func (s *Service) roadLeg(ctx context.Context, from, to routing.Point) (routing.RouteResult, error) {
	if s.routes == nil {
		return routing.RouteResult{}, routing.ErrProviderUnavailable
	}
	return s.routes.Route(ctx, routing.RouteRequest{
		Origin: from, Destination: to, RouteMode: routing.RouteFastest, TrafficMode: routing.TrafficCurrent,
	})
}

func addWindows(out *fillCandidate, load domain.LoadOpportunity, pickupAt, deliveryAt time.Time) {
	if load.PickupWindow.Start == nil || load.PickupWindow.End == nil {
		out.indeterminate = appendUnique(out.indeterminate, ReasonPickupUnknown)
	} else if pickupAt.Before(*load.PickupWindow.Start) || !pickupAt.Before(*load.PickupWindow.End) {
		out.hard = appendUnique(out.hard, ReasonPickupWindowMiss)
	}
	if load.DeliveryWindow.Start == nil || load.DeliveryWindow.End == nil {
		out.indeterminate = appendUnique(out.indeterminate, ReasonDeliveryUnknown)
	} else if deliveryAt.Before(*load.DeliveryWindow.Start) || !deliveryAt.Before(*load.DeliveryWindow.End) {
		out.hard = appendUnique(out.hard, ReasonDeliveryWindowMiss)
	}
}

func placementOf(policy string, trip currenttrip.CurrentTripContext, load domain.LoadOpportunity) string {
	if load.Pickup.LocationID == nil || load.Delivery.LocationID == nil || trip.DestinationLocationID == uuid.Nil {
		return PlacementNotEvaluated
	}
	pickupAtDestination := *load.Pickup.LocationID == trip.DestinationLocationID
	deliveryAtDestination := *load.Delivery.LocationID == trip.DestinationLocationID
	if pickupAtDestination && !deliveryAtDestination {
		if policy == PolicyRehandlingAllowed {
			return PlacementRehandleRequired
		}
		return PlacementSequenceConflict
	}
	return PlacementSequenceOK
}

func fresh(status string) bool { return status == "FRESH" }

func containsCode(codes []string, code string) bool {
	for _, item := range codes {
		if item == code {
			return true
		}
	}
	return false
}

func cargoFromProfile(unit currenttrip.OnboardCargoUnit) compat.Cargo {
	profile := unit.Profile
	return compat.Cargo{
		ID: unit.CargoID.String(), CargoTypeCode: profile.CargoTypeCode, WeightKg: profile.WeightKg,
		VolumeM3: profile.VolumeM3, PalletCount: profile.PalletCount, PalletTypeCode: profile.PalletTypeCode,
		LinearMeters: profile.LinearMeters, MaxLoadedHeightMM: profile.MaxLoadedHeightMM, Stackable: profile.Stackable,
		Fragile: profile.Fragile, PackagingTypeCode: profile.PackagingTypeCode, FoodGradeRequired: profile.FoodGradeRequired,
		TemperatureRequired: profile.TemperatureRequired, TemperatureMinC: profile.TemperatureMinC, TemperatureMaxC: profile.TemperatureMaxC,
		PreferredSetpointC: profile.PreferredTemperatureSetpointC, DangerousGoods: profile.DangerousGoods,
		HazardClasses: append([]string(nil), profile.HazardClasses...), OdorEmissionClass: profile.OdorEmissionClass,
		OdorSensitive: profile.OdorSensitive, ContaminationClass: profile.ContaminationClass,
	}
}

func equipmentFromVehicle(vehicle *currenttrip.VehicleCapability) compat.Equipment {
	if vehicle == nil {
		return compat.Equipment{}
	}
	return compat.Equipment{
		BodyType: vehicle.BodyType, EquipmentTypeCode: vehicle.EquipmentType, CombinationType: vehicle.CombinationType,
		PayloadKg: vehicle.CapacityWeight, VolumeM3: vehicle.CapacityVolume, PalletPositions: vehicle.PalletPositions,
		UsableLinearMeters: vehicle.UsableLinearMeters, InternalLengthMM: vehicle.InternalLengthMM,
		InternalWidthMM: vehicle.InternalWidthMM, InternalHeightMM: vehicle.InternalHeightMM,
		LoadingAccess: append([]string(nil), vehicle.LoadingAccess...), UnloadingAccess: append([]string(nil), vehicle.UnloadingAccess...),
		TemperatureControlMode: vehicle.TemperatureControlMode, TemperatureMinC: vehicle.TemperatureCapabilityMinC,
		TemperatureMaxC: vehicle.TemperatureCapabilityMaxC, TemperatureZoneCount: vehicle.TemperatureZoneCount,
		IndependentTemperatureControl: vehicle.IndependentTemperatureControl, FoodGradeCapability: vehicle.FoodGradeCapability,
		ADRCapability: vehicle.ADRCapability,
	}
}

func fillFingerprint(trip currenttrip.CurrentTripContext, load domain.LoadOpportunity, out fillCandidate, proof *RouteProof) string {
	vehicle := ""
	if trip.VehicleID != nil {
		vehicle = trip.VehicleID.String()
	}
	evidence := make([]string, 0, len(trip.OnboardCargoUnits))
	for _, unit := range trip.OnboardCargoUnits {
		evidence = append(evidence, unit.CargoID.String()+":"+unit.EvidenceState+":"+itoa(unit.EvidenceStateVersion)+":"+itoa(unit.CargoVersion))
	}
	sort.Strings(evidence)
	reasons := append(append([]string{}, out.hard...), out.indeterminate...)
	sort.Strings(reasons)
	prints := append([]string(nil), out.compatibilityPrints...)
	rules := append([]string(nil), out.ruleVersions...)
	catalogs := append([]string(nil), out.catalogVersions...)
	sort.Strings(prints)
	sort.Strings(rules)
	sort.Strings(catalogs)
	doc := map[string]any{
		"shipment_id": trip.ShipmentID, "shipment_version": trip.ShipmentVersion,
		"vehicle_id": vehicle, "vehicle_version": trip.VehicleVersion,
		"evidence": evidence, "load_id": load.ID, "load_version": load.Version,
		"context_fingerprint": trip.InputFingerprint,
		"position_freshness":  trip.PositionFreshnessStatus, "position_observed": observedStamp(trip.PositionObservedAt),
		"eta_freshness": trip.ETAFreshnessStatus, "eta_observed": observedStamp(trip.ETAObservedAt),
		"policy": out.policy, "placement": out.placement, "status": out.status, "reasons": reasons,
		"compatibility_fingerprints": prints, "rule_sets": rules, "catalog_versions": catalogs,
	}
	if proof != nil {
		doc["routing_fingerprint"] = proof.RequestFingerprint
		doc["distance_m"] = proof.DistanceM
		doc["duration_seconds"] = proof.DurationSeconds
		doc["detour_distance_m"] = proof.DetourDistanceM
		doc["detour_duration_seconds"] = proof.DetourDurationSeconds
		doc["provider"] = proof.Provider
	}
	raw, _ := json.Marshal(doc)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func fillTrace(trip currenttrip.CurrentTripContext, load domain.LoadOpportunity, out fillCandidate, proof *RouteProof) []byte {
	onboard := make([]map[string]any, 0, len(trip.OnboardCargoUnits))
	for _, unit := range trip.OnboardCargoUnits {
		onboard = append(onboard, map[string]any{
			"cargo_id": unit.CargoID, "cargo_version": unit.CargoVersion,
			"evidence_state": unit.EvidenceState, "evidence_state_version": unit.EvidenceStateVersion,
		})
	}
	evaluations := out.compatibilityTrace
	if len(evaluations) == 0 {
		evaluations = []byte("[]")
	}
	doc := map[string]any{
		"shipment_id": trip.ShipmentID, "shipment_version": trip.ShipmentVersion,
		"vehicle_version": trip.VehicleVersion, "confirmed_onboard": onboard,
		"additional_load_id": load.ID, "additional_load_version": load.Version,
		"position": map[string]any{"freshness": trip.PositionFreshnessStatus, "observed_at": observedStamp(trip.PositionObservedAt)},
		"eta":      map[string]any{"freshness": trip.ETAFreshnessStatus, "observed_at": observedStamp(trip.ETAObservedAt)},
		"residual_capacity": map[string]any{
			"payload":          residualAudit(trip.ResidualCapacity.Payload),
			"volume":           residualAudit(trip.ResidualCapacity.Volume),
			"pallet_positions": residualAudit(trip.ResidualCapacity.PalletPositions),
			"linear_meters":    residualAudit(trip.ResidualCapacity.LinearMeters),
			"height": map[string]any{
				"status": trip.ResidualCapacity.Height.Status, "provenance": trip.ResidualCapacity.Height.Provenance,
			},
		},
		"policy": out.policy, "placement_check": out.placement,
		"compatibility_fingerprints": append([]string(nil), out.compatibilityPrints...),
		"compatibility_evaluations":  json.RawMessage(evaluations),
		"hard_reasons":               append([]string(nil), out.hard...),
		"indeterminate_reasons":      append([]string(nil), out.indeterminate...),
		"conditions":                 append([]string(nil), out.conditions...),
		"candidate_fingerprint":      out.fingerprint,
		"input_fingerprint":          trip.InputFingerprint,
	}
	if trip.VehicleID != nil {
		doc["vehicle_id"] = trip.VehicleID.String()
	}
	if proof != nil {
		doc["routing"] = proof
	}
	raw, _ := json.Marshal(doc)
	return raw
}

func residualAudit(dim currenttrip.SubtractiveDimension) map[string]any {
	return map[string]any{"status": dim.Status, "provenance": dim.Provenance, "reason": dim.Reason}
}

func observedStamp(at *time.Time) string {
	if at == nil {
		return ""
	}
	return at.UTC().Format(time.RFC3339Nano)
}

func itoa(v int) string {
	return jsonNumber(v)
}

func sortedCopy(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func jsonNumber(v int) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

func (c fillCandidate) view() ConsolidationCandidateView {
	view := ConsolidationCandidateView{
		Status: c.status, ExecutionSupported: false, PlacementCheck: c.placement,
		Compatibility: c.compatibility, CapacityUsage: c.usage, Conditions: c.conditions,
		IndeterminateReasonCodes: c.indeterminate, Explanation: c.explanation, Routing: c.routing,
		Members: []ConsolidationMemberView{{
			Ordinal: 1, LoadOpportunityID: c.load.ID, LoadVersion: c.load.Version, Load: c.load.MarketplaceView(),
		}},
	}
	return view
}

func (c fillCandidate) stored(searchID, tenant uuid.UUID, trip currenttrip.CurrentTripContext, now time.Time) repository.ConsolidationCandidate {
	conditions, _ := json.Marshal(c.conditions)
	trace := c.trace
	if len(trace) == 0 {
		trace = []byte("{}")
	}
	reasons := append([]string{}, c.hard...)
	sort.Strings(reasons)
	return repository.ConsolidationCandidate{
		ID: uuid.New(), SearchRunID: searchID, TenantID: tenant, Pattern: PatternCurrentTripFill,
		Status: c.status, ExecutionSupported: false, CompatibilityStatus: c.compatibility,
		CompatibilityFingerprint: joinPrints(sortedCopy(c.compatibilityPrints)), CandidateFingerprint: c.fingerprint,
		PlacementCheck: c.placement, HardRejectReasons: emptyStrings(reasons),
		IndeterminateReasonCodes: emptyStrings(c.indeterminate), Conditions: conditions,
		Warnings: []byte("[]"), CompatibilityTrace: trace, CreatedAt: now,
		Members: []repository.ConsolidationMember{{
			Ordinal: 1, LoadOpportunityID: c.load.ID, LoadVersion: c.load.Version,
			LoadOwnerTenantID: c.load.OwnerTenantID, CreatedAt: now,
		}},
	}
}
