package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/repository"
)

const WarningUnknownShipmentStatus = "UNKNOWN_SHIPMENT_STATUS"

type StatusSummaryRepository interface {
	GetStatusSummary(ctx context.Context, tenantID uuid.UUID) ([]repository.StatusSummaryRow, error)
}

type StatusSummary struct {
	TotalShipments   int64
	CountedShipments int64
	ByStatus         map[string]int64
	Complete         bool
	CalculatedAt     time.Time
	Warnings         []string
}

type StatusSummaryService struct {
	repo StatusSummaryRepository
	now  func() time.Time
}

func NewStatusSummaryService(repo StatusSummaryRepository) *StatusSummaryService {
	return &StatusSummaryService{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

func (s *StatusSummaryService) GetStatusSummary(ctx context.Context, tenantID uuid.UUID) (StatusSummary, error) {
	if tenantID == uuid.Nil {
		return StatusSummary{}, apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}

	rows, err := s.repo.GetStatusSummary(ctx, tenantID)
	if err != nil {
		return StatusSummary{}, apperrors.Internal("shipment status aggregate is temporarily unavailable", err)
	}

	summary := StatusSummary{
		ByStatus:     map[string]int64{},
		Complete:     true,
		CalculatedAt: s.now(),
		Warnings:     []string{},
	}

	var total int64
	unknownSeen := false
	for _, row := range rows {
		if total == 0 {
			total = row.TotalShipments
		}
		if !domain.IsValidShipmentStatus(row.Status) {
			summary.Complete = false
			if !unknownSeen {
				summary.Warnings = append(summary.Warnings, WarningUnknownShipmentStatus)
				unknownSeen = true
			}
			continue
		}
		if row.ShipmentCount < 0 {
			return StatusSummary{}, apperrors.Internal("invalid shipment count in aggregate", nil)
		}
		summary.ByStatus[row.Status] = row.ShipmentCount
	}

	var counted int64
	for _, count := range summary.ByStatus {
		counted += count
	}
	if total == 0 {
		total = counted
	}
	summary.TotalShipments = total
	summary.CountedShipments = counted
	if counted != total {
		summary.Complete = false
	}
	return summary, nil
}
