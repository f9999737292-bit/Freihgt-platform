package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/payment-service/internal/domain"
	"github.com/freight-platform/payment-service/internal/repository"
	apperrors "github.com/freight-platform/payment-service/internal/platform/errors"
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
	ListObligations(ctx context.Context, tenantID uuid.UUID, actor domain.PaymentActorInput, limit, offset int) ([]domain.PaymentObligation, error)
	ListPayments(ctx context.Context, tenantID uuid.UUID, actor domain.PaymentActorInput, limit, offset int) ([]domain.Payment, error)
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
	if err := domain.ValidatePaymentActor(actor); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	return s.payments.ListPayments(ctx, actor.TenantID, actor, limit, offset)
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
			s.markOutboxPublishedIfNeeded(ctx, actor.TenantID, result.Obligation.ID)
		}
	}
	return outcome, nil
}

func (s *PaymentService) markOutboxPublishedIfNeeded(ctx context.Context, tenantID, obligationID uuid.UUID) {
	if s.outbox == nil {
		return
	}
	_ = s.outbox.MarkPublishedByAggregate(ctx, tenantID, domain.PaymentEventObligationPaid, obligationID, time.Now().UTC())
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
	obligationID := obligation.ID
	s.markOutboxPublishedIfNeeded(ctx, tenantID, obligationID)
	return nil
}
