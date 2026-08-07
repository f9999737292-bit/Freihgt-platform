package shipmentevents

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
)

var validCategories = map[string]struct{}{
	CategoryShipment: {}, CategoryOperation: {}, CategoryDocument: {},
	CategorySLA: {}, CategoryBilling: {}, CategoryTechnical: {},
	CategoryGeolocation: {}, CategorySystem: {},
}

var validEventTypes = map[string]struct{}{
	EventTypeShipmentCreated: {}, EventTypeShipmentStatusChanged: {}, EventTypeShipmentCancelled: {},
	EventTypePickupPlanned: {}, EventTypePickupCompleted: {}, EventTypeDeliveryPlanned: {}, EventTypeDeliveryCompleted: {},
	EventTypeDocumentCreated: {}, EventTypeDocumentSigned: {}, EventTypeDocumentRejected: {},
	EventTypeDocumentsCompleted: {}, EventTypeDocumentsMissing: {},
	EventTypeReadyForBilling: {}, EventTypeBillingRegisterAdded: {}, EventTypeClosingDocumentsCreated: {},
	EventTypeSignedByCounterparty: {}, EventTypePaymentMarked: {}, EventTypeFinanciallyClosed: {},
	EventTypeSLAAtRisk: {}, EventTypeSLADelayed: {}, EventTypeSLACritical: {},
	EventTypeTechnicalProblem: {}, EventTypeGeolocationLost: {}, EventTypeRouteDeviation: {},
	EventTypeUnknownEvent: {},
}

var validSources = map[string]struct{}{
	SourceShipmentState: {}, SourceSLACalculator: {}, SourceDocumentState: {}, SourceBillingState: {},
}

var validSeverities = map[string]struct{}{
	SeverityInfo: {}, SeverityWarning: {}, SeverityCritical: {},
}

func ParseListQuery(r *http.Request) (ListQuery, error) {
	q := ListQuery{
		Type:     strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("type"))),
		Category: strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("category"))),
		Source:   strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("source"))),
		Severity: strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("severity"))),
		Order:    strings.ToLower(strings.TrimSpace(r.URL.Query().Get("order"))),
		Page:     1,
		Limit:    50,
	}

	if q.Order == "" {
		q.Order = "desc"
	}
	if q.Order != "asc" && q.Order != "desc" {
		return ListQuery{}, apperrors.Validation("invalid order", map[string]any{"field": "order"})
	}

	if raw := strings.TrimSpace(r.URL.Query().Get("page")); raw != "" {
		page, err := parsePositiveInt(raw, "page")
		if err != nil {
			return ListQuery{}, err
		}
		q.Page = page
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		limit, err := parsePositiveInt(raw, "limit")
		if err != nil {
			return ListQuery{}, err
		}
		if limit > 200 {
			return ListQuery{}, apperrors.Validation("limit must be at most 200", map[string]any{"field": "limit"})
		}
		q.Limit = limit
	}

	if raw := strings.TrimSpace(r.URL.Query().Get("date_from")); raw != "" {
		parsed, err := parseDate(raw, "date_from")
		if err != nil {
			return ListQuery{}, err
		}
		q.DateFrom = &parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("date_to")); raw != "" {
		parsed, err := parseDateEnd(raw, "date_to")
		if err != nil {
			return ListQuery{}, err
		}
		q.DateTo = &parsed
	}
	if q.DateFrom != nil && q.DateTo != nil && q.DateFrom.After(*q.DateTo) {
		return ListQuery{}, apperrors.Validation("date_from must be before or equal to date_to", map[string]any{"field": "date_from"})
	}

	if raw := strings.TrimSpace(r.URL.Query().Get("derived")); raw != "" {
		parsed, err := parseBool(raw, "derived")
		if err != nil {
			return ListQuery{}, err
		}
		q.Derived = &parsed
	}

	if q.Type != "" {
		if _, ok := validEventTypes[q.Type]; !ok {
			return ListQuery{}, apperrors.Validation("unknown event type", map[string]any{"field": "type"})
		}
	}
	if q.Category != "" {
		if _, ok := validCategories[q.Category]; !ok {
			return ListQuery{}, apperrors.Validation("unknown category", map[string]any{"field": "category"})
		}
	}
	if q.Source != "" {
		if _, ok := validSources[q.Source]; !ok {
			return ListQuery{}, apperrors.Validation("unknown source", map[string]any{"field": "source"})
		}
	}
	if q.Severity != "" {
		if _, ok := validSeverities[q.Severity]; !ok {
			return ListQuery{}, apperrors.Validation("unknown severity", map[string]any{"field": "severity"})
		}
	}

	return q, nil
}

func ValidateShipmentID(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", apperrors.Validation("shipmentId is required", map[string]any{"field": "shipmentId"})
	}
	if _, err := uuid.Parse(raw); err != nil {
		return "", apperrors.Validation("invalid shipmentId format", map[string]any{"field": "shipmentId"})
	}
	return raw, nil
}

func parsePositiveInt(raw, field string) (int, error) {
	var value int
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return 0, apperrors.Validation("invalid integer", map[string]any{"field": field})
		}
		value = value*10 + int(ch-'0')
	}
	if value < 1 {
		return 0, apperrors.Validation("must be at least 1", map[string]any{"field": field})
	}
	return value, nil
}

func parseDate(raw, field string) (time.Time, error) {
	layouts := []string{"2006-01-02", time.RFC3339}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, apperrors.Validation("invalid date format", map[string]any{"field": field})
}

func parseDateEnd(raw, field string) (time.Time, error) {
	if len(raw) == 10 {
		parsed, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return time.Time{}, apperrors.Validation("invalid date format", map[string]any{"field": field})
		}
		end := parsed.Add(24*time.Hour - time.Nanosecond)
		return end.UTC(), nil
	}
	return parseDate(raw, field)
}

func parseBool(raw, field string) (bool, error) {
	switch strings.ToLower(raw) {
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	default:
		return false, apperrors.Validation("invalid boolean", map[string]any{"field": field})
	}
}

func filterEvents(events []ShipmentTimelineEvent, query ListQuery) []ShipmentTimelineEvent {
	filtered := make([]ShipmentTimelineEvent, 0, len(events))
	for _, event := range events {
		if query.Type != "" && event.Type != query.Type {
			continue
		}
		if query.Category != "" && event.Category != query.Category {
			continue
		}
		if query.Source != "" && event.Source != query.Source {
			continue
		}
		if query.Severity != "" && event.Severity != query.Severity {
			continue
		}
		if query.Derived != nil && event.Derived != *query.Derived {
			continue
		}
		if query.DateFrom != nil && event.OccurredAt.Before(*query.DateFrom) {
			continue
		}
		if query.DateTo != nil && event.OccurredAt.After(*query.DateTo) {
			continue
		}
		filtered = append(filtered, event)
	}
	return filtered
}

func sortEvents(events []ShipmentTimelineEvent, order string) {
	sort.SliceStable(events, func(i, j int) bool {
		a, b := events[i], events[j]
		if order == "asc" {
			if a.OccurredAt.Equal(b.OccurredAt) {
				if a.Source != b.Source {
					return a.Source < b.Source
				}
				if a.Type != b.Type {
					return a.Type < b.Type
				}
				return a.ID < b.ID
			}
			return a.OccurredAt.Before(b.OccurredAt)
		}
		if a.OccurredAt.Equal(b.OccurredAt) {
			if a.Source != b.Source {
				return a.Source > b.Source
			}
			if a.Type != b.Type {
				return a.Type > b.Type
			}
			return a.ID > b.ID
		}
		return a.OccurredAt.After(b.OccurredAt)
	})
}

func dedupeEvents(events []ShipmentTimelineEvent) []ShipmentTimelineEvent {
	type dedupeKey struct {
		shipmentID    string
		eventType     string
		occurredAt    string
		sourceEventID string
		relatedID     string
	}

	seen := make(map[dedupeKey]ShipmentTimelineEvent)
	order := make([]dedupeKey, 0, len(events))

	for _, event := range events {
		relatedID := ""
		if event.Metadata != nil {
			if v, ok := event.Metadata["documentId"]; ok {
				relatedID = fmtAny(v)
			} else if v, ok := event.Metadata["billingRegisterId"]; ok {
				relatedID = fmtAny(v)
			}
		}
		sourceEventID := ""
		if event.SourceEventID != nil {
			sourceEventID = *event.SourceEventID
		}
		key := dedupeKey{
			shipmentID:    event.ShipmentID,
			eventType:     event.Type,
			occurredAt:    event.OccurredAt.UTC().Format(time.RFC3339Nano),
			sourceEventID: sourceEventID,
			relatedID:     relatedID,
		}

		existing, ok := seen[key]
		if !ok {
			seen[key] = event
			order = append(order, key)
			continue
		}
		if existing.Derived && !event.Derived {
			seen[key] = event
		}
	}

	result := make([]ShipmentTimelineEvent, 0, len(order))
	for _, key := range order {
		result = append(result, seen[key])
	}
	return result
}

func paginateEvents(events []ShipmentTimelineEvent, page, limit int) TimelinePage {
	total := len(events)
	start := (page - 1) * limit
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return TimelinePage{
		Items:   events[start:end],
		Page:    page,
		Limit:   limit,
		Total:   total,
		HasNext: end < total,
	}
}

func buildFilterOptions(events []ShipmentTimelineEvent) FiltersResponse {
	typeSet := map[string]struct{}{}
	categorySet := map[string]struct{}{}
	sourceSet := map[string]struct{}{}
	severitySet := map[string]struct{}{}

	for _, event := range events {
		typeSet[event.Type] = struct{}{}
		categorySet[event.Category] = struct{}{}
		sourceSet[event.Source] = struct{}{}
		severitySet[event.Severity] = struct{}{}
	}

	return FiltersResponse{
		Types:      toFilterOptions(typeSet),
		Categories: toFilterOptions(categorySet),
		Sources:    toFilterOptions(sourceSet),
		Severities: toFilterOptions(severitySet),
	}
}

func toFilterOptions(values map[string]struct{}) []FilterOption {
	items := make([]FilterOption, 0, len(values))
	for value := range values {
		items = append(items, FilterOption{Value: value, Label: value})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Value < items[j].Value })
	return items
}

func fmtAny(v any) string {
	switch typed := v.(type) {
	case string:
		return typed
	default:
		return ""
	}
}
