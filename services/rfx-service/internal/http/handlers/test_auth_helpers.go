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

func withBuyerHeaders(req *http.Request) {
	req.Header.Set("X-User-ID", testBuyerUserID)
}
