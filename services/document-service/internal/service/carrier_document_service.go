package service

import (
	"context"
	"io"

	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
)

type carrierDocuments interface {
	CarrierShipmentVisible(ctx context.Context, tenantID, shipmentID, carrierCompanyID uuid.UUID) (bool, error)
	ListCarrierDocuments(ctx context.Context, filter domain.CarrierDocumentFilter) ([]domain.Document, int, error)
	GetCarrierDocument(ctx context.Context, tenantID, carrierCompanyID, documentID uuid.UUID) (*domain.Document, error)
}

type carrierFiles interface {
	ListFinalized(ctx context.Context, tenantID, documentID uuid.UUID) ([]domain.Attachment, error)
	GetFinalized(ctx context.Context, tenantID, documentID, attachmentID uuid.UUID) (*domain.Attachment, error)
	OpenFinalized(ctx context.Context, tenantID, documentID, attachmentID uuid.UUID) (*domain.Attachment, io.ReadCloser, error)
}

type CarrierDocumentService struct {
	docs  carrierDocuments
	files carrierFiles
}

func NewCarrierDocumentService(docs carrierDocuments, files carrierFiles) *CarrierDocumentService {
	return &CarrierDocumentService{docs: docs, files: files}
}

func (s *CarrierDocumentService) List(ctx context.Context, filter domain.CarrierDocumentFilter) ([]domain.Document, int, error) {
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if err := domain.ValidateCarrierDocumentFilter(filter); err != nil {
		return nil, 0, err
	}
	visible, err := s.docs.CarrierShipmentVisible(ctx, filter.TenantID, filter.ShipmentID, filter.CarrierCompanyID)
	if err != nil {
		return nil, 0, err
	}
	if !visible {
		return nil, 0, apperrors.NotFound("shipment not found")
	}
	return s.docs.ListCarrierDocuments(ctx, filter)
}

func (s *CarrierDocumentService) Get(ctx context.Context, tenantID, carrierCompanyID, documentID uuid.UUID) (*domain.Document, error) {
	if err := requireCarrierScope(tenantID, carrierCompanyID, documentID, "document_id"); err != nil {
		return nil, err
	}
	return s.docs.GetCarrierDocument(ctx, tenantID, carrierCompanyID, documentID)
}

func (s *CarrierDocumentService) ListAttachments(ctx context.Context, tenantID, carrierCompanyID, documentID uuid.UUID) ([]domain.Attachment, error) {
	if _, err := s.Get(ctx, tenantID, carrierCompanyID, documentID); err != nil {
		return nil, err
	}
	return s.files.ListFinalized(ctx, tenantID, documentID)
}

func (s *CarrierDocumentService) GetAttachment(ctx context.Context, tenantID, carrierCompanyID, documentID, attachmentID uuid.UUID) (*domain.Attachment, error) {
	if _, err := s.Get(ctx, tenantID, carrierCompanyID, documentID); err != nil {
		return nil, err
	}
	if attachmentID == uuid.Nil {
		return nil, apperrors.Validation("attachment_id is required", map[string]any{"field": "attachment_id"})
	}
	return s.files.GetFinalized(ctx, tenantID, documentID, attachmentID)
}

func (s *CarrierDocumentService) OpenContent(ctx context.Context, tenantID, carrierCompanyID, documentID, attachmentID uuid.UUID) (*domain.Attachment, io.ReadCloser, error) {
	if _, err := s.Get(ctx, tenantID, carrierCompanyID, documentID); err != nil {
		return nil, nil, err
	}
	if attachmentID == uuid.Nil {
		return nil, nil, apperrors.Validation("attachment_id is required", map[string]any{"field": "attachment_id"})
	}
	return s.files.OpenFinalized(ctx, tenantID, documentID, attachmentID)
}

func requireCarrierScope(tenantID, carrierCompanyID, documentID uuid.UUID, documentField string) error {
	if tenantID == uuid.Nil {
		return apperrors.Unauthorized("trusted tenant context is required")
	}
	if carrierCompanyID == uuid.Nil {
		return apperrors.Validation("carrier_company_id is required", map[string]any{"field": "carrier_company_id"})
	}
	if documentID == uuid.Nil {
		return apperrors.Validation(documentField+" is required", map[string]any{"field": documentField})
	}
	return nil
}
