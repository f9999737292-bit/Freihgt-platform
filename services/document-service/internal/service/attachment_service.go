package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
	"github.com/freight-platform/document-service/internal/platform/storage"
	"github.com/freight-platform/document-service/internal/repository"
)

type attachmentPersistence interface {
	FindByIdempotency(ctx context.Context, tenantID uuid.UUID, key string) (*domain.Attachment, error)
	Insert(ctx context.Context, item domain.Attachment) error
	Get(ctx context.Context, tenantID, documentID, attachmentID uuid.UUID) (*domain.Attachment, error)
	Finalize(ctx context.Context, tenantID, documentID, attachmentID uuid.UUID, at time.Time) (*domain.Attachment, error)
	FindSignatureByIdempotency(ctx context.Context, tenantID, attachmentID uuid.UUID, key string) (*domain.AttachmentSignature, error)
	InsertSignature(ctx context.Context, item domain.AttachmentSignature) error
	DeleteDraft(ctx context.Context, tenantID, documentID, attachmentID uuid.UUID) error
	GetSignature(ctx context.Context, tenantID, attachmentID, signatureID uuid.UUID) (*domain.AttachmentSignature, error)
	InsertAudit(ctx context.Context, tenantID, documentID uuid.UUID, attachmentID, signatureID, actor *uuid.UUID, eventType, idempotencyKey string) error
}

type AttachmentService struct {
	docs    DocumentStore
	store   storage.ObjectStore
	repo    attachmentPersistence
	maxByte int64
}

func NewAttachmentService(docs DocumentStore, store storage.ObjectStore, repo *repository.AttachmentRepository) *AttachmentService {
	return &AttachmentService{docs: docs, store: store, repo: repo, maxByte: domain.MaxAttachmentBytes}
}

type CreateAttachmentInput struct {
	TenantID          uuid.UUID
	DocumentVersionID *uuid.UUID
	FileName          string
	MediaType         string
	ClientSHA256      string
	IdempotencyKey    string
	ActorUserID       *uuid.UUID
	Body              io.Reader
}

func (s *AttachmentService) Create(ctx context.Context, documentID uuid.UUID, in CreateAttachmentInput) (*domain.Attachment, error) {
	if documentID == uuid.Nil || in.TenantID == uuid.Nil {
		return nil, apperrors.Validation("document and tenant are required", nil)
	}
	if _, err := s.docs.GetByIDAndTenant(ctx, documentID, in.TenantID); err != nil {
		return nil, err
	}
	if in.DocumentVersionID != nil {
		ok, err := s.docs.VersionBelongsToDocument(ctx, *in.DocumentVersionID, documentID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, apperrors.NotFound("document version not found")
		}
	}
	fileName, err := domain.SafeFileName(in.FileName)
	if err != nil {
		return nil, err
	}
	media, err := domain.NormalizeMediaType(in.MediaType)
	if err != nil {
		return nil, err
	}
	clientHash := strings.TrimSpace(strings.ToLower(in.ClientSHA256))
	if clientHash != "" && !domain.ValidSHA256(clientHash) {
		return nil, apperrors.Validation("client checksum is not sha-256", map[string]any{"field": "sha256"})
	}
	var replay *domain.Attachment
	if in.IdempotencyKey != "" {
		existing, err := s.repo.FindByIdempotency(ctx, in.TenantID, in.IdempotencyKey)
		if err != nil {
			return nil, err
		}
		replay = existing
	}
	content, err := readBounded(in.Body, s.maxByte)
	if err != nil {
		return nil, err
	}
	if len(content) == 0 {
		return nil, apperrors.Validation("empty file is not allowed", map[string]any{"field": "body"})
	}
	if err := domain.MediaMatchesContent(media, content); err != nil {
		return nil, err
	}
	sum := domain.SHA256Hex(content)
	if clientHash != "" && clientHash != sum {
		return nil, apperrors.Validation("client checksum does not match file bytes", map[string]any{"field": "sha256"})
	}
	if replay != nil {
		if replay.DocumentID == documentID && replay.SHA256 == sum && replay.FileName == fileName {
			return replay, nil
		}
		return nil, apperrors.Conflict("idempotency key was used for different content", nil)
	}
	id := uuid.New()
	key := fmt.Sprintf("tenants/%s/documents/%s/attachments/%s", in.TenantID, documentID, id)
	if err := storage.ValidateObjectKey(key); err != nil {
		return nil, mapStoreErr(err)
	}
	putCtx := storage.WithUserMetadata(ctx, map[string]string{
		"attachment_id": id.String(),
		"document_id":   documentID.String(),
		"tenant_id":     in.TenantID.String(),
		"sha256":        sum,
		"media_type":    media,
	})
	if _, err := s.store.Put(putCtx, key, bytes.NewReader(content), s.maxByte); err != nil {
		return nil, mapStoreErr(err)
	}
	var idem *string
	if in.IdempotencyKey != "" {
		idem = &in.IdempotencyKey
	}
	item := domain.Attachment{
		ID: id, DocumentID: documentID, TenantID: in.TenantID, DocumentVersionID: in.DocumentVersionID,
		FileName: fileName, MediaType: media, SizeBytes: int64(len(content)), SHA256: sum,
		StorageKey: key, Status: domain.AttachmentStatusDraft, IdempotencyKey: idem,
	}
	if err := s.repo.Insert(ctx, item); err != nil {
		_ = s.store.Delete(ctx, key)
		return nil, err
	}
	if err := s.repo.InsertAudit(ctx, in.TenantID, documentID, &id, nil, in.ActorUserID, domain.EventAttachmentCreated, in.IdempotencyKey); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, in.TenantID, documentID, id)
}

func (s *AttachmentService) Get(ctx context.Context, tenantID, documentID, attachmentID uuid.UUID) (*domain.Attachment, error) {
	if _, err := s.docs.GetByIDAndTenant(ctx, documentID, tenantID); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, tenantID, documentID, attachmentID)
}

func (s *AttachmentService) Open(ctx context.Context, tenantID, documentID, attachmentID uuid.UUID) (*domain.Attachment, io.ReadCloser, error) {
	item, err := s.Get(ctx, tenantID, documentID, attachmentID)
	if err != nil {
		return nil, nil, err
	}
	content, err := s.readVerified(ctx, item)
	if err != nil {
		return nil, nil, err
	}
	return item, io.NopCloser(bytes.NewReader(content)), nil
}

func (s *AttachmentService) Finalize(ctx context.Context, tenantID, documentID, attachmentID uuid.UUID, actor *uuid.UUID) (*domain.Attachment, error) {
	current, err := s.Get(ctx, tenantID, documentID, attachmentID)
	if err != nil {
		return nil, err
	}
	if current.Status == domain.AttachmentStatusFinalized {
		return current, nil
	}
	if _, err := s.readVerified(ctx, current); err != nil {
		return nil, err
	}
	item, err := s.repo.Finalize(ctx, tenantID, documentID, attachmentID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if item.Status != domain.AttachmentStatusFinalized {
		return nil, apperrors.Conflict("attachment could not be finalized", nil)
	}
	key := ""
	if item.IdempotencyKey != nil {
		key = *item.IdempotencyKey
	}
	if err := s.repo.InsertAudit(ctx, tenantID, documentID, &attachmentID, nil, actor, domain.EventAttachmentFinalized, key); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *AttachmentService) DeleteDraft(ctx context.Context, tenantID, documentID, attachmentID uuid.UUID) error {
	item, err := s.Get(ctx, tenantID, documentID, attachmentID)
	if err != nil {
		return err
	}
	if item.Status != domain.AttachmentStatusDraft {
		return apperrors.Conflict("finalized object delete is not allowed", map[string]any{"error_code": "OBJECT_UPLOAD_FAILED"})
	}
	if err := s.store.Delete(ctx, item.StorageKey); err != nil {
		return mapStoreErr(err)
	}
	return s.repo.DeleteDraft(ctx, tenantID, documentID, attachmentID)
}

func (s *AttachmentService) readVerified(ctx context.Context, item *domain.Attachment) ([]byte, error) {
	body, err := s.store.Get(ctx, item.StorageKey)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	content, err := io.ReadAll(io.LimitReader(body, s.maxByte+1))
	_ = body.Close()
	if err != nil {
		return nil, mapStoreErr(err)
	}
	if int64(len(content)) > s.maxByte || domain.SHA256Hex(content) != item.SHA256 || int64(len(content)) != item.SizeBytes {
		storage.RecordIntegrityFailure()
		return nil, apperrors.Conflict("object integrity mismatch", map[string]any{"error_code": "OBJECT_INTEGRITY_MISMATCH"})
	}
	return content, nil
}

func mapStoreErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, storage.ErrObjectNotFound):
		return apperrors.NotFound("object not found")
	case errors.Is(err, storage.ErrObjectExists):
		return apperrors.Conflict("object already exists", map[string]any{"error_code": "OBJECT_UPLOAD_FAILED"})
	case errors.Is(err, storage.ErrIntegrityMismatch):
		return apperrors.Conflict("object integrity mismatch", map[string]any{"error_code": "OBJECT_INTEGRITY_MISMATCH"})
	case errors.Is(err, storage.ErrStorageUnavailable), errors.Is(err, storage.ErrInvalidStorageConfig):
		return apperrors.Internal("object storage unavailable", nil)
	case errors.Is(err, storage.ErrDownloadFailed):
		return apperrors.Internal("object download failed", nil)
	default:
		return apperrors.Internal("object upload failed", nil)
	}
}

type AttachSignatureInput struct {
	Format         string
	Reference      string
	Subject        *string
	Issuer         *string
	Serial         *string
	Thumbprint     *string
	SigningTime    *time.Time
	IdempotencyKey string
	ActorUserID    *uuid.UUID
}

func (s *AttachmentService) AttachSignature(ctx context.Context, tenantID, documentID, attachmentID uuid.UUID, in AttachSignatureInput) (*domain.AttachmentSignature, error) {
	item, err := s.Get(ctx, tenantID, documentID, attachmentID)
	if err != nil {
		return nil, err
	}
	if item.Status != domain.AttachmentStatusFinalized {
		return nil, apperrors.Conflict("signature metadata requires a finalized attachment", nil)
	}
	format := strings.TrimSpace(in.Format)
	switch format {
	case "CAdES", "PKCS7", "XMLDSIG", "OTHER":
	default:
		return nil, apperrors.Validation("signature format is not allowed", map[string]any{"field": "signature_format"})
	}
	reference := strings.TrimSpace(in.Reference)
	if reference == "" {
		return nil, apperrors.Validation("signature reference is required", map[string]any{"field": "signature_reference"})
	}
	if err := domain.RejectPrivateKeyMaterial(reference); err != nil {
		return nil, err
	}
	if in.Thumbprint != nil {
		thumb := strings.TrimSpace(strings.ToLower(*in.Thumbprint))
		if !domain.ValidSHA256(thumb) {
			return nil, apperrors.Validation("certificate thumbprint must be sha-256 hex", map[string]any{"field": "certificate_thumbprint"})
		}
		in.Thumbprint = &thumb
	}
	if in.IdempotencyKey != "" {
		existing, err := s.repo.FindSignatureByIdempotency(ctx, tenantID, attachmentID, in.IdempotencyKey)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing, nil
		}
	}
	var idem *string
	if in.IdempotencyKey != "" {
		idem = &in.IdempotencyKey
	}
	sig := domain.AttachmentSignature{
		ID: uuid.New(), AttachmentID: attachmentID, TenantID: tenantID,
		SignatureFormat: format, SignatureReference: reference,
		CertificateSubject: in.Subject, CertificateIssuer: in.Issuer, CertificateSerial: in.Serial,
		CertificateThumbprint: in.Thumbprint, SigningTime: in.SigningTime,
		VerificationStatus: domain.VerificationUnverified, IdempotencyKey: idem,
	}
	if err := s.repo.InsertSignature(ctx, sig); err != nil {
		return nil, err
	}
	if err := s.repo.InsertAudit(ctx, tenantID, documentID, &attachmentID, &sig.ID, in.ActorUserID, domain.EventSignatureAttached, in.IdempotencyKey); err != nil {
		return nil, err
	}
	return s.repo.GetSignature(ctx, tenantID, attachmentID, sig.ID)
}

func (s *AttachmentService) GetSignature(ctx context.Context, tenantID, documentID, attachmentID, signatureID uuid.UUID) (*domain.AttachmentSignature, error) {
	if _, err := s.Get(ctx, tenantID, documentID, attachmentID); err != nil {
		return nil, err
	}
	return s.repo.GetSignature(ctx, tenantID, attachmentID, signatureID)
}

func readBounded(r io.Reader, maxBytes int64) ([]byte, error) {
	if r == nil {
		return nil, apperrors.Validation("file body is required", map[string]any{"field": "body"})
	}
	limited := io.LimitReader(r, maxBytes+1)
	content, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > maxBytes {
		return nil, apperrors.Validation("file exceeds max size", map[string]any{"field": "body", "max_bytes": maxBytes})
	}
	return content, nil
}
