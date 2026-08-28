package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestCanReadTransportOrderShipperAndConsignee(t *testing.T) {
	t.Parallel()
	shipper := uuid.New()
	consignee := uuid.New()
	order := &TransportOrder{ShipperCompanyID: shipper, ConsigneeCompanyID: consignee}
	if !CanReadTransportOrder(order, OrderAccessActor{CompanyID: shipper, ActorKind: ActorKindBuyer}, nil) {
		t.Fatal("shipper must read")
	}
	if !CanReadTransportOrder(order, OrderAccessActor{CompanyID: consignee, ActorKind: ActorKindBuyer}, nil) {
		t.Fatal("consignee must read")
	}
	if CanReadTransportOrder(order, OrderAccessActor{CompanyID: uuid.New(), ActorKind: ActorKindBuyer}, nil) {
		t.Fatal("foreign company must not read")
	}
}

func TestCanReadTransportOrderCarrierSnapshot(t *testing.T) {
	t.Parallel()
	carrier := uuid.New()
	order := &TransportOrder{ShipperCompanyID: uuid.New(), ConsigneeCompanyID: uuid.New()}
	if !CanReadTransportOrder(order, OrderAccessActor{CompanyID: carrier, ActorKind: ActorKindCarrier}, &carrier) {
		t.Fatal("selected carrier must read")
	}
}

func TestCanMutateTransportOrderShipperOnly(t *testing.T) {
	t.Parallel()
	shipper := uuid.New()
	order := &TransportOrder{ShipperCompanyID: shipper}
	if !CanMutateTransportOrder(order, OrderAccessActor{CompanyID: shipper, ActorKind: ActorKindBuyer}) {
		t.Fatal("shipper buyer must mutate")
	}
	if CanMutateTransportOrder(order, OrderAccessActor{CompanyID: uuid.New(), ActorKind: ActorKindBuyer}) {
		t.Fatal("foreign buyer must not mutate")
	}
	if CanMutateTransportOrder(order, OrderAccessActor{CompanyID: shipper, ActorKind: ActorKindCarrier}) {
		t.Fatal("carrier must not mutate")
	}
}
