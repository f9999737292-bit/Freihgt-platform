package dto

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/domain"
)

func TestFC_A_API_001_PlannedSummaryStableNullableJSONShape(t *testing.T) {
	t.Parallel()

	summary := plannedOnlySummary()
	raw, err := json.Marshal(ToCostSummaryResponse(summary))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	required := []string{
		"accrued_amount",
		"forecast_exposure",
		"current_actual_amount",
		"final_actual_amount",
		"billing_register_amount",
		"paid_amount",
		"current_variance_amount",
		"final_variance_amount",
		"billing_reconciliation_status",
	}
	for _, key := range required {
		if _, ok := payload[key]; !ok {
			t.Fatalf("missing key %q", key)
		}
	}
}

func TestFC_A_API_002_UnknownFinancialAmountsSerializeAsNull(t *testing.T) {
	t.Parallel()

	summary := plannedOnlySummary()
	raw, err := json.Marshal(ToCostSummaryResponse(summary))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["accrued_amount"] != nil {
		t.Fatalf("accrued_amount = %v", payload["accrued_amount"])
	}
}

func TestFC_A_API_003_KnownZeroSerializesAsDecimalString(t *testing.T) {
	t.Parallel()

	zero, err := domain.NewMoney(decimal.Zero, "RUB")
	if err != nil {
		t.Fatalf("new money: %v", err)
	}
	got := MoneyAmountToDTO(zero)
	if got == nil || *got != "0.00" {
		t.Fatalf("zero dto = %v", got)
	}
}

func plannedOnlySummary() *domain.CostSummary {
	planned, _ := domain.NewMoney(decimal.RequireFromString("150000.00"), "RUB")
	pricingModelVersion := domain.PricingModelVersionSnapshot
	return &domain.CostSummary{
		TenantID:          uuid.New(),
		TransportOrderID:  uuid.New(),
		BuyerCompanyID:    uuid.New(),
		CarrierCompanyID:  uuid.New(),
		CurrencyCode:      "RUB",
		PlannedAmount:     planned,
		DataStage:         domain.DataStagePlannedOnly,
		FinancialFinality: domain.FinancialFinalityNotEvaluated,
		SourcesAvailable:  []string{domain.SourceTypeTORateSnapshot},
		PlannedSourceRef: &domain.CanonicalSourceRef{
			SourceService:       domain.SourceServiceTransportOrder,
			SourceType:          domain.SourceTypeTORateSnapshot,
			SourceID:            uuid.New(),
			PricingModelVersion: &pricingModelVersion,
		},
	}
}
