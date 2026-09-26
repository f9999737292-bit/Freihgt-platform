package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/compat"
	"github.com/freight-platform/network-optimizer-service/internal/domain"
	apperrors "github.com/freight-platform/network-optimizer-service/internal/platform/errors"
	bnometrics "github.com/freight-platform/network-optimizer-service/internal/platform/metrics"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
)

const (
	PatternSameOriginDestination = "SAME_ORIGIN_SAME_DESTINATION"
	PatternCurrentTripFill       = "CURRENT_TRIP_FILL"

	ConsolidationFeasible      = "FEASIBLE"
	ConsolidationHardReject    = "HARD_REJECT"
	ConsolidationIndeterminate = "INDETERMINATE"
	PlacementNotEvaluated      = "NOT_EVALUATED"

	ReasonOriginUnproven          = "ORIGIN_IDENTITY_UNPROVEN"
	ReasonDestinationUnproven     = "DESTINATION_IDENTITY_UNPROVEN"
	ReasonSameOwnerOptInAbsent    = "SAME_OWNER_OPT_IN_ABSENT"
	ReasonCrossShipperOptInAbsent = "CROSS_SHIPPER_OPT_IN_ABSENT"
	ReasonPickupDisjoint          = "PICKUP_WINDOWS_DISJOINT"
	ReasonDeliveryDisjoint        = "DELIVERY_WINDOWS_DISJOINT"
	ReasonPickupUnknown           = "PICKUP_WINDOW_UNKNOWN"
	ReasonDeliveryUnknown         = "DELIVERY_WINDOW_UNKNOWN"
	ReasonMultiPartyUnavailable   = "MULTI_PARTY_REFERENCE_CONTEXT_UNAVAILABLE"
)

type ConsolidationCommand struct {
	CapacityID     uuid.UUID
	Pattern        string
	CandidateLimit *int
}

type ConsolidationMemberView struct {
	Ordinal           int                    `json:"ordinal"`
	LoadOpportunityID uuid.UUID              `json:"load_opportunity_id"`
	LoadVersion       int                    `json:"load_version"`
	Load              domain.MarketplaceLoad `json:"load"`
}

type WindowOverlap struct {
	Start *time.Time `json:"start,omitempty"`
	End   *time.Time `json:"end,omitempty"`
}

type ConsolidationCandidateView struct {
	CandidateID              uuid.UUID                 `json:"candidate_id"`
	Status                   string                    `json:"status"`
	ExecutionSupported       bool                      `json:"execution_supported"`
	PlacementCheck           string                    `json:"placement_check"`
	Members                  []ConsolidationMemberView `json:"members"`
	PickupWindowOverlap      *WindowOverlap            `json:"pickup_window_overlap,omitempty"`
	DeliveryWindowOverlap    *WindowOverlap            `json:"delivery_window_overlap,omitempty"`
	Compatibility            string                    `json:"compatibility"`
	CapacityUsage            *compat.Usage             `json:"capacity_usage,omitempty"`
	Conditions               []string                  `json:"conditions,omitempty"`
	IndeterminateReasonCodes []string                  `json:"indeterminate_reason_codes,omitempty"`
	Explanation              []string                  `json:"explanation,omitempty"`
}

type ConsolidationResponse struct {
	SearchID                    uuid.UUID                    `json:"search_id"`
	CapacityID                  uuid.UUID                    `json:"capacity_id"`
	CapacityVersion             int                          `json:"capacity_version"`
	Pattern                     string                       `json:"pattern"`
	PoolLoadCount               int                          `json:"pool_load_count"`
	EvaluatedPairCount          int                          `json:"evaluated_pair_count"`
	FeasibleCandidateCount      int                          `json:"feasible_candidate_count"`
	IndeterminateCandidateCount int                          `json:"indeterminate_candidate_count"`
	HardRejectCandidateCount    int                          `json:"hard_reject_candidate_count"`
	ReturnedCandidateCount      int                          `json:"returned_candidate_count"`
	ExcludedCountsByReason      map[string]int               `json:"excluded_counts_by_reason"`
	HardRejectCountsByReason    map[string]int               `json:"hard_reject_counts_by_reason"`
	IndeterminateCountsByReason map[string]int               `json:"indeterminate_counts_by_reason"`
	Candidates                  []ConsolidationCandidateView `json:"candidates"`
}

func (s *Service) SearchConsolidation(ctx context.Context, actor Actor, cmd ConsolidationCommand) (Result, error) {
	started := s.now()
	if cmd.CapacityID == uuid.Nil {
		return Result{}, apperrors.Validation("capacity_id is required", map[string]any{"field": "capacity_id"})
	}
	if cmd.Pattern == "" {
		return Result{}, apperrors.Validation("pattern is required", map[string]any{"field": "pattern"})
	}
	if cmd.Pattern == PatternCurrentTripFill {
		return Result{}, apperrors.Validation("pattern is not implemented", map[string]any{"reason": "PATTERN_NOT_IMPLEMENTED"})
	}
	if cmd.Pattern != PatternSameOriginDestination {
		return Result{}, apperrors.Validation("pattern is not implemented", map[string]any{"reason": "PATTERN_NOT_IMPLEMENTED"})
	}
	if cmd.CandidateLimit != nil && *cmd.CandidateLimit < 0 {
		return Result{}, apperrors.Validation("candidate_limit must be zero or greater", map[string]any{"field": "candidate_limit"})
	}
	if s.consolidations == nil {
		return Result{}, apperrors.ServiceUnavailable("consolidation persistence is unavailable")
	}
	var capacity domain.Capacity
	var pool []domain.LoadOpportunity
	err := s.store.Within(ctx, func(tx repository.Tx) error {
		row, err := tx.GetCapacity(ctx, cmd.CapacityID)
		if err != nil {
			return err
		}
		capacity = row
		rows, err := tx.ListMarketplaceLoads(ctx, actor.TenantID, actor.CompanyID, 100000, 0)
		if err != nil {
			return err
		}
		pool = rows
		return nil
	})
	if err != nil || capacity.OwnerTenantID != actor.TenantID {
		bnometrics.API("consolidation_search", "not_found")
		return Result{}, apperrors.NotFound("capacity is not available")
	}
	if capacity.Status != domain.CapacityAvailable {
		return Result{}, apperrors.Validation("capacity is not available", map[string]any{"reason": "CAPACITY_NOT_AVAILABLE"})
	}
	_, prediction, err := s.effectiveAvailableAt(ctx, actor.TenantID, capacity)
	if err != nil {
		return Result{}, err
	}
	equipment := equipmentFromCapacity(capacity, prediction)
	excluded := map[string]int{}
	grouped := map[string][]domain.LoadOpportunity{}
	for _, load := range pool {
		if load.Pickup.LocationID == nil {
			excluded[ReasonOriginUnproven]++
		}
		if load.Delivery.LocationID == nil {
			excluded[ReasonDestinationUnproven]++
		}
		if load.Pickup.LocationID == nil || load.Delivery.LocationID == nil {
			continue
		}
		key := load.Pickup.LocationID.String() + "|" + load.Delivery.LocationID.String()
		grouped[key] = append(grouped[key], load)
	}
	var assessed []assessedPair
	for _, members := range grouped {
		sort.Slice(members, func(i, j int) bool { return members[i].ID.String() < members[j].ID.String() })
		for i := 0; i < len(members); i++ {
			for j := i + 1; j < len(members); j++ {
				left, right := members[i], members[j]
				if !pairOptedIn(left, right) {
					if left.OwnerTenantID == right.OwnerTenantID {
						excluded[ReasonSameOwnerOptInAbsent]++
					} else {
						excluded[ReasonCrossShipperOptInAbsent]++
					}
					continue
				}
				assessed = append(assessed, s.assessPair(ctx, actor.TenantID, equipment, left, right))
			}
		}
	}
	sort.Slice(assessed, func(i, j int) bool {
		if assessed[i].statusRank() != assessed[j].statusRank() {
			return assessed[i].statusRank() < assessed[j].statusRank()
		}
		if assessed[i].left.ID.String() != assessed[j].left.ID.String() {
			return assessed[i].left.ID.String() < assessed[j].left.ID.String()
		}
		return assessed[i].right.ID.String() < assessed[j].right.ID.String()
	})
	response := ConsolidationResponse{
		SearchID: uuid.New(), CapacityID: capacity.ID, CapacityVersion: capacity.Version,
		Pattern: PatternSameOriginDestination, PoolLoadCount: len(pool), EvaluatedPairCount: len(assessed),
		ExcludedCountsByReason: nonzero(excluded), HardRejectCountsByReason: map[string]int{},
		IndeterminateCountsByReason: map[string]int{}, Candidates: []ConsolidationCandidateView{},
	}
	now := s.now()
	stored := make([]repository.ConsolidationCandidate, 0, len(assessed))
	returnable := make([]ConsolidationCandidateView, 0)
	for _, pair := range assessed {
		switch pair.status {
		case ConsolidationFeasible:
			response.FeasibleCandidateCount++
		case ConsolidationIndeterminate:
			response.IndeterminateCandidateCount++
			for _, code := range pair.indeterminate {
				response.IndeterminateCountsByReason[code]++
			}
		default:
			response.HardRejectCandidateCount++
			for _, code := range pair.hard {
				response.HardRejectCountsByReason[code]++
			}
		}
		row := pair.stored(response.SearchID, actor.TenantID, capacity, now)
		view := pair.view()
		view.CandidateID = row.ID
		if pair.status != ConsolidationHardReject {
			returnable = append(returnable, view)
		}
		stored = append(stored, row)
	}
	limit := len(returnable)
	if cmd.CandidateLimit != nil && *cmd.CandidateLimit < limit {
		limit = *cmd.CandidateLimit
	}
	response.Candidates = append([]ConsolidationCandidateView(nil), returnable[:limit]...)
	response.ReturnedCandidateCount = len(response.Candidates)
	if len(response.HardRejectCountsByReason) == 0 {
		response.HardRejectCountsByReason = map[string]int{}
	}
	if len(response.IndeterminateCountsByReason) == 0 {
		response.IndeterminateCountsByReason = map[string]int{}
	}
	run := repository.ConsolidationRun{
		ID: response.SearchID, TenantID: actor.TenantID, CapacityID: capacity.ID, CapacityVersion: capacity.Version,
		Pattern: PatternSameOriginDestination, StartedAt: started, CompletedAt: now, Status: "COMPLETED",
		CandidateLimit: cmd.CandidateLimit, PoolLoadCount: response.PoolLoadCount, EvaluatedPairCount: response.EvaluatedPairCount,
		CreatedAt: now,
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
	slog.Info("consolidation search",
		slog.String("request_id", actor.RequestID),
		slog.String("search_id", response.SearchID.String()),
		slog.String("pattern", response.Pattern),
		slog.Int("pair_count", response.EvaluatedPairCount),
		slog.Int("feasible", response.FeasibleCandidateCount),
		slog.Int("indeterminate", response.IndeterminateCandidateCount),
		slog.Int("hard_reject", response.HardRejectCandidateCount),
	)
	return Result{Status: http.StatusOK, Body: raw, AggregateID: response.SearchID}, nil
}

func pairOptedIn(left, right domain.LoadOpportunity) bool {
	if left.OwnerTenantID == right.OwnerTenantID {
		return left.ConsolidationAllowed && right.ConsolidationAllowed
	}
	return left.CrossShipperConsolidationAllowed && right.CrossShipperConsolidationAllowed
}

type assessedPair struct {
	left, right      domain.LoadOpportunity
	status           string
	compatibility    string
	hard             []string
	indeterminate    []string
	conditions       []string
	explanation      []string
	usage            *compat.Usage
	fingerprints     []string
	ruleFingerprints []string
	pickup           *WindowOverlap
	delivery         *WindowOverlap
	trace            []byte
	crossShipper     bool
}

func (p assessedPair) statusRank() int {
	switch p.status {
	case ConsolidationFeasible:
		return 0
	case ConsolidationIndeterminate:
		return 1
	default:
		return 2
	}
}

func (s *Service) assessPair(ctx context.Context, actor uuid.UUID, equipment compat.Equipment, left, right domain.LoadOpportunity) assessedPair {
	out := assessedPair{left: left, right: right, compatibility: compat.StatusIndeterminate, crossShipper: left.OwnerTenantID != right.OwnerTenantID}
	pickup, pickupCode := overlap(left.PickupWindow, right.PickupWindow, ReasonPickupUnknown, ReasonPickupDisjoint)
	delivery, deliveryCode := overlap(left.DeliveryWindow, right.DeliveryWindow, ReasonDeliveryUnknown, ReasonDeliveryDisjoint)
	if pickupCode == "ok" {
		out.pickup = pickup
	}
	if deliveryCode == "ok" {
		out.delivery = delivery
	}
	if pickupCode == ReasonPickupDisjoint || deliveryCode == ReasonDeliveryDisjoint {
		out.status = ConsolidationHardReject
		out.compatibility = compat.StatusIncompatible
		if pickupCode == ReasonPickupDisjoint {
			out.hard = append(out.hard, ReasonPickupDisjoint)
		}
		if deliveryCode == ReasonDeliveryDisjoint {
			out.hard = append(out.hard, ReasonDeliveryDisjoint)
		}
		out.explanation = append([]string(nil), out.hard...)
		out.fingerprints = []string{"NOT_EVALUATED"}
		return out
	}
	tenants := []uuid.UUID{actor}
	if left.OwnerTenantID != actor {
		tenants = append(tenants, left.OwnerTenantID)
	}
	if right.OwnerTenantID != actor && right.OwnerTenantID != left.OwnerTenantID {
		tenants = append(tenants, right.OwnerTenantID)
	}
	contexts, err := s.contextsFor(ctx, tenants)
	if err != nil {
		out.status = ConsolidationIndeterminate
		out.indeterminate = []string{ReasonMultiPartyUnavailable}
		if pickupCode == ReasonPickupUnknown {
			out.indeterminate = append(out.indeterminate, ReasonPickupUnknown)
		}
		if deliveryCode == ReasonDeliveryUnknown {
			out.indeterminate = append(out.indeterminate, ReasonDeliveryUnknown)
		}
		out.explanation = append([]string(nil), out.indeterminate...)
		out.fingerprints = []string{"NOT_EVALUATED"}
		return out
	}
	items := []compat.GroupageItem{
		{Cargo: cargoFromLoad(left), AccessNeed: accessFromLoad(left)},
		{Cargo: cargoFromLoad(right), AccessNeed: accessFromLoad(right)},
	}
	var results []compat.Result
	for _, evalCtx := range contexts {
		results = append(results, compat.EvaluateGroupageItems(equipment, items, evalCtx))
	}
	out = mergeAssessments(out, results)
	if pickupCode == ReasonPickupUnknown {
		out.indeterminate = append(out.indeterminate, ReasonPickupUnknown)
	}
	if deliveryCode == ReasonDeliveryUnknown {
		out.indeterminate = append(out.indeterminate, ReasonDeliveryUnknown)
	}
	if out.crossShipper && s.catalog == nil && out.status == ConsolidationFeasible {
		out.status = ConsolidationIndeterminate
		out.indeterminate = append(out.indeterminate, ReasonMultiPartyUnavailable)
	}
	if len(out.hard) > 0 {
		out.status = ConsolidationHardReject
	} else if len(out.indeterminate) > 0 {
		out.status = ConsolidationIndeterminate
	}
	out.explanation = append(append([]string{}, out.hard...), out.indeterminate...)
	out.trace = publicTrace(results)
	return out
}

func (s *Service) contextsFor(ctx context.Context, tenants []uuid.UUID) ([]compat.Context, error) {
	if s.catalog == nil {
		return []compat.Context{{}}, nil
	}
	out := make([]compat.Context, 0, len(tenants))
	seen := map[uuid.UUID]struct{}{}
	for _, tenant := range tenants {
		if _, ok := seen[tenant]; ok {
			continue
		}
		seen[tenant] = struct{}{}
		loaded, err := s.catalog.Evaluation(ctx, tenant)
		if err != nil {
			return nil, err
		}
		out = append(out, loaded)
	}
	return out, nil
}

func mergeAssessments(base assessedPair, results []compat.Result) assessedPair {
	base.status = ConsolidationFeasible
	base.compatibility = compat.StatusCompatible
	for _, result := range results {
		base.fingerprints = append(base.fingerprints, result.Fingerprint)
		for _, ref := range result.RuleSetsUsed {
			base.ruleFingerprints = append(base.ruleFingerprints, ref.ID+":"+ref.Scope)
		}
		switch result.Status {
		case compat.StatusIncompatible:
			base.status = ConsolidationHardReject
			base.compatibility = compat.StatusIncompatible
		case compat.StatusIndeterminate:
			if base.status != ConsolidationHardReject {
				base.status = ConsolidationIndeterminate
				base.compatibility = compat.StatusIndeterminate
			}
		}
		for _, reason := range result.HardRejects {
			base.hard = appendUnique(base.hard, reason.ReasonCode)
		}
		for _, reason := range result.IndeterminateReasons {
			base.indeterminate = appendUnique(base.indeterminate, reason.ReasonCode)
		}
		for _, reason := range result.Conditions {
			base.conditions = appendUnique(base.conditions, reason.ReasonCode)
		}
		if base.usage == nil {
			base.usage = result.CapacityUsage
		}
	}
	return base
}

func overlap(left, right domain.TimeWindow, unknown, disjoint string) (*WindowOverlap, string) {
	if left.Start == nil || left.End == nil || right.Start == nil || right.End == nil {
		return nil, unknown
	}
	start := *left.Start
	if right.Start.After(start) {
		start = *right.Start
	}
	end := *left.End
	if right.End.Before(end) {
		end = *right.End
	}
	if !end.After(start) {
		return nil, disjoint
	}
	return &WindowOverlap{Start: &start, End: &end}, "ok"
}

func (p assessedPair) view() ConsolidationCandidateView {
	view := ConsolidationCandidateView{
		Status: p.status, ExecutionSupported: false, PlacementCheck: PlacementNotEvaluated,
		Compatibility: p.compatibility, CapacityUsage: p.usage, Conditions: p.conditions,
		IndeterminateReasonCodes: p.indeterminate, Explanation: p.explanation,
		PickupWindowOverlap: p.pickup, DeliveryWindowOverlap: p.delivery,
		Members: []ConsolidationMemberView{
			{Ordinal: 1, LoadOpportunityID: p.left.ID, LoadVersion: p.left.Version, Load: p.left.MarketplaceView()},
			{Ordinal: 2, LoadOpportunityID: p.right.ID, LoadVersion: p.right.Version, Load: p.right.MarketplaceView()},
		},
	}
	return view
}

func (p assessedPair) stored(searchID, tenant uuid.UUID, capacity domain.Capacity, now time.Time) repository.ConsolidationCandidate {
	id := uuid.New()
	conditions, _ := json.Marshal(p.conditions)
	warnings, _ := json.Marshal([]string{})
	var usage []byte
	if p.usage != nil {
		usage, _ = json.Marshal(p.usage)
	}
	trace := p.trace
	if len(trace) == 0 {
		trace = []byte("{}")
	}
	row := repository.ConsolidationCandidate{
		ID: id, SearchRunID: searchID, TenantID: tenant, CapacityID: capacity.ID,
		Pattern: PatternSameOriginDestination, Status: p.status, ExecutionSupported: false,
		CompatibilityStatus: p.compatibility, CompatibilityFingerprint: joinPrints(p.fingerprints),
		CandidateFingerprint: p.fingerprint(capacity), PlacementCheck: PlacementNotEvaluated,
		HardRejectReasons: emptyStrings(p.hard), IndeterminateReasonCodes: emptyStrings(p.indeterminate),
		Conditions: conditions, Warnings: warnings, CapacityUsage: usage, CompatibilityTrace: trace, CreatedAt: now,
		Members: []repository.ConsolidationMember{
			{Ordinal: 1, LoadOpportunityID: p.left.ID, LoadVersion: p.left.Version, LoadOwnerTenantID: p.left.OwnerTenantID, CreatedAt: now},
			{Ordinal: 2, LoadOpportunityID: p.right.ID, LoadVersion: p.right.Version, LoadOwnerTenantID: p.right.OwnerTenantID, CreatedAt: now},
		},
	}
	if p.pickup != nil {
		row.PickupOverlapStart = p.pickup.Start
		row.PickupOverlapEnd = p.pickup.End
	}
	if p.delivery != nil {
		row.DeliveryOverlapStart = p.delivery.Start
		row.DeliveryOverlapEnd = p.delivery.End
	}
	return row
}

func (p assessedPair) fingerprint(capacity domain.Capacity) string {
	body := struct {
		Pattern         string   `json:"pattern"`
		CapacityID      string   `json:"capacity_id"`
		CapacityVersion int      `json:"capacity_version"`
		Members         []string `json:"members"`
		Versions        []int    `json:"versions"`
		OptIn           []bool   `json:"opt_in"`
		Pickup          string   `json:"pickup"`
		Delivery        string   `json:"delivery"`
		Windows         []string `json:"windows"`
		Compatibility   []string `json:"compatibility"`
		Rules           []string `json:"rules"`
	}{
		Pattern: PatternSameOriginDestination, CapacityID: capacity.ID.String(), CapacityVersion: capacity.Version,
		Members:  []string{p.left.ID.String(), p.right.ID.String()},
		Versions: []int{p.left.Version, p.right.Version},
		OptIn:    []bool{p.left.ConsolidationAllowed, p.left.CrossShipperConsolidationAllowed, p.right.ConsolidationAllowed, p.right.CrossShipperConsolidationAllowed},
		Pickup:   p.left.Pickup.LocationID.String(), Delivery: p.left.Delivery.LocationID.String(),
		Windows:       []string{stamp(p.left.PickupWindow), stamp(p.right.PickupWindow), stamp(p.left.DeliveryWindow), stamp(p.right.DeliveryWindow)},
		Compatibility: append([]string(nil), p.fingerprints...), Rules: append([]string(nil), p.ruleFingerprints...),
	}
	sort.Strings(body.Rules)
	raw, _ := json.Marshal(body)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func stamp(window domain.TimeWindow) string {
	left, right := "", ""
	if window.Start != nil {
		left = window.Start.UTC().Format(time.RFC3339Nano)
	}
	if window.End != nil {
		right = window.End.UTC().Format(time.RFC3339Nano)
	}
	return left + "/" + right
}

func publicTrace(results []compat.Result) []byte {
	type ruleSet struct {
		ID      string `json:"id,omitempty"`
		Scope   string `json:"scope,omitempty"`
		Version int    `json:"version,omitempty"`
	}
	type catalogRef struct {
		CatalogKind string `json:"catalog_kind,omitempty"`
		Scope       string `json:"scope,omitempty"`
		Version     int    `json:"version,omitempty"`
	}
	type reason struct {
		ReasonCode string `json:"reason_code"`
		Dimension  string `json:"dimension,omitempty"`
		Scope      string `json:"rule_set_scope,omitempty"`
	}
	strip := func(items []compat.Reason) []reason {
		out := make([]reason, 0, len(items))
		for _, item := range items {
			row := reason{ReasonCode: item.ReasonCode, Dimension: item.Dimension}
			if item.RuleSetScope != nil {
				row.Scope = *item.RuleSetScope
			}
			out = append(out, row)
		}
		return out
	}
	docs := make([]map[string]any, 0, len(results))
	for _, result := range results {
		sets := make([]ruleSet, 0, len(result.RuleSetsUsed))
		for _, ref := range result.RuleSetsUsed {
			sets = append(sets, ruleSet{ID: ref.ID, Scope: ref.Scope, Version: ref.Version})
		}
		catalogs := make([]catalogRef, 0, len(result.CatalogVersionsUsed))
		for _, ref := range result.CatalogVersionsUsed {
			catalogs = append(catalogs, catalogRef{CatalogKind: ref.CatalogKind, Scope: ref.Scope, Version: ref.Version})
		}
		docs = append(docs, map[string]any{
			"status": result.Status, "fingerprint": result.Fingerprint,
			"hard_rejects": strip(result.HardRejects), "indeterminate_reasons": strip(result.IndeterminateReasons),
			"conditions": strip(result.Conditions), "rule_sets_used": sets, "catalog_versions_used": catalogs,
		})
	}
	raw, _ := json.Marshal(docs)
	return raw
}

func joinPrints(values []string) string {
	if len(values) == 0 {
		return "NOT_EVALUATED"
	}
	out := values[0]
	for _, value := range values[1:] {
		out += "|" + value
	}
	return out
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func emptyStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func nonzero(counts map[string]int) map[string]int {
	out := map[string]int{}
	for key, value := range counts {
		if value > 0 {
			out[key] = value
		}
	}
	return out
}
