package repository

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	apperrors "github.com/freight-platform/control-tower-read-model-service/internal/platform/errors"
)

type HandoffRepository struct {
	pool         *pgxpool.Pool
	workItemRepo *WorkItemRepository
	workflowRepo *WorkflowRepository
	riskRepo     *RiskRepository
}

func NewHandoffRepository(
	pool *pgxpool.Pool,
	workItemRepo *WorkItemRepository,
	workflowRepo *WorkflowRepository,
	riskRepo *RiskRepository,
) *HandoffRepository {
	return &HandoffRepository{
		pool: pool, workItemRepo: workItemRepo, workflowRepo: workflowRepo, riskRepo: riskRepo,
	}
}

type CreateHandoffInput struct {
	TenantID   uuid.UUID
	FromUserID uuid.UUID
	ToUserID   uuid.UUID
	Title      *string
	Note       *string
	Items      []domain.BulkActionItem
}

func (r *HandoffRepository) CreateHandoff(ctx context.Context, input CreateHandoffInput) (domain.ShiftHandoff, domain.BulkActionOutcome, error) {
	if input.ToUserID == uuid.Nil {
		return domain.ShiftHandoff{}, domain.BulkActionOutcome{}, apperrors.Validation("toUserId is required", map[string]any{"field": "toUserId"})
	}
	if len(input.Items) == 0 {
		return domain.ShiftHandoff{}, domain.BulkActionOutcome{}, apperrors.Validation("items are required", map[string]any{"field": "items"})
	}
	if len(input.Items) > domain.BulkActionMaxBatch {
		return domain.ShiftHandoff{}, domain.BulkActionOutcome{}, apperrors.Validation("batch size exceeds limit", map[string]any{"max": domain.BulkActionMaxBatch})
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.ShiftHandoff{}, domain.BulkActionOutcome{}, err
	}
	defer tx.Rollback(ctx)

	var handoffID uuid.UUID
	var createdAt time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO control_tower.shift_handoff (tenant_id, from_user_id, to_user_id, title, note)
		VALUES ($1,$2,$3,$4,$5) RETURNING id, created_at
	`, input.TenantID, input.FromUserID, input.ToUserID, input.Title, input.Note).Scan(&handoffID, &createdAt)
	if err != nil {
		return domain.ShiftHandoff{}, domain.BulkActionOutcome{}, err
	}

	outcome := domain.BulkActionOutcome{Requested: len(input.Items), Results: make([]domain.BulkActionResult, 0, len(input.Items))}
	handoffItems := make([]domain.ShiftHandoffItem, 0, len(input.Items))

	for _, item := range input.Items {
		result := domain.BulkActionResult{ItemType: item.ItemType, ItemID: item.ItemID, Success: false}
		var transferErr error
		switch item.ItemType {
		case domain.WorkItemTypeException:
			_, transferErr = r.workflowRepo.AssignCriticalEvent(ctx, domain.AssignCriticalEventInput{
				TenantID: input.TenantID, ActorUserID: input.FromUserID,
				EventID: item.ItemID, AssignedToUser: input.ToUserID,
			})
		case domain.WorkItemTypeRisk:
			_, transferErr = r.riskRepo.AssignRiskOwner(ctx, AssignRiskOwnerInput{
				TenantID: input.TenantID, ActorUserID: input.FromUserID,
				RiskKey: item.ItemID, OwnerUserID: input.ToUserID,
			})
		default:
			msg := "unsupported item type"
			result.Error = &msg
			outcome.Results = append(outcome.Results, result)
			handoffItems = append(handoffItems, domain.ShiftHandoffItem{
				ItemType: item.ItemType, SourceID: item.ItemID, Outcome: domain.HandoffOutcomeFailed, ErrorCode: &msg,
			})
			continue
		}
		if transferErr != nil {
			msg := transferErr.Error()
			result.Error = &msg
			outcome.Failed++
			code := "transfer_failed"
			handoffItems = append(handoffItems, domain.ShiftHandoffItem{
				ItemType: item.ItemType, SourceID: item.ItemID, Outcome: domain.HandoffOutcomeFailed, ErrorCode: &code,
			})
		} else {
			result.Success = true
			outcome.Succeeded++
			handoffItems = append(handoffItems, domain.ShiftHandoffItem{
				ItemType: item.ItemType, SourceID: item.ItemID, Outcome: domain.HandoffOutcomeTransferred,
			})
		}
		outcome.Results = append(outcome.Results, result)
	}

	for _, hi := range handoffItems {
		var errCode *string
		if hi.ErrorCode != nil {
			errCode = hi.ErrorCode
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO control_tower.shift_handoff_item (tenant_id, handoff_id, item_type, source_id, shipment_id, outcome, error_code)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
		`, input.TenantID, handoffID, hi.ItemType, hi.SourceID, hi.ShipmentID, hi.Outcome, errCode)
		if err != nil {
			return domain.ShiftHandoff{}, domain.BulkActionOutcome{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.ShiftHandoff{}, domain.BulkActionOutcome{}, err
	}

	handoff, err := r.GetHandoff(ctx, input.TenantID, handoffID)
	if err != nil {
		return domain.ShiftHandoff{}, outcome, err
	}
	return handoff, outcome, nil
}

type HandoffListFilter struct {
	FromUserID *uuid.UUID
	ToUserID   *uuid.UUID
	Limit      int
}

func (r *HandoffRepository) ListHandoffs(ctx context.Context, tenantID uuid.UUID, filter HandoffListFilter) ([]domain.ShiftHandoff, error) {
	limit := filter.Limit
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	query := `
		SELECT id, tenant_id, from_user_id, to_user_id, title, note, created_at
		FROM control_tower.shift_handoff WHERE tenant_id = $1`
	args := []any{tenantID}
	argN := 2
	if filter.FromUserID != nil {
		query += ` AND from_user_id = $` + itoa(argN)
		args = append(args, *filter.FromUserID)
		argN++
	}
	if filter.ToUserID != nil {
		query += ` AND to_user_id = $` + itoa(argN)
		args = append(args, *filter.ToUserID)
		argN++
	}
	query += ` ORDER BY created_at DESC LIMIT $` + itoa(argN)
	args = append(args, limit)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.ShiftHandoff, 0)
	for rows.Next() {
		var h domain.ShiftHandoff
		if err := rows.Scan(&h.ID, &h.TenantID, &h.FromUserID, &h.ToUserID, &h.Title, &h.Note, &h.CreatedAt); err != nil {
			return nil, err
		}
		h.CreatedAt = h.CreatedAt.UTC()
		items, err := r.loadHandoffItems(ctx, tenantID, h.ID)
		if err != nil {
			return nil, err
		}
		h.Items = items
		out = append(out, h)
	}
	return out, rows.Err()
}

func (r *HandoffRepository) GetHandoff(ctx context.Context, tenantID, handoffID uuid.UUID) (domain.ShiftHandoff, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, from_user_id, to_user_id, title, note, created_at
		FROM control_tower.shift_handoff WHERE tenant_id = $1 AND id = $2
	`, tenantID, handoffID)
	var h domain.ShiftHandoff
	if err := row.Scan(&h.ID, &h.TenantID, &h.FromUserID, &h.ToUserID, &h.Title, &h.Note, &h.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return domain.ShiftHandoff{}, apperrors.NotFound("handoff not found")
		}
		return domain.ShiftHandoff{}, err
	}
	h.CreatedAt = h.CreatedAt.UTC()
	items, err := r.loadHandoffItems(ctx, tenantID, h.ID)
	if err != nil {
		return domain.ShiftHandoff{}, err
	}
	h.Items = items
	return h, nil
}

func (r *HandoffRepository) loadHandoffItems(ctx context.Context, tenantID, handoffID uuid.UUID) ([]domain.ShiftHandoffItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, item_type, source_id, shipment_id, outcome, error_code
		FROM control_tower.shift_handoff_item
		WHERE tenant_id = $1 AND handoff_id = $2 ORDER BY created_at ASC
	`, tenantID, handoffID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.ShiftHandoffItem, 0)
	for rows.Next() {
		var item domain.ShiftHandoffItem
		if err := rows.Scan(&item.ID, &item.ItemType, &item.SourceID, &item.ShipmentID, &item.Outcome, &item.ErrorCode); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
