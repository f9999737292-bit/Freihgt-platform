package service

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/company-service/internal/domain"
	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
)

type scriptedAuthority struct {
	actor   domain.ActorAuthority
	carrier bool
	calls   int
	userID  uuid.UUID
	err     error
}

func (s *scriptedAuthority) LoadRelationshipActor(_ context.Context, _, userID, _ uuid.UUID) (domain.ActorAuthority, error) {
	s.calls++
	s.userID = userID
	if s.err != nil {
		return domain.ActorAuthority{}, s.err
	}
	return s.actor, nil
}

func (s *scriptedAuthority) ActiveCompanyInTenant(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return s.carrier, s.err
}

type scriptedStore struct {
	mu       sync.Mutex
	creates  int
	accepts  int
	gets     int
	rows     map[string]domain.PersistCreateResult
	hashes   map[string][]byte
	pending  map[string]struct{}
	lastHash []byte
}

func newScriptedStore() *scriptedStore {
	return &scriptedStore{rows: map[string]domain.PersistCreateResult{}, hashes: map[string][]byte{}, pending: map[string]struct{}{}}
}

func (s *scriptedStore) CreateRelationshipRequest(_ context.Context, in domain.PersistCreate) (domain.PersistCreateResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.creates++
	s.lastHash = append([]byte(nil), in.TokenHash...)
	key := in.IdempotencyKey
	pendingKey := in.SourceID.String() + in.ShipperCarrierCompanyID.String()
	if existing, ok := s.rows[key]; ok {
		if string(s.hashes[key]) != string(in.RequestHash) {
			return domain.PersistCreateResult{}, apperrors.New(apperrors.CodeRelationshipIdempotencyConflict, "idempotency conflict")
		}
		return domain.PersistCreateResult{RequestID: existing.RequestID, Status: existing.Status, Created: false}, nil
	}
	if _, ok := s.pending[pendingKey]; ok {
		return domain.PersistCreateResult{}, apperrors.New(apperrors.CodeRelationshipRequestPendingConflict, "pending relationship request already exists")
	}
	created := domain.PersistCreateResult{RequestID: uuid.New(), Status: domain.RequestStatusPending, Created: true}
	s.rows[key] = created
	s.hashes[key] = append([]byte(nil), in.RequestHash...)
	s.pending[pendingKey] = struct{}{}
	return created, nil
}

func (s *scriptedStore) AcceptRelationshipRequest(_ context.Context, _ domain.PersistAccept, source domain.SourceRevalidator) (domain.PersistAcceptResult, error) {
	s.mu.Lock()
	s.accepts++
	s.mu.Unlock()
	if source != nil {
		_, _ = source.Revalidate(context.Background(), domain.SourceSnapshot{})
	}
	return domain.PersistAcceptResult{}, apperrors.New(apperrors.CodeRelationshipRequestNotFound, "relationship request not found")
}

func (s *scriptedStore) GetVisibleRelationshipRequest(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.VisibleRequest, error) {
	s.mu.Lock()
	s.gets++
	s.mu.Unlock()
	return domain.VisibleRequest{}, apperrors.New(apperrors.CodeRelationshipRequestNotFound, "relationship request not found")
}

func TestCreateRefusesBeforePersistence(t *testing.T) {
	store := newScriptedStore()
	authority := &scriptedAuthority{actor: domain.ActorAuthority{Active: true, ActiveMembership: true, CompanyRoles: []string{domain.RoleShipperLogist}}, carrier: true}
	svc := NewRelationshipService(store, authority, RelationshipConfig{}, nil)
	principal := domain.Principal{UserID: uuid.New(), TenantID: uuid.New()}
	companyID := uuid.New()
	_, err := svc.Create(context.Background(), validCreate(principal, companyID))
	if codeOf(t, err) != apperrors.CodeRelationshipCommandsDisabled || store.creates != 0 || authority.calls != 0 {
		t.Fatalf("flag off code=%s creates=%d calls=%d", codeOf(t, err), store.creates, authority.calls)
	}

	svc.cfg.CommandsEnabled = true
	_, err = svc.Create(context.Background(), validCreate(principal, companyID))
	if codeOf(t, err) != apperrors.CodeRelationshipStoreUnavailable || authority.calls != 0 {
		t.Fatalf("empty pepper code=%s calls=%d", codeOf(t, err), authority.calls)
	}

	svc.cfg.TokenPepper = "pepper"
	authority.actor.Active = false
	_, err = svc.Create(context.Background(), validCreate(principal, companyID))
	if codeOf(t, err) != apperrors.CodeRelationshipRequestForbidden || store.creates != 0 {
		t.Fatalf("inactive code=%s creates=%d", codeOf(t, err), store.creates)
	}

	authority.actor = domain.ActorAuthority{Active: true, TenantGlobalPlatformAdmin: true}
	_, err = svc.Create(context.Background(), validCreate(principal, companyID))
	if codeOf(t, err) != apperrors.CodeRelationshipRequestForbidden || store.creates != 0 {
		t.Fatal("platform admin without membership created a request")
	}

	authority.actor = domain.ActorAuthority{Active: true, ActiveMembership: true}
	_, err = svc.Accept(context.Background(), AcceptRelationshipInput{Principal: principal, SelectedCompanyID: companyID, IdempotencyKey: "k"})
	if codeOf(t, err) != apperrors.CodeRelationshipAcceptanceForbidden || store.accepts != 0 {
		t.Fatal("ordinary member reached acceptance persistence")
	}
}

func TestCreateIdempotencyAndTenantSelector(t *testing.T) {
	store := newScriptedStore()
	authority := &scriptedAuthority{actor: domain.ActorAuthority{Active: true, ActiveMembership: true, CompanyRoles: []string{domain.RoleShipperAdmin}}, carrier: true}
	svc := NewRelationshipService(store, authority, RelationshipConfig{CommandsEnabled: true, TokenPepper: "pepper"}, nil)
	principal := domain.Principal{UserID: uuid.New(), TenantID: uuid.New()}
	companyID := uuid.New()
	firstIn := validCreate(principal, companyID)
	first, err := svc.Create(context.Background(), firstIn)
	if err != nil || !first.Created || first.AcceptanceToken == "" {
		t.Fatalf("first create: %v %+v", err, first)
	}
	second, err := svc.Create(context.Background(), firstIn)
	if err != nil || second.Created || second.AcceptanceToken != "" || second.RequestID != first.RequestID {
		t.Fatalf("replay: %v %+v", err, second)
	}
	changed := firstIn
	changed.Fingerprint = "changed"
	_, err = svc.Create(context.Background(), changed)
	if codeOf(t, err) != apperrors.CodeRelationshipIdempotencyConflict {
		t.Fatalf("mismatch code=%s", codeOf(t, err))
	}
	otherKey := firstIn
	otherKey.IdempotencyKey = "other-key"
	_, err = svc.Create(context.Background(), otherKey)
	if codeOf(t, err) != apperrors.CodeRelationshipRequestPendingConflict {
		t.Fatalf("pending code=%s", codeOf(t, err))
	}
	foreignTenant := uuid.New()
	foreign := firstIn
	foreign.BodyTenantID = &foreignTenant
	_, err = svc.Create(context.Background(), foreign)
	if codeOf(t, err) != apperrors.CodeTenantContextConflict {
		t.Fatalf("tenant code=%s", codeOf(t, err))
	}
	authority.carrier = false
	outside := firstIn
	outside.IdempotencyKey = "outside"
	outside.SourceID = uuid.New()
	_, err = svc.Create(context.Background(), outside)
	if codeOf(t, err) != apperrors.CodeRelationshipRequestForbidden {
		t.Fatalf("foreign carrier code=%s", codeOf(t, err))
	}
	if authority.userID != principal.UserID {
		t.Fatal("authority used a user other than the verified principal")
	}
}

func TestAcceptUsesFailClosedSourceAfterRoleCheck(t *testing.T) {
	store := newScriptedStore()
	authority := &scriptedAuthority{actor: domain.ActorAuthority{Active: true, ActiveMembership: true, CompanyRoles: []string{domain.RoleCarrierAdmin}}}
	svc := NewRelationshipService(store, authority, RelationshipConfig{CommandsEnabled: true, TokenPepper: "pepper"}, nil)
	raw, err := domain.NewAcceptanceToken()
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Accept(context.Background(), AcceptRelationshipInput{
		Principal: domain.Principal{UserID: uuid.New(), TenantID: uuid.New()}, SelectedCompanyID: uuid.New(),
		AcceptanceToken: domain.EncodeAcceptanceToken(raw), IdempotencyKey: "accept-key",
	})
	if codeOf(t, err) != apperrors.CodeRelationshipRequestNotFound || store.accepts != 1 {
		t.Fatalf("accept code=%s calls=%d", codeOf(t, err), store.accepts)
	}
}

func validCreate(principal domain.Principal, companyID uuid.UUID) CreateRelationshipInput {
	return CreateRelationshipInput{
		Principal: principal, SelectedCompanyID: companyID, SourceType: domain.SourceTypeShipment,
		SourceID: uuid.New(), Fingerprint: "version-1", ShipperCarrierCompanyID: uuid.New(), IdempotencyKey: "same-key",
	}
}

func codeOf(t *testing.T, err error) apperrors.Code {
	t.Helper()
	var appErr *apperrors.AppError
	if err == nil || !asApp(err, &appErr) {
		t.Fatalf("expected app error, got %v", err)
	}
	return appErr.Code
}

func asApp(err error, target **apperrors.AppError) bool {
	for err != nil {
		if app, ok := err.(*apperrors.AppError); ok {
			*target = app
			return true
		}
		break
	}
	return false
}
