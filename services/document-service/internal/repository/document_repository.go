package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/document-service/internal/domain"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
)

type DocumentRepository struct {
	pool *pgxpool.Pool
}

func NewDocumentRepository(pool *pgxpool.Pool) *DocumentRepository {
	return &DocumentRepository{pool: pool}
}

type DocumentDetail struct {
	Document      *domain.Document
	LatestVersion *domain.DocumentVersion
	Files         []domain.DocumentFile
}

func (r *DocumentRepository) CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error) {
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

func (r *DocumentRepository) UserExists(ctx context.Context, userID, tenantID uuid.UUID) (bool, error) {
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

func (r *DocumentRepository) ShipmentExists(ctx context.Context, shipmentID, tenantID uuid.UUID) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM transport.shipments
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		)
	`
	var exists bool
	if err := r.pool.QueryRow(ctx, query, shipmentID, tenantID).Scan(&exists); err != nil {
		return false, mapDBError(err)
	}
	return exists, nil
}

func (r *DocumentRepository) CreateDocument(ctx context.Context, in domain.CreateDocumentInput) (*domain.Document, *domain.DocumentVersion, error) {
	var doc *domain.Document
	var version *domain.DocumentVersion
	err := measureDB("document_repository", "create_document", func() error {
		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return mapDBError(err)
		}
		defer tx.Rollback(ctx)

		const docQuery = `
		INSERT INTO documents.documents (
			tenant_id, document_number, document_type, document_status,
			owner_company_id, related_entity_type, related_entity_id, legal_language
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, tenant_id, document_number, document_type, document_status,
			owner_company_id, related_entity_type, related_entity_id, legal_language,
			created_at, updated_at, version
	`
		createdDoc, err := scanDocument(tx.QueryRow(ctx, docQuery,
			in.TenantID,
			strings.TrimSpace(in.DocumentNumber),
			strings.TrimSpace(in.DocumentType),
			domain.DocumentStatusDraft,
			in.OwnerCompanyID,
			optionalString(in.RelatedEntityType),
			optionalUUID(in.RelatedEntityID),
			domain.NormalizeLegalLanguage(in.LegalLanguage),
		))
		if err != nil {
			return err
		}

		const versionQuery = `
		INSERT INTO documents.document_versions (
			document_id, version_number, payload_json
		) VALUES ($1, 1, $2)
		RETURNING id, document_id, version_number, payload_json, payload_xml_path, pdf_file_path, created_at
	`
		createdVersion, err := scanDocumentVersion(tx.QueryRow(ctx, versionQuery, createdDoc.ID, payloadJSONArg(in.PayloadJSON)))
		if err != nil {
			return err
		}

		if err := tx.Commit(ctx); err != nil {
			return mapDBError(err)
		}
		doc = createdDoc
		version = createdVersion
		return nil
	})
	return doc, version, err
}

func (r *DocumentRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Document, error) {
	var result *domain.Document
	err := measureDB("document_repository", "get_document", func() error {
		const query = `
		SELECT id, tenant_id, document_number, document_type, document_status,
			owner_company_id, related_entity_type, related_entity_id, legal_language,
			created_at, updated_at, version
		FROM documents.documents
		WHERE id = $1 AND deleted_at IS NULL
	`
		doc, err := scanDocument(r.pool.QueryRow(ctx, query, id))
		if err != nil {
			return err
		}
		result = doc
		return nil
	})
	return result, err
}

func (r *DocumentRepository) GetByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.Document, error) {
	var result *domain.Document
	err := measureDB("document_repository", "get_document", func() error {
		const query = `
		SELECT id, tenant_id, document_number, document_type, document_status,
			owner_company_id, related_entity_type, related_entity_id, legal_language,
			created_at, updated_at, version
		FROM documents.documents
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`
		doc, err := scanDocument(r.pool.QueryRow(ctx, query, id, tenantID))
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.NotFound("document not found")
		}
		if err != nil {
			return err
		}
		result = doc
		return nil
	})
	return result, err
}

func (r *DocumentRepository) GetDetail(ctx context.Context, id uuid.UUID) (*DocumentDetail, error) {
	doc, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	latestVersion, err := r.getLatestVersion(ctx, id)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		latestVersion = nil
	}

	files, err := r.listFiles(ctx, id)
	if err != nil {
		return nil, err
	}

	return &DocumentDetail{
		Document:      doc,
		LatestVersion: latestVersion,
		Files:         files,
	}, nil
}

func (r *DocumentRepository) List(ctx context.Context, filter domain.ListDocumentsFilter) ([]domain.Document, int, error) {
	var docs []domain.Document
	var total int
	err := measureDB("document_repository", "list_documents", func() error {
		args := []any{filter.TenantID}
		where := []string{"tenant_id = $1", "deleted_at IS NULL"}

		if filter.DocumentType != nil {
			args = append(args, *filter.DocumentType)
			where = append(where, fmt.Sprintf("document_type = $%d", len(args)))
		}
		if filter.DocumentStatus != nil {
			args = append(args, *filter.DocumentStatus)
			where = append(where, fmt.Sprintf("document_status = $%d", len(args)))
		}
		if filter.RelatedEntityType != nil {
			args = append(args, *filter.RelatedEntityType)
			where = append(where, fmt.Sprintf("related_entity_type = $%d", len(args)))
		}
		if filter.RelatedEntityID != nil {
			args = append(args, *filter.RelatedEntityID)
			where = append(where, fmt.Sprintf("related_entity_id = $%d", len(args)))
		}

		whereClause := strings.Join(where, " AND ")
		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM documents.documents WHERE %s", whereClause)
		if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
			return mapDBError(err)
		}

		args = append(args, filter.Limit, filter.Offset)
		listQuery := fmt.Sprintf(`
		SELECT id, tenant_id, document_number, document_type, document_status,
			owner_company_id, related_entity_type, related_entity_id, legal_language,
			created_at, updated_at, version
		FROM documents.documents
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, len(args)-1, len(args))

		rows, err := r.pool.Query(ctx, listQuery, args...)
		if err != nil {
			return mapDBError(err)
		}
		defer rows.Close()

		docs = make([]domain.Document, 0)
		for rows.Next() {
			doc, err := scanDocumentRows(rows)
			if err != nil {
				return err
			}
			docs = append(docs, *doc)
		}
		if err := rows.Err(); err != nil {
			return mapDBError(err)
		}
		return nil
	})
	return docs, total, err
}

func (r *DocumentRepository) HasVersions(ctx context.Context, documentID uuid.UUID) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM documents.document_versions WHERE document_id = $1
		)
	`
	var exists bool
	if err := r.pool.QueryRow(ctx, query, documentID).Scan(&exists); err != nil {
		return false, mapDBError(err)
	}
	return exists, nil
}

func (r *DocumentRepository) VersionBelongsToDocument(ctx context.Context, versionID, documentID uuid.UUID) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM documents.document_versions
			WHERE id = $1 AND document_id = $2
		)
	`
	var exists bool
	if err := r.pool.QueryRow(ctx, query, versionID, documentID).Scan(&exists); err != nil {
		return false, mapDBError(err)
	}
	return exists, nil
}

func (r *DocumentRepository) CreateVersion(ctx context.Context, documentID uuid.UUID, in domain.CreateDocumentVersionInput) (*domain.DocumentVersion, error) {
	var result *domain.DocumentVersion
	err := measureDB("document_repository", "create_document_version", func() error {
		const maxQuery = `
		SELECT COALESCE(MAX(version_number), 0) FROM documents.document_versions WHERE document_id = $1
	`
		var maxVersion int
		if err := r.pool.QueryRow(ctx, maxQuery, documentID).Scan(&maxVersion); err != nil {
			return mapDBError(err)
		}

		const query = `
		INSERT INTO documents.document_versions (
			document_id, version_number, payload_json, payload_xml_path, pdf_file_path
		) VALUES ($1, $2, $3, $4, $5)
		RETURNING id, document_id, version_number, payload_json, payload_xml_path, pdf_file_path, created_at
	`
		version, err := scanDocumentVersion(r.pool.QueryRow(ctx, query,
			documentID,
			maxVersion+1,
			payloadJSONArg(in.PayloadJSON),
			optionalString(in.PayloadXMLPath),
			optionalString(in.PDFFilePath),
		))
		if err != nil {
			return err
		}
		result = version
		return nil
	})
	return result, err
}

func (r *DocumentRepository) AddFile(ctx context.Context, documentID uuid.UUID, in domain.CreateDocumentFileInput) (*domain.DocumentFile, error) {
	var result *domain.DocumentFile
	err := measureDB("document_repository", "add_document_file", func() error {
		const query = `
		INSERT INTO documents.document_files (
			document_id, document_version_id, file_type, storage_provider,
			bucket_name, object_key, file_name, mime_type, file_size_bytes, checksum_sha256
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, document_id, document_version_id, file_type, storage_provider,
			bucket_name, object_key, file_name, mime_type, file_size_bytes, checksum_sha256, created_at
	`
		file, err := scanDocumentFile(r.pool.QueryRow(ctx, query,
			documentID,
			in.DocumentVersionID,
			strings.TrimSpace(in.FileType),
			domain.NormalizeStorageProvider(in.StorageProvider),
			optionalString(in.BucketName),
			strings.TrimSpace(in.ObjectKey),
			optionalString(in.FileName),
			optionalString(in.MimeType),
			optionalInt64(in.FileSizeBytes),
			optionalString(in.ChecksumSHA256),
		))
		if err != nil {
			return err
		}
		result = file
		return nil
	})
	return result, err
}

func (r *DocumentRepository) UpdateDocumentStatus(ctx context.Context, id, tenantID uuid.UUID, status string, expectedVersion int) (*domain.Document, error) {
	operation := documentStatusOperation(status)
	var result *domain.Document
	err := measureDB("document_repository", operation, func() error {
		const query = `
		UPDATE documents.documents
		SET document_status = $1, version = version + 1, updated_at = now()
		WHERE id = $2 AND tenant_id = $3 AND deleted_at IS NULL AND version = $4
		RETURNING id, tenant_id, document_number, document_type, document_status,
			owner_company_id, related_entity_type, related_entity_id, legal_language,
			created_at, updated_at, version
	`
		doc, err := scanDocumentUpdate(r.pool.QueryRow(ctx, query, status, id, tenantID, expectedVersion))
		if err != nil {
			return err
		}
		result = doc
		return nil
	})
	return result, err
}

func documentStatusOperation(status string) string {
	switch status {
	case domain.DocumentStatusReadyForSigning:
		return "mark_ready_for_signing"
	case domain.DocumentStatusCancelled:
		return "cancel_document"
	case domain.DocumentStatusArchived:
		return "archive_document"
	default:
		return "update_document_status"
	}
}

func (r *DocumentRepository) getLatestVersion(ctx context.Context, documentID uuid.UUID) (*domain.DocumentVersion, error) {
	const query = `
		SELECT id, document_id, version_number, payload_json, payload_xml_path, pdf_file_path, created_at
		FROM documents.document_versions
		WHERE document_id = $1
		ORDER BY version_number DESC
		LIMIT 1
	`
	return scanDocumentVersion(r.pool.QueryRow(ctx, query, documentID))
}

func (r *DocumentRepository) listFiles(ctx context.Context, documentID uuid.UUID) ([]domain.DocumentFile, error) {
	const query = `
		SELECT id, document_id, document_version_id, file_type, storage_provider,
			bucket_name, object_key, file_name, mime_type, file_size_bytes, checksum_sha256, created_at
		FROM documents.document_files
		WHERE document_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.pool.Query(ctx, query, documentID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	files := make([]domain.DocumentFile, 0)
	for rows.Next() {
		file, err := scanDocumentFileRows(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, *file)
	}
	if err := rows.Err(); err != nil {
		return nil, mapDBError(err)
	}
	return files, nil
}

func payloadJSONArg(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return []byte(raw)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanDocument(row rowScanner) (*domain.Document, error) {
	var d domain.Document
	err := row.Scan(
		&d.ID, &d.TenantID, &d.DocumentNumber, &d.DocumentType, &d.DocumentStatus,
		&d.OwnerCompanyID, &d.RelatedEntityType, &d.RelatedEntityID, &d.LegalLanguage,
		&d.CreatedAt, &d.UpdatedAt, &d.Version,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NotFound("document not found")
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	return &d, nil
}

func scanDocumentRows(rows pgx.Rows) (*domain.Document, error) {
	var d domain.Document
	err := rows.Scan(
		&d.ID, &d.TenantID, &d.DocumentNumber, &d.DocumentType, &d.DocumentStatus,
		&d.OwnerCompanyID, &d.RelatedEntityType, &d.RelatedEntityID, &d.LegalLanguage,
		&d.CreatedAt, &d.UpdatedAt, &d.Version,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &d, nil
}

func scanDocumentUpdate(row rowScanner) (*domain.Document, error) {
	doc, err := scanDocument(row)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func scanDocumentVersion(row rowScanner) (*domain.DocumentVersion, error) {
	var v domain.DocumentVersion
	var payload []byte
	err := row.Scan(
		&v.ID, &v.DocumentID, &v.VersionNumber, &payload,
		&v.PayloadXMLPath, &v.PDFFilePath, &v.CreatedAt,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	if len(payload) > 0 {
		v.PayloadJSON = json.RawMessage(payload)
	}
	return &v, nil
}

func scanDocumentFile(row rowScanner) (*domain.DocumentFile, error) {
	var f domain.DocumentFile
	err := row.Scan(
		&f.ID, &f.DocumentID, &f.DocumentVersionID, &f.FileType, &f.StorageProvider,
		&f.BucketName, &f.ObjectKey, &f.FileName, &f.MimeType, &f.FileSizeBytes, &f.ChecksumSHA256, &f.CreatedAt,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &f, nil
}

func scanDocumentFileRows(rows pgx.Rows) (*domain.DocumentFile, error) {
	var f domain.DocumentFile
	err := rows.Scan(
		&f.ID, &f.DocumentID, &f.DocumentVersionID, &f.FileType, &f.StorageProvider,
		&f.BucketName, &f.ObjectKey, &f.FileName, &f.MimeType, &f.FileSizeBytes, &f.ChecksumSHA256, &f.CreatedAt,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &f, nil
}
