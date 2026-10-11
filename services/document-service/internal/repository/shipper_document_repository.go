package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
)

func (r *DocumentRepository) ShipperShipmentVisible(ctx context.Context, tenantID, shipmentID, shipperCompanyID uuid.UUID) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM transport.shipments
			WHERE id = $1
				AND tenant_id = $2
				AND shipper_company_id = $3
				AND deleted_at IS NULL
		)
	`
	var visible bool
	err := measureDB("document_repository", "shipper_shipment_visible", func() error {
		if err := r.pool.QueryRow(ctx, query, shipmentID, tenantID, shipperCompanyID).Scan(&visible); err != nil {
			return mapDBError(err)
		}
		return nil
	})
	return visible, err
}

func (r *DocumentRepository) ListShipperDocuments(ctx context.Context, filter domain.ShipperDocumentFilter) ([]domain.Document, int, error) {
	var docs []domain.Document
	var total int
	err := measureDB("document_repository", "list_shipper_documents", func() error {
		args := []any{
			filter.TenantID,
			filter.ShipmentID,
			filter.ShipperCompanyID,
			domain.ShipperOperationalDocumentTypes(),
			domain.ShipperVisibleDocumentStatuses(),
		}
		where := []string{
			"d.tenant_id = $1",
			"d.related_entity_type = 'SHIPMENT'",
			"d.related_entity_id = $2",
			"d.deleted_at IS NULL",
			"s.tenant_id = d.tenant_id",
			"s.id = d.related_entity_id",
			"s.shipper_company_id = $3",
			"s.deleted_at IS NULL",
			"d.document_type = ANY($4::text[])",
			"d.document_status = ANY($5::text[])",
		}
		if filter.DocumentType != nil {
			args = append(args, *filter.DocumentType)
			where = append(where, fmt.Sprintf("d.document_type = $%d", len(args)))
		}
		if filter.DocumentStatus != nil {
			args = append(args, *filter.DocumentStatus)
			where = append(where, fmt.Sprintf("d.document_status = $%d", len(args)))
		}
		whereClause := strings.Join(where, " AND ")
		from := `
			FROM documents.documents d
			JOIN transport.shipments s
				ON s.id = d.related_entity_id
				AND s.tenant_id = d.tenant_id
			WHERE ` + whereClause
		if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) "+from, args...).Scan(&total); err != nil {
			return mapDBError(err)
		}
		args = append(args, filter.Limit, filter.Offset)
		listQuery := `
			SELECT d.id, d.tenant_id, d.document_number, d.document_type, d.document_status,
				d.owner_company_id, d.related_entity_type, d.related_entity_id, d.legal_language,
				d.created_at, d.updated_at, d.version
		` + from + fmt.Sprintf(" ORDER BY d.created_at DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))
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
		return rows.Err()
	})
	return docs, total, err
}

func (r *DocumentRepository) GetShipperDocument(ctx context.Context, tenantID, shipperCompanyID, documentID uuid.UUID) (*domain.Document, error) {
	const query = `
		SELECT d.id, d.tenant_id, d.document_number, d.document_type, d.document_status,
			d.owner_company_id, d.related_entity_type, d.related_entity_id, d.legal_language,
			d.created_at, d.updated_at, d.version
		FROM documents.documents d
		JOIN transport.shipments s
			ON s.id = d.related_entity_id
			AND s.tenant_id = d.tenant_id
		WHERE d.id = $1
			AND d.tenant_id = $2
			AND d.related_entity_type = 'SHIPMENT'
			AND d.deleted_at IS NULL
			AND s.shipper_company_id = $3
			AND s.deleted_at IS NULL
			AND d.document_type = ANY($4::text[])
			AND d.document_status = ANY($5::text[])
	`
	var doc *domain.Document
	err := measureDB("document_repository", "get_shipper_document", func() error {
		row := r.pool.QueryRow(ctx, query,
			documentID, tenantID, shipperCompanyID,
			domain.ShipperOperationalDocumentTypes(),
			domain.ShipperVisibleDocumentStatuses(),
		)
		scanned, err := scanDocument(row)
		if err != nil {
			return err
		}
		doc = scanned
		return nil
	})
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, apperrors.NotFound("document not found")
	}
	return doc, nil
}
