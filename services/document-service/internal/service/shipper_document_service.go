package service

import (
	"context"
	"io"

	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
)

type shipperDocuments interface {
	ShipperShipmentVisible(ctx context.Context, tenantID, shipmentID, shipperCompanyID uuid.UUID) (bool, error)
	ListShipperDocuments(ctx context.Context, filter domain.ShipperDocumentFilter) ([]domain.Document, int, error)
	GetShipperDocument(ctx context.Context, tenantID, shipperCompanyID, documentID uuid.UUID) (*domain.Document, error)
}

type shipperFiles interface {
	ListFinalized(ctx context.Context, tenantID, documentID uuid.UUID) ([]domain.Attachment, error)
	GetFinalized(ctx context.Context, tenantID, documentID, attachmentID uuid.UUID) (*domain.Attachment, error)
	OpenFinalized(ctx context.Context, tenantID, documentID, attachmentID uuid.UUID) (*domain.Attachment, io.ReadCloser, error)
}

type ShipperDocumentService struct {
	docs  shipperDocuments
	files shipperFiles
}

func NewShipperDocumentService(docs shipperDocuments, files shipperFiles) *ShipperDocumentService {
	return &ShipperDocumentService{docs: docs, files: files}
}

func (s *ShipperDocumentService) List(ctx context.Context, filter domain.ShipperDocumentFilter) ([]domain.Document, int, error) {
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if err := domain.ValidateShipperDocumentFilter(filter); err != nil {
		return nil, 0, err
	}
	visible, err := s.docs.ShipperShipmentVisible(ctx, filter.TenantID, filter.ShipmentID, filter.ShipperCompanyID)
	if err != nil {
		return nil, 0, err
	}
	if !visible {
		return nil, 0, apperrors.NotFound("shipment not found")
	}
	return s.docs.ListShipperDocuments(ctx, filter)
}

func (s *ShipperDocumentService) Get(ctx context.Context, tenantID, shipperCompanyID, documentID uuid.UUID) (*domain.Document, error) {
	if err := requireShipperScope(tenantID, shipperCompanyID, documentID, "document_id"); err != nil {
		return nil, err
	}
	return s.docs.GetShipperDocument(ctx, tenantID, shipperCompanyID, documentID)
}

func (s *ShipperDocumentService) ListAttachments(ctx context.Context, tenantID, shipperCompanyID, documentID uuid.UUID) ([]domain.Attachment, error) {
	if _, err := s.Get(ctx, tenantID, shipperCompanyID, documentID); err != nil {
		return nil, err
	}
	return s.files.ListFinalized(ctx, tenantID, documentID)
}

func (s *ShipperDocumentService) GetAttachment(ctx context.Context, tenantID, shipperCompanyID, documentID, attachmentID uuid.UUID) (*domain.Attachment, error) {
	if _, err := s.Get(ctx, tenantID, shipperCompanyID, documentID); err != nil {
		return nil, err
	}
	if attachmentID == uuid.Nil {
		return nil, apperrors.Validation("attachment_id is required", map[string]any{"field": "attachment_id"})
	}
	return s.files.GetFinalized(ctx, tenantID, documentID, attachmentID)
}

func (s *ShipperDocumentService) OpenContent(ctx context.Context, tenantID, shipperCompanyID, documentID, attachmentID uuid.UUID) (*domain.Attachment, io.ReadCloser, error) {
	if _, err := s.Get(ctx, tenantID, shipperCompanyID, documentID); err != nil {
		return nil, nil, err
	}
	if attachmentID == uuid.Nil {
		return nil, nil, apperrors.Validation("attachment_id is required", map[string]any{"field": "attachment_id"})
	}
	return s.files.OpenFinalized(ctx, tenantID, documentID, attachmentID)
}

func requireShipperScope(tenantID, shipperCompanyID, documentID uuid.UUID, documentField string) error {
	if tenantID == uuid.Nil {
		return apperrors.Unauthorized("trusted tenant context is required")
	}
	if shipperCompanyID == uuid.Nil {
		return apperrors.Validation("shipper_company_id is required", map[string]any{"field": "shipper_company_id"})
	}
	if documentID == uuid.Nil {
		return apperrors.Validation(documentField+" is required", map[string]any{"field": documentField})
	}
	return nil
}
