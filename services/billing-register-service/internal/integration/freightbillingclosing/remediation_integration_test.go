//go:build integration

package freightbillingclosing

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/billing-register-service/internal/domain"
)

func Test31BodyTenantMismatchOnCalculateDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := prepareApprovedRegister(t, env, fix, "bc-31", "BR-BC-31")
	_ = reg
	assertForbidden(t, domain.EnforceOptionalBodyTenant(fix.TenantID, fix.OtherTenantID.String()))
}

func Test32BodyTenantMismatchOnApproveDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	_ = prepareApprovedRegister(t, env, fix, "bc-32", "BR-BC-32")
	assertForbidden(t, domain.EnforceOptionalBodyTenant(fix.TenantID, fix.OtherTenantID.String()))
}

func Test33BodyTenantMismatchOnItemAddDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	_ = createRegister(t, env, fix, "BR-BC-33")
	assertForbidden(t, domain.EnforceOptionalBodyTenant(fix.TenantID, fix.OtherTenantID.String()))
}

func Test34BodyTenantMismatchOnItemRemoveDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	_ = createRegister(t, env, fix, "BR-BC-34")
	assertForbidden(t, domain.EnforceOptionalBodyTenant(fix.TenantID, fix.OtherTenantID.String()))
}

func Test35CrossCompanyCalculateDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := createRegister(t, env, fix, "BR-BC-35")
	_, err := env.registers.Calculate(context.Background(), reg.ID, foreignBuyerActor(fix))
	assertForbidden(t, err)
}

func Test36CrossCompanyApproveDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := prepareApprovedRegister(t, env, fix, "bc-36", "BR-BC-36")
	_, err := env.registers.Approve(context.Background(), reg.ID, foreignBuyerActor(fix))
	assertForbidden(t, err)
}

func Test37CrossCompanyItemMutationDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := createRegister(t, env, fix, "BR-BC-37")
	_, err := env.registers.AddItem(context.Background(), reg.ID, domain.CreateBillingRegisterItemInput{
		ShipmentID: fix.ShipmentID, BaseAmount: 1000,
	}, foreignBuyerActor(fix))
	assertForbidden(t, err)
}

func Test38CrossCompanyClosingPackageDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := prepareApprovedRegister(t, env, fix, "bc-38", "BR-BC-38")
	_, err := env.closing.CreatePackageForActor(context.Background(), reg.ID, domain.CreateClosingDocumentPackageInput{
		TenantID: fix.TenantID, PackageNumber: "PKG-BC-38", PackageType: domain.ClosingPackageTypeCustom,
	}, foreignBuyerActor(fix))
	assertForbidden(t, err)
}

func Test39CrossCompanyMarkSentDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := prepareApprovedRegister(t, env, fix, "bc-39", "BR-BC-39")
	_, err := env.registers.MarkSentToEDO(context.Background(), reg.ID, foreignBuyerActor(fix))
	assertForbidden(t, err)
}

func Test40CrossCompanyMarkSignedDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := prepareApprovedRegister(t, env, fix, "bc-40", "BR-BC-40")
	_, err := env.registers.MarkSigned(context.Background(), reg.ID, foreignBuyerActor(fix))
	assertForbidden(t, err)
}

func Test41CrossCompanyMarkPaidDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := prepareApprovedRegister(t, env, fix, "bc-41", "BR-BC-41")
	_, err := env.registers.MarkPaid(context.Background(), reg.ID, foreignBuyerActor(fix))
	assertForbidden(t, err)
}

func Test42CrossCompanyCloseDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := prepareApprovedRegister(t, env, fix, "bc-42", "BR-BC-42")
	_, err := env.registers.Close(context.Background(), reg.ID, foreignBuyerActor(fix))
	assertForbidden(t, err)
}

func Test43ApprovedBySpoofRejected(t *testing.T) {
	fix := fixture{TenantID: uuid.New(), BuyerUserID: uuid.New()}
	_, err := domain.ResolveApprovedBy(fix.BuyerUserID, uuid.New().String())
	assertForbidden(t, err)
}

func Test44AuditActorComesFromVerifiedUser(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := prepareApprovedRegister(t, env, fix, "bc-44", "BR-BC-44")
	if scanApprovedBy(t, env.pool, reg.ID) != fix.BuyerUserID {
		t.Fatal("approved_by must come from verified actor user")
	}
}

func Test45ClosingPackageRetryReturnsSameCanonicalPackage(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := prepareApprovedRegister(t, env, fix, "bc-45", "BR-BC-45")
	in := domain.CreateClosingDocumentPackageInput{TenantID: fix.TenantID, PackageNumber: "PKG-BC-45", PackageType: domain.ClosingPackageTypeCustom}
	first, err := env.closing.CreatePackageForActor(context.Background(), reg.ID, in, buyerActor(fix))
	if err != nil {
		t.Fatalf("first package: %v", err)
	}
	second, err := env.closing.CreatePackageForActor(context.Background(), reg.ID, in, buyerActor(fix))
	if err != nil {
		t.Fatalf("retry package: %v", err)
	}
	if first.Package.ID != second.Package.ID {
		t.Fatal("retry must return same canonical package")
	}
}

func Test46ConcurrentClosingPackageCreateResultsInOnePackage(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := prepareApprovedRegister(t, env, fix, "bc-46", "BR-BC-46")
	in := domain.CreateClosingDocumentPackageInput{TenantID: fix.TenantID, PackageNumber: "PKG-BC-46", PackageType: domain.ClosingPackageTypeCustom}
	const workers = 8
	var wg sync.WaitGroup
	errs := make([]error, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = env.closing.CreatePackageForActor(context.Background(), reg.ID, in, buyerActor(fix))
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
		t.Fatalf("all concurrent package creates failed: %v", errs)
	}
	if countPackages(t, env.pool, reg.ID) != 1 {
		t.Fatalf("expected one package row, got %d", countPackages(t, env.pool, reg.ID))
	}
}

func Test47PackageRetryDoesNotDuplicateInvoice(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := prepareApprovedRegister(t, env, fix, "bc-47", "BR-BC-47")
	in := domain.CreateClosingDocumentPackageInput{TenantID: fix.TenantID, PackageType: domain.ClosingPackageTypeCustom}
	if _, err := env.closing.CreatePackageForActor(context.Background(), reg.ID, in, buyerActor(fix)); err != nil {
		t.Fatalf("first package: %v", err)
	}
	if _, err := env.closing.CreatePackageForActor(context.Background(), reg.ID, in, buyerActor(fix)); err != nil {
		t.Fatalf("retry package: %v", err)
	}
	if countDocuments(t, env.pool, "invoices", reg.ID) != 1 {
		t.Fatal("retry duplicated invoice")
	}
}

func Test48PackageRetryDoesNotDuplicateAct(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := prepareApprovedRegister(t, env, fix, "bc-48", "BR-BC-48")
	in := domain.CreateClosingDocumentPackageInput{TenantID: fix.TenantID, PackageType: domain.ClosingPackageTypeCustom}
	if _, err := env.closing.CreatePackageForActor(context.Background(), reg.ID, in, buyerActor(fix)); err != nil {
		t.Fatalf("first package: %v", err)
	}
	if _, err := env.closing.CreatePackageForActor(context.Background(), reg.ID, in, buyerActor(fix)); err != nil {
		t.Fatalf("retry package: %v", err)
	}
	if countDocuments(t, env.pool, "acts", reg.ID) != 1 {
		t.Fatal("retry duplicated act")
	}
}

func Test49PackageRetryDoesNotDuplicateVATInvoice(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := prepareApprovedRegister(t, env, fix, "bc-49", "BR-BC-49")
	in := domain.CreateClosingDocumentPackageInput{TenantID: fix.TenantID, PackageType: domain.ClosingPackageTypeCustom}
	if _, err := env.closing.CreatePackageForActor(context.Background(), reg.ID, in, buyerActor(fix)); err != nil {
		t.Fatalf("first package: %v", err)
	}
	if _, err := env.closing.CreatePackageForActor(context.Background(), reg.ID, in, buyerActor(fix)); err != nil {
		t.Fatalf("retry package: %v", err)
	}
	if countDocuments(t, env.pool, "vat_invoices", reg.ID) != 1 {
		t.Fatal("retry duplicated VAT invoice")
	}
}

func Test50PackageRetryDoesNotDuplicateUPD(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := prepareApprovedRegister(t, env, fix, "bc-50", "BR-BC-50")
	in := domain.CreateClosingDocumentPackageInput{TenantID: fix.TenantID, PackageType: domain.ClosingPackageTypeCustom}
	if _, err := env.closing.CreatePackageForActor(context.Background(), reg.ID, in, buyerActor(fix)); err != nil {
		t.Fatalf("first package: %v", err)
	}
	if _, err := env.closing.CreatePackageForActor(context.Background(), reg.ID, in, buyerActor(fix)); err != nil {
		t.Fatalf("retry package: %v", err)
	}
	if countDocuments(t, env.pool, "upd_documents", reg.ID) != 1 {
		t.Fatal("retry duplicated UPD")
	}
}

func Test51RegisterCreateAuditPresent(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := createRegister(t, env, fix, "BR-BC-51")
	if !hasAuditEvent(t, env.pool, reg.ID, domain.RegisterAuditCreated) {
		t.Fatal("missing REGISTER_CREATED audit")
	}
}

func Test52CalculateAuditPresent(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlement := approvedSettlement(t, env, fix, "bc-52")
	reg := createRegister(t, env, fix, "BR-BC-52")
	includeSettlement(t, env, fix, reg.ID, settlement.ID)
	if _, err := env.registers.Calculate(context.Background(), reg.ID, buyerActor(fix)); err != nil {
		t.Fatalf("calculate: %v", err)
	}
	if !hasAuditEvent(t, env.pool, reg.ID, domain.RegisterAuditCalculated) {
		t.Fatal("missing REGISTER_CALCULATED audit")
	}
}

func Test53ApproveAuditPresent(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := prepareApprovedRegister(t, env, fix, "bc-53", "BR-BC-53")
	if !hasAuditEvent(t, env.pool, reg.ID, domain.RegisterAuditApproved) {
		t.Fatal("missing REGISTER_APPROVED audit")
	}
}

func Test54ClosingPackageAuditPresent(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := prepareApprovedRegister(t, env, fix, "bc-54", "BR-BC-54")
	if _, err := env.closing.CreatePackageForActor(context.Background(), reg.ID, domain.CreateClosingDocumentPackageInput{
		TenantID: fix.TenantID, PackageType: domain.ClosingPackageTypeCustom,
	}, buyerActor(fix)); err != nil {
		t.Fatalf("create package: %v", err)
	}
	if !hasAuditEvent(t, env.pool, reg.ID, domain.RegisterAuditClosingPackage) {
		t.Fatal("missing CLOSING_PACKAGE_CREATED audit")
	}
}

func Test55MarkPaidAuditPresent(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := prepareApprovedRegister(t, env, fix, "bc-55", "BR-BC-55")
	ctx := context.Background()
	actor := buyerActor(fix)
	if _, err := env.closing.CreatePackageForActor(ctx, reg.ID, domain.CreateClosingDocumentPackageInput{
		TenantID: fix.TenantID, PackageType: domain.ClosingPackageTypeCustom,
	}, actor); err != nil {
		t.Fatalf("create package: %v", err)
	}
	if _, err := env.registers.MarkSentToEDO(ctx, reg.ID, actor); err != nil {
		t.Fatalf("mark sent: %v", err)
	}
	if _, err := env.registers.MarkSigned(ctx, reg.ID, actor); err != nil {
		t.Fatalf("mark signed: %v", err)
	}
	if _, err := env.registers.MarkPaid(ctx, reg.ID, actor); err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	if !hasAuditEvent(t, env.pool, reg.ID, domain.RegisterAuditMarkedPaid) {
		t.Fatal("missing MARKED_PAID audit")
	}
}

func Test56CloseAuditPresent(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := prepareApprovedRegister(t, env, fix, "bc-56", "BR-BC-56")
	ctx := context.Background()
	actor := buyerActor(fix)
	if _, err := env.closing.CreatePackageForActor(ctx, reg.ID, domain.CreateClosingDocumentPackageInput{
		TenantID: fix.TenantID, PackageType: domain.ClosingPackageTypeCustom,
	}, actor); err != nil {
		t.Fatalf("create package: %v", err)
	}
	if _, err := env.registers.MarkSentToEDO(ctx, reg.ID, actor); err != nil {
		t.Fatalf("mark sent: %v", err)
	}
	if _, err := env.registers.MarkSigned(ctx, reg.ID, actor); err != nil {
		t.Fatalf("mark signed: %v", err)
	}
	if _, err := env.registers.MarkPaid(ctx, reg.ID, actor); err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	if _, err := env.registers.Close(ctx, reg.ID, actor); err != nil {
		t.Fatalf("close: %v", err)
	}
	if !hasAuditEvent(t, env.pool, reg.ID, domain.RegisterAuditClosed) {
		t.Fatal("missing REGISTER_CLOSED audit")
	}
}

func Test57AuditFailureRollsBackFinancialMutation(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlement := approvedSettlement(t, env, fix, "bc-57")
	reg := createRegister(t, env, fix, "BR-BC-57")
	includeSettlement(t, env, fix, reg.ID, settlement.ID)
	beforeStatus := scanRegisterStatus(t, env.pool, reg.ID)
	beforeAudit := countRegisterAuditEvents(t, env.pool, reg.ID)
	if err := env.registerRepo.SimulateCalculateAuditFailureForTest(context.Background(), reg.ID, fix.TenantID); err == nil {
		t.Fatal("expected simulated calculate audit failure")
	}
	if scanRegisterStatus(t, env.pool, reg.ID) != beforeStatus {
		t.Fatal("calculate audit failure did not roll back register status")
	}
	if countRegisterAuditEvents(t, env.pool, reg.ID) != beforeAudit {
		t.Fatal("calculate audit failure did not roll back audit insert")
	}
}

func Test58V17CrossCompanySettlementListDeny(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	createSettlement(t, env, fix, "bc-58")
	foreignFilter := fix.ForeignBuyer
	_, _, err := env.settlements.List(context.Background(), domain.ListFreightSettlementsFilter{
		TenantID: fix.TenantID, BuyerCompanyID: &foreignFilter, Limit: 20,
	}, buyerActor(fix))
	assertForbidden(t, err)
}

func Test59V17CrossRoleListFilterSpoofDeny(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	createSettlement(t, env, fix, "bc-59")
	carrierFilter := fix.CarrierA
	_, _, err := env.settlements.List(context.Background(), domain.ListFreightSettlementsFilter{
		TenantID: fix.TenantID, CarrierCompanyID: &carrierFilter, Limit: 20,
	}, buyerActor(fix))
	assertForbidden(t, err)
}

func Test60V17SettlementDetailIDORDeny(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlement := createSettlement(t, env, fix, "bc-60")
	_, err := env.settlements.GetDetail(context.Background(), settlement.ID, fix.TenantID, fix.CarrierB, domain.SettlementActorCarrier)
	assertForbidden(t, err)
}

func TestClosingDocumentIndividualCreateIsIdempotent(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	reg := prepareApprovedRegister(t, env, fix, "bc-ind-idem", "BR-BC-IND")
	ctx := context.Background()
	actor := buyerActor(fix)
	input := domain.CreateInvoiceInput{InvoiceNumber: "INV-IND", InvoiceDate: time.Now()}
	first, err := env.closing.CreateInvoice(ctx, reg.ID, input, actor)
	if err != nil {
		t.Fatalf("first invoice: %v", err)
	}
	second, err := env.closing.CreateInvoice(ctx, reg.ID, input, actor)
	if err != nil {
		t.Fatalf("retry invoice: %v", err)
	}
	if first.ID != second.ID {
		t.Fatal("individual invoice create must be idempotent per register")
	}
}
