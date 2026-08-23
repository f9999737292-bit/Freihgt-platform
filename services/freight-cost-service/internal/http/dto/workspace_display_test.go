package dto

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	"github.com/freight-platform/freight-cost-service/internal/security"
)

func TestSanitizeDisplayLabelRejectsUUID(t *testing.T) {
	t.Parallel()
	id := uuid.NewString()
	if got := SanitizeDisplayLabel(id); got != "" {
		t.Fatalf("UUID must not be used as display label, got %q", got)
	}
	if got := SanitizeDisplayLabel("ORD-1001"); got != "ORD-1001" {
		t.Fatalf("human reference must be preserved, got %q", got)
	}
}

func TestFC_D_SEC_010_CarrierSummaryJSONOmitsBuyerOnlyFields(t *testing.T) {
	t.Parallel()
	planned := decimal.RequireFromString("100.00")
	accrued := decimal.RequireFromString("50.00")
	projection := domainCostSummaryProjectionFixture(t, planned, accrued)
	dtoItem := ToWorkspaceSummaryDTO(projection, securityCarrierActor(), projectionUpdatedAt())

	raw, err := json.Marshal(dtoItem)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"accrued_amount", "forecast_exposure", "current_variance_amount", "final_variance_amount"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("carrier JSON must omit %q, body=%s", key, string(raw))
		}
	}
}

func domainCostSummaryProjectionFixture(t *testing.T, planned, accrued decimal.Decimal) *domain.CostSummaryProjection {
	t.Helper()
	return &domain.CostSummaryProjection{
		TenantID:                    uuid.New(),
		TransportOrderID:            uuid.New(),
		BuyerCompanyID:              uuid.New(),
		CarrierCompanyID:            uuid.New(),
		CurrencyCode:                "RUB",
		DataStage:                   domain.DataStageAccrualAvailable,
		FinancialFinality:           domain.FinancialFinalityCurrentActual,
		SourcesAvailable:            []string{"PLANNED", "ACCRUED"},
		PlannedAmount:               &planned,
		AccruedAmount:               &accrued,
		ForecastExposure:            ptrDecimal(decimal.RequireFromString("10.00")),
		CurrentVarianceAmount:       ptrDecimal(decimal.RequireFromString("5.00")),
		FinalVarianceAmount:         ptrDecimal(decimal.RequireFromString("5.00")),
		BillingReconciliationStatus: domain.BillingReconciliationMatch,
	}
}

func ptrDecimal(v decimal.Decimal) *decimal.Decimal {
	return &v
}

func projectionUpdatedAt() time.Time {
	return time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
}

func securityCarrierActor() security.TrustedActor {
	return security.TrustedActor{
		TenantID:  uuid.New(),
		UserID:    uuid.New(),
		CompanyID: uuid.New(),
		ActorKind: security.ActorKindCarrier,
	}
}
