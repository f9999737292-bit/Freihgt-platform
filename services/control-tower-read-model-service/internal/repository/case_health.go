package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	apperrors "github.com/freight-platform/control-tower-read-model-service/internal/platform/errors"
)

type caseLinkRef struct {
	caseID     uuid.UUID
	entityType string
	entityID   string
}

type exceptionRow struct {
	eventID            string
	status             string
	priority           string
	businessImpact     string
	escalationLevel    string
	ackDue             time.Time
	assignDue          time.Time
	resolveDue         time.Time
	ackBreach          *time.Time
	assignBreach       *time.Time
	resolveBreach      *time.Time
	exceptionActivated time.Time
	acknowledgedAt     *time.Time
	assignedAt         *time.Time
	resolvedAt         *time.Time
}

type riskRow struct {
	riskKey       string
	riskLevel     string
	status        string
	actualEventID *string
}

func (r *CaseRepository) GetCaseHealth(ctx context.Context, tenantID, caseID uuid.UUID) (domain.CaseHealth, error) {
	batch, err := r.BatchCaseHealth(ctx, tenantID, []uuid.UUID{caseID})
	if err != nil {
		return domain.CaseHealth{}, err
	}
	if h, ok := batch[caseID]; ok {
		return h, nil
	}
	return domain.CaseHealth{}, nil
}

func (r *CaseRepository) BatchCaseHealth(ctx context.Context, tenantID uuid.UUID, caseIDs []uuid.UUID) (map[uuid.UUID]domain.CaseHealth, error) {
	out := make(map[uuid.UUID]domain.CaseHealth, len(caseIDs))
	if len(caseIDs) == 0 {
		return out, nil
	}
	for _, id := range caseIDs {
		out[id] = domain.CaseHealth{}
	}

	actionStats, err := r.loadCaseActionStats(ctx, tenantID, caseIDs)
	if err != nil {
		return nil, err
	}
	for caseID, stats := range actionStats {
		h := out[caseID]
		h.OpenActionCount = stats.open
		h.OverdueActionCount = stats.overdue
		h.NearestActionDueAt = stats.nearestDue
		out[caseID] = h
	}

	links, err := r.loadCaseWorkLinks(ctx, tenantID, caseIDs)
	if err != nil {
		return nil, err
	}
	if len(links) == 0 {
		return out, nil
	}

	exceptionIDs := make([]string, 0)
	riskIDs := make([]string, 0)
	linksByCase := map[uuid.UUID][]caseLinkRef{}
	for _, link := range links {
		linksByCase[link.caseID] = append(linksByCase[link.caseID], link)
		switch link.entityType {
		case domain.CaseLinkException:
			exceptionIDs = append(exceptionIDs, link.entityID)
		case domain.CaseLinkRisk:
			riskIDs = append(riskIDs, link.entityID)
		}
	}

	exceptions, err := r.loadExceptionRows(ctx, tenantID, exceptionIDs)
	if err != nil {
		return nil, err
	}
	risks, err := r.loadRiskRows(ctx, tenantID, riskIDs)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	activeExceptionsByCase := make(map[uuid.UUID]map[string]struct{}, len(linksByCase))
	for caseID, caseLinks := range linksByCase {
		active := make(map[string]struct{})
		for _, link := range caseLinks {
			if link.entityType != domain.CaseLinkException {
				continue
			}
			if ex, ok := exceptions[link.entityID]; ok && ex.status != domain.WorkflowStatusResolved {
				active[link.entityID] = struct{}{}
			}
		}
		activeExceptionsByCase[caseID] = active
	}

	for caseID, caseLinks := range linksByCase {
		h := out[caseID]
		activeExceptions := activeExceptionsByCase[caseID]
		for _, link := range caseLinks {
			switch link.entityType {
			case domain.CaseLinkException:
				if ex, ok := exceptions[link.entityID]; ok {
					if ex.status == domain.WorkflowStatusResolved {
						continue
					}
					h.ActiveWorkItemCount++
					h.ActiveExceptionCount++
					applyExceptionSignals(&h, ex, now)
				}
			case domain.CaseLinkRisk:
				if rk, ok := risks[link.entityID]; ok {
					if rk.status == domain.RiskStatusCleared {
						continue
					}
					if rk.status == domain.RiskStatusMaterialized {
						if rk.actualEventID != nil {
							if _, linked := activeExceptions[*rk.actualEventID]; linked {
								continue
							}
						}
						continue
					}
					h.ActiveWorkItemCount++
					h.ActiveRiskCount++
					applyRiskSignals(&h, rk)
				}
			}
		}
		out[caseID] = h
	}
	return out, nil
}

type caseActionStats struct {
	open       int
	overdue    int
	nearestDue *time.Time
}

func (r *CaseRepository) loadCaseActionStats(ctx context.Context, tenantID uuid.UUID, caseIDs []uuid.UUID) (map[uuid.UUID]caseActionStats, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT case_id,
		       COUNT(*) FILTER (WHERE status IN ('open','in_progress')) AS open_count,
		       COUNT(*) FILTER (WHERE status IN ('open','in_progress') AND due_at IS NOT NULL AND due_at < NOW()) AS overdue_count,
		       MIN(due_at) FILTER (WHERE status IN ('open','in_progress') AND due_at IS NOT NULL) AS nearest_due
		FROM control_tower.operational_case_action_item
		WHERE tenant_id = $1 AND case_id = ANY($2)
		GROUP BY case_id
	`, tenantID, caseIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[uuid.UUID]caseActionStats)
	for rows.Next() {
		var caseID uuid.UUID
		var stats caseActionStats
		var nearest *time.Time
		if err := rows.Scan(&caseID, &stats.open, &stats.overdue, &nearest); err != nil {
			return nil, err
		}
		if nearest != nil {
			t := nearest.UTC()
			stats.nearestDue = &t
		}
		out[caseID] = stats
	}
	return out, rows.Err()
}

func (r *CaseRepository) loadCaseWorkLinks(ctx context.Context, tenantID uuid.UUID, caseIDs []uuid.UUID) ([]caseLinkRef, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT case_id, entity_type, entity_id
		FROM control_tower.operational_case_link
		WHERE tenant_id = $1 AND case_id = ANY($2) AND entity_type IN ('exception','risk')
	`, tenantID, caseIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]caseLinkRef, 0)
	for rows.Next() {
		var link caseLinkRef
		if err := rows.Scan(&link.caseID, &link.entityType, &link.entityID); err != nil {
			return nil, err
		}
		out = append(out, link)
	}
	return out, rows.Err()
}

func (r *CaseRepository) loadExceptionRows(ctx context.Context, tenantID uuid.UUID, eventIDs []string) (map[string]exceptionRow, error) {
	out := make(map[string]exceptionRow)
	if len(eventIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT event_id, status, priority, business_impact, escalation_level,
		       acknowledge_due_at, assignment_due_at, resolution_due_at,
		       ack_sla_breached_at, assign_sla_breached_at, resolve_sla_breached_at,
		       exception_activated_at, acknowledged_at, assigned_at, resolved_at
		FROM control_tower.critical_event_workflow
		WHERE tenant_id = $1 AND event_id = ANY($2)
	`, tenantID, eventIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var ex exceptionRow
		if err := rows.Scan(&ex.eventID, &ex.status, &ex.priority, &ex.businessImpact, &ex.escalationLevel,
			&ex.ackDue, &ex.assignDue, &ex.resolveDue, &ex.ackBreach, &ex.assignBreach, &ex.resolveBreach,
			&ex.exceptionActivated, &ex.acknowledgedAt, &ex.assignedAt, &ex.resolvedAt); err != nil {
			return nil, err
		}
		out[ex.eventID] = ex
	}
	return out, rows.Err()
}

func (r *CaseRepository) loadRiskRows(ctx context.Context, tenantID uuid.UUID, riskKeys []string) (map[string]riskRow, error) {
	out := make(map[string]riskRow)
	if len(riskKeys) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT risk_key, risk_level, status, actual_event_id
		FROM control_tower.shipment_risk
		WHERE tenant_id = $1 AND risk_key = ANY($2)
	`, tenantID, riskKeys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var rk riskRow
		if err := rows.Scan(&rk.riskKey, &rk.riskLevel, &rk.status, &rk.actualEventID); err != nil {
			return nil, err
		}
		out[rk.riskKey] = rk
	}
	return out, rows.Err()
}

func applyExceptionSignals(h *domain.CaseHealth, ex exceptionRow, now time.Time) {
	wf := domain.CriticalEventWorkflow{
		Status:               ex.status,
		Priority:             ex.priority,
		BusinessImpact:       ex.businessImpact,
		EscalationLevel:      ex.escalationLevel,
		AcknowledgeDueAt:     ex.ackDue,
		AssignmentDueAt:      ex.assignDue,
		ResolutionDueAt:      ex.resolveDue,
		AckSLABreachedAt:     ex.ackBreach,
		AssignSLABreachedAt:  ex.assignBreach,
		ResolveSLABreachedAt: ex.resolveBreach,
		ExceptionActivatedAt: ex.exceptionActivated,
		AcknowledgedAt:       ex.acknowledgedAt,
		AssignedAt:           ex.assignedAt,
		ResolvedAt:           ex.resolvedAt,
	}
	sla := domain.EvaluateSLA(wf, now)
	switch sla.Status {
	case domain.SLAStatusBreached:
		h.HasSLABreach = true
	case domain.SLAStatusWarning:
		h.HasSLAWarning = true
	}
	if sla.RemainingSeconds != nil && *sla.RemainingSeconds > 0 {
		due := now.Add(time.Duration(*sla.RemainingSeconds) * time.Second)
		trackNearestSLA(h, &due)
	} else if sla.Status == domain.SLAStatusBreached {
		trackNearestSLA(h, &now)
	}
	h.HighestExceptionPriority = higherPriority(h.HighestExceptionPriority, &ex.priority)
}

func applyRiskSignals(h *domain.CaseHealth, rk riskRow) {
	h.HighestRiskLevel = higherRiskLevel(h.HighestRiskLevel, &rk.riskLevel)
}

func trackNearestSLA(h *domain.CaseHealth, due *time.Time) {
	if due == nil {
		return
	}
	if h.NearestSLADueAt == nil || due.Before(*h.NearestSLADueAt) {
		t := due.UTC()
		h.NearestSLADueAt = &t
	}
}

func higherPriority(current *string, candidate *string) *string {
	if candidate == nil {
		return current
	}
	if current == nil || domain.PriorityRank(*candidate) < domain.PriorityRank(*current) {
		c := *candidate
		return &c
	}
	return current
}

func higherRiskLevel(current *string, candidate *string) *string {
	if candidate == nil {
		return current
	}
	rank := func(level string) int {
		switch level {
		case "critical":
			return 1
		case "high":
			return 2
		case "medium":
			return 3
		case "low":
			return 4
		default:
			return 5
		}
	}
	if current == nil || rank(*candidate) < rank(*current) {
		c := *candidate
		return &c
	}
	return current
}

func buildSeverityInput(h domain.CaseHealth, exceptions map[string]exceptionRow, risks map[string]riskRow, links []caseLinkRef) domain.CaseSeverityInput {
	in := domain.CaseSeverityInput{Health: h}
	activeExceptions := make(map[string]struct{})
	for _, link := range links {
		if link.entityType != domain.CaseLinkException {
			continue
		}
		if ex, ok := exceptions[link.entityID]; ok && ex.status != domain.WorkflowStatusResolved {
			activeExceptions[link.entityID] = struct{}{}
		}
	}
	for _, link := range links {
		switch link.entityType {
		case domain.CaseLinkException:
			if ex, ok := exceptions[link.entityID]; ok && ex.status != domain.WorkflowStatusResolved {
				if ex.priority == domain.PriorityP1 {
					in.HasP1Exception = true
				}
				if ex.priority == domain.PriorityP2 {
					in.HasP2Exception = true
				}
				if ex.businessImpact == "critical" {
					in.HasCriticalImpact = true
				}
			}
		case domain.CaseLinkRisk:
			if rk, ok := risks[link.entityID]; ok {
				if rk.status == domain.RiskStatusCleared {
					continue
				}
				if rk.status == domain.RiskStatusMaterialized {
					if rk.actualEventID != nil {
						if _, linked := activeExceptions[*rk.actualEventID]; linked {
							continue
						}
					}
					continue
				}
				if rk.riskLevel == "critical" {
					in.HasCriticalRisk = true
				}
				if rk.riskLevel == "high" {
					in.HasHighRisk = true
				}
			}
		}
	}
	return in
}

func (r *CaseRepository) computeDerivedSeverity(ctx context.Context, tenantID, caseID uuid.UUID) (string, error) {
	health, err := r.GetCaseHealth(ctx, tenantID, caseID)
	if err != nil {
		return domain.CaseSeverityMedium, err
	}
	links, err := r.loadCaseWorkLinks(ctx, tenantID, []uuid.UUID{caseID})
	if err != nil {
		return domain.CaseSeverityMedium, err
	}
	exceptionIDs := make([]string, 0)
	riskIDs := make([]string, 0)
	for _, link := range links {
		switch link.entityType {
		case domain.CaseLinkException:
			exceptionIDs = append(exceptionIDs, link.entityID)
		case domain.CaseLinkRisk:
			riskIDs = append(riskIDs, link.entityID)
		}
	}
	exceptions, err := r.loadExceptionRows(ctx, tenantID, exceptionIDs)
	if err != nil {
		return domain.CaseSeverityMedium, err
	}
	risks, err := r.loadRiskRows(ctx, tenantID, riskIDs)
	if err != nil {
		return domain.CaseSeverityMedium, err
	}
	return domain.DeriveCaseSeverity(buildSeverityInput(health, exceptions, risks, links)), nil
}

func (r *CaseRepository) validateParticipantRole(role string) error {
	switch role {
	case domain.ParticipantRoleCollaborator, domain.ParticipantRoleObserver:
		return nil
	default:
		return apperrors.Validation("invalid participant role; use ownership commands for owner", map[string]any{"field": "role"})
	}
}

func (r *CaseRepository) tenantUserExists(ctx context.Context, tenantID, userID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM core.users
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		)
	`, userID, tenantID).Scan(&exists)
	return exists, err
}

func (r *CaseRepository) UpdateParticipantRole(ctx context.Context, tenantID, actorID, caseID, targetID uuid.UUID, role string) error {
	if err := r.validateParticipantRole(role); err != nil {
		return err
	}
	var previousRole string
	err := r.pool.QueryRow(ctx, `
		SELECT role FROM control_tower.operational_case_participant
		WHERE tenant_id = $1 AND case_id = $2 AND user_id = $3
	`, tenantID, caseID, targetID).Scan(&previousRole)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NotFound("participant not found")
		}
		return err
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE control_tower.operational_case_participant
		SET role = $4
		WHERE tenant_id = $1 AND case_id = $2 AND user_id = $3
	`, tenantID, caseID, targetID, role)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("participant not found")
	}
	_ = r.recordEvent(ctx, tenantID, caseID, domain.CaseEventSourceCase, "participant_role_changed", &actorID, map[string]any{
		"participantUserId": targetID.String(), "previousRole": previousRole, "role": role,
	})
	return r.touchCase(ctx, tenantID, caseID)
}

func (r *CaseRepository) ClearSeverityOverride(ctx context.Context, tenantID, userID, caseID uuid.UUID, expectedVersion int64) (domain.OperationalCase, error) {
	existing, err := r.GetCase(ctx, tenantID, caseID)
	if err != nil {
		return domain.OperationalCase{}, err
	}
	if existing.Version != expectedVersion {
		return domain.OperationalCase{}, apperrors.Conflict("case was modified by another operator", map[string]any{
			"caseId": caseID.String(), "reference": existing.Reference,
		})
	}
	derived, err := r.computeDerivedSeverity(ctx, tenantID, caseID)
	if err != nil {
		derived = domain.CaseSeverityMedium
	}
	row := r.pool.QueryRow(ctx, `
		UPDATE control_tower.operational_case
		SET derived_severity = $4, effective_severity = $4, severity_override = FALSE,
		    version = version + 1, updated_at = NOW(), last_activity_at = NOW()
		WHERE tenant_id = $1 AND id = $2 AND version = $3
		RETURNING `+caseSelectColumns,
		tenantID, caseID, expectedVersion, derived)
	c, err := scanCase(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.OperationalCase{}, apperrors.Conflict("case was modified by another operator", nil)
		}
		return domain.OperationalCase{}, err
	}
	_ = r.recordEvent(ctx, tenantID, caseID, domain.CaseEventSourceCase, "case_severity_override_cleared", &userID, map[string]any{
		"previousDerivedSeverity":   existing.DerivedSeverity,
		"previousEffectiveSeverity": existing.EffectiveSeverity,
		"previousOverride":          existing.SeverityOverride,
		"newOverride":               nil,
		"newEffectiveSeverity":      derived,
	})
	return c, nil
}
