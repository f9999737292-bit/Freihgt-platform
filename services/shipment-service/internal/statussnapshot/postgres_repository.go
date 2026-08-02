package statussnapshot

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/statussnapshot"
	"github.com/freight-platform/shipment-service/internal/domain"
)

const createdStatus = domain.ShipmentStatusCreated

type PostgresSnapshotRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresSnapshotRepository(pool *pgxpool.Pool) *PostgresSnapshotRepository {
	return &PostgresSnapshotRepository{pool: pool}
}

func (r *PostgresSnapshotRepository) StreamShipmentStatusSnapshot(
	ctx context.Context,
	request SnapshotRequest,
	consume func(ShipmentSnapshotRow) error,
) (SnapshotStats, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return SnapshotStats{}, newExportError(CodeDatabaseUnavailable, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	authoritativeRows, authoritativeTenants, err := r.authoritativeCounts(ctx, tx, request)
	if err != nil {
		return SnapshotStats{}, err
	}

	streamQuery := snapshotStreamQueryAll
	var args []any
	if request.TenantID != nil {
		streamQuery = snapshotStreamQueryTenant
		args = append(args, *request.TenantID)
	}

	rows, err := tx.Query(ctx, streamQuery, args...)
	if err != nil {
		return SnapshotStats{}, newExportError(CodeDatabaseUnavailable, err)
	}
	defer rows.Close()

	var (
		stats       SnapshotStats
		lastTenant  uuid.UUID
		hasTenant   bool
		prevKey     *struct{ tenantID, shipmentID uuid.UUID }
		hasPrevious bool
	)

	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return SnapshotStats{}, newExportError(CodeExportCancelled, err)
		}

		row, err := scanSnapshotRow(rows)
		if err != nil {
			return SnapshotStats{}, err
		}
		if err := validateAuthoritativeRow(row); err != nil {
			return SnapshotStats{}, err
		}

		key := struct{ tenantID, shipmentID uuid.UUID }{row.TenantID, row.ShipmentID}
		if hasPrevious {
			cmp := compareKeys(key.tenantID, key.shipmentID, prevKey.tenantID, prevKey.shipmentID)
			if cmp < 0 {
				return SnapshotStats{}, newExportError(statussnapshot.CodeRecordOrderViolation, nil)
			}
			if cmp == 0 {
				return SnapshotStats{}, newExportError(statussnapshot.CodeDuplicateShipment, nil)
			}
		}
		prevKey = &key
		hasPrevious = true

		if err := consume(row.ShipmentSnapshotRow); err != nil {
			return SnapshotStats{}, err
		}

		stats.RowCount++
		if !hasTenant {
			lastTenant = row.TenantID
			hasTenant = true
			stats.TenantCount = 1
		} else if row.TenantID != lastTenant {
			lastTenant = row.TenantID
			stats.TenantCount++
		}
	}
	if err := rows.Err(); err != nil {
		return SnapshotStats{}, newExportError(CodeDatabaseUnavailable, err)
	}

	if stats.RowCount != authoritativeRows {
		return SnapshotStats{}, newExportError(CodeSourceRowCountMismatch, nil)
	}
	if stats.TenantCount != authoritativeTenants {
		return SnapshotStats{}, newExportError(CodeSourceTenantCountMismatch, nil)
	}

	if err := tx.Commit(ctx); err != nil {
		return SnapshotStats{}, newExportError(CodeDatabaseUnavailable, err)
	}
	return stats, nil
}

func (r *PostgresSnapshotRepository) authoritativeCounts(ctx context.Context, tx pgx.Tx, request SnapshotRequest) (int64, int64, error) {
	query := snapshotCountQueryAll
	var args []any
	if request.TenantID != nil {
		query = snapshotCountQueryTenant
		args = append(args, *request.TenantID)
	}
	var rowCount, tenantCount int64
	if err := tx.QueryRow(ctx, query, args...).Scan(&rowCount, &tenantCount); err != nil {
		return 0, 0, newExportError(CodeDatabaseUnavailable, err)
	}
	return rowCount, tenantCount, nil
}

type scannedRow struct {
	ShipmentSnapshotRow
	historyToStatus string
	historyVersion  int64
	hasHistory      bool
	outboxAggregateID      *uuid.UUID
	outboxAggregateVersion *int32
	outboxEventType        *string
	outboxStatus           *string
	outboxPayload          []byte
}

func scanSnapshotRow(rows pgx.Rows) (scannedRow, error) {
	var (
		row            scannedRow
		previous       *string
		lastEvent      *uuid.UUID
		lastSource     *uuid.UUID
		sourceUpdated  *time.Time
		historyStatus  *string
		historyVersion *int64
	)
	if err := rows.Scan(
		&row.TenantID,
		&row.ShipmentID,
		&row.CurrentStatus,
		&previous,
		&row.AggregateVersion,
		&lastEvent,
		&lastSource,
		&sourceUpdated,
		&historyStatus,
		&historyVersion,
		&row.hasHistory,
		&row.outboxAggregateID,
		&row.outboxAggregateVersion,
		&row.outboxEventType,
		&row.outboxStatus,
		&row.outboxPayload,
	); err != nil {
		return scannedRow{}, newExportError(CodeDatabaseUnavailable, err)
	}
	row.PreviousStatus = previous
	row.LastEventID = lastEvent
	row.LastSourceEventID = lastSource
	if sourceUpdated != nil {
		row.SourceUpdatedAt = sourceUpdated.UTC()
	}
	if historyStatus != nil {
		row.historyToStatus = *historyStatus
	}
	if historyVersion != nil {
		row.historyVersion = *historyVersion
	}
	return row, nil
}

func validateAuthoritativeRow(row scannedRow) error {
	if row.TenantID == uuid.Nil || row.ShipmentID == uuid.Nil {
		return newExportError(CodeDatabaseUnavailable, errors.New("zero uuid in authoritative row"))
	}
	if row.CurrentStatus == createdStatus {
		return newExportError(CodeUnsupportedShipmentStatus, nil)
	}
	if !row.hasHistory {
		return newExportError(CodeMissingCanonicalStatusHistory, nil)
	}
	if row.AggregateVersion < 1 {
		return newExportError(CodeMissingAggregateVersion, nil)
	}
	if row.historyVersion != row.AggregateVersion {
		return newExportError(CodeAuthoritativeStatusMismatch, nil)
	}
	if row.historyToStatus != row.CurrentStatus {
		return newExportError(CodeAuthoritativeStatusMismatch, nil)
	}
	if row.SourceUpdatedAt.IsZero() {
		return newExportError(CodeMissingAggregateVersion, nil)
	}
	if row.PreviousStatus != nil && *row.PreviousStatus == createdStatus {
		return newExportError(CodeUnsupportedShipmentStatus, nil)
	}
	if row.LastEventID != nil && row.LastSourceEventID == nil {
		return newExportError(statussnapshot.CodeInconsistentMetadata, nil)
	}
	return validateOutboxConsistency(row)
}

func validateOutboxConsistency(row scannedRow) error {
	hasOutbox := row.outboxAggregateID != nil
	if row.LastEventID == nil && !hasOutbox {
		return nil
	}
	if row.LastEventID != nil && !hasOutbox {
		return newExportError(CodeInconsistentOutboxEventID, nil)
	}
	if !hasOutbox {
		return nil
	}
	if *row.outboxAggregateID != row.ShipmentID {
		return newExportError(CodeOutboxAggregateIDMismatch, nil)
	}
	if row.outboxAggregateVersion == nil || int64(*row.outboxAggregateVersion) != row.AggregateVersion {
		return newExportError(CodeOutboxAggregateVersionMismatch, nil)
	}
	if row.LastSourceEventID == nil || row.LastEventID == nil {
		return newExportError(CodeInconsistentOutboxEventID, nil)
	}
	if row.outboxEventType == nil {
		return newExportError(CodeInconsistentOutboxEventType, nil)
	}
	expectedType := expectedOutboxEventType(row.PreviousStatus, row.CurrentStatus)
	if *row.outboxEventType != expectedType {
		return newExportError(CodeInconsistentOutboxEventType, nil)
	}
	if len(row.outboxPayload) > 0 {
		var envelope domain.ShipmentStatusEventEnvelope
		if err := json.Unmarshal(row.outboxPayload, &envelope); err != nil {
			return newExportError(CodeInconsistentOutboxEventID, err)
		}
		if envelope.EventID != row.LastEventID.String() {
			return newExportError(CodeInconsistentOutboxEventID, nil)
		}
		if envelope.SourceEventID != row.LastSourceEventID.String() {
			return newExportError(CodeInconsistentOutboxEventID, nil)
		}
	}
	// PENDING, PUBLISHED, and FAILED all expose the canonical outbox identity as lastEventId.
	_ = row.outboxStatus
	return nil
}

func expectedOutboxEventType(previousStatus *string, currentStatus string) string {
	history := domain.ShipmentStatusHistory{
		FromStatus: previousStatus,
		ToStatus:   currentStatus,
	}
	return domain.MapOutboxEventType(history)
}

func compareKeys(tenantA, shipmentA, tenantB, shipmentB uuid.UUID) int {
	if tenantA != tenantB {
		if tenantA.String() < tenantB.String() {
			return -1
		}
		return 1
	}
	if shipmentA == shipmentB {
		return 0
	}
	if shipmentA.String() < shipmentB.String() {
		return -1
	}
	return 1
}
