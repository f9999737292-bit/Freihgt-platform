package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/low-code-service/internal/domain"
	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
)

type ConfigurationAuditStore interface {
	List(ctx context.Context, filter domain.ListAuditEventsFilter) ([]domain.ConfigurationAuditEntry, error)
}

type AuditService struct {
	audit ConfigurationAuditStore
}

func NewAuditService(audit ConfigurationAuditStore) *AuditService {
	return &AuditService{audit: audit}
}

func (s *AuditService) List(ctx context.Context, filter domain.ListAuditEventsFilter) ([]domain.ConfigurationAuditEntry, error) {
	if filter.TenantID == uuid.Nil {
		return nil, apperrors.TenantRequired()
	}
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	return s.audit.List(ctx, filter)
}
