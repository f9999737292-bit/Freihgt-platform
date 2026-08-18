package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestBuildAwardConversionScopesMultiLot(t *testing.T) {
	lotA := uuid.New()
	lotB := uuid.New()
	lots := []RfxLot{
		{ID: lotA, LotNumber: "LOT-A"},
		{ID: lotB, LotNumber: "LOT-B"},
	}
	lines := []RfxResponseOfferLine{
		{RfxLotID: lotA, Amount: 100, CurrencyCode: "RUB"},
		{RfxLotID: lotB, Amount: 200, CurrencyCode: "RUB"},
	}
	scopes, err := BuildAwardConversionScopes(2, lots, lines, "RUB")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scopes) != 2 {
		t.Fatalf("expected 2 scopes, got %d", len(scopes))
	}
	amounts := map[uuid.UUID]float64{}
	for _, scope := range scopes {
		amounts[scope.RfxLotID] = scope.Amount
	}
	if amounts[lotA] != 100 || amounts[lotB] != 200 {
		t.Fatalf("unexpected amounts: %+v", amounts)
	}
}

func TestValidateAwardConversionEventStatus(t *testing.T) {
	if err := ValidateAwardConversionEventStatus(RfxStatusAwarded); err != nil {
		t.Fatalf("expected awarded status allowed: %v", err)
	}
	if err := ValidateAwardConversionEventStatus(RfxStatusPublished); err == nil {
		t.Fatal("expected published status denied")
	}
}
