//go:build integration

package contractrate

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/freight-platform/contract-rate-service/internal/domain"
	apperrors "github.com/freight-platform/contract-rate-service/internal/platform/errors"
)

func TestCRPG001_CreateDraftContract(t *testing.T) {
	env := setupEnv(t)
	created := env.createDraftContract(t, "CR-PG-001")
	if created.Status != domain.ContractStatusDraft {
		t.Fatalf("expected DRAFT, got %s", created.Status)
	}
}

func TestCRPG002_DuplicateContractNumberSameTenantBuyer(t *testing.T) {
	env := setupEnv(t)
	_ = env.createDraftContract(t, "CR-PG-002")
	_, err := env.Contracts.Create(context.Background(), domain.CreateContractInput{
		TenantID: env.TenantID, BuyerCompanyID: env.BuyerID, CarrierCompanyID: env.CarrierID,
		ContractNumber: "CR-PG-002", Name: "Dup", ValidFrom: env.Today,
		CurrencyCode: "RUB", Actor: env.Actor,
	}, nil)
	if !isAppErrorCode(err, apperrors.CodeConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestCRPG003_SameContractNumberDifferentTenant(t *testing.T) {
	env := setupEnv(t)
	_ = env.createDraftContract(t, "CR-PG-003")

	otherTenant := uuid.New()
	otherBuyer := uuid.New()
	otherCarrier := uuid.New()
	ctx := context.Background()
	seedTenantOnly(t, ctx, env.Pool, otherTenant)
	for _, row := range []struct {
		id, tenant uuid.UUID
		typ, name  string
	}{
		{otherBuyer, otherTenant, "SHIPPER", "Other Buyer"},
		{otherCarrier, otherTenant, "CARRIER", "Other Carrier"},
	} {
		_, err := env.Pool.Exec(ctx, `
			INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status)
			VALUES ($1,$2,$3,$4,'ACTIVE')`, row.id, row.tenant, row.typ, row.name)
		if err != nil {
			t.Fatalf("seed other company: %v", err)
		}
	}
	otherActor := domain.ActorInput{
		TenantID: otherTenant, ActorUserID: uuid.New(),
		ActorCompanyID: otherBuyer, ActorKind: domain.ActorKindBuyer,
	}
	created, err := env.Contracts.Create(ctx, domain.CreateContractInput{
		TenantID: otherTenant, BuyerCompanyID: otherBuyer, CarrierCompanyID: otherCarrier,
		ContractNumber: "CR-PG-003", Name: "Other tenant", ValidFrom: env.Today,
		CurrencyCode: "RUB", Actor: otherActor,
	}, nil)
	if err != nil {
		t.Fatalf("expected allow, got %v", err)
	}
	if created.ContractNumber != "CR-PG-003" {
		t.Fatalf("unexpected contract number %s", created.ContractNumber)
	}
}

func TestCRPG004_ActivateDraft(t *testing.T) {
	env := setupEnv(t)
	draft := env.createDraftContract(t, "CR-PG-004")
	activated, err := env.Contracts.Activate(context.Background(), env.TenantID, draft.ID, env.Actor, nil)
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	if activated.Status != domain.ContractStatusActive {
		t.Fatalf("expected ACTIVE, got %s", activated.Status)
	}
}

func TestCRPG005_RepeatActivateIdempotent(t *testing.T) {
	env := setupEnv(t)
	draft := env.createDraftContract(t, "CR-PG-005")
	ctx := context.Background()
	first, err := env.Contracts.Activate(ctx, env.TenantID, draft.ID, env.Actor, nil)
	if err != nil {
		t.Fatalf("first activate: %v", err)
	}
	beforeCount := auditCount(t, env.Pool, env.TenantID, draft.ID, domain.AuditActionContractActivated)
	second, err := env.Contracts.Activate(ctx, env.TenantID, draft.ID, env.Actor, nil)
	if err != nil {
		t.Fatalf("second activate: %v", err)
	}
	if second.Status != domain.ContractStatusActive {
		t.Fatalf("expected ACTIVE, got %s", second.Status)
	}
	if first.Version != second.Version {
		t.Fatalf("expected idempotent no-op on version")
	}
	afterCount := auditCount(t, env.Pool, env.TenantID, draft.ID, domain.AuditActionContractActivated)
	if afterCount != beforeCount {
		t.Fatalf("expected single activate audit, got before=%d after=%d", beforeCount, afterCount)
	}
}

func TestCRPG006_ActiveToSuspended(t *testing.T) {
	env := setupEnv(t)
	draft := env.createDraftContract(t, "CR-PG-006")
	ctx := context.Background()
	_, _ = env.Contracts.Activate(ctx, env.TenantID, draft.ID, env.Actor, nil)
	suspended, err := env.Contracts.Suspend(ctx, env.TenantID, draft.ID, env.Actor, nil)
	if err != nil {
		t.Fatalf("suspend: %v", err)
	}
	if suspended.Status != domain.ContractStatusSuspended {
		t.Fatalf("expected SUSPENDED, got %s", suspended.Status)
	}
}

func TestCRPG007_SuspendedToActiveWhenNotExpired(t *testing.T) {
	env := setupEnv(t)
	draft := env.createDraftContract(t, "CR-PG-007")
	ctx := context.Background()
	_, _ = env.Contracts.Activate(ctx, env.TenantID, draft.ID, env.Actor, nil)
	_, _ = env.Contracts.Suspend(ctx, env.TenantID, draft.ID, env.Actor, nil)
	reactivated, err := env.Contracts.Reactivate(ctx, env.TenantID, draft.ID, env.Actor, nil)
	if err != nil {
		t.Fatalf("reactivate: %v", err)
	}
	if reactivated.Status != domain.ContractStatusActive {
		t.Fatalf("expected ACTIVE, got %s", reactivated.Status)
	}
}

func TestCRPG008_ActiveToTerminated(t *testing.T) {
	env := setupEnv(t)
	draft := env.createDraftContract(t, "CR-PG-008")
	ctx := context.Background()
	_, _ = env.Contracts.Activate(ctx, env.TenantID, draft.ID, env.Actor, nil)
	terminated, err := env.Contracts.Terminate(ctx, env.TenantID, draft.ID, env.Actor, nil, nil)
	if err != nil {
		t.Fatalf("terminate: %v", err)
	}
	if terminated.Status != domain.ContractStatusTerminated {
		t.Fatalf("expected TERMINATED, got %s", terminated.Status)
	}
}

func TestCRPG009_SuspendedToTerminated(t *testing.T) {
	env := setupEnv(t)
	draft := env.createDraftContract(t, "CR-PG-009")
	ctx := context.Background()
	_, _ = env.Contracts.Activate(ctx, env.TenantID, draft.ID, env.Actor, nil)
	_, _ = env.Contracts.Suspend(ctx, env.TenantID, draft.ID, env.Actor, nil)
	terminated, err := env.Contracts.Terminate(ctx, env.TenantID, draft.ID, env.Actor, nil, nil)
	if err != nil {
		t.Fatalf("terminate from suspended: %v", err)
	}
	if terminated.Status != domain.ContractStatusTerminated {
		t.Fatalf("expected TERMINATED, got %s", terminated.Status)
	}
}

func TestCRPG010_TerminatedMutationDenied(t *testing.T) {
	env := setupEnv(t)
	draft := env.createDraftContract(t, "CR-PG-010")
	ctx := context.Background()
	_, _ = env.Contracts.Activate(ctx, env.TenantID, draft.ID, env.Actor, nil)
	_, _ = env.Contracts.Terminate(ctx, env.TenantID, draft.ID, env.Actor, nil, nil)
	_, err := env.Contracts.PatchMetadata(ctx, env.TenantID, draft.ID, domain.PatchContractMetadataInput{
		Description: strPtr("blocked"), Actor: env.Actor,
	}, nil)
	if !isAppErrorCode(err, apperrors.CodeValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestCRPG011_CancelledMutationDenied(t *testing.T) {
	env := setupEnv(t)
	draft := env.createDraftContract(t, "CR-PG-011")
	ctx := context.Background()
	cancelled, err := env.Contracts.Cancel(ctx, env.TenantID, draft.ID, env.Actor, nil)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if cancelled.Status != domain.ContractStatusCancelled {
		t.Fatalf("expected CANCELLED, got %s", cancelled.Status)
	}
	_, err = env.Contracts.Activate(ctx, env.TenantID, draft.ID, env.Actor, nil)
	if !isAppErrorCode(err, apperrors.CodeValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestCRPG012_ExpiredReactivationDenied(t *testing.T) {
	env := setupEnv(t)
	draft := env.createDraftContract(t, "CR-PG-012")
	ctx := context.Background()
	yesterday := env.Today.AddDate(0, 0, -1)
	_, err := env.Pool.Exec(ctx, `
		UPDATE contract_rate.transport_contract
		SET valid_to = $1, status = 'EXPIRED', updated_at = NOW()
		WHERE tenant_id = $2 AND id = $3`, yesterday, env.TenantID, draft.ID)
	if err != nil {
		t.Fatalf("force expired: %v", err)
	}
	_, err = env.Contracts.Reactivate(ctx, env.TenantID, draft.ID, env.Actor, nil)
	if !isAppErrorCode(err, apperrors.CodeValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestCRPG013_ActiveImmutableCommercialFields(t *testing.T) {
	env := setupEnv(t)
	draft := env.createDraftContract(t, "CR-PG-013")
	ctx := context.Background()
	active, err := env.Contracts.Activate(ctx, env.TenantID, draft.ID, env.Actor, nil)
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	name := "changed"
	_, err = env.Contracts.UpdateDraft(ctx, env.TenantID, active.ID, domain.UpdateContractInput{
		Name: &name, Actor: env.Actor,
	}, nil)
	if !isAppErrorCode(err, apperrors.CodeValidation) {
		t.Fatalf("expected draft-only validation error, got %v", err)
	}
	after, err := env.Contracts.GetByIDAndTenant(ctx, env.TenantID, active.ID)
	if err != nil {
		t.Fatalf("get after: %v", err)
	}
	for _, check := range []struct {
		name string
		ok   bool
	}{
		{"buyer_company_id", after.BuyerCompanyID == active.BuyerCompanyID},
		{"carrier_company_id", after.CarrierCompanyID == active.CarrierCompanyID},
		{"contract_number", after.ContractNumber == active.ContractNumber},
		{"valid_from", after.ValidFrom.Equal(active.ValidFrom)},
		{"currency_code", after.CurrencyCode == active.CurrencyCode},
	} {
		if !check.ok {
			t.Fatalf("immutable field %s changed after denied mutation attempt", check.name)
		}
	}
}

func TestCRPG014_CrossTenantContractRead(t *testing.T) {
	env := setupEnv(t)
	draft := env.createDraftContract(t, "CR-PG-014")
	otherTenant := uuid.New()
	_, err := env.Contracts.GetByIDAndTenant(context.Background(), otherTenant, draft.ID)
	if !isAppErrorCode(err, apperrors.CodeNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestCRPG015_CrossTenantContractMutation(t *testing.T) {
	env := setupEnv(t)
	draft := env.createDraftContract(t, "CR-PG-015")
	otherTenant := uuid.New()
	otherActor := domain.ActorInput{
		TenantID: otherTenant, ActorUserID: uuid.New(),
		ActorCompanyID: uuid.New(), ActorKind: domain.ActorKindBuyer,
	}
	_, err := env.Contracts.Activate(context.Background(), otherTenant, draft.ID, otherActor, nil)
	if !isAppErrorCode(err, apperrors.CodeNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestCRPG016_CrossTenantRateCardParent(t *testing.T) {
	env := setupEnv(t)
	draft := env.createDraftContract(t, "CR-PG-016")
	ctx := context.Background()
	otherTenant := uuid.New()
	otherActor := domain.ActorInput{
		TenantID: otherTenant, ActorUserID: uuid.New(),
		ActorCompanyID: uuid.New(), ActorKind: domain.ActorKindBuyer,
	}
	_, err := env.RateCards.Create(ctx, domain.CreateRateCardInput{
		TenantID: otherTenant, ContractID: draft.ID, Name: "Cross tenant card", Actor: otherActor,
	}, nil)
	if !isAppErrorCode(err, apperrors.CodeNotFound) {
		t.Fatalf("expected not found at service level, got %v", err)
	}

	otherCardID := uuid.New()
	_, err = env.Pool.Exec(ctx, `
		INSERT INTO contract_rate.rate_card (id, tenant_id, contract_id, name, created_at, updated_at)
		VALUES ($1, $2, $3, 'Direct FK test', NOW(), NOW())`,
		otherCardID, otherTenant, draft.ID)
	if err == nil {
		t.Fatal("expected composite FK violation on direct SQL insert")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		t.Fatalf("expected FK violation 23503, got %v", err)
	}
}

func TestCRPG017_CrossTenantRateCardVersionParent(t *testing.T) {
	env := setupEnv(t)
	draft := env.createDraftContract(t, "CR-PG-017")
	ctx := context.Background()
	card, err := env.RateCards.Create(ctx, domain.CreateRateCardInput{
		TenantID: env.TenantID, ContractID: draft.ID, Name: "Card", Actor: env.Actor,
	}, nil)
	if err != nil {
		t.Fatalf("create card: %v", err)
	}
	otherTenant := uuid.New()
	otherActor := domain.ActorInput{
		TenantID: otherTenant, ActorUserID: uuid.New(),
		ActorCompanyID: uuid.New(), ActorKind: domain.ActorKindBuyer,
	}
	_, err = env.RateCards.CreateDraftVersion(ctx, domain.CreateRateVersionInput{
		TenantID: otherTenant, RateCardID: card.ID, ValidFrom: env.Today, Actor: otherActor,
	}, nil)
	if !isAppErrorCode(err, apperrors.CodeNotFound) {
		t.Fatalf("expected not found at service level, got %v", err)
	}

	otherVersionID := uuid.New()
	_, err = env.Pool.Exec(ctx, `
		INSERT INTO contract_rate.rate_card_version
		(id, tenant_id, rate_card_id, version_number, valid_from, status, created_at)
		VALUES ($1, $2, $3, 1, $4, 'DRAFT', NOW())`,
		otherVersionID, otherTenant, card.ID, env.Today)
	if err == nil {
		t.Fatal("expected composite FK violation on direct SQL insert")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		t.Fatalf("expected FK violation 23503, got %v", err)
	}
}

func TestCRPG018_OneActiveRateCardVersionDBInvariant(t *testing.T) {
	env := setupEnv(t)
	draft := env.createDraftContract(t, "CR-PG-018")
	ctx := context.Background()
	card, err := env.RateCards.Create(ctx, domain.CreateRateCardInput{
		TenantID: env.TenantID, ContractID: draft.ID, Name: "One active card", Actor: env.Actor,
	}, nil)
	if err != nil {
		t.Fatalf("create card: %v", err)
	}
	v1, err := env.RateCards.CreateDraftVersion(ctx, domain.CreateRateVersionInput{
		TenantID: env.TenantID, RateCardID: card.ID, ValidFrom: env.Today, Actor: env.Actor,
	}, nil)
	if err != nil {
		t.Fatalf("create v1: %v", err)
	}
	v2, err := env.RateCards.CreateDraftVersion(ctx, domain.CreateRateVersionInput{
		TenantID: env.TenantID, RateCardID: card.ID, ValidFrom: env.Today, Actor: env.Actor,
	}, nil)
	if err != nil {
		t.Fatalf("create v2: %v", err)
	}
	activateRateVersionSQL(t, env, v1.ID)
	_, err = env.Pool.Exec(ctx, `
		UPDATE contract_rate.rate_card_version
		SET status='ACTIVE', activated_at=now()
		WHERE tenant_id=$1 AND id=$2`, env.TenantID, v2.ID)
	if err == nil {
		t.Fatal("expected second ACTIVE rejected by uq_rate_card_version_one_active")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("expected unique violation 23505, got %v", err)
	}
}

func TestCRPG019_ConcurrentDraftVersionCreation(t *testing.T) {
	env := setupEnv(t)
	draft := env.createDraftContract(t, "CR-PG-019")
	ctx := context.Background()
	card, err := env.RateCards.Create(ctx, domain.CreateRateCardInput{
		TenantID: env.TenantID, ContractID: draft.ID, Name: "Concurrent card", Actor: env.Actor,
	}, nil)
	if err != nil {
		t.Fatalf("create card: %v", err)
	}
	const workers = 8
	var wg sync.WaitGroup
	wg.Add(workers)
	results := make([]int, workers)
	errs := make([]error, workers)
	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			ver, err := env.RateCards.CreateDraftVersion(ctx, domain.CreateRateVersionInput{
				TenantID: env.TenantID, RateCardID: card.ID, ValidFrom: env.Today, Actor: env.Actor,
			}, nil)
			if err != nil {
				errs[i] = err
				return
			}
			results[i] = ver.VersionNumber
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("worker %d: %v", i, err)
		}
	}
	seen := map[int]bool{}
	for _, n := range results {
		if n <= 0 {
			t.Fatalf("invalid version number %d", n)
		}
		if seen[n] {
			t.Fatalf("duplicate version number %d", n)
		}
		seen[n] = true
	}
	if len(seen) != workers {
		t.Fatalf("expected %d unique versions, got %d", workers, len(seen))
	}
}

func TestCRPG020_AuditInSameBusinessTransaction(t *testing.T) {
	env := setupEnv(t)
	draft := env.createDraftContract(t, "CR-PG-020")
	count := auditCount(t, env.Pool, env.TenantID, draft.ID, domain.AuditActionContractCreated)
	if count != 1 {
		t.Fatalf("expected 1 create audit, got %d", count)
	}
}

func TestCRPG021_AuditInsertionFailureRollsBack(t *testing.T) {
	env := setupEnv(t)
	draft := env.createDraftContract(t, "CR-PG-021")
	ctx := context.Background()
	before, err := env.Contracts.GetByIDAndTenant(ctx, env.TenantID, draft.ID)
	if err != nil {
		t.Fatalf("get before: %v", err)
	}
	if before.Status != domain.ContractStatusDraft {
		t.Fatalf("expected DRAFT before, got %s", before.Status)
	}
	err = env.Contracts.SimulateActivateAuditFailureForTest(ctx, env.TenantID, draft.ID, env.Actor)
	if err == nil {
		t.Fatal("expected audit failure error")
	}
	after, err := env.Contracts.GetByIDAndTenant(ctx, env.TenantID, draft.ID)
	if err != nil {
		t.Fatalf("get after: %v", err)
	}
	if after.Status != domain.ContractStatusDraft {
		t.Fatalf("expected rollback to DRAFT, got %s", after.Status)
	}
	if after.Version != before.Version {
		t.Fatalf("expected no lifecycle mutation persisted")
	}
}

func TestCRPG022_NoCommercialHistoryDeletionAPI(t *testing.T) {
	env := setupEnv(t)
	draft := env.createDraftContract(t, "CR-PG-022")
	ctx := context.Background()
	card, err := env.RateCards.Create(ctx, domain.CreateRateCardInput{
		TenantID: env.TenantID, ContractID: draft.ID, Name: "Card", Actor: env.Actor,
	}, nil)
	if err != nil {
		t.Fatalf("create card: %v", err)
	}
	version, err := env.RateCards.CreateDraftVersion(ctx, domain.CreateRateVersionInput{
		TenantID: env.TenantID, RateCardID: card.ID, ValidFrom: env.Today, Actor: env.Actor,
	}, nil)
	if err != nil {
		t.Fatalf("create version: %v", err)
	}
	activateRateVersionSQL(t, env, version.ID)
	err = env.RateCards.DiscardDraftVersion(ctx, env.TenantID, version.ID, env.Actor, nil)
	if !isAppErrorCode(err, apperrors.CodeValidation) {
		t.Fatalf("expected discard denied for non-draft version, got %v", err)
	}
}

func TestOneActiveDifferentCardsAllow(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	draft1 := env.createDraftContract(t, "ONE-ACTIVE-A")
	draft2 := env.createDraftContract(t, "ONE-ACTIVE-B")
	card1, err := env.RateCards.Create(ctx, domain.CreateRateCardInput{
		TenantID: env.TenantID, ContractID: draft1.ID, Name: "Card A", Actor: env.Actor,
	}, nil)
	if err != nil {
		t.Fatalf("card1: %v", err)
	}
	card2, err := env.RateCards.Create(ctx, domain.CreateRateCardInput{
		TenantID: env.TenantID, ContractID: draft2.ID, Name: "Card B", Actor: env.Actor,
	}, nil)
	if err != nil {
		t.Fatalf("card2: %v", err)
	}
	v1, err := env.RateCards.CreateDraftVersion(ctx, domain.CreateRateVersionInput{
		TenantID: env.TenantID, RateCardID: card1.ID, ValidFrom: env.Today, Actor: env.Actor,
	}, nil)
	if err != nil {
		t.Fatalf("v1: %v", err)
	}
	v2, err := env.RateCards.CreateDraftVersion(ctx, domain.CreateRateVersionInput{
		TenantID: env.TenantID, RateCardID: card2.ID, ValidFrom: env.Today, Actor: env.Actor,
	}, nil)
	if err != nil {
		t.Fatalf("v2: %v", err)
	}
	activateRateVersionSQL(t, env, v1.ID)
	activateRateVersionSQL(t, env, v2.ID)
}

func TestContractLifecycleEndToEnd(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	draft := env.createDraftContract(t, "E2E")
	_, _ = env.Contracts.Activate(ctx, env.TenantID, draft.ID, env.Actor, nil)
	_, _ = env.Contracts.Suspend(ctx, env.TenantID, draft.ID, env.Actor, nil)
	reactivated, _ := env.Contracts.Reactivate(ctx, env.TenantID, draft.ID, env.Actor, nil)
	if reactivated.Status != domain.ContractStatusActive {
		t.Fatalf("expected ACTIVE after reactivate")
	}
	_, _ = env.Contracts.Terminate(ctx, env.TenantID, draft.ID, env.Actor, nil, nil)
	final, err := env.Contracts.GetByIDAndTenant(ctx, env.TenantID, draft.ID)
	if err != nil {
		t.Fatalf("get final: %v", err)
	}
	if final.Status != domain.ContractStatusTerminated {
		t.Fatalf("expected TERMINATED, got %s", final.Status)
	}
}
