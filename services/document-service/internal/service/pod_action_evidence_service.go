package service

import (
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
)

// PODActionEvidenceStore is the document-side persistence for action-bound POD evidence.
type PODActionEvidenceStore interface {
	CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error)
	FindIntentByIdempotency(ctx context.Context, documentTenantID uuid.UUID, key string) (*domain.PODActionIntent, error)
	LoadIntent(ctx context.Context, uploadID uuid.UUID) (*domain.PODActionIntent, error)
	InsertIntent(ctx context.Context, in domain.PODActionIntent) (domain.PODActionIntent, error)
	RotateUploadToken(ctx context.Context, uploadID uuid.UUID, tokenHash string) error
	MarkIntentUploaded(ctx context.Context, uploadID uuid.UUID, size int64) error
	EvidenceByIntent(ctx context.Context, uploadID uuid.UUID) (*domain.PODActionEvidence, error)
	CompleteIntent(ctx context.Context, uploadID uuid.UUID, checksum, storageProvider string, bucketName *string) (*domain.PODActionEvidence, error)
	ListPresentEvidence(ctx context.Context, operatingTenantID, executionID, actionID uuid.UUID) ([]domain.PODActionEvidence, error)
}

// ActionObjectPlacement is the server-owned file metadata for the configured object store.
type ActionObjectPlacement struct {
	StorageProvider string
	BucketName      *string
}

func ActionObjectPlacementFromConfig(cfg storage.Config) ActionObjectPlacement {
	if strings.EqualFold(strings.TrimSpace(cfg.Provider), "s3") {
		bucket := cfg.Bucket
		return ActionObjectPlacement{StorageProvider: "s3", BucketName: &bucket}
	}
	return ActionObjectPlacement{StorageProvider: "local"}
}

type PODActionEvidenceService struct {
	store     PODActionEvidenceStore
	storage   storage.ObjectStore
	maxBytes  int64
	placement ActionObjectPlacement
}

func NewPODActionEvidenceService(store PODActionEvidenceStore, objects storage.ObjectStore, maxBytes int64, placement ActionObjectPlacement) *PODActionEvidenceService {
	if maxBytes <= 0 {
		maxBytes = 10 << 20
	}
	if strings.TrimSpace(placement.StorageProvider) == "" {
		placement.StorageProvider = "local"
	}
	return &PODActionEvidenceService{store: store, storage: objects, maxBytes: maxBytes, placement: placement}
}

type CreateActionPODIntentInput struct {
	ActionType        string
	DocumentTenantID  uuid.UUID
	OperatingTenantID uuid.UUID
	ExecutionID       uuid.UUID
	ExecutionActionID uuid.UUID
	ExecutionStopID   uuid.UUID
	ShipmentID        uuid.UUID
	ShipmentTenantID  uuid.UUID
	CargoID           uuid.UUID
	OwnerCompanyID    uuid.UUID
	AuthorDriverID    uuid.UUID
	AuthorCompanyID   uuid.UUID
	MimeType          string
	FileName          string
	IdempotencyKey    string
}

type ActionPODIntentResult struct {
	UploadID    uuid.UUID `json:"uploadId"`
	DocumentID  uuid.UUID `json:"documentId"`
	UploadToken string    `json:"uploadToken,omitempty"`
	ObjectKey   string    `json:"-"`
	ExpiresAt   time.Time `json:"expiresAt"`
	MimeType    string    `json:"mimeType"`
	MaxBytes    int64     `json:"maxBytes"`
	Replayed    bool      `json:"-"`
}

type CompleteActionPODResult struct {
	EvidenceID uuid.UUID `json:"evidenceId"`
	DocumentID uuid.UUID `json:"documentId"`
	UploadID   uuid.UUID `json:"uploadId"`
	FileID     uuid.UUID `json:"fileId"`
	Status     string    `json:"status"`
}

func (s *PODActionEvidenceService) CreateUploadIntent(ctx context.Context, in CreateActionPODIntentInput) (*ActionPODIntentResult, error) {
	in.ActionType = strings.TrimSpace(in.ActionType)
	in.MimeType = strings.TrimSpace(in.MimeType)
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	if err := validateCreateActionPOD(in); err != nil {
		return nil, err
	}
	if in.DocumentTenantID != in.OperatingTenantID {
		return nil, apperrors.Validation("document tenant must equal the operating tenant", map[string]any{"field": "documentTenantId"})
	}

	existing, err := s.store.FindIntentByIdempotency(ctx, in.DocumentTenantID, in.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return s.replayIntent(ctx, existing, in)
	}

	companyOK, err := s.store.CompanyExists(ctx, in.OwnerCompanyID, in.DocumentTenantID)
	if err != nil {
		return nil, err
	}
	if !companyOK {
		return nil, apperrors.NotFound("owner_company_id not found")
	}

	uploadID := uuid.New()
	documentID := uuid.New()
	token, tokenHash, err := newUploadToken()
	if err != nil {
		return nil, err
	}
	objectKey := fmt.Sprintf("tenants/%s/executions/%s/actions/%s/pod/%s/%s",
		in.DocumentTenantID, in.ExecutionID, in.ExecutionActionID, documentID, sanitizeFileName(in.FileName, uploadID))
	expires := time.Now().UTC().Add(15 * time.Minute)
	stored, err := s.store.InsertIntent(ctx, domain.PODActionIntent{
		ID: uploadID, DocumentTenantID: in.DocumentTenantID, OperatingTenantID: in.OperatingTenantID,
		ExecutionID: in.ExecutionID, ExecutionActionID: in.ExecutionActionID, ExecutionStopID: in.ExecutionStopID,
		ShipmentID: in.ShipmentID, ShipmentTenantID: in.ShipmentTenantID, CargoID: in.CargoID,
		OwnerCompanyID: in.OwnerCompanyID, AuthorDriverID: in.AuthorDriverID, AuthorCompanyID: in.AuthorCompanyID,
		DocumentID: documentID, ObjectKey: objectKey, UploadTokenHash: tokenHash, MimeType: in.MimeType,
		MaxBytes: s.maxBytes, FileName: in.FileName, IdempotencyKey: in.IdempotencyKey, ExpiresAt: expires,
	})
	if err != nil {
		if apperrors.IsConflict(err) {
			again, findErr := s.store.FindIntentByIdempotency(ctx, in.DocumentTenantID, in.IdempotencyKey)
			if findErr != nil {
				return nil, findErr
			}
			if again == nil {
				return nil, err
			}
			return s.replayIntent(ctx, again, in)
		}
		return nil, err
	}
	return intentResult(&stored, token, false), nil
}

func (s *PODActionEvidenceService) UploadContent(ctx context.Context, uploadID uuid.UUID, token string, body io.Reader, confirm domain.SubjectConfirmation) error {
	intent, err := s.store.LoadIntent(ctx, uploadID)
	if err != nil {
		return err
	}
	if err := confirmMatches(intent, confirm); err != nil {
		return err
	}
	if err := verifyToken(token, intent.UploadTokenHash); err != nil {
		return apperrors.Validation("invalid upload token", nil)
	}
	if intent.Status == "completed" {
		return apperrors.Conflict("upload intent is not active", map[string]any{"status": intent.Status})
	}
	if intent.Status == "uploaded" {
		if _, err := s.storage.Metadata(ctx, intent.ObjectKey); err != nil {
			return err
		}
		return nil
	}
	if intent.Status != "pending" {
		return apperrors.Conflict("upload intent is not active", map[string]any{"status": intent.Status})
	}
	if time.Now().UTC().After(intent.ExpiresAt) {
		return apperrors.Validation("upload intent expired", nil)
	}
	exists, err := s.storage.Exists(ctx, intent.ObjectKey)
	if err != nil {
		return err
	}
	if exists {
		return s.recoverUploadedObject(ctx, uploadID, intent.ObjectKey)
	}
	size, err := s.storage.Put(ctx, intent.ObjectKey, body, intent.MaxBytes)
	if errors.Is(err, storage.ErrObjectExists) {
		return s.recoverUploadedObject(ctx, uploadID, intent.ObjectKey)
	}
	if err != nil {
		return apperrors.Validation("upload failed", map[string]any{"detail": err.Error()})
	}
	return s.store.MarkIntentUploaded(ctx, uploadID, size)
}

func (s *PODActionEvidenceService) recoverUploadedObject(ctx context.Context, uploadID uuid.UUID, objectKey string) error {
	meta, err := s.storage.Metadata(ctx, objectKey)
	if err != nil {
		return err
	}
	return s.store.MarkIntentUploaded(ctx, uploadID, meta.SizeBytes)
}

func (s *PODActionEvidenceService) CompleteUpload(ctx context.Context, uploadID uuid.UUID, checksum string, confirm domain.SubjectConfirmation) (*CompleteActionPODResult, error) {
	intent, err := s.store.LoadIntent(ctx, uploadID)
	if err != nil {
		return nil, err
	}
	if err := confirmMatches(intent, confirm); err != nil {
		return nil, err
	}
	checksum = strings.ToLower(strings.TrimSpace(checksum))
	if intent.Status == "completed" {
		existing, err := s.store.EvidenceByIntent(ctx, uploadID)
		if err != nil {
			return nil, err
		}
		return completeResult(existing), nil
	}
	if intent.Status != "uploaded" {
		return nil, apperrors.Validation("upload content required before finalize", map[string]any{"status": intent.Status})
	}
	ok, err := s.storage.Exists(ctx, intent.ObjectKey)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, apperrors.Validation("stored object missing", nil)
	}
	evidence, err := s.store.CompleteIntent(ctx, uploadID, checksum, s.placement.StorageProvider, s.placement.BucketName)
	if err != nil {
		return nil, err
	}
	return completeResult(evidence), nil
}

func (s *PODActionEvidenceService) GetEvidence(ctx context.Context, operatingTenantID, executionID, actionID uuid.UUID) ([]domain.PODActionEvidence, error) {
	if operatingTenantID == uuid.Nil || executionID == uuid.Nil || actionID == uuid.Nil {
		return nil, apperrors.Validation("delivery action subject is required", nil)
	}
	rows, err := s.store.ListPresentEvidence(ctx, operatingTenantID, executionID, actionID)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []domain.PODActionEvidence{}
	}
	return rows, nil
}

func validateCreateActionPOD(in CreateActionPODIntentInput) error {
	if in.ActionType != domain.PODActionTypeDelivery {
		return apperrors.Validation("delivery action subject is required", map[string]any{"field": "actionType"})
	}
	if in.IdempotencyKey == "" {
		return apperrors.Validation("idempotency key is required", map[string]any{"field": "idempotencyKey"})
	}
	if len(in.IdempotencyKey) > 128 {
		return apperrors.Validation("idempotency key exceeds 128 characters", map[string]any{"field": "idempotencyKey"})
	}
	if in.MimeType == "" {
		return apperrors.Validation("mimeType is required", map[string]any{"field": "mimeType"})
	}
	if !allowedPODMime(in.MimeType) {
		return apperrors.Validation("unsupported mime type", map[string]any{"field": "mimeType"})
	}
	required := []struct {
		id    uuid.UUID
		field string
	}{
		{in.DocumentTenantID, "documentTenantId"},
		{in.OperatingTenantID, "operatingTenantId"},
		{in.ExecutionID, "executionId"},
		{in.ExecutionActionID, "executionActionId"},
		{in.ExecutionStopID, "executionStopId"},
		{in.ShipmentID, "shipmentId"},
		{in.ShipmentTenantID, "shipmentTenantId"},
		{in.CargoID, "cargoId"},
		{in.OwnerCompanyID, "ownerCompanyId"},
		{in.AuthorDriverID, "authorDriverId"},
		{in.AuthorCompanyID, "authorCompanyId"},
	}
	for _, item := range required {
		if item.id == uuid.Nil {
			return apperrors.Validation(item.field+" is required", map[string]any{"field": item.field})
		}
	}
	return nil
}

func (s *PODActionEvidenceService) replayIntent(ctx context.Context, existing *domain.PODActionIntent, in CreateActionPODIntentInput) (*ActionPODIntentResult, error) {
	if err := sameActionPin(existing, in); err != nil {
		return nil, err
	}
	if existing.Status == "completed" {
		return intentResult(existing, "", true), nil
	}
	token, tokenHash, err := newUploadToken()
	if err != nil {
		return nil, err
	}
	if err := s.store.RotateUploadToken(ctx, existing.ID, tokenHash); err != nil {
		return nil, err
	}
	return intentResult(existing, token, true), nil
}

func sameActionPin(existing *domain.PODActionIntent, in CreateActionPODIntentInput) error {
	switch {
	case existing.ExecutionActionID != in.ExecutionActionID:
		return apperrors.Conflict("idempotency key was used for a different delivery action", nil)
	case existing.ExecutionID != in.ExecutionID || existing.ExecutionStopID != in.ExecutionStopID:
		return apperrors.Conflict("execution pin mismatch", nil)
	case existing.ShipmentID != in.ShipmentID:
		return apperrors.Conflict("shipment pin mismatch", nil)
	case existing.ShipmentTenantID != in.ShipmentTenantID:
		return apperrors.Conflict("shipment tenant pin mismatch", nil)
	case existing.CargoID != in.CargoID || existing.OwnerCompanyID != in.OwnerCompanyID:
		return apperrors.Conflict("subject pin mismatch", nil)
	case existing.AuthorDriverID != in.AuthorDriverID || existing.AuthorCompanyID != in.AuthorCompanyID:
		return apperrors.Conflict("author pin mismatch", nil)
	case existing.DocumentTenantID != in.DocumentTenantID || existing.OperatingTenantID != in.OperatingTenantID:
		return apperrors.Conflict("tenant pin mismatch", nil)
	default:
		return nil
	}
}

func confirmMatches(intent *domain.PODActionIntent, c domain.SubjectConfirmation) error {
	checks := []struct {
		got  *uuid.UUID
		want uuid.UUID
		msg  string
	}{
		{c.DocumentTenantID, intent.DocumentTenantID, "tenant pin mismatch"},
		{c.OperatingTenantID, intent.OperatingTenantID, "tenant pin mismatch"},
		{c.ExecutionID, intent.ExecutionID, "execution pin mismatch"},
		{c.ExecutionActionID, intent.ExecutionActionID, "execution pin mismatch"},
		{c.ExecutionStopID, intent.ExecutionStopID, "execution pin mismatch"},
		{c.ShipmentID, intent.ShipmentID, "shipment pin mismatch"},
		{c.ShipmentTenantID, intent.ShipmentTenantID, "shipment tenant pin mismatch"},
		{c.CargoID, intent.CargoID, "subject pin mismatch"},
		{c.OwnerCompanyID, intent.OwnerCompanyID, "owner company pin mismatch"},
		{c.AuthorDriverID, intent.AuthorDriverID, "author pin mismatch"},
		{c.AuthorCompanyID, intent.AuthorCompanyID, "author pin mismatch"},
	}
	for _, check := range checks {
		if check.got != nil && *check.got != check.want {
			return apperrors.Conflict(check.msg, nil)
		}
	}
	return nil
}

func intentResult(intent *domain.PODActionIntent, token string, replayed bool) *ActionPODIntentResult {
	return &ActionPODIntentResult{
		UploadID: intent.ID, DocumentID: intent.DocumentID, UploadToken: token,
		ObjectKey: intent.ObjectKey, ExpiresAt: intent.ExpiresAt, MimeType: intent.MimeType,
		MaxBytes: intent.MaxBytes, Replayed: replayed,
	}
}

func completeResult(evidence *domain.PODActionEvidence) *CompleteActionPODResult {
	return &CompleteActionPODResult{
		EvidenceID: evidence.ID, DocumentID: evidence.DocumentID, UploadID: evidence.UploadIntentID,
		FileID: evidence.DocumentFileID, Status: "completed",
	}
}
