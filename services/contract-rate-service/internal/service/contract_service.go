package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/contract-rate-service/internal/domain"
	"github.com/freight-platform/contract-rate-service/internal/repository"
	apperrors "github.com/freight-platform/contract-rate-service/internal/platform/errors"
)

type CompanyValidator interface {
	CompanyExistsInTenant(ctx context.Context, tenantID, companyID uuid.UUID) (bool, error)
}

type ContractService struct {
	contracts *repository.ContractRepository
	companies CompanyValidator
}

func NewContractService(contracts *repository.ContractRepository, companies CompanyValidator) *ContractService {
	return &ContractService{contracts: contracts, companies: companies}
}

func (s *ContractService) Create(ctx context.Context, in domain.CreateContractInput, correlationID *string) (*domain.TransportContract, error) {
	if err := in.Actor.RequireBuyerMutation(); err != nil {
		return nil, err
	}
	if err := s.ensureCompanies(ctx, in.TenantID, in.BuyerCompanyID, in.CarrierCompanyID); err != nil {
		return nil, err
	}
	return s.contracts.Create(ctx, in, correlationID)
}

func (s *ContractService) Get(ctx context.Context, tenantID, contractID uuid.UUID, actor domain.ActorInput) (*domain.TransportContract, error) {
	contract, err := s.contracts.GetByIDAndTenant(ctx, tenantID, contractID)
	if err != nil {
		return nil, err
	}
	if err := actor.CanReadContract(contract.BuyerCompanyID, contract.CarrierCompanyID); err != nil {
		return nil, err
	}
	return contract, nil
}

func (s *ContractService) List(ctx context.Context, tenantID uuid.UUID, actor domain.ActorInput) ([]domain.TransportContract, error) {
	var buyerFilter *uuid.UUID
	if !actor.IsPlatformAdmin {
		switch actor.ActorKind {
		case domain.ActorKindBuyer:
			buyerFilter = &actor.ActorCompanyID
		case domain.ActorKindCarrier:
			items, err := s.contracts.ListByTenant(ctx, tenantID, nil)
			if err != nil {
				return nil, err
			}
			filtered := make([]domain.TransportContract, 0, len(items))
			for _, item := range items {
				if item.CarrierCompanyID == actor.ActorCompanyID {
					filtered = append(filtered, item)
				}
			}
			return filtered, nil
		default:
			return nil, apperrors.Forbidden("verified actor context is required", nil)
		}
	}
	return s.contracts.ListByTenant(ctx, tenantID, buyerFilter)
}

func (s *ContractService) UpdateDraft(ctx context.Context, tenantID, contractID uuid.UUID, patch domain.UpdateContractInput, correlationID *string) (*domain.TransportContract, error) {
	if err := patch.Actor.RequireBuyerMutation(); err != nil {
		return nil, err
	}
	return s.contracts.UpdateDraft(ctx, tenantID, contractID, patch, correlationID)
}

func (s *ContractService) PatchMetadata(ctx context.Context, tenantID, contractID uuid.UUID, patch domain.PatchContractMetadataInput, correlationID *string) (*domain.TransportContract, error) {
	if err := patch.Actor.RequireBuyerMutation(); err != nil {
		return nil, err
	}
	return s.contracts.PatchMetadata(ctx, tenantID, contractID, patch, correlationID)
}

func (s *ContractService) Activate(ctx context.Context, tenantID, contractID uuid.UUID, actor domain.ActorInput, correlationID *string) (*domain.TransportContract, error) {
	if err := actor.RequireBuyerMutation(); err != nil {
		return nil, err
	}
	return s.contracts.Activate(ctx, tenantID, contractID, actor, correlationID)
}

func (s *ContractService) Suspend(ctx context.Context, tenantID, contractID uuid.UUID, actor domain.ActorInput, correlationID *string) (*domain.TransportContract, error) {
	if err := actor.RequireBuyerMutation(); err != nil {
		return nil, err
	}
	return s.contracts.Suspend(ctx, tenantID, contractID, actor, correlationID)
}

func (s *ContractService) Reactivate(ctx context.Context, tenantID, contractID uuid.UUID, actor domain.ActorInput, correlationID *string) (*domain.TransportContract, error) {
	if err := actor.RequireBuyerMutation(); err != nil {
		return nil, err
	}
	return s.contracts.Reactivate(ctx, tenantID, contractID, actor, correlationID)
}

func (s *ContractService) Terminate(ctx context.Context, tenantID, contractID uuid.UUID, actor domain.ActorInput, reason *string, correlationID *string) (*domain.TransportContract, error) {
	if err := actor.RequireBuyerMutation(); err != nil {
		return nil, err
	}
	return s.contracts.Terminate(ctx, tenantID, contractID, actor, reason, correlationID)
}

func (s *ContractService) Cancel(ctx context.Context, tenantID, contractID uuid.UUID, actor domain.ActorInput, correlationID *string) (*domain.TransportContract, error) {
	if err := actor.RequireBuyerMutation(); err != nil {
		return nil, err
	}
	return s.contracts.Cancel(ctx, tenantID, contractID, actor, correlationID)
}

func (s *ContractService) ensureCompanies(ctx context.Context, tenantID, buyerID, carrierID uuid.UUID) error {
	ok, err := s.companies.CompanyExistsInTenant(ctx, tenantID, buyerID)
	if err != nil {
		return err
	}
	if !ok {
		return apperrors.Validation("buyer company not found in tenant", map[string]any{"field": "buyer_company_id"})
	}
	ok, err = s.companies.CompanyExistsInTenant(ctx, tenantID, carrierID)
	if err != nil {
		return err
	}
	if !ok {
		return apperrors.Validation("carrier company not found in tenant", map[string]any{"field": "carrier_company_id"})
	}
	return nil
}
