package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/payment-service/internal/domain"
	apperrors "github.com/freight-platform/payment-service/internal/platform/errors"
	paymentmetrics "github.com/freight-platform/payment-service/internal/platform/metrics"
	"github.com/freight-platform/payment-service/internal/repository"
)

type RegisterLookup interface {
	GetSnapshot(ctx context.Context, tenantID, registerID uuid.UUID) (*repository.BillingRegisterSnapshot, error)
}

type PaymentStore interface {
	EnsureObligationForBillingRegister(ctx context.Context, tenantID, registerID uuid.UUID, snap *repository.BillingRegisterSnapshot) (*domain.PaymentObligation, error)
	GetObligationByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.PaymentObligation, error)
	GetObligationBySource(ctx context.Context, tenantID uuid.UUID, sourceType string, sourceID uuid.UUID) (*domain.PaymentObligation, error)
	UpdateObligationDueDate(ctx context.Context, tenantID, obligationID uuid.UUID, dueDate *time.Time, actor domain.PaymentActorInput) (*domain.PaymentObligation, error)
	CreateManualPayment(ctx context.Context, in domain.CreateManualPaymentInput) (*domain.Payment, error)
	GetPaymentByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Payment, error)
	ReconcilePayment(ctx context.Context, tenantID, paymentID uuid.UUID, actor domain.PaymentActorInput) (*domain.Payment, error)
	Allocate(ctx context.Context, in domain.CreateAllocationInput) (*repository.AllocateResult, error)
	GetAllocationByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.PaymentAllocation, error)
	VoidAllocation(ctx context.Context, in domain.VoidAllocationInput) (*repository.VoidAllocationResult, error)
	VoidPayment(ctx context.Context, in domain.VoidPaymentInput) (*domain.Payment, error)
	ListObligations(ctx context.Context, tenantID uuid.UUID, actor domain.PaymentActorInput, limit, offset int) ([]domain.PaymentObligation, error)
	ListPayments(ctx context.Context, tenantID uuid.UUID, actor domain.PaymentActorInput, limit, offset int) ([]domain.Payment, error)
	ListPaymentsFiltered(ctx context.Context, tenantID uuid.UUID, actor domain.PaymentActorInput, query domain.PaymentListQuery) (domain.PaymentListResult, error)
	ListAllocationsByPaymentID(ctx context.Context, tenantID, paymentID uuid.UUID, limit, offset int) ([]domain.PaymentAllocation, error)
	ListEligibleObligationsForPayment(ctx context.Context, tenantID uuid.UUID, payment *domain.Payment, actor domain.PaymentActorInput, limit, offset int) ([]domain.PaymentObligation, error)
	ListPaymentAuditEvents(ctx context.Context, tenantID, paymentID uuid.UUID, limit, offset int) ([]domain.PaymentAuditEvent, error)
}

type MembershipStore interface {
	CompanyExistsInTenant(ctx context.Context, tenantID, companyID uuid.UUID) (bool, error)
}

type BillingRegisterSync interface {
	SyncRegisterPaid(ctx context.Context, tenantID, registerID uuid.UUID) error
}

type OutboxProjectionStore interface {
	MarkPublishedByAggregate(ctx context.Context, tenantID uuid.UUID, eventType string, aggregateID uuid.UUID, publishedAt time.Time) error
}

type PaymentService struct {
	payments   PaymentStore
	registers  RegisterLookup
	membership MembershipStore
	billing    BillingRegisterSync
	outbox     OutboxProjectionStore
}

func NewPaymentService(payments PaymentStore, registers RegisterLookup, membership MembershipStore, billing BillingRegisterSync, outbox OutboxProjectionStore) *PaymentService {
	return &PaymentService{payments: payments, registers: registers, membership: membership, billing: billing, outbox: outbox}
}

func (s *PaymentService) EnsurePaymentObligationForBillingRegister(ctx context.Context, tenantID, registerID uuid.UUID) (*domain.PaymentObligation, error) {
	if tenantID == uuid.Nil || registerID == uuid.Nil {
		return nil, apperrors.Validation("tenant_id and register_id are required", nil)
	}
	snap, err := s.registers.GetSnapshot(ctx, tenantID, registerID)
	if err != nil {
		return nil, err
	}
	return s.payments.EnsureObligationForBillingRegister(ctx, tenantID, registerID, snap)
}

func (s *PaymentService) GetObligation(ctx context.Context, id uuid.UUID, actor domain.PaymentActorInput) (*domain.PaymentObligation, error) {
	if err := domain.ValidatePaymentActor(actor); err != nil {
		return nil, err
	}
	o, err := s.payments.GetObligationByID(ctx, actor.TenantID, id)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidatePaymentAccess(o.PayerCompanyID, o.PayeeCompanyID, actor.ActorCompanyID, actor.ActorKind); err != nil {
		return nil, err
	}
	return o, nil
}

func (s *PaymentService) ListObligations(ctx context.Context, actor domain.PaymentActorInput, limit, offset int) ([]domain.PaymentObligation, error) {
	if err := domain.ValidatePaymentActor(actor); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	return s.payments.ListObligations(ctx, actor.TenantID, actor, limit, offset)
}

func (s *PaymentService) UpdateDueDate(ctx context.Context, obligationID uuid.UUID, dueDate *time.Time, actor domain.PaymentActorInput) (*domain.PaymentObligation, error) {
	if obligationID == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidatePaymentActor(actor); err != nil {
		return nil, err
	}
	return s.payments.UpdateObligationDueDate(ctx, actor.TenantID, obligationID, dueDate, actor)
}

func (s *PaymentService) CreateManualPayment(ctx context.Context, in domain.CreateManualPaymentInput, actor domain.PaymentActorInput) (*domain.Payment, error) {
	if err := domain.ValidatePaymentActor(actor); err != nil {
		return nil, err
	}
	if err := domain.ValidateManualPaymentSource(in.Source); err != nil {
		return nil, err
	}
	if err := domain.ValidateCurrencyCode(in.CurrencyCode); err != nil {
		return nil, err
	}
	if err := domain.ValidateMoneyScale(in.Amount, "amount"); err != nil {
		return nil, err
	}
	in.Amount = domain.RoundMoney(in.Amount)
	in.Source = domain.PaymentSourceManual
	in.TenantID = actor.TenantID
	in.CreatedBy = actor.ActorUserID
	if err := domain.ValidatePaymentActorForCreate(in.PayerCompanyID, in.PayeeCompanyID, actor.ActorCompanyID, actor.ActorKind); err != nil {
		return nil, err
	}
	for _, companyID := range []uuid.UUID{in.PayerCompanyID, in.PayeeCompanyID} {
		exists, err := s.membership.CompanyExistsInTenant(ctx, actor.TenantID, companyID)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, apperrors.NotFound("company not found")
		}
	}
	return s.payments.CreateManualPayment(ctx, in)
}

func (s *PaymentService) GetPayment(ctx context.Context, id uuid.UUID, actor domain.PaymentActorInput) (*domain.Payment, error) {
	if err := domain.ValidatePaymentActor(actor); err != nil {
		return nil, err
	}
	p, err := s.payments.GetPaymentByID(ctx, actor.TenantID, id)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidatePaymentAccess(p.PayerCompanyID, p.PayeeCompanyID, actor.ActorCompanyID, actor.ActorKind); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *PaymentService) ListPayments(ctx context.Context, actor domain.PaymentActorInput, limit, offset int) ([]domain.Payment, error) {
	result, err := s.ListPaymentsFiltered(ctx, actor, domain.PaymentListQuery{Limit: limit, Offset: offset})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

func (s *PaymentService) ListPaymentsFiltered(ctx context.Context, actor domain.PaymentActorInput, query domain.PaymentListQuery) (domain.PaymentListResult, error) {
	if err := domain.ValidatePaymentActor(actor); err != nil {
		return domain.PaymentListResult{}, err
	}
	query = domain.NormalizePaymentListQuery(query)
	if err := domain.ValidatePaymentListQuery(query); err != nil {
		return domain.PaymentListResult{}, err
	}
	return s.payments.ListPaymentsFiltered(ctx, actor.TenantID, actor, query)
}

func (s *PaymentService) ListPaymentAllocations(ctx context.Context, paymentID uuid.UUID, actor domain.PaymentActorInput, limit, offset int) ([]domain.PaymentAllocation, error) {
	if paymentID == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidatePaymentActor(actor); err != nil {
		return nil, err
	}
	if _, err := s.GetPayment(ctx, paymentID, actor); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	return s.payments.ListAllocationsByPaymentID(ctx, actor.TenantID, paymentID, limit, offset)
}

func (s *PaymentService) ListEligibleObligationsForPayment(ctx context.Context, paymentID uuid.UUID, actor domain.PaymentActorInput, limit, offset int) ([]domain.PaymentObligation, error) {
	if paymentID == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidatePaymentActor(actor); err != nil {
		return nil, err
	}
	payment, err := s.GetPayment(ctx, paymentID, actor)
	if err != nil {
		return nil, err
	}
	if payment.Status == domain.PaymentStatusReconciled || payment.Status == domain.PaymentStatusVoided {
		return []domain.PaymentObligation{}, nil
	}
	if limit <= 0 {
		limit = 20
	}
	return s.payments.ListEligibleObligationsForPayment(ctx, actor.TenantID, payment, actor, limit, offset)
}

func (s *PaymentService) ListPaymentAuditEvents(ctx context.Context, paymentID uuid.UUID, actor domain.PaymentActorInput, limit, offset int) ([]domain.PaymentAuditEvent, error) {
	if paymentID == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidatePaymentActor(actor); err != nil {
		return nil, err
	}
	if _, err := s.GetPayment(ctx, paymentID, actor); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	return s.payments.ListPaymentAuditEvents(ctx, actor.TenantID, paymentID, limit, offset)
}

func (s *PaymentService) ReconcilePayment(ctx context.Context, paymentID uuid.UUID, actor domain.PaymentActorInput) (*domain.Payment, error) {
	if paymentID == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidatePaymentActor(actor); err != nil {
		return nil, err
	}
	return s.payments.ReconcilePayment(ctx, actor.TenantID, paymentID, actor)
}

func (s *PaymentService) Allocate(ctx context.Context, in domain.CreateAllocationInput, actor domain.PaymentActorInput) (*AllocateOutcome, error) {
	if err := domain.ValidatePaymentActor(actor); err != nil {
		return nil, err
	}
	in.TenantID = actor.TenantID
	in.CreatedBy = actor.ActorUserID
	in.ActorCompanyID = actor.ActorCompanyID
	in.ActorKind = actor.ActorKind
	if err := domain.ValidateMoneyScale(in.AllocatedAmount, "allocated_amount"); err != nil {
		return nil, err
	}
	in.AllocatedAmount = domain.RoundMoney(in.AllocatedAmount)
	result, err := s.payments.Allocate(ctx, in)
	if err != nil {
		return nil, err
	}
	outcome := &AllocateOutcome{Result: result}
	if result.Obligation.Status == domain.ObligationStatusPaid && s.billing != nil {
		if syncErr := s.billing.SyncRegisterPaid(ctx, actor.TenantID, result.Obligation.SourceID); syncErr != nil {
			outcome.RegisterPaidProjection = &RegisterPaidProjection{
				Status:    RegisterPaidProjectionFailed,
				Retryable: true,
				Message:   syncErr.Error(),
			}
		} else {
			outcome.RegisterPaidProjection = &RegisterPaidProjection{Status: RegisterPaidProjectionSynced}
			if markErr := s.markOutboxPublishedIfNeeded(ctx, actor.TenantID, result.Obligation.ID); markErr != nil {
				paymentmetrics.ObserveOutboxMarkPublishedFailed(domain.PaymentEventObligationPaid)
				outcome.OutboxProjection = &OutboxProjection{
					Status:    OutboxProjectionMarkFailed,
					Retryable: true,
					Message:   markErr.Error(),
				}
			}
		}
	}
	return outcome, nil
}

func (s *PaymentService) VoidAllocation(ctx context.Context, allocationID uuid.UUID, reason string, actor domain.PaymentActorInput) (*repository.VoidAllocationResult, error) {
	if allocationID == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidatePaymentActor(actor); err != nil {
		return nil, err
	}
	alloc, err := s.payments.GetAllocationByID(ctx, actor.TenantID, allocationID)
	if err != nil {
		return nil, err
	}
	payment, err := s.payments.GetPaymentByID(ctx, actor.TenantID, alloc.PaymentID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidatePaymentAccess(payment.PayerCompanyID, payment.PayeeCompanyID, actor.ActorCompanyID, actor.ActorKind); err != nil {
		return nil, err
	}
	return s.payments.VoidAllocation(ctx, domain.VoidAllocationInput{
		TenantID: actor.TenantID, AllocationID: allocationID, Reason: reason,
		ActorUserID: actor.ActorUserID, ActorCompanyID: actor.ActorCompanyID, ActorKind: actor.ActorKind,
	})
}

func (s *PaymentService) VoidPayment(ctx context.Context, paymentID uuid.UUID, reason string, actor domain.PaymentActorInput) (*domain.Payment, error) {
	if paymentID == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidatePaymentActor(actor); err != nil {
		return nil, err
	}
	payment, err := s.payments.GetPaymentByID(ctx, actor.TenantID, paymentID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidatePaymentAccess(payment.PayerCompanyID, payment.PayeeCompanyID, actor.ActorCompanyID, actor.ActorKind); err != nil {
		return nil, err
	}
	return s.payments.VoidPayment(ctx, domain.VoidPaymentInput{
		TenantID: actor.TenantID, PaymentID: paymentID, Reason: reason,
		ActorUserID: actor.ActorUserID, ActorCompanyID: actor.ActorCompanyID, ActorKind: actor.ActorKind,
	})
}

func (s *PaymentService) markOutboxPublishedIfNeeded(ctx context.Context, tenantID, obligationID uuid.UUID) error {
	if s.outbox == nil {
		return nil
	}
	return s.outbox.MarkPublishedByAggregate(ctx, tenantID, domain.PaymentEventObligationPaid, obligationID, time.Now().UTC())
}

func (s *PaymentService) EnsureBillingRegisterPaidProjection(ctx context.Context, tenantID, registerID uuid.UUID) error {
	if tenantID == uuid.Nil || registerID == uuid.Nil {
		return apperrors.Validation("tenant_id and register_id are required", nil)
	}
	obligation, err := s.payments.GetObligationBySource(ctx, tenantID, domain.ObligationSourceBillingRegister, registerID)
	if err != nil {
		return err
	}
	if obligation.Status != domain.ObligationStatusPaid {
		return apperrors.Conflict("payment obligation is not PAID", map[string]any{"obligation_status": obligation.Status})
	}
	if !domain.MoneyEqual(obligation.PaidAmount, obligation.OriginalAmount) {
		return apperrors.Conflict("payment obligation paid_amount does not match original_amount", nil)
	}
	if !obligation.OutstandingAmount.IsZero() {
		return apperrors.Conflict("payment obligation outstanding_amount must be zero", nil)
	}
	if s.billing == nil {
		return apperrors.Internal("billing register sync is not configured", nil)
	}
	if err := s.billing.SyncRegisterPaid(ctx, tenantID, registerID); err != nil {
		return err
	}
	if markErr := s.markOutboxPublishedIfNeeded(ctx, tenantID, obligation.ID); markErr != nil {
		paymentmetrics.ObserveOutboxMarkPublishedFailed(domain.PaymentEventObligationPaid)
	}
	return nil
}
