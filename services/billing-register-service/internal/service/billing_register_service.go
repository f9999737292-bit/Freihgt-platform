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
	GetDetail(ctx context.Context, id uuid.UUID) (*repository.RegisterDetail, error)
	List(ctx context.Context, filter domain.ListBillingRegistersFilter) ([]domain.BillingRegister, int, error)
	GetByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.BillingRegister, error)
	AddItem(ctx context.Context, registerID uuid.UUID, amounts domain.ItemAmounts, in domain.CreateBillingRegisterItemInput) (*domain.BillingRegisterItem, error)
	ListItems(ctx context.Context, registerID, tenantID uuid.UUID) ([]domain.BillingRegisterItem, error)
	DeleteItem(ctx context.Context, registerID, itemID, tenantID uuid.UUID) error
	Calculate(ctx context.Context, id, tenantID uuid.UUID, expectedVersion int) (*domain.BillingRegister, error)
	RecalculateAfterItemChange(ctx context.Context, id, tenantID uuid.UUID, expectedVersion int) (*domain.BillingRegister, error)
	Approve(ctx context.Context, id, tenantID, approvedBy uuid.UUID, expectedVersion int) (*domain.BillingRegister, error)
	UpdateStatus(ctx context.Context, id, tenantID uuid.UUID, status string, expectedVersion int) (*domain.BillingRegister, error)
}

type BillingRegisterService struct {
	registers BillingRegisterStore
}

func NewBillingRegisterService(registers BillingRegisterStore) *BillingRegisterService {
	return &BillingRegisterService{registers: registers}
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

func (s *BillingRegisterService) GetByID(ctx context.Context, id uuid.UUID) (*repository.RegisterDetail, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	return s.registers.GetDetail(ctx, id)
}

func (s *BillingRegisterService) List(ctx context.Context, filter domain.ListBillingRegistersFilter) ([]domain.BillingRegister, int, error) {
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if err := domain.ValidateListBillingRegistersFilter(filter); err != nil {
		return nil, 0, err
	}
	return s.registers.List(ctx, filter)
}

func (s *BillingRegisterService) AddItem(ctx context.Context, registerID uuid.UUID, in domain.CreateBillingRegisterItemInput) (*domain.BillingRegisterItem, error) {
	if registerID == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateCreateBillingRegisterItemInput(in); err != nil {
		return nil, err
	}
	reg, err := s.registers.GetByIDAndTenant(ctx, registerID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateAddItemRegisterStatus(reg.Status); err != nil {
		return nil, err
	}
	status, err := s.registers.GetShipmentStatus(ctx, in.ShipmentID, in.TenantID)
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

func (s *BillingRegisterService) ListItems(ctx context.Context, registerID, tenantID uuid.UUID) ([]domain.BillingRegisterItem, error) {
	if registerID == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateTenantActionInput(domain.TenantActionInput{TenantID: tenantID}); err != nil {
		return nil, err
	}
	if _, err := s.registers.GetByIDAndTenant(ctx, registerID, tenantID); err != nil {
		return nil, err
	}
	return s.registers.ListItems(ctx, registerID, tenantID)
}

func (s *BillingRegisterService) DeleteItem(ctx context.Context, registerID, itemID uuid.UUID, tenantID uuid.UUID) (*domain.BillingRegister, error) {
	if registerID == uuid.Nil || itemID == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	reg, err := s.registers.GetByIDAndTenant(ctx, registerID, tenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateDeleteItemRegisterStatus(reg.Status); err != nil {
		return nil, err
	}
	if err := s.registers.DeleteItem(ctx, registerID, itemID, tenantID); err != nil {
		return nil, err
	}
	return s.registers.RecalculateAfterItemChange(ctx, registerID, tenantID, reg.Version)
}

func (s *BillingRegisterService) Calculate(ctx context.Context, registerID uuid.UUID, in domain.TenantActionInput) (*domain.BillingRegister, error) {
	if registerID == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateTenantActionInput(in); err != nil {
		return nil, err
	}
	reg, err := s.registers.GetByIDAndTenant(ctx, registerID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateCalculateRegisterStatus(reg.Status); err != nil {
		return nil, err
	}
	return s.registers.Calculate(ctx, registerID, in.TenantID, reg.Version)
}

func (s *BillingRegisterService) Approve(ctx context.Context, registerID uuid.UUID, in domain.ApproveRegisterInput) (*domain.BillingRegister, error) {
	if registerID == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateApproveRegisterInput(in); err != nil {
		return nil, err
	}
	reg, err := s.registers.GetByIDAndTenant(ctx, registerID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateApproveRegisterStatus(reg.Status); err != nil {
		return nil, err
	}
	if err := domain.ValidateApproveRegisterTotals(reg.TotalWithVAT); err != nil {
		return nil, err
	}
	return s.registers.Approve(ctx, registerID, in.TenantID, in.ApprovedBy, reg.Version)
}

func (s *BillingRegisterService) MarkSentToEDO(ctx context.Context, registerID uuid.UUID, in domain.TenantActionInput) (*domain.BillingRegister, error) {
	return s.transition(ctx, registerID, in, domain.ValidateMarkSentToEDOStatus, domain.RegisterStatusSentToEDO)
}

func (s *BillingRegisterService) MarkSigned(ctx context.Context, registerID uuid.UUID, in domain.TenantActionInput) (*domain.BillingRegister, error) {
	return s.transition(ctx, registerID, in, domain.ValidateMarkSignedStatus, domain.RegisterStatusSignedByCounterparty)
}

func (s *BillingRegisterService) MarkPaid(ctx context.Context, registerID uuid.UUID, in domain.TenantActionInput) (*domain.BillingRegister, error) {
	return s.transition(ctx, registerID, in, domain.ValidateMarkPaidStatus, domain.RegisterStatusPaid)
}

func (s *BillingRegisterService) Close(ctx context.Context, registerID uuid.UUID, in domain.TenantActionInput) (*domain.BillingRegister, error) {
	return s.transition(ctx, registerID, in, domain.ValidateCloseRegisterStatus, domain.RegisterStatusClosed)
}

func (s *BillingRegisterService) transition(
	ctx context.Context,
	registerID uuid.UUID,
	in domain.TenantActionInput,
	validate func(string) error,
	nextStatus string,
) (*domain.BillingRegister, error) {
	if registerID == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateTenantActionInput(in); err != nil {
		return nil, err
	}
	reg, err := s.registers.GetByIDAndTenant(ctx, registerID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if err := validate(reg.Status); err != nil {
		return nil, err
	}
	return s.registers.UpdateStatus(ctx, registerID, in.TenantID, nextStatus, reg.Version)
}
