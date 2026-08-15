//go:build integration

package enterprise

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain/tender"
)

func TestScoringEvaluationAllocationQuotaAwardPersistence(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedEnterpriseFixture(t, env.pool, "persist")

	rules := tender.QualificationRules{MinimumSLAScore: 60, MinimumCapacity: 100, RequireCarrierActive: true}
	evalID, scenarioID, proposalID := runEvaluationChain(t, ctx, env.pool, fix, rules, 1000, quotaTargetsFromFixture(fix), map[string]float64{
		fix.Carriers[0].CompanyID.String(): 20,
		fix.Carriers[1].CompanyID.String(): 30,
		fix.Carriers[2].CompanyID.String(): 25,
		fix.Carriers[3].CompanyID.String(): 25,
	})

	var evalCount int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.tender_evaluations WHERE id = $1`, evalID).Scan(&evalCount); err != nil || evalCount != 1 {
		t.Fatalf("evaluation not persisted: count=%d err=%v", evalCount, err)
	}
	var scoreCount int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.tender_carrier_scores WHERE evaluation_id = $1`, evalID).Scan(&scoreCount); err != nil || scoreCount != 4 {
		t.Fatalf("scores not persisted: count=%d err=%v", scoreCount, err)
	}
	var allocCount int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.allocation_results WHERE scenario_id = $1`, scenarioID).Scan(&allocCount); err != nil || allocCount == 0 {
		t.Fatalf("allocation results not persisted: count=%d err=%v", allocCount, err)
	}
	var quotaCount int
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.quota_balance_positions p
		JOIN rfx.quota_balance_policies pol ON pol.id = p.policy_id
		JOIN rfx.tender_evaluations te ON te.rfx_event_id = pol.rfx_event_id
		WHERE te.id = $1
	`, evalID).Scan(&quotaCount); err != nil || quotaCount != 4 {
		t.Fatalf("quota positions not persisted: count=%d err=%v", quotaCount, err)
	}
	var proposalStatus string
	if err := env.pool.QueryRow(ctx, `SELECT status FROM rfx.award_proposals WHERE id = $1`, proposalID).Scan(&proposalStatus); err != nil {
		t.Fatalf("award proposal lookup: %v", err)
	}
	if proposalStatus != tender.AwardProposalPendingApproval {
		t.Fatalf("proposal status=%s want %s", proposalStatus, tender.AwardProposalPendingApproval)
	}
}

func TestApproveRejectRace(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedEnterpriseFixture(t, env.pool, "race-ar")
	rules := tender.QualificationRules{MinimumSLAScore: 60, MinimumCapacity: 100}
	_, _, proposalID := runEvaluationChain(t, ctx, env.pool, fix, rules, 1000, quotaTargetsFromFixture(fix), balancedActualShares(fix))

	evalSvc := newEvaluationService(env.pool)
	approver := uuid.New()
	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errs[0] = evalSvc.ApproveAwardProposal(ctx, proposalID, fix.TenantID, approver)
	}()
	go func() {
		defer wg.Done()
		errs[1] = evalSvc.RejectAwardProposal(ctx, proposalID, fix.TenantID, uuid.New())
	}()
	wg.Wait()

	successes := 0
	for _, err := range errs {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("approve/reject race expected exactly 1 success, got %d errors=%v", successes, errs)
	}

	var status string
	if err := env.pool.QueryRow(ctx, `SELECT status FROM rfx.award_proposals WHERE id = $1`, proposalID).Scan(&status); err != nil {
		t.Fatalf("load proposal status: %v", err)
	}
	if status != tender.AwardProposalApproved && status != tender.AwardProposalRejected {
		t.Fatalf("unexpected terminal status after race: %s", status)
	}
}

func TestAwardFinalizationConcurrency(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedEnterpriseFixture(t, env.pool, "race-fin")
	rules := tender.QualificationRules{MinimumSLAScore: 60, MinimumCapacity: 100}
	_, _, proposalID := runEvaluationChain(t, ctx, env.pool, fix, rules, 1000, quotaTargetsFromFixture(fix), balancedActualShares(fix))

	evalSvc := newEvaluationService(env.pool)
	if err := evalSvc.ApproveAwardProposal(ctx, proposalID, fix.TenantID, uuid.New()); err != nil {
		t.Fatalf("approve proposal: %v", err)
	}

	idem := "finalize-" + proposalID.String()
	var wg sync.WaitGroup
	awards := make([]uuid.UUID, 2)
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			awards[idx], _, errs[idx] = evalSvc.FinalizeAward(ctx, proposalID, fix.TenantID, uuid.New(), &idem)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("finalize goroutine %d failed: %v", i, err)
		}
	}
	if awards[0] != awards[1] {
		t.Fatalf("concurrent finalize returned different award ids: %s vs %s", awards[0], awards[1])
	}

	var awardCount int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.awards WHERE award_proposal_id = $1`, proposalID).Scan(&awardCount); err != nil || awardCount != 1 {
		t.Fatalf("expected exactly 1 award row, count=%d err=%v", awardCount, err)
	}
	assertRfxEventStatus(t, env.pool, fix.EventID, fix.TenantID, "AWARDED")
}

func TestOrderConversionIdempotency(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedPrimaryE2EFixture(t, env.pool, "conv-idem")
	rules := tender.QualificationRules{MinimumSLAScore: 60, MinimumCapacity: 100}
	_, _, proposalID := runEvaluationChain(t, ctx, env.pool, fix, rules, 1000, quotaTargetsFromFixture(fix), balancedActualShares(fix))

	srv, _ := stubShipmentServer(t)
	evalSvc := newEvaluationServiceWithConversion(t, env.pool, srv.URL)
	if err := evalSvc.ApproveAwardProposal(ctx, proposalID, fix.TenantID, uuid.New()); err != nil {
		t.Fatalf("approve: %v", err)
	}

	idem := "finalize-" + proposalID.String()
	awardID1, conv1, err := evalSvc.FinalizeAward(ctx, proposalID, fix.TenantID, uuid.New(), &idem)
	if err != nil {
		t.Fatalf("first finalize: %v", err)
	}
	if conv1 == nil || conv1.Status != "COMPLETED" {
		t.Fatalf("expected completed conversion, got %+v err=%v", conv1, err)
	}

	awardID2, conv2, err := evalSvc.FinalizeAward(ctx, proposalID, fix.TenantID, uuid.New(), &idem)
	if err != nil {
		t.Fatalf("second finalize: %v", err)
	}
	if awardID1 != awardID2 {
		t.Fatalf("idempotent finalize award mismatch")
	}
	if conv2 == nil || conv2.Status != "COMPLETED" {
		t.Fatalf("idempotent conversion not completed: %+v", conv2)
	}

	var linkCount int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.award_transport_orders WHERE award_id = $1`, awardID1).Scan(&linkCount); err != nil || linkCount != 1 {
		t.Fatalf("expected one award_transport_orders row, count=%d err=%v", linkCount, err)
	}
}
