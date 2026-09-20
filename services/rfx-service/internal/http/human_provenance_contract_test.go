package http

import (
	"strings"
	"testing"
)

func TestHumanGetRfxEventOpenAPIHasCreationChannelOnly(t *testing.T) {
	t.Parallel()
	rfxOpenAPI, err := readExcelExchangeRepoFile(t, "packages/openapi/rfx-service.yaml")
	if err != nil {
		t.Fatalf("read rfx openapi: %v", err)
	}
	unified, err := readExcelExchangeRepoFile(t, "packages/openapi/openapi.yaml")
	if err != nil {
		t.Fatalf("read unified openapi: %v", err)
	}

	detailStart := strings.Index(rfxOpenAPI, "    RfxEventDetailResponse:")
	if detailStart < 0 {
		t.Fatal("RfxEventDetailResponse missing from rfx-service OpenAPI")
	}
	detailEnd := strings.Index(rfxOpenAPI[detailStart:], "    CarrierInvitedEventResponse:")
	if detailEnd < 0 {
		t.Fatal("RfxEventDetailResponse block boundary missing")
	}
	detail := rfxOpenAPI[detailStart : detailStart+detailEnd]
	if !strings.Contains(detail, "creation_channel:") {
		t.Fatal("RfxEventDetailResponse must include creation_channel")
	}
	for _, value := range []string{"MANUAL", "TEMPLATE", "EXCEL", "ERP"} {
		if !strings.Contains(detail, value) {
			t.Fatalf("RfxEventDetailResponse creation_channel must allow %s", value)
		}
	}
	if strings.Contains(detail, "external_link") {
		t.Fatal("human GET DTO must not add external_link")
	}

	getBlockStart := strings.Index(rfxOpenAPI, "  /api/v1/rfx-events/{id}:")
	if getBlockStart < 0 {
		t.Fatal("human GET /api/v1/rfx-events/{id} missing")
	}
	getBlock := rfxOpenAPI[getBlockStart:]
	if !strings.Contains(getBlock, "RfxEventDetailResponse") {
		t.Fatal("human GET must keep RfxEventDetailResponse")
	}
	if !strings.Contains(unified, "creation_channel:") {
		t.Fatal("unified OpenAPI must include creation_channel after generation")
	}
}
