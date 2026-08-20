//go:build integration

package contractrate

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/contract-rate-service/internal/domain"
	apperrors "github.com/freight-platform/contract-rate-service/internal/platform/errors"
)

func TestCRB001_CreateRateLineOnDraftVersion(t *testing.T) {
	env := setupEnv(t)
	_, version := env.createDraftVersion(t, "CR-B-001", "Card")
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	if line.RateCardVersionID != version.ID {
		t.Fatalf("unexpected version id")
	}
}

func TestCRB002_ListRateLinesByVersion(t *testing.T) {
	env := setupEnv(t)
	_, version := env.createDraftVersion(t, "CR-B-002", "Card")
	_ = env.createRateLine(t, version.ID, "TAUTLINER")
	items, err := env.RateLines.ListByVersion(context.Background(), env.TenantID, version.ID)
	if err != nil || len(items) != 1 {
		t.Fatalf("expected one line, got %v err=%v", len(items), err)
	}
}

func TestCRB003_UpdateRateLine(t *testing.T) {
	env := setupEnv(t)
	_, version := env.createDraftVersion(t, "CR-B-003", "Card")
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	newEquip := "REFRIGERATED"
	updated, err := env.RateLines.Update(context.Background(), env.TenantID, line.ID, domain.UpdateRateLineInput{
		EquipmentType: &newEquip, Actor: env.Actor,
	}, nil)
	if err != nil || updated.EquipmentType != newEquip {
		t.Fatalf("update failed: %v", err)
	}
}

func TestCRB004_DeleteRateLine(t *testing.T) {
	env := setupEnv(t)
	_, version := env.createDraftVersion(t, "CR-B-004", "Card")
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	if err := env.RateLines.Delete(context.Background(), env.TenantID, line.ID, env.Actor, nil); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestCRB005_DuplicateLaneRejected(t *testing.T) {
	env := setupEnv(t)
	_, version := env.createDraftVersion(t, "CR-B-005", "Card")
	_ = env.createRateLine(t, version.ID, "TAUTLINER")
	_, err := env.RateLines.Create(context.Background(), domain.CreateRateLineInput{
		TenantID: env.TenantID, RateCardVersionID: version.ID,
		OriginLocationID: env.OriginID, DestinationLocationID: env.DestID,
		EquipmentType: "TAUTLINER", TransportMode: domain.TransportModeRoad, Actor: env.Actor,
	}, nil)
	if !isAppErrorCode(err, apperrors.CodeConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestCRB006_OriginEqualsDestinationRejected(t *testing.T) {
	env := setupEnv(t)
	_, version := env.createDraftVersion(t, "CR-B-006", "Card")
	_, err := env.RateLines.Create(context.Background(), domain.CreateRateLineInput{
		TenantID: env.TenantID, RateCardVersionID: version.ID,
		OriginLocationID: env.OriginID, DestinationLocationID: env.OriginID,
		EquipmentType: "TAUTLINER", Actor: env.Actor,
	}, nil)
	if !isAppErrorCode(err, apperrors.CodeValidation) {
		t.Fatalf("expected validation, got %v", err)
	}
}

func TestCRB007_InvalidLocationNotFound(t *testing.T) {
	env := setupEnv(t)
	_, version := env.createDraftVersion(t, "CR-B-007", "Card")
	_, err := env.RateLines.Create(context.Background(), domain.CreateRateLineInput{
		TenantID: env.TenantID, RateCardVersionID: version.ID,
		OriginLocationID: uuid.New(), DestinationLocationID: env.DestID,
		EquipmentType: "TAUTLINER", Actor: env.Actor,
	}, nil)
	if !isAppErrorCode(err, apperrors.CodeNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestCRB008_DraftOnlyLineMutationOnActiveVersion(t *testing.T) {
	env := setupEnv(t)
	contract := env.createActiveContract(t, "CR-B-008")
	card, err := env.RateCards.Create(context.Background(), domain.CreateRateCardInput{
		TenantID: env.TenantID, ContractID: contract.ID, Name: "Card", Actor: env.Actor,
	}, nil)
	if err != nil {
		t.Fatalf("card: %v", err)
	}
	version, err := env.RateCards.CreateDraftVersion(context.Background(), domain.CreateRateVersionInput{
		TenantID: env.TenantID, RateCardID: card.ID, ValidFrom: env.Today, Actor: env.Actor,
	}, nil)
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	env.addBaseFreight(t, line.ID, "1000.00")
	active := env.activateVersion(t, version.ID)
	_, err = env.RateLines.Create(context.Background(), domain.CreateRateLineInput{
		TenantID: env.TenantID, RateCardVersionID: active.ID,
		OriginLocationID: env.OriginID, DestinationLocationID: env.DestID,
		EquipmentType: "REFRIGERATED", Actor: env.Actor,
	}, nil)
	if !isAppErrorCode(err, apperrors.CodeValidation) {
		t.Fatalf("expected draft-only validation, got %v", err)
	}
}

func TestCRB009_CrossTenantRateLineReadDenied(t *testing.T) {
	env := setupEnv(t)
	_, version := env.createDraftVersion(t, "CR-B-009", "Card")
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	_, err := env.RateLines.GetByIDAndTenant(context.Background(), uuid.New(), line.ID)
	if !isAppErrorCode(err, apperrors.CodeNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestCRB010_EquipmentTypeRequired(t *testing.T) {
	env := setupEnv(t)
	_, version := env.createDraftVersion(t, "CR-B-010", "Card")
	_, err := env.RateLines.Create(context.Background(), domain.CreateRateLineInput{
		TenantID: env.TenantID, RateCardVersionID: version.ID,
		OriginLocationID: env.OriginID, DestinationLocationID: env.DestID,
		EquipmentType: "  ", Actor: env.Actor,
	}, nil)
	if !isAppErrorCode(err, apperrors.CodeValidation) {
		t.Fatalf("expected validation, got %v", err)
	}
}

func TestCRB011_TransportModeNonRoadRejected(t *testing.T) {
	env := setupEnv(t)
	_, version := env.createDraftVersion(t, "CR-B-011", "Card")
	_, err := env.RateLines.Create(context.Background(), domain.CreateRateLineInput{
		TenantID: env.TenantID, RateCardVersionID: version.ID,
		OriginLocationID: env.OriginID, DestinationLocationID: env.DestID,
		EquipmentType: "TAUTLINER", TransportMode: "RAIL", Actor: env.Actor,
	}, nil)
	if !isAppErrorCode(err, apperrors.CodeValidation) {
		t.Fatalf("expected validation, got %v", err)
	}
}

func TestCRB012_RateLineAuditOnCreate(t *testing.T) {
	env := setupEnv(t)
	_, version := env.createDraftVersion(t, "CR-B-012", "Card")
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	if auditCount(t, env.Pool, env.TenantID, line.ID, domain.AuditActionRateLineCreated) != 1 {
		t.Fatal("expected create audit")
	}
}

func TestCRB013_RateLineAuditOnUpdate(t *testing.T) {
	env := setupEnv(t)
	_, version := env.createDraftVersion(t, "CR-B-013", "Card")
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	equip := "REFRIGERATED"
	_, _ = env.RateLines.Update(context.Background(), env.TenantID, line.ID, domain.UpdateRateLineInput{
		EquipmentType: &equip, Actor: env.Actor,
	}, nil)
	if auditCount(t, env.Pool, env.TenantID, line.ID, domain.AuditActionRateLineUpdated) != 1 {
		t.Fatal("expected update audit")
	}
}

func TestCRB014_RateLineAuditOnDelete(t *testing.T) {
	env := setupEnv(t)
	_, version := env.createDraftVersion(t, "CR-B-014", "Card")
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	_ = env.RateLines.Delete(context.Background(), env.TenantID, line.ID, env.Actor, nil)
	if auditCount(t, env.Pool, env.TenantID, line.ID, domain.AuditActionRateLineDeleted) != 1 {
		t.Fatal("expected delete audit")
	}
}

func TestCRB015_DefaultTransportModeRoad(t *testing.T) {
	env := setupEnv(t)
	_, version := env.createDraftVersion(t, "CR-B-015", "Card")
	line, err := env.RateLines.Create(context.Background(), domain.CreateRateLineInput{
		TenantID: env.TenantID, RateCardVersionID: version.ID,
		OriginLocationID: env.OriginID, DestinationLocationID: env.DestID,
		EquipmentType: "TAUTLINER", Actor: env.Actor,
	}, nil)
	if err != nil || line.TransportMode != domain.TransportModeRoad {
		t.Fatalf("expected ROAD default, got %v err=%v", line.TransportMode, err)
	}
}

func TestCRB016_CreateBaseFreightComponent(t *testing.T) {
	env := setupEnv(t)
	_, version := env.createDraftVersion(t, "CR-B-016", "Card")
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	env.addBaseFreight(t, line.ID, "1500.00")
	items, _ := env.RateComponents.ListByLine(context.Background(), env.TenantID, line.ID)
	if len(items) != 1 || items[0].ComponentType != domain.ComponentTypeBaseFreight {
		t.Fatalf("expected base freight component")
	}
}

func TestCRB017_CreateFuelSurchargeComponent(t *testing.T) {
	env := setupEnv(t)
	_, version := env.createDraftVersion(t, "CR-B-017", "Card")
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	env.addBaseFreight(t, line.ID, "1000.00")
	env.addFuelSurcharge(t, line.ID, "10")
	items, _ := env.RateComponents.ListByLine(context.Background(), env.TenantID, line.ID)
	if len(items) != 2 {
		t.Fatalf("expected 2 components, got %d", len(items))
	}
}

func TestCRB018_CreateWaitingComponent(t *testing.T) {
	env := setupEnv(t)
	_, version := env.createDraftVersion(t, "CR-B-018", "Card")
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	env.addBaseFreight(t, line.ID, "1000.00")
	env.addWaitingRule(t, line.ID, "250.00")
	items, _ := env.RateComponents.ListByLine(context.Background(), env.TenantID, line.ID)
	found := false
	for _, item := range items {
		if item.ComponentType == domain.ComponentTypeWaiting {
			found = true
		}
	}
	if !found {
		t.Fatal("expected waiting component")
	}
}

func TestCRB019_CreateDetentionComponent(t *testing.T) {
	env := setupEnv(t)
	_, version := env.createDraftVersion(t, "CR-B-019", "Card")
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	env.addBaseFreight(t, line.ID, "1000.00")
	env.addDetentionRule(t, line.ID, "300.00")
	items, _ := env.RateComponents.ListByLine(context.Background(), env.TenantID, line.ID)
	found := false
	for _, item := range items {
		if item.ComponentType == domain.ComponentTypeDetention {
			found = true
		}
	}
	if !found {
		t.Fatal("expected detention component")
	}
}

func TestCRB020_DuplicateComponentTypeRejected(t *testing.T) {
	env := setupEnv(t)
	_, version := env.createDraftVersion(t, "CR-B-020", "Card")
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	env.addBaseFreight(t, line.ID, "1000.00")
	_, err := env.RateComponents.Create(context.Background(), domain.CreateRateComponentInput{
		TenantID: env.TenantID, RateLineID: line.ID,
		ComponentType: domain.ComponentTypeBaseFreight, CalculationMethod: domain.CalcMethodFlat,
		Amount: dec("2000.00"), Actor: env.Actor,
	}, nil)
	if !isAppErrorCode(err, apperrors.CodeConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestCRB021_BaseFreightRequiresFlat(t *testing.T) {
	env := setupEnv(t)
	_, version := env.createDraftVersion(t, "CR-B-021", "Card")
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	_, err := env.RateComponents.Create(context.Background(), domain.CreateRateComponentInput{
		TenantID: env.TenantID, RateLineID: line.ID,
		ComponentType: domain.ComponentTypeBaseFreight, CalculationMethod: domain.CalcMethodPercent,
		PercentValue: dec("10"), Actor: env.Actor,
	}, nil)
	if !isAppErrorCode(err, apperrors.CodeValidation) {
		t.Fatalf("expected validation, got %v", err)
	}
}

func TestCRB022_FuelRequiresPercent(t *testing.T) {
	env := setupEnv(t)
	_, version := env.createDraftVersion(t, "CR-B-022", "Card")
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	_, err := env.RateComponents.Create(context.Background(), domain.CreateRateComponentInput{
		TenantID: env.TenantID, RateLineID: line.ID,
		ComponentType: domain.ComponentTypeFuelSurcharge, CalculationMethod: domain.CalcMethodFlat,
		Amount: dec("100.00"), Actor: env.Actor,
	}, nil)
	if !isAppErrorCode(err, apperrors.CodeValidation) {
		t.Fatalf("expected validation, got %v", err)
	}
}

func TestCRB023_WaitingRequiresUnitRate(t *testing.T) {
	env := setupEnv(t)
	_, version := env.createDraftVersion(t, "CR-B-023", "Card")
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	_, err := env.RateComponents.Create(context.Background(), domain.CreateRateComponentInput{
		TenantID: env.TenantID, RateLineID: line.ID,
		ComponentType: domain.ComponentTypeWaiting, CalculationMethod: domain.CalcMethodFlat,
		Amount: dec("100.00"), Actor: env.Actor,
	}, nil)
	if !isAppErrorCode(err, apperrors.CodeValidation) {
		t.Fatalf("expected validation, got %v", err)
	}
}

func TestCRB024_UpdateComponentAmount(t *testing.T) {
	env := setupEnv(t)
	_, version := env.createDraftVersion(t, "CR-B-024", "Card")
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	env.addBaseFreight(t, line.ID, "1000.00")
	items, _ := env.RateComponents.ListByLine(context.Background(), env.TenantID, line.ID)
	newAmt := dec("1200.00")
	updated, err := env.RateComponents.Update(context.Background(), env.TenantID, items[0].ID, domain.UpdateRateComponentInput{
		Amount: newAmt, Actor: env.Actor,
	}, nil)
	if err != nil || !updated.Amount.Equal(*newAmt) {
		t.Fatalf("update failed: %v", err)
	}
}

func TestCRB025_DeleteComponent(t *testing.T) {
	env := setupEnv(t)
	_, version := env.createDraftVersion(t, "CR-B-025", "Card")
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	env.addBaseFreight(t, line.ID, "1000.00")
	items, _ := env.RateComponents.ListByLine(context.Background(), env.TenantID, line.ID)
	if err := env.RateComponents.Delete(context.Background(), env.TenantID, items[0].ID, env.Actor, nil); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestCRB026_ActivateRejectsLineWithoutBaseFreight(t *testing.T) {
	env := setupEnv(t)
	contract := env.createActiveContract(t, "CR-B-026")
	card, _ := env.RateCards.Create(context.Background(), domain.CreateRateCardInput{
		TenantID: env.TenantID, ContractID: contract.ID, Name: "Card", Actor: env.Actor,
	}, nil)
	version, _ := env.RateCards.CreateDraftVersion(context.Background(), domain.CreateRateVersionInput{
		TenantID: env.TenantID, RateCardID: card.ID, ValidFrom: env.Today, Actor: env.Actor,
	}, nil)
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	env.addFuelSurcharge(t, line.ID, "10")
	_, err := env.RateCards.ActivateVersion(context.Background(), env.TenantID, version.ID, env.Actor, nil)
	if !isAppErrorCode(err, apperrors.CodeValidation) {
		t.Fatalf("expected validation, got %v", err)
	}
}

func TestCRB027_ActivateRejectsDuplicateComponentTypes(t *testing.T) {
	env := setupEnv(t)
	contract := env.createActiveContract(t, "CR-B-027")
	card, _ := env.RateCards.Create(context.Background(), domain.CreateRateCardInput{
		TenantID: env.TenantID, ContractID: contract.ID, Name: "Card", Actor: env.Actor,
	}, nil)
	version, _ := env.RateCards.CreateDraftVersion(context.Background(), domain.CreateRateVersionInput{
		TenantID: env.TenantID, RateCardID: card.ID, ValidFrom: env.Today, Actor: env.Actor,
	}, nil)
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	env.addBaseFreight(t, line.ID, "1000.00")
	_, err := env.Pool.Exec(context.Background(), `
		INSERT INTO contract_rate.rate_component (
			tenant_id, rate_line_id, component_type, calculation_method, amount
		) VALUES ($1,$2,'BASE_FREIGHT','FLAT',2000)`, env.TenantID, line.ID)
	if err == nil {
		t.Fatal("expected DB unique constraint on duplicate component type")
	}
}

func TestCRB028_DraftOnlyComponentMutationDenied(t *testing.T) {
	env := setupEnv(t)
	contract := env.createActiveContract(t, "CR-B-028")
	card, _ := env.RateCards.Create(context.Background(), domain.CreateRateCardInput{
		TenantID: env.TenantID, ContractID: contract.ID, Name: "Card", Actor: env.Actor,
	}, nil)
	version, _ := env.RateCards.CreateDraftVersion(context.Background(), domain.CreateRateVersionInput{
		TenantID: env.TenantID, RateCardID: card.ID, ValidFrom: env.Today, Actor: env.Actor,
	}, nil)
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	env.addBaseFreight(t, line.ID, "1000.00")
	env.activateVersion(t, version.ID)
	_, err := env.RateComponents.Create(context.Background(), domain.CreateRateComponentInput{
		TenantID: env.TenantID, RateLineID: line.ID,
		ComponentType: domain.ComponentTypeFuelSurcharge, CalculationMethod: domain.CalcMethodPercent,
		PercentValue: dec("10"), Actor: env.Actor,
	}, nil)
	if !isAppErrorCode(err, apperrors.CodeValidation) {
		t.Fatalf("expected validation, got %v", err)
	}
}

func TestCRB029_ComponentAuditOnCreate(t *testing.T) {
	env := setupEnv(t)
	_, version := env.createDraftVersion(t, "CR-B-029", "Card")
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	component, err := env.RateComponents.Create(context.Background(), domain.CreateRateComponentInput{
		TenantID: env.TenantID, RateLineID: line.ID,
		ComponentType: domain.ComponentTypeBaseFreight, CalculationMethod: domain.CalcMethodFlat,
		Amount: dec("1000.00"), Actor: env.Actor,
	}, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if auditCount(t, env.Pool, env.TenantID, component.ID, domain.AuditActionRateComponentCreated) != 1 {
		t.Fatal("expected component create audit")
	}
}

func TestCRB030_PercentValidationNonNegative(t *testing.T) {
	env := setupEnv(t)
	_, version := env.createDraftVersion(t, "CR-B-030", "Card")
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	neg := dec("-1")
	_, err := env.RateComponents.Create(context.Background(), domain.CreateRateComponentInput{
		TenantID: env.TenantID, RateLineID: line.ID,
		ComponentType: domain.ComponentTypeFuelSurcharge, CalculationMethod: domain.CalcMethodPercent,
		PercentValue: neg, Actor: env.Actor,
	}, nil)
	if !isAppErrorCode(err, apperrors.CodeValidation) {
		t.Fatalf("expected validation, got %v", err)
	}
}

func TestCRB031_ActivateDraftVersionSuccess(t *testing.T) {
	env := setupEnv(t)
	contract := env.createActiveContract(t, "CR-B-031")
	card, _ := env.RateCards.Create(context.Background(), domain.CreateRateCardInput{
		TenantID: env.TenantID, ContractID: contract.ID, Name: "Card", Actor: env.Actor,
	}, nil)
	version, _ := env.RateCards.CreateDraftVersion(context.Background(), domain.CreateRateVersionInput{
		TenantID: env.TenantID, RateCardID: card.ID, ValidFrom: env.Today, Actor: env.Actor,
	}, nil)
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	env.addBaseFreight(t, line.ID, "1000.00")
	activated := env.activateVersion(t, version.ID)
	if activated.Status != domain.RateVersionStatusActive {
		t.Fatalf("expected ACTIVE, got %s", activated.Status)
	}
}

func TestCRB032_ActivateRequiresRateLine(t *testing.T) {
	env := setupEnv(t)
	contract := env.createActiveContract(t, "CR-B-032")
	card, _ := env.RateCards.Create(context.Background(), domain.CreateRateCardInput{
		TenantID: env.TenantID, ContractID: contract.ID, Name: "Card", Actor: env.Actor,
	}, nil)
	version, _ := env.RateCards.CreateDraftVersion(context.Background(), domain.CreateRateVersionInput{
		TenantID: env.TenantID, RateCardID: card.ID, ValidFrom: env.Today, Actor: env.Actor,
	}, nil)
	_, err := env.RateCards.ActivateVersion(context.Background(), env.TenantID, version.ID, env.Actor, nil)
	if !isAppErrorCode(err, apperrors.CodeValidation) {
		t.Fatalf("expected validation, got %v", err)
	}
}

func TestCRB033_SupersedesPreviousActiveVersion(t *testing.T) {
	env := setupEnv(t)
	contract := env.createActiveContract(t, "CR-B-033")
	card, _ := env.RateCards.Create(context.Background(), domain.CreateRateCardInput{
		TenantID: env.TenantID, ContractID: contract.ID, Name: "Card", Actor: env.Actor,
	}, nil)
	v1, _ := env.RateCards.CreateDraftVersion(context.Background(), domain.CreateRateVersionInput{
		TenantID: env.TenantID, RateCardID: card.ID, ValidFrom: env.Today, Actor: env.Actor,
	}, nil)
	line1 := env.createRateLine(t, v1.ID, "TAUTLINER")
	env.addBaseFreight(t, line1.ID, "1000.00")
	env.activateVersion(t, v1.ID)
	v2, _ := env.RateCards.CreateDraftVersion(context.Background(), domain.CreateRateVersionInput{
		TenantID: env.TenantID, RateCardID: card.ID, ValidFrom: env.Today, Actor: env.Actor,
	}, nil)
	line2 := env.createRateLine(t, v2.ID, "REFRIGERATED")
	env.addBaseFreight(t, line2.ID, "2000.00")
	env.activateVersion(t, v2.ID)
	prev, err := env.RateCards.GetVersionByIDAndTenant(context.Background(), env.TenantID, v1.ID)
	if err != nil || prev.Status != domain.RateVersionStatusSuperseded {
		t.Fatalf("expected superseded v1, got %v err=%v", prev.Status, err)
	}
}

func TestCRB034_OneActivePerCardPreserved(t *testing.T) {
	env := setupEnv(t)
	contract := env.createActiveContract(t, "CR-B-034")
	card, _ := env.RateCards.Create(context.Background(), domain.CreateRateCardInput{
		TenantID: env.TenantID, ContractID: contract.ID, Name: "Card", Actor: env.Actor,
	}, nil)
	v1, _ := env.RateCards.CreateDraftVersion(context.Background(), domain.CreateRateVersionInput{
		TenantID: env.TenantID, RateCardID: card.ID, ValidFrom: env.Today, Actor: env.Actor,
	}, nil)
	v2, _ := env.RateCards.CreateDraftVersion(context.Background(), domain.CreateRateVersionInput{
		TenantID: env.TenantID, RateCardID: card.ID, ValidFrom: env.Today, Actor: env.Actor,
	}, nil)
	activateRateVersionSQL(t, env, v1.ID)
	_, err := env.Pool.Exec(context.Background(), `
		UPDATE contract_rate.rate_card_version SET status='ACTIVE', activated_at=now()
		WHERE tenant_id=$1 AND id=$2`, env.TenantID, v2.ID)
	if err == nil {
		t.Fatal("expected uq_rate_card_version_one_active violation")
	}
}

func TestCRB035_CrossCardLaneConflictRejected(t *testing.T) {
	env := setupEnv(t)
	contract := env.createActiveContract(t, "CR-B-035")
	card1, _ := env.RateCards.Create(context.Background(), domain.CreateRateCardInput{
		TenantID: env.TenantID, ContractID: contract.ID, Name: "Card 1", Actor: env.Actor,
	}, nil)
	card2, _ := env.RateCards.Create(context.Background(), domain.CreateRateCardInput{
		TenantID: env.TenantID, ContractID: contract.ID, Name: "Card 2", Actor: env.Actor,
	}, nil)
	v1, _ := env.RateCards.CreateDraftVersion(context.Background(), domain.CreateRateVersionInput{
		TenantID: env.TenantID, RateCardID: card1.ID, ValidFrom: env.Today, Actor: env.Actor,
	}, nil)
	line1 := env.createRateLine(t, v1.ID, "TAUTLINER")
	env.addBaseFreight(t, line1.ID, "1000.00")
	env.activateVersion(t, v1.ID)
	v2, _ := env.RateCards.CreateDraftVersion(context.Background(), domain.CreateRateVersionInput{
		TenantID: env.TenantID, RateCardID: card2.ID, ValidFrom: env.Today, Actor: env.Actor,
	}, nil)
	line2 := env.createRateLine(t, v2.ID, "TAUTLINER")
	env.addBaseFreight(t, line2.ID, "1100.00")
	_, err := env.RateCards.ActivateVersion(context.Background(), env.TenantID, v2.ID, env.Actor, nil)
	if !isAppErrorCode(err, apperrors.CodeConflict) {
		t.Fatalf("expected lane conflict, got %v", err)
	}
}

func TestCRB036_ActivateNonDraftRejected(t *testing.T) {
	env := setupEnv(t)
	contract := env.createActiveContract(t, "CR-B-036")
	card, _ := env.RateCards.Create(context.Background(), domain.CreateRateCardInput{
		TenantID: env.TenantID, ContractID: contract.ID, Name: "Card", Actor: env.Actor,
	}, nil)
	version, _ := env.RateCards.CreateDraftVersion(context.Background(), domain.CreateRateVersionInput{
		TenantID: env.TenantID, RateCardID: card.ID, ValidFrom: env.Today, Actor: env.Actor,
	}, nil)
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	env.addBaseFreight(t, line.ID, "1000.00")
	env.activateVersion(t, version.ID)
	_, err := env.RateCards.ActivateVersion(context.Background(), env.TenantID, version.ID, env.Actor, nil)
	if err != nil {
		t.Fatalf("expected idempotent activate, got %v", err)
	}
}

func TestCRB037_ActivateAuditEmitted(t *testing.T) {
	env := setupEnv(t)
	contract := env.createActiveContract(t, "CR-B-037")
	card, _ := env.RateCards.Create(context.Background(), domain.CreateRateCardInput{
		TenantID: env.TenantID, ContractID: contract.ID, Name: "Card", Actor: env.Actor,
	}, nil)
	version, _ := env.RateCards.CreateDraftVersion(context.Background(), domain.CreateRateVersionInput{
		TenantID: env.TenantID, RateCardID: card.ID, ValidFrom: env.Today, Actor: env.Actor,
	}, nil)
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	env.addBaseFreight(t, line.ID, "1000.00")
	activated := env.activateVersion(t, version.ID)
	if auditCount(t, env.Pool, env.TenantID, activated.ID, domain.AuditActionRateVersionActivated) != 1 {
		t.Fatal("expected activation audit")
	}
}

func TestCRB038_ActivateAuditFailureRollsBack(t *testing.T) {
	env := setupEnv(t)
	contract := env.createActiveContract(t, "CR-B-038")
	card, _ := env.RateCards.Create(context.Background(), domain.CreateRateCardInput{
		TenantID: env.TenantID, ContractID: contract.ID, Name: "Card", Actor: env.Actor,
	}, nil)
	version, _ := env.RateCards.CreateDraftVersion(context.Background(), domain.CreateRateVersionInput{
		TenantID: env.TenantID, RateCardID: card.ID, ValidFrom: env.Today, Actor: env.Actor,
	}, nil)
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	env.addBaseFreight(t, line.ID, "1000.00")
	err := env.RateCards.SimulateActivateVersionAuditFailureForTest(context.Background(), env.TenantID, version.ID, env.Actor)
	if err == nil {
		t.Fatal("expected simulated audit failure")
	}
	current, _ := env.RateCards.GetVersionByIDAndTenant(context.Background(), env.TenantID, version.ID)
	if current.Status != domain.RateVersionStatusDraft {
		t.Fatalf("expected rollback to DRAFT, got %s", current.Status)
	}
}

func TestCRB039_RepeatActivateIdempotent(t *testing.T) {
	env := setupEnv(t)
	contract := env.createActiveContract(t, "CR-B-039")
	card, _ := env.RateCards.Create(context.Background(), domain.CreateRateCardInput{
		TenantID: env.TenantID, ContractID: contract.ID, Name: "Card", Actor: env.Actor,
	}, nil)
	version, _ := env.RateCards.CreateDraftVersion(context.Background(), domain.CreateRateVersionInput{
		TenantID: env.TenantID, RateCardID: card.ID, ValidFrom: env.Today, Actor: env.Actor,
	}, nil)
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	env.addBaseFreight(t, line.ID, "1000.00")
	first := env.activateVersion(t, version.ID)
	before := auditCount(t, env.Pool, env.TenantID, version.ID, domain.AuditActionRateVersionActivated)
	second, err := env.RateCards.ActivateVersion(context.Background(), env.TenantID, version.ID, env.Actor, nil)
	if err != nil || second.Status != domain.RateVersionStatusActive {
		t.Fatalf("idempotent activate failed: %v", err)
	}
	after := auditCount(t, env.Pool, env.TenantID, version.ID, domain.AuditActionRateVersionActivated)
	if before != after || first.ID != second.ID {
		t.Fatalf("expected idempotent activate without extra audit")
	}
}

func TestCRB040_ActiveContractRequiredForActivation(t *testing.T) {
	env := setupEnv(t)
	draft := env.createDraftContract(t, "CR-B-040")
	card, _ := env.RateCards.Create(context.Background(), domain.CreateRateCardInput{
		TenantID: env.TenantID, ContractID: draft.ID, Name: "Card", Actor: env.Actor,
	}, nil)
	version, _ := env.RateCards.CreateDraftVersion(context.Background(), domain.CreateRateVersionInput{
		TenantID: env.TenantID, RateCardID: card.ID, ValidFrom: env.Today, Actor: env.Actor,
	}, nil)
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	env.addBaseFreight(t, line.ID, "1000.00")
	_, err := env.RateCards.ActivateVersion(context.Background(), env.TenantID, version.ID, env.Actor, nil)
	if !isAppErrorCode(err, apperrors.CodeValidation) {
		t.Fatalf("expected parent contract validation, got %v", err)
	}
}

func setupActiveRate(t *testing.T, env *testEnv, contractNo, equip, baseAmount string) {
	t.Helper()
	contract := env.createActiveContract(t, contractNo)
	card, _ := env.RateCards.Create(context.Background(), domain.CreateRateCardInput{
		TenantID: env.TenantID, ContractID: contract.ID, Name: "Card", Actor: env.Actor,
	}, nil)
	version, _ := env.RateCards.CreateDraftVersion(context.Background(), domain.CreateRateVersionInput{
		TenantID: env.TenantID, RateCardID: card.ID, ValidFrom: env.Today, Actor: env.Actor,
	}, nil)
	line := env.createRateLine(t, version.ID, equip)
	env.addBaseFreight(t, line.ID, baseAmount)
	env.activateVersion(t, version.ID)
}

func TestCRB041_ContractRateMatched(t *testing.T) {
	env := setupEnv(t)
	setupActiveRate(t, env, "CR-B-041", "TAUTLINER", "1000.00")
	result, err := env.ResolutionSvc.Resolve(context.Background(), env.resolveReq("TAUTLINER"), nil)
	if err != nil || result.Status != domain.ResolveStatusMatched || result.PricingSource != domain.PricingSourceContractRate {
		t.Fatalf("expected contract match, got status=%s source=%s err=%v", result.Status, result.PricingSource, err)
	}
}

func TestCRB042_NoMatchWhenNoCandidates(t *testing.T) {
	env := setupEnv(t)
	result, err := env.ResolutionSvc.Resolve(context.Background(), env.resolveReq("TAUTLINER"), nil)
	if err != nil || result.Status != domain.ResolveStatusNoMatch {
		t.Fatalf("expected NO_MATCH, got %s err=%v", result.Status, err)
	}
}

func TestCRB043_AmbiguousMultipleCandidates(t *testing.T) {
	env := setupEnv(t)
	for i, number := range []string{"CR-B-043-A", "CR-B-043-B"} {
		contract := env.createActiveContract(t, number)
		card, _ := env.RateCards.Create(context.Background(), domain.CreateRateCardInput{
			TenantID: env.TenantID, ContractID: contract.ID, Name: "Card", Actor: env.Actor,
		}, nil)
		version, _ := env.RateCards.CreateDraftVersion(context.Background(), domain.CreateRateVersionInput{
			TenantID: env.TenantID, RateCardID: card.ID,
			ValidFrom: env.Today.AddDate(0, 0, -i), Actor: env.Actor,
		}, nil)
		line := env.createRateLine(t, version.ID, "TAUTLINER")
		env.addBaseFreight(t, line.ID, "1000.00")
		env.activateVersion(t, version.ID)
	}
	result, err := env.ResolutionSvc.Resolve(context.Background(), env.resolveReq("TAUTLINER"), nil)
	if err != nil || result.Status != domain.ResolveStatusAmbiguous {
		t.Fatalf("expected AMBIGUOUS, got %s err=%v", result.Status, err)
	}
}

func TestCRB044_PricingDateOutsideContractValidity(t *testing.T) {
	env := setupEnv(t)
	setupActiveRate(t, env, "CR-B-044", "TAUTLINER", "1000.00")
	req := env.resolveReq("TAUTLINER")
	req.PricingDate = env.Today.AddDate(-2, 0, 0)
	result, err := env.ResolutionSvc.Resolve(context.Background(), req, nil)
	if err != nil || result.Status != domain.ResolveStatusNoMatch {
		t.Fatalf("expected NO_MATCH for old pricing date, got %s", result.Status)
	}
}

func TestCRB045_PricingDateOutsideVersionValidity(t *testing.T) {
	env := setupEnv(t)
	contract := env.createActiveContract(t, "CR-B-045")
	card, _ := env.RateCards.Create(context.Background(), domain.CreateRateCardInput{
		TenantID: env.TenantID, ContractID: contract.ID, Name: "Card", Actor: env.Actor,
	}, nil)
	validTo := env.Today.AddDate(0, 0, -1)
	version, _ := env.RateCards.CreateDraftVersion(context.Background(), domain.CreateRateVersionInput{
		TenantID: env.TenantID, RateCardID: card.ID,
		ValidFrom: env.Today.AddDate(0, -2, 0), ValidTo: &validTo, Actor: env.Actor,
	}, nil)
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	env.addBaseFreight(t, line.ID, "1000.00")
	env.activateVersion(t, version.ID)
	result, err := env.ResolutionSvc.Resolve(context.Background(), env.resolveReq("TAUTLINER"), nil)
	if err != nil || result.Status != domain.ResolveStatusNoMatch {
		t.Fatalf("expected NO_MATCH outside version validity, got %s", result.Status)
	}
}

func TestCRB046_CurrencyFilterMismatchExcluded(t *testing.T) {
	env := setupEnv(t)
	setupActiveRate(t, env, "CR-B-046", "TAUTLINER", "1000.00")
	req := env.resolveReq("TAUTLINER")
	cur := "USD"
	req.CurrencyCode = &cur
	result, err := env.ResolutionSvc.Resolve(context.Background(), req, nil)
	if err != nil || result.Status != domain.ResolveStatusNoMatch {
		t.Fatalf("expected NO_MATCH for currency filter, got %s", result.Status)
	}
}

func TestCRB047_FuelSurchargeInTotal(t *testing.T) {
	env := setupEnv(t)
	contract := env.createActiveContract(t, "CR-B-047")
	card, _ := env.RateCards.Create(context.Background(), domain.CreateRateCardInput{
		TenantID: env.TenantID, ContractID: contract.ID, Name: "Card", Actor: env.Actor,
	}, nil)
	version, _ := env.RateCards.CreateDraftVersion(context.Background(), domain.CreateRateVersionInput{
		TenantID: env.TenantID, RateCardID: card.ID, ValidFrom: env.Today, Actor: env.Actor,
	}, nil)
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	env.addBaseFreight(t, line.ID, "1000.00")
	env.addFuelSurcharge(t, line.ID, "10")
	env.activateVersion(t, version.ID)
	result, err := env.ResolutionSvc.Resolve(context.Background(), env.resolveReq("TAUTLINER"), nil)
	if err != nil || result.TotalAmount == nil || *result.TotalAmount != "1100.00" {
		t.Fatalf("expected total 1100.00, got %v err=%v", result.TotalAmount, err)
	}
}

func TestCRB048_WaitingNotInPreExecTotal(t *testing.T) {
	env := setupEnv(t)
	contract := env.createActiveContract(t, "CR-B-048")
	card, _ := env.RateCards.Create(context.Background(), domain.CreateRateCardInput{
		TenantID: env.TenantID, ContractID: contract.ID, Name: "Card", Actor: env.Actor,
	}, nil)
	version, _ := env.RateCards.CreateDraftVersion(context.Background(), domain.CreateRateVersionInput{
		TenantID: env.TenantID, RateCardID: card.ID, ValidFrom: env.Today, Actor: env.Actor,
	}, nil)
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	env.addBaseFreight(t, line.ID, "1000.00")
	env.addWaitingRule(t, line.ID, "250.00")
	env.activateVersion(t, version.ID)
	result, err := env.ResolutionSvc.Resolve(context.Background(), env.resolveReq("TAUTLINER"), nil)
	if err != nil || result.TotalAmount == nil || *result.TotalAmount != "1000.00" {
		t.Fatalf("waiting must not affect total, got %v", result.TotalAmount)
	}
	if len(result.AccessorialRules) != 1 {
		t.Fatalf("expected accessorial rule, got %d", len(result.AccessorialRules))
	}
}

func TestCRB049_DetentionNotInPreExecTotal(t *testing.T) {
	env := setupEnv(t)
	contract := env.createActiveContract(t, "CR-B-049")
	card, _ := env.RateCards.Create(context.Background(), domain.CreateRateCardInput{
		TenantID: env.TenantID, ContractID: contract.ID, Name: "Card", Actor: env.Actor,
	}, nil)
	version, _ := env.RateCards.CreateDraftVersion(context.Background(), domain.CreateRateVersionInput{
		TenantID: env.TenantID, RateCardID: card.ID, ValidFrom: env.Today, Actor: env.Actor,
	}, nil)
	line := env.createRateLine(t, version.ID, "TAUTLINER")
	env.addBaseFreight(t, line.ID, "1000.00")
	env.addDetentionRule(t, line.ID, "300.00")
	env.activateVersion(t, version.ID)
	result, err := env.ResolutionSvc.Resolve(context.Background(), env.resolveReq("TAUTLINER"), nil)
	if err != nil || result.TotalAmount == nil || *result.TotalAmount != "1000.00" {
		t.Fatalf("detention must not affect total, got %v", result.TotalAmount)
	}
}

func TestCRB050_ManualSpotAuthorized(t *testing.T) {
	env := setupEnv(t)
	userID := uuid.New()
	seedGlobalRole(t, context.Background(), env.Pool, env.TenantID, userID, "PROCUREMENT_MANAGER")
	actor := env.Actor
	actor.ActorUserID = userID
	amount := decimal.RequireFromString("5000.00")
	currency := "RUB"
	req := env.resolveReq("TAUTLINER")
	req.Actor = actor
	req.ManualSpotAmount = &amount
	req.ManualSpotCurrency = &currency
	result, err := env.ResolutionSvc.Resolve(context.Background(), req, nil)
	if err != nil || result.Status != domain.ResolveStatusMatched || result.PricingSource != domain.PricingSourceManualSpot {
		t.Fatalf("expected authorized manual spot, got status=%s err=%v", result.Status, err)
	}
}

func TestCRB051_ManualSpotForbiddenWithoutRole(t *testing.T) {
	env := setupEnv(t)
	amount := decimal.RequireFromString("5000.00")
	currency := "RUB"
	req := env.resolveReq("TAUTLINER")
	req.ManualSpotAmount = &amount
	req.ManualSpotCurrency = &currency
	_, err := env.ResolutionSvc.Resolve(context.Background(), req, nil)
	if !isAppErrorCode(err, apperrors.CodeForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestCRB052_ManualSpotForbiddenForCarrier(t *testing.T) {
	env := setupEnv(t)
	userID := uuid.New()
	seedGlobalRole(t, context.Background(), env.Pool, env.TenantID, userID, "PROCUREMENT_MANAGER")
	amount := decimal.RequireFromString("5000.00")
	currency := "RUB"
	req := env.resolveReq("TAUTLINER")
	req.Actor = domain.ActorInput{
		TenantID: env.TenantID, ActorUserID: userID, ActorCompanyID: env.CarrierID,
		ActorKind: domain.ActorKindCarrier,
	}
	req.ManualSpotAmount = &amount
	req.ManualSpotCurrency = &currency
	_, err := env.ResolutionSvc.Resolve(context.Background(), req, nil)
	if !isAppErrorCode(err, apperrors.CodeForbidden) {
		t.Fatalf("expected carrier forbidden, got %v", err)
	}
}

func TestCRB053_RFQAwardPricingSourceRejected(t *testing.T) {
	env := setupEnv(t)
	src := domain.PricingSourceRFQAward
	req := env.resolveReq("TAUTLINER")
	req.PricingSource = &src
	_, err := env.ResolutionSvc.Resolve(context.Background(), req, nil)
	if !isAppErrorCode(err, apperrors.CodeValidation) {
		t.Fatalf("expected validation, got %v", err)
	}
}

func TestCRB054_RFxIdentifiersRejected(t *testing.T) {
	env := setupEnv(t)
	award := uuid.New()
	bid := uuid.New()
	for _, tc := range []domain.ResolveRateRequest{
		{TenantID: env.TenantID, BuyerCompanyID: env.BuyerID, CarrierCompanyID: env.CarrierID, AwardLinkID: &award, Actor: env.Actor},
		{TenantID: env.TenantID, BuyerCompanyID: env.BuyerID, CarrierCompanyID: env.CarrierID, BidID: &bid, Actor: env.Actor},
	} {
		_, err := env.ResolutionSvc.Resolve(context.Background(), tc, nil)
		if !isAppErrorCode(err, apperrors.CodeValidation) {
			t.Fatalf("expected RFx identifier rejection, got %v", err)
		}
	}
}
