package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBuildSettlementSnapshotPayloadsUsesExactDecimalMoney(t *testing.T) {
	settlement := &FreightSettlement{
		ID:               uuid.New(),
		TenantID:         uuid.New(),
		TransportOrderID: uuid.New(),
		ShipmentID:       uuid.New(),
		Status:           SettlementStatusApproved,
		Version:          3,
		CurrencyCode:     "RUB",
	}
	occurredAt := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	accrual := "1000.01"
	total := "1500.99"

	ids, payloads, err := BuildSettlementSnapshotPayloads(
		[]string{EventFreightSettlementAccrualSnapshot, EventFreightSettlementCurrentActualSnapshot},
		settlement, 0, occurredAt, accrual, total,
	)
	if err != nil {
		t.Fatalf("build payloads: %v", err)
	}
	if len(ids) != 2 || len(payloads) != 2 {
		t.Fatalf("expected 2 payloads, got ids=%d payloads=%d", len(ids), len(payloads))
	}

	var accrualPayload settlementSnapshotPayload
	if err := json.Unmarshal(payloads[0], &accrualPayload); err != nil {
		t.Fatalf("unmarshal accrual: %v", err)
	}
	if accrualPayload.Amount == nil || *accrualPayload.Amount != accrual {
		t.Fatalf("accrual amount = %v, want %s", accrualPayload.Amount, accrual)
	}

	var actualPayload settlementSnapshotPayload
	if err := json.Unmarshal(payloads[1], &actualPayload); err != nil {
		t.Fatalf("unmarshal actual: %v", err)
	}
	if actualPayload.Amount == nil || *actualPayload.Amount != total {
		t.Fatalf("actual amount = %v, want %s", actualPayload.Amount, total)
	}
}
