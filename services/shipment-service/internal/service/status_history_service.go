package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

type StatusHistoryStore interface {
	GetByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.Shipment, error)
	ListStatusHistory(ctx context.Context, filter domain.ListStatusHistoryFilter) ([]domain.ShipmentStatusHistory, int, error)
	HasInitialStatusHistory(ctx context.Context, tenantID, shipmentID uuid.UUID) (bool, error)
}

type StatusHistoryListResult struct {
	Shipment domain.Shipment
	Complete bool
	Items    []domain.ShipmentStatusHistory
	Page     int
	Limit    int
	Total    int
	HasNext  bool
	Warnings []string
}

type StatusHistoryService struct {
	store StatusHistoryStore
}

func NewStatusHistoryService(store StatusHistoryStore) *StatusHistoryService {
	return &StatusHistoryService{store: store}
}

func (s *StatusHistoryService) List(ctx context.Context, filter domain.ListStatusHistoryFilter) (StatusHistoryListResult, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Order == "" {
		filter.Order = "desc"
	}
	if err := domain.ValidateListStatusHistoryFilter(filter); err != nil {
		return StatusHistoryListResult{}, err
	}

	shipment, err := s.store.GetByIDAndTenant(ctx, filter.ShipmentID, filter.TenantID)
	if err != nil {
		return StatusHistoryListResult{}, err
	}

	items, total, err := s.store.ListStatusHistory(ctx, filter)
	if err != nil {
		return StatusHistoryListResult{}, err
	}

	complete, err := s.store.HasInitialStatusHistory(ctx, filter.TenantID, filter.ShipmentID)
	if err != nil {
		return StatusHistoryListResult{}, err
	}

	warnings := make([]string, 0)
	if !complete {
		warnings = append(warnings, domain.StatusHistoryWarningPartial)
	}

	hasNext := filter.Page*filter.Limit < total
	return StatusHistoryListResult{
		Shipment: *shipment,
		Complete: complete,
		Items:    items,
		Page:     filter.Page,
		Limit:    filter.Limit,
		Total:    total,
		HasNext:  hasNext,
		Warnings: warnings,
	}, nil
}

func ParseStatusHistoryListFilter(tenantID, shipmentID uuid.UUID, page, limit int, order string) (domain.ListStatusHistoryFilter, error) {
	if tenantID == uuid.Nil {
		return domain.ListStatusHistoryFilter{}, apperrors.Unauthorized("tenant context is required")
	}
	if shipmentID == uuid.Nil {
		return domain.ListStatusHistoryFilter{}, apperrors.Validation("shipment_id is required", map[string]any{"field": "shipment_id"})
	}
	filter := domain.ListStatusHistoryFilter{
		TenantID:   tenantID,
		ShipmentID: shipmentID,
		Page:       page,
		Limit:      limit,
		Order:      order,
	}
	if err := domain.ValidateListStatusHistoryFilter(filter); err != nil {
		return domain.ListStatusHistoryFilter{}, err
	}
	return filter, nil
}
