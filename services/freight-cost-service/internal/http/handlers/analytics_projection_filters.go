package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/freight-cost-service/internal/platform/errors"
	"github.com/freight-platform/freight-cost-service/internal/repository"
)

func parseLaneListFilter(r *http.Request, tenantID uuid.UUID) (repository.AnalyticsLaneListFilter, error) {
	filter := repository.AnalyticsLaneListFilter{}
	if raw := strings.TrimSpace(r.URL.Query().Get("buyer_company_id")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return filter, apperrors.Validation("invalid buyer_company_id", map[string]any{"field": "buyer_company_id"})
		}
		filter.BuyerCompanyID = &id
	}
	filter.CurrencyCode = strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("currency_code")))
	filter.LaneKey = strings.TrimSpace(r.URL.Query().Get("lane_key"))
	filter.TransportMode = strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("transport_mode")))
	filter.EquipmentType = strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("equipment_type")))
	if raw := strings.TrimSpace(r.URL.Query().Get("period_from")); raw != "" {
		t, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return filter, apperrors.Validation("invalid period_from", map[string]any{"field": "period_from"})
		}
		filter.PeriodFrom = &t
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("period_to")); raw != "" {
		t, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return filter, apperrors.Validation("invalid period_to", map[string]any{"field": "period_to"})
		}
		filter.PeriodTo = &t
	}
	filter.Limit = parseLimitOffset(r, "limit", 50)
	filter.Offset = parseLimitOffset(r, "offset", 0)
	_ = tenantID
	return filter, nil
}

func parseCarrierListFilter(r *http.Request, tenantID uuid.UUID) (repository.AnalyticsCarrierListFilter, error) {
	filter := repository.AnalyticsCarrierListFilter{}
	if raw := strings.TrimSpace(r.URL.Query().Get("buyer_company_id")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return filter, apperrors.Validation("invalid buyer_company_id", map[string]any{"field": "buyer_company_id"})
		}
		filter.BuyerCompanyID = &id
	}
	filter.CurrencyCode = strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("currency_code")))
	if raw := strings.TrimSpace(r.URL.Query().Get("carrier_company_id")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return filter, apperrors.Validation("invalid carrier_company_id", map[string]any{"field": "carrier_company_id"})
		}
		filter.CarrierCompanyID = &id
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("period_from")); raw != "" {
		t, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return filter, apperrors.Validation("invalid period_from", map[string]any{"field": "period_from"})
		}
		filter.PeriodFrom = &t
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("period_to")); raw != "" {
		t, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return filter, apperrors.Validation("invalid period_to", map[string]any{"field": "period_to"})
		}
		filter.PeriodTo = &t
	}
	filter.Limit = parseLimitOffset(r, "limit", 50)
	filter.Offset = parseLimitOffset(r, "offset", 0)
	_ = tenantID
	return filter, nil
}

func parseLimitOffset(r *http.Request, field string, defaultValue int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(field))
	if raw == "" {
		return defaultValue
	}
	var value int
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return defaultValue
		}
		value = value*10 + int(ch-'0')
	}
	return value
}
