package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain/tender"
	"github.com/freight-platform/rfx-service/internal/service"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type TenderRepository struct {
	pool *pgxpool.Pool
}

func NewTenderRepository(pool *pgxpool.Pool) *TenderRepository {
	return &TenderRepository{pool: pool}
}

func (r *TenderRepository) GetScoringTemplateVersion(ctx context.Context, id, tenantID uuid.UUID) (*tender.ScoringTemplateSnapshot, error) {
	const q = `
		SELECT version_number, factors
		FROM rfx.scoring_template_versions
		WHERE id = $1 AND tenant_id = $2
	`
	var versionNumber int
	var raw []byte
	if err := r.pool.QueryRow(ctx, q, id, tenantID).Scan(&versionNumber, &raw); err != nil {
		return nil, mapDBError(err)
	}
	var factors []tender.ScoringFactorWeight
	if err := json.Unmarshal(raw, &factors); err != nil {
		return nil, err
	}
	return &tender.ScoringTemplateSnapshot{VersionNumber: versionNumber, Factors: factors}, nil
}

func (r *TenderRepository) CreateScoringTemplate(ctx context.Context, tenantID uuid.UUID, code, name string, factors []tender.ScoringFactorWeight, createdBy *uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	if err := tender.ValidateScoringTemplate(factors); err != nil {
		return uuid.Nil, uuid.Nil, apperrors.Validation(err.Error(), map[string]any{"field": "factors"})
	}
	raw, err := json.Marshal(factors)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, uuid.Nil, mapDBError(err)
	}
	defer tx.Rollback(ctx)

	var templateID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO rfx.scoring_templates (tenant_id, code, name, created_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (tenant_id, code) DO UPDATE SET name = EXCLUDED.name, updated_at = now()
		RETURNING id
	`, tenantID, code, name, createdBy).Scan(&templateID)
	if err != nil {
		return uuid.Nil, uuid.Nil, mapDBError(err)
	}

	var versionNumber int
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(version_number), 0) + 1 FROM rfx.scoring_template_versions WHERE scoring_template_id = $1
	`, templateID).Scan(&versionNumber); err != nil {
		return uuid.Nil, uuid.Nil, mapDBError(err)
	}

	var versionID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO rfx.scoring_template_versions (tenant_id, scoring_template_id, version_number, factors, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, tenantID, templateID, versionNumber, raw, createdBy).Scan(&versionID)
	if err != nil {
		return uuid.Nil, uuid.Nil, mapDBError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, uuid.Nil, mapDBError(err)
	}
	return templateID, versionID, nil
}

func (r *TenderRepository) ListEvaluationCandidates(ctx context.Context, rfxEventID, tenantID uuid.UUID) ([]tender.BidCandidate, error) {
	const q = `
		SELECT rr.participant_company_id::text,
			COALESCE(rev.price_amount, rr.price_amount, 0),
			COALESCE(rev.currency_code, rr.currency_code, 'RUB'),
			COALESCE(rev.capacity_units, rr.capacity_units, 0),
			COALESCE(rev.transit_hours, rr.transit_hours, 0),
			COALESCE(rev.sla_score_input, rr.sla_score_input, 0),
			COALESCE(rev.carrier_kpi_score_input, rr.carrier_kpi_score_input, 0),
			COALESCE(rev.reliability_score_input, rr.reliability_score_input, 0),
			COALESCE(c.status = 'ACTIVE', false),
			COALESCE(rev.revision_number, rr.active_revision_number, 1),
			COALESCE(rev.id::text, '')
		FROM rfx.rfx_responses rr
		LEFT JOIN rfx.rfx_response_revisions rev ON rev.rfx_response_id = rr.id AND rev.is_active = true
		LEFT JOIN core.companies c ON c.id = rr.participant_company_id AND c.tenant_id = rr.tenant_id
		WHERE rr.rfx_event_id = $1 AND rr.tenant_id = $2 AND rr.status = 'SUBMITTED' AND rr.deleted_at IS NULL
	`
	rows, err := r.pool.Query(ctx, q, rfxEventID, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	out := make([]tender.BidCandidate, 0)
	for rows.Next() {
		var c tender.BidCandidate
		if err := rows.Scan(
			&c.CarrierCompanyID, &c.PriceAmount, &c.CurrencyCode, &c.CapacityUnits, &c.TransitHours,
			&c.SLAScoreInput, &c.CarrierKPIInput, &c.ReliabilityInput, &c.CarrierActive,
			&c.RevisionNumber, &c.BidRevisionID,
		); err != nil {
			return nil, mapDBError(err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *TenderRepository) CreateEvaluation(ctx context.Context, in service.RunEvaluationInput, snapshot tender.ScoringTemplateSnapshot, qual []tender.QualificationResult, scores []tender.CarrierScoreResult) (uuid.UUID, error) {
	snapRaw, err := json.Marshal(snapshot)
	if err != nil {
		return uuid.Nil, err
	}
	rulesRaw, err := json.Marshal(in.QualificationRules)
	if err != nil {
		return uuid.Nil, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, mapDBError(err)
	}
	defer tx.Rollback(ctx)

	var evalID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO rfx.tender_evaluations (
			tenant_id, rfx_event_id, scoring_template_version_id, status, scoring_snapshot, qualification_rules, created_by
		) VALUES ($1, $2, $3, 'COMPLETED', $4, $5, NULL)
		RETURNING id
	`, in.TenantID, in.RfxEventID, in.ScoringTemplateVersionID, snapRaw, rulesRaw).Scan(&evalID)
	if err != nil {
		return uuid.Nil, mapDBError(err)
	}

	for _, q := range qual {
		reasons, _ := json.Marshal(q.Reasons)
		if _, err := tx.Exec(ctx, `
			INSERT INTO rfx.tender_qualification_results (tenant_id, evaluation_id, rfx_lot_id, carrier_company_id, result, reasons)
			VALUES ($1, $2, NULLIF($3, '')::uuid, $4, $5, $6)
		`, in.TenantID, evalID, nullIfEmpty(q.LotID), q.CarrierCompanyID, q.Result, reasons); err != nil {
			return uuid.Nil, mapDBError(err)
		}
	}
	for _, s := range scores {
		breakdown, _ := json.Marshal(s.Contributions)
		bidRevID := nullIfEmpty(s.BidRevisionID)
		if _, err := tx.Exec(ctx, `
			INSERT INTO rfx.tender_carrier_scores (
				tenant_id, evaluation_id, rfx_lot_id, carrier_company_id, bid_revision_id, total_score,
				price_score, sla_score, carrier_kpi_score, capacity_score, reliability_score, transit_time_score, breakdown
			) VALUES ($1, $2, NULLIF($3, '')::uuid, $4, NULLIF($5, '')::uuid, $6, $7, $8, $9, $10, $11, $12, $13)
		`, in.TenantID, evalID, nullIfEmpty(s.LotID), s.CarrierCompanyID, bidRevID, s.TotalScore,
			s.PriceScore, s.SLAScore, s.CarrierKPIScore, s.CapacityScore, s.ReliabilityScore, s.TransitScore, breakdown); err != nil {
			return uuid.Nil, mapDBError(err)
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE rfx.rfx_events SET status = 'EVALUATION_IN_PROGRESS', updated_at = now()
		WHERE id = $1 AND tenant_id = $2
	`, in.RfxEventID, in.TenantID); err != nil {
		return uuid.Nil, mapDBError(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, mapDBError(err)
	}
	return evalID, nil
}

func (r *TenderRepository) LoadEvaluationForAllocation(ctx context.Context, evaluationID, tenantID uuid.UUID) (tender.AllocationOutcome, []tender.CarrierScoreResult, []tender.BidCandidate, error) {
	var rfxEventID uuid.UUID
	if err := r.pool.QueryRow(ctx, `
		SELECT rfx_event_id FROM rfx.tender_evaluations WHERE id = $1 AND tenant_id = $2
	`, evaluationID, tenantID).Scan(&rfxEventID); err != nil {
		return tender.AllocationOutcome{}, nil, nil, mapDBError(err)
	}
	candidates, err := r.ListEvaluationCandidates(ctx, rfxEventID, tenantID)
	if err != nil {
		return tender.AllocationOutcome{}, nil, nil, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT carrier_company_id::text, COALESCE(rfx_lot_id::text, ''), total_score,
			price_score, sla_score, carrier_kpi_score, capacity_score, reliability_score, transit_time_score
		FROM rfx.tender_carrier_scores WHERE evaluation_id = $1 AND tenant_id = $2
	`, evaluationID, tenantID)
	if err != nil {
		return tender.AllocationOutcome{}, nil, nil, mapDBError(err)
	}
	defer rows.Close()
	scores := make([]tender.CarrierScoreResult, 0)
	for rows.Next() {
		var s tender.CarrierScoreResult
		if err := rows.Scan(&s.CarrierCompanyID, &s.LotID, &s.TotalScore, &s.PriceScore, &s.SLAScore, &s.CarrierKPIScore, &s.CapacityScore, &s.ReliabilityScore, &s.TransitScore); err != nil {
			return tender.AllocationOutcome{}, nil, nil, mapDBError(err)
		}
		scores = append(scores, s)
	}
	return tender.AllocationOutcome{}, scores, candidates, rows.Err()
}

func (r *TenderRepository) SaveAllocationScenario(ctx context.Context, in service.CreateAllocationScenarioInput, outcome tender.AllocationOutcome, positions []tender.QuotaPosition) (uuid.UUID, error) {
	cfgRaw, err := json.Marshal(in.Config)
	if err != nil {
		return uuid.Nil, err
	}
	summaryRaw, err := json.Marshal(outcome.Summary)
	if err != nil {
		return uuid.Nil, err
	}
	status := outcome.Status
	if status == "" {
		status = tender.AllocationStatusComputed
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, mapDBError(err)
	}
	defer tx.Rollback(ctx)

	var scenarioID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO rfx.allocation_scenarios (tenant_id, evaluation_id, name, strategy, config, status, summary)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, in.TenantID, in.EvaluationID, in.Name, in.Config.Strategy, cfgRaw, status, summaryRaw).Scan(&scenarioID)
	if err != nil {
		return uuid.Nil, mapDBError(err)
	}

	for _, line := range outcome.Lines {
		if _, err := tx.Exec(ctx, `
			INSERT INTO rfx.allocation_results (
				tenant_id, scenario_id, rfx_lot_id, carrier_company_id, score,
				target_share_pct, base_share_pct, balance_adjustment_pct, proposed_share_pct,
				committed_capacity, proposed_volume
			) VALUES ($1, $2, NULLIF($3, '')::uuid, $4, $5, $6, $7, $8, $9, $10, $11)
		`, in.TenantID, scenarioID, nullIfEmpty(line.LotID), line.CarrierCompanyID, line.Score,
			line.ProposedSharePct, line.BaseSharePct, line.BalanceAdjustmentPct, line.ProposedSharePct,
			line.CommittedCapacity, line.ProposedVolume); err != nil {
			return uuid.Nil, mapDBError(err)
		}
	}

	if len(in.QuotaTargets) > 0 {
		var policyID uuid.UUID
		err = tx.QueryRow(ctx, `
			INSERT INTO rfx.quota_balance_policies (tenant_id, rfx_event_id, period_type, tolerance_pct, carry_balance, max_correction_pct)
			SELECT te.tenant_id, te.rfx_event_id, $3, $4, $5, $6
			FROM rfx.tender_evaluations te WHERE te.id = $1 AND te.tenant_id = $2
			RETURNING id
		`, in.EvaluationID, in.TenantID, in.QuotaPolicy.PeriodType, in.QuotaPolicy.TolerancePct, in.QuotaPolicy.CarryBalance, in.QuotaPolicy.MaxCorrectionPct).Scan(&policyID)
		if err != nil {
			return uuid.Nil, mapDBError(err)
		}
		for _, t := range in.QuotaTargets {
			if _, err := tx.Exec(ctx, `
				INSERT INTO rfx.quota_balance_targets (tenant_id, policy_id, carrier_company_id, target_share_pct)
				VALUES ($1, $2, $3, $4)
			`, in.TenantID, policyID, t.CarrierCompanyID, t.TargetSharePct); err != nil {
				return uuid.Nil, mapDBError(err)
			}
		}
		for _, p := range positions {
			if _, err := tx.Exec(ctx, `
				INSERT INTO rfx.quota_balance_positions (tenant_id, policy_id, carrier_company_id, target_share_pct, actual_share_pct, balance_pp, status)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`, in.TenantID, policyID, p.CarrierCompanyID, p.TargetSharePct, p.ActualSharePct, p.BalancePP, p.Status); err != nil {
				return uuid.Nil, mapDBError(err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, mapDBError(err)
	}
	return scenarioID, nil
}

func (r *TenderRepository) LoadScenarioLines(ctx context.Context, scenarioID, tenantID uuid.UUID) ([]tender.AllocationLine, tender.ScoringTemplateSnapshot, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT carrier_company_id::text, COALESCE(rfx_lot_id::text, ''), score, base_share_pct,
			balance_adjustment_pct, proposed_share_pct, committed_capacity, proposed_volume
		FROM rfx.allocation_results WHERE scenario_id = $1 AND tenant_id = $2
	`, scenarioID, tenantID)
	if err != nil {
		return nil, tender.ScoringTemplateSnapshot{}, mapDBError(err)
	}
	defer rows.Close()
	lines := make([]tender.AllocationLine, 0)
	for rows.Next() {
		var l tender.AllocationLine
		if err := rows.Scan(&l.CarrierCompanyID, &l.LotID, &l.Score, &l.BaseSharePct, &l.BalanceAdjustmentPct, &l.ProposedSharePct, &l.CommittedCapacity, &l.ProposedVolume); err != nil {
			return nil, tender.ScoringTemplateSnapshot{}, mapDBError(err)
		}
		lines = append(lines, l)
	}
	var snap tender.ScoringTemplateSnapshot
	var raw []byte
	err = r.pool.QueryRow(ctx, `
		SELECT te.scoring_snapshot FROM rfx.tender_evaluations te
		JOIN rfx.allocation_scenarios s ON s.evaluation_id = te.id
		WHERE s.id = $1 AND s.tenant_id = $2
	`, scenarioID, tenantID).Scan(&raw)
	if err != nil {
		return lines, snap, mapDBError(err)
	}
	_ = json.Unmarshal(raw, &snap)
	return lines, snap, rows.Err()
}

func (r *TenderRepository) CreateAwardProposal(ctx context.Context, tenantID, rfxEventID, evaluationID, scenarioID uuid.UUID, lines []tender.AllocationLine, snapshot tender.ScoringTemplateSnapshot, createdBy *uuid.UUID, idempotencyKey *string) (uuid.UUID, error) {
	if idempotencyKey != nil && *idempotencyKey != "" {
		var existing uuid.UUID
		err := r.pool.QueryRow(ctx, `
			SELECT id FROM rfx.award_proposals WHERE tenant_id = $1 AND idempotency_key = $2
		`, tenantID, *idempotencyKey).Scan(&existing)
		if err == nil {
			return existing, nil
		}
		if err != pgx.ErrNoRows {
			return uuid.Nil, mapDBError(err)
		}
	}
	snapRaw, _ := json.Marshal(snapshot)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, mapDBError(err)
	}
	defer tx.Rollback(ctx)
	var proposalID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO rfx.award_proposals (
			tenant_id, rfx_event_id, evaluation_id, allocation_scenario_id, status, scoring_snapshot, created_by, idempotency_key
		) VALUES ($1, $2, $3, $4, 'DRAFT_PROPOSAL', $5, $6, $7)
		RETURNING id
	`, tenantID, rfxEventID, evaluationID, scenarioID, snapRaw, createdBy, idempotencyKey).Scan(&proposalID)
	if err != nil {
		return uuid.Nil, mapDBError(err)
	}
	for _, l := range lines {
		if _, err := tx.Exec(ctx, `
			INSERT INTO rfx.award_proposal_lines (
				tenant_id, award_proposal_id, rfx_lot_id, carrier_company_id, share_pct, volume, score,
				base_share_pct, balance_adjustment_pct
			) VALUES ($1, $2, NULLIF($3, '')::uuid, $4, $5, $6, $7, $8, $9)
		`, tenantID, proposalID, nullIfEmpty(l.LotID), l.CarrierCompanyID, l.ProposedSharePct, l.ProposedVolume, l.Score, l.BaseSharePct, l.BalanceAdjustmentPct); err != nil {
			return uuid.Nil, mapDBError(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, mapDBError(err)
	}
	return proposalID, nil
}

func (r *TenderRepository) GetAwardProposal(ctx context.Context, proposalID, tenantID uuid.UUID) (string, uuid.UUID, error) {
	var status string
	var eventID uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT status, rfx_event_id FROM rfx.award_proposals WHERE id = $1 AND tenant_id = $2
	`, proposalID, tenantID).Scan(&status, &eventID)
	return status, eventID, mapDBError(err)
}

func (r *TenderRepository) SubmitAwardProposal(ctx context.Context, proposalID, tenantID uuid.UUID) error {
	return r.transitionProposal(ctx, proposalID, tenantID, tender.AwardProposalDraft, tender.AwardProposalPendingApproval)
}

func (r *TenderRepository) ApproveAwardProposal(ctx context.Context, proposalID, tenantID uuid.UUID, approvedBy uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE rfx.award_proposals
		SET status = $3, approved_at = now(), approved_by = $4
		WHERE id = $1 AND tenant_id = $2 AND status = $5
	`, proposalID, tenantID, tender.AwardProposalApproved, approvedBy, tender.AwardProposalPendingApproval)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.Validation("award proposal cannot be approved from current status", map[string]any{})
	}
	return nil
}

func (r *TenderRepository) RejectAwardProposal(ctx context.Context, proposalID, tenantID uuid.UUID, rejectedBy uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE rfx.award_proposals SET status = $3, rejected_at = now(), rejected_by = $4
		WHERE id = $1 AND tenant_id = $2 AND status = $5
	`, proposalID, tenantID, tender.AwardProposalRejected, rejectedBy, tender.AwardProposalPendingApproval)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.Validation("award proposal cannot be rejected from current status", map[string]any{})
	}
	return nil
}

func (r *TenderRepository) FinalizeAward(ctx context.Context, proposalID, tenantID uuid.UUID, finalizedBy uuid.UUID, idempotencyKey *string) (uuid.UUID, error) {
	if idempotencyKey != nil && *idempotencyKey != "" {
		var existing uuid.UUID
		err := r.pool.QueryRow(ctx, `
			SELECT id FROM rfx.awards WHERE tenant_id = $1 AND idempotency_key = $2
		`, tenantID, *idempotencyKey).Scan(&existing)
		if err == nil {
			return existing, nil
		}
		if err != pgx.ErrNoRows {
			return uuid.Nil, mapDBError(err)
		}
	}

	var existingAward uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT id FROM rfx.awards WHERE award_proposal_id = $1 AND tenant_id = $2
	`, proposalID, tenantID).Scan(&existingAward)
	if err == nil {
		return existingAward, nil
	}
	if err != pgx.ErrNoRows {
		return uuid.Nil, mapDBError(err)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, mapDBError(err)
	}
	defer tx.Rollback(ctx)

	var eventID uuid.UUID
	err = tx.QueryRow(ctx, `
		UPDATE rfx.award_proposals SET status = $3
		WHERE id = $1 AND tenant_id = $2 AND status = $4
		RETURNING rfx_event_id
	`, proposalID, tenantID, tender.AwardProposalAwarded, tender.AwardProposalApproved).Scan(&eventID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				return uuid.Nil, mapDBError(rbErr)
			}
			var existing uuid.UUID
			if err2 := r.pool.QueryRow(ctx, `
				SELECT id FROM rfx.awards WHERE award_proposal_id = $1 AND tenant_id = $2
			`, proposalID, tenantID).Scan(&existing); err2 == nil {
				return existing, nil
			}
		}
		return uuid.Nil, mapDBError(err)
	}

	var awardID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO rfx.awards (tenant_id, award_proposal_id, rfx_event_id, finalized_by, idempotency_key)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, tenantID, proposalID, eventID, finalizedBy, idempotencyKey).Scan(&awardID)
	if err != nil {
		return uuid.Nil, mapDBError(err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE rfx.rfx_events SET status = 'AWARDED', updated_at = now()
		WHERE id = $1 AND tenant_id = $2
	`, eventID, tenantID); err != nil {
		return uuid.Nil, mapDBError(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, mapDBError(err)
	}
	return awardID, nil
}

func (r *TenderRepository) transitionProposal(ctx context.Context, proposalID, tenantID uuid.UUID, from, to string) error {
	if err := tender.ValidateAwardProposalTransition(from, to); err != nil {
		return apperrors.Validation(err.Error(), map[string]any{"field": "status"})
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE rfx.award_proposals
		SET status = $3,
		    submitted_at = CASE WHEN $5::text = 'PENDING_APPROVAL' THEN now() ELSE submitted_at END
		WHERE id = $1 AND tenant_id = $2 AND status = $4
	`, proposalID, tenantID, to, from, to)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.Validation(fmt.Sprintf("invalid award proposal transition from %s", from), map[string]any{})
	}
	return nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
