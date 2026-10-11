package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
)

func (r *AttachmentRepository) ListFinalized(ctx context.Context, tenantID, documentID uuid.UUID) ([]domain.Attachment, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, document_id, tenant_id, document_version_id, file_name, media_type,
			size_bytes, sha256, storage_key, status, idempotency_key, created_at, finalized_at
		FROM documents.document_attachments
		WHERE tenant_id = $1 AND document_id = $2 AND status = 'FINALIZED'
		ORDER BY created_at ASC`, tenantID, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Attachment, 0)
	for rows.Next() {
		item, err := scanDocumentAttachment(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (r *AttachmentRepository) GetFinalized(ctx context.Context, tenantID, documentID, attachmentID uuid.UUID) (*domain.Attachment, error) {
	item, err := r.Get(ctx, tenantID, documentID, attachmentID)
	if err != nil {
		return nil, err
	}
	if item.Status != domain.AttachmentStatusFinalized {
		return nil, apperrors.NotFound("attachment not found")
	}
	return item, nil
}
