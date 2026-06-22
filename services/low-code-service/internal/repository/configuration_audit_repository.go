package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/low-code-service/internal/domain"
)

type ConfigurationAuditRepository struct {
	pool *pgxpool.Pool
}

func NewConfigurationAuditRepository(pool *pgxpool.Pool) *ConfigurationAuditRepository {
	return &ConfigurationAuditRepository{pool: pool}
}

func (r *ConfigurationAuditRepository) InsertInTx(
	ctx context.Context,
	tx pgx.Tx,
	entry domain.ConfigurationAuditEntry,
) error {
	const query = `
		INSERT INTO lowcode.configuration_audit_log (
			tenant_id, configuration_id, entity_type, entity_id, action,
			old_value_json, new_value_json, changed_by_user_id, request_id, ip_address, user_agent
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := tx.Exec(ctx, query,
		entry.TenantID,
		entry.ConfigurationID,
		entry.EntityType,
		entry.EntityID,
		entry.Action,
		entry.OldValueJSON,
		entry.NewValueJSON,
		entry.ChangedByUserID,
		nullIfEmpty(entry.RequestID),
		nullIfEmpty(entry.IPAddress),
		nullIfEmpty(entry.UserAgent),
	)
	return mapDBError(err)
}

func (r *ConfigurationAuditRepository) List(
	ctx context.Context,
	filter domain.ListAuditEventsFilter,
) ([]domain.ConfigurationAuditEntry, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	var items []domain.ConfigurationAuditEntry
	err := measureDB("configuration_audit_repository", "list", func() error {
		const query = `
			SELECT id, tenant_id, configuration_id, entity_type, entity_id, action,
			       old_value_json, new_value_json, changed_by_user_id, request_id, changed_at
			FROM lowcode.configuration_audit_log
			WHERE tenant_id = $1
			  AND ($2 = '' OR entity_type = $2)
			  AND ($3::uuid IS NULL OR entity_id = $3)
			  AND (
			    $4 = ''
			    OR new_value_json->>'event_kind' = $4
			  )
			ORDER BY changed_at DESC
			LIMIT $5
		`

		var entityID any
		if filter.EntityID != nil {
			entityID = *filter.EntityID
		}

		rows, err := r.pool.Query(ctx, query,
			filter.TenantID,
			filter.EntityType,
			entityID,
			filter.Action,
			limit,
		)
		if err != nil {
			return mapDBError(err)
		}
		defer rows.Close()

		items = make([]domain.ConfigurationAuditEntry, 0)
		for rows.Next() {
			var item domain.ConfigurationAuditEntry
			var configurationID *uuid.UUID
			var changedBy *uuid.UUID
			var oldJSON []byte
			var newJSON []byte
			if err := rows.Scan(
				&item.ID,
				&item.TenantID,
				&configurationID,
				&item.EntityType,
				&item.EntityID,
				&item.Action,
				&oldJSON,
				&newJSON,
				&changedBy,
				&item.RequestID,
				&item.ChangedAt,
			); err != nil {
				return mapDBError(err)
			}
			item.ConfigurationID = configurationID
			item.ChangedByUserID = changedBy
			item.OldValueJSON = normalizeJSON(oldJSON)
			item.NewValueJSON = normalizeJSON(newJSON)
			items = append(items, item)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
