package handlers

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
)

func TestGetOwnResponseSerializesOfferLines(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile("carrier_rfx_handler.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "toRfxResponseDetailResponse(response, nil)") {
		t.Fatal("GetOwnResponse must serialize offer_lines through toRfxResponseDetailResponse")
	}

	lotID := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	resp := toRfxResponseDetailResponse(&domain.RfxResponse{
		ID:                   uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"),
		TenantID:             uuid.MustParse("cccccccc-cccc-4ccc-8ccc-cccccccccccc"),
		RfxEventID:           uuid.MustParse("dddddddd-dddd-4ddd-8ddd-dddddddddddd"),
		ParticipantCompanyID: uuid.MustParse("eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"),
		Status:               domain.RfxResponseStatusDraft,
		CreatedAt:            time.Unix(0, 0).UTC(),
		UpdatedAt:            time.Unix(0, 0).UTC(),
		Version:              1,
		OfferLines: []domain.RfxResponseOfferLine{{
			ID:           uuid.MustParse("ffffffff-ffff-4fff-8fff-ffffffffffff"),
			RfxLotID:     lotID,
			Amount:       15000,
			CurrencyCode: "RUB",
		}},
	}, nil)
	if resp["status"] != domain.RfxResponseStatusDraft {
		t.Fatalf("status=%v", resp["status"])
	}
	lines, ok := resp["offer_lines"].([]map[string]any)
	if !ok || len(lines) != 1 {
		t.Fatalf("offer_lines=%v", resp["offer_lines"])
	}
	if lines[0]["rfx_lot_id"] != lotID.String() {
		t.Fatalf("rfx_lot_id=%v", lines[0]["rfx_lot_id"])
	}
	if lines[0]["amount"] != 15000.0 {
		t.Fatalf("amount=%v", lines[0]["amount"])
	}
	if lines[0]["currency_code"] != "RUB" {
		t.Fatalf("currency_code=%v", lines[0]["currency_code"])
	}
}
