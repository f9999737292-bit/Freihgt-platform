package service

import (
	"context"

	"github.com/freight-platform/company-service/internal/domain"
)

// sourceDispatcher sends each source type to its owning service.
// A shipment credential path cannot confirm an RFx source, and the RFx path cannot confirm a shipment.
type sourceDispatcher struct {
	shipmentRevalidator domain.SourceRevalidator
	shipmentAuthority   domain.SourceCreateAuthority
	rfxRevalidator      domain.SourceRevalidator
	rfxAuthority        domain.SourceCreateAuthority
}

// WireRelationshipSources keeps each owner fail-closed unless that owner's callback URL and the shared revalidation token are both configured.
func WireRelationshipSources(shipmentURL, rfxURL, token string) (domain.SourceRevalidator, domain.SourceCreateAuthority) {
	shipmentRevalidator, shipmentAuthority := WireShipmentSourceAuthority(shipmentURL, token)
	rfxRevalidator, rfxAuthority := WireRfxSourceAuthority(rfxURL, token)
	dispatcher := &sourceDispatcher{
		shipmentRevalidator: shipmentRevalidator,
		shipmentAuthority:   shipmentAuthority,
		rfxRevalidator:      rfxRevalidator,
		rfxAuthority:        rfxAuthority,
	}
	return dispatcher, dispatcher
}

func (d *sourceDispatcher) ConfirmCreate(ctx context.Context, subject domain.SourceCreateSubject) (domain.SourceCreateDecision, error) {
	if d == nil {
		return domain.SourceCreateUnavailable, nil
	}
	switch subject.SourceType {
	case domain.SourceTypeShipment:
		if d.shipmentAuthority == nil {
			return domain.SourceCreateUnavailable, nil
		}
		return d.shipmentAuthority.ConfirmCreate(ctx, subject)
	case domain.SourceTypeRfxAward, domain.SourceTypeAwardLink:
		if d.rfxAuthority == nil {
			return domain.SourceCreateUnavailable, nil
		}
		return d.rfxAuthority.ConfirmCreate(ctx, subject)
	default:
		return domain.SourceCreateUnavailable, nil
	}
}

func (d *sourceDispatcher) Revalidate(ctx context.Context, source domain.SourceSnapshot) (domain.SourceDecision, error) {
	if d == nil {
		return domain.SourceUnavailable, nil
	}
	switch source.SourceType {
	case domain.SourceTypeShipment:
		if d.shipmentRevalidator == nil {
			return domain.SourceUnavailable, nil
		}
		return d.shipmentRevalidator.Revalidate(ctx, source)
	case domain.SourceTypeRfxAward, domain.SourceTypeAwardLink:
		if d.rfxRevalidator == nil {
			return domain.SourceUnavailable, nil
		}
		return d.rfxRevalidator.Revalidate(ctx, source)
	default:
		return domain.SourceUnavailable, nil
	}
}
