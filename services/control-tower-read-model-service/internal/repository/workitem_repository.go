package repository

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
)

type WorkItemRepository struct {
	pool         *pgxpool.Pool
	workflowRepo *WorkflowRepository
	riskRepo     *RiskRepository
}

func NewWorkItemRepository(pool *pgxpool.Pool, workflowRepo *WorkflowRepository, riskRepo *RiskRepository) *WorkItemRepository {
	return &WorkItemRepository{pool: pool, workflowRepo: workflowRepo, riskRepo: riskRepo}
}

func (r *WorkItemRepository) ListWorkItems(ctx context.Context, tenantID uuid.UUID, filter domain.WorkItemFilter, currentUserID *uuid.UUID) (domain.WorkItemPage, error) {
	exceptions, err := r.loadExceptionItems(ctx, tenantID, filter)
	if err != nil {
		return domain.WorkItemPage{}, err
	}
	risks, err := r.loadRiskItems(ctx, tenantID, filter, exceptions)
	if err != nil {
		return domain.WorkItemPage{}, err
	}

	items := append(exceptions, risks...)
	items = applyWorkItemFilters(items, filter, currentUserID)
	sortWorkItems(items)

	total := len(items)
	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit < 1 {
		limit = 50
	}
	if limit > domain.WorkItemsMaxPageLimit {
		limit = domain.WorkItemsMaxPageLimit
	}

	start := (page - 1) * limit
	if start >= total {
		return domain.WorkItemPage{Items: []domain.WorkItem{}, Page: page, Limit: limit, Total: total, HasNext: false}, nil
	}
	end := start + limit
	if end > total {
		end = total
	}
	return domain.WorkItemPage{
		Items:   items[start:end],
		Page:    page,
		Limit:   limit,
		Total:   total,
		HasNext: end < total,
	}, nil
}

func (r *WorkItemRepository) loadExceptionItems(ctx context.Context, tenantID uuid.UUID, filter domain.WorkItemFilter) ([]domain.WorkItem, error) {
	statusClause := "status <> 'resolved'"
	if filter.IncludeCompleted {
		statusClause = "1=1"
	} else {
		statusClause = "status IN ('open', 'acknowledged', 'assigned')"
	}

	rows, err := r.pool.Query(ctx, `
		SELECT event_id, shipment_id, event_type, status, priority, exception_category, business_impact,
		       escalation_level, assigned_to_user_id, occurred_at, updated_at,
		       acknowledge_due_at, assignment_due_at, resolution_due_at,
		       ack_sla_breached_at, assign_sla_breached_at, resolve_sla_breached_at,
		       exception_activated_at, acknowledged_at, assigned_at
		FROM control_tower.critical_event_workflow
		WHERE tenant_id = $1 AND `+statusClause, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := time.Now().UTC()
	items := make([]domain.WorkItem, 0)
	for rows.Next() {
		var eventID, eventType, status, priority, category, impact, escalation string
		var shipmentID uuid.UUID
		var assignedTo *uuid.UUID
		var occurredAt, updatedAt, ackDue, assignDue, resolveDue time.Time
		var ackBreach, assignBreach, resolveBreach *time.Time
		var exceptionActivated time.Time
		var acknowledgedAt, assignedAt *time.Time
		if err := rows.Scan(&eventID, &shipmentID, &eventType, &status, &priority, &category, &impact,
			&escalation, &assignedTo, &occurredAt, &updatedAt, &ackDue, &assignDue, &resolveDue,
			&ackBreach, &assignBreach, &resolveBreach, &exceptionActivated, &acknowledgedAt, &assignedAt); err != nil {
			return nil, err
		}

		wf := domain.CriticalEventWorkflow{
			Status: status, Priority: priority, ExceptionCategory: category, BusinessImpact: impact,
			EscalationLevel: escalation, AcknowledgeDueAt: ackDue, AssignmentDueAt: assignDue,
			ResolutionDueAt: resolveDue, AckSLABreachedAt: ackBreach, AssignSLABreachedAt: assignBreach,
			ResolveSLABreachedAt: resolveBreach, AssignedToUserID: assignedTo,
			ExceptionActivatedAt: exceptionActivated, AcknowledgedAt: acknowledgedAt, AssignedAt: assignedAt,
		}
		sla := domain.EvaluateSLA(wf, now)
		slaStatus := sla.Status
		slaPhase := sla.Phase
		var slaDue *time.Time
		switch slaPhase {
		case domain.SLAPhaseAcknowledgement:
			t := ackDue
			slaDue = &t
		case domain.SLAPhaseAssignment:
			t := assignDue
			slaDue = &t
		default:
			t := resolveDue
			slaDue = &t
		}

		priorityCopy := priority
		impactCopy := impact
		categoryCopy := category
		slaStatusCopy := slaStatus
		slaPhaseCopy := slaPhase
		escCopy := escalation

		item := domain.WorkItem{
			ID:       domain.WorkItemKey(domain.WorkItemTypeException, eventID),
			ItemType: domain.WorkItemTypeException, SourceID: eventID, ShipmentID: shipmentID,
			TenantID: tenantID, Title: eventType, Summary: eventType,
			WorkflowStatus: status, Priority: &priorityCopy, BusinessImpact: &impactCopy,
			ExceptionCategory: &categoryCopy, SLAStatus: &slaStatusCopy, SLAPhase: &slaPhaseCopy,
			SLADueAt: slaDue, EscalationLevel: &escCopy, OwnerUserID: assignedTo,
			CreatedAt: occurredAt, UpdatedAt: updatedAt, EventType: &eventType,
			Urgency:          deriveExceptionUrgency(priority, slaStatus, escalation),
			AvailableActions: availableExceptionActions(status),
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *WorkItemRepository) loadRiskItems(ctx context.Context, tenantID uuid.UUID, filter domain.WorkItemFilter, exceptions []domain.WorkItem) ([]domain.WorkItem, error) {
	exceptionIDs := map[string]struct{}{}
	for _, ex := range exceptions {
		exceptionIDs[ex.SourceID] = struct{}{}
	}

	statusClause := "status IN ('active', 'acknowledged', 'mitigating')"
	if filter.IncludeCompleted {
		statusClause = "status IN ('active', 'acknowledged', 'mitigating', 'cleared', 'materialized')"
	}

	rows, err := r.pool.Query(ctx, `
		SELECT risk_key, shipment_id, predicted_exception_type, score, risk_level, status,
		       owner_user_id, first_detected_at, evaluated_at, threatened_deadline_at,
		       actual_event_id, mitigation_code
		FROM control_tower.shipment_risk
		WHERE tenant_id = $1 AND `+statusClause, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.WorkItem, 0)
	for rows.Next() {
		var riskKey, predictedType, level, status string
		var shipmentID uuid.UUID
		var score int
		var ownerID *uuid.UUID
		var firstDetected, evaluatedAt time.Time
		var threatened *time.Time
		var actualEventID, mitigationCode *string
		if err := rows.Scan(&riskKey, &shipmentID, &predictedType, &score, &level, &status,
			&ownerID, &firstDetected, &evaluatedAt, &threatened, &actualEventID, &mitigationCode); err != nil {
			return nil, err
		}

		if status == domain.RiskStatusMaterialized {
			if actualEventID != nil {
				if _, ok := exceptionIDs[*actualEventID]; ok && !filter.IncludeCompleted {
					continue
				}
			}
			if !filter.IncludeCompleted {
				continue
			}
		}
		if status == domain.RiskStatusCleared && !filter.IncludeCompleted {
			continue
		}

		levelCopy := level
		statusCopy := status
		predictedCopy := predictedType
		scoreCopy := score
		item := domain.WorkItem{
			ID:       domain.WorkItemKey(domain.WorkItemTypeRisk, riskKey),
			ItemType: domain.WorkItemTypeRisk, SourceID: riskKey, ShipmentID: shipmentID,
			TenantID: tenantID, Title: predictedType, Summary: predictedType,
			WorkflowStatus: status, RiskLevel: &levelCopy, RiskScore: &scoreCopy,
			RiskStatus: &statusCopy, PredictedType: &predictedCopy,
			OwnerUserID: ownerID, CreatedAt: firstDetected, UpdatedAt: evaluatedAt,
			ThreatenedDeadline: threatened, LinkedEventID: actualEventID,
			Urgency:          deriveRiskUrgency(level, status),
			AvailableActions: availableRiskActions(status),
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func applyWorkItemFilters(items []domain.WorkItem, filter domain.WorkItemFilter, currentUserID *uuid.UUID) []domain.WorkItem {
	out := make([]domain.WorkItem, 0, len(items))
	for _, item := range items {
		if filter.ItemType != "" && item.ItemType != filter.ItemType {
			continue
		}
		if filter.MyWorkOnly && currentUserID != nil {
			if item.OwnerUserID == nil || *item.OwnerUserID != *currentUserID {
				continue
			}
		}
		if filter.UnassignedOnly && item.OwnerUserID != nil {
			continue
		}
		if filter.OwnerUserID != nil {
			if item.OwnerUserID == nil || *item.OwnerUserID != *filter.OwnerUserID {
				continue
			}
		}
		if filter.Priority != "" && (item.Priority == nil || *item.Priority != filter.Priority) {
			continue
		}
		if filter.RiskLevel != "" && (item.RiskLevel == nil || *item.RiskLevel != filter.RiskLevel) {
			continue
		}
		if filter.RiskStatus != "" && (item.RiskStatus == nil || *item.RiskStatus != filter.RiskStatus) {
			continue
		}
		if filter.SLAStatus != "" && (item.SLAStatus == nil || *item.SLAStatus != filter.SLAStatus) {
			continue
		}
		if filter.PredictedType != "" && (item.PredictedType == nil || *item.PredictedType != filter.PredictedType) {
			continue
		}
		if filter.ExceptionCategory != "" && (item.ExceptionCategory == nil || *item.ExceptionCategory != filter.ExceptionCategory) {
			continue
		}
		if filter.EscalationLevel != "" && (item.EscalationLevel == nil || *item.EscalationLevel != filter.EscalationLevel) {
			continue
		}
		if filter.BusinessImpact != "" && (item.BusinessImpact == nil || *item.BusinessImpact != filter.BusinessImpact) {
			continue
		}
		if filter.WorkflowStatus != "" && item.WorkflowStatus != filter.WorkflowStatus {
			continue
		}
		if filter.Search != "" {
			search := strings.ToLower(strings.TrimSpace(filter.Search))
			haystack := strings.ToLower(strings.Join([]string{
				item.SourceID, item.ShipmentID.String(), item.Title, item.Summary,
			}, " "))
			if !strings.Contains(haystack, search) {
				continue
			}
		}
		if !matchesPreset(item, filter.Preset) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func matchesPreset(item domain.WorkItem, preset string) bool {
	switch preset {
	case "my_work", "":
		return true
	case "unassigned":
		return item.OwnerUserID == nil
	case "critical":
		return item.Urgency == domain.UrgencyCritical
	case "sla_breached":
		return item.SLAStatus != nil && *item.SLAStatus == domain.SLAStatusBreached
	case "sla_warning":
		return item.SLAStatus != nil && *item.SLAStatus == domain.SLAStatusWarning
	case "p1_exceptions":
		return item.ItemType == domain.WorkItemTypeException && item.Priority != nil && *item.Priority == domain.PriorityP1
	case "p2_exceptions":
		return item.ItemType == domain.WorkItemTypeException && item.Priority != nil && *item.Priority == domain.PriorityP2
	case "critical_risks":
		return item.ItemType == domain.WorkItemTypeRisk && item.RiskLevel != nil && *item.RiskLevel == domain.RiskLevelCritical
	case "high_risks":
		return item.ItemType == domain.WorkItemTypeRisk && item.RiskLevel != nil && *item.RiskLevel == domain.RiskLevelHigh
	case "mitigating_risks":
		return item.ItemType == domain.WorkItemTypeRisk && item.RiskStatus != nil && *item.RiskStatus == domain.RiskStatusMitigating
	case "all_active":
		return true
	case "completed":
		return item.WorkflowStatus == domain.WorkflowStatusResolved ||
			(item.RiskStatus != nil && (*item.RiskStatus == domain.RiskStatusCleared || *item.RiskStatus == domain.RiskStatusMaterialized))
	default:
		return true
	}
}

func deriveExceptionUrgency(priority, slaStatus, escalation string) string {
	if slaStatus == domain.SLAStatusBreached {
		if priority == domain.PriorityP1 {
			return domain.UrgencyCritical
		}
		return domain.UrgencyHigh
	}
	if priority == domain.PriorityP1 {
		return domain.UrgencyCritical
	}
	if slaStatus == domain.SLAStatusWarning || priority == domain.PriorityP2 {
		return domain.UrgencyHigh
	}
	if escalation == domain.EscalationLevel2 || escalation == domain.EscalationLevel3 {
		return domain.UrgencyHigh
	}
	if priority == domain.PriorityP3 {
		return domain.UrgencyNormal
	}
	return domain.UrgencyLow
}

func deriveRiskUrgency(level, status string) string {
	switch level {
	case domain.RiskLevelCritical:
		return domain.UrgencyCritical
	case domain.RiskLevelHigh:
		return domain.UrgencyHigh
	case domain.RiskLevelMedium:
		return domain.UrgencyNormal
	default:
		return domain.UrgencyLow
	}
}

func urgencyRank(u string) int {
	switch u {
	case domain.UrgencyCritical:
		return 1
	case domain.UrgencyHigh:
		return 2
	case domain.UrgencyNormal:
		return 3
	default:
		return 4
	}
}

func sortWorkItems(items []domain.WorkItem) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		// 1. Exception SLA breach before others
		aBreached := a.ItemType == domain.WorkItemTypeException && a.SLAStatus != nil && *a.SLAStatus == domain.SLAStatusBreached
		bBreached := b.ItemType == domain.WorkItemTypeException && b.SLAStatus != nil && *b.SLAStatus == domain.SLAStatusBreached
		if aBreached != bBreached {
			return aBreached
		}
		// 2. P1 exceptions
		aP1 := a.ItemType == domain.WorkItemTypeException && a.Priority != nil && *a.Priority == domain.PriorityP1
		bP1 := b.ItemType == domain.WorkItemTypeException && b.Priority != nil && *b.Priority == domain.PriorityP1
		if aP1 != bP1 {
			return aP1
		}
		// 3. SLA warning
		aWarn := a.SLAStatus != nil && *a.SLAStatus == domain.SLAStatusWarning
		bWarn := b.SLAStatus != nil && *b.SLAStatus == domain.SLAStatusWarning
		if aWarn != bWarn {
			return aWarn
		}
		// 4. Critical risk
		aCritRisk := a.ItemType == domain.WorkItemTypeRisk && a.RiskLevel != nil && *a.RiskLevel == domain.RiskLevelCritical
		bCritRisk := b.ItemType == domain.WorkItemTypeRisk && b.RiskLevel != nil && *b.RiskLevel == domain.RiskLevelCritical
		if aCritRisk != bCritRisk {
			return aCritRisk
		}
		// 5. P2 exceptions
		aP2 := a.ItemType == domain.WorkItemTypeException && a.Priority != nil && *a.Priority == domain.PriorityP2
		bP2 := b.ItemType == domain.WorkItemTypeException && b.Priority != nil && *b.Priority == domain.PriorityP2
		if aP2 != bP2 {
			return aP2
		}
		// 6. High risk
		aHighRisk := a.ItemType == domain.WorkItemTypeRisk && a.RiskLevel != nil && *a.RiskLevel == domain.RiskLevelHigh
		bHighRisk := b.ItemType == domain.WorkItemTypeRisk && b.RiskLevel != nil && *b.RiskLevel == domain.RiskLevelHigh
		if aHighRisk != bHighRisk {
			return aHighRisk
		}
		// urgency rank
		if ur := urgencyRank(a.Urgency) - urgencyRank(b.Urgency); ur != 0 {
			return ur < 0
		}
		// nearest deadline
		aDeadline := deadlineUnix(a.SLADueAt, a.ThreatenedDeadline)
		bDeadline := deadlineUnix(b.SLADueAt, b.ThreatenedDeadline)
		if aDeadline != bDeadline {
			return aDeadline < bDeadline
		}
		// risk score desc
		aScore, bScore := riskScoreOf(a), riskScoreOf(b)
		if aScore != bScore {
			return aScore > bScore
		}
		// oldest waiting
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.Before(b.CreatedAt)
		}
		return a.ID < b.ID
	})
}

func deadlineUnix(slaDue, threatened *time.Time) int64 {
	if slaDue != nil {
		return slaDue.UTC().Unix()
	}
	if threatened != nil {
		return threatened.UTC().Unix()
	}
	return 1 << 62
}

func riskScoreOf(item domain.WorkItem) int {
	if item.RiskScore != nil {
		return *item.RiskScore
	}
	return 0
}

func availableExceptionActions(status string) []string {
	switch status {
	case domain.WorkflowStatusOpen:
		return []string{"acknowledge", "claim", "assign"}
	case domain.WorkflowStatusAcknowledged:
		return []string{"claim", "assign", "resolve"}
	case domain.WorkflowStatusAssigned:
		return []string{"reassign", "unassign", "resolve"}
	default:
		return nil
	}
}

func availableRiskActions(status string) []string {
	switch status {
	case domain.RiskStatusActive:
		return []string{"acknowledge", "claim", "assign", "mitigate"}
	case domain.RiskStatusAcknowledged:
		return []string{"claim", "assign", "mitigate"}
	case domain.RiskStatusMitigating:
		return []string{"reassign", "unassign", "mitigate"}
	default:
		return nil
	}
}

func (r *WorkItemRepository) GetWorkItem(ctx context.Context, tenantID uuid.UUID, itemType, sourceID string) (domain.WorkItem, error) {
	page, err := r.ListWorkItems(ctx, tenantID, domain.WorkItemFilter{
		ItemType: itemType, IncludeCompleted: true, Limit: domain.WorkItemsMaxPageLimit, Page: 1,
	}, nil)
	if err != nil {
		return domain.WorkItem{}, err
	}
	for _, item := range page.Items {
		if item.ItemType == itemType && item.SourceID == sourceID {
			return item, nil
		}
	}
	return domain.WorkItem{}, fmt.Errorf("work item not found")
}

func (r *WorkItemRepository) ExecuteBulkAction(
	ctx context.Context,
	tenantID, actorUserID uuid.UUID,
	action string,
	items []domain.BulkActionItem,
	targetUserID *uuid.UUID,
) domain.BulkActionOutcome {
	outcome := domain.BulkActionOutcome{
		Requested: len(items),
		Results:   make([]domain.BulkActionResult, 0, len(items)),
	}
	for _, item := range items {
		result := domain.BulkActionResult{ItemType: item.ItemType, ItemID: item.ItemID}
		var err error
		switch action {
		case "claim":
			err = r.claimItem(ctx, tenantID, actorUserID, item)
		case "assign":
			if targetUserID == nil || *targetUserID == uuid.Nil {
				msg := "targetUserId is required"
				result.Error = &msg
				outcome.Failed++
				outcome.Results = append(outcome.Results, result)
				continue
			}
			err = r.assignItem(ctx, tenantID, actorUserID, *targetUserID, item)
		case "unassign":
			err = r.unassignItem(ctx, tenantID, actorUserID, item)
		case "acknowledge":
			err = r.acknowledgeItem(ctx, tenantID, actorUserID, item)
		default:
			msg := "unsupported action"
			result.Error = &msg
			outcome.Failed++
			outcome.Results = append(outcome.Results, result)
			continue
		}
		if err != nil {
			msg := err.Error()
			result.Error = &msg
			outcome.Failed++
		} else {
			result.Success = true
			outcome.Succeeded++
		}
		outcome.Results = append(outcome.Results, result)
	}
	return outcome
}

func (r *WorkItemRepository) claimItem(ctx context.Context, tenantID, actorUserID uuid.UUID, item domain.BulkActionItem) error {
	switch item.ItemType {
	case domain.WorkItemTypeException:
		_, err := r.workflowRepo.ClaimCriticalEvent(ctx, tenantID, actorUserID, item.ItemID)
		return err
	case domain.WorkItemTypeRisk:
		_, err := r.riskRepo.ClaimRiskOwner(ctx, ClaimRiskOwnerInput{TenantID: tenantID, ActorUserID: actorUserID, RiskKey: item.ItemID})
		return err
	default:
		return fmt.Errorf("unsupported item type")
	}
}

func (r *WorkItemRepository) assignItem(ctx context.Context, tenantID, actorUserID, targetUserID uuid.UUID, item domain.BulkActionItem) error {
	switch item.ItemType {
	case domain.WorkItemTypeException:
		_, err := r.workflowRepo.AssignCriticalEvent(ctx, domain.AssignCriticalEventInput{
			TenantID: tenantID, ActorUserID: actorUserID, EventID: item.ItemID, AssignedToUser: targetUserID,
		})
		return err
	case domain.WorkItemTypeRisk:
		_, err := r.riskRepo.AssignRiskOwner(ctx, AssignRiskOwnerInput{
			TenantID: tenantID, ActorUserID: actorUserID, RiskKey: item.ItemID, OwnerUserID: targetUserID,
		})
		return err
	default:
		return fmt.Errorf("unsupported item type")
	}
}

func (r *WorkItemRepository) unassignItem(ctx context.Context, tenantID, actorUserID uuid.UUID, item domain.BulkActionItem) error {
	switch item.ItemType {
	case domain.WorkItemTypeException:
		_, err := r.workflowRepo.UnassignCriticalEvent(ctx, tenantID, actorUserID, item.ItemID)
		return err
	case domain.WorkItemTypeRisk:
		_, err := r.riskRepo.UnassignRiskOwner(ctx, tenantID, actorUserID, item.ItemID)
		return err
	default:
		return fmt.Errorf("unsupported item type")
	}
}

func (r *WorkItemRepository) acknowledgeItem(ctx context.Context, tenantID, actorUserID uuid.UUID, item domain.BulkActionItem) error {
	switch item.ItemType {
	case domain.WorkItemTypeException:
		return fmt.Errorf("exception acknowledge requires event context")
	case domain.WorkItemTypeRisk:
		_, err := r.riskRepo.AcknowledgeRisk(ctx, domain.AcknowledgeRiskInput{
			TenantID: tenantID, ActorUserID: actorUserID, RiskKey: item.ItemID,
		})
		return err
	default:
		return fmt.Errorf("unsupported item type")
	}
}

func (r *WorkItemRepository) GetWorkItemTimeline(ctx context.Context, tenantID uuid.UUID, itemType, sourceID string) ([]domain.WorkItemTimelineEntry, error) {
	entries := make([]domain.WorkItemTimelineEntry, 0)
	if itemType == domain.WorkItemTypeException {
		rows, err := r.pool.Query(ctx, `
			SELECT action_type, actor_user_id, occurred_at, metadata
			FROM control_tower.critical_event_action WHERE tenant_id = $1 AND event_id = $2
			ORDER BY occurred_at ASC`, tenantID, sourceID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var e domain.WorkItemTimelineEntry
			var meta []byte
			e.Source = "workflow"
			if err := rows.Scan(&e.ActionType, &e.ActorUserID, &e.OccurredAt, &meta); err != nil {
				return nil, err
			}
			entries = append(entries, e)
		}
	} else {
		var riskID uuid.UUID
		err := r.pool.QueryRow(ctx, `SELECT id FROM control_tower.shipment_risk WHERE tenant_id = $1 AND risk_key = $2`, tenantID, sourceID).Scan(&riskID)
		if err != nil {
			return nil, fmt.Errorf("risk not found")
		}
		rows, err := r.pool.Query(ctx, `
			SELECT action_type, actor_user_id, occurred_at, metadata
			FROM control_tower.shipment_risk_action WHERE tenant_id = $1 AND shipment_risk_id = $2
			ORDER BY occurred_at ASC`, tenantID, riskID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var e domain.WorkItemTimelineEntry
			var actor *uuid.UUID
			var meta []byte
			e.Source = "risk"
			if err := rows.Scan(&e.ActionType, &actor, &e.OccurredAt, &meta); err != nil {
				return nil, err
			}
			e.ActorUserID = actor
			entries = append(entries, e)
		}
	}
	return entries, nil
}

func (r *WorkItemRepository) CountWorkload(ctx context.Context, tenantID uuid.UUID) ([]domain.WorkloadSummary, int, error) {
	page, err := r.ListWorkItems(ctx, tenantID, domain.WorkItemFilter{Preset: "all_active", Limit: 10000, Page: 1}, nil)
	if err != nil {
		return nil, 0, err
	}
	byUser := map[uuid.UUID]*domain.WorkloadSummary{}
	unassigned := 0
	for _, item := range page.Items {
		if item.OwnerUserID == nil {
			unassigned++
			continue
		}
		uid := *item.OwnerUserID
		s, ok := byUser[uid]
		if !ok {
			s = &domain.WorkloadSummary{UserID: uid}
			byUser[uid] = s
		}
		s.ActiveWork++
		if item.ItemType == domain.WorkItemTypeException {
			if item.WorkflowStatus == domain.WorkflowStatusOpen {
				s.Unacknowledged++
			}
			if item.Priority != nil && *item.Priority == domain.PriorityP1 {
				s.P1++
			}
			if item.Priority != nil && *item.Priority == domain.PriorityP2 {
				s.P2++
			}
			if item.SLAStatus != nil && *item.SLAStatus == domain.SLAStatusBreached {
				s.SLABreached++
			}
			if item.SLAStatus != nil && *item.SLAStatus == domain.SLAStatusWarning {
				s.SLAWarning++
			}
		}
		if item.ItemType == domain.WorkItemTypeRisk {
			if item.RiskLevel != nil && *item.RiskLevel == domain.RiskLevelCritical {
				s.CriticalRisks++
			}
			if item.RiskLevel != nil && *item.RiskLevel == domain.RiskLevelHigh {
				s.HighRisks++
			}
		}
	}
	out := make([]domain.WorkloadSummary, 0, len(byUser))
	for _, s := range byUser {
		out = append(out, *s)
	}
	return out, unassigned, nil
}

func (r *WorkItemRepository) CountWorkspaceKPI(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID) (domain.WorkspaceKPI, error) {
	page, err := r.ListWorkItems(ctx, tenantID, domain.WorkItemFilter{Preset: "all_active", Limit: 10000, Page: 1}, nil)
	if err != nil {
		return domain.WorkspaceKPI{}, err
	}
	kpi := domain.WorkspaceKPI{}
	for _, item := range page.Items {
		if item.OwnerUserID == nil {
			kpi.UnassignedWork++
		} else if *item.OwnerUserID == userID {
			kpi.MyActiveWork++
			if item.Urgency == domain.UrgencyCritical {
				kpi.MyCriticalWork++
			}
		}
		kpi.TeamActiveWork++
		if item.SLAStatus != nil && *item.SLAStatus == domain.SLAStatusBreached {
			kpi.SLABreachedWork++
		}
		if item.SLAStatus != nil && *item.SLAStatus == domain.SLAStatusWarning {
			kpi.SLAWarningWork++
		}
		if item.ItemType == domain.WorkItemTypeRisk && item.RiskLevel != nil &&
			(*item.RiskLevel == domain.RiskLevelCritical || *item.RiskLevel == domain.RiskLevelHigh) {
			kpi.CriticalRiskWork++
		}
	}
	return kpi, nil
}
