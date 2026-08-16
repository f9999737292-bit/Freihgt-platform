package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/document-service/internal/domain"
	"github.com/freight-platform/document-service/internal/platform/storage"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
)

type PODUploadService struct {
	pool    *pgxpool.Pool
	docs    *DocumentService
	storage storage.ObjectStore
	maxBytes int64
}

func NewPODUploadService(pool *pgxpool.Pool, docs *DocumentService, store storage.ObjectStore, maxBytes int64) *PODUploadService {
	if maxBytes <= 0 {
		maxBytes = 10 << 20
	}
	return &PODUploadService{pool: pool, docs: docs, storage: store, maxBytes: maxBytes}
}

type CreatePODUploadInput struct {
	TenantID       uuid.UUID
	ShipmentID     uuid.UUID
	DriverID       uuid.UUID
	OwnerCompanyID uuid.UUID
	MimeType       string
	FileName       string
	IdempotencyKey string
}

type PODUploadIntent struct {
	UploadID    uuid.UUID `json:"uploadId"`
	DocumentID  uuid.UUID `json:"documentId"`
	UploadToken string    `json:"uploadToken"`
	ObjectKey   string    `json:"-"`
	ExpiresAt   time.Time `json:"expiresAt"`
	MimeType    string    `json:"mimeType"`
	MaxBytes    int64     `json:"maxBytes"`
}

func (s *PODUploadService) CreateUploadIntent(ctx context.Context, in CreatePODUploadInput) (*PODUploadIntent, error) {
	in.MimeType = strings.TrimSpace(in.MimeType)
	if in.MimeType == "" {
		return nil, apperrors.Validation("mimeType is required", map[string]any{"field": "mimeType"})
	}
	if !allowedPODMime(in.MimeType) {
		return nil, apperrors.Validation("unsupported mime type", map[string]any{"field": "mimeType"})
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	if key != "" {
		if existing, err := s.findIntentByIdempotency(ctx, in.TenantID, in.DriverID, key); err != nil {
			return nil, err
		} else if existing != nil {
			return existing, nil
		}
	}

	shipmentType := domain.RelatedEntityTypeShipment
	uploadID := uuid.New()
	doc, err := s.docs.Create(ctx, domain.CreateDocumentInput{
		TenantID: in.TenantID, DocumentNumber: "POD-" + strings.ReplaceAll(uploadID.String(), "-", "")[:12],
		DocumentType: "POD", OwnerCompanyID: in.OwnerCompanyID,
		RelatedEntityType: &shipmentType, RelatedEntityID: &in.ShipmentID,
		LegalLanguage: "ru-RU",
	})
	if err != nil {
		return nil, err
	}

	objectKey := fmt.Sprintf("tenants/%s/shipments/%s/pod/%s/%s",
		in.TenantID, in.ShipmentID, doc.ID, sanitizeFileName(in.FileName, uploadID))
	token, tokenHash, err := newUploadToken()
	if err != nil {
		return nil, err
	}
	expires := time.Now().UTC().Add(15 * time.Minute)
	_, err = s.pool.Exec(ctx, `
		INSERT INTO documents.document_upload_intent
		(id, tenant_id, document_id, shipment_id, driver_id, object_key, upload_token_hash,
		 mime_type, max_bytes, file_name, status, idempotency_key, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'pending',$11,$12)`,
		uploadID, in.TenantID, doc.ID, in.ShipmentID, in.DriverID, objectKey, tokenHash,
		in.MimeType, s.maxBytes, nullIfEmpty(in.FileName), nullIfEmpty(key), expires)
	if err != nil {
		return nil, err
	}
	return &PODUploadIntent{
		UploadID: uploadID, DocumentID: doc.ID, UploadToken: token,
		ObjectKey: objectKey, ExpiresAt: expires, MimeType: in.MimeType, MaxBytes: s.maxBytes,
	}, nil
}

func (s *PODUploadService) UploadContent(ctx context.Context, tenantID, uploadID uuid.UUID, token string, body io.Reader) error {
	intent, err := s.loadIntent(ctx, tenantID, uploadID)
	if err != nil {
		return err
	}
	if err := verifyToken(token, intent.tokenHash); err != nil {
		return apperrors.Validation("invalid upload token", nil)
	}
	if intent.status != "pending" && intent.status != "uploaded" {
		return apperrors.Conflict("upload intent is not active", map[string]any{"status": intent.status})
	}
	if time.Now().UTC().After(intent.expiresAt) {
		return apperrors.Validation("upload intent expired", nil)
	}
	size, err := s.storage.Put(ctx, intent.objectKey, body, intent.maxBytes)
	if err != nil {
		return apperrors.Validation("upload failed", map[string]any{"detail": err.Error()})
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE documents.document_upload_intent SET status='uploaded', byte_size=$3
		WHERE tenant_id=$1 AND id=$2`, tenantID, uploadID, size)
	return err
}

type CompletePODUploadInput struct {
	TenantID       uuid.UUID
	UploadID       uuid.UUID
	DriverID       uuid.UUID
	ChecksumSHA256 string
}

type CompletePODUploadResult struct {
	DocumentID uuid.UUID `json:"documentId"`
	UploadID   uuid.UUID `json:"uploadId"`
	FileID     uuid.UUID `json:"fileId"`
	Status     string    `json:"status"`
}

func (s *PODUploadService) CompleteUpload(ctx context.Context, in CompletePODUploadInput) (*CompletePODUploadResult, error) {
	intent, err := s.loadIntent(ctx, in.TenantID, in.UploadID)
	if err != nil {
		return nil, err
	}
	if intent.driverID != in.DriverID {
		return nil, apperrors.Validation("upload intent driver mismatch", nil)
	}
	if intent.status == "completed" {
		fileID, err := s.lookupCompletedFile(ctx, intent.documentID)
		if err != nil {
			return nil, err
		}
		return &CompletePODUploadResult{DocumentID: intent.documentID, UploadID: intent.id, FileID: fileID, Status: "completed"}, nil
	}
	if intent.status != "uploaded" {
		return nil, apperrors.Validation("upload content required before finalize", map[string]any{"status": intent.status})
	}
	ok, err := s.storage.Exists(ctx, intent.objectKey)
	if err != nil || !ok {
		return nil, apperrors.Validation("stored object missing", nil)
	}
	checksum := strings.ToLower(strings.TrimSpace(in.ChecksumSHA256))
	detail, err := s.docs.GetByID(ctx, intent.documentID)
	if err != nil || detail.LatestVersion == nil {
		return nil, apperrors.NotFound("document version not found")
	}
	fileType := domain.FileTypeImage
	if strings.Contains(strings.ToLower(intent.mimeType), "pdf") {
		fileType = domain.FileTypePDF
	}
	bucket := "local"
	fileName := coalesce(intent.fileName, path.Base(intent.objectKey))
	size := intent.byteSize
	mime := intent.mimeType
	var checksumPtr *string
	if checksum != "" {
		checksumPtr = &checksum
	}
	file, err := s.docs.AddFile(ctx, intent.documentID, domain.CreateDocumentFileInput{
		TenantID: in.TenantID, DocumentVersionID: detail.LatestVersion.ID,
		FileType: fileType, StorageProvider: "local", BucketName: &bucket,
		ObjectKey: intent.objectKey, FileName: &fileName, MimeType: &mime,
		FileSizeBytes: &size, ChecksumSHA256: checksumPtr,
	})
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	_, err = s.pool.Exec(ctx, `
		UPDATE documents.document_upload_intent
		SET status='completed', checksum_sha256=$3, completed_at=$4
		WHERE tenant_id=$1 AND id=$2`, in.TenantID, in.UploadID, nullIfEmpty(checksum), now)
	if err != nil {
		return nil, err
	}
	return &CompletePODUploadResult{DocumentID: intent.documentID, UploadID: intent.id, FileID: file.ID, Status: "completed"}, nil
}

type loadedIntent struct {
	id, documentID, driverID uuid.UUID
	objectKey, tokenHash, mimeType, fileName, status string
	maxBytes, byteSize int64
	expiresAt time.Time
}

func (s *PODUploadService) loadIntent(ctx context.Context, tenantID, uploadID uuid.UUID) (*loadedIntent, error) {
	var intent loadedIntent
	err := s.pool.QueryRow(ctx, `
		SELECT id, document_id, driver_id, object_key, upload_token_hash, mime_type, COALESCE(file_name,''),
		       status, max_bytes, COALESCE(byte_size,0), expires_at
		FROM documents.document_upload_intent WHERE tenant_id=$1 AND id=$2`,
		tenantID, uploadID).Scan(
		&intent.id, &intent.documentID, &intent.driverID, &intent.objectKey, &intent.tokenHash,
		&intent.mimeType, &intent.fileName, &intent.status, &intent.maxBytes, &intent.byteSize, &intent.expiresAt,
	)
	if err != nil {
		return nil, apperrors.NotFound("upload intent not found")
	}
	return &intent, nil
}

func (s *PODUploadService) findIntentByIdempotency(ctx context.Context, tenantID, driverID uuid.UUID, key string) (*PODUploadIntent, error) {
	var uploadID, documentID uuid.UUID
	var objectKey, mimeType string
	var maxBytes int64
	var expires time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT id, document_id, object_key, mime_type, max_bytes, expires_at
		FROM documents.document_upload_intent
		WHERE tenant_id=$1 AND driver_id=$2 AND idempotency_key=$3`,
		tenantID, driverID, key).Scan(&uploadID, &documentID, &objectKey, &mimeType, &maxBytes, &expires)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &PODUploadIntent{
		UploadID: uploadID, DocumentID: documentID, UploadToken: "",
		ObjectKey: objectKey, ExpiresAt: expires, MimeType: mimeType, MaxBytes: maxBytes,
	}, nil
}

func (s *PODUploadService) lookupCompletedFile(ctx context.Context, documentID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT id FROM documents.document_files WHERE document_id=$1 ORDER BY created_at DESC LIMIT 1`, documentID).Scan(&id)
	return id, err
}

func newUploadToken() (plain, hash string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	plain = hex.EncodeToString(buf)
	sum := sha256.Sum256([]byte(plain))
	hash = hex.EncodeToString(sum[:])
	return plain, hash, nil
}

func verifyToken(plain, hash string) error {
	sum := sha256.Sum256([]byte(strings.TrimSpace(plain)))
	if hex.EncodeToString(sum[:]) != strings.TrimSpace(hash) {
		return fmt.Errorf("token mismatch")
	}
	return nil
}

func allowedPODMime(m string) bool {
	switch strings.ToLower(m) {
	case "image/jpeg", "image/png", "image/webp", "application/pdf":
		return true
	default:
		return false
	}
}

func sanitizeFileName(name string, fallback uuid.UUID) string {
	name = strings.TrimSpace(path.Base(name))
	if name == "" || name == "." {
		return fallback.String() + ".bin"
	}
	return name
}

func nullIfEmpty(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

func coalesce(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
