package shipmentevents

import (
	"time"
)

const SourceShipmentStatusHistory = "SHIPMENT_STATUS_HISTORY"

const (
	WarningShipmentStatusHistoryPartial     = "SHIPMENT_STATUS_HISTORY_PARTIAL"
	WarningShipmentStatusHistoryUnavailable = "SHIPMENT_STATUS_HISTORY_UNAVAILABLE"
)

type rawStatusHistory struct {
	ID              string
	ShipmentID      string
	ShipmentVersion int
	FromStatus      *string
	ToStatus        string
	ReasonCode      *string
	Source          string
	ActorType       string
	ActorID         *string
	CorrelationID   *string
	OccurredAt      time.Time
	RecordedAt      time.Time
}

type statusHistoryFetchResult struct {
	Complete bool
	Items    []rawStatusHistory
	Warnings []string
	NotFound bool
	Failed   bool
}

func buildCanonicalStatusHistoryEvents(shipment rawShipment, rows []rawStatusHistory) []ShipmentTimelineEvent {
	events := make([]ShipmentTimelineEvent, 0, len(rows))
	for _, row := range rows {
		eventType := mapStatusHistoryEventType(row)
		meta := statusHistoryMetadata(row)
		sourceEventID := row.ID
		var actor *ShipmentEventActor
		if row.ActorType != "" {
			actor = &ShipmentEventActor{Type: row.ActorType}
			if row.ActorID != nil && *row.ActorID != "" {
				actor.ID = row.ActorID
			}
		}
		recordedAt := row.RecordedAt.UTC()
		events = append(events, ShipmentTimelineEvent{
			ID:              row.ID,
			ShipmentID:      shipment.ID,
			ShipmentNumber:  shipment.ShipmentNumber,
			Type:            eventType,
			Category:        categoryForType(eventType),
			Source:          SourceShipmentStatusHistory,
			Severity:        severityForType(eventType),
			TitleCode:       titleCode(eventType),
			DescriptionCode: descriptionPtr(eventType),
			OccurredAt:      row.OccurredAt.UTC(),
			RecordedAt:      &recordedAt,
			Actor:           actor,
			Metadata:        meta,
			Derived:         false,
			CorrelationID:   row.CorrelationID,
			SourceEventID:   &sourceEventID,
		})
	}
	return events
}

func mapStatusHistoryEventType(row rawStatusHistory) string {
	if row.FromStatus == nil {
		return EventTypeShipmentCreated
	}
	switch row.ToStatus {
	case "CANCELLED":
		return EventTypeShipmentCancelled
	case "READY_FOR_BILLING":
		return EventTypeReadyForBilling
	case "DOCUMENTS_COMPLETED":
		return EventTypeDocumentsCompleted
	case "FINANCIALLY_CLOSED":
		return EventTypeFinanciallyClosed
	default:
		return EventTypeShipmentStatusChanged
	}
}

func statusHistoryMetadata(row rawStatusHistory) map[string]interface{} {
	meta := map[string]interface{}{
		"toStatus":        row.ToStatus,
		"shipmentVersion": row.ShipmentVersion,
	}
	if row.FromStatus != nil {
		meta["fromStatus"] = *row.FromStatus
	}
	if row.ReasonCode != nil && *row.ReasonCode != "" {
		meta["reasonCode"] = *row.ReasonCode
	}
	return sanitizeMetadata(meta)
}

func descriptionPtr(eventType string) *string {
	desc := descriptionCode(eventType)
	return &desc
}

func buildDerivedShipmentEventsWithoutStatusDuplicates(shipment rawShipment, canonical []ShipmentTimelineEvent) []ShipmentTimelineEvent {
	events := buildDerivedShipmentEvents(shipment)
	if len(canonical) == 0 {
		return events
	}

	canonicalKeys := make(map[string]struct{})
	for _, event := range canonical {
		if event.Type == EventTypeShipmentCreated {
			canonicalKeys["SHIPMENT_CREATED"] = struct{}{}
		}
	}

	filtered := make([]ShipmentTimelineEvent, 0, len(events))
	for _, event := range events {
		if event.Type == EventTypeShipmentCreated {
			if _, skip := canonicalKeys["SHIPMENT_CREATED"]; skip {
				continue
			}
		}
		filtered = append(filtered, event)
	}
	return filtered
}

func removeDerivedStatusTransitionDuplicates(events []ShipmentTimelineEvent) []ShipmentTimelineEvent {
	canonicalTransitions := make(map[string]struct{})
	for _, event := range events {
		if event.Derived || event.Source != SourceShipmentStatusHistory {
			continue
		}
		if event.Type == EventTypeShipmentStatusChanged || event.Type == EventTypeShipmentCancelled ||
			event.Type == EventTypeReadyForBilling || event.Type == EventTypeDocumentsCompleted ||
			event.Type == EventTypeFinanciallyClosed || event.Type == EventTypeShipmentCreated {
			key := canonicalTransitionKey(event)
			canonicalTransitions[key] = struct{}{}
		}
	}

	if len(canonicalTransitions) == 0 {
		return events
	}

	filtered := make([]ShipmentTimelineEvent, 0, len(events))
	for _, event := range events {
		if !event.Derived {
			filtered = append(filtered, event)
			continue
		}
		if event.Type == EventTypeShipmentCreated {
			if _, exists := canonicalTransitions["created"]; exists {
				continue
			}
		}
		filtered = append(filtered, event)
	}
	return filtered
}

func canonicalTransitionKey(event ShipmentTimelineEvent) string {
	toStatus := ""
	if event.Metadata != nil {
		if v, ok := event.Metadata["toStatus"]; ok {
			toStatus = fmtAny(v)
		}
	}
	return event.Type + "|" + toStatus + "|" + event.OccurredAt.UTC().Format(time.RFC3339Nano)
}
