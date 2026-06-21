package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
)

type SigningStore interface {
	CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error)
	UserExists(ctx context.Context, userID, tenantID uuid.UUID) (bool, error)
	CreateSession(ctx context.Context, documentID uuid.UUID, in domain.CreateSigningSessionInput) (*domain.SigningSession, error)
	GetSessionByID(ctx context.Context, id uuid.UUID) (*domain.SigningSession, error)
	GetSessionByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.SigningSession, error)
	AddSignature(ctx context.Context, session *domain.SigningSession, in domain.AddSignatureInput) (*domain.Signature, *domain.SigningSession, *domain.Document, error)
}

type DocumentLookup interface {
	GetByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.Document, error)
}

type SigningService struct {
	signing   SigningStore
	documents DocumentLookup
}

func NewSigningService(signing SigningStore, documents DocumentLookup) *SigningService {
	return &SigningService{signing: signing, documents: documents}
}

func (s *SigningService) CreateSession(ctx context.Context, documentID uuid.UUID, in domain.CreateSigningSessionInput) (*domain.SigningSession, error) {
	if documentID == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateCreateSigningSessionInput(in); err != nil {
		return nil, err
	}

	doc, err := s.documents.GetByIDAndTenant(ctx, documentID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateCreateSigningSessionStatus(doc.DocumentStatus); err != nil {
		return nil, err
	}
	return s.signing.CreateSession(ctx, documentID, in)
}

func (s *SigningService) GetSession(ctx context.Context, id uuid.UUID) (*domain.SigningSession, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	return s.signing.GetSessionByID(ctx, id)
}

func (s *SigningService) AddSignature(ctx context.Context, sessionID uuid.UUID, in domain.AddSignatureInput) (*domain.Signature, *domain.SigningSession, *domain.Document, error) {
	if sessionID == uuid.Nil {
		return nil, nil, nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateAddSignatureInput(in); err != nil {
		return nil, nil, nil, err
	}

	session, err := s.signing.GetSessionByIDAndTenant(ctx, sessionID, in.TenantID)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := domain.ValidateSigningSessionForSignature(session.Status); err != nil {
		return nil, nil, nil, err
	}

	doc, err := s.documents.GetByIDAndTenant(ctx, session.DocumentID, in.TenantID)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := domain.ValidateSigningDocumentStatus(doc.DocumentStatus); err != nil {
		return nil, nil, nil, err
	}

	userExists, err := s.signing.UserExists(ctx, in.SignerUserID, in.TenantID)
	if err != nil {
		return nil, nil, nil, err
	}
	if !userExists {
		return nil, nil, nil, apperrors.NotFound("signer_user_id not found")
	}

	companyExists, err := s.signing.CompanyExists(ctx, in.SignerCompanyID, in.TenantID)
	if err != nil {
		return nil, nil, nil, err
	}
	if !companyExists {
		return nil, nil, nil, apperrors.NotFound("signer_company_id not found")
	}

	return s.signing.AddSignature(ctx, session, in)
}
