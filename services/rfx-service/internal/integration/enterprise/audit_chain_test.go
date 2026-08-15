//go:build integration

package enterprise

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain/tender"
)

type auditChainLink struct {
	Step string
	ID   uuid.UUID
}

func TestAuditChainReconstructsPersistedFlow(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedEnterpriseFixture(t, env.pool, "audit")

	quotaTargets := []tender.QuotaTarget{
		{CarrierCompanyID: fix.Carriers[0].CompanyID.String(), TargetSharePct: 40},
		{CarrierCompanyID: fix.Carriers[1].CompanyID.String(), TargetSharePct: 30},
		{CarrierCompanyID: fix.Carriers[2].CompanyID.String(), TargetSharePct: 20},
		{CarrierCompanyID: fix.Carriers[3].CompanyID.String(), TargetSharePct: 10},
	}

	evalID, scenarioID, proposalID := runEvaluationChain(
		t, ctx, env.pool, fix,
		tender.QualificationRules{MinimumSLAScore: 75, MinimumCapacity: 150},
		400,
		quotaTargets,
		balancedActualSharesWith(fix, map[int]float64{0: 27, 1: 23, 2: 25, 3: 25}),
	)

	evalSvc := newEvaluationService(env.pool)
	_ = evalSvc.ApproveAwardProposal(ctx, proposalID, fix.TenantID, uuid.New())
	awardID, _, err := evalSvc.FinalizeAward(ctx, proposalID, fix.TenantID, uuid.New(), strPtr("audit-finalize-"+proposalID.String()))
	if err != nil {
		t.Fatalf("finalize award: %v", err)
	}

	chain := make([]auditChainLink, 0, 10)

	var templateID, templateVersionID uuid.UUID
	err = env.pool.QueryRow(ctx, `
		SELECT st.id, stv.id
		FROM rfx.scoring_template_versions stv
		JOIN rfx.scoring_templates st ON st.id = stv.scoring_template_id
		WHERE stv.id = $1
	`, fix.TemplateVer).Scan(&templateID, &templateVersionID)
	if err != nil {
		t.Fatalf("load scoring template chain: %v", err)
	}
	chain = append(chain, auditChainLink{Step: "scoring_template", ID: templateID})
	chain = append(chain, auditChainLink{Step: "scoring_template_version", ID: templateVersionID})

	var eventFromEval uuid.UUID
	var evalStatus string
	err = env.pool.QueryRow(ctx, `
		SELECT rfx_event_id, status FROM rfx.tender_evaluations WHERE id = $1
	`, evalID).Scan(&eventFromEval, &evalStatus)
	if err != nil {
		t.Fatalf("load evaluation: %v", err)
	}
	if eventFromEval != fix.EventID {
		t.Fatalf("evaluation event mismatch: %s vs %s", eventFromEval, fix.EventID)
	}
	chain = append(chain, auditChainLink{Step: "tender_evaluation", ID: evalID})

	var qualCount int
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.tender_qualification_results WHERE evaluation_id = $1
	`, evalID).Scan(&qualCount); err != nil {
		t.Fatalf("qualification count: %v", err)
	}
	if qualCount != 4 {
		t.Fatalf("expected 4 qualification results in chain, got %d", qualCount)
	}

	var scoreCount int
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.tender_carrier_scores WHERE evaluation_id = $1
	`, evalID).Scan(&scoreCount); err != nil {
		t.Fatalf("score count: %v", err)
	}
	if scoreCount == 0 {
		t.Fatal("expected carrier scores in chain")
	}

	var evalFromScenario uuid.UUID
	var scenarioStatus string
	err = env.pool.QueryRow(ctx, `
		SELECT evaluation_id, status FROM rfx.allocation_scenarios WHERE id = $1
	`, scenarioID).Scan(&evalFromScenario, &scenarioStatus)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	if evalFromScenario != evalID {
		t.Fatalf("scenario evaluation mismatch")
	}
	chain = append(chain, auditChainLink{Step: "allocation_scenario", ID: scenarioID})

	var allocLineCount int
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.allocation_results WHERE scenario_id = $1
	`, scenarioID).Scan(&allocLineCount); err != nil {
		t.Fatalf("allocation lines: %v", err)
	}
	if allocLineCount == 0 {
		t.Fatal("expected allocation results in chain")
	}

	var policyID uuid.UUID
	err = env.pool.QueryRow(ctx, `
		SELECT id FROM rfx.quota_balance_policies WHERE rfx_event_id = $1 LIMIT 1
	`, fix.EventID).Scan(&policyID)
	if err != nil {
		t.Fatalf("load quota policy: %v", err)
	}
	chain = append(chain, auditChainLink{Step: "quota_balance_policy", ID: policyID})

	var targetCount int
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.quota_balance_targets WHERE policy_id = $1
	`, policyID).Scan(&targetCount); err != nil {
		t.Fatalf("quota targets: %v", err)
	}
	if targetCount != 4 {
		t.Fatalf("expected 4 quota targets, got %d", targetCount)
	}

	var positionCount int
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.quota_balance_positions WHERE policy_id = $1
	`, policyID).Scan(&positionCount); err != nil {
		t.Fatalf("quota positions: %v", err)
	}
	if positionCount != 4 {
		t.Fatalf("expected 4 quota positions, got %d", positionCount)
	}

	var proposalEval, proposalScenario, proposalEvent uuid.UUID
	var proposalStatus string
	err = env.pool.QueryRow(ctx, `
		SELECT evaluation_id, allocation_scenario_id, rfx_event_id, status
		FROM rfx.award_proposals WHERE id = $1
	`, proposalID).Scan(&proposalEval, &proposalScenario, &proposalEvent, &proposalStatus)
	if err != nil {
		t.Fatalf("load proposal: %v", err)
	}
	if proposalEval != evalID || proposalScenario != scenarioID || proposalEvent != fix.EventID {
		t.Fatal("award proposal FK chain broken")
	}
	chain = append(chain, auditChainLink{Step: "award_proposal", ID: proposalID})

	var lineCount int
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.award_proposal_lines WHERE award_proposal_id = $1
	`, proposalID).Scan(&lineCount); err != nil {
		t.Fatalf("proposal lines: %v", err)
	}
	if lineCount == 0 {
		t.Fatal("expected award proposal lines in chain")
	}

	var awardProposalID uuid.UUID
	var awardEvent uuid.UUID
	err = env.pool.QueryRow(ctx, `
		SELECT award_proposal_id, rfx_event_id FROM rfx.awards WHERE id = $1
	`, awardID).Scan(&awardProposalID, &awardEvent)
	if err != nil {
		t.Fatalf("load award: %v", err)
	}
	if awardProposalID != proposalID || awardEvent != fix.EventID {
		t.Fatal("award FK chain broken")
	}
	chain = append(chain, auditChainLink{Step: "award", ID: awardID})

	// Verify response → evaluation candidate linkage exists.
	var submittedResponses int
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_responses
		WHERE rfx_event_id = $1 AND tenant_id = $2 AND status = 'SUBMITTED'
	`, fix.EventID, fix.TenantID).Scan(&submittedResponses); err != nil {
		t.Fatalf("submitted responses: %v", err)
	}
	if submittedResponses != 4 {
		t.Fatalf("expected 4 submitted responses at chain root, got %d", submittedResponses)
	}

	t.Logf("reconstructed audit chain (%d links):", len(chain))
	for i, link := range chain {
		t.Logf("  %d. %s → %s", i+1, link.Step, link.ID)
	}

	if len(chain) < 7 {
		t.Fatalf("expected at least 7 persisted chain links, got %d", len(chain))
	}
}

func TestAuditChainProposalGovernanceTimestamps(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedEnterpriseFixture(t, env.pool, "audit-ts")

	_, _, proposalID := runEvaluationChain(
		t, ctx, env.pool, fix,
		tender.QualificationRules{MinimumSLAScore: 60},
		400,
		nil,
		nil,
	)
	evalSvc := newEvaluationService(env.pool)
	approver := uuid.New()
	if err := evalSvc.ApproveAwardProposal(ctx, proposalID, fix.TenantID, approver); err != nil {
		t.Fatalf("approve: %v", err)
	}

	var submittedAt, approvedAt *string
	var approvedBy *uuid.UUID
	err := env.pool.QueryRow(ctx, `
		SELECT submitted_at::text, approved_at::text, approved_by
		FROM rfx.award_proposals WHERE id = $1
	`, proposalID).Scan(&submittedAt, &approvedAt, &approvedBy)
	if err != nil {
		t.Fatalf("load governance timestamps: %v", err)
	}
	if submittedAt == nil || *submittedAt == "" {
		t.Fatal("expected submitted_at in audit chain")
	}
	if approvedAt == nil || *approvedAt == "" {
		t.Fatal("expected approved_at in audit chain")
	}
	if approvedBy == nil || *approvedBy != approver {
		t.Fatalf("approved_by=%v want %s", approvedBy, approver)
	}
}
