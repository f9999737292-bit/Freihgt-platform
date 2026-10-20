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
	actor      domain.ActorAuthority
	carrier    bool
	userActive bool
	calls      int
	userID     uuid.UUID
	err        error
}

func (s *scriptedAuthority) LoadRelationshipActor(_ context.Context, _, userID, _ uuid.UUID) (domain.ActorAuthority, error) {
	s.calls++
	s.userID = userID
	if s.err != nil {
		return domain.ActorAuthority{}, s.err
	}
	return s.actor, nil
}

func (s *scriptedAuthority) ActiveUserInTenant(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return s.userActive, s.err
}

func (s *scriptedAuthority) ActiveCompanyInTenant(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return s.carrier, s.err
}

type scriptedStore struct {
	mu         sync.Mutex
	creates    int
	accepts    int
	gets       int
	rows       map[string]domain.PersistCreateResult
	hashes     map[string][]byte
	pending    map[string]struct{}
	lastHash   []byte
	lastCaller string
	lists      int
}

func newScriptedStore() *scriptedStore {
	return &scriptedStore{rows: map[string]domain.PersistCreateResult{}, hashes: map[string][]byte{}, pending: map[string]struct{}{}}
}

func (s *scriptedStore) CreateRelationshipRequest(_ context.Context, in domain.PersistCreate) (domain.PersistCreateResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.creates++
	s.lastCaller = in.CallerService
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

func (s *scriptedStore) ListSourceRelationshipRequests(context.Context, string, uuid.UUID, uuid.UUID, uuid.UUID) ([]domain.SourceRequestHead, error) {
	s.mu.Lock()
	s.lists++
	s.mu.Unlock()
	return []domain.SourceRequestHead{{
		RequestID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), Status: domain.RequestStatusPending,
		CarrierCompanyID: uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"), RequestingCompanyID: uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"),
	}}, nil
}

func (s *scriptedStore) CancelRelationshipRequest(context.Context, domain.PersistCancel) (domain.PersistCancelResult, error) {
	return domain.PersistCancelResult{}, apperrors.New(apperrors.CodeRelationshipRequestNotFound, "relationship request not found")
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
	svc := NewRelationshipService(store, authority, RelationshipConfig{}, nil, nil)
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
	svc.cfg.ShipmentServiceToken = shipmentTestToken
	svc.cfg.RfxServiceToken = rfxTestToken
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
	svc := NewRelationshipService(store, authority, testRelationshipConfig(), nil, confirmingSource{})
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
	if store.lastCaller != domain.CallerShipmentService {
		t.Fatalf("caller stored as %s", store.lastCaller)
	}
}

func TestAcceptUsesFailClosedSourceAfterRoleCheck(t *testing.T) {
	store := newScriptedStore()
	authority := &scriptedAuthority{actor: domain.ActorAuthority{Active: true, ActiveMembership: true, CompanyRoles: []string{domain.RoleCarrierAdmin}}}
	svc := NewRelationshipService(store, authority, testRelationshipConfig(), nil, nil)
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

const (
	shipmentTestToken = "shipment-relationship-test-token"
	rfxTestToken      = "rfx-relationship-test-token"
)

func testRelationshipConfig() RelationshipConfig {
	return RelationshipConfig{CommandsEnabled: true, TokenPepper: "pepper", ShipmentServiceToken: shipmentTestToken, RfxServiceToken: rfxTestToken}
}

type confirmingSource struct{}

func (confirmingSource) ConfirmCreate(context.Context, domain.SourceCreateSubject) (domain.SourceCreateDecision, error) {
	return domain.SourceCreateConfirmed, nil
}

type scriptedSource struct {
	decision domain.SourceCreateDecision
	calls    int
	last     domain.SourceCreateSubject
}

func (s *scriptedSource) ConfirmCreate(_ context.Context, subject domain.SourceCreateSubject) (domain.SourceCreateDecision, error) {
	s.calls++
	s.last = subject
	return s.decision, nil
}

func validCreate(principal domain.Principal, companyID uuid.UUID) CreateRelationshipInput {
	return CreateRelationshipInput{
		ServiceToken: shipmentTestToken, Principal: principal, SelectedCompanyID: companyID, SourceType: domain.SourceTypeShipment,
		SourceID: uuid.New(), Fingerprint: "version-1", ShipperCarrierCompanyID: uuid.New(), IdempotencyKey: "same-key",
	}
}

func TestCreateSourceAuthorityDeniesBeforePersistence(t *testing.T) {
	store := newScriptedStore()
	authority := &scriptedAuthority{actor: domain.ActorAuthority{Active: true, ActiveMembership: true, CompanyRoles: []string{domain.RoleShipperAdmin}}, carrier: true}
	source := &scriptedSource{decision: domain.SourceCreateConfirmed}
	svc := NewRelationshipService(store, authority, testRelationshipConfig(), nil, source)
	principal := domain.Principal{UserID: uuid.New(), TenantID: uuid.New()}
	companyID := uuid.New()

	missing := validCreate(principal, companyID)
	missing.ServiceToken = ""
	_, err := svc.Create(context.Background(), missing)
	if codeOf(t, err) != apperrors.CodeAuthenticationRequired || store.creates != 0 || source.calls != 0 {
		t.Fatalf("missing credential code=%s creates=%d calls=%d", codeOf(t, err), store.creates, source.calls)
	}
	wrong := validCreate(principal, companyID)
	wrong.ServiceToken = "shared-internal-token"
	_, err = svc.Create(context.Background(), wrong)
	if codeOf(t, err) != apperrors.CodeAuthenticationRequired || store.creates != 0 {
		t.Fatalf("wrong credential code=%s creates=%d", codeOf(t, err), store.creates)
	}

	rfxWithShipment := validCreate(principal, companyID)
	rfxWithShipment.SourceType = domain.SourceTypeRfxAward
	_, err = svc.Create(context.Background(), rfxWithShipment)
	if codeOf(t, err) != apperrors.CodeRelationshipRequestForbidden || store.creates != 0 || source.calls != 0 {
		t.Fatalf("shipment credential for rfx code=%s creates=%d calls=%d", codeOf(t, err), store.creates, source.calls)
	}
	shipmentWithRfx := validCreate(principal, companyID)
	shipmentWithRfx.ServiceToken = rfxTestToken
	_, err = svc.Create(context.Background(), shipmentWithRfx)
	if codeOf(t, err) != apperrors.CodeRelationshipRequestForbidden || store.creates != 0 || source.calls != 0 {
		t.Fatalf("rfx credential for shipment code=%s creates=%d calls=%d", codeOf(t, err), store.creates, source.calls)
	}

	for _, decision := range []domain.SourceCreateDecision{domain.SourceCreateNotFound, domain.SourceCreateTenantMismatch, domain.SourceCreateStale, domain.SourceCreateActorMismatch} {
		source.decision = decision
		_, err = svc.Create(context.Background(), validCreate(principal, companyID))
		if codeOf(t, err) != apperrors.CodeRelationshipRequestForbidden || store.creates != 0 {
			t.Fatalf("decision %s code=%s creates=%d", decision, codeOf(t, err), store.creates)
		}
	}
	if source.last.CallerService != domain.CallerShipmentService {
		t.Fatal("source confirmation saw a caller other than the credential identity")
	}

	source.decision = domain.SourceCreateUnavailable
	_, err = svc.Create(context.Background(), validCreate(principal, companyID))
	if codeOf(t, err) != apperrors.CodeRelationshipSourceUnavailable || store.creates != 0 {
		t.Fatalf("unavailable code=%s creates=%d", codeOf(t, err), store.creates)
	}

	source.decision = domain.SourceCreateConfirmed
	created, err := svc.Create(context.Background(), validCreate(principal, companyID))
	if err != nil || !created.Created || created.AcceptanceToken == "" || store.lastCaller != domain.CallerShipmentService {
		t.Fatalf("confirmed create: %v %+v caller=%s", err, created, store.lastCaller)
	}
}

func TestUnconfirmedSourceDoesNotReservePendingSlot(t *testing.T) {
	store := newScriptedStore()
	authority := &scriptedAuthority{actor: domain.ActorAuthority{Active: true, ActiveMembership: true, CompanyRoles: []string{domain.RoleShipperAdmin}}, carrier: true}
	source := &scriptedSource{decision: domain.SourceCreateNotFound}
	svc := NewRelationshipService(store, authority, testRelationshipConfig(), nil, source)
	principal := domain.Principal{UserID: uuid.New(), TenantID: uuid.New()}
	companyID := uuid.New()
	in := validCreate(principal, companyID)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.Create(context.Background(), in)
		}()
	}
	wg.Wait()
	if store.creates != 0 || len(store.pending) != 0 || len(store.rows) != 0 {
		t.Fatalf("unconfirmed concurrency reserved state creates=%d pending=%d rows=%d", store.creates, len(store.pending), len(store.rows))
	}
}

func TestListAuthorizesBeforeReadingRequests(t *testing.T) {
	store := newScriptedStore()
	authority := &scriptedAuthority{carrier: true}
	svc := NewRelationshipService(store, authority, testRelationshipConfig(), nil, nil)
	companyID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	base := ListSourceRequestsInput{
		ServiceToken: shipmentTestToken, SourceType: domain.SourceTypeShipment, SourceID: uuid.New(),
		SourceTenantID: uuid.New(), RequestingUserID: uuid.New(), RequestingCompanyID: companyID,
	}
	_, err := svc.ListSourceRequests(context.Background(), base)
	if codeOf(t, err) != apperrors.CodeRelationshipRequestForbidden || store.lists != 0 {
		t.Fatalf("inactive user code=%s lists=%d", codeOf(t, err), store.lists)
	}

	authority.userActive = true
	_, err = svc.ListSourceRequests(context.Background(), base)
	if codeOf(t, err) != apperrors.CodeRelationshipRequestNotFound || store.lists != 0 {
		t.Fatalf("non-member code=%s lists=%d", codeOf(t, err), store.lists)
	}

	authority.actor = domain.ActorAuthority{Active: true, ActiveMembership: true}
	_, err = svc.ListSourceRequests(context.Background(), base)
	if codeOf(t, err) != apperrors.CodeRelationshipRequestForbidden || store.lists != 0 {
		t.Fatalf("member without role code=%s lists=%d", codeOf(t, err), store.lists)
	}

	authority.actor.TenantGlobalPlatformAdmin = true
	authority.actor.ActiveMembership = false
	_, err = svc.ListSourceRequests(context.Background(), base)
	if codeOf(t, err) != apperrors.CodeRelationshipRequestNotFound || store.lists != 0 {
		t.Fatalf("platform admin without membership code=%s lists=%d", codeOf(t, err), store.lists)
	}

	rfx := base
	rfx.SourceType = domain.SourceTypeRfxAward
	authority.actor.ActiveMembership = true
	_, err = svc.ListSourceRequests(context.Background(), rfx)
	if codeOf(t, err) != apperrors.CodeRelationshipRequestForbidden || store.lists != 0 {
		t.Fatalf("shipment credential on rfx code=%s lists=%d", codeOf(t, err), store.lists)
	}

	authority.actor = domain.ActorAuthority{Active: true, ActiveMembership: true, CompanyRoles: []string{domain.RoleShipperAdmin}}
	rows, err := svc.ListSourceRequests(context.Background(), base)
	if err != nil || len(rows) != 1 || store.lists != 1 || rows[0].RequestID.String() == "" {
		t.Fatalf("creator list err=%v rows=%d lists=%d", err, len(rows), store.lists)
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
