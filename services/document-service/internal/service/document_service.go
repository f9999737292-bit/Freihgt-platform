package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
	"github.com/freight-platform/document-service/internal/repository"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
)

type DocumentStore interface {
	CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error)
	ShipmentExists(ctx context.Context, shipmentID, tenantID uuid.UUID) (bool, error)
	CreateDocument(ctx context.Context, in domain.CreateDocumentInput) (*domain.Document, *domain.DocumentVersion, error)
	GetDetail(ctx context.Context, id uuid.UUID) (*repository.DocumentDetail, error)
	List(ctx context.Context, filter domain.ListDocumentsFilter) ([]domain.Document, int, error)
	GetByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.Document, error)
	HasVersions(ctx context.Context, documentID uuid.UUID) (bool, error)
	CreateVersion(ctx context.Context, documentID uuid.UUID, in domain.CreateDocumentVersionInput) (*domain.DocumentVersion, error)
	VersionBelongsToDocument(ctx context.Context, versionID, documentID uuid.UUID) (bool, error)
	AddFile(ctx context.Context, documentID uuid.UUID, in domain.CreateDocumentFileInput) (*domain.DocumentFile, error)
	UpdateDocumentStatus(ctx context.Context, id, tenantID uuid.UUID, status string, expectedVersion int) (*domain.Document, error)
}

type DocumentService struct {
	documents DocumentStore
}

func NewDocumentService(documents DocumentStore) *DocumentService {
	return &DocumentService{documents: documents}
}

func (s *DocumentService) Create(ctx context.Context, in domain.CreateDocumentInput) (*domain.Document, error) {
	in.LegalLanguage = domain.NormalizeLegalLanguage(in.LegalLanguage)
	if err := domain.ValidateCreateDocumentInput(in); err != nil {
		return nil, err
	}

	exists, err := s.documents.CompanyExists(ctx, in.OwnerCompanyID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, apperrors.NotFound("owner_company_id not found")
	}

	if in.RelatedEntityType != nil && *in.RelatedEntityType == domain.RelatedEntityTypeShipment {
		if in.RelatedEntityID == nil {
			return nil, apperrors.Validation("related_entity_id is required for SHIPMENT", map[string]any{"field": "related_entity_id"})
		}
		shipmentExists, err := s.documents.ShipmentExists(ctx, *in.RelatedEntityID, in.TenantID)
		if err != nil {
			return nil, err
		}
		if !shipmentExists {
			return nil, apperrors.NotFound("shipment not found")
		}
	}

	doc, _, err := s.documents.CreateDocument(ctx, in)
	return doc, err
}

func (s *DocumentService) GetByID(ctx context.Context, id uuid.UUID) (*repository.DocumentDetail, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	return s.documents.GetDetail(ctx, id)
}

func (s *DocumentService) List(ctx context.Context, filter domain.ListDocumentsFilter) ([]domain.Document, int, error) {
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if err := domain.ValidateListDocumentsFilter(filter); err != nil {
		return nil, 0, err
	}
	return s.documents.List(ctx, filter)
}

func (s *DocumentService) CreateVersion(ctx context.Context, id uuid.UUID, in domain.CreateDocumentVersionInput) (*domain.DocumentVersion, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateCreateDocumentVersionInput(in); err != nil {
		return nil, err
	}

	doc, err := s.documents.GetByIDAndTenant(ctx, id, in.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateCreateVersionStatus(doc.DocumentStatus); err != nil {
		return nil, err
	}
	return s.documents.CreateVersion(ctx, id, in)
}

func (s *DocumentService) AddFile(ctx context.Context, id uuid.UUID, in domain.CreateDocumentFileInput) (*domain.DocumentFile, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	in.StorageProvider = domain.NormalizeStorageProvider(in.StorageProvider)
	if err := domain.ValidateCreateDocumentFileInput(in); err != nil {
		return nil, err
	}

	if _, err := s.documents.GetByIDAndTenant(ctx, id, in.TenantID); err != nil {
		return nil, err
	}
	belongs, err := s.documents.VersionBelongsToDocument(ctx, in.DocumentVersionID, id)
	if err != nil {
		return nil, err
	}
	if !belongs {
		return nil, apperrors.NotFound("document_version_id not found for document")
	}
	return s.documents.AddFile(ctx, id, in)
}

func (s *DocumentService) ReadyForSigning(ctx context.Context, id uuid.UUID, in domain.ReadyForSigningInput) (*domain.Document, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if in.TenantID == uuid.Nil {
		return nil, apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}

	doc, err := s.documents.GetByIDAndTenant(ctx, id, in.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateReadyForSigningStatus(doc.DocumentStatus); err != nil {
		return nil, err
	}
	hasVersions, err := s.documents.HasVersions(ctx, id)
	if err != nil {
		return nil, err
	}
	if !hasVersions {
		return nil, apperrors.Validation("document must have at least one version", map[string]any{"field": "versions"})
	}
	return s.documents.UpdateDocumentStatus(ctx, id, in.TenantID, domain.DocumentStatusReadyForSigning, doc.Version)
}

func (s *DocumentService) Cancel(ctx context.Context, id uuid.UUID, in domain.CancelDocumentInput) (*domain.Document, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if in.TenantID == uuid.Nil {
		return nil, apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}

	doc, err := s.documents.GetByIDAndTenant(ctx, id, in.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateCancelDocumentStatus(doc.DocumentStatus); err != nil {
		return nil, err
	}
	return s.documents.UpdateDocumentStatus(ctx, id, in.TenantID, domain.DocumentStatusCancelled, doc.Version)
}

func (s *DocumentService) Archive(ctx context.Context, id uuid.UUID, in domain.ArchiveDocumentInput) (*domain.Document, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if in.TenantID == uuid.Nil {
		return nil, apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}

	doc, err := s.documents.GetByIDAndTenant(ctx, id, in.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateArchiveDocumentStatus(doc.DocumentStatus); err != nil {
		return nil, err
	}
	return s.documents.UpdateDocumentStatus(ctx, id, in.TenantID, domain.DocumentStatusArchived, doc.Version)
}
