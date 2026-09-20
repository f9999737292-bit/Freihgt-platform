package handlers

import (
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
)

func TestToRfxEventDetailResponseSerializesStoredChannels(t *testing.T) {
	t.Parallel()
	event := &domain.RfxEvent{
		ID:             uuid.MustParse("11111111-1111-4111-8111-111111111111"),
		TenantID:       uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"),
		OwnerCompanyID: uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"),
		RfxNumber:      "RFX-F4-1",
		RfxType:        "LANE_TENDER",
		Category:       "FREIGHT",
		Title:          "Human provenance",
		Status:         domain.RfxStatusDraft,
	}
	for _, channel := range []string{
		domain.CreationChannelManual,
		domain.CreationChannelTemplate,
		domain.CreationChannelExcel,
		domain.CreationChannelERP,
	} {
		resp := toRfxEventDetailResponse(event, nil, channel)
		if got, ok := resp["creation_channel"].(string); !ok || got != channel {
			t.Fatalf("creation_channel=%v want %q", resp["creation_channel"], channel)
		}
		if _, ok := resp["external_link"]; ok {
			t.Fatal("human GET must not include external_link")
		}
	}
}

func TestToRfxEventDetailResponseOmitsEmptyHistoricalChannel(t *testing.T) {
	t.Parallel()
	event := &domain.RfxEvent{
		ID:             uuid.MustParse("11111111-1111-4111-8111-111111111111"),
		TenantID:       uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"),
		OwnerCompanyID: uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"),
		RfxNumber:      "RFX-F4-HIST",
		Title:          "Historical",
		Status:         domain.RfxStatusDraft,
	}
	resp := toRfxEventDetailResponse(event, nil, "   ")
	if _, ok := resp["creation_channel"]; ok {
		t.Fatalf("empty stored channel must be omitted, got %v", resp["creation_channel"])
	}
}

func TestToRfxEventResponseDoesNotSerializeCreationChannel(t *testing.T) {
	t.Parallel()
	event := &domain.RfxEvent{
		ID:             uuid.MustParse("11111111-1111-4111-8111-111111111111"),
		TenantID:       uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"),
		OwnerCompanyID: uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"),
		RfxNumber:      "RFX-F4-LIST",
		Title:          "List item",
		Status:         domain.RfxStatusDraft,
	}
	resp := toRfxEventResponse(event)
	if _, ok := resp["creation_channel"]; ok {
		t.Fatal("list/create/update DTO must not include creation_channel")
	}
}
