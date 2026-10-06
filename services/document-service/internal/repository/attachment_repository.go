package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/document-service/internal/domain"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
)

type AttachmentRepository struct {
	pool *pgxpool.Pool
}

func NewAttachmentRepository(pool *pgxpool.Pool) *AttachmentRepository {
	return &AttachmentRepository{pool: pool}
}

func (r *AttachmentRepository) FindByIdempotency(ctx context.Context, tenantID uuid.UUID, key string) (*domain.Attachment, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, document_id, tenant_id, document_version_id, file_name, media_type,
			size_bytes, sha256, storage_key, status, idempotency_key, created_at, finalized_at
		FROM documents.document_attachments
		WHERE tenant_id = $1 AND idempotency_key = $2`, tenantID, key)
	item, err := scanDocumentAttachment(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return item, err
}

func (r *AttachmentRepository) Insert(ctx context.Context, item domain.Attachment) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO documents.document_attachments (
			id, document_id, tenant_id, document_version_id, file_name, media_type,
			size_bytes, sha256, storage_key, status, idempotency_key
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'DRAFT',$10)`,
		item.ID, item.DocumentID, item.TenantID, item.DocumentVersionID, item.FileName, item.MediaType,
		item.SizeBytes, item.SHA256, item.StorageKey, item.IdempotencyKey)
	return mapUnique(err)
}

func (r *AttachmentRepository) Get(ctx context.Context, tenantID, documentID, attachmentID uuid.UUID) (*domain.Attachment, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, document_id, tenant_id, document_version_id, file_name, media_type,
			size_bytes, sha256, storage_key, status, idempotency_key, created_at, finalized_at
		FROM documents.document_attachments
		WHERE id = $1 AND document_id = $2 AND tenant_id = $3`, attachmentID, documentID, tenantID)
	item, err := scanDocumentAttachment(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NotFound("attachment not found")
	}
	return item, err
}

func (r *AttachmentRepository) Finalize(ctx context.Context, tenantID, documentID, attachmentID uuid.UUID, at time.Time) (*domain.Attachment, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE documents.document_attachments
		SET status = 'FINALIZED', finalized_at = $4
		WHERE id = $1 AND document_id = $2 AND tenant_id = $3 AND status = 'DRAFT'
		RETURNING id, document_id, tenant_id, document_version_id, file_name, media_type,
			size_bytes, sha256, storage_key, status, idempotency_key, created_at, finalized_at`,
		attachmentID, documentID, tenantID, at)
	item, err := scanDocumentAttachment(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return r.Get(ctx, tenantID, documentID, attachmentID)
	}
	return item, err
}

func (r *AttachmentRepository) FindSignatureByIdempotency(ctx context.Context, tenantID, attachmentID uuid.UUID, key string) (*domain.AttachmentSignature, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, attachment_id, tenant_id, signature_format, signature_reference,
			certificate_subject, certificate_issuer, certificate_serial, certificate_thumbprint,
			signing_time, verification_status, verification_time, verification_error,
			idempotency_key, created_at
		FROM documents.attachment_signatures
		WHERE tenant_id = $1 AND attachment_id = $2 AND idempotency_key = $3`, tenantID, attachmentID, key)
	item, err := scanAttachmentSignature(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return item, err
}

func (r *AttachmentRepository) InsertSignature(ctx context.Context, item domain.AttachmentSignature) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `
		INSERT INTO documents.attachment_signatures (
			id, attachment_id, tenant_id, signature_format, signature_reference,
			certificate_subject, certificate_issuer, certificate_serial, certificate_thumbprint,
			signing_time, verification_status, idempotency_key
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'UNVERIFIED',$11)`,
		item.ID, item.AttachmentID, item.TenantID, item.SignatureFormat, item.SignatureReference,
		item.CertificateSubject, item.CertificateIssuer, item.CertificateSerial, item.CertificateThumbprint,
		item.SigningTime, item.IdempotencyKey)
	if err != nil {
		return mapUnique(err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO documents.signature_verification_history (
			signature_id, tenant_id, verification_status
		) VALUES ($1,$2,'UNVERIFIED')`, item.ID, item.TenantID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *AttachmentRepository) DeleteDraft(ctx context.Context, tenantID, documentID, attachmentID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM documents.document_attachments
		WHERE id = $1 AND document_id = $2 AND tenant_id = $3 AND status = 'DRAFT'`,
		attachmentID, documentID, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.Conflict("finalized object delete is not allowed", map[string]any{"error_code": "OBJECT_UPLOAD_FAILED"})
	}
	return nil
}

func (r *AttachmentRepository) GetSignature(ctx context.Context, tenantID, attachmentID, signatureID uuid.UUID) (*domain.AttachmentSignature, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, attachment_id, tenant_id, signature_format, signature_reference,
			certificate_subject, certificate_issuer, certificate_serial, certificate_thumbprint,
			signing_time, verification_status, verification_time, verification_error,
			idempotency_key, created_at
		FROM documents.attachment_signatures
		WHERE id = $1 AND attachment_id = $2 AND tenant_id = $3`, signatureID, attachmentID, tenantID)
	item, err := scanAttachmentSignature(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NotFound("signature not found")
	}
	return item, err
}

func (r *AttachmentRepository) SignatureBlobDigest(ctx context.Context, tenantID, signatureID uuid.UUID) (string, error) {
	var sum string
	err := r.pool.QueryRow(ctx, `
		SELECT sha256 FROM documents.attachment_signature_blobs
		WHERE signature_id = $1 AND tenant_id = $2`, signatureID, tenantID).Scan(&sum)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apperrors.NotFound("signature blob not found")
	}
	return sum, err
}

func (r *AttachmentRepository) LatestEvidence(ctx context.Context, tenantID, signatureID uuid.UUID, policyID, policyVersion string) (string, string, error) {
	var status, reason string
	err := r.pool.QueryRow(ctx, `
		SELECT verification_status, reason_code
		FROM documents.signature_verification_evidence
		WHERE signature_id = $1 AND tenant_id = $2
		  AND policy_id = $3 AND policy_version = $4
		ORDER BY created_at DESC, id DESC
		LIMIT 1`, signatureID, tenantID, policyID, policyVersion).Scan(&status, &reason)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil
	}
	return status, reason, err
}

func (r *AttachmentRepository) InsertDetachedSignature(ctx context.Context, sig domain.AttachmentSignature, objectKey string, size int64, sum, mediaType, profile string, attemptedAt time.Time, verifierVersion, policyID, policyVersion string, documentID uuid.UUID, actor *uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		INSERT INTO documents.attachment_signatures (
			id, attachment_id, tenant_id, signature_format, signature_reference,
			certificate_subject, certificate_issuer, certificate_serial, certificate_thumbprint,
			signing_time, verification_status, idempotency_key
		) VALUES ($1,$2,$3,$4,$5,NULL,NULL,NULL,NULL,NULL,'UNVERIFIED',$6)`,
		sig.ID, sig.AttachmentID, sig.TenantID, sig.SignatureFormat, sig.SignatureReference, sig.IdempotencyKey); err != nil {
		return mapUnique(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO documents.signature_verification_history (
			signature_id, tenant_id, verification_status
		) VALUES ($1,$2,'UNVERIFIED')`, sig.ID, sig.TenantID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO documents.attachment_signature_blobs (
			signature_id, tenant_id, object_key, size_bytes, sha256, media_type, profile
		) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		sig.ID, sig.TenantID, objectKey, size, sum, mediaType, profile); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO documents.signature_verification_evidence (
			signature_id, tenant_id, verification_status, reason_code, attempted_at,
			verifier_version, policy_id, policy_version
		) VALUES ($1,$2,'PENDING','VERIFIER_UNAVAILABLE',$3,$4,$5,$6)`,
		sig.ID, sig.TenantID, attemptedAt, verifierVersion, policyID, policyVersion); err != nil {
		return err
	}
	var idem *string
	if sig.IdempotencyKey != nil && *sig.IdempotencyKey != "" {
		idem = sig.IdempotencyKey
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO documents.attachment_audit_events (
			tenant_id, document_id, attachment_id, signature_id, event_type, actor_user_id, idempotency_key
		) VALUES ($1,$2,$3,$4,'edo.signature.attached',$5,$6)`,
		sig.TenantID, documentID, sig.AttachmentID, sig.ID, actor, idem); err != nil {
		return mapUnique(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return ClassifyCommitError(err)
	}
	return nil
}

func (r *AttachmentRepository) InsertAudit(ctx context.Context, tenantID, documentID uuid.UUID, attachmentID, signatureID, actor *uuid.UUID, eventType, idempotencyKey string) error {
	var key *string
	if idempotencyKey != "" {
		key = &idempotencyKey
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO documents.attachment_audit_events (
			tenant_id, document_id, attachment_id, signature_id, event_type, actor_user_id, idempotency_key
		) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		tenantID, documentID, attachmentID, signatureID, eventType, actor, key)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil
		}
	}
	return err
}

func (r *AttachmentRepository) CountAudit(ctx context.Context, tenantID, attachmentID uuid.UUID, eventType string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM documents.attachment_audit_events
		WHERE tenant_id = $1 AND attachment_id = $2 AND event_type = $3`,
		tenantID, attachmentID, eventType).Scan(&n)
	return n, err
}

func scanDocumentAttachment(row pgx.Row) (*domain.Attachment, error) {
	var item domain.Attachment
	err := row.Scan(
		&item.ID, &item.DocumentID, &item.TenantID, &item.DocumentVersionID, &item.FileName, &item.MediaType,
		&item.SizeBytes, &item.SHA256, &item.StorageKey, &item.Status, &item.IdempotencyKey, &item.CreatedAt, &item.FinalizedAt,
	)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func scanAttachmentSignature(row pgx.Row) (*domain.AttachmentSignature, error) {
	var item domain.AttachmentSignature
	err := row.Scan(
		&item.ID, &item.AttachmentID, &item.TenantID, &item.SignatureFormat, &item.SignatureReference,
		&item.CertificateSubject, &item.CertificateIssuer, &item.CertificateSerial, &item.CertificateThumbprint,
		&item.SigningTime, &item.VerificationStatus, &item.VerificationTime, &item.VerificationError,
		&item.IdempotencyKey, &item.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func mapUnique(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return apperrors.Conflict("attachment idempotency key already exists", map[string]any{"field": "idempotency_key"})
	}
	return err
}
