//go:build integration

package freightsettlement

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/billing-register-service/internal/domain"
	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

func assertForbidden(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected forbidden error")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func assertValidation(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected validation error")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeValidation {
		t.Fatalf("expected validation, got %v", err)
	}
}

func Test01EligibleSettlementCreation(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)

	settlement := createSettlement(t, env, fix, fix.ShipmentID, "eg-01-create")
	if settlement.Status != domain.SettlementStatusDraft {
		t.Fatalf("status=%s want DRAFT", settlement.Status)
	}
	if settlement.BaseFreightAmount != fix.AwardAmount {
		t.Fatalf("base=%v want award %v", settlement.BaseFreightAmount, fix.AwardAmount)
	}
	if countSettlementsForShipment(t, env.pool, fix.TenantID, fix.ShipmentID) != 1 {
		t.Fatal("expected single settlement row")
	}
}

func Test02NonEligibleDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()

	noPOD := seedIneligibleShipment(t, env.pool, fix, domain.ShipmentStatusDelivered, false)
	_, err := env.settlements.Create(ctx, createSettlementInput(fix, noPOD, "eg-02-no-pod"))
	assertValidation(t, err)

	wrongStatus := seedIneligibleShipment(t, env.pool, fix, "IN_TRANSIT", true)
	_, err = env.settlements.Create(ctx, createSettlementInput(fix, wrongStatus, "eg-02-wrong-status"))
	assertValidation(t, err)
}

func Test03DuplicateIdempotent(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()

	first, err := env.settlements.Create(ctx, createSettlementInput(fix, fix.ShipmentID, "eg-03-idem"))
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	second, err := env.settlements.Create(ctx, createSettlementInput(fix, fix.ShipmentID, "eg-03-idem"))
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("idempotent retry changed id %s -> %s", first.ID, second.ID)
	}
	if countSettlementsForShipment(t, env.pool, fix.TenantID, fix.ShipmentID) != 1 {
		t.Fatal("idempotent retry created duplicate settlement")
	}
}

func Test04ConcurrentCreateNoDuplicate(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()

	const workers = 8
	var wg sync.WaitGroup
	errs := make([]error, workers)
	ids := make([]uuid.UUID, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			settlement, err := env.settlements.Create(ctx, createSettlementInput(fix, fix.ShipmentID, ""))
			errs[i] = err
			if settlement != nil {
				ids[i] = settlement.ID
			}
		}()
	}
	wg.Wait()
	success := 0
	var winner uuid.UUID
	for i, err := range errs {
		if err == nil {
			success++
			winner = ids[i]
		}
	}
	if success == 0 {
		t.Fatalf("all concurrent creates failed: %v", errs)
	}
	for _, id := range ids {
		if id != uuid.Nil && id != winner {
			t.Fatalf("concurrent creates returned different ids: %s vs %s", winner, id)
		}
	}
	if countSettlementsForShipment(t, env.pool, fix.TenantID, fix.ShipmentID) != 1 {
		t.Fatal("concurrent create produced duplicate settlements")
	}
}

func Test05BaseAmountFromAwardNotClient(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)

	settlement := createSettlement(t, env, fix, fix.ShipmentID, "eg-05-award-base")
	if settlement.BaseFreightAmount != fix.AwardAmount {
		t.Fatalf("base=%v want award amount %v", settlement.BaseFreightAmount, fix.AwardAmount)
	}
	if settlement.BaseFreightAmount == fix.OfferAmount {
		t.Fatalf("base=%v must not use client offer amount %v", settlement.BaseFreightAmount, fix.OfferAmount)
	}
}

func Test06ClientBaseTamperingIgnored(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()

	if _, err := env.pool.Exec(ctx, `UPDATE rfx.rfx_awards SET total_amount=$2 WHERE id=$1`, fix.AwardID, 50000); err != nil {
		t.Fatalf("tamper award header: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `UPDATE transport.transport_orders SET external_reference=$2 WHERE id=$1`,
		fix.OrderID, "CLIENT_AMOUNT:99999"); err != nil {
		t.Fatalf("tamper order reference: %v", err)
	}

	settlement := createSettlement(t, env, fix, fix.ShipmentID, "eg-06-server-derived")
	if settlement.BaseFreightAmount != fix.AwardAmount {
		t.Fatalf("base=%v want server-derived award link amount %v", settlement.BaseFreightAmount, fix.AwardAmount)
	}
}

func Test07CarrierOwnAccessorialAllow(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlement := createSettlement(t, env, fix, fix.ShipmentID, "eg-07-accessorial")

	item, err := env.settlements.ProposeAccessorial(context.Background(), settlement.ID, domain.ProposeAccessorialInput{
		SettlementActorInput: carrierActor(fix),
		ChargeCode:           "DETENTION", Amount: 2500,
	})
	if err != nil {
		t.Fatalf("propose accessorial: %v", err)
	}
	if item.Status != domain.AccessorialStatusProposed {
		t.Fatalf("status=%s want PROPOSED", item.Status)
	}
}

func Test08CompetitorCarrierDeny(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlement := createSettlement(t, env, fix, fix.ShipmentID, "eg-08-competitor")

	_, err := env.settlements.ProposeAccessorial(context.Background(), settlement.ID, domain.ProposeAccessorialInput{
		SettlementActorInput: domain.SettlementActorInput{
			TenantID: fix.TenantID, ActorCompanyID: fix.CarrierB,
			ActorKind: domain.SettlementActorCarrier, ActorUserID: fix.UserID,
		},
		ChargeCode: "DETENTION", Amount: 2500,
	})
	assertForbidden(t, err)
}

func Test09CrossTenantDeny(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)

	_, err := env.settlements.Create(context.Background(), domain.CreateFreightSettlementInput{
		TenantID: fix.OtherTenantID, ShipmentID: fix.ShipmentID,
		ActorCompanyID: fix.CarrierA, ActorKind: domain.SettlementActorCarrier, ActorUserID: fix.UserID,
		IdempotencyKey: "eg-09-cross-tenant",
	})
	if err == nil {
		t.Fatal("cross-tenant create must fail")
	}
}

func Test10BuyerApproveAllow(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlement := createSettlement(t, env, fix, fix.ShipmentID, "eg-10-buyer-approve")

	if _, err := env.settlements.SubmitForReview(context.Background(), settlement.ID, carrierActor(fix)); err != nil {
		t.Fatalf("submit: %v", err)
	}
	approved, err := env.settlements.Approve(context.Background(), settlement.ID, buyerActor(fix))
	if err != nil {
		t.Fatalf("buyer approve: %v", err)
	}
	if approved.Status != domain.SettlementStatusApproved {
		t.Fatalf("status=%s want APPROVED", approved.Status)
	}
	if approved.ServiceAcceptedBy == nil || *approved.ServiceAcceptedBy != fix.BuyerUserID {
		t.Fatal("service acceptance not recorded for buyer")
	}
}

func Test11ForeignBuyerDeny(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlement := createSettlement(t, env, fix, fix.ShipmentID, "eg-11-foreign-buyer")

	if _, err := env.settlements.SubmitForReview(context.Background(), settlement.ID, carrierActor(fix)); err != nil {
		t.Fatalf("submit: %v", err)
	}
	_, err := env.settlements.Approve(context.Background(), settlement.ID, domain.SettlementActorInput{
		TenantID: fix.TenantID, ActorCompanyID: fix.ForeignBuyer,
		ActorKind: domain.SettlementActorBuyer, ActorUserID: fix.BuyerUserID,
	})
	assertForbidden(t, err)
}

func Test12PendingExcludedFromTotal(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	settlement := createSettlement(t, env, fix, fix.ShipmentID, "eg-12-pending")

	if _, err := env.settlements.ProposeAccessorial(ctx, settlement.ID, domain.ProposeAccessorialInput{
		SettlementActorInput: carrierActor(fix), ChargeCode: "DETENTION", Amount: 3000,
	}); err != nil {
		t.Fatalf("propose: %v", err)
	}
	detail, err := env.settlements.GetDetail(ctx, settlement.ID, fix.TenantID, fix.CarrierA, domain.SettlementActorCarrier)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if detail.Settlement.ApprovedAccessorialTotal != 0 || detail.Settlement.TotalWithoutVAT != fix.AwardAmount {
		t.Fatalf("pending included in totals: approved=%v total=%v", detail.Settlement.ApprovedAccessorialTotal, detail.Settlement.TotalWithoutVAT)
	}
}

func Test13ApprovedIncluded(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	settlement := createSettlement(t, env, fix, fix.ShipmentID, "eg-13-approved")

	item, err := env.settlements.ProposeAccessorial(ctx, settlement.ID, domain.ProposeAccessorialInput{
		SettlementActorInput: carrierActor(fix), ChargeCode: "LUMPER", Amount: 4000,
	})
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	detail, err := env.settlements.ApproveAccessorial(ctx, settlement.ID, item.ID, buyerActor(fix))
	if err != nil {
		t.Fatalf("approve accessorial: %v", err)
	}
	if detail.Settlement.ApprovedAccessorialTotal != 4000 {
		t.Fatalf("approved total=%v want 4000", detail.Settlement.ApprovedAccessorialTotal)
	}
	wantTotal := fix.AwardAmount + 4000
	if detail.Settlement.TotalWithoutVAT != wantTotal {
		t.Fatalf("total_without_vat=%v want %v", detail.Settlement.TotalWithoutVAT, wantTotal)
	}
}

func Test14RejectedExcluded(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	settlement := createSettlement(t, env, fix, fix.ShipmentID, "eg-14-rejected")

	item, err := env.settlements.ProposeAccessorial(ctx, settlement.ID, domain.ProposeAccessorialInput{
		SettlementActorInput: carrierActor(fix), ChargeCode: "REWORK", Amount: 6000,
	})
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	detail, err := env.settlements.RejectAccessorial(ctx, settlement.ID, item.ID, buyerActor(fix))
	if err != nil {
		t.Fatalf("reject accessorial: %v", err)
	}
	if detail.Settlement.ApprovedAccessorialTotal != 0 || detail.Settlement.TotalWithoutVAT != fix.AwardAmount {
		t.Fatalf("rejected included in totals: approved=%v total=%v", detail.Settlement.ApprovedAccessorialTotal, detail.Settlement.TotalWithoutVAT)
	}
}

func Test15DisputeBlocksApproval(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	settlement := createSettlement(t, env, fix, fix.ShipmentID, "eg-15-dispute-block")

	if _, err := env.settlements.SubmitForReview(ctx, settlement.ID, carrierActor(fix)); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := env.settlements.RaiseDispute(ctx, settlement.ID, domain.RaiseDisputeInput{
		SettlementActorInput: buyerActor(fix), Reason: "Rate mismatch",
	}); err != nil {
		t.Fatalf("raise dispute: %v", err)
	}
	_, err := env.settlements.Approve(ctx, settlement.ID, buyerActor(fix))
	assertValidation(t, err)
}

func Test16ResolutionAllowsProgression(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	settlement := createSettlement(t, env, fix, fix.ShipmentID, "eg-16-resolve")

	if _, err := env.settlements.SubmitForReview(ctx, settlement.ID, carrierActor(fix)); err != nil {
		t.Fatalf("submit: %v", err)
	}
	dispute, err := env.settlements.RaiseDispute(ctx, settlement.ID, domain.RaiseDisputeInput{
		SettlementActorInput: buyerActor(fix), Reason: "Needs clarification",
	})
	if err != nil {
		t.Fatalf("raise dispute: %v", err)
	}
	if _, err := env.settlements.ResolveDispute(ctx, settlement.ID, dispute.ID, domain.ResolveDisputeInput{
		SettlementActorInput: buyerActor(fix), ResolutionNote: "Clarified and accepted",
	}); err != nil {
		t.Fatalf("resolve dispute: %v", err)
	}
	approved, err := env.settlements.Approve(ctx, settlement.ID, buyerActor(fix))
	if err != nil {
		t.Fatalf("approve after resolution: %v", err)
	}
	if approved.Status != domain.SettlementStatusApproved {
		t.Fatalf("status=%s want APPROVED", approved.Status)
	}
}

func Test17RegisterIncludesSettlement(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	settlement := createSettlement(t, env, fix, fix.ShipmentID, "eg-17-register")
	submitAndApproveSettlement(t, env, fix, settlement.ID)

	included, err := env.settlements.IncludeInRegister(ctx, settlement.ID, buyerActor(fix), "BR-EG-17")
	if err != nil {
		t.Fatalf("include in register: %v", err)
	}
	if included.BillingRegisterID == nil || included.BillingRegisterItemID == nil {
		t.Fatal("register linkage not persisted on settlement")
	}
	if countRegisterItems(t, env.pool, *included.BillingRegisterID) != 1 {
		t.Fatal("register item not created")
	}
}

func Test18DuplicateRegisterInclusionPrevented(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	settlement := createSettlement(t, env, fix, fix.ShipmentID, "eg-18-dup-register")
	submitAndApproveSettlement(t, env, fix, settlement.ID)

	first, err := env.settlements.IncludeInRegister(ctx, settlement.ID, buyerActor(fix), "BR-EG-18")
	if err != nil {
		t.Fatalf("first include: %v", err)
	}
	second, err := env.settlements.IncludeInRegister(ctx, settlement.ID, buyerActor(fix), "BR-EG-18")
	if err != nil {
		t.Fatalf("second include: %v", err)
	}
	if first.BillingRegisterItemID == nil || second.BillingRegisterItemID == nil {
		t.Fatal("missing register item ids")
	}
	if *first.BillingRegisterItemID != *second.BillingRegisterItemID {
		t.Fatalf("duplicate inclusion created new item %s vs %s", first.BillingRegisterItemID, second.BillingRegisterItemID)
	}
	if countRegisterItems(t, env.pool, *first.BillingRegisterID) != 1 {
		t.Fatal("duplicate inclusion added extra register item")
	}
}

func Test19RegisterTotalServerDerived(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	settlement := createSettlement(t, env, fix, fix.ShipmentID, "eg-19-register-total")

	item, err := env.settlements.ProposeAccessorial(ctx, settlement.ID, domain.ProposeAccessorialInput{
		SettlementActorInput: carrierActor(fix), ChargeCode: "FUEL", Amount: 5000,
	})
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if _, err := env.settlements.ApproveAccessorial(ctx, settlement.ID, item.ID, buyerActor(fix)); err != nil {
		t.Fatalf("approve accessorial: %v", err)
	}
	submitAndApproveSettlement(t, env, fix, settlement.ID)

	included, err := env.settlements.IncludeInRegister(ctx, settlement.ID, buyerActor(fix), "BR-EG-19")
	if err != nil {
		t.Fatalf("include: %v", err)
	}
	withoutVAT, withVAT := scanRegisterTotals(t, env.pool, *included.BillingRegisterID)
	wantWithout := fix.AwardAmount + 5000
	if withoutVAT != wantWithout {
		t.Fatalf("register total_without_vat=%v want server-derived %v", withoutVAT, wantWithout)
	}
	if withVAT != wantWithout {
		t.Fatalf("register total_with_vat=%v want %v (no VAT configured)", withVAT, wantWithout)
	}
}

func Test20AuditFailureRollsBack(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlement := createSettlement(t, env, fix, fix.ShipmentID, "eg-20-audit-rollback")

	beforeStatus := scanSettlementStatus(t, env.pool, settlement.ID, fix.TenantID)
	beforeAudit := countAuditEvents(t, env.pool, settlement.ID)

	if err := env.repo.SimulateAuditFailureForTest(context.Background(), settlement.ID, fix.TenantID); err == nil {
		t.Fatal("expected simulated audit failure")
	}

	afterStatus := scanSettlementStatus(t, env.pool, settlement.ID, fix.TenantID)
	if afterStatus != beforeStatus {
		t.Fatalf("status rolled forward on audit failure: before=%s after=%s", beforeStatus, afterStatus)
	}
	if countAuditEvents(t, env.pool, settlement.ID) != beforeAudit {
		t.Fatal("audit failure did not roll back audit insert side effects")
	}
}

func Test21CompetitorReadDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlement := createSettlement(t, env, fix, fix.ShipmentID, "eg-21-competitor-read")

	_, err := env.settlements.GetDetail(context.Background(), settlement.ID, fix.TenantID, fix.CarrierB, domain.SettlementActorCarrier)
	assertForbidden(t, err)
}

func Test22DocumentAssociationTenantSafe(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlement := createSettlement(t, env, fix, fix.ShipmentID, "eg-22-doc-tenant")

	otherTenantDoc := seedPODDocument(t, env.pool, fix.OtherTenantID, fix.CarrierA, fix.ShipmentID, "POD-OTHER-TENANT")
	_, err := env.settlements.ProposeAccessorial(context.Background(), settlement.ID, domain.ProposeAccessorialInput{
		SettlementActorInput: carrierActor(fix),
		ChargeCode:           "DETENTION", Amount: 1500,
		EvidenceDocumentID:   &otherTenantDoc,
	})
	assertForbidden(t, err)

	foreignShipment := uuid.New()
	foreignDoc := seedPODDocument(t, env.pool, fix.TenantID, fix.CarrierA, foreignShipment, "POD-FOREIGN-SHIP")
	_, err = env.settlements.ProposeAccessorial(context.Background(), settlement.ID, domain.ProposeAccessorialInput{
		SettlementActorInput: carrierActor(fix),
		ChargeCode:           "DETENTION", Amount: 1500,
		EvidenceDocumentID:   &foreignDoc,
	})
	assertForbidden(t, err)
}

func Test23CompetitorFinancialIsolation(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	settlement := createSettlement(t, env, fix, fix.ShipmentID, "eg-23-financial-isolation")

	carrierBFilter := fix.CarrierB
	items, total, err := env.settlements.List(ctx, domain.ListFreightSettlementsFilter{
		TenantID: fix.TenantID, CarrierCompanyID: &carrierBFilter, Limit: 20,
	}, domain.SettlementActorInput{
		TenantID: fix.TenantID, ActorCompanyID: fix.CarrierB,
		ActorKind: domain.SettlementActorCarrier, ActorUserID: fix.UserID,
	})
	if err != nil {
		t.Fatalf("list carrier B: %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Fatalf("competitor carrier must not see settlements: total=%d len=%d", total, len(items))
	}

	buyerFilter := fix.BuyerID
	_, _, err = env.settlements.List(ctx, domain.ListFreightSettlementsFilter{
		TenantID: fix.TenantID, BuyerCompanyID: &buyerFilter, Limit: 20,
	}, domain.SettlementActorInput{
		TenantID: fix.TenantID, ActorCompanyID: fix.CarrierB,
		ActorKind: domain.SettlementActorCarrier, ActorUserID: fix.UserID,
	})
	assertForbidden(t, err)

	_, err = env.settlements.GetDetail(ctx, settlement.ID, fix.TenantID, fix.CarrierB, domain.SettlementActorCarrier)
	assertForbidden(t, err)

	_, err = env.settlements.Approve(ctx, settlement.ID, domain.SettlementActorInput{
		TenantID: fix.TenantID, ActorCompanyID: fix.ForeignBuyer,
		ActorKind: domain.SettlementActorBuyer, ActorUserID: fix.BuyerUserID,
	})
	assertForbidden(t, err)
}
