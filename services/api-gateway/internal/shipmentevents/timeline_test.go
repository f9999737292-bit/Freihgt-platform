package shipmentevents

import (
	"testing"
	"time"
)

func TestStableSortingDesc(t *testing.T) {
	base := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	events := []ShipmentTimelineEvent{
		{ID: "b", OccurredAt: base, Source: "B", Type: "B"},
		{ID: "a", OccurredAt: base, Source: "A", Type: "A"},
		{ID: "c", OccurredAt: base.Add(time.Hour), Source: "A", Type: "A"},
	}
	sortEvents(events, "desc")
	if events[0].ID != "c" || events[1].ID != "b" || events[2].ID != "a" {
		t.Fatalf("unexpected desc order: %#v", events)
	}
}

func TestStableSortingAsc(t *testing.T) {
	base := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	events := []ShipmentTimelineEvent{
		{ID: "b", OccurredAt: base, Source: "B", Type: "B"},
		{ID: "a", OccurredAt: base, Source: "A", Type: "A"},
		{ID: "c", OccurredAt: base.Add(-time.Hour), Source: "A", Type: "A"},
	}
	sortEvents(events, "asc")
	if events[0].ID != "c" || events[1].ID != "a" || events[2].ID != "b" {
		t.Fatalf("unexpected asc order: %#v", events)
	}
}

func TestDeterministicDerivedEventID(t *testing.T) {
	at := time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC)
	id1 := deterministicEventID("ship-1", EventTypeShipmentCreated, at, "", SourceShipmentState)
	id2 := deterministicEventID("ship-1", EventTypeShipmentCreated, at, "", SourceShipmentState)
	if id1 != id2 || id1 == "" {
		t.Fatalf("expected stable id, got %q and %q", id1, id2)
	}
}

func TestDeduplicationPrefersCanonicalOverDerived(t *testing.T) {
	at := time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC)
	docID := "doc-1"
	canonicalID := "canonical-event-id"
	events := []ShipmentTimelineEvent{
		{
			ID: "derived", ShipmentID: "s1", Type: EventTypeDocumentCreated, OccurredAt: at,
			Derived: true, Metadata: map[string]interface{}{"documentId": docID},
		},
		{
			ID: canonicalID, ShipmentID: "s1", Type: EventTypeDocumentCreated, OccurredAt: at,
			Derived: false, Metadata: map[string]interface{}{"documentId": docID},
		},
	}
	deduped := dedupeEvents(events)
	if len(deduped) != 1 || deduped[0].ID != canonicalID {
		t.Fatalf("expected canonical event to win, got %#v", deduped)
	}
}

func TestFiltersAppliedBeforePagination(t *testing.T) {
	base := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	events := []ShipmentTimelineEvent{
		{ID: "1", Type: EventTypeShipmentCreated, Category: CategoryShipment, Source: SourceShipmentState, Severity: SeverityInfo, OccurredAt: base},
		{ID: "2", Type: EventTypePickupPlanned, Category: CategoryOperation, Source: SourceShipmentState, Severity: SeverityInfo, OccurredAt: base.Add(time.Hour)},
		{ID: "3", Type: EventTypeDeliveryPlanned, Category: CategoryOperation, Source: SourceShipmentState, Severity: SeverityInfo, OccurredAt: base.Add(2 * time.Hour)},
	}
	query := ListQuery{Category: CategoryOperation, Page: 1, Limit: 1, Order: "desc"}
	filtered := filterEvents(events, query)
	sortEvents(filtered, query.Order)
	page := paginateEvents(filtered, query.Page, query.Limit)
	if page.Total != 2 || len(page.Items) != 1 || page.Items[0].ID != "3" {
		t.Fatalf("unexpected pagination result: %#v", page)
	}
}
