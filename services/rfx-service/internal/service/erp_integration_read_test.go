package service

import (
	"encoding/json"
	"testing"

	"github.com/freight-platform/rfx-service/internal/erpjson"
)

func TestErpRfxDraftSummaryOmitsForbiddenFields(t *testing.T) {
	raw, err := json.Marshal(ErpRfxDraftSummary{Status: "DRAFT", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"participants", "carriers", "bids", "scores", "awards", "actor_id"} {
		if _, ok := out[key]; ok {
			t.Fatalf("forbidden field %s present", key)
		}
	}
	readiness, _ := json.Marshal(ErpPublishReadinessSummary{Ready: true})
	if string(readiness) != `{"ready":true,"blocking_fail_count":0,"warning_count":0}` {
		t.Fatalf("readiness=%s", readiness)
	}
}

func TestCapabilitiesDeferredFieldsFrozen(t *testing.T) {
	found := map[string]bool{}
	for _, field := range erpjson.DeferredFieldsV1() {
		found[field] = true
	}
	for _, want := range []string{"lanes", "cargo", "invited_carriers", "mapping_context"} {
		if !found[want] {
			t.Fatalf("missing deferred field %s", want)
		}
	}
}
