package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestResolveRateCandidatesSingleMatch(t *testing.T) {
	base := decimal.RequireFromString("1000.00")
	pct := decimal.RequireFromString("10")
	lineID := uuid.New()
	contractID := uuid.New()
	cardID := uuid.New()
	versionID := uuid.New()
	origin := uuid.New()
	dest := uuid.New()
	pricingDate := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	candidate := RateCandidate{
		ContractID: contractID, ContractNumber: "C-1",
		ContractValidFrom: pricingDate.AddDate(0, -1, 0), ContractStatus: ContractStatusActive,
		ContractCurrency: "RUB", BuyerCompanyID: uuid.New(), CarrierCompanyID: uuid.New(),
		RateCardID: cardID, RateCardName: "Card", RateVersionID: versionID, VersionNumber: 1,
		VersionValidFrom: pricingDate.AddDate(0, -1, 0),
		RateLineID: lineID, OriginLocationID: origin, DestinationLocationID: dest,
		EquipmentType: "TAUTLINER", TransportMode: TransportModeRoad,
		Components: []RateComponent{
			{ComponentType: ComponentTypeBaseFreight, CalculationMethod: CalcMethodFlat, Amount: &base},
			{ComponentType: ComponentTypeFuelSurcharge, CalculationMethod: CalcMethodPercent, PercentValue: &pct},
		},
	}
	result := ResolveRateCandidates(ResolveRateRequest{
		TenantID: uuid.New(), BuyerCompanyID: candidate.BuyerCompanyID, CarrierCompanyID: candidate.CarrierCompanyID,
		OriginLocationID: origin, DestinationLocationID: dest,
		EquipmentType: "TAUTLINER", TransportMode: TransportModeRoad, PricingDate: pricingDate,
	}, []RateCandidate{candidate})
	if result.Status != ResolveStatusMatched {
		t.Fatalf("expected MATCHED, got %s", result.Status)
	}
	if result.PricingSource != PricingSourceContractRate {
		t.Fatalf("expected CONTRACT_RATE, got %s", result.PricingSource)
	}
	if result.TotalAmount == nil || *result.TotalAmount != "1100.00" {
		t.Fatalf("expected total 1100.00, got %v", result.TotalAmount)
	}
}

func TestResolveRateCandidatesAmbiguous(t *testing.T) {
	pricingDate := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	base := decimal.RequireFromString("100.00")
	makeCandidate := func() RateCandidate {
		amount := base
		return RateCandidate{
			ContractID: uuid.New(), ContractNumber: "C-1",
			ContractValidFrom: pricingDate, ContractStatus: ContractStatusActive, ContractCurrency: "RUB",
			BuyerCompanyID: uuid.New(), CarrierCompanyID: uuid.New(),
			RateCardID: uuid.New(), RateVersionID: uuid.New(), VersionNumber: 1,
			VersionValidFrom: pricingDate, RateLineID: uuid.New(),
			OriginLocationID: uuid.New(), DestinationLocationID: uuid.New(),
			EquipmentType: "TAUTLINER", TransportMode: TransportModeRoad,
			Components: []RateComponent{
				{ComponentType: ComponentTypeBaseFreight, CalculationMethod: CalcMethodFlat, Amount: &amount},
			},
		}
	}
	result := ResolveRateCandidates(ResolveRateRequest{PricingDate: pricingDate}, []RateCandidate{makeCandidate(), makeCandidate()})
	if result.Status != ResolveStatusAmbiguous {
		t.Fatalf("expected AMBIGUOUS, got %s", result.Status)
	}
}

func TestResolveRateCandidatesNoMatch(t *testing.T) {
	result := ResolveRateCandidates(ResolveRateRequest{}, nil)
	if result.Status != ResolveStatusNoMatch {
		t.Fatalf("expected NO_MATCH, got %s", result.Status)
	}
}

func TestCalculatePreExecutionTotalExcludesAccessorials(t *testing.T) {
	base := decimal.RequireFromString("1000.00")
	wait := decimal.RequireFromString("250.00")
	hour := UnitCodeHour
	components := []RateComponent{
		{ComponentType: ComponentTypeBaseFreight, CalculationMethod: CalcMethodFlat, Amount: &base},
		{ComponentType: ComponentTypeWaiting, CalculationMethod: CalcMethodUnitRate, Amount: &wait, UnitCode: &hour},
	}
	calc, err := CalculatePreExecutionTotal(components, "RUB")
	if err != nil {
		t.Fatalf("calc: %v", err)
	}
	if !calc.TotalAmount.Equal(decimal.RequireFromString("1000.00")) {
		t.Fatalf("expected total 1000.00, got %s", calc.TotalAmount)
	}
	if len(calc.AccessorialRules) != 1 {
		t.Fatalf("expected one accessorial rule, got %d", len(calc.AccessorialRules))
	}
}

func TestApplyManualSpotFallback(t *testing.T) {
	amount := decimal.RequireFromString("5000.00")
	currency := "RUB"
	req := ResolveRateRequest{
		ManualSpotAmount: &amount, ManualSpotCurrency: &currency,
		PricingDate: time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC),
	}
	base := ResolveRateResult{Status: ResolveStatusNoMatch}
	result, err := ApplyManualSpotFallback(req, base, true)
	if err != nil {
		t.Fatalf("apply manual spot: %v", err)
	}
	if result.PricingSource != PricingSourceManualSpot {
		t.Fatalf("expected MANUAL_SPOT, got %s", result.PricingSource)
	}
	if result.TotalAmount == nil || *result.TotalAmount != "5000.00" {
		t.Fatalf("unexpected total %v", result.TotalAmount)
	}
}

func TestValidateResolveRateRequestRejectsRFxIdentifiers(t *testing.T) {
	award := uuid.New()
	_, err := ValidateResolveRateRequest(ResolveRateRequest{AwardLinkID: &award, TenantID: uuid.New()})
	if err == nil {
		t.Fatal("expected RFx identifier rejection")
	}
	src := PricingSourceSpotBid
	_, err = ValidateResolveRateRequest(ResolveRateRequest{PricingSource: &src, TenantID: uuid.New()})
	if err == nil {
		t.Fatal("expected explicit pricing source rejection")
	}
}
