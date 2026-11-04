package service

import (
	"context"
	"time"

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
	MarkSessionExpired(ctx context.Context, id, tenantID uuid.UUID) (*domain.SigningSession, error)
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
	if err := domain.ValidateSigningSessionExpiry(in.ExpiresAt, time.Now().UTC()); err != nil {
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

func (s *SigningService) GetSession(ctx context.Context, id, tenantID uuid.UUID) (*domain.SigningSession, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if tenantID == uuid.Nil {
		return nil, apperrors.Unauthorized("trusted tenant context is required")
	}
	session, err := s.signing.GetSessionByIDAndTenant(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	return s.persistOverdue(ctx, session)
}

func (s *SigningService) persistOverdue(ctx context.Context, session *domain.SigningSession) (*domain.SigningSession, error) {
	if !domain.SigningSessionIsOverdue(session, time.Now().UTC()) {
		return session, nil
	}
	return s.signing.MarkSessionExpired(ctx, session.ID, session.TenantID)
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
	if domain.SigningSessionIsOverdue(session, time.Now().UTC()) {
		if _, markErr := s.signing.MarkSessionExpired(ctx, session.ID, session.TenantID); markErr != nil {
			return nil, nil, nil, markErr
		}
		return nil, nil, nil, apperrors.Validation("signing session is expired", map[string]any{
			"field":  "status",
			"status": domain.SigningSessionStatusExpired,
		})
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
