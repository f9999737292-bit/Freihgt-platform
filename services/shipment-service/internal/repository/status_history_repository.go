package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

func (r *ShipmentRepository) ListStatusHistory(ctx context.Context, filter domain.ListStatusHistoryFilter) ([]domain.ShipmentStatusHistory, int, error) {
	var items []domain.ShipmentStatusHistory
	var total int
	err := measureDB("shipment_repository", "list_status_history", func() error {
		if err := r.pool.QueryRow(ctx, countStatusHistoryQuery, filter.TenantID, filter.ShipmentID).Scan(&total); err != nil {
			return mapDBError(err)
		}

		order := strings.ToLower(strings.TrimSpace(filter.Order))
		if order != "asc" {
			order = "desc"
		}
		orderClause := "occurred_at DESC, recorded_at DESC, id ASC"
		if order == "asc" {
			orderClause = "occurred_at ASC, recorded_at ASC, id ASC"
		}

		offset := (filter.Page - 1) * filter.Limit
		query := fmt.Sprintf(`%s ORDER BY %s LIMIT $3 OFFSET $4`, listStatusHistoryQuery, orderClause)
		rows, err := r.pool.Query(ctx, query, filter.TenantID, filter.ShipmentID, filter.Limit, offset)
		if err != nil {
			return mapDBError(err)
		}
		defer rows.Close()

		result := make([]domain.ShipmentStatusHistory, 0)
		for rows.Next() {
			item, err := scanStatusHistory(rows)
			if err != nil {
				return err
			}
			result = append(result, item)
		}
		if err := rows.Err(); err != nil {
			return mapDBError(err)
		}
		items = result
		return nil
	})
	return items, total, err
}

func (r *ShipmentRepository) HasInitialStatusHistory(ctx context.Context, tenantID, shipmentID uuid.UUID) (bool, error) {
	var exists bool
	err := measureDB("shipment_repository", "has_initial_status_history", func() error {
		if err := r.pool.QueryRow(ctx, hasInitialStatusHistoryQuery, tenantID, shipmentID).Scan(&exists); err != nil {
			return mapDBError(err)
		}
		return nil
	})
	return exists, err
}

type statusHistoryScanner interface {
	Scan(dest ...any) error
}

func scanStatusHistory(row statusHistoryScanner) (domain.ShipmentStatusHistory, error) {
	var item domain.ShipmentStatusHistory
	var fromStatus *string
	var reasonCode *string
	var actorType string
	var actorID *uuid.UUID
	var correlationID *string

	err := row.Scan(
		&item.ID,
		&item.TenantID,
		&item.ShipmentID,
		&item.ShipmentVersion,
		&fromStatus,
		&item.ToStatus,
		&reasonCode,
		&item.Source,
		&actorType,
		&actorID,
		&correlationID,
		&item.OccurredAt,
		&item.RecordedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ShipmentStatusHistory{}, apperrors.NotFound("status history not found")
	}
	if err != nil {
		return domain.ShipmentStatusHistory{}, mapDBError(err)
	}

	item.FromStatus = fromStatus
	item.ReasonCode = reasonCode
	item.ActorType = domain.ActorType(actorType)
	item.ActorID = actorID
	item.CorrelationID = correlationID
	return item, nil
}
