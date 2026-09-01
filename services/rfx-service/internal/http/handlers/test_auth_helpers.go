package handlers

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
)

const testBuyerUserID = "44444444-4444-4444-4444-444444444444"
const testBuyerShipperCompanyID = "cccccccc-cccc-cccc-cccc-cccccccccccc"

type testBuyerMembershipResolver struct {
	shipperID uuid.UUID
}

func (m testBuyerMembershipResolver) ResolveActorKind(context.Context, domain.ActorContext) (domain.ActorKind, []uuid.UUID, error) {
	return domain.ActorKindBuyer, nil, nil
}

func (m testBuyerMembershipResolver) ListBuyerCompanyIDs(context.Context, domain.ActorContext) ([]uuid.UUID, error) {
	return []uuid.UUID{m.shipperID}, nil
}

func (m testBuyerMembershipResolver) ListUserRoleCodes(context.Context, uuid.UUID, uuid.UUID) ([]string, error) {
	return []string{"PROCUREMENT_MANAGER"}, nil
}

func defaultBuyerMembershipResolver() testBuyerMembershipResolver {
	return testBuyerMembershipResolver{shipperID: uuid.MustParse(testBuyerShipperCompanyID)}
}

const testCarrierUserID = "55555555-5555-5555-5555-555555555555"
const testCarrierCompanyIDHeader = "dddddddd-dddd-dddd-dddd-dddddddddddd"

type testCarrierMembershipResolver struct {
	carrierID uuid.UUID
}

func (m testCarrierMembershipResolver) ResolveActorKind(context.Context, domain.ActorContext) (domain.ActorKind, []uuid.UUID, error) {
	return domain.ActorKindCarrier, []uuid.UUID{m.carrierID}, nil
}

func (m testCarrierMembershipResolver) ListBuyerCompanyIDs(context.Context, domain.ActorContext) ([]uuid.UUID, error) {
	return nil, nil
}

func (m testCarrierMembershipResolver) ListUserRoleCodes(context.Context, uuid.UUID, uuid.UUID) ([]string, error) {
	return []string{"CARRIER_DISPATCHER"}, nil
}

func defaultCarrierMembershipResolver() testCarrierMembershipResolver {
	return testCarrierMembershipResolver{carrierID: uuid.MustParse(testCarrierCompanyIDHeader)}
}

func withBuyerHeaders(req *http.Request) {
	req.Header.Set("X-User-ID", testBuyerUserID)
}

func withCarrierHeaders(req *http.Request) {
	req.Header.Set("X-User-ID", testCarrierUserID)
}
