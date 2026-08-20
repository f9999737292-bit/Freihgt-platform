package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/payment-service/internal/domain"
)

func (r *PaymentRepository) ListPaymentsFiltered(
	ctx context.Context,
	tenantID uuid.UUID,
	actor domain.PaymentActorInput,
	query domain.PaymentListQuery,
) (domain.PaymentListResult, error) {
	query = domain.NormalizePaymentListQuery(query)
	where, args := paymentListWhereClause(tenantID, actor, query)
	countQuery := `SELECT COUNT(*) FROM billing.payments WHERE ` + where
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return domain.PaymentListResult{}, mapDBError(err)
	}

	listArgs := append(append([]any{}, args...), query.Limit, query.Offset)
	listQuery := fmt.Sprintf(
		`SELECT %s FROM billing.payments WHERE %s ORDER BY created_at DESC, id DESC LIMIT $%d OFFSET $%d`,
		paymentSelectCols, where, len(args)+1, len(args)+2,
	)
	rows, err := r.pool.Query(ctx, listQuery, listArgs...)
	if err != nil {
		return domain.PaymentListResult{}, mapDBError(err)
	}
	defer rows.Close()

	items := make([]domain.Payment, 0)
	for rows.Next() {
		p, err := scanPayment(rows)
		if err != nil {
			return domain.PaymentListResult{}, err
		}
		items = append(items, *p)
	}
	if err := rows.Err(); err != nil {
		return domain.PaymentListResult{}, mapDBError(err)
	}
	return domain.PaymentListResult{
		Items:  items,
		Total:  total,
		Limit:  query.Limit,
		Offset: query.Offset,
	}, nil
}

func paymentListWhereClause(tenantID uuid.UUID, actor domain.PaymentActorInput, query domain.PaymentListQuery) (string, []any) {
	args := []any{tenantID}
	clauses := []string{"tenant_id = $1"}
	switch actor.ActorKind {
	case domain.PaymentActorBuyer:
		args = append(args, actor.ActorCompanyID)
		clauses = append(clauses, fmt.Sprintf("payer_company_id = $%d", len(args)))
	case domain.PaymentActorCarrier:
		args = append(args, actor.ActorCompanyID)
		clauses = append(clauses, fmt.Sprintf("payee_company_id = $%d", len(args)))
	}
	if query.Status != "" {
		args = append(args, query.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if query.CurrencyCode != "" {
		args = append(args, query.CurrencyCode)
		clauses = append(clauses, fmt.Sprintf("currency_code = $%d", len(args)))
	}
	if query.FromDate != nil {
		args = append(args, *query.FromDate)
		clauses = append(clauses, fmt.Sprintf("payment_date >= $%d", len(args)))
	}
	if query.ToDate != nil {
		args = append(args, *query.ToDate)
		clauses = append(clauses, fmt.Sprintf("payment_date <= $%d", len(args)))
	}
	if query.Search != "" {
		pattern := "%" + strings.ReplaceAll(query.Search, "%", "") + "%"
		args = append(args, pattern)
		idx := len(args)
		clauses = append(clauses, fmt.Sprintf(
			"(payment_number ILIKE $%d OR COALESCE(external_id, '') ILIKE $%d OR COALESCE(external_reference, '') ILIKE $%d OR COALESCE(reference, '') ILIKE $%d)",
			idx, idx, idx, idx,
		))
	}
	return strings.Join(clauses, " AND "), args
}

func (r *PaymentRepository) ListAllocationsByPaymentID(
	ctx context.Context,
	tenantID, paymentID uuid.UUID,
	limit, offset int,
) ([]domain.PaymentAllocation, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > domain.MaxPaymentListLimit {
		limit = domain.MaxPaymentListLimit
	}
	if offset < 0 {
		offset = 0
	}
	query := `SELECT ` + allocationSelectCols + `
		FROM billing.payment_allocations
		WHERE tenant_id = $1 AND payment_id = $2
		ORDER BY created_at DESC, id DESC
		LIMIT $3 OFFSET $4`
	rows, err := r.pool.Query(ctx, query, tenantID, paymentID, limit, offset)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	result := make([]domain.PaymentAllocation, 0)
	for rows.Next() {
		alloc, err := scanAllocation(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *alloc)
	}
	return result, rows.Err()
}

func (r *PaymentRepository) ListEligibleObligationsForPayment(
	ctx context.Context,
	tenantID uuid.UUID,
	payment *domain.Payment,
	actor domain.PaymentActorInput,
	limit, offset int,
) ([]domain.PaymentObligation, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > domain.MaxPaymentListLimit {
		limit = domain.MaxPaymentListLimit
	}
	if offset < 0 {
		offset = 0
	}
	query := `SELECT ` + obligationSelectCols + `
		FROM billing.payment_obligations
		WHERE tenant_id = $1
			AND payer_company_id = $2
			AND payee_company_id = $3
			AND currency_code = $4
			AND status IN ($5, $6)
			AND outstanding_amount > 0`
	args := []any{
		tenantID,
		payment.PayerCompanyID,
		payment.PayeeCompanyID,
		payment.CurrencyCode,
		domain.ObligationStatusOpen,
		domain.ObligationStatusPartiallyPaid,
	}
	switch actor.ActorKind {
	case domain.PaymentActorBuyer:
		query += ` AND payer_company_id = $7`
		args = append(args, actor.ActorCompanyID)
	case domain.PaymentActorCarrier:
		query += ` AND payee_company_id = $7`
		args = append(args, actor.ActorCompanyID)
	}
	query += ` ORDER BY created_at DESC LIMIT $8 OFFSET $9`
	args = append(args, limit, offset)
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	result := make([]domain.PaymentObligation, 0)
	for rows.Next() {
		o, err := scanObligation(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *o)
	}
	return result, rows.Err()
}

func (r *PaymentRepository) ListPaymentAuditEvents(
	ctx context.Context,
	tenantID, paymentID uuid.UUID,
	limit, offset int,
) ([]domain.PaymentAuditEvent, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > domain.MaxPaymentListLimit {
		limit = domain.MaxPaymentListLimit
	}
	if offset < 0 {
		offset = 0
	}
	const query = `
		SELECT id, tenant_id, entity_type, entity_id, event_type, actor_user_id, actor_company_id, payload, created_at
		FROM billing.payment_audit_events
		WHERE tenant_id = $1 AND (
			(entity_type = 'PAYMENT' AND entity_id = $2)
			OR (entity_type = 'PAYMENT_ALLOCATION' AND entity_id IN (
				SELECT id FROM billing.payment_allocations WHERE tenant_id = $1 AND payment_id = $2
			))
			OR (entity_type = 'PAYMENT_OBLIGATION' AND entity_id IN (
				SELECT DISTINCT obligation_id FROM billing.payment_allocations WHERE tenant_id = $1 AND payment_id = $2
			))
		)
		ORDER BY created_at DESC, id DESC
		LIMIT $3 OFFSET $4`
	rows, err := r.pool.Query(ctx, query, tenantID, paymentID, limit, offset)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	result := make([]domain.PaymentAuditEvent, 0)
	for rows.Next() {
		event, err := scanPaymentAuditEvent(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, event)
	}
	return result, rows.Err()
}

func scanPaymentAuditEvent(row pgx.Row) (domain.PaymentAuditEvent, error) {
	var event domain.PaymentAuditEvent
	var payloadRaw []byte
	var actorUserID, actorCompanyID *uuid.UUID
	if err := row.Scan(
		&event.ID, &event.TenantID, &event.EntityType, &event.EntityID, &event.EventType,
		&actorUserID, &actorCompanyID, &payloadRaw, &event.CreatedAt,
	); err != nil {
		return domain.PaymentAuditEvent{}, mapDBError(err)
	}
	if actorUserID != nil {
		s := actorUserID.String()
		event.ActorUserID = &s
	}
	if actorCompanyID != nil {
		s := actorCompanyID.String()
		event.ActorCompanyID = &s
	}
	event.Payload = map[string]any{}
	if len(payloadRaw) > 0 {
		_ = json.Unmarshal(payloadRaw, &event.Payload)
	}
	return event, nil
}
