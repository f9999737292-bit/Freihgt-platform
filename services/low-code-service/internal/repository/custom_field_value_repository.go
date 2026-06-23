package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/low-code-service/internal/domain"
	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
)

type CustomFieldValueRepository struct {
	pool      *pgxpool.Pool
	auditRepo *ConfigurationAuditRepository
}

func NewCustomFieldValueRepository(pool *pgxpool.Pool, auditRepo *ConfigurationAuditRepository) *CustomFieldValueRepository {
	return &CustomFieldValueRepository{pool: pool, auditRepo: auditRepo}
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
			SELECT field_id, field_code, value_json, COALESCE(form_template_id, '00000000-0000-0000-0000-000000000000'::uuid), updated_at
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
			if err := rows.Scan(&item.FieldID, &item.FieldCode, &valueJSON, &item.FormTemplateID, &item.UpdatedAt); err != nil {
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

		fieldCodes := make([]string, 0, len(values))
		for _, value := range values {
			fieldCodes = append(fieldCodes, value.FieldCode)
		}

		var oldValues map[string][]byte
		if r.auditRepo != nil && len(values) > 0 {
			var err error
			oldValues, err = r.loadExistingValuesInTx(ctx, tx, input.TenantID, input.EntityType, input.EntityID, fieldCodes)
			if err != nil {
				return err
			}
		}

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

		if r.auditRepo != nil && saved > 0 {
			auditValues := make([]domain.ResolvedCustomFieldValueForAudit, 0, len(values))
			for _, value := range values {
				auditValues = append(auditValues, domain.ResolvedCustomFieldValueForAudit{
					FieldCode: value.FieldCode,
					ValueJSON: value.ValueJSON,
				})
			}

			oldJSON, newJSON, _, err := domain.BuildCustomFieldValuesAuditPayload(input.FormTemplateID, auditValues, oldValues)
			if err != nil {
				return apperrors.Internal("failed to build audit payload", err)
			}

			configID := input.FormTemplateID
			if err := r.auditRepo.InsertInTx(ctx, tx, domain.ConfigurationAuditEntry{
				TenantID:        input.TenantID,
				ConfigurationID: &configID,
				EntityType:      input.EntityType,
				EntityID:        input.EntityID,
				Action:          domain.AuditDBActionUpdate,
				OldValueJSON:    oldJSON,
				NewValueJSON:    newJSON,
				ChangedByUserID: input.Audit.ChangedByUserID,
				RequestID:       input.Audit.RequestID,
				IPAddress:       input.Audit.IPAddress,
				UserAgent:       input.Audit.UserAgent,
			}); err != nil {
				return err
			}
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

func (r *CustomFieldValueRepository) ReplaceFieldCodesBatch(
	ctx context.Context,
	input domain.UpsertCustomFieldValuesInput,
	fieldCodes []string,
	values []ResolvedCustomFieldValue,
) (int, error) {
	var saved int
	err := measureDB("custom_field_value_repository", "replace_field_codes_batch", func() error {
		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return mapDBError(err)
		}
		defer tx.Rollback(ctx)

		if len(fieldCodes) > 0 {
			const deleteQuery = `
				DELETE FROM lowcode.custom_field_values
				WHERE tenant_id = $1 AND entity_type = $2 AND entity_id = $3 AND field_code = ANY($4)
			`
			if _, err := tx.Exec(ctx, deleteQuery, input.TenantID, input.EntityType, input.EntityID, fieldCodes); err != nil {
				return mapDBError(err)
			}
		}

		const query = `
			INSERT INTO lowcode.custom_field_values (
				tenant_id, entity_type, entity_id, form_template_id, field_id, field_code, value_json
			) VALUES ($1, $2, $3, $4, $5, $6, $7)
		`

		var oldValues map[string][]byte
		if r.auditRepo != nil && len(values) > 0 {
			oldValues = make(map[string][]byte, len(fieldCodes))
		}

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

		if r.auditRepo != nil && saved > 0 {
			if input.MigrationAudit != nil {
				batchCtx := &domain.BatchMigrationAuditContext{
					BatchID:      input.MigrationAudit.BatchID,
					EntityType:   input.EntityType,
					EntityID:     input.EntityID,
					TemplateCode: input.MigrationAudit.TemplateCode,
					SkipBlocked:  input.MigrationAudit.SkipBlocked,
				}
				migrationStatus := "migrated"
				if input.MigrationAudit.PreviewItem.Status == domain.MigrationPreviewStatusWarning {
					migrationStatus = "migrated_with_warnings"
				}
				execCtx := &domain.MigrationExecutionAuditContext{
					MigrationStatus: migrationStatus,
					MigratedCount:   saved,
					SkippedCount:    len(input.MigrationAudit.PreviewItem.LegacyFields),
				}
				oldJSON, newJSON, err := domain.BuildCustomFieldValuesMigratedToActiveAuditPayload(
					input.MigrationAudit.SourceTemplateID,
					input.FormTemplateID,
					input.MigrationAudit.PreviewItem,
					input.MigrationAudit.AllowWarnings,
					batchCtx,
					execCtx,
				)
				if err != nil {
					return apperrors.Internal("failed to build migration audit payload", err)
				}
				configID := input.FormTemplateID
				if err := r.auditRepo.InsertInTx(ctx, tx, domain.ConfigurationAuditEntry{
					TenantID:        input.TenantID,
					ConfigurationID: &configID,
					EntityType:      input.EntityType,
					EntityID:        input.EntityID,
					Action:          domain.AuditDBActionUpdate,
					OldValueJSON:    oldJSON,
					NewValueJSON:    newJSON,
					ChangedByUserID: input.Audit.ChangedByUserID,
					RequestID:       input.Audit.RequestID,
					IPAddress:       input.Audit.IPAddress,
					UserAgent:       input.Audit.UserAgent,
				}); err != nil {
					return err
				}
			} else {
				auditValues := make([]domain.ResolvedCustomFieldValueForAudit, 0, len(values))
				for _, value := range values {
					auditValues = append(auditValues, domain.ResolvedCustomFieldValueForAudit{
						FieldCode: value.FieldCode,
						ValueJSON: value.ValueJSON,
					})
				}

				oldJSON, newJSON, _, err := domain.BuildCustomFieldValuesAuditPayload(input.FormTemplateID, auditValues, oldValues)
				if err != nil {
					return apperrors.Internal("failed to build audit payload", err)
				}

				configID := input.FormTemplateID
				if err := r.auditRepo.InsertInTx(ctx, tx, domain.ConfigurationAuditEntry{
					TenantID:        input.TenantID,
					ConfigurationID: &configID,
					EntityType:      input.EntityType,
					EntityID:        input.EntityID,
					Action:          domain.AuditDBActionUpdate,
					OldValueJSON:    oldJSON,
					NewValueJSON:    newJSON,
					ChangedByUserID: input.Audit.ChangedByUserID,
					RequestID:       input.Audit.RequestID,
					IPAddress:       input.Audit.IPAddress,
					UserAgent:       input.Audit.UserAgent,
				}); err != nil {
					return err
				}
			}
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

func (r *CustomFieldValueRepository) loadExistingValuesInTx(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	entityType string,
	entityID uuid.UUID,
	fieldCodes []string,
) (map[string][]byte, error) {
	if len(fieldCodes) == 0 {
		return map[string][]byte{}, nil
	}

	const query = `
		SELECT field_code, value_json
		FROM lowcode.custom_field_values
		WHERE tenant_id = $1 AND entity_type = $2 AND entity_id = $3 AND field_code = ANY($4)
	`
	rows, err := tx.Query(ctx, query, tenantID, entityType, entityID, fieldCodes)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	result := make(map[string][]byte, len(fieldCodes))
	for rows.Next() {
		var code string
		var valueJSON []byte
		if err := rows.Scan(&code, &valueJSON); err != nil {
			return nil, mapDBError(err)
		}
		result[code] = valueJSON
	}
	if err := rows.Err(); err != nil {
		return nil, mapDBError(err)
	}
	return result, nil
}
