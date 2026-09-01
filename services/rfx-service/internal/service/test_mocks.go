package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/rfx-service/internal/domain"
)

type mockMembershipResolver struct {
	kind       domain.ActorKind
	buyerIDs   []uuid.UUID
	carrierIDs []uuid.UUID
	roles      []string
}

func (m *mockMembershipResolver) ResolveActorKind(context.Context, domain.ActorContext) (domain.ActorKind, []uuid.UUID, error) {
	if m.kind == domain.ActorKindUnknown {
		return domain.ActorKindBuyer, m.carrierIDs, nil
	}
	return m.kind, m.carrierIDs, nil
}

func (m *mockMembershipResolver) ListBuyerCompanyIDs(_ context.Context, actor domain.ActorContext) ([]uuid.UUID, error) {
	if len(m.buyerIDs) > 0 {
		return m.buyerIDs, nil
	}
	return []uuid.UUID{actor.TenantID}, nil
}

func (m *mockMembershipResolver) ListUserRoleCodes(context.Context, uuid.UUID, uuid.UUID) ([]string, error) {
	if len(m.roles) > 0 {
		return m.roles, nil
	}
	return []string{"PROCUREMENT_MANAGER"}, nil
}

func buyerTestActor(tenantID, userID, companyID uuid.UUID) domain.ActorContext {
	return domain.ActorContext{TenantID: tenantID, UserID: userID}
}

func buyerMembershipResolver(ownerCompanyID uuid.UUID) *mockMembershipResolver {
	return &mockMembershipResolver{
		kind:     domain.ActorKindBuyer,
		buyerIDs: []uuid.UUID{ownerCompanyID},
	}
}

func buyerMembershipResolverMulti(ownerCompanyIDs ...uuid.UUID) *mockMembershipResolver {
	return &mockMembershipResolver{
		kind:     domain.ActorKindBuyer,
		buyerIDs: ownerCompanyIDs,
	}
}

func carrierTestActor(tenantID, userID uuid.UUID) domain.ActorContext {
	return domain.ActorContext{TenantID: tenantID, UserID: userID}
}

func carrierMembershipResolver(carrierCompanyIDs ...uuid.UUID) *mockMembershipResolver {
	return &mockMembershipResolver{
		kind:       domain.ActorKindCarrier,
		carrierIDs: carrierCompanyIDs,
	}
}

func acceptBidNoop(context.Context, uuid.UUID, uuid.UUID, func(context.Context, pgx.Tx) error) (*domain.Bid, error) {
	return nil, nil
}
