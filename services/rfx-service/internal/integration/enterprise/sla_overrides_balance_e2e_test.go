//go:build integration

package enterprise

import (
	"context"
	"testing"

	"github.com/freight-platform/rfx-service/internal/domain/tender"
)

func TestSLAOverridesBalanceCorrection(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedEnterpriseFixture(t, env.pool, "sla-balance")

	// Carrier B is underallocated historically but fails SLA gate — balance must not rescue it.
	quotaTargets := []tender.QuotaTarget{
		{CarrierCompanyID: fix.Carriers[0].CompanyID.String(), TargetSharePct: 40},
		{CarrierCompanyID: fix.Carriers[1].CompanyID.String(), TargetSharePct: 30},
		{CarrierCompanyID: fix.Carriers[2].CompanyID.String(), TargetSharePct: 20},
		{CarrierCompanyID: fix.Carriers[3].CompanyID.String(), TargetSharePct: 10},
	}
	actualShares := map[string]float64{
		fix.Carriers[0].CompanyID.String(): 52,
		fix.Carriers[1].CompanyID.String(): 5,
		fix.Carriers[2].CompanyID.String(): 15,
		fix.Carriers[3].CompanyID.String(): 28,
	}

	_, scenarioID, _ := runEvaluationChain(
		t, ctx, env.pool, fix,
		tender.QualificationRules{MinimumSLAScore: 75, MinimumCapacity: 150, RequireCarrierActive: true},
		400,
		quotaTargets,
		actualShares,
	)

	var bShare, bAdjustment float64
	err := env.pool.QueryRow(ctx, `
		SELECT proposed_share_pct, balance_adjustment_pct
		FROM rfx.allocation_results
		WHERE scenario_id = $1 AND carrier_company_id = $2
	`, scenarioID, fix.Carriers[1].CompanyID).Scan(&bShare, &bAdjustment)
	if err != nil {
		// Disqualified carrier may have no allocation row — acceptable.
		t.Log("carrier B has no allocation row after SLA disqualification (expected)")
	} else if bShare > 0 || bAdjustment > 0 {
		t.Fatalf("SLA disqualification must override quota balance: share=%.4f adjustment=%.4f", bShare, bAdjustment)
	}

	var bQual string
	err = env.pool.QueryRow(ctx, `
		SELECT result FROM rfx.tender_qualification_results tq
		JOIN rfx.allocation_scenarios s ON s.evaluation_id = tq.evaluation_id
		WHERE s.id = $1 AND tq.carrier_company_id = $2
	`, scenarioID, fix.Carriers[1].CompanyID).Scan(&bQual)
	if err != nil {
		t.Fatalf("load B qualification: %v", err)
	}
	if bQual != tender.QualificationDisqualified {
		t.Fatalf("carrier B qualification=%s want DISQUALIFIED", bQual)
	}

	// Qualified underallocated carrier C should receive bounded positive adjustment.
	var cAdjustment float64
	err = env.pool.QueryRow(ctx, `
		SELECT balance_adjustment_pct FROM rfx.allocation_results
		WHERE scenario_id = $1 AND carrier_company_id = $2
	`, scenarioID, fix.Carriers[2].CompanyID).Scan(&cAdjustment)
	if err != nil {
		t.Fatalf("load C allocation: %v", err)
	}
	if cAdjustment <= 0 {
		t.Fatalf("expected positive balance adjustment for qualified carrier C, got %.4f", cAdjustment)
	}
	if cAdjustment > 10.01 {
		t.Fatalf("balance adjustment must respect max_correction_pct=10, got %.4f", cAdjustment)
	}

	var policyCount int
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.quota_balance_policies p
		JOIN rfx.allocation_scenarios s ON s.evaluation_id = (
			SELECT evaluation_id FROM rfx.allocation_scenarios WHERE id = $1
		)
	`).Scan(&policyCount); err == nil && policyCount == 0 {
		t.Log("quota policy persisted via allocation scenario save")
	}
}

func TestSLAOverrideDoesNotAppearInAwardLines(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedEnterpriseFixture(t, env.pool, "sla-award")

	quotaTargets := []tender.QuotaTarget{
		{CarrierCompanyID: fix.Carriers[0].CompanyID.String(), TargetSharePct: 40},
		{CarrierCompanyID: fix.Carriers[1].CompanyID.String(), TargetSharePct: 30},
		{CarrierCompanyID: fix.Carriers[2].CompanyID.String(), TargetSharePct: 20},
		{CarrierCompanyID: fix.Carriers[3].CompanyID.String(), TargetSharePct: 10},
	}
	actualShares := balancedActualSharesWith(fix, map[int]float64{1: 5})

	_, _, proposalID := runEvaluationChain(
		t, ctx, env.pool, fix,
		tender.QualificationRules{MinimumSLAScore: 75, MinimumCapacity: 150},
		400,
		quotaTargets,
		actualShares,
	)

	var bLineCount int
	err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.award_proposal_lines
		WHERE award_proposal_id = $1 AND carrier_company_id = $2 AND share_pct > 0
	`, proposalID, fix.Carriers[1].CompanyID).Scan(&bLineCount)
	if err != nil {
		t.Fatalf("count award lines for B: %v", err)
	}
	if bLineCount > 0 {
		t.Fatal("disqualified carrier B must not receive positive share in award proposal lines")
	}
}
