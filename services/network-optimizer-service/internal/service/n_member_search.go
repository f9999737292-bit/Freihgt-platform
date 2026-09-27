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
	nMemberAlgorithmPolicyVersion = "nlo-0.3e-lex-v1"
	nMemberBudgetPolicyVersion    = "nlo-0.3e-budget-v1"

	ReasonPoolLimitExceeded    = "POOL_LIMIT_EXCEEDED"
	ReasonSearchBudgetExceeded = "SEARCH_BUDGET_EXCEEDED"
)

type nMemberPolicy struct {
	configured             bool
	CandidatePool          int
	MaxSetSize             int
	MaxSetsEvaluated       int
	MaxGroupageCalls       int
	TimeBudget             time.Duration
	MaxRoutingCalls        int
	AlgorithmPolicyVersion string
	BudgetPolicyVersion    string
}

func defaultNMemberPolicy() nMemberPolicy {
	return nMemberPolicy{
		configured:             true,
		CandidatePool:          10,
		MaxSetSize:             3,
		MaxSetsEvaluated:       165,
		MaxGroupageCalls:       330,
		TimeBudget:             5 * time.Second,
		MaxRoutingCalls:        0,
		AlgorithmPolicyVersion: nMemberAlgorithmPolicyVersion,
		BudgetPolicyVersion:    nMemberBudgetPolicyVersion,
	}
}

func (s *Service) nMemberBounds() nMemberPolicy {
	if !s.nMemberPolicy.configured {
		return defaultNMemberPolicy()
	}
	policy := s.nMemberPolicy
	if policy.AlgorithmPolicyVersion == "" {
		policy.AlgorithmPolicyVersion = nMemberAlgorithmPolicyVersion
	}
	if policy.BudgetPolicyVersion == "" {
		policy.BudgetPolicyVersion = nMemberBudgetPolicyVersion
	}
	if policy.MaxSetSize > 3 {
		policy.MaxSetSize = 3
	}
	return policy
}

// nMemberWorkBudget is created inside one N-member search and is never stored on Service.
type nMemberWorkBudget struct {
	maxGroupage int
	reserved    int
	actual      int
	blocked     bool
}

type nMemberBudgetObservation struct {
	actual   int
	reserved int
}

type nMemberBudgetObservationKey struct{}

func observeNMemberBudget(ctx context.Context) (context.Context, *nMemberBudgetObservation) {
	obs := &nMemberBudgetObservation{}
	return context.WithValue(ctx, nMemberBudgetObservationKey{}, obs), obs
}

func publishNMemberBudget(ctx context.Context, work *nMemberWorkBudget) {
	if ctx == nil || work == nil {
		return
	}
	obs, _ := ctx.Value(nMemberBudgetObservationKey{}).(*nMemberBudgetObservation)
	if obs == nil {
		return
	}
	obs.actual = work.actual
	obs.reserved = work.reserved
}

func noteNMemberGroupage(work *nMemberWorkBudget) bool {
	if work == nil {
		return true
	}
	if work.actual+1 > work.reserved || work.actual+1 > work.maxGroupage {
		work.blocked = true
		return false
	}
	work.actual++
	return true
}

type NMemberConsolidationSearchResponse struct {
	SearchID                    uuid.UUID                    `json:"search_id"`
	CapacityID                  uuid.UUID                    `json:"capacity_id"`
	CapacityVersion             int                          `json:"capacity_version"`
	Pattern                     string                       `json:"pattern"`
	PoolLoadCount               int                          `json:"pool_load_count"`
	EvaluatedSetCount           int                          `json:"evaluated_set_count"`
	FeasibleCandidateCount      int                          `json:"feasible_candidate_count"`
	IndeterminateCandidateCount int                          `json:"indeterminate_candidate_count"`
	HardRejectCandidateCount    int                          `json:"hard_reject_candidate_count"`
	ReturnedCandidateCount      int                          `json:"returned_candidate_count"`
	Candidates                  []ConsolidationCandidateView `json:"candidates"`
}

func (s *Service) searchNMember(ctx context.Context, actor Actor, cmd ConsolidationCommand, started time.Time) (Result, error) {
	policy := s.nMemberBounds()
	work := &nMemberWorkBudget{maxGroupage: policy.MaxGroupageCalls}
	defer publishNMemberBudget(ctx, work)
	if cmd.CapacityID == uuid.Nil {
		return Result{}, apperrors.Validation("capacity_id is required", map[string]any{"field": "capacity_id"})
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
		rows, err := tx.ListPublicConsolidationPoolLimited(ctx, actor.TenantID, actor.CompanyID, policy.CandidatePool+1)
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
	if len(pool) > policy.CandidatePool {
		bnometrics.ConsolidationBudget(ReasonPoolLimitExceeded)
		bnometrics.API("consolidation_search", "pool_limit")
		return Result{}, apperrors.Validation("candidate pool limit exceeded", map[string]any{"reason": ReasonPoolLimitExceeded})
	}
	_, prediction, err := s.effectiveAvailableAt(ctx, actor.TenantID, capacity)
	if err != nil {
		return Result{}, err
	}
	equipment := equipmentFromCapacity(capacity, prediction)
	grouped := map[string][]domain.LoadOpportunity{}
	for _, load := range pool {
		if load.Pickup.LocationID == nil || load.Delivery.LocationID == nil {
			continue
		}
		key := load.Pickup.LocationID.String() + "|" + load.Delivery.LocationID.String()
		grouped[key] = append(grouped[key], load)
	}
	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	maxSize := policy.MaxSetSize
	if maxSize < 2 {
		maxSize = 2
	}
	var assessed []assessedSet
	evaluated := 0
	reservedGroupage := 0
	for _, key := range keys {
		members := grouped[key]
		sort.Slice(members, func(i, j int) bool { return members[i].ID.String() < members[j].ID.String() })
		for size := 2; size <= maxSize && size <= len(members); size++ {
			for _, set := range combinations(members, size) {
				if !sameCanonicalOD(set) {
					continue
				}
				if opted, _ := setOptedIn(set); !opted {
					continue
				}
				planned := plannedNMemberGroupage(s.catalog, capacity.OwnerTenantID, set)
				if budget := structuralBudget(evaluated+1, reservedGroupage+planned, policy); budget != "" {
					return s.nMemberBudgetFailure(budget)
				}
				if !s.now().Before(started.Add(policy.TimeBudget)) {
					return s.nMemberBudgetFailure("time")
				}
				evaluated++
				reservedGroupage += planned
				work.reserved = reservedGroupage
				row := s.assessNMemberSet(ctx, capacity.OwnerTenantID, equipment, set, work)
				if work.blocked || work.actual > work.reserved || work.actual > work.maxGroupage {
					return s.nMemberBudgetFailure("groupage")
				}
				assessed = append(assessed, row)
			}
		}
	}
	sort.Slice(assessed, func(i, j int) bool {
		if assessed[i].statusRank() != assessed[j].statusRank() {
			return assessed[i].statusRank() < assessed[j].statusRank()
		}
		return memberKey(assessed[i].members) < memberKey(assessed[j].members)
	})
	response := NMemberConsolidationSearchResponse{
		SearchID: uuid.New(), CapacityID: capacity.ID, CapacityVersion: capacity.Version,
		Pattern: PatternSameOriginDestinationNMember, PoolLoadCount: len(pool), EvaluatedSetCount: evaluated,
		Candidates: []ConsolidationCandidateView{},
	}
	now := s.now()
	stored := make([]repository.ConsolidationCandidate, 0, len(assessed))
	returnable := make([]ConsolidationCandidateView, 0)
	for _, set := range assessed {
		switch set.status {
		case ConsolidationFeasible:
			response.FeasibleCandidateCount++
		case ConsolidationIndeterminate:
			response.IndeterminateCandidateCount++
		default:
			response.HardRejectCandidateCount++
		}
		row := set.stored(response.SearchID, actor.TenantID, capacity, now, cmd.Policy, policy)
		view := set.view()
		view.CandidateID = row.ID
		if set.status != ConsolidationHardReject {
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
	setCount := evaluated
	run := repository.ConsolidationRun{
		ID: response.SearchID, TenantID: actor.TenantID, CapacityID: &capacity.ID, CapacityVersion: &capacity.Version,
		Pattern: PatternSameOriginDestinationNMember, StartedAt: started, CompletedAt: now, Status: "COMPLETED",
		CandidateLimit: cmd.CandidateLimit, PoolLoadCount: response.PoolLoadCount,
		PairCountNull: true, EvaluatedSetCount: &setCount, CreatedAt: now,
	}
	if err := s.consolidations.SaveConsolidation(ctx, run, stored); err != nil {
		return Result{}, err
	}
	raw, err := json.Marshal(response)
	if err != nil {
		return Result{}, err
	}
	bnometrics.ConsolidationSearch(time.Since(started), response.EvaluatedSetCount, response.FeasibleCandidateCount, response.IndeterminateCandidateCount, response.HardRejectCandidateCount)
	bnometrics.API("consolidation_search", "ok")
	slog.Info("consolidation search",
		slog.String("request_id", actor.RequestID),
		slog.String("search_id", response.SearchID.String()),
		slog.String("pattern", response.Pattern),
		slog.Int("set_count", response.EvaluatedSetCount),
		slog.Int("feasible", response.FeasibleCandidateCount),
		slog.Int("indeterminate", response.IndeterminateCandidateCount),
		slog.Int("hard_reject", response.HardRejectCandidateCount),
	)
	return Result{Status: http.StatusOK, Body: raw, AggregateID: response.SearchID}, nil
}

func (s *Service) nMemberBudgetFailure(kind string) (Result, error) {
	bnometrics.ConsolidationBudget(ReasonSearchBudgetExceeded)
	bnometrics.API("consolidation_search", "search_budget")
	return Result{}, apperrors.Validation("search budget exceeded", map[string]any{
		"reason": ReasonSearchBudgetExceeded,
		"budget": kind,
	})
}

func structuralBudget(nextSets, nextGroupage int, policy nMemberPolicy) string {
	if nextSets > policy.MaxSetsEvaluated {
		return "sets"
	}
	if nextGroupage > policy.MaxGroupageCalls {
		return "groupage"
	}
	return ""
}

func plannedNMemberGroupage(catalog catalogEvaluator, capacityOwner uuid.UUID, members []domain.LoadOpportunity) int {
	if len(members) == 0 || nMemberCrossShipper(members) || catalog == nil || capacityOwner == members[0].OwnerTenantID {
		return 1
	}
	return 2
}

type assessedSet struct {
	members          []domain.LoadOpportunity
	status           string
	compatibility    string
	hard             []string
	indeterminate    []string
	conditions       []string
	explanation      []string
	usage            *compat.Usage
	fingerprints     []string
	ruleFingerprints []string
	ruleVersions     []string
	catalogVersions  []string
	pickup           *WindowOverlap
	delivery         *WindowOverlap
	crossShipper     bool
}

func (set assessedSet) statusRank() int {
	switch set.status {
	case ConsolidationFeasible:
		return 0
	case ConsolidationIndeterminate:
		return 1
	default:
		return 2
	}
}

func (s *Service) assessNMemberSet(ctx context.Context, capacityOwner uuid.UUID, equipment compat.Equipment, members []domain.LoadOpportunity, work *nMemberWorkBudget) assessedSet {
	sorted := append([]domain.LoadOpportunity(nil), members...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID.String() < sorted[j].ID.String() })
	out := assessedSet{members: sorted, compatibility: compat.StatusIndeterminate, crossShipper: nMemberCrossShipper(sorted)}
	pickups := make([]domain.TimeWindow, len(sorted))
	deliveries := make([]domain.TimeWindow, len(sorted))
	for i, member := range sorted {
		pickups[i] = member.PickupWindow
		deliveries[i] = member.DeliveryWindow
	}
	pickup, pickupCode := overlapAll(pickups, ReasonPickupUnknown, ReasonPickupDisjoint)
	delivery, deliveryCode := overlapAll(deliveries, ReasonDeliveryUnknown, ReasonDeliveryDisjoint)
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
	items := make([]compat.GroupageItem, len(sorted))
	for i, member := range sorted {
		items[i] = compat.GroupageItem{Cargo: cargoFromLoad(member), AccessNeed: accessFromLoad(member)}
	}
	shell := assessedPair{compatibility: compat.StatusIndeterminate, crossShipper: out.crossShipper}
	if out.crossShipper {
		shell = assessCrossShipper(shell, equipment, items, pickupCode, deliveryCode, work)
	} else {
		shell = s.assessSameOwner(ctx, shell, equipment, items, pickupCode, deliveryCode, capacityOwner, sorted[0].OwnerTenantID, work)
	}
	out.status = shell.status
	out.compatibility = shell.compatibility
	out.hard = shell.hard
	out.indeterminate = shell.indeterminate
	out.conditions = shell.conditions
	out.explanation = shell.explanation
	out.usage = shell.usage
	out.fingerprints = shell.fingerprints
	out.ruleFingerprints = shell.ruleFingerprints
	out.ruleVersions = shell.ruleVersions
	out.catalogVersions = shell.catalogVersions
	return out
}

func (set assessedSet) view() ConsolidationCandidateView {
	members := make([]ConsolidationMemberView, len(set.members))
	for i, member := range set.members {
		load := member.MarketplaceView()
		load.OwnerTenantID = nil
		members[i] = ConsolidationMemberView{
			Ordinal: i + 1, LoadOpportunityID: member.ID, LoadVersion: member.Version, Load: load,
		}
	}
	return ConsolidationCandidateView{
		Status: set.status, ExecutionSupported: false, PlacementCheck: PlacementNotEvaluated,
		Compatibility: set.compatibility, CapacityUsage: set.usage, Conditions: set.conditions,
		IndeterminateReasonCodes: set.indeterminate, Explanation: set.explanation,
		PickupWindowOverlap: set.pickup, DeliveryWindowOverlap: set.delivery, Members: members,
	}
}

func (set assessedSet) stored(searchID, tenant uuid.UUID, capacity domain.Capacity, now time.Time, requestPolicy string, policy nMemberPolicy) repository.ConsolidationCandidate {
	conditions, _ := json.Marshal(set.conditions)
	warnings, _ := json.Marshal([]string{})
	var usage []byte
	if set.usage != nil {
		usage, _ = json.Marshal(set.usage)
	}
	members := make([]repository.ConsolidationMember, len(set.members))
	for i, member := range set.members {
		members[i] = repository.ConsolidationMember{
			Ordinal: i + 1, LoadOpportunityID: member.ID, LoadVersion: member.Version,
			LoadOwnerTenantID: member.OwnerTenantID, CreatedAt: now,
		}
	}
	row := repository.ConsolidationCandidate{
		ID: uuid.New(), SearchRunID: searchID, TenantID: tenant, CapacityID: capacity.ID,
		Pattern: PatternSameOriginDestinationNMember, Status: set.status, ExecutionSupported: false,
		CompatibilityStatus: set.compatibility, CompatibilityFingerprint: joinPrints(set.fingerprints),
		CandidateFingerprint: set.fingerprint(capacity, requestPolicy, policy), PlacementCheck: PlacementNotEvaluated,
		HardRejectReasons: emptyStrings(set.hard), IndeterminateReasonCodes: emptyStrings(set.indeterminate),
		Conditions: conditions, Warnings: warnings, CapacityUsage: usage, CompatibilityTrace: []byte("{}"), CreatedAt: now,
		Members: members,
	}
	if set.pickup != nil {
		row.PickupOverlapStart = set.pickup.Start
		row.PickupOverlapEnd = set.pickup.End
	}
	if set.delivery != nil {
		row.DeliveryOverlapStart = set.delivery.Start
		row.DeliveryOverlapEnd = set.delivery.End
	}
	return row
}

func (set assessedSet) fingerprint(capacity domain.Capacity, requestPolicy string, policy nMemberPolicy) string {
	raw := set.fingerprintDocument(capacity, requestPolicy, policy)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (set assessedSet) fingerprintDocument(capacity domain.Capacity, requestPolicy string, policy nMemberPolicy) []byte {
	ids := make([]string, len(set.members))
	versions := make([]int, len(set.members))
	cargo := make([]string, len(set.members))
	windows := make([]string, 0, len(set.members)*2)
	for i, member := range set.members {
		ids[i] = member.ID.String()
		versions[i] = member.Version
		encoded, _ := json.Marshal(cargoFromLoad(member))
		cargo[i] = string(encoded)
		windows = append(windows, stamp(member.PickupWindow), stamp(member.DeliveryWindow))
	}
	body := struct {
		Pattern                string   `json:"pattern"`
		CapacityID             string   `json:"capacity_id"`
		CapacityVersion        int      `json:"capacity_version"`
		Members                []string `json:"members"`
		Versions               []int    `json:"versions"`
		CargoProfiles          []string `json:"cargo_profiles"`
		CatalogVersions        []string `json:"catalog_versions"`
		RuleSets               []string `json:"rule_sets"`
		Compatibility          []string `json:"compatibility"`
		PolicyVersion          string   `json:"policy_version"`
		AlgorithmPolicyVersion string   `json:"algorithm_policy_version"`
		BudgetPolicyVersion    string   `json:"budget_policy_version"`
		PickupLocationID       string   `json:"pickup_location_id"`
		DeliveryLocationID     string   `json:"delivery_location_id"`
		Windows                []string `json:"windows"`
	}{
		Pattern: PatternSameOriginDestinationNMember, CapacityID: capacity.ID.String(), CapacityVersion: capacity.Version,
		Members: ids, Versions: versions, CargoProfiles: cargo,
		CatalogVersions: append([]string(nil), set.catalogVersions...),
		RuleSets:        append([]string(nil), set.ruleVersions...),
		Compatibility:   append([]string(nil), set.fingerprints...),
		PolicyVersion:   requestPolicy, AlgorithmPolicyVersion: policy.AlgorithmPolicyVersion, BudgetPolicyVersion: policy.BudgetPolicyVersion,
		PickupLocationID: set.members[0].Pickup.LocationID.String(), DeliveryLocationID: set.members[0].Delivery.LocationID.String(),
		Windows: windows,
	}
	sort.Strings(body.CatalogVersions)
	sort.Strings(body.RuleSets)
	sort.Strings(body.Compatibility)
	raw, _ := json.Marshal(body)
	return raw
}

func nMemberCrossShipper(members []domain.LoadOpportunity) bool {
	for i := 1; i < len(members); i++ {
		if members[i].OwnerTenantID != members[0].OwnerTenantID {
			return true
		}
	}
	return false
}

func setOptedIn(members []domain.LoadOpportunity) (bool, string) {
	if nMemberCrossShipper(members) {
		for _, member := range members {
			if !member.CrossShipperConsolidationAllowed {
				return false, ReasonCrossShipperOptInAbsent
			}
		}
		return true, ""
	}
	for _, member := range members {
		if !member.ConsolidationAllowed {
			return false, ReasonSameOwnerOptInAbsent
		}
	}
	return true, ""
}

func sameCanonicalOD(members []domain.LoadOpportunity) bool {
	if len(members) == 0 || members[0].Pickup.LocationID == nil || members[0].Delivery.LocationID == nil {
		return false
	}
	pickup := members[0].Pickup.LocationID.String()
	delivery := members[0].Delivery.LocationID.String()
	for _, member := range members[1:] {
		if member.Pickup.LocationID == nil || member.Delivery.LocationID == nil {
			return false
		}
		if member.Pickup.LocationID.String() != pickup || member.Delivery.LocationID.String() != delivery {
			return false
		}
	}
	return true
}

func overlapAll(windows []domain.TimeWindow, unknown, disjoint string) (*WindowOverlap, string) {
	known := make([]domain.TimeWindow, 0, len(windows))
	missing := false
	for _, window := range windows {
		if window.Start == nil || window.End == nil {
			missing = true
			continue
		}
		known = append(known, window)
	}
	if len(known) > 0 {
		start := *known[0].Start
		end := *known[0].End
		for _, window := range known[1:] {
			if window.Start.After(start) {
				start = *window.Start
			}
			if window.End.Before(end) {
				end = *window.End
			}
		}
		if !end.After(start) {
			return nil, disjoint
		}
		if missing {
			return nil, unknown
		}
		overlapStart := start
		overlapEnd := end
		return &WindowOverlap{Start: &overlapStart, End: &overlapEnd}, "ok"
	}
	return nil, unknown
}

func combinations(loads []domain.LoadOpportunity, size int) [][]domain.LoadOpportunity {
	if size < 2 || size > len(loads) || size > 3 {
		return nil
	}
	index := make([]int, size)
	for i := range index {
		index[i] = i
	}
	var out [][]domain.LoadOpportunity
	for {
		set := make([]domain.LoadOpportunity, size)
		for i, at := range index {
			set[i] = loads[at]
		}
		out = append(out, set)
		cursor := size - 1
		for cursor >= 0 && index[cursor] == len(loads)-size+cursor {
			cursor--
		}
		if cursor < 0 {
			return out
		}
		index[cursor]++
		for next := cursor + 1; next < size; next++ {
			index[next] = index[next-1] + 1
		}
	}
}

func memberKey(members []domain.LoadOpportunity) string {
	out := ""
	for _, member := range members {
		out += member.ID.String() + "|"
	}
	return out
}
