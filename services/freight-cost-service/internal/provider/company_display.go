package provider

import (
	"context"

	"github.com/google/uuid"
)

type CompanyDisplay struct {
	CompanyID uuid.UUID
	LegalName string
	ShortName *string
	Status    string
}

type CompanyDisplayReader interface {
	BatchGetCompanyDisplay(
		ctx context.Context,
		tenantID uuid.UUID,
		companyIDs []uuid.UUID,
	) (map[uuid.UUID]CompanyDisplay, error)
}
