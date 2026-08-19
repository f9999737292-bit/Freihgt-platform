package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/billing-register-service/internal/domain"
	"github.com/freight-platform/billing-register-service/internal/repository"
	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

type BillingRegisterStore interface {
	CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error)
	GetShipmentStatus(ctx context.Context, shipmentID, tenantID uuid.UUID) (string, error)
	Create(ctx context.Context, in domain.CreateBillingRegisterInput) (*domain.BillingRegister, error)
	CreateWithAudit(ctx context.Context, in domain.CreateBillingRegisterInput, actor domain.SettlementActorInput) (*domain.BillingRegister, error)
	GetDetail(ctx context.Context, id uuid.UUID) (*repository.RegisterDetail, error)
	List(ctx context.Context, filter domain.ListBillingRegistersFilter) ([]domain.BillingRegister, int, error)
	GetByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.BillingRegister, error)
	AddItem(ctx context.Context, registerID uuid.UUID, amounts domain.ItemAmounts, in domain.CreateBillingRegisterItemInput) (*domain.BillingRegisterItem, error)
	ListItems(ctx context.Context, registerID, tenantID uuid.UUID) ([]domain.BillingRegisterItem, error)
	DeleteItem(ctx context.Context, registerID, itemID, tenantID uuid.UUID) error
	Calculate(ctx context.Context, id, tenantID uuid.UUID, expectedVersion int) (*domain.BillingRegister, error)
	CalculateForActor(ctx context.Context, registerID uuid.UUID, actor domain.SettlementActorInput) (*domain.BillingRegister, error)
	RecalculateAfterItemChange(ctx context.Context, id, tenantID uuid.UUID, expectedVersion int) (*domain.BillingRegister, error)
	Approve(ctx context.Context, id, tenantID, approvedBy uuid.UUID, expectedVersion int) (*domain.BillingRegister, error)
	ApproveForActor(ctx context.Context, registerID uuid.UUID, actor domain.SettlementActorInput) (*domain.BillingRegister, error)
	UpdateStatus(ctx context.Context, id, tenantID uuid.UUID, status string, expectedVersion int) (*domain.BillingRegister, error)
	TransitionStatusForActor(ctx context.Context, registerID uuid.UUID, actor domain.SettlementActorInput, nextStatus, auditEvent string, validate func(string) error) (*domain.BillingRegister, error)
	SyncPaidFromPaymentObligation(ctx context.Context, registerID, tenantID uuid.UUID) (*domain.BillingRegister, error)
	IncludeSettlement(ctx context.Context, registerID, settlementID uuid.UUID, actor domain.SettlementActorInput) (*repository.IncludeSettlementResult, error)
	RemoveSettlement(ctx context.Context, registerID, settlementID uuid.UUID, actor domain.SettlementActorInput) (*domain.BillingRegister, error)
	GetDetailByTenant(ctx context.Context, id, tenantID uuid.UUID) (*repository.RegisterDetail, error)
	SimulateRegisterAuditFailureForTest(ctx context.Context, registerID, tenantID uuid.UUID) error
	SimulateCalculateAuditFailureForTest(ctx context.Context, registerID, tenantID uuid.UUID) error
}

type PaymentObligationLookup interface {
	ValidateRegisterPaidPreconditions(ctx context.Context, tenantID, registerID uuid.UUID) error
}

type PaymentObligationEnsurer interface {
	EnsurePaymentObligation(ctx context.Context, tenantID, registerID uuid.UUID) error
}

type BillingRegisterService struct {
	registers   BillingRegisterStore
	obligations PaymentObligationLookup
	payments    PaymentObligationEnsurer
}

func NewBillingRegisterService(registers BillingRegisterStore) *BillingRegisterService {
	return &BillingRegisterService{registers: registers}
}

func NewBillingRegisterServiceWithPayments(registers BillingRegisterStore, obligations PaymentObligationLookup, payments PaymentObligationEnsurer) *BillingRegisterService {
	return &BillingRegisterService{registers: registers, obligations: obligations, payments: payments}
}

func (s *BillingRegisterService) Create(ctx context.Context, in domain.CreateBillingRegisterInput) (*domain.BillingRegister, error) {
	in.CurrencyCode = domain.NormalizeCurrencyCode(in.CurrencyCode)
	if err := domain.ValidateCreateBillingRegisterInput(in); err != nil {
		return nil, err
	}
	for _, companyID := range []uuid.UUID{in.CustomerCompanyID, in.ContractorCompanyID} {
		exists, err := s.registers.CompanyExists(ctx, companyID, in.TenantID)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, apperrors.NotFound("company not found")
		}
	}
	return s.registers.Create(ctx, in)
}

func (s *BillingRegisterService) CreateForActor(ctx context.Context, in domain.CreateBillingRegisterInput, actor domain.SettlementActorInput) (*domain.BillingRegister, error) {
	if err := domain.ValidateSettlementActor(actor); err != nil {
		return nil, err
	}
	if actor.ActorKind != domain.SettlementActorBuyer {
		return nil, apperrors.Forbidden("only buyer can create billing register")
	}
	if in.CustomerCompanyID != actor.ActorCompanyID {
		return nil, apperrors.Forbidden("buyer cannot create register for another buyer")
	}
	in.TenantID = actor.TenantID
	in.CurrencyCode = domain.NormalizeCurrencyCode(in.CurrencyCode)
	if err := domain.ValidateCreateBillingRegisterInput(in); err != nil {
		return nil, err
	}
	for _, companyID := range []uuid.UUID{in.CustomerCompanyID, in.ContractorCompanyID} {
		exists, err := s.registers.CompanyExists(ctx, companyID, actor.TenantID)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, apperrors.NotFound("company not found")
		}
	}
	return s.registers.CreateWithAudit(ctx, in, actor)
}

func (s *BillingRegisterService) GetByID(ctx context.Context, id, tenantID uuid.UUID, actor domain.SettlementActorInput) (*repository.RegisterDetail, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateSettlementActor(actor); err != nil {
		return nil, err
	}
	detail, err := s.registers.GetDetailByTenant(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateBillingRegisterAccess(detail.Register, actor.ActorCompanyID, actor.ActorKind); err != nil {
		return nil, err
	}
	return detail, nil
}

func (s *BillingRegisterService) List(ctx context.Context, filter domain.ListBillingRegistersFilter, actor domain.SettlementActorInput) ([]domain.BillingRegister, int, error) {
	if err := domain.ValidateSettlementActor(actor); err != nil {
		return nil, 0, err
	}
	switch actor.ActorKind {
	case domain.SettlementActorBuyer:
		if filter.CustomerCompanyID != nil && *filter.CustomerCompanyID != actor.ActorCompanyID {
			return nil, 0, apperrors.Forbidden("buyer cannot list another buyer's billing registers")
		}
		filter.CustomerCompanyID = &actor.ActorCompanyID
	case domain.SettlementActorCarrier:
		if filter.ContractorCompanyID != nil && *filter.ContractorCompanyID != actor.ActorCompanyID {
			return nil, 0, apperrors.Forbidden("carrier cannot list another carrier's billing registers")
		}
		filter.ContractorCompanyID = &actor.ActorCompanyID
	}
	filter.TenantID = actor.TenantID
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if err := domain.ValidateListBillingRegistersFilter(filter); err != nil {
		return nil, 0, err
	}
	return s.registers.List(ctx, filter)
}

func (s *BillingRegisterService) IncludeSettlement(ctx context.Context, registerID, settlementID uuid.UUID, actor domain.SettlementActorInput) (*repository.IncludeSettlementResult, error) {
	if registerID == uuid.Nil || settlementID == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateSettlementActor(actor); err != nil {
		return nil, err
	}
	return s.registers.IncludeSettlement(ctx, registerID, settlementID, actor)
}

func (s *BillingRegisterService) RemoveSettlement(ctx context.Context, registerID, settlementID uuid.UUID, actor domain.SettlementActorInput) (*domain.BillingRegister, error) {
	if registerID == uuid.Nil || settlementID == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateSettlementActor(actor); err != nil {
		return nil, err
	}
	return s.registers.RemoveSettlement(ctx, registerID, settlementID, actor)
}

func (s *BillingRegisterService) AddItem(ctx context.Context, registerID uuid.UUID, in domain.CreateBillingRegisterItemInput, actor domain.SettlementActorInput) (*domain.BillingRegisterItem, error) {
	if registerID == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateSettlementActor(actor); err != nil {
		return nil, err
	}
	in.TenantID = actor.TenantID
	if err := domain.ValidateCreateBillingRegisterItemInput(in); err != nil {
		return nil, err
	}
	reg, err := s.registers.GetByIDAndTenant(ctx, registerID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateRegisterBuyerMutation(reg, actor); err != nil {
		return nil, err
	}
	if err := domain.ValidateAddItemRegisterStatus(reg.Status); err != nil {
		return nil, err
	}
	status, err := s.registers.GetShipmentStatus(ctx, in.ShipmentID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateShipmentForBilling(status); err != nil {
		return nil, err
	}
	vatRate := in.VATRate
	if vatRate == nil {
		vatRate = reg.VATRate
	}
	amounts := domain.CalculateItemAmounts(in.BaseAmount, in.ExtraCharges, in.Penalties, vatRate)
	return s.registers.AddItem(ctx, registerID, amounts, in)
}

func (s *BillingRegisterService) ListItems(ctx context.Context, registerID uuid.UUID, actor domain.SettlementActorInput) ([]domain.BillingRegisterItem, error) {
	if registerID == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateSettlementActor(actor); err != nil {
		return nil, err
	}
	reg, err := s.registers.GetByIDAndTenant(ctx, registerID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateBillingRegisterAccess(reg, actor.ActorCompanyID, actor.ActorKind); err != nil {
		return nil, err
	}
	return s.registers.ListItems(ctx, registerID, actor.TenantID)
}

func (s *BillingRegisterService) DeleteItem(ctx context.Context, registerID, itemID uuid.UUID, actor domain.SettlementActorInput) (*domain.BillingRegister, error) {
	if registerID == uuid.Nil || itemID == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateSettlementActor(actor); err != nil {
		return nil, err
	}
	reg, err := s.registers.GetByIDAndTenant(ctx, registerID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateRegisterBuyerMutation(reg, actor); err != nil {
		return nil, err
	}
	if err := domain.ValidateDeleteItemRegisterStatus(reg.Status); err != nil {
		return nil, err
	}
	if err := s.registers.DeleteItem(ctx, registerID, itemID, actor.TenantID); err != nil {
		return nil, err
	}
	return s.registers.RecalculateAfterItemChange(ctx, registerID, actor.TenantID, reg.Version)
}

func (s *BillingRegisterService) Calculate(ctx context.Context, registerID uuid.UUID, actor domain.SettlementActorInput) (*domain.BillingRegister, error) {
	if registerID == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateSettlementActor(actor); err != nil {
		return nil, err
	}
	return s.registers.CalculateForActor(ctx, registerID, actor)
}

func (s *BillingRegisterService) Approve(ctx context.Context, registerID uuid.UUID, actor domain.SettlementActorInput) (*domain.BillingRegister, error) {
	if registerID == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateSettlementActor(actor); err != nil {
		return nil, err
	}
	return s.registers.ApproveForActor(ctx, registerID, actor)
}

func (s *BillingRegisterService) MarkSentToEDO(ctx context.Context, registerID uuid.UUID, actor domain.SettlementActorInput) (*domain.BillingRegister, error) {
	return s.transitionForActor(ctx, registerID, actor, domain.ValidateMarkSentToEDOStatus, domain.RegisterStatusSentToEDO, domain.RegisterAuditMarkedSent)
}

func (s *BillingRegisterService) MarkSigned(ctx context.Context, registerID uuid.UUID, actor domain.SettlementActorInput) (*domain.BillingRegister, error) {
	if registerID == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateSettlementActor(actor); err != nil {
		return nil, err
	}
	signedReg, err := s.transitionForActor(ctx, registerID, actor, domain.ValidateMarkSignedStatus, domain.RegisterStatusSignedByCounterparty, domain.RegisterAuditMarkedSigned)
	if err != nil {
		return nil, err
	}
	if s.payments == nil {
		return signedReg, nil
	}
	if ensureErr := s.payments.EnsurePaymentObligation(ctx, actor.TenantID, registerID); ensureErr != nil {
		return signedReg, apperrors.Unavailable("payment obligation ensure failed; register remains signed", ensureErr)
	}
	return signedReg, nil
}

func (s *BillingRegisterService) MarkPaid(ctx context.Context, registerID uuid.UUID, actor domain.SettlementActorInput) (*domain.BillingRegister, error) {
	if registerID == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateSettlementActor(actor); err != nil {
		return nil, err
	}
	if s.obligations != nil {
		if err := s.obligations.ValidateRegisterPaidPreconditions(ctx, actor.TenantID, registerID); err != nil {
			return nil, err
		}
	}
	return s.transitionForActor(ctx, registerID, actor, domain.ValidateMarkPaidStatus, domain.RegisterStatusPaid, domain.RegisterAuditMarkedPaid)
}

func (s *BillingRegisterService) SyncPaidFromObligation(ctx context.Context, registerID, tenantID uuid.UUID) (*domain.BillingRegister, error) {
	if registerID == uuid.Nil || tenantID == uuid.Nil {
		return nil, apperrors.Validation("id and tenant_id are required", nil)
	}
	if s.obligations == nil {
		return nil, apperrors.Internal("payment obligation lookup is not configured", nil)
	}
	if err := s.obligations.ValidateRegisterPaidPreconditions(ctx, tenantID, registerID); err != nil {
		return nil, err
	}
	return s.registers.SyncPaidFromPaymentObligation(ctx, registerID, tenantID)
}

func (s *BillingRegisterService) Close(ctx context.Context, registerID uuid.UUID, actor domain.SettlementActorInput) (*domain.BillingRegister, error) {
	return s.transitionForActor(ctx, registerID, actor, domain.ValidateCloseRegisterStatus, domain.RegisterStatusClosed, domain.RegisterAuditClosed)
}

func (s *BillingRegisterService) transitionForActor(
	ctx context.Context,
	registerID uuid.UUID,
	actor domain.SettlementActorInput,
	validate func(string) error,
	nextStatus, auditEvent string,
) (*domain.BillingRegister, error) {
	if registerID == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateSettlementActor(actor); err != nil {
		return nil, err
	}
	return s.registers.TransitionStatusForActor(ctx, registerID, actor, nextStatus, auditEvent, validate)
}
