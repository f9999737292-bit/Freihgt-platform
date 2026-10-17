package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/document-service/internal/domain"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
)

type PODActionEvidenceRepository struct {
	pool *pgxpool.Pool
}

func NewPODActionEvidenceRepository(pool *pgxpool.Pool) *PODActionEvidenceRepository {
	return &PODActionEvidenceRepository{pool: pool}
}

func (r *PODActionEvidenceRepository) CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error) {
	return NewDocumentRepository(r.pool).CompanyExists(ctx, companyID, tenantID)
}

func (r *PODActionEvidenceRepository) ShipmentOwnerTenant(ctx context.Context, shipmentID uuid.UUID) (uuid.UUID, bool, error) {
	var tenantID uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT tenant_id
		FROM transport.shipments
		WHERE id = $1 AND deleted_at IS NULL`, shipmentID).Scan(&tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, mapDBError(err)
	}
	return tenantID, true, nil
}

func (r *PODActionEvidenceRepository) FindDeliveryAction(ctx context.Context, operatingTenantID, executionID, actionID uuid.UUID) (*domain.DeliveryActionSubject, error) {
	var subject domain.DeliveryActionSubject
	err := r.pool.QueryRow(ctx, `
		SELECT a.action_type,
		       e.operating_tenant_id,
		       e.id,
		       a.id,
		       a.execution_stop_id,
		       a.shipment_id,
		       a.shipment_tenant_id,
		       a.cargo_id,
		       e.carrier_company_id
		FROM transport.transport_execution_actions AS a
		JOIN transport.transport_execution_stops AS s ON s.id = a.execution_stop_id
		JOIN transport.transport_executions AS e
		  ON e.id = s.execution_id
		 AND e.operating_tenant_id = s.operating_tenant_id
		WHERE e.operating_tenant_id = $1
		  AND e.id = $2
		  AND a.id = $3`,
		operatingTenantID, executionID, actionID).Scan(
		&subject.ActionType,
		&subject.OperatingTenantID,
		&subject.ExecutionID,
		&subject.ExecutionActionID,
		&subject.ExecutionStopID,
		&subject.ShipmentID,
		&subject.ShipmentTenantID,
		&subject.CargoID,
		&subject.CarrierCompanyID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	return &subject, nil
}

func (r *PODActionEvidenceRepository) FindIntentByIdempotency(ctx context.Context, documentTenantID uuid.UUID, key string) (*domain.PODActionIntent, error) {
	intent, err := scanActionIntent(r.pool.QueryRow(ctx, actionIntentSelect+`
		WHERE document_tenant_id = $1 AND idempotency_key = $2`, documentTenantID, key))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	return intent, nil
}

func (r *PODActionEvidenceRepository) LoadIntent(ctx context.Context, uploadID uuid.UUID) (*domain.PODActionIntent, error) {
	intent, err := scanActionIntent(r.pool.QueryRow(ctx, actionIntentSelect+` WHERE id = $1`, uploadID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NotFound("upload intent not found")
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	return intent, nil
}

func (r *PODActionEvidenceRepository) InsertIntent(ctx context.Context, in domain.PODActionIntent) (domain.PODActionIntent, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.PODActionIntent{}, mapDBError(err)
	}
	defer tx.Rollback(ctx)

	shipmentType := domain.RelatedEntityTypeShipment
	if _, err := tx.Exec(ctx, `
		INSERT INTO documents.documents (
			id, tenant_id, document_number, document_type, document_status,
			owner_company_id, related_entity_type, related_entity_id, legal_language
		) VALUES ($1,$2,$3,'POD','DRAFT',$4,$5,$6,'ru-RU')`,
		in.DocumentID,
		in.DocumentTenantID,
		"POD-"+strings.ReplaceAll(in.ID.String(), "-", "")[:12],
		in.OwnerCompanyID,
		shipmentType,
		in.ShipmentID,
	); err != nil {
		return domain.PODActionIntent{}, mapDBError(err)
	}

	if err := tx.QueryRow(ctx, `
		INSERT INTO documents.document_versions (document_id, version_number, payload_json)
		VALUES ($1, 1, NULL)
		RETURNING id`, in.DocumentID).Scan(&in.DocumentVersionID); err != nil {
		return domain.PODActionIntent{}, mapDBError(err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO documents.pod_delivery_action_upload_intent (
			id, document_tenant_id, operating_tenant_id, execution_id, execution_action_id,
			execution_stop_id, shipment_id, shipment_tenant_id, cargo_id, owner_company_id,
			author_driver_id, author_company_id, document_id, document_version_id,
			object_key, upload_token_hash, mime_type, max_bytes, file_name, status,
			idempotency_key, expires_at
		) VALUES (
			$1,$2,$3,$4,$5,
			$6,$7,$8,$9,$10,
			$11,$12,$13,$14,
			$15,$16,$17,$18,$19,'pending',
			$20,$21
		)`,
		in.ID, in.DocumentTenantID, in.OperatingTenantID, in.ExecutionID, in.ExecutionActionID,
		in.ExecutionStopID, in.ShipmentID, in.ShipmentTenantID, in.CargoID, in.OwnerCompanyID,
		in.AuthorDriverID, in.AuthorCompanyID, in.DocumentID, in.DocumentVersionID,
		in.ObjectKey, in.UploadTokenHash, in.MimeType, in.MaxBytes, emptyAsNil(in.FileName),
		in.IdempotencyKey, in.ExpiresAt,
	); err != nil {
		return domain.PODActionIntent{}, mapDBError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.PODActionIntent{}, mapDBError(err)
	}
	in.Status = "pending"
	return in, nil
}

func (r *PODActionEvidenceRepository) MarkIntentUploaded(ctx context.Context, uploadID uuid.UUID, size int64) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE documents.pod_delivery_action_upload_intent
		SET status = 'uploaded', byte_size = $2
		WHERE id = $1 AND status IN ('pending', 'uploaded')`, uploadID, size)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.Conflict("upload intent is not active", nil)
	}
	return nil
}

func (r *PODActionEvidenceRepository) EvidenceByIntent(ctx context.Context, uploadID uuid.UUID) (*domain.PODActionEvidence, error) {
	row, err := scanActionEvidence(r.pool.QueryRow(ctx, actionEvidenceSelect+` WHERE upload_intent_id = $1`, uploadID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.Internal("completed upload has no action evidence binding", nil)
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	return row, nil
}

// CompleteIntent inserts the file row, the immutable evidence binding, and the completed
// intent status in one database transaction. A completed intent returns the existing binding.
func (r *PODActionEvidenceRepository) CompleteIntent(ctx context.Context, uploadID uuid.UUID, checksum string) (*domain.PODActionEvidence, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer tx.Rollback(ctx)

	intent, err := scanActionIntent(tx.QueryRow(ctx, actionIntentSelect+` WHERE id = $1 FOR UPDATE`, uploadID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NotFound("upload intent not found")
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	if intent.Status == "completed" {
		existing, err := scanActionEvidence(tx.QueryRow(ctx, actionEvidenceSelect+` WHERE upload_intent_id = $1`, uploadID))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Internal("completed upload has no action evidence binding", nil)
		}
		if err != nil {
			return nil, mapDBError(err)
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, mapDBError(err)
		}
		return existing, nil
	}
	if intent.Status != "uploaded" {
		return nil, apperrors.Validation("upload content required before finalize", map[string]any{"status": intent.Status})
	}

	var alive bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM documents.documents
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		)`, intent.DocumentID, intent.DocumentTenantID).Scan(&alive); err != nil {
		return nil, mapDBError(err)
	}
	if !alive {
		return nil, apperrors.NotFound("document not found")
	}
	var versionOK bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM documents.document_versions
			WHERE id = $1 AND document_id = $2
		)`, intent.DocumentVersionID, intent.DocumentID).Scan(&versionOK); err != nil {
		return nil, mapDBError(err)
	}
	if !versionOK {
		return nil, apperrors.NotFound("document version not found")
	}

	fileType := domain.FileTypeImage
	if strings.Contains(strings.ToLower(intent.MimeType), "pdf") {
		fileType = domain.FileTypePDF
	}
	fileName := intent.FileName
	if strings.TrimSpace(fileName) == "" {
		fileName = intent.ObjectKey
	}
	var checksumPtr any
	if strings.TrimSpace(checksum) != "" {
		checksumPtr = checksum
	}
	var size any
	if intent.ByteSize > 0 {
		size = intent.ByteSize
	}
	fileID := uuid.New()
	bucket := "local"
	if _, err := tx.Exec(ctx, `
		INSERT INTO documents.document_files (
			id, document_id, document_version_id, file_type, storage_provider,
			bucket_name, object_key, file_name, mime_type, file_size_bytes, checksum_sha256
		) VALUES ($1,$2,$3,$4,'local',$5,$6,$7,$8,$9,$10)`,
		fileID, intent.DocumentID, intent.DocumentVersionID, fileType, bucket,
		intent.ObjectKey, fileName, intent.MimeType, size, checksumPtr,
	); err != nil {
		return nil, mapDBError(err)
	}

	evidenceID := uuid.New()
	now := time.Now().UTC()
	if _, err := tx.Exec(ctx, `
		INSERT INTO documents.pod_delivery_action_evidence (
			id, upload_intent_id, document_tenant_id, operating_tenant_id, execution_id,
			execution_action_id, execution_stop_id, shipment_id, shipment_tenant_id, cargo_id,
			owner_company_id, author_driver_id, author_company_id, document_id, document_version_id,
			document_file_id, idempotency_key, created_at
		) VALUES (
			$1,$2,$3,$4,$5,
			$6,$7,$8,$9,$10,
			$11,$12,$13,$14,$15,
			$16,$17,$18
		)`,
		evidenceID, intent.ID, intent.DocumentTenantID, intent.OperatingTenantID, intent.ExecutionID,
		intent.ExecutionActionID, intent.ExecutionStopID, intent.ShipmentID, intent.ShipmentTenantID, intent.CargoID,
		intent.OwnerCompanyID, intent.AuthorDriverID, intent.AuthorCompanyID, intent.DocumentID, intent.DocumentVersionID,
		fileID, intent.IdempotencyKey, now,
	); err != nil {
		return nil, mapDBError(err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE documents.pod_delivery_action_upload_intent
		SET status = 'completed', checksum_sha256 = $2, completed_at = $3
		WHERE id = $1`, uploadID, checksumPtr, now); err != nil {
		return nil, mapDBError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, mapDBError(err)
	}
	return &domain.PODActionEvidence{
		ID: evidenceID, UploadIntentID: intent.ID,
		DocumentTenantID: intent.DocumentTenantID, OperatingTenantID: intent.OperatingTenantID,
		ExecutionID: intent.ExecutionID, ExecutionActionID: intent.ExecutionActionID,
		ExecutionStopID: intent.ExecutionStopID, ShipmentID: intent.ShipmentID,
		ShipmentTenantID: intent.ShipmentTenantID, CargoID: intent.CargoID,
		OwnerCompanyID: intent.OwnerCompanyID, AuthorDriverID: intent.AuthorDriverID,
		AuthorCompanyID: intent.AuthorCompanyID, DocumentID: intent.DocumentID,
		DocumentVersionID: intent.DocumentVersionID, DocumentFileID: fileID,
		IdempotencyKey: intent.IdempotencyKey, CreatedAt: now,
	}, nil
}

func (r *PODActionEvidenceRepository) ListPresentEvidence(ctx context.Context, operatingTenantID, executionID, actionID uuid.UUID) ([]domain.PODActionEvidence, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT e.id, e.upload_intent_id, e.document_tenant_id, e.operating_tenant_id, e.execution_id,
		       e.execution_action_id, e.execution_stop_id, e.shipment_id, e.shipment_tenant_id, e.cargo_id,
		       e.owner_company_id, e.author_driver_id, e.author_company_id, e.document_id, e.document_version_id,
		       e.document_file_id, e.idempotency_key, e.created_at
		FROM documents.pod_delivery_action_evidence AS e
		JOIN documents.pod_delivery_action_upload_intent AS i ON i.id = e.upload_intent_id
		JOIN documents.document_files AS f ON f.id = e.document_file_id
		JOIN documents.documents AS d ON d.id = e.document_id AND d.deleted_at IS NULL
		WHERE e.operating_tenant_id = $1
		  AND e.execution_id = $2
		  AND e.execution_action_id = $3
		  AND i.status = 'completed'
		ORDER BY e.created_at ASC, e.id ASC`, operatingTenantID, executionID, actionID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	out := make([]domain.PODActionEvidence, 0)
	for rows.Next() {
		item, err := scanActionEvidence(rows)
		if err != nil {
			return nil, mapDBError(err)
		}
		out = append(out, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, mapDBError(err)
	}
	return out, nil
}

const actionIntentSelect = `
	SELECT id, document_tenant_id, operating_tenant_id, execution_id, execution_action_id,
	       execution_stop_id, shipment_id, shipment_tenant_id, cargo_id, owner_company_id,
	       author_driver_id, author_company_id, document_id, document_version_id,
	       object_key, upload_token_hash, mime_type, max_bytes, COALESCE(file_name, ''),
	       status, idempotency_key, COALESCE(checksum_sha256, ''), COALESCE(byte_size, 0),
	       expires_at, completed_at, created_at
	FROM documents.pod_delivery_action_upload_intent`

const actionEvidenceSelect = `
	SELECT id, upload_intent_id, document_tenant_id, operating_tenant_id, execution_id,
	       execution_action_id, execution_stop_id, shipment_id, shipment_tenant_id, cargo_id,
	       owner_company_id, author_driver_id, author_company_id, document_id, document_version_id,
	       document_file_id, idempotency_key, created_at
	FROM documents.pod_delivery_action_evidence`

type intentScanner interface {
	Scan(dest ...any) error
}

func scanActionIntent(row intentScanner) (*domain.PODActionIntent, error) {
	var intent domain.PODActionIntent
	err := row.Scan(
		&intent.ID, &intent.DocumentTenantID, &intent.OperatingTenantID, &intent.ExecutionID, &intent.ExecutionActionID,
		&intent.ExecutionStopID, &intent.ShipmentID, &intent.ShipmentTenantID, &intent.CargoID, &intent.OwnerCompanyID,
		&intent.AuthorDriverID, &intent.AuthorCompanyID, &intent.DocumentID, &intent.DocumentVersionID,
		&intent.ObjectKey, &intent.UploadTokenHash, &intent.MimeType, &intent.MaxBytes, &intent.FileName,
		&intent.Status, &intent.IdempotencyKey, &intent.ChecksumSHA256, &intent.ByteSize,
		&intent.ExpiresAt, &intent.CompletedAt, &intent.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &intent, nil
}

func emptyAsNil(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

func scanActionEvidence(row intentScanner) (*domain.PODActionEvidence, error) {
	var item domain.PODActionEvidence
	err := row.Scan(
		&item.ID, &item.UploadIntentID, &item.DocumentTenantID, &item.OperatingTenantID, &item.ExecutionID,
		&item.ExecutionActionID, &item.ExecutionStopID, &item.ShipmentID, &item.ShipmentTenantID, &item.CargoID,
		&item.OwnerCompanyID, &item.AuthorDriverID, &item.AuthorCompanyID, &item.DocumentID, &item.DocumentVersionID,
		&item.DocumentFileID, &item.IdempotencyKey, &item.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &item, nil
}
