package controltower

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
)

var validSLAStatuses = map[string]SLAStatus{
	"ON_TIME":  SLAStatusOnTime,
	"AT_RISK":  SLAStatusAtRisk,
	"DELAYED":  SLAStatusDelayed,
	"CRITICAL": SLAStatusCritical,
	"UNKNOWN":  SLAStatusUnknown,
}

func ParseListQuery(r *http.Request) (ListQuery, error) {
	q := ListQuery{
		Q:         strings.TrimSpace(r.URL.Query().Get("q")),
		Status:    strings.TrimSpace(r.URL.Query().Get("status")),
		SLAStatus: strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("sla_status"))),
		ShipperID: strings.TrimSpace(r.URL.Query().Get("shipper_id")),
		CarrierID: strings.TrimSpace(r.URL.Query().Get("carrier_id")),
		Page:      1,
		Limit:     50,
	}

	if raw := strings.TrimSpace(r.URL.Query().Get("page")); raw != "" {
		page, err := strconv.Atoi(raw)
		if err != nil || page < 1 {
			return ListQuery{}, apperrors.Validation("page must be >= 1", map[string]any{"field": "page"})
		}
		q.Page = page
	}

	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 {
			return ListQuery{}, apperrors.Validation("limit must be >= 1", map[string]any{"field": "limit"})
		}
		if limit > 200 {
			return ListQuery{}, apperrors.Validation("limit must be <= 200", map[string]any{"field": "limit"})
		}
		q.Limit = limit
	}

	if q.SLAStatus != "" {
		if _, ok := validSLAStatuses[q.SLAStatus]; !ok {
			return ListQuery{}, apperrors.Validation("unknown sla_status", map[string]any{"field": "sla_status"})
		}
	}

	if q.Status != "" && !IsKnownShipmentStatus(q.Status) {
		return ListQuery{}, apperrors.Validation("unknown shipment status", map[string]any{"field": "status"})
	}

	if raw := strings.TrimSpace(r.URL.Query().Get("date_from")); raw != "" {
		parsed, err := parseDate(raw, "date_from")
		if err != nil {
			return ListQuery{}, err
		}
		q.DateFrom = parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("date_to")); raw != "" {
		parsed, err := parseDate(raw, "date_to")
		if err != nil {
			return ListQuery{}, err
		}
		end := parsed.Add(24*time.Hour - time.Nanosecond)
		q.DateTo = &end
	}
	if q.DateFrom != nil && q.DateTo != nil && q.DateFrom.After(*q.DateTo) {
		return ListQuery{}, apperrors.Validation("date_from must not be after date_to", map[string]any{
			"field": "date_from",
		})
	}

	if raw := strings.TrimSpace(r.URL.Query().Get("critical_only")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return ListQuery{}, apperrors.Validation("critical_only must be a boolean", map[string]any{"field": "critical_only"})
		}
		q.CriticalOnly = parsed
	}

	q.EventStatus = strings.TrimSpace(r.URL.Query().Get("event_status"))
	q.Priority = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("priority")))
	q.ExceptionCategory = strings.TrimSpace(r.URL.Query().Get("exception_category"))
	q.BusinessImpact = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("business_impact")))
	q.EventSLAStatus = strings.TrimSpace(r.URL.Query().Get("event_sla_status"))
	q.EscalationLevel = strings.TrimSpace(r.URL.Query().Get("escalation_level"))
	if raw := strings.TrimSpace(r.URL.Query().Get("unassigned_only")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return ListQuery{}, apperrors.Validation("unassigned_only must be a boolean", map[string]any{"field": "unassigned_only"})
		}
		q.UnassignedOnly = parsed
	}

	if q.Priority != "" && q.Priority != PriorityP1 && q.Priority != PriorityP2 && q.Priority != PriorityP3 && q.Priority != PriorityP4 {
		return ListQuery{}, apperrors.Validation("unknown priority", map[string]any{"field": "priority"})
	}
	if q.EventSLAStatus != "" && q.EventSLAStatus != SLAStatusWithinSLA && q.EventSLAStatus != SLAStatusWarning && q.EventSLAStatus != SLAStatusBreached && q.EventSLAStatus != SLAStatusCompleted {
		return ListQuery{}, apperrors.Validation("unknown event_sla_status", map[string]any{"field": "event_sla_status"})
	}

	return q, nil
}

func parseDate(raw, field string) (*time.Time, error) {
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, apperrors.Validation("invalid date format, expected YYYY-MM-DD", map[string]any{"field": field})
	}
	utc := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC)
	return &utc, nil
}

func ApplyFilters(rows []ControlTowerShipment, query ListQuery) []ControlTowerShipment {
	search := strings.ToLower(strings.TrimSpace(query.Q))
	filtered := make([]ControlTowerShipment, 0, len(rows))

	for _, row := range rows {
		if !IsActiveShipmentStatus(row.Status) {
			continue
		}
		if query.CriticalOnly && row.SLAStatus != SLAStatusCritical && row.SLAStatus != SLAStatusDelayed {
			continue
		}
		if query.Status != "" && row.Status != query.Status {
			continue
		}
		if query.SLAStatus != "" && string(row.SLAStatus) != query.SLAStatus {
			continue
		}
		if query.ShipperID != "" && (row.ShipperID == nil || *row.ShipperID != query.ShipperID) {
			continue
		}
		if query.CarrierID != "" && (row.CarrierID == nil || *row.CarrierID != query.CarrierID) {
			continue
		}
		if search != "" && !matchesSearch(row, search) {
			continue
		}
		if query.DateFrom != nil || query.DateTo != nil {
			if !matchesDateRange(row, query.DateFrom, query.DateTo) {
				continue
			}
		}
		filtered = append(filtered, row)
	}
	return filtered
}

func matchesSearch(row ControlTowerShipment, search string) bool {
	parts := []string{
		row.ShipmentNumber,
		derefString(row.TransportOrderNumber),
		derefString(row.ShipperName),
		derefString(row.CarrierName),
		derefString(row.OriginName),
		derefString(row.DestinationName),
	}
	haystack := strings.ToLower(strings.Join(parts, " "))
	return strings.Contains(haystack, search)
}

func matchesDateRange(row ControlTowerShipment, from, to *time.Time) bool {
	candidates := []*time.Time{row.PlannedPickupAt, row.PlannedDeliveryAt, row.ActualPickupAt, row.ActualDeliveryAt}
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		if from != nil && candidate.Before(*from) {
			continue
		}
		if to != nil && candidate.After(*to) {
			continue
		}
		return true
	}
	return false
}

func Paginate(rows []ControlTowerShipment, page, limit int) ShipmentsPage {
	total := len(rows)
	if total == 0 {
		return ShipmentsPage{Items: []ControlTowerShipment{}, Page: page, Limit: limit, Total: 0, HasNext: false}
	}

	start := (page - 1) * limit
	if start >= total {
		return ShipmentsPage{Items: []ControlTowerShipment{}, Page: page, Limit: limit, Total: total, HasNext: false}
	}
	end := start + limit
	if end > total {
		end = total
	}
	items := rows[start:end]
	hasNext := end < total
	return ShipmentsPage{Items: items, Page: page, Limit: limit, Total: total, HasNext: hasNext}
}

func CalculateKPI(rows []ControlTowerShipment) KPI {
	kpi := KPI{}
	for _, row := range rows {
		if !IsActiveShipmentStatus(row.Status) {
			continue
		}
		kpi.Active++
		switch row.SLAStatus {
		case SLAStatusOnTime:
			kpi.OnTime++
		case SLAStatusAtRisk:
			kpi.AtRisk++
		case SLAStatusDelayed:
			kpi.Delayed++
		case SLAStatusCritical:
			kpi.Critical++
		}
		if row.Status == "DELIVERED" || row.Status == "DELIVERY_CONFIRMED" {
			kpi.AwaitingDocuments++
		}
		if row.ReadyForBilling {
			kpi.ReadyForBilling++
		}
	}
	return kpi
}

func BuildFilterOptions(allRows []ControlTowerShipment, companies []rawCompany, companiesLoaded bool) FiltersResponse {
	statusSet := map[string]struct{}{}
	shipperSet := map[string]string{}
	carrierSet := map[string]string{}

	for _, row := range allRows {
		if IsActiveShipmentStatus(row.Status) {
			statusSet[row.Status] = struct{}{}
		}
		if row.ShipperID != nil && row.ShipperName != nil {
			shipperSet[*row.ShipperID] = *row.ShipperName
		}
		if row.CarrierID != nil && row.CarrierName != nil {
			carrierSet[*row.CarrierID] = *row.CarrierName
		}
	}

	if companiesLoaded {
		for _, company := range companies {
			label := companyLabel(company)
			switch company.CompanyType {
			case "SHIPPER":
				shipperSet[company.ID] = label
			case "CARRIER":
				carrierSet[company.ID] = label
			}
		}
	}

	return FiltersResponse{
		Statuses: mapToFilterOptions(statusSet),
		Shippers: idLabelToFilterOptions(shipperSet),
		Carriers: idLabelToFilterOptions(carrierSet),
	}
}

func mapToFilterOptions(set map[string]struct{}) []FilterOption {
	items := make([]FilterOption, 0, len(set))
	for value := range set {
		items = append(items, FilterOption{Value: value, Label: value})
	}
	sortFilterOptions(items)
	return items
}

func idLabelToFilterOptions(set map[string]string) []FilterOption {
	items := make([]FilterOption, 0, len(set))
	for value, label := range set {
		items = append(items, FilterOption{Value: value, Label: label})
	}
	sortFilterOptions(items)
	return items
}

func sortFilterOptions(items []FilterOption) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if strings.ToLower(items[j].Label) < strings.ToLower(items[i].Label) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
