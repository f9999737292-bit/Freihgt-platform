package domain

import "github.com/google/uuid"

const (
	ActorKindBuyer   = "BUYER"
	ActorKindCarrier = "CARRIER"
)

// OrderAccessActor carries verified gateway-injected identity for transport order authorization.
type OrderAccessActor struct {
	CompanyID       uuid.UUID
	ActorKind       string
	IsPlatformAdmin bool
}

// CanReadTransportOrder enforces same-tenant company visibility without cross-tenant leakage.
// Shipper and consignee companies may read; selected carrier may read when carrierCompanyID matches.
func CanReadTransportOrder(order *TransportOrder, actor OrderAccessActor, carrierCompanyID *uuid.UUID) bool {
	if actor.IsPlatformAdmin {
		return true
	}
	if actor.CompanyID == order.ShipperCompanyID || actor.CompanyID == order.ConsigneeCompanyID {
		return true
	}
	if carrierCompanyID != nil && actor.CompanyID == *carrierCompanyID {
		return true
	}
	return false
}

// CanMutateTransportOrder restricts draft updates and lifecycle transitions to shipper-side buyers.
func CanMutateTransportOrder(order *TransportOrder, actor OrderAccessActor) bool {
	if actor.IsPlatformAdmin {
		return true
	}
	if actor.ActorKind != ActorKindBuyer {
		return false
	}
	return actor.CompanyID == order.ShipperCompanyID
}
