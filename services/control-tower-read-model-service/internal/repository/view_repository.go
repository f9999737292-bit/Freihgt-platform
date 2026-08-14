package repository

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	apperrors "github.com/freight-platform/control-tower-read-model-service/internal/platform/errors"
)

type ViewRepository struct {
	pool *pgxpool.Pool
}

func NewViewRepository(pool *pgxpool.Pool) *ViewRepository {
	return &ViewRepository{pool: pool}
}

var allowedViewFilterKeys = map[string]struct{}{
	"itemType": {}, "workflowStatus": {}, "priority": {}, "businessImpact": {},
	"slaStatus": {}, "escalationLevel": {}, "riskLevel": {}, "riskStatus": {},
	"predictedExceptionType": {}, "owner": {}, "assigned": {}, "shipmentReference": {},
	"exceptionCategory": {}, "preset": {},
}

func validateViewFilters(filters map[string]any) error {
	if filters == nil {
		return nil
	}
	for key := range filters {
		if _, ok := allowedViewFilterKeys[key]; !ok {
			return apperrors.Validation("unsupported filter key", map[string]any{"field": key})
		}
	}
	return nil
}

func (r *ViewRepository) ListViews(ctx context.Context, tenantID, userID uuid.UUID) ([]domain.SavedView, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT v.id, v.tenant_id, v.owner_user_id, v.name, v.scope, v.filter_schema_version,
		       v.filters, v.sort, v.is_default, v.created_at, v.updated_at
		FROM control_tower.saved_view v
		WHERE v.tenant_id = $1 AND (v.owner_user_id = $2 OR v.scope = 'shared')
		ORDER BY v.name ASC
	`, tenantID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSavedViews(rows)
}

func (r *ViewRepository) CreateView(ctx context.Context, view domain.SavedView) (domain.SavedView, error) {
	if err := validateViewFilters(view.Filters); err != nil {
		return domain.SavedView{}, err
	}
	if view.Scope != domain.ViewScopePrivate && view.Scope != domain.ViewScopeShared {
		return domain.SavedView{}, apperrors.Validation("invalid scope", map[string]any{"field": "scope"})
	}
	filtersRaw, _ := json.Marshal(view.Filters)
	sortRaw, _ := json.Marshal(view.Sort)
	if view.FilterSchemaVersion == 0 {
		view.FilterSchemaVersion = domain.FilterSchemaVersion
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO control_tower.saved_view
		    (tenant_id, owner_user_id, name, scope, filter_schema_version, filters, sort, is_default)
		VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7::jsonb,$8)
		RETURNING id, tenant_id, owner_user_id, name, scope, filter_schema_version, filters, sort, is_default, created_at, updated_at
	`, view.TenantID, view.OwnerUserID, strings.TrimSpace(view.Name), view.Scope,
		view.FilterSchemaVersion, string(filtersRaw), string(sortRaw), view.IsDefault)
	return scanSavedViewRow(row)
}

func (r *ViewRepository) UpdateView(ctx context.Context, tenantID, userID, viewID uuid.UUID, patch domain.SavedView) (domain.SavedView, error) {
	existing, err := r.getView(ctx, tenantID, viewID)
	if err != nil {
		return domain.SavedView{}, err
	}
	if existing.Scope == domain.ViewScopePrivate && existing.OwnerUserID != userID {
		return domain.SavedView{}, apperrors.Unauthorized("cannot modify private view of another user")
	}
	name := existing.Name
	if strings.TrimSpace(patch.Name) != "" {
		name = strings.TrimSpace(patch.Name)
	}
	scope := existing.Scope
	if patch.Scope != "" {
		scope = patch.Scope
	}
	filters := existing.Filters
	if patch.Filters != nil {
		if err := validateViewFilters(patch.Filters); err != nil {
			return domain.SavedView{}, err
		}
		filters = patch.Filters
	}
	sort := existing.Sort
	if patch.Sort != nil {
		sort = patch.Sort
	}
	isDefault := existing.IsDefault
	if patch.IsDefault {
		isDefault = true
	}
	filtersRaw, _ := json.Marshal(filters)
	sortRaw, _ := json.Marshal(sort)
	row := r.pool.QueryRow(ctx, `
		UPDATE control_tower.saved_view
		SET name = $4, scope = $5, filters = $6::jsonb, sort = $7::jsonb, is_default = $8, updated_at = NOW()
		WHERE tenant_id = $1 AND id = $2 AND owner_user_id = $3
		RETURNING id, tenant_id, owner_user_id, name, scope, filter_schema_version, filters, sort, is_default, created_at, updated_at
	`, tenantID, viewID, existing.OwnerUserID, name, scope, string(filtersRaw), string(sortRaw), isDefault)
	item, err := scanSavedViewRow(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.SavedView{}, apperrors.NotFound("view not found")
		}
		return domain.SavedView{}, err
	}
	if isDefault {
		_ = r.setDefaultView(ctx, tenantID, userID, viewID)
	}
	return item, nil
}

func (r *ViewRepository) DeleteView(ctx context.Context, tenantID, userID, viewID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM control_tower.saved_view WHERE tenant_id = $1 AND id = $2 AND owner_user_id = $3
	`, tenantID, viewID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("view not found")
	}
	return nil
}

func (r *ViewRepository) SetDefaultView(ctx context.Context, tenantID, userID, viewID uuid.UUID) error {
	if _, err := r.getViewForUser(ctx, tenantID, userID, viewID); err != nil {
		return err
	}
	return r.setDefaultView(ctx, tenantID, userID, viewID)
}

func (r *ViewRepository) setDefaultView(ctx context.Context, tenantID, userID, viewID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `
		UPDATE control_tower.saved_view SET is_default = FALSE
		WHERE tenant_id = $1 AND owner_user_id = $2
	`, tenantID, userID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE control_tower.saved_view SET is_default = TRUE
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, viewID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO control_tower.user_workspace_preference (tenant_id, user_id, default_view_id, updated_at)
		VALUES ($1,$2,$3,NOW())
		ON CONFLICT (tenant_id, user_id) DO UPDATE SET default_view_id = EXCLUDED.default_view_id, updated_at = NOW()
	`, tenantID, userID, viewID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *ViewRepository) GetDefaultViewID(ctx context.Context, tenantID, userID uuid.UUID) (*uuid.UUID, error) {
	var viewID *uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT default_view_id FROM control_tower.user_workspace_preference
		WHERE tenant_id = $1 AND user_id = $2
	`, tenantID, userID).Scan(&viewID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return viewID, nil
}

func (r *ViewRepository) getView(ctx context.Context, tenantID, viewID uuid.UUID) (domain.SavedView, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, owner_user_id, name, scope, filter_schema_version, filters, sort, is_default, created_at, updated_at
		FROM control_tower.saved_view WHERE tenant_id = $1 AND id = $2
	`, tenantID, viewID)
	item, err := scanSavedViewRow(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.SavedView{}, apperrors.NotFound("view not found")
		}
		return domain.SavedView{}, err
	}
	return item, nil
}

func (r *ViewRepository) getViewForUser(ctx context.Context, tenantID, userID, viewID uuid.UUID) (domain.SavedView, error) {
	item, err := r.getView(ctx, tenantID, viewID)
	if err != nil {
		return domain.SavedView{}, err
	}
	if item.Scope == domain.ViewScopePrivate && item.OwnerUserID != userID {
		return domain.SavedView{}, apperrors.Unauthorized("view not accessible")
	}
	return item, nil
}

type savedViewRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

func scanSavedViews(rows savedViewRows) ([]domain.SavedView, error) {
	defer rows.Close()
	out := make([]domain.SavedView, 0)
	for rows.Next() {
		var item domain.SavedView
		var filtersRaw, sortRaw []byte
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&item.ID, &item.TenantID, &item.OwnerUserID, &item.Name, &item.Scope,
			&item.FilterSchemaVersion, &filtersRaw, &sortRaw, &item.IsDefault, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		item.Filters = map[string]any{}
		item.Sort = map[string]any{}
		if len(filtersRaw) > 0 {
			_ = json.Unmarshal(filtersRaw, &item.Filters)
		}
		if len(sortRaw) > 0 {
			_ = json.Unmarshal(sortRaw, &item.Sort)
		}
		item.CreatedAt = createdAt.UTC()
		item.UpdatedAt = updatedAt.UTC()
		out = append(out, item)
	}
	return out, rows.Err()
}

func scanSavedViewRow(row pgx.Row) (domain.SavedView, error) {
	var item domain.SavedView
	var filtersRaw, sortRaw []byte
	var createdAt, updatedAt time.Time
	if err := row.Scan(&item.ID, &item.TenantID, &item.OwnerUserID, &item.Name, &item.Scope,
		&item.FilterSchemaVersion, &filtersRaw, &sortRaw, &item.IsDefault, &createdAt, &updatedAt); err != nil {
		return domain.SavedView{}, err
	}
	item.Filters = map[string]any{}
	item.Sort = map[string]any{}
	if len(filtersRaw) > 0 {
		_ = json.Unmarshal(filtersRaw, &item.Filters)
	}
	if len(sortRaw) > 0 {
		_ = json.Unmarshal(sortRaw, &item.Sort)
	}
	item.CreatedAt = createdAt.UTC()
	item.UpdatedAt = updatedAt.UTC()
	return item, nil
}

func FilterFromSavedView(view domain.SavedView) domain.WorkItemFilter {
	filter := domain.WorkItemFilter{Page: 1, Limit: 50}
	if view.Filters == nil {
		return filter
	}
	if v, ok := view.Filters["itemType"].(string); ok {
		filter.ItemType = v
	}
	if v, ok := view.Filters["workflowStatus"].(string); ok {
		filter.WorkflowStatus = v
	}
	if v, ok := view.Filters["priority"].(string); ok {
		filter.Priority = v
	}
	if v, ok := view.Filters["businessImpact"].(string); ok {
		filter.BusinessImpact = v
	}
	if v, ok := view.Filters["slaStatus"].(string); ok {
		filter.SLAStatus = v
	}
	if v, ok := view.Filters["escalationLevel"].(string); ok {
		filter.EscalationLevel = v
	}
	if v, ok := view.Filters["riskLevel"].(string); ok {
		filter.RiskLevel = v
	}
	if v, ok := view.Filters["riskStatus"].(string); ok {
		filter.RiskStatus = v
	}
	if v, ok := view.Filters["predictedExceptionType"].(string); ok {
		filter.PredictedType = v
	}
	if v, ok := view.Filters["exceptionCategory"].(string); ok {
		filter.ExceptionCategory = v
	}
	if v, ok := view.Filters["preset"].(string); ok {
		filter.Preset = v
	}
	if assigned, ok := view.Filters["assigned"].(bool); ok {
		if !assigned {
			filter.UnassignedOnly = true
		}
	}
	if v, ok := view.Filters["shipmentReference"].(string); ok {
		filter.Search = v
	}
	return filter
}

func (r *ViewRepository) EnsureNameUnique(ctx context.Context, tenantID, ownerID uuid.UUID, name string) error {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM control_tower.saved_view WHERE tenant_id = $1 AND owner_user_id = $2 AND name = $3)
	`, tenantID, ownerID, strings.TrimSpace(name)).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		return apperrors.Conflict("view name already exists", map[string]any{"field": "name"})
	}
	return nil
}

func (r *ViewRepository) CreateViewChecked(ctx context.Context, view domain.SavedView) (domain.SavedView, error) {
	if err := r.EnsureNameUnique(ctx, view.TenantID, view.OwnerUserID, view.Name); err != nil {
		return domain.SavedView{}, err
	}
	return r.CreateView(ctx, view)
}

func (r *ViewRepository) DuplicateView(ctx context.Context, tenantID, userID, viewID uuid.UUID, newName string) (domain.SavedView, error) {
	source, err := r.getViewForUser(ctx, tenantID, userID, viewID)
	if err != nil {
		return domain.SavedView{}, err
	}
	copy := domain.SavedView{
		TenantID:            tenantID,
		OwnerUserID:         userID,
		Name:                newName,
		Scope:               domain.ViewScopePrivate,
		FilterSchemaVersion: source.FilterSchemaVersion,
		Filters:             source.Filters,
		Sort:                source.Sort,
	}
	return r.CreateViewChecked(ctx, copy)
}
