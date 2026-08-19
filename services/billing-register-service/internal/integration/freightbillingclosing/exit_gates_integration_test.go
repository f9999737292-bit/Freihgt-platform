//go:build integration

package freightbillingclosing

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

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

func Test01AuthorizedBuyerRegisterCreation(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := createRegister(t, env, fix, "BR-BC-01")
	if reg.CustomerCompanyID != fix.BuyerID || reg.ContractorCompanyID != fix.CarrierA {
		t.Fatal("register parties mismatch")
	}
	if reg.Status != domain.RegisterStatusDraft {
		t.Fatalf("status=%s want DRAFT", reg.Status)
	}
}

func Test02EligibleSettlementInclusion(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlement := approvedSettlement(t, env, fix, "bc-02")
	reg := createRegister(t, env, fix, "BR-BC-02")
	result := includeSettlement(t, env, fix, reg.ID, settlement.ID)
	if result.Item.SettlementID == nil || *result.Item.SettlementID != settlement.ID {
		t.Fatal("settlement_id not persisted on register item")
	}
}

func Test03ServerDerivedAmountVerification(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	settlement := createSettlement(t, env, fix, "bc-03")
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
	reg := createRegister(t, env, fix, "BR-BC-03")
	result := includeSettlement(t, env, fix, reg.ID, settlement.ID)
	want := fix.AwardAmount + 5000
	if result.Item.BaseAmount != fix.AwardAmount || result.Item.ExtraCharges != 5000 {
		t.Fatalf("item amounts not server-derived: base=%v extra=%v", result.Item.BaseAmount, result.Item.ExtraCharges)
	}
	if result.Register.TotalWithoutVAT != want {
		t.Fatalf("register total=%v want %v", result.Register.TotalWithoutVAT, want)
	}
}

func Test04WrongSettlementStateDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlement := createSettlement(t, env, fix, "bc-04")
	reg := createRegister(t, env, fix, "BR-BC-04")
	_, err := env.registers.IncludeSettlement(context.Background(), reg.ID, settlement.ID, buyerActor(fix))
	assertValidation(t, err)
}

func Test05OpenDisputeDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	settlement := createSettlement(t, env, fix, "bc-05")
	if _, err := env.settlements.SubmitForReview(ctx, settlement.ID, carrierActor(fix)); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := env.settlements.RaiseDispute(ctx, settlement.ID, domain.RaiseDisputeInput{
		SettlementActorInput: buyerActor(fix), Reason: "Dispute open",
	}); err != nil {
		t.Fatalf("raise dispute: %v", err)
	}
	reg := createRegister(t, env, fix, "BR-BC-05")
	_, err := env.registers.IncludeSettlement(ctx, reg.ID, settlement.ID, buyerActor(fix))
	assertValidation(t, err)
}

func Test06CrossTenantInclusionDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlement := approvedSettlement(t, env, fix, "bc-06")
	reg := createRegister(t, env, fix, "BR-BC-06")
	wrongTenant := buyerActor(fix)
	wrongTenant.TenantID = fix.OtherTenantID
	_, err := env.registers.IncludeSettlement(context.Background(), reg.ID, settlement.ID, wrongTenant)
	if err == nil {
		t.Fatal("expected error for cross-tenant inclusion")
	}
}

func Test07CrossCompanyInclusionDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlement := approvedSettlement(t, env, fix, "bc-07")
	regInput := createRegisterInput(fix, "BR-BC-07")
	regInput.ContractorCompanyID = fix.CarrierB
	reg, err := env.registers.CreateForActor(context.Background(), regInput, buyerActor(fix))
	if err != nil {
		t.Fatalf("create register: %v", err)
	}
	_, err = env.registers.IncludeSettlement(context.Background(), reg.ID, settlement.ID, buyerActor(fix))
	assertForbidden(t, err)
}

func Test08UnrelatedBuyerDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := createRegister(t, env, fix, "BR-BC-08")
	_, err := env.registers.GetByID(context.Background(), reg.ID, fix.TenantID, foreignBuyerActor(fix))
	assertForbidden(t, err)
}

func Test09UnrelatedCarrierDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := createRegister(t, env, fix, "BR-BC-09")
	_, err := env.registers.GetByID(context.Background(), reg.ID, fix.TenantID, carrierBActor(fix))
	assertForbidden(t, err)
}

func Test10SpoofedTenantDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := createRegister(t, env, fix, "BR-BC-10")
	settlement := approvedSettlement(t, env, fix, "bc-10")
	spoofed := buyerActor(fix)
	spoofed.TenantID = fix.OtherTenantID
	_, err := env.registers.IncludeSettlement(context.Background(), reg.ID, settlement.ID, spoofed)
	if err == nil {
		t.Fatal("expected tenant mismatch error")
	}
}

func Test11SpoofedCompanyDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := createRegister(t, env, fix, "BR-BC-11")
	spoofed := buyerActor(fix)
	spoofed.ActorCompanyID = fix.ForeignBuyer
	_, err := env.registers.GetByID(context.Background(), reg.ID, fix.TenantID, spoofed)
	assertForbidden(t, err)
}

func Test12DuplicateSettlementInclusionIdempotent(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlement := approvedSettlement(t, env, fix, "bc-12")
	reg := createRegister(t, env, fix, "BR-BC-12")
	first := includeSettlement(t, env, fix, reg.ID, settlement.ID)
	second, err := env.registers.IncludeSettlement(context.Background(), reg.ID, settlement.ID, buyerActor(fix))
	if err != nil {
		t.Fatalf("retry include: %v", err)
	}
	if first.Item.ID != second.Item.ID {
		t.Fatal("idempotent retry changed item id")
	}
	if countRegisterItems(t, env.pool, reg.ID) != 1 {
		t.Fatal("duplicate inclusion created extra item")
	}
}

func Test13ConcurrentSettlementInclusion(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlement := approvedSettlement(t, env, fix, "bc-13")
	reg := createRegister(t, env, fix, "BR-BC-13")
	const workers = 8
	var wg sync.WaitGroup
	errs := make([]error, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = env.registers.IncludeSettlement(context.Background(), reg.ID, settlement.ID, buyerActor(fix))
		}(i)
	}
	wg.Wait()
	success := 0
	for _, err := range errs {
		if err == nil {
			success++
		}
	}
	if success == 0 {
		t.Fatalf("all concurrent includes failed: %v", errs)
	}
	if countRegisterItems(t, env.pool, reg.ID) != 1 {
		t.Fatal("concurrent inclusion created duplicate items")
	}
}

func Test14MixedCurrencyInclusionDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlement := approvedSettlement(t, env, fix, "bc-14")
	regInput := createRegisterInput(fix, "BR-BC-14")
	regInput.CurrencyCode = "USD"
	reg, err := env.registers.CreateForActor(context.Background(), regInput, buyerActor(fix))
	if err != nil {
		t.Fatalf("create register: %v", err)
	}
	_, err = env.registers.IncludeSettlement(context.Background(), reg.ID, settlement.ID, buyerActor(fix))
	assertValidation(t, err)
}

func Test15CalculateTotals(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlement := approvedSettlement(t, env, fix, "bc-15")
	reg := createRegister(t, env, fix, "BR-BC-15")
	includeSettlement(t, env, fix, reg.ID, settlement.ID)
	calculated, err := env.registers.Calculate(context.Background(), reg.ID, buyerActor(fix))
	if err != nil {
		t.Fatalf("calculate: %v", err)
	}
	if calculated.Status != domain.RegisterStatusCalculated {
		t.Fatalf("status=%s want CALCULATED", calculated.Status)
	}
	if calculated.TotalWithoutVAT != fix.AwardAmount {
		t.Fatalf("total=%v want %v", calculated.TotalWithoutVAT, fix.AwardAmount)
	}
}

func Test16ApprovalTransition(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlement := approvedSettlement(t, env, fix, "bc-16")
	reg := createRegister(t, env, fix, "BR-BC-16")
	includeSettlement(t, env, fix, reg.ID, settlement.ID)
	if _, err := env.registers.Calculate(context.Background(), reg.ID, buyerActor(fix)); err != nil {
		t.Fatalf("calculate: %v", err)
	}
	approved, err := env.registers.Approve(context.Background(), reg.ID, buyerActor(fix))
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if approved.Status != domain.RegisterStatusApproved {
		t.Fatalf("status=%s want APPROVED", approved.Status)
	}
}

func Test17IllegalApprovalTransitionDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := createRegister(t, env, fix, "BR-BC-17")
	_, err := env.registers.Approve(context.Background(), reg.ID, buyerActor(fix))
	assertValidation(t, err)
}

func Test18ClosingPackageCreation(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlement := approvedSettlement(t, env, fix, "bc-18")
	reg := createRegister(t, env, fix, "BR-BC-18")
	includeSettlement(t, env, fix, reg.ID, settlement.ID)
	if _, err := env.registers.Calculate(context.Background(), reg.ID, buyerActor(fix)); err != nil {
		t.Fatalf("calculate: %v", err)
	}
	if _, err := env.registers.Approve(context.Background(), reg.ID, buyerActor(fix)); err != nil {
		t.Fatalf("approve: %v", err)
	}
	pkgResult, err := env.closing.CreatePackageForActor(context.Background(), reg.ID, domain.CreateClosingDocumentPackageInput{
		TenantID: fix.TenantID, PackageNumber: "PKG-BC-18", PackageType: domain.ClosingPackageTypeActPlusVATInvoice,
	}, buyerActor(fix))
	if err != nil {
		t.Fatalf("create package: %v", err)
	}
	if pkgResult.Package.RegisterID != reg.ID {
		t.Fatal("package register mismatch")
	}
}

func Test19DuplicateClosingPackageRetry(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlement := approvedSettlement(t, env, fix, "bc-19")
	reg := createRegister(t, env, fix, "BR-BC-19")
	includeSettlement(t, env, fix, reg.ID, settlement.ID)
	if _, err := env.registers.Calculate(context.Background(), reg.ID, buyerActor(fix)); err != nil {
		t.Fatalf("calculate: %v", err)
	}
	if _, err := env.registers.Approve(context.Background(), reg.ID, buyerActor(fix)); err != nil {
		t.Fatalf("approve: %v", err)
	}
	in := domain.CreateClosingDocumentPackageInput{TenantID: fix.TenantID, PackageNumber: "PKG-BC-19", PackageType: domain.ClosingPackageTypeCustom}
	first, err := env.closing.CreatePackageForActor(context.Background(), reg.ID, in, buyerActor(fix))
	if err != nil {
		t.Fatalf("first package: %v", err)
	}
	second, err := env.closing.CreatePackageForActor(context.Background(), reg.ID, in, buyerActor(fix))
	if err != nil {
		t.Fatalf("retry package: %v", err)
	}
	if first.Package.ID != second.Package.ID {
		t.Fatal("expected same canonical package on retry")
	}
	if countPackages(t, env.pool, reg.ID) != 1 {
		t.Fatal("retry created duplicate package row")
	}
}

func Test20InvoicePartyDerivation(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlement := approvedSettlement(t, env, fix, "bc-20")
	reg := createRegister(t, env, fix, "BR-BC-20")
	includeSettlement(t, env, fix, reg.ID, settlement.ID)
	if _, err := env.registers.Calculate(context.Background(), reg.ID, buyerActor(fix)); err != nil {
		t.Fatalf("calculate: %v", err)
	}
	if _, err := env.registers.Approve(context.Background(), reg.ID, buyerActor(fix)); err != nil {
		t.Fatalf("approve: %v", err)
	}
	inv, err := env.closing.CreateInvoice(context.Background(), reg.ID, domain.CreateInvoiceInput{
		InvoiceNumber: "INV-BC-20", InvoiceDate: time.Now(),
		SellerCompanyID: fix.CarrierA, BuyerCompanyID: fix.BuyerID,
	}, buyerActor(fix))
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	if inv.SellerCompanyID != fix.CarrierA || inv.BuyerCompanyID != fix.BuyerID {
		t.Fatal("invoice parties mismatch")
	}
}

func Test21DocumentAmountDerivation(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlement := approvedSettlement(t, env, fix, "bc-21")
	reg := createRegister(t, env, fix, "BR-BC-21")
	includeSettlement(t, env, fix, reg.ID, settlement.ID)
	calculated, err := env.registers.Calculate(context.Background(), reg.ID, buyerActor(fix))
	if err != nil {
		t.Fatalf("calculate: %v", err)
	}
	if _, err := env.registers.Approve(context.Background(), reg.ID, buyerActor(fix)); err != nil {
		t.Fatalf("approve: %v", err)
	}
	inv, err := env.closing.CreateInvoice(context.Background(), reg.ID, domain.CreateInvoiceInput{
		InvoiceNumber: "INV-BC-21", InvoiceDate: time.Now(),
		SellerCompanyID: fix.CarrierA, BuyerCompanyID: fix.BuyerID,
	}, buyerActor(fix))
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	if inv.TotalAmount != calculated.TotalWithVAT {
		t.Fatalf("invoice total=%v register total=%v", inv.TotalAmount, calculated.TotalWithVAT)
	}
}

func Test22CrossCompanyRegisterListDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	createRegister(t, env, fix, "BR-BC-22")
	foreignFilter := fix.ForeignBuyer
	_, _, err := env.registers.List(context.Background(), domain.ListBillingRegistersFilter{
		TenantID: fix.TenantID, CustomerCompanyID: &foreignFilter, Limit: 20,
	}, buyerActor(fix))
	assertForbidden(t, err)
}

func Test23CrossCompanyRegisterDetailDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := createRegister(t, env, fix, "BR-BC-23")
	_, err := env.registers.GetByID(context.Background(), reg.ID, fix.TenantID, carrierBActor(fix))
	assertForbidden(t, err)
}

func Test24AuditPersistence(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlement := approvedSettlement(t, env, fix, "bc-24")
	reg := createRegister(t, env, fix, "BR-BC-24")
	before := countRegisterAuditEvents(t, env.pool, reg.ID)
	includeSettlement(t, env, fix, reg.ID, settlement.ID)
	after := countRegisterAuditEvents(t, env.pool, reg.ID)
	if after <= before {
		t.Fatal("settlement inclusion did not persist register audit event")
	}
}

func Test25AuditRollback(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := createRegister(t, env, fix, "BR-BC-25")
	beforeStatus := scanRegisterStatus(t, env.pool, reg.ID)
	beforeAudit := countRegisterAuditEvents(t, env.pool, reg.ID)
	if err := env.registerRepo.SimulateRegisterAuditFailureForTest(context.Background(), reg.ID, fix.TenantID); err == nil {
		t.Fatal("expected simulated audit failure")
	}
	if scanRegisterStatus(t, env.pool, reg.ID) != beforeStatus {
		t.Fatal("audit failure did not roll back register status change")
	}
	if countRegisterAuditEvents(t, env.pool, reg.ID) != beforeAudit {
		t.Fatal("audit failure did not roll back audit insert")
	}
}

func Test26MarkSignedTransitionValidation(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := createRegister(t, env, fix, "BR-BC-26")
	_, err := env.registers.MarkSigned(context.Background(), reg.ID, buyerActor(fix))
	assertValidation(t, err)
}

func Test27MarkPaidTransitionValidation(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := createRegister(t, env, fix, "BR-BC-27")
	_, err := env.registers.MarkPaid(context.Background(), reg.ID, buyerActor(fix))
	assertValidation(t, err)
}

func Test28CloseTransitionValidation(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := createRegister(t, env, fix, "BR-BC-28")
	_, err := env.registers.Close(context.Background(), reg.ID, buyerActor(fix))
	assertValidation(t, err)
}

func TestCarrierCannotIncludeSettlement(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlement := approvedSettlement(t, env, fix, "bc-carrier-include")
	reg := createRegister(t, env, fix, "BR-BC-CARRIER")
	_, err := env.registers.IncludeSettlement(context.Background(), reg.ID, settlement.ID, carrierActor(fix))
	assertForbidden(t, err)
}

func TestBuyerCannotCreateRegisterForAnotherBuyer(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	input := createRegisterInput(fix, "BR-BC-FOREIGN-BUYER")
	input.CustomerCompanyID = fix.ForeignBuyer
	_, err := env.registers.CreateForActor(context.Background(), input, buyerActor(fix))
	assertForbidden(t, err)
}
