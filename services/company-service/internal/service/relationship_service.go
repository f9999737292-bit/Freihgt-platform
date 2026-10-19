package service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/company-service/internal/domain"
	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
)

type RelationshipStore interface {
	CreateRelationshipRequest(ctx context.Context, in domain.PersistCreate) (domain.PersistCreateResult, error)
	AcceptRelationshipRequest(ctx context.Context, in domain.PersistAccept, source domain.SourceRevalidator) (domain.PersistAcceptResult, error)
	GetVisibleRelationshipRequest(ctx context.Context, tenantID, companyID, requestID uuid.UUID) (domain.VisibleRequest, error)
	ListSourceRelationshipRequests(ctx context.Context, sourceType string, sourceID, sourceTenantID, requestingCompanyID uuid.UUID) ([]domain.SourceRequestHead, error)
	CancelRelationshipRequest(ctx context.Context, in domain.PersistCancel) (domain.PersistCancelResult, error)
}

type AuthorityReader interface {
	LoadRelationshipActor(ctx context.Context, tenantID, userID, companyID uuid.UUID) (domain.ActorAuthority, error)
	ActiveUserInTenant(ctx context.Context, tenantID, userID uuid.UUID) (bool, error)
	ActiveCompanyInTenant(ctx context.Context, tenantID, companyID uuid.UUID) (bool, error)
}

type RelationshipConfig struct {
	CommandsEnabled      bool
	TokenPepper          string
	ShipmentServiceToken string
	RfxServiceToken      string
}

type RelationshipService struct {
	store     RelationshipStore
	authority AuthorityReader
	cfg       RelationshipConfig
	sources   domain.SourceRevalidator
	source    domain.SourceCreateAuthority
}

func NewRelationshipService(store RelationshipStore, authority AuthorityReader, cfg RelationshipConfig, sources domain.SourceRevalidator, source domain.SourceCreateAuthority) *RelationshipService {
	if sources == nil {
		sources = FailClosedSourceRevalidator{}
	}
	if source == nil {
		source = FailClosedSourceAuthority{}
	}
	return &RelationshipService{store: store, authority: authority, cfg: cfg, sources: sources, source: source}
}

type FailClosedSourceRevalidator struct{}

func (FailClosedSourceRevalidator) Revalidate(context.Context, domain.SourceSnapshot) (domain.SourceDecision, error) {
	return domain.SourceUnavailable, nil
}

type CreateRelationshipInput struct {
	ServiceToken            string
	Principal               domain.Principal
	SelectedCompanyID       uuid.UUID
	SourceType              string
	SourceID                uuid.UUID
	Fingerprint             string
	ShipperCarrierCompanyID uuid.UUID
	IdempotencyKey          string
	BodyTenantID            *uuid.UUID
	BodyCompanyID           *uuid.UUID
	CorrelationID           string
}

type CreateRelationshipOutput struct {
	RequestID       uuid.UUID
	Status          string
	AcceptanceToken string
	Created         bool
}

type AcceptRelationshipInput struct {
	Principal         domain.Principal
	SelectedCompanyID uuid.UUID
	AcceptanceToken   string
	IdempotencyKey    string
	RequestID         *uuid.UUID
	LocatorInvalid    bool
	BodyTenantID      *uuid.UUID
	BodyCompanyID     *uuid.UUID
	CorrelationID     string
}

type AcceptRelationshipOutput struct {
	RelationshipID      uuid.UUID
	RelationshipVersion int
	RequestStatus       string
	Created             bool
}

func (s *RelationshipService) Create(ctx context.Context, in CreateRelationshipInput) (CreateRelationshipOutput, error) {
	if err := s.commandsOpen(); err != nil {
		return CreateRelationshipOutput{}, err
	}
	if strings.TrimSpace(s.cfg.TokenPepper) == "" {
		return CreateRelationshipOutput{}, apperrors.New(apperrors.CodeRelationshipStoreUnavailable, "relationship store unavailable")
	}
	caller, ok := identifyRelationshipCaller(s.cfg.ShipmentServiceToken, s.cfg.RfxServiceToken, in.ServiceToken)
	if !ok {
		return CreateRelationshipOutput{}, apperrors.New(apperrors.CodeAuthenticationRequired, "authentication required")
	}
	if !domain.CallerAllowsSource(caller, strings.TrimSpace(in.SourceType)) {
		return CreateRelationshipOutput{}, refuseCreate()
	}
	if in.BodyTenantID != nil && *in.BodyTenantID != in.Principal.TenantID {
		return CreateRelationshipOutput{}, apperrors.New(apperrors.CodeTenantContextConflict, "tenant context conflict")
	}
	if in.BodyCompanyID != nil && *in.BodyCompanyID != in.SelectedCompanyID {
		return CreateRelationshipOutput{}, refuseCreate()
	}
	if err := s.requireCreateActor(ctx, in.Principal, in.SelectedCompanyID, in.SourceType); err != nil {
		return CreateRelationshipOutput{}, err
	}
	fingerprint := strings.TrimSpace(in.Fingerprint)
	key := strings.TrimSpace(in.IdempotencyKey)
	if !domain.ValidSourceType(in.SourceType) || in.SourceID == uuid.Nil || fingerprint == "" || len(fingerprint) > 2000 || key == "" || len(key) > 200 || in.ShipperCarrierCompanyID == uuid.Nil || in.ShipperCarrierCompanyID == in.SelectedCompanyID {
		return CreateRelationshipOutput{}, apperrors.Validation("relationship request is invalid", nil)
	}
	carrierInTenant, err := s.authority.ActiveCompanyInTenant(ctx, in.Principal.TenantID, in.ShipperCarrierCompanyID)
	if err != nil {
		return CreateRelationshipOutput{}, err
	}
	if !carrierInTenant {
		return CreateRelationshipOutput{}, refuseCreate()
	}
	decision, err := s.source.ConfirmCreate(ctx, domain.SourceCreateSubject{
		CallerService: caller, SourceType: in.SourceType, SourceID: in.SourceID, SourceTenantID: in.Principal.TenantID,
		Fingerprint: fingerprint, ShipperCarrierCompanyID: in.ShipperCarrierCompanyID,
		RequestingCompanyID: in.SelectedCompanyID, RequestingUserID: in.Principal.UserID,
	})
	if err != nil || decision == domain.SourceCreateUnavailable {
		return CreateRelationshipOutput{}, apperrors.New(apperrors.CodeRelationshipSourceUnavailable, "relationship source unavailable")
	}
	if decision != domain.SourceCreateConfirmed {
		return CreateRelationshipOutput{}, refuseCreate()
	}
	rawToken, err := domain.NewAcceptanceToken()
	if err != nil {
		return CreateRelationshipOutput{}, apperrors.New(apperrors.CodeRelationshipStoreUnavailable, "relationship store unavailable")
	}
	stored, err := s.store.CreateRelationshipRequest(ctx, domain.PersistCreate{
		CallerService:           caller,
		SourceType:              in.SourceType,
		SourceID:                in.SourceID,
		SourceTenantID:          in.Principal.TenantID,
		Fingerprint:             fingerprint,
		ShipperCarrierCompanyID: in.ShipperCarrierCompanyID,
		RequestingCompanyID:     in.SelectedCompanyID,
		RequestingUserID:        in.Principal.UserID,
		IdempotencyKey:          key,
		RequestHash:             domain.CreateRequestHash(in.SourceType, in.SourceID, fingerprint, in.ShipperCarrierCompanyID),
		TokenHash:               domain.FingerprintAcceptanceToken(s.cfg.TokenPepper, rawToken),
		CorrelationID:           strings.TrimSpace(in.CorrelationID),
	})
	if err != nil {
		return CreateRelationshipOutput{}, err
	}
	out := CreateRelationshipOutput{RequestID: stored.RequestID, Status: stored.Status, Created: stored.Created}
	if stored.Created {
		out.AcceptanceToken = domain.EncodeAcceptanceToken(rawToken)
	}
	return out, nil
}

func (s *RelationshipService) Accept(ctx context.Context, in AcceptRelationshipInput) (AcceptRelationshipOutput, error) {
	if err := s.commandsOpen(); err != nil {
		return AcceptRelationshipOutput{}, err
	}
	if strings.TrimSpace(s.cfg.TokenPepper) == "" {
		return AcceptRelationshipOutput{}, apperrors.New(apperrors.CodeRelationshipStoreUnavailable, "relationship store unavailable")
	}
	if in.BodyTenantID != nil && *in.BodyTenantID != in.Principal.TenantID {
		return AcceptRelationshipOutput{}, apperrors.New(apperrors.CodeTenantContextConflict, "tenant context conflict")
	}
	if in.BodyCompanyID != nil && *in.BodyCompanyID != in.SelectedCompanyID {
		return AcceptRelationshipOutput{}, refuseAccept()
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	if err := s.requireAcceptActor(ctx, in.Principal, in.SelectedCompanyID); err != nil {
		return AcceptRelationshipOutput{}, err
	}
	if in.LocatorInvalid {
		return AcceptRelationshipOutput{}, relationshipNotFound()
	}
	if key == "" || len(key) > 200 {
		return AcceptRelationshipOutput{}, apperrors.Validation("relationship request is invalid", nil)
	}
	tokenPresent := false
	var tokenHash []byte
	if strings.TrimSpace(in.AcceptanceToken) != "" {
		raw, err := domain.DecodeAcceptanceToken(strings.TrimSpace(in.AcceptanceToken))
		if err != nil {
			tokenPresent = false
		} else {
			tokenPresent = true
			tokenHash = domain.FingerprintAcceptanceToken(s.cfg.TokenPepper, raw)
		}
	}
	stored, err := s.store.AcceptRelationshipRequest(ctx, domain.PersistAccept{
		ActorTenantID:      in.Principal.TenantID,
		ActorUserID:        in.Principal.UserID,
		OperatingCompanyID: in.SelectedCompanyID,
		IdempotencyKey:     key,
		BodyHash:           domain.AcceptBodyHash(in.SelectedCompanyID, in.RequestID),
		TokenHash:          tokenHash,
		TokenPresent:       tokenPresent,
		RequestID:          in.RequestID,
		CorrelationID:      strings.TrimSpace(in.CorrelationID),
	}, s.sources)
	if err != nil {
		return AcceptRelationshipOutput{}, err
	}
	return AcceptRelationshipOutput{
		RelationshipID: stored.RelationshipID, RelationshipVersion: stored.RelationshipVersion,
		RequestStatus: stored.RequestStatus, Created: stored.Created,
	}, nil
}

func (s *RelationshipService) Get(ctx context.Context, principal domain.Principal, companyID, requestID uuid.UUID) (domain.VisibleRequest, error) {
	if err := s.commandsOpen(); err != nil {
		return domain.VisibleRequest{}, err
	}
	if err := s.requireReadActor(ctx, principal, companyID); err != nil {
		return domain.VisibleRequest{}, err
	}
	if requestID == uuid.Nil {
		return domain.VisibleRequest{}, relationshipNotFound()
	}
	return s.store.GetVisibleRelationshipRequest(ctx, principal.TenantID, companyID, requestID)
}

func (s *RelationshipService) commandsOpen() error {
	if !s.cfg.CommandsEnabled {
		return apperrors.New(apperrors.CodeRelationshipCommandsDisabled, "relationship commands are disabled")
	}
	return nil
}

func (s *RelationshipService) requireCreateActor(ctx context.Context, principal domain.Principal, companyID uuid.UUID, sourceType string) error {
	actor, err := s.loadActor(ctx, principal, companyID)
	if err != nil {
		return err
	}
	if !domain.CanCreateRelationshipRequest(actor, sourceType) {
		return refuseCreate()
	}
	return nil
}

func (s *RelationshipService) requireAcceptActor(ctx context.Context, principal domain.Principal, companyID uuid.UUID) error {
	actor, err := s.loadActor(ctx, principal, companyID)
	if err != nil {
		return err
	}
	if !domain.CanAcceptRelationship(actor) {
		return refuseAccept()
	}
	return nil
}

func (s *RelationshipService) requireReadActor(ctx context.Context, principal domain.Principal, companyID uuid.UUID) error {
	actor, err := s.loadActor(ctx, principal, companyID)
	if err != nil {
		return err
	}
	if !domain.CanReadRelationshipRequest(actor) {
		return refuseCreate()
	}
	return nil
}

func (s *RelationshipService) loadActor(ctx context.Context, principal domain.Principal, companyID uuid.UUID) (domain.ActorAuthority, error) {
	if companyID == uuid.Nil {
		return domain.ActorAuthority{}, refuseCreate()
	}
	actor, err := s.authority.LoadRelationshipActor(ctx, principal.TenantID, principal.UserID, companyID)
	if err != nil {
		return domain.ActorAuthority{}, err
	}
	return actor, nil
}

func refuseCreate() error {
	return apperrors.New(apperrors.CodeRelationshipRequestForbidden, "relationship command refused")
}

func refuseAccept() error {
	return apperrors.New(apperrors.CodeRelationshipAcceptanceForbidden, "relationship command refused")
}

func relationshipNotFound() error {
	return apperrors.New(apperrors.CodeRelationshipRequestNotFound, "relationship request not found")
}
