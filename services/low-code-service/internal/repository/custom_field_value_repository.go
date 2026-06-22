package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/low-code-service/internal/domain"
)

type CustomFieldValueRepository struct {
	pool *pgxpool.Pool
}

func NewCustomFieldValueRepository(pool *pgxpool.Pool) *CustomFieldValueRepository {
	return &CustomFieldValueRepository{pool: pool}
}

func (r *CustomFieldValueRepository) ListByEntity(
	ctx context.Context,
	tenantID uuid.UUID,
	entityType string,
	entityID uuid.UUID,
) ([]domain.CustomFieldValue, error) {
	var items []domain.CustomFieldValue
	err := measureDB("custom_field_value_repository", "list_by_entity", func() error {
		const query = `
			SELECT field_id, field_code, value_json, updated_at
			FROM lowcode.custom_field_values
			WHERE tenant_id = $1 AND entity_type = $2 AND entity_id = $3
			ORDER BY field_code
		`
		rows, err := r.pool.Query(ctx, query, tenantID, entityType, entityID)
		if err != nil {
			return mapDBError(err)
		}
		defer rows.Close()

		items = make([]domain.CustomFieldValue, 0)
		for rows.Next() {
			var item domain.CustomFieldValue
			var valueJSON []byte
			if err := rows.Scan(&item.FieldID, &item.FieldCode, &valueJSON, &item.UpdatedAt); err != nil {
				return mapDBError(err)
			}
			item.ValueJSON = normalizeJSON(valueJSON)
			items = append(items, item)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

type ResolvedCustomFieldValue struct {
	FieldID   uuid.UUID
	FieldCode string
	ValueJSON []byte
}

func (r *CustomFieldValueRepository) UpsertBatch(
	ctx context.Context,
	input domain.UpsertCustomFieldValuesInput,
	values []ResolvedCustomFieldValue,
) (int, error) {
	var saved int
	err := measureDB("custom_field_value_repository", "upsert_batch", func() error {
		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return mapDBError(err)
		}
		defer tx.Rollback(ctx)

		const query = `
			INSERT INTO lowcode.custom_field_values (
				tenant_id, entity_type, entity_id, form_template_id, field_id, field_code, value_json
			) VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (tenant_id, entity_type, entity_id, field_id)
			DO UPDATE SET
				form_template_id = EXCLUDED.form_template_id,
				field_code = EXCLUDED.field_code,
				value_json = EXCLUDED.value_json,
				updated_at = now()
		`

		for _, value := range values {
			if _, err := tx.Exec(ctx, query,
				input.TenantID,
				input.EntityType,
				input.EntityID,
				input.FormTemplateID,
				value.FieldID,
				value.FieldCode,
				value.ValueJSON,
			); err != nil {
				return mapDBError(err)
			}
			saved++
		}

		if err := tx.Commit(ctx); err != nil {
			return mapDBError(err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return saved, nil
}
