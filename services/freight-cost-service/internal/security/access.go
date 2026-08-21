package security

import (
	"github.com/google/uuid"

	apperrors "github.com/freight-platform/freight-cost-service/internal/platform/errors"
)

type CanonicalCompanyFacts struct {
	BuyerCompanyID   uuid.UUID
	CarrierCompanyID uuid.UUID
}

func AuthorizeCompanyAccess(actor TrustedActor, facts CanonicalCompanyFacts) error {
	switch actor.ActorKind {
	case ActorKindBuyer:
		if facts.BuyerCompanyID != actor.CompanyID {
			return apperrors.Forbidden("buyer cannot access another buyer's transport order cost")
		}
	case ActorKindCarrier:
		if facts.CarrierCompanyID != actor.CompanyID {
			return apperrors.Forbidden("carrier cannot access another carrier's transport order cost")
		}
	default:
		return apperrors.Validation("actor kind must be BUYER or CARRIER", map[string]any{"field": "actor_kind"})
	}
	return nil
}
