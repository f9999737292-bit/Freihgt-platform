package legacyaggregate

import "testing"

func TestValidateCompleteLegacyAggregateMatch(t *testing.T) {
	t.Parallel()
	err := ValidateCompleteLegacyAggregate(AggregateSummary{
		TotalShipments:   3,
		CountedShipments: 3,
		ByStatus:         map[string]int64{"IN_TRANSIT": 2, "DELIVERED": 1},
		Complete:         true,
	})
	if err != nil {
		t.Fatalf("expected valid aggregate, got %v", err)
	}
}

func TestValidateCompleteLegacyAggregateRejectsIncomplete(t *testing.T) {
	t.Parallel()
	err := ValidateCompleteLegacyAggregate(AggregateSummary{
		TotalShipments:   3,
		CountedShipments: 2,
		ByStatus:         map[string]int64{"IN_TRANSIT": 2},
		Complete:         false,
	})
	if err == nil {
		t.Fatal("expected incomplete aggregate rejection")
	}
}

func TestValidateCompleteLegacyAggregateRejectsUnknownStatus(t *testing.T) {
	t.Parallel()
	err := ValidateCompleteLegacyAggregate(AggregateSummary{
		TotalShipments:   1,
		CountedShipments: 1,
		ByStatus:         map[string]int64{"UNKNOWN": 1},
		Complete:         true,
	})
	if err == nil {
		t.Fatal("expected unknown status rejection")
	}
}

func TestValidateAggregateContractAllowsIncompleteArithmetic(t *testing.T) {
	t.Parallel()
	err := ValidateAggregateContract(AggregateSummary{
		TotalShipments:   3,
		CountedShipments: 2,
		ByStatus:         map[string]int64{"IN_TRANSIT": 2},
		Complete:         false,
	})
	if err != nil {
		t.Fatalf("expected structurally valid incomplete aggregate, got %v", err)
	}
}
