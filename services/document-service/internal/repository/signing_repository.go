package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/document-service/internal/domain"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
)

type SigningRepository struct {
	pool *pgxpool.Pool
}

func NewSigningRepository(pool *pgxpool.Pool) *SigningRepository {
	return &SigningRepository{pool: pool}
}

func (r *SigningRepository) CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM core.companies
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		)
	`
	var exists bool
	if err := r.pool.QueryRow(ctx, query, companyID, tenantID).Scan(&exists); err != nil {
		return false, mapDBError(err)
	}
	return exists, nil
}

func (r *SigningRepository) UserExists(ctx context.Context, userID, tenantID uuid.UUID) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM core.users
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		)
	`
	var exists bool
	if err := r.pool.QueryRow(ctx, query, userID, tenantID).Scan(&exists); err != nil {
		return false, mapDBError(err)
	}
	return exists, nil
}

func (r *SigningRepository) CreateSession(ctx context.Context, documentID uuid.UUID, in domain.CreateSigningSessionInput) (*domain.SigningSession, error) {
	var result *domain.SigningSession
	err := measureDB("signing_repository", "create_signing_session", func() error {
		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return mapDBError(err)
		}
		defer tx.Rollback(ctx)

		const sessionQuery = `
		INSERT INTO documents.signing_sessions (
			tenant_id, document_id, status, required_signers_count, expires_at
		) VALUES ($1, $2, $3, $4, $5)
		RETURNING id, tenant_id, document_id, status, required_signers_count,
			completed_signers_count, created_at, expires_at
	`
		session, err := scanSigningSession(tx.QueryRow(ctx, sessionQuery,
			in.TenantID,
			documentID,
			domain.SigningSessionStatusCreated,
			in.RequiredSignersCount,
			optionalTime(in.ExpiresAt),
		))
		if err != nil {
			return err
		}

		const docQuery = `
		UPDATE documents.documents
		SET document_status = $1, version = version + 1, updated_at = now()
		WHERE id = $2 AND tenant_id = $3 AND deleted_at IS NULL
	`
		if _, err := tx.Exec(ctx, docQuery, domain.DocumentStatusSigningInProgress, documentID, in.TenantID); err != nil {
			return mapDBError(err)
		}

		if err := tx.Commit(ctx); err != nil {
			return mapDBError(err)
		}
		result = session
		return nil
	})
	return result, err
}

func (r *SigningRepository) GetSessionByID(ctx context.Context, id uuid.UUID) (*domain.SigningSession, error) {
	var result *domain.SigningSession
	err := measureDB("signing_repository", "get_signing_session", func() error {
		const query = `
		SELECT id, tenant_id, document_id, status, required_signers_count,
			completed_signers_count, created_at, expires_at
		FROM documents.signing_sessions
		WHERE id = $1
	`
		session, err := scanSigningSession(r.pool.QueryRow(ctx, query, id))
		if err != nil {
			return err
		}
		result = session
		return nil
	})
	return result, err
}

func (r *SigningRepository) GetSessionByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.SigningSession, error) {
	var result *domain.SigningSession
	err := measureDB("signing_repository", "get_signing_session", func() error {
		const query = `
		SELECT id, tenant_id, document_id, status, required_signers_count,
			completed_signers_count, created_at, expires_at
		FROM documents.signing_sessions
		WHERE id = $1 AND tenant_id = $2
	`
		session, err := scanSigningSession(r.pool.QueryRow(ctx, query, id, tenantID))
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.NotFound("signing session not found")
		}
		if err != nil {
			return err
		}
		result = session
		return nil
	})
	return result, err
}

func (r *SigningRepository) AddSignature(
	ctx context.Context,
	session *domain.SigningSession,
	in domain.AddSignatureInput,
) (*domain.Signature, *domain.SigningSession, *domain.Document, error) {
	var signature *domain.Signature
	var updatedSession *domain.SigningSession
	var doc *domain.Document
	err := measureDB("signing_repository", "add_signature", func() error {
		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return mapDBError(err)
		}
		defer tx.Rollback(ctx)

		now := time.Now().UTC()
		const signatureQuery = `
		INSERT INTO documents.signatures (
			tenant_id, signing_session_id, document_id,
			signer_user_id, signer_company_id, signature_type,
			signature_payload_path, certificate_fingerprint,
			signed_at, verification_status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, tenant_id, signing_session_id, document_id,
			signer_user_id, signer_company_id, signature_type,
			signature_payload_path, certificate_fingerprint,
			signed_at, verification_status, created_at
	`
		createdSignature, err := scanSignature(tx.QueryRow(ctx, signatureQuery,
			in.TenantID,
			session.ID,
			session.DocumentID,
			in.SignerUserID,
			in.SignerCompanyID,
			in.SignatureType,
			optionalString(in.SignaturePayloadPath),
			optionalString(in.CertificateFingerprint),
			now,
			domain.SignatureVerificationValid,
		))
		if err != nil {
			return err
		}

		completed := session.CompletedSignersCount + 1
		sessionStatus, documentStatus := domain.ResolveSigningAfterSignature(session.RequiredSignersCount, completed)

		const sessionUpdateQuery = `
		UPDATE documents.signing_sessions
		SET status = $1, completed_signers_count = $2
		WHERE id = $3
		RETURNING id, tenant_id, document_id, status, required_signers_count,
			completed_signers_count, created_at, expires_at
	`
		updated, err := scanSigningSession(tx.QueryRow(ctx, sessionUpdateQuery, sessionStatus, completed, session.ID))
		if err != nil {
			return err
		}

		const docQuery = `
		UPDATE documents.documents
		SET document_status = $1, version = version + 1, updated_at = now()
		WHERE id = $2 AND tenant_id = $3 AND deleted_at IS NULL
		RETURNING id, tenant_id, document_number, document_type, document_status,
			owner_company_id, related_entity_type, related_entity_id, legal_language,
			created_at, updated_at, version
	`
		updatedDoc, err := scanDocument(tx.QueryRow(ctx, docQuery, documentStatus, session.DocumentID, in.TenantID))
		if err != nil {
			return err
		}

		if err := tx.Commit(ctx); err != nil {
			return mapDBError(err)
		}
		signature = createdSignature
		updatedSession = updated
		doc = updatedDoc
		return nil
	})
	return signature, updatedSession, doc, err
}

func scanSigningSession(row rowScanner) (*domain.SigningSession, error) {
	var s domain.SigningSession
	err := row.Scan(
		&s.ID, &s.TenantID, &s.DocumentID, &s.Status,
		&s.RequiredSignersCount, &s.CompletedSignersCount, &s.CreatedAt, &s.ExpiresAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NotFound("signing session not found")
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	return &s, nil
}

func scanSignature(row rowScanner) (*domain.Signature, error) {
	var s domain.Signature
	err := row.Scan(
		&s.ID, &s.TenantID, &s.SigningSessionID, &s.DocumentID,
		&s.SignerUserID, &s.SignerCompanyID, &s.SignatureType,
		&s.SignaturePayloadPath, &s.CertificateFingerprint,
		&s.SignedAt, &s.VerificationStatus, &s.CreatedAt,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &s, nil
}
