//go:build integration

package enterprise

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/domain/tender"
)

func TestPrimaryEnterpriseChainFourCarriers(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedEnterpriseFixture(t, env.pool, "primary")

	t.Log("step 1/8: seeded tenant, event, 4 carrier responses")
	if fix.Carriers[0].ResponseID == uuid.Nil {
		t.Fatal("expected carrier responses seeded")
	}

	quotaTargets := []tender.QuotaTarget{
		{CarrierCompanyID: fix.Carriers[0].CompanyID.String(), TargetSharePct: 40},
		{CarrierCompanyID: fix.Carriers[1].CompanyID.String(), TargetSharePct: 30},
		{CarrierCompanyID: fix.Carriers[2].CompanyID.String(), TargetSharePct: 20},
		{CarrierCompanyID: fix.Carriers[3].CompanyID.String(), TargetSharePct: 10},
	}
	actualShares := map[string]float64{
		fix.Carriers[0].CompanyID.String(): 52,
		fix.Carriers[1].CompanyID.String(): 18,
		fix.Carriers[2].CompanyID.String(): 21,
		fix.Carriers[3].CompanyID.String(): 9,
	}

	evalID, scenarioID, proposalID := runEvaluationChain(
		t, ctx, env.pool, fix,
		tender.QualificationRules{MinimumSLAScore: 75, MinimumCapacity: 150, RequireCarrierActive: true},
		400,
		quotaTargets,
		actualShares,
	)
	t.Logf("step 2/8: evaluation=%s scenario=%s proposal=%s", evalID, scenarioID, proposalID)

	var qualCount, scoreCount int
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.tender_qualification_results WHERE evaluation_id = $1
	`, evalID).Scan(&qualCount); err != nil {
		t.Fatalf("count qualification results: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.tender_carrier_scores WHERE evaluation_id = $1
	`, evalID).Scan(&scoreCount); err != nil {
		t.Fatalf("count carrier scores: %v", err)
	}
	if qualCount != 4 {
		t.Fatalf("expected 4 qualification rows, got %d", qualCount)
	}
	if scoreCount != 3 {
		t.Fatalf("expected 3 scored carriers (B disqualified on SLA), got %d", scoreCount)
	}
	t.Log("step 3/8: qualification and scoring persisted")

	var allocLines int
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.allocation_results WHERE scenario_id = $1
	`, scenarioID).Scan(&allocLines); err != nil {
		t.Fatalf("count allocation lines: %v", err)
	}
	if allocLines == 0 {
		t.Fatal("expected allocation result lines")
	}
	t.Log("step 4/8: allocation scenario persisted")

	evalSvc := newEvaluationService(env.pool)
	if err := evalSvc.ApproveAwardProposal(ctx, proposalID, fix.TenantID, uuid.New()); err != nil {
		t.Fatalf("approve proposal: %v", err)
	}
	t.Log("step 5/8: award proposal submitted and approved")

	finalizeKey := "finalize-" + proposalID.String()
	awardID, _, err := evalSvc.FinalizeAward(ctx, proposalID, fix.TenantID, uuid.New(), &finalizeKey)
	if err != nil {
		t.Fatalf("finalize award: %v", err)
	}
	t.Logf("step 6/8: award finalized award_id=%s", awardID)

	assertRfxEventStatus(t, env.pool, fix.EventID, fix.TenantID, "AWARDED")

	var proposalStatus string
	if err := env.pool.QueryRow(ctx, `
		SELECT status FROM rfx.award_proposals WHERE id = $1
	`, proposalID).Scan(&proposalStatus); err != nil {
		t.Fatalf("load proposal status: %v", err)
	}
	if proposalStatus != tender.AwardProposalAwarded {
		t.Fatalf("proposal status=%s want AWARDED", proposalStatus)
	}
	t.Log("step 7/8: governance chain complete")

	t.Run("transport_order_conversion", func(t *testing.T) {
		convFix := seedPrimaryE2EFixture(t, env.pool, "primary-conv")
		_, _, convProposalID := runEvaluationChain(
			t, ctx, env.pool, convFix,
			tender.QualificationRules{MinimumSLAScore: 60, MinimumCapacity: 100},
			1000,
			quotaTargetsFromFixture(convFix),
			balancedActualShares(convFix),
		)
		srv, _ := stubShipmentServer(t)
		convEvalSvc := newEvaluationServiceWithConversion(t, env.pool, srv.URL)
		approver := uuid.New()
		if err := convEvalSvc.ApproveAwardProposal(ctx, convProposalID, convFix.TenantID, approver); err != nil {
			t.Fatalf("approve for conversion: %v", err)
		}
		key := "primary-conv-" + convProposalID.String()
		awardID, conversion, err := convEvalSvc.FinalizeAward(ctx, convProposalID, convFix.TenantID, approver, &key)
		if err != nil {
			t.Fatalf("finalize with conversion: %v", err)
		}
		if conversion == nil || conversion.Status != "COMPLETED" {
			t.Fatalf("conversion not completed: %+v", conversion)
		}
		var linkCount int
		if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.award_transport_orders WHERE award_id = $1`, awardID).Scan(&linkCount); err != nil || linkCount != 1 {
			t.Fatalf("award_transport_orders link missing: count=%d err=%v", linkCount, err)
		}
	})
}

func TestPrimaryChainProposalIdempotency(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedEnterpriseFixture(t, env.pool, "idem-proposal")

	evalID, scenarioID, firstProposalID := runEvaluationChain(
		t, ctx, env.pool, fix,
		tender.QualificationRules{MinimumSLAScore: 60},
		400,
		nil,
		nil,
	)

	evalSvc := newEvaluationService(env.pool)
	idem := "proposal-idem-" + fix.EventID.String()
	secondID, err := evalSvc.CreateAwardProposal(ctx, fix.TenantID, fix.EventID, evalID, scenarioID, nil, &idem)
	if err != nil {
		t.Fatalf("second proposal with new key: %v", err)
	}
	if secondID == firstProposalID {
		t.Fatal("expected distinct proposal ids for different idempotency keys")
	}

	dupID, err := evalSvc.CreateAwardProposal(ctx, fix.TenantID, fix.EventID, evalID, scenarioID, nil, &idem)
	if err != nil {
		t.Fatalf("duplicate idempotency key: %v", err)
	}
	if dupID != secondID {
		t.Fatalf("idempotent proposal: first=%s dup=%s", secondID, dupID)
	}
}

func TestPrimaryChainRfxServiceLifecycle(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	tenantID := uuid.New()
	ownerID := uuid.New()
	_, err := env.pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1, 'svc', 'Service Test')`, tenantID)
	if err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	_, err = env.pool.Exec(ctx, `
		INSERT INTO core.companies (id, tenant_id, legal_name, company_type, status)
		VALUES ($1, $2, 'Owner', 'SHIPPER', 'ACTIVE')
	`, ownerID, tenantID)
	if err != nil {
		t.Fatalf("seed owner: %v", err)
	}

	deadline := time.Now().UTC().Add(24 * time.Hour)
	rfxSvc := newRfxService(env.pool)
	event, err := rfxSvc.CreateEvent(ctx, domain.CreateRfxEventInput{
		TenantID:         tenantID,
		RfxNumber:        "RFX-SVC-001",
		RfxType:          "CONTRACT_TENDER",
		Category:         "FREIGHT",
		Title:            "Service lifecycle",
		OwnerCompanyID:   ownerID,
		ResponseDeadline: &deadline,
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	published, err := rfxSvc.PublishEvent(ctx, event.ID, tenantID)
	if err != nil {
		t.Fatalf("publish event: %v", err)
	}
	if published.Status != domain.RfxStatusPublished {
		t.Fatalf("published status=%s", published.Status)
	}
}
