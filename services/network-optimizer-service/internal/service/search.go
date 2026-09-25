package service

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/compat"
	"github.com/freight-platform/network-optimizer-service/internal/domain"
	apperrors "github.com/freight-platform/network-optimizer-service/internal/platform/errors"
	bnometrics "github.com/freight-platform/network-optimizer-service/internal/platform/metrics"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
)

type SearchCommand struct {
	CapacityID     uuid.UUID
	Policy         domain.NextLoadSearchPolicy
	CandidateLimit *int
}

type SearchCandidateView struct {
	LoadOpportunityID   uuid.UUID              `json:"load_opportunity_id"`
	LoadVersion         int                    `json:"load_version"`
	Eligibility         string                 `json:"eligibility"`
	Load                domain.MarketplaceLoad `json:"load"`
	DeadheadBucket      string                 `json:"deadhead_bucket,omitempty"`
	RoadDeadheadKm      *float64               `json:"road_deadhead_km,omitempty"`
	RoadDeadheadMinutes *float64               `json:"road_deadhead_minutes,omitempty"`
	ForwardProgressKm   *float64               `json:"forward_progress_km,omitempty"`
	LateralDistanceKm   *float64               `json:"lateral_distance_km,omitempty"`
	RouteIncreaseKm     *float64               `json:"route_increase_km,omitempty"`
	Timing              string                 `json:"timing"`
	WaitingMinutes      *float64               `json:"waiting_minutes,omitempty"`
	Compatibility       string                 `json:"compatibility"`
	Explanation         []string               `json:"explanation,omitempty"`
}

type SearchResponse struct {
	SearchID                uuid.UUID                   `json:"search_id"`
	CapacityID              uuid.UUID                   `json:"capacity_id"`
	CapacityVersion         int                         `json:"capacity_version"`
	SearchMode              string                      `json:"search_mode"`
	EffectivePolicy         domain.NextLoadSearchPolicy `json:"effective_policy"`
	PolicyFingerprint       string                      `json:"effective_policy_fingerprint"`
	EligibleCandidateCount  int                         `json:"eligible_candidate_count"`
	RejectionCountsByReason map[string]int              `json:"rejection_counts_by_reason"`
	Candidates              []SearchCandidateView       `json:"candidates"`
}

func (s *Service) SearchNextLoad(ctx context.Context, actor Actor, cmd SearchCommand) (Result, error) {
	started := s.now()
	if cmd.CapacityID == uuid.Nil {
		return Result{}, apperrors.Validation("capacity_id is required", map[string]any{"field": "capacity_id"})
	}
	if cmd.CandidateLimit != nil && *cmd.CandidateLimit < 0 {
		return Result{}, apperrors.Validation("candidate_limit must be zero or greater", map[string]any{"field": "candidate_limit"})
	}
	var capacity domain.Capacity
	err := s.store.Within(ctx, func(tx repository.Tx) error {
		row, err := tx.GetCapacity(ctx, cmd.CapacityID)
		if err != nil {
			return err
		}
		capacity = row
		return nil
	})
	if err != nil || capacity.OwnerTenantID != actor.TenantID {
		bnometrics.API("next_load_search", "not_found")
		return Result{}, apperrors.NotFound("capacity is not available")
	}
	if capacity.Status != domain.CapacityAvailable {
		return Result{}, apperrors.Validation("capacity is not executable", map[string]any{"reason": "CAPACITY_NOT_EXECUTABLE"})
	}
	if capacity.Latitude == nil || capacity.Longitude == nil || capacity.AvailableUntil.IsZero() || !capacity.AvailableUntil.After(capacity.AvailableFrom) {
		return Result{}, apperrors.Validation("capacity search geography or availability is incomplete", nil)
	}
	availableAt, prediction, err := s.effectiveAvailableAt(ctx, actor.TenantID, capacity)
	if err != nil {
		return Result{}, err
	}
	effective, err := s.effectivePolicy(ctx, actor.TenantID, capacity.ID, cmd.Policy)
	if err != nil {
		return Result{}, err
	}
	if err := effective.Validate(); err != nil {
		return Result{}, apperrors.Validation(err.Error(), nil)
	}
	target, line, err := s.targetRoute(ctx, actor.TenantID, capacity, effective, availableAt)
	if err != nil {
		return Result{}, err
	}
	loads, err := s.visibleLoads(ctx, actor)
	if err != nil {
		return Result{}, err
	}
	sort.Slice(loads, func(i, j int) bool { return loads[i].ID.String() < loads[j].ID.String() })
	bnometrics.SearchPool(len(loads))
	evaluated := s.evaluateLoads(ctx, actor.TenantID, capacity, prediction, availableAt, effective, target, line, loads)
	runID := uuid.New()
	now := s.now()
	providerName := routingProviderName(s.routes)
	if err := s.persistSearch(ctx, runID, actor.TenantID, capacity, effective, providerName, started, now, evaluated); err != nil {
		return Result{}, err
	}
	response := s.publicSearch(runID, capacity, effective, evaluated, cmd.CandidateLimit)
	raw, err := json.Marshal(response)
	if err != nil {
		return Result{}, err
	}
	bnometrics.SearchRun(time.Since(started), response.EligibleCandidateCount, len(evaluated)-response.EligibleCandidateCount, response.RejectionCountsByReason)
	bnometrics.API("next_load_search", "ok")
	return Result{Status: http.StatusOK, Body: raw, AggregateID: runID}, nil
}

func (s *Service) effectiveAvailableAt(ctx context.Context, tenant uuid.UUID, capacity domain.Capacity) (time.Time, *domain.PredictedCapacity, error) {
	if capacity.Source != domain.SourceCurrentShipmentPrediction {
		return capacity.AvailableFrom, nil, nil
	}
	lookup, ok := s.store.(interface {
		CurrentPredictionForCapacity(context.Context, uuid.UUID, uuid.UUID) (domain.PredictedCapacity, error)
	})
	if !ok {
		return time.Time{}, nil, apperrors.Validation("predicted availability is unavailable", map[string]any{"reason": "CAPACITY_NOT_EXECUTABLE"})
	}
	prediction, err := lookup.CurrentPredictionForCapacity(ctx, tenant, capacity.ID)
	if err != nil {
		return time.Time{}, nil, apperrors.Validation("predicted availability is unavailable", map[string]any{"reason": "CAPACITY_NOT_EXECUTABLE"})
	}
	return prediction.PredictedAvailableAt, &prediction, nil
}

func (s *Service) effectivePolicy(ctx context.Context, tenant, capacityID uuid.UUID, request domain.NextLoadSearchPolicy) (domain.NextLoadSearchPolicy, error) {
	if err := request.ValidateOverride(); err != nil {
		return domain.NextLoadSearchPolicy{}, apperrors.Validation(err.Error(), nil)
	}
	var carrier, capacity domain.NextLoadSearchPolicy
	if s.policies != nil {
		if row, err := s.policies.GetCarrierPolicy(ctx, tenant); err == nil {
			carrier = row
		}
		if row, err := s.policies.GetCapacityPolicy(ctx, tenant, capacityID); err == nil {
			capacity = row
		}
	}
	return domain.EffectiveSearchPolicy(carrier, capacity, request), nil
}

func (s *Service) targetRoute(ctx context.Context, tenant uuid.UUID, capacity domain.Capacity, policy domain.NextLoadSearchPolicy, availableAt time.Time) (routing.Point, [][]float64, error) {
	if policy.SearchMode == domain.SearchRadius {
		return routing.Point{}, nil, nil
	}
	if policy.TargetLocationID == nil || s.directory == nil {
		return routing.Point{}, nil, apperrors.Validation(domain.ErrDirectionTargetGeo.Error(), nil)
	}
	snap, err := s.directory.Projection(ctx, tenant, *policy.TargetLocationID)
	if err != nil || snap.Latitude == nil || snap.Longitude == nil {
		return routing.Point{}, nil, apperrors.Validation(domain.ErrDirectionTargetGeo.Error(), nil)
	}
	target := routing.Point{Latitude: *snap.Latitude, Longitude: *snap.Longitude}
	if s.routes == nil {
		return target, nil, nil
	}
	result, err := s.routes.Route(ctx, routing.RouteRequest{
		Origin: routing.Point{Latitude: *capacity.Latitude, Longitude: *capacity.Longitude}, Destination: target,
		DepartureAt: &availableAt, RouteMode: routing.RouteFastest, TrafficMode: routing.TrafficStatistical,
	})
	if err != nil {
		bnometrics.RoutingError()
		return target, nil, nil
	}
	return target, result.Geometry.Coordinates, nil
}

func (s *Service) visibleLoads(ctx context.Context, actor Actor) ([]domain.LoadOpportunity, error) {
	var loads []domain.LoadOpportunity
	err := s.store.Within(ctx, func(tx repository.Tx) error {
		rows, err := tx.ListMarketplaceLoads(ctx, actor.TenantID, actor.CompanyID, 100000, 0)
		if err != nil {
			return err
		}
		loads = rows
		return nil
	})
	return loads, err
}

type evaluatedLoad struct {
	load          domain.LoadOpportunity
	reasons       []string
	roadKm        *float64
	roadMinutes   *float64
	forward       *float64
	lateral       *float64
	increase      *float64
	timing        string
	waiting       *float64
	compatibility string
	fingerprint   string
}

func (s *Service) evaluateLoads(ctx context.Context, tenant uuid.UUID, capacity domain.Capacity, prediction *domain.PredictedCapacity, availableAt time.Time, policy domain.NextLoadSearchPolicy, target routing.Point, line [][]float64, loads []domain.LoadOpportunity) []evaluatedLoad {
	release := routing.Point{Latitude: *capacity.Latitude, Longitude: *capacity.Longitude}
	prefiltered := make([]domain.LoadOpportunity, 0, len(loads))
	early := map[uuid.UUID]evaluatedLoad{}
	for _, load := range loads {
		item := evaluatedLoad{load: load, timing: "UNKNOWN", compatibility: compat.StatusIndeterminate}
		if load.Pickup.Latitude == nil || load.Pickup.Longitude == nil {
			item.reasons = append(item.reasons, domain.ReasonRoadDistanceUnknown)
			early[load.ID] = item
			continue
		}
		if policy.SearchMode == domain.SearchRadius {
			straight := routing.StraightLineKm(release.Latitude, release.Longitude, *load.Pickup.Latitude, *load.Pickup.Longitude)
			if policy.RadiusKm != nil && straight > *policy.RadiusKm {
				item.reasons = append(item.reasons, domain.ReasonOutsideRadius)
				early[load.ID] = item
				continue
			}
		}
		if policy.SearchMode == domain.SearchDirectionalCorridor {
			if len(line) < 2 {
				item.reasons = append(item.reasons, domain.ReasonRoadDistanceUnknown)
				early[load.ID] = item
				continue
			}
			projection, err := domain.ProjectPointOnRoute(*load.Pickup.Latitude, *load.Pickup.Longitude, line)
			if err != nil {
				item.reasons = append(item.reasons, domain.ReasonRoadDistanceUnknown)
				early[load.ID] = item
				continue
			}
			forward := projection.ForwardProgressKm
			lateral := projection.LateralDistanceKm
			item.forward = &forward
			item.lateral = &lateral
			limitForward := valueOr(policy.ForwardSearchKm, 0)
			limitLateral := valueOr(policy.CorridorDeviationKm, 0)
			if err := domain.CheckCorridorPlacement(domain.CorridorPlacement{ForwardProgressKm: forward, LateralKm: lateral, RoadDeadheadKm: 0}, limitForward, limitLateral, 1e18); err != nil {
				item.reasons = append(item.reasons, err.Error())
				early[load.ID] = item
				continue
			}
		}
		prefiltered = append(prefiltered, load)
		early[load.ID] = item
	}
	roads := s.roadFacts(ctx, release, target, availableAt, policy, prefiltered)
	out := make([]evaluatedLoad, 0, len(loads))
	for _, load := range loads {
		item := early[load.ID]
		if len(item.reasons) > 0 && !contains(prefiltered, load.ID) {
			item.compatibility, item.fingerprint = s.compatibility(ctx, tenant, capacity, prediction, load)
			out = append(out, item)
			continue
		}
		facts := roads[load.ID]
		item.roadKm = facts.deadheadKm
		item.roadMinutes = facts.deadheadMinutes
		if facts.deadheadKm == nil {
			item.reasons = appendReason(item.reasons, domain.ReasonRoadDistanceUnknown)
		} else {
			if policy.MaxDeadheadKm != nil && *facts.deadheadKm > *policy.MaxDeadheadKm {
				item.reasons = append(item.reasons, domain.ReasonDeadheadExceeded)
			}
			if policy.MaxDeadheadMinutes != nil && facts.deadheadMinutes != nil && *facts.deadheadMinutes > *policy.MaxDeadheadMinutes {
				item.reasons = append(item.reasons, domain.ReasonDeadheadTimeExceeded)
			}
		}
		if policy.SearchMode == domain.SearchDirectionalCorridor {
			if facts.pickupToTarget == nil || facts.deliveryToTarget == nil {
				item.reasons = appendReason(item.reasons, domain.ReasonRoadDistanceUnknown)
			} else if *facts.deliveryToTarget >= *facts.pickupToTarget {
				item.reasons = append(item.reasons, domain.ReasonDeliveryNotTowardTarget)
			}
		}
		if policy.SearchMode == domain.SearchRouteEllipse {
			if facts.deadheadKm == nil || facts.pickupToDelivery == nil || facts.deliveryToTarget == nil || facts.baseline == nil {
				item.reasons = appendReason(item.reasons, domain.ReasonRoadDistanceUnknown)
			} else if policy.MaxRouteIncreaseKm != nil {
				increase := domain.NextLoadInsertionIncreaseKm(*facts.baseline, *facts.deadheadKm, *facts.pickupToDelivery, *facts.deliveryToTarget)
				item.increase = &increase
				if err := domain.CheckRouteIncrease(increase, *policy.MaxRouteIncreaseKm); err != nil {
					item.reasons = append(item.reasons, err.Error())
				}
			}
		}
		if facts.deadheadMinutes != nil {
			arrival := domain.ArrivalAtPickup(availableAt, *facts.deadheadMinutes)
			if load.PickupWindow.Start == nil && load.PickupWindow.End == nil {
				item.timing = "UNKNOWN"
			} else {
				waiting, err := domain.EvaluatePickupWindow(arrival, load.PickupWindow)
				if err != nil {
					item.reasons = append(item.reasons, domain.ReasonPickupWindowMissed)
					item.timing = "MISSED"
				} else {
					item.timing = "FEASIBLE"
					if waiting > 0 {
						minutes := waiting.Minutes()
						item.waiting = &minutes
					}
				}
			}
		}
		item.reasons = append(item.reasons, physicalReasons(equipmentFromCapacity(capacity, prediction), load)...)
		item.compatibility, item.fingerprint = s.compatibility(ctx, tenant, capacity, prediction, load)
		if item.compatibility == compat.StatusIncompatible {
			item.reasons = append(item.reasons, domain.ReasonCargoIncompatible)
		}
		if item.compatibility == compat.StatusIndeterminate {
			item.reasons = append(item.reasons, domain.ReasonCargoIndeterminate)
		}
		out = append(out, item)
	}
	return out
}

type roadFact struct {
	deadheadKm       *float64
	deadheadMinutes  *float64
	pickupToTarget   *float64
	deliveryToTarget *float64
	pickupToDelivery *float64
	baseline         *float64
}

func (s *Service) roadFacts(ctx context.Context, release, target routing.Point, availableAt time.Time, policy domain.NextLoadSearchPolicy, loads []domain.LoadOpportunity) map[uuid.UUID]roadFact {
	facts := map[uuid.UUID]roadFact{}
	if s.routes == nil || len(loads) == 0 {
		return facts
	}
	destinations := make([]routing.Point, 0, len(loads))
	for _, load := range loads {
		destinations = append(destinations, routing.Point{Latitude: *load.Pickup.Latitude, Longitude: *load.Pickup.Longitude})
	}
	matrix, err := routing.BatchMatrix(ctx, s.routes, routing.MatrixRequest{
		Origins: []routing.Point{release}, Destinations: destinations, DepartureAt: &availableAt,
		RouteMode: routing.RouteFastest, TrafficMode: routing.TrafficStatistical,
	}, routing.SyncMatrixLimit)
	bnometrics.MatrixBatches(batchCount(len(destinations)))
	if err != nil {
		bnometrics.RoutingError()
		return facts
	}
	for _, cell := range matrix.Cells {
		if cell.OriginIndex != 0 || cell.DestinationIndex < 0 || cell.DestinationIndex >= len(loads) || cell.Err != nil {
			continue
		}
		km := float64(cell.DistanceM) / 1000
		minutes := float64(cell.DurationSeconds) / 60
		loadID := loads[cell.DestinationIndex].ID
		fact := facts[loadID]
		fact.deadheadKm = &km
		fact.deadheadMinutes = &minutes
		facts[loadID] = fact
	}
	if policy.SearchMode == domain.SearchRadius {
		return facts
	}
	baseline, err := s.routes.Route(ctx, routing.RouteRequest{
		Origin: release, Destination: target, DepartureAt: &availableAt,
		RouteMode: routing.RouteFastest, TrafficMode: routing.TrafficStatistical,
	})
	if err != nil {
		bnometrics.RoutingError()
		return facts
	}
	baseKm := float64(baseline.DistanceM) / 1000
	for id, fact := range facts {
		fact.baseline = &baseKm
		facts[id] = fact
	}
	s.fillDirection(ctx, loads, target, availableAt, facts)
	if policy.SearchMode == domain.SearchRouteEllipse {
		s.fillLoadedLeg(ctx, loads, availableAt, facts)
	}
	return facts
}

func (s *Service) fillDirection(ctx context.Context, loads []domain.LoadOpportunity, target routing.Point, availableAt time.Time, facts map[uuid.UUID]roadFact) {
	points := make([]routing.Point, 0, len(loads)*2)
	index := map[int]struct {
		id       uuid.UUID
		delivery bool
	}{}
	for _, load := range loads {
		if load.Pickup.Latitude != nil {
			index[len(points)] = struct {
				id       uuid.UUID
				delivery bool
			}{id: load.ID}
			points = append(points, routing.Point{Latitude: *load.Pickup.Latitude, Longitude: *load.Pickup.Longitude})
		}
		if load.Delivery.Latitude != nil {
			index[len(points)] = struct {
				id       uuid.UUID
				delivery bool
			}{id: load.ID, delivery: true}
			points = append(points, routing.Point{Latitude: *load.Delivery.Latitude, Longitude: *load.Delivery.Longitude})
		}
	}
	if len(points) == 0 {
		return
	}
	for start := 0; start < len(points); start += routing.SyncMatrixLimit {
		end := start + routing.SyncMatrixLimit
		if end > len(points) {
			end = len(points)
		}
		matrix, err := routing.BatchMatrix(ctx, s.routes, routing.MatrixRequest{
			Origins: points[start:end], Destinations: []routing.Point{target}, DepartureAt: &availableAt,
			RouteMode: routing.RouteFastest, TrafficMode: routing.TrafficStatistical,
		}, routing.SyncMatrixLimit)
		if err != nil {
			bnometrics.RoutingError()
			return
		}
		for _, cell := range matrix.Cells {
			if cell.Err != nil {
				continue
			}
			ref, ok := index[start+cell.OriginIndex]
			if !ok {
				continue
			}
			km := float64(cell.DistanceM) / 1000
			fact := facts[ref.id]
			if ref.delivery {
				fact.deliveryToTarget = &km
			} else {
				fact.pickupToTarget = &km
			}
			facts[ref.id] = fact
		}
	}
}

func (s *Service) fillLoadedLeg(ctx context.Context, loads []domain.LoadOpportunity, availableAt time.Time, facts map[uuid.UUID]roadFact) {
	for start := 0; start < len(loads); start += routing.SyncMatrixLimit {
		end := start + routing.SyncMatrixLimit
		if end > len(loads) {
			end = len(loads)
		}
		chunk := loads[start:end]
		type pair struct {
			id   uuid.UUID
			from routing.Point
			to   routing.Point
		}
		pairs := make([]pair, 0, len(chunk))
		for _, load := range chunk {
			if load.Pickup.Latitude == nil || load.Delivery.Latitude == nil {
				continue
			}
			pairs = append(pairs, pair{
				id:   load.ID,
				from: routing.Point{Latitude: *load.Pickup.Latitude, Longitude: *load.Pickup.Longitude},
				to:   routing.Point{Latitude: *load.Delivery.Latitude, Longitude: *load.Delivery.Longitude},
			})
		}
		if len(pairs) == 0 {
			continue
		}
		origins := make([]routing.Point, 0, len(pairs))
		destinations := make([]routing.Point, 0, len(pairs))
		for _, item := range pairs {
			origins = append(origins, item.from)
			destinations = append(destinations, item.to)
		}
		matrix, err := routing.BatchMatrix(ctx, s.routes, routing.MatrixRequest{
			Origins: origins, Destinations: destinations, DepartureAt: &availableAt,
			RouteMode: routing.RouteFastest, TrafficMode: routing.TrafficStatistical,
		}, routing.SyncMatrixLimit)
		if err != nil {
			bnometrics.RoutingError()
			return
		}
		for _, cell := range matrix.Cells {
			if cell.Err != nil || cell.OriginIndex != cell.DestinationIndex || cell.OriginIndex >= len(pairs) {
				continue
			}
			km := float64(cell.DistanceM) / 1000
			fact := facts[pairs[cell.OriginIndex].id]
			fact.pickupToDelivery = &km
			facts[pairs[cell.OriginIndex].id] = fact
		}
	}
}

func (s *Service) compatibility(ctx context.Context, tenant uuid.UUID, capacity domain.Capacity, prediction *domain.PredictedCapacity, load domain.LoadOpportunity) (string, string) {
	var evalCtx compat.Context
	if s.catalog != nil {
		loaded, err := s.catalog.Evaluation(ctx, tenant)
		if err != nil {
			return compat.StatusIndeterminate, ""
		}
		evalCtx = loaded
	}
	result := compat.EvaluateCargoEquipment(cargoFromLoad(load), equipmentFromCapacity(capacity, prediction), accessFromLoad(load), evalCtx)
	return result.Status, result.Fingerprint
}

func routingProviderName(provider routing.Provider) string {
	if provider == nil {
		return "UNCONFIGURED"
	}
	named, ok := provider.(routing.IdentifiedProvider)
	if !ok || named.ProviderName() == "" {
		return "UNSPECIFIED"
	}
	return named.ProviderName()
}

func (s *Service) persistSearch(ctx context.Context, id, tenant uuid.UUID, capacity domain.Capacity, policy domain.NextLoadSearchPolicy, provider string, started, completed time.Time, rows []evaluatedLoad) error {
	if s.searches == nil {
		return nil
	}
	stored := make([]repository.StoredCandidate, 0, len(rows))
	for _, row := range rows {
		eligibility := "ELIGIBLE"
		if len(row.reasons) > 0 {
			eligibility = "REJECTED"
		}
		stored = append(stored, repository.StoredCandidate{
			ID: uuid.New(), SearchRunID: id, TenantID: tenant, CapacityID: capacity.ID,
			LoadOpportunityID: row.load.ID, LoadVersion: row.load.Version, Eligibility: eligibility,
			RejectReasons: append([]string(nil), row.reasons...), RoadDeadheadKm: row.roadKm,
			RoadDeadheadMinutes: row.roadMinutes, WaitingMinutes: row.waiting,
			CompatibilityStatus: row.compatibility, CompatibilityFingerprint: row.fingerprint,
			PolicyFingerprint: policy.Fingerprint(), CreatedAt: completed,
		})
	}
	return s.searches.SaveSearch(ctx, repository.SearchRun{
		ID: id, TenantID: tenant, CapacityID: capacity.ID, CapacityVersion: capacity.Version,
		EffectivePolicyFingerprint: policy.Fingerprint(), StartedAt: started, CompletedAt: completed,
		RoutingProvider: provider, Status: "COMPLETED",
	}, stored)
}

func (s *Service) publicSearch(id uuid.UUID, capacity domain.Capacity, policy domain.NextLoadSearchPolicy, rows []evaluatedLoad, limit *int) SearchResponse {
	counts := map[string]int{}
	eligible := make([]SearchCandidateView, 0)
	for _, row := range rows {
		if len(row.reasons) > 0 {
			for _, reason := range row.reasons {
				counts[reason]++
			}
			continue
		}
		view := SearchCandidateView{
			LoadOpportunityID: row.load.ID, LoadVersion: row.load.Version, Eligibility: "ELIGIBLE",
			Load: row.load.MarketplaceView(), Timing: row.timing, WaitingMinutes: row.waiting,
			Compatibility: row.compatibility, ForwardProgressKm: row.forward, LateralDistanceKm: row.lateral,
			RouteIncreaseKm: row.increase, Explanation: []string{"ELIGIBLE"},
		}
		if row.roadKm != nil {
			if row.load.VisibilityScope == domain.VisAnonymized {
				view.DeadheadBucket = domain.DeadheadBucket(*row.roadKm)
				view.ForwardProgressKm = nil
				view.LateralDistanceKm = nil
				view.RouteIncreaseKm = nil
			} else {
				view.RoadDeadheadKm = row.roadKm
				view.RoadDeadheadMinutes = row.roadMinutes
			}
		}
		eligible = append(eligible, view)
	}
	sort.Slice(eligible, func(i, j int) bool {
		return eligible[i].LoadOpportunityID.String() < eligible[j].LoadOpportunityID.String()
	})
	totalEligible := len(eligible)
	if limit != nil && *limit >= 0 && *limit < len(eligible) {
		eligible = eligible[:*limit]
	}
	if counts == nil {
		counts = map[string]int{}
	}
	return SearchResponse{
		SearchID: id, CapacityID: capacity.ID, CapacityVersion: capacity.Version, SearchMode: policy.SearchMode,
		EffectivePolicy: policy, PolicyFingerprint: policy.Fingerprint(), EligibleCandidateCount: totalEligible,
		RejectionCountsByReason: counts, Candidates: eligible,
	}
}

func physicalReasons(equipment compat.Equipment, load domain.LoadOpportunity) []string {
	var reasons []string
	if load.WeightKg != nil {
		if equipment.PayloadKg == nil {
			reasons = append(reasons, domain.ReasonCapacityFactUnknown)
		} else if *load.WeightKg > *equipment.PayloadKg {
			reasons = append(reasons, domain.ReasonPayloadExceeded)
		}
	}
	if load.VolumeM3 != nil {
		if equipment.VolumeM3 == nil {
			reasons = append(reasons, domain.ReasonCapacityFactUnknown)
		} else if *load.VolumeM3 > *equipment.VolumeM3 {
			reasons = append(reasons, domain.ReasonVolumeExceeded)
		}
	}
	return reasons
}

func appendReason(reasons []string, reason string) []string {
	for _, item := range reasons {
		if item == reason {
			return reasons
		}
	}
	return append(reasons, reason)
}

func valueOr(value *float64, fallback float64) float64 {
	if value == nil {
		return fallback
	}
	return *value
}

func contains(loads []domain.LoadOpportunity, id uuid.UUID) bool {
	for _, load := range loads {
		if load.ID == id {
			return true
		}
	}
	return false
}

func firstEquipment(values []string) *string {
	if len(values) == 0 || values[0] == "" {
		return nil
	}
	value := values[0]
	return &value
}

func batchCount(n int) int {
	if n == 0 {
		return 0
	}
	count := n / routing.SyncMatrixLimit
	if n%routing.SyncMatrixLimit != 0 {
		count++
	}
	return count
}
