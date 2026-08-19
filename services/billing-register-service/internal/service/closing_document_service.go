package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/billing-register-service/internal/domain"
	"github.com/freight-platform/billing-register-service/internal/repository"
	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

type RegisterLookup interface {
	GetByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.BillingRegister, error)
	CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error)
}

type ClosingDocumentStore interface {
	CreatePackage(ctx context.Context, registerID uuid.UUID, in domain.CreateClosingDocumentPackageInput) (*domain.ClosingDocumentPackage, error)
	CreatePackageForActor(ctx context.Context, registerID uuid.UUID, in domain.CreateClosingDocumentPackageInput, actor domain.SettlementActorInput) (*repository.ClosingPackageResult, error)
	CreateInvoice(ctx context.Context, register *domain.BillingRegister, in domain.CreateInvoiceInput, actor domain.SettlementActorInput) (*domain.Invoice, error)
	CreateAct(ctx context.Context, register *domain.BillingRegister, in domain.CreateActInput, actor domain.SettlementActorInput) (*domain.Act, error)
	CreateVATInvoice(ctx context.Context, register *domain.BillingRegister, in domain.CreateVATInvoiceInput, actor domain.SettlementActorInput) (*domain.VATInvoice, error)
	CreateUPD(ctx context.Context, register *domain.BillingRegister, in domain.CreateUPDInput, actor domain.SettlementActorInput) (*domain.UPDDocument, error)
}

type ClosingDocumentService struct {
	registers RegisterLookup
	closing   ClosingDocumentStore
}

func NewClosingDocumentService(registers RegisterLookup, closing ClosingDocumentStore) *ClosingDocumentService {
	return &ClosingDocumentService{registers: registers, closing: closing}
}

func (s *ClosingDocumentService) CreatePackage(ctx context.Context, registerID uuid.UUID, in domain.CreateClosingDocumentPackageInput) (*domain.ClosingDocumentPackage, error) {
	if registerID == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateCreateClosingDocumentPackageInput(in); err != nil {
		return nil, err
	}
	reg, err := s.registers.GetByIDAndTenant(ctx, registerID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateClosingDocumentRegisterStatus(reg.Status); err != nil {
		return nil, err
	}
	return s.closing.CreatePackage(ctx, registerID, in)
}

func (s *ClosingDocumentService) CreatePackageForActor(ctx context.Context, registerID uuid.UUID, in domain.CreateClosingDocumentPackageInput, actor domain.SettlementActorInput) (*repository.ClosingPackageResult, error) {
	if registerID == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateSettlementActor(actor); err != nil {
		return nil, err
	}
	in.TenantID = actor.TenantID
	if err := domain.ValidateCreateClosingDocumentPackageInput(in); err != nil {
		return nil, err
	}
	reg, err := s.registers.GetByIDAndTenant(ctx, registerID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateRegisterBuyerMutation(reg, actor); err != nil {
		return nil, err
	}
	if err := domain.ValidateCreateClosingDocumentRegisterStatus(reg.Status); err != nil {
		return nil, err
	}
	return s.closing.CreatePackageForActor(ctx, registerID, in, actor)
}

func (s *ClosingDocumentService) CreateInvoice(ctx context.Context, registerID uuid.UUID, in domain.CreateInvoiceInput, actor domain.SettlementActorInput) (*domain.Invoice, error) {
	return createClosingDocumentForActor(ctx, s.registers, registerID, actor, func(reg *domain.BillingRegister) error {
		in.TenantID = actor.TenantID
		in.SellerCompanyID = reg.ContractorCompanyID
		in.BuyerCompanyID = reg.CustomerCompanyID
		return domain.ValidateCreateInvoiceInput(in)
	}, func(reg *domain.BillingRegister) (*domain.Invoice, error) {
		return s.closing.CreateInvoice(ctx, reg, in, actor)
	})
}

func (s *ClosingDocumentService) CreateAct(ctx context.Context, registerID uuid.UUID, in domain.CreateActInput, actor domain.SettlementActorInput) (*domain.Act, error) {
	return createClosingDocumentForActor(ctx, s.registers, registerID, actor, func(reg *domain.BillingRegister) error {
		in.TenantID = actor.TenantID
		in.SellerCompanyID = reg.ContractorCompanyID
		in.BuyerCompanyID = reg.CustomerCompanyID
		return domain.ValidateCreateActInput(in)
	}, func(reg *domain.BillingRegister) (*domain.Act, error) {
		return s.closing.CreateAct(ctx, reg, in, actor)
	})
}

func (s *ClosingDocumentService) CreateVATInvoice(ctx context.Context, registerID uuid.UUID, in domain.CreateVATInvoiceInput, actor domain.SettlementActorInput) (*domain.VATInvoice, error) {
	return createClosingDocumentForActor(ctx, s.registers, registerID, actor, func(reg *domain.BillingRegister) error {
		in.TenantID = actor.TenantID
		in.SellerCompanyID = reg.ContractorCompanyID
		in.BuyerCompanyID = reg.CustomerCompanyID
		return domain.ValidateCreateVATInvoiceInput(in)
	}, func(reg *domain.BillingRegister) (*domain.VATInvoice, error) {
		return s.closing.CreateVATInvoice(ctx, reg, in, actor)
	})
}

func (s *ClosingDocumentService) CreateUPD(ctx context.Context, registerID uuid.UUID, in domain.CreateUPDInput, actor domain.SettlementActorInput) (*domain.UPDDocument, error) {
	return createClosingDocumentForActor(ctx, s.registers, registerID, actor, func(reg *domain.BillingRegister) error {
		in.TenantID = actor.TenantID
		in.SellerCompanyID = reg.ContractorCompanyID
		in.BuyerCompanyID = reg.CustomerCompanyID
		return domain.ValidateCreateUPDInput(in)
	}, func(reg *domain.BillingRegister) (*domain.UPDDocument, error) {
		return s.closing.CreateUPD(ctx, reg, in, actor)
	})
}

func createClosingDocumentForActor[T any](
	ctx context.Context,
	registers RegisterLookup,
	registerID uuid.UUID,
	actor domain.SettlementActorInput,
	validateInput func(*domain.BillingRegister) error,
	create func(*domain.BillingRegister) (T, error),
) (T, error) {
	var zero T
	if registerID == uuid.Nil {
		return zero, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateSettlementActor(actor); err != nil {
		return zero, err
	}
	reg, err := registers.GetByIDAndTenant(ctx, registerID, actor.TenantID)
	if err != nil {
		return zero, err
	}
	if err := domain.ValidateRegisterBuyerMutation(reg, actor); err != nil {
		return zero, err
	}
	if err := domain.ValidateCreateClosingDocumentRegisterStatus(reg.Status); err != nil {
		return zero, err
	}
	if err := validateInput(reg); err != nil {
		return zero, err
	}
	return create(reg)
}

func createClosingDocument[T any](
	ctx context.Context,
	registers RegisterLookup,
	registerID uuid.UUID,
	tenantID uuid.UUID,
	validateInput func(*domain.BillingRegister) error,
	create func(*domain.BillingRegister) (T, error),
) (T, error) {
	var zero T
	if registerID == uuid.Nil {
		return zero, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	reg, err := registers.GetByIDAndTenant(ctx, registerID, tenantID)
	if err != nil {
		return zero, err
	}
	if err := domain.ValidateCreateClosingDocumentRegisterStatus(reg.Status); err != nil {
		return zero, err
	}
	if err := validateInput(reg); err != nil {
		return zero, err
	}
	return create(reg)
}

// Legacy methods without actor — kept for unit tests.
func (s *ClosingDocumentService) CreateInvoiceLegacy(ctx context.Context, registerID uuid.UUID, in domain.CreateInvoiceInput) (*domain.Invoice, error) {
	return createClosingDocument(ctx, s.registers, registerID, in.TenantID, func(*domain.BillingRegister) error {
		return domain.ValidateCreateInvoiceInput(in)
	}, func(reg *domain.BillingRegister) (*domain.Invoice, error) {
		return s.closing.CreateInvoice(ctx, reg, in, domain.SettlementActorInput{TenantID: in.TenantID})
	})
}

func (s *ClosingDocumentService) CreateActLegacy(ctx context.Context, registerID uuid.UUID, in domain.CreateActInput) (*domain.Act, error) {
	return createClosingDocument(ctx, s.registers, registerID, in.TenantID, func(*domain.BillingRegister) error {
		return domain.ValidateCreateActInput(in)
	}, func(reg *domain.BillingRegister) (*domain.Act, error) {
		return s.closing.CreateAct(ctx, reg, in, domain.SettlementActorInput{TenantID: in.TenantID})
	})
}

func (s *ClosingDocumentService) CreateVATInvoiceLegacy(ctx context.Context, registerID uuid.UUID, in domain.CreateVATInvoiceInput) (*domain.VATInvoice, error) {
	return createClosingDocument(ctx, s.registers, registerID, in.TenantID, func(*domain.BillingRegister) error {
		return domain.ValidateCreateVATInvoiceInput(in)
	}, func(reg *domain.BillingRegister) (*domain.VATInvoice, error) {
		return s.closing.CreateVATInvoice(ctx, reg, in, domain.SettlementActorInput{TenantID: in.TenantID})
	})
}

func (s *ClosingDocumentService) CreateUPDLegacy(ctx context.Context, registerID uuid.UUID, in domain.CreateUPDInput) (*domain.UPDDocument, error) {
	return createClosingDocument(ctx, s.registers, registerID, in.TenantID, func(*domain.BillingRegister) error {
		return domain.ValidateCreateUPDInput(in)
	}, func(reg *domain.BillingRegister) (*domain.UPDDocument, error) {
		return s.closing.CreateUPD(ctx, reg, in, domain.SettlementActorInput{TenantID: in.TenantID})
	})
}
