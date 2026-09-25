package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	"github.com/freight-platform/network-optimizer-service/internal/locationclient"
	apperrors "github.com/freight-platform/network-optimizer-service/internal/platform/errors"
	bnometrics "github.com/freight-platform/network-optimizer-service/internal/platform/metrics"
	"github.com/freight-platform/network-optimizer-service/internal/predict"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
	"github.com/freight-platform/network-optimizer-service/internal/sourceverify"
)

type Service struct {
	store     repository.Store
	verifier  sourceverify.Verifier
	directory locationclient.Directory
	routes    routing.Provider
	policies  repository.PolicyStore
	sources   predict.Sources
	policy    predict.Policy
	now       func() time.Time
}

func New(store repository.Store, verifier sourceverify.Verifier) *Service {
	if verifier == nil {
		verifier = sourceverify.Unavailable{}
	}
	return &Service{store: store, verifier: verifier, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) UseDirectory(directory locationclient.Directory) { s.directory = directory }

func (s *Service) UseRouting(provider routing.Provider) { s.routes = provider }

func (s *Service) UsePolicies(store repository.PolicyStore) { s.policies = store }

func (s *Service) ConfigurePrediction(sources predict.Sources, policy predict.Policy) {
	s.sources = sources
	s.policy = policy
}

func (s *Service) SetClock(now func() time.Time) {
	if now != nil {
		s.now = now
	}
}

type Actor struct {
	TenantID  uuid.UUID
	UserID    uuid.UUID
	CompanyID *uuid.UUID
	RequestID string
}

type Result struct {
	Status      int
	Body        []byte
	AggregateID uuid.UUID
	Replay      bool
}

type CreateLoadCommand struct {
	Load    domain.LoadOpportunity
	Publish bool
}

type LoadPatch struct {
	Version         int
	VisibilityScope *string
	Invited         *[]uuid.UUID
	Pickup          *domain.Place
	PickupWindow    *domain.TimeWindow
	Delivery        *domain.Place
	DeliveryWindow  *domain.TimeWindow
	WeightKg        *float64
	VolumeM3        *float64
	BodyType        *string
	Equipment       *[]string
	Cargo           *domain.CargoConstraints
	Commercial      *domain.Commercial
	Changed         bool
}

type CreateCapacityCommand struct {
	Capacity domain.Capacity
}

type CapacityPatch struct {
	Version            int
	CarrierCompanyID   *uuid.UUID
	VehicleID          *uuid.UUID
	LocationID         *uuid.UUID
	LocationLabel      *string
	Latitude           *float64
	Longitude          *float64
	CountryCode        *string
	Region             *string
	City               *string
	AvailableFrom      *time.Time
	AvailableUntil     *time.Time
	BodyType           *string
	Equipment          *[]string
	PayloadRemainingKg *float64
	VolumeRemainingM3  *float64
	VisibilityScope    *string
	Audience           *[]uuid.UUID
	Changed            bool
}

func HashBody(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func (s *Service) CreateLoad(ctx context.Context, actor Actor, key, hash string, cmd CreateLoadCommand) (Result, error) {
	load := cmd.Load
	load.OwnerTenantID = actor.TenantID
	if err := s.bindLoadGeography(ctx, actor.TenantID, &load); err != nil {
		return Result{}, err
	}
	if err := domain.ValidateLoad(load); err != nil {
		return Result{}, apperrors.Validation(err.Error(), nil)
	}
	owned, err := s.verifier.Owns(ctx, actor.TenantID, load.SourceType, load.SourceID)
	if err != nil {
		return Result{}, apperrors.ServiceUnavailable("source ownership could not be verified")
	}
	if !owned {
		return Result{}, apperrors.NotFound("source is not available for publication")
	}
	result, err := s.mutate(ctx, actor.TenantID, key, hash, func(tx repository.Tx, replay *Result) error {
		if done, err := takeIdempotency(ctx, tx, actor.TenantID, key, hash, replay); done || err != nil {
			return err
		}
		if _, err := tx.ActiveLoadBySource(ctx, actor.TenantID, load.SourceType, load.SourceID); err == nil {
			return apperrors.Conflict("an active publication already exists for this source", nil)
		} else if !errors.Is(err, repository.ErrNotFound) {
			return err
		}
		now := s.now()
		load.ID = uuid.New()
		load.Version = 1
		load.CreatedAt = now
		load.UpdatedAt = now
		load.Status = domain.LoadDraft
		if cmd.Publish {
			load.Status = domain.LoadPublished
		}
		if err := tx.InsertLoad(ctx, load); err != nil {
			if errors.Is(err, repository.ErrDuplicateSource) {
				return apperrors.Conflict("an active publication already exists for this source", nil)
			}
			return err
		}
		if err := s.audit(ctx, tx, actor, "load_opportunity", load.ID, "create", "", load.Status, load.VisibilityScope, load.SourceType, &load.SourceID, now); err != nil {
			return err
		}
		if load.Status == domain.LoadPublished {
			if err := s.emit(ctx, tx, domain.EventLoadPublished, actor.TenantID, load.ID, load.Version, load.Status, load.VisibilityScope, nil, now); err != nil {
				return err
			}
		}
		body, err := domain.MarshalLoad(load)
		if err != nil {
			return err
		}
		replay.Status = 201
		replay.Body = body
		replay.AggregateID = load.ID
		return saveIdempotency(ctx, tx, actor.TenantID, key, hash, replay.Status, body)
	})
	if err != nil {
		return Result{}, err
	}
	if !result.Replay {
		bnometrics.LoadOpportunities.Inc()
		if cmd.Publish {
			bnometrics.Publications.Inc()
		}
	}
	return result, nil
}

func (s *Service) PublishLoad(ctx context.Context, actor Actor, id uuid.UUID, version int, key, hash string) (Result, error) {
	result, err := s.mutate(ctx, actor.TenantID, key, hash, func(tx repository.Tx, replay *Result) error {
		if done, err := takeIdempotency(ctx, tx, actor.TenantID, key, hash, replay); done || err != nil {
			return err
		}
		load, err := s.ownLoad(ctx, tx, actor.TenantID, id)
		if err != nil {
			return err
		}
		if load.Version != version {
			return apperrors.Conflict("version conflict", nil)
		}
		if !domain.CanPublishLoad(load.Status) {
			return apperrors.Conflict("invalid status transition", nil)
		}
		now := s.now()
		from := load.Status
		load.Status = domain.LoadPublished
		load.Version++
		load.UpdatedAt = now
		if err := tx.UpdateLoad(ctx, load, version); err != nil {
			return mapWrite(err)
		}
		if err := s.audit(ctx, tx, actor, "load_opportunity", load.ID, "publish", from, load.Status, load.VisibilityScope, load.SourceType, &load.SourceID, now); err != nil {
			return err
		}
		if err := s.emit(ctx, tx, domain.EventLoadPublished, actor.TenantID, load.ID, load.Version, load.Status, load.VisibilityScope, nil, now); err != nil {
			return err
		}
		body, err := domain.MarshalLoad(load)
		if err != nil {
			return err
		}
		replay.Status = 200
		replay.Body = body
		replay.AggregateID = load.ID
		return saveIdempotency(ctx, tx, actor.TenantID, key, hash, 200, body)
	})
	if err != nil {
		return Result{}, err
	}
	if !result.Replay {
		bnometrics.Publications.Inc()
	}
	return result, nil
}

func (s *Service) UpdateLoad(ctx context.Context, actor Actor, id uuid.UUID, patch LoadPatch, key, hash string) (Result, error) {
	if !patch.Changed {
		return Result{}, apperrors.Validation("no fields to update", nil)
	}
	if patch.Version < 1 {
		return Result{}, apperrors.Validation("version is required", nil)
	}
	result, err := s.mutate(ctx, actor.TenantID, key, hash, func(tx repository.Tx, replay *Result) error {
		if done, err := takeIdempotency(ctx, tx, actor.TenantID, key, hash, replay); done || err != nil {
			return err
		}
		load, err := s.ownLoad(ctx, tx, actor.TenantID, id)
		if err != nil {
			return err
		}
		if load.Version != patch.Version {
			return apperrors.Conflict("version conflict", nil)
		}
		if !domain.CanUpdateLoad(load.Status) {
			return apperrors.Conflict("invalid status transition", nil)
		}
		applyLoadPatch(&load, patch)
		if err := s.bindLoadGeography(ctx, actor.TenantID, &load); err != nil {
			return err
		}
		if err := domain.ValidateLoad(load); err != nil {
			return apperrors.Validation(err.Error(), nil)
		}
		now := s.now()
		from := load.Status
		load.Version++
		load.UpdatedAt = now
		if err := tx.UpdateLoad(ctx, load, patch.Version); err != nil {
			return mapWrite(err)
		}
		if err := s.audit(ctx, tx, actor, "load_opportunity", load.ID, "update", from, load.Status, load.VisibilityScope, load.SourceType, &load.SourceID, now); err != nil {
			return err
		}
		if load.Status == domain.LoadPublished {
			if err := s.emit(ctx, tx, domain.EventLoadUpdated, actor.TenantID, load.ID, load.Version, load.Status, load.VisibilityScope, nil, now); err != nil {
				return err
			}
		}
		body, err := domain.MarshalLoad(load)
		if err != nil {
			return err
		}
		replay.Status = 200
		replay.Body = body
		replay.AggregateID = load.ID
		return saveIdempotency(ctx, tx, actor.TenantID, key, hash, 200, body)
	})
	return result, err
}

func (s *Service) WithdrawLoad(ctx context.Context, actor Actor, id uuid.UUID, version int, key, hash string) (Result, error) {
	if version < 1 {
		return Result{}, apperrors.Validation("version is required", nil)
	}
	result, err := s.mutate(ctx, actor.TenantID, key, hash, func(tx repository.Tx, replay *Result) error {
		if done, err := takeIdempotency(ctx, tx, actor.TenantID, key, hash, replay); done || err != nil {
			return err
		}
		load, err := s.ownLoad(ctx, tx, actor.TenantID, id)
		if err != nil {
			return err
		}
		if load.Version != version {
			return apperrors.Conflict("version conflict", nil)
		}
		if !domain.CanWithdrawLoad(load.Status) {
			return apperrors.Conflict("invalid status transition", nil)
		}
		now := s.now()
		from := load.Status
		load.Status = domain.LoadWithdrawn
		load.Version++
		load.UpdatedAt = now
		if err := tx.UpdateLoad(ctx, load, version); err != nil {
			return mapWrite(err)
		}
		if err := s.audit(ctx, tx, actor, "load_opportunity", load.ID, "withdraw", from, load.Status, load.VisibilityScope, load.SourceType, &load.SourceID, now); err != nil {
			return err
		}
		if from == domain.LoadPublished {
			if err := s.emit(ctx, tx, domain.EventLoadWithdrawn, actor.TenantID, load.ID, load.Version, load.Status, load.VisibilityScope, nil, now); err != nil {
				return err
			}
		}
		body, err := domain.MarshalLoad(load)
		if err != nil {
			return err
		}
		replay.Status = 200
		replay.Body = body
		replay.AggregateID = load.ID
		return saveIdempotency(ctx, tx, actor.TenantID, key, hash, 200, body)
	})
	if err == nil && !result.Replay {
		bnometrics.Withdrawals.Inc()
	}
	return result, err
}

func (s *Service) GetOwnLoad(ctx context.Context, actor Actor, id uuid.UUID) (Result, error) {
	var body []byte
	err := s.store.Within(ctx, func(tx repository.Tx) error {
		load, err := s.ownLoad(ctx, tx, actor.TenantID, id)
		if err != nil {
			return err
		}
		body, err = domain.MarshalLoad(load)
		return err
	})
	if err != nil {
		return Result{}, err
	}
	return Result{Status: 200, Body: body, AggregateID: id}, nil
}

func (s *Service) ListOwnLoads(ctx context.Context, actor Actor, limit, offset int) (Result, error) {
	limit, offset, err := normalizePage(limit, offset)
	if err != nil {
		return Result{}, err
	}
	var items []json.RawMessage
	err = s.store.Within(ctx, func(tx repository.Tx) error {
		rows, err := tx.ListOwnLoads(ctx, actor.TenantID, limit, offset)
		if err != nil {
			return err
		}
		items = make([]json.RawMessage, 0, len(rows))
		for _, row := range rows {
			raw, err := domain.MarshalLoad(row)
			if err != nil {
				return err
			}
			items = append(items, raw)
		}
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	body, err := marshalList(items, limit, offset)
	if err != nil {
		return Result{}, err
	}
	return Result{Status: 200, Body: body}, nil
}

func (s *Service) GetMarketplaceLoad(ctx context.Context, actor Actor, id uuid.UUID) (Result, error) {
	var body []byte
	err := s.store.Within(ctx, func(tx repository.Tx) error {
		load, err := tx.GetLoad(ctx, id)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return apperrors.NotFound("load opportunity not found")
			}
			return err
		}
		ok, _ := domain.LoadVisible(load, actor.TenantID, actor.CompanyID)
		if !ok {
			return apperrors.NotFound("load opportunity not found")
		}
		body, err = domain.MarshalMarketplaceLoad(load)
		return err
	})
	if err != nil {
		return Result{}, err
	}
	return Result{Status: 200, Body: body, AggregateID: id}, nil
}

func (s *Service) ListMarketplaceLoads(ctx context.Context, actor Actor, limit, offset int) (Result, error) {
	limit, offset, err := normalizePage(limit, offset)
	if err != nil {
		return Result{}, err
	}
	var items []json.RawMessage
	err = s.store.Within(ctx, func(tx repository.Tx) error {
		rows, err := tx.ListMarketplaceLoads(ctx, actor.TenantID, actor.CompanyID, limit, offset)
		if err != nil {
			return err
		}
		items = make([]json.RawMessage, 0, len(rows))
		for _, row := range rows {
			raw, err := domain.MarshalMarketplaceLoad(row)
			if err != nil {
				return err
			}
			items = append(items, raw)
		}
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	body, err := marshalList(items, limit, offset)
	if err != nil {
		return Result{}, err
	}
	return Result{Status: 200, Body: body}, nil
}

func (s *Service) CreateCapacity(ctx context.Context, actor Actor, key, hash string, cmd CreateCapacityCommand) (Result, error) {
	cap := cmd.Capacity
	cap.OwnerTenantID = actor.TenantID
	if cap.Source == "" {
		cap.Source = domain.SourceManual
	}
	if err := s.bindCapacityLocation(ctx, actor.TenantID, &cap); err != nil {
		return Result{}, err
	}
	if err := domain.ValidateCapacity(cap); err != nil {
		return Result{}, apperrors.Validation(err.Error(), nil)
	}
	result, err := s.mutate(ctx, actor.TenantID, key, hash, func(tx repository.Tx, replay *Result) error {
		if done, err := takeIdempotency(ctx, tx, actor.TenantID, key, hash, replay); done || err != nil {
			return err
		}
		now := s.now()
		cap.ID = uuid.New()
		cap.Status = domain.CapacityAvailable
		cap.Version = 1
		cap.CreatedAt = now
		cap.UpdatedAt = now
		if err := tx.InsertCapacity(ctx, cap); err != nil {
			return err
		}
		if err := s.audit(ctx, tx, actor, "capacity", cap.ID, "create", "", cap.Status, cap.VisibilityScope, cap.Source, nil, now); err != nil {
			return err
		}
		if err := s.emit(ctx, tx, domain.EventCapacityPublished, actor.TenantID, cap.ID, cap.Version, cap.Status, cap.VisibilityScope, cap.LocationID, now); err != nil {
			return err
		}
		body, err := domain.MarshalCapacity(cap)
		if err != nil {
			return err
		}
		replay.Status = 201
		replay.Body = body
		replay.AggregateID = cap.ID
		return saveIdempotency(ctx, tx, actor.TenantID, key, hash, 201, body)
	})
	if err == nil && !result.Replay {
		bnometrics.Capacities.Inc()
		bnometrics.Publications.Inc()
	}
	return result, err
}

func (s *Service) UpdateCapacity(ctx context.Context, actor Actor, id uuid.UUID, patch CapacityPatch, key, hash string) (Result, error) {
	if !patch.Changed {
		return Result{}, apperrors.Validation("no fields to update", nil)
	}
	if patch.Version < 1 {
		return Result{}, apperrors.Validation("version is required", nil)
	}
	result, err := s.mutate(ctx, actor.TenantID, key, hash, func(tx repository.Tx, replay *Result) error {
		if done, err := takeIdempotency(ctx, tx, actor.TenantID, key, hash, replay); done || err != nil {
			return err
		}
		cap, err := s.ownCapacity(ctx, tx, actor.TenantID, id)
		if err != nil {
			return err
		}
		if cap.Version != patch.Version {
			return apperrors.Conflict("version conflict", nil)
		}
		if !domain.CanUpdateCapacity(cap.Status) {
			return apperrors.Conflict("invalid status transition", nil)
		}
		applyCapacityPatch(&cap, patch)
		if err := s.bindCapacityLocation(ctx, actor.TenantID, &cap); err != nil {
			return err
		}
		if err := domain.ValidateCapacity(cap); err != nil {
			return apperrors.Validation(err.Error(), nil)
		}
		now := s.now()
		from := cap.Status
		cap.Version++
		cap.UpdatedAt = now
		if err := tx.UpdateCapacity(ctx, cap, patch.Version); err != nil {
			return mapWrite(err)
		}
		if err := s.audit(ctx, tx, actor, "capacity", cap.ID, "update", from, cap.Status, cap.VisibilityScope, cap.Source, nil, now); err != nil {
			return err
		}
		if err := s.emit(ctx, tx, domain.EventCapacityUpdated, actor.TenantID, cap.ID, cap.Version, cap.Status, cap.VisibilityScope, cap.LocationID, now); err != nil {
			return err
		}
		body, err := domain.MarshalCapacity(cap)
		if err != nil {
			return err
		}
		replay.Status = 200
		replay.Body = body
		replay.AggregateID = cap.ID
		return saveIdempotency(ctx, tx, actor.TenantID, key, hash, 200, body)
	})
	return result, err
}

func (s *Service) WithdrawCapacity(ctx context.Context, actor Actor, id uuid.UUID, version int, key, hash string) (Result, error) {
	if version < 1 {
		return Result{}, apperrors.Validation("version is required", nil)
	}
	result, err := s.mutate(ctx, actor.TenantID, key, hash, func(tx repository.Tx, replay *Result) error {
		if done, err := takeIdempotency(ctx, tx, actor.TenantID, key, hash, replay); done || err != nil {
			return err
		}
		cap, err := s.ownCapacity(ctx, tx, actor.TenantID, id)
		if err != nil {
			return err
		}
		if cap.Version != version {
			return apperrors.Conflict("version conflict", nil)
		}
		if !domain.CanWithdrawCapacity(cap.Status) {
			return apperrors.Conflict("invalid status transition", nil)
		}
		now := s.now()
		from := cap.Status
		cap.Status = domain.CapacityWithdrawn
		cap.Version++
		cap.UpdatedAt = now
		if err := tx.UpdateCapacity(ctx, cap, version); err != nil {
			return mapWrite(err)
		}
		if err := s.audit(ctx, tx, actor, "capacity", cap.ID, "withdraw", from, cap.Status, cap.VisibilityScope, cap.Source, nil, now); err != nil {
			return err
		}
		if err := s.emit(ctx, tx, domain.EventCapacityWithdrawn, actor.TenantID, cap.ID, cap.Version, cap.Status, cap.VisibilityScope, cap.LocationID, now); err != nil {
			return err
		}
		body, err := domain.MarshalCapacity(cap)
		if err != nil {
			return err
		}
		replay.Status = 200
		replay.Body = body
		replay.AggregateID = cap.ID
		return saveIdempotency(ctx, tx, actor.TenantID, key, hash, 200, body)
	})
	if err == nil && !result.Replay {
		bnometrics.Withdrawals.Inc()
	}
	return result, err
}

func (s *Service) GetOwnCapacity(ctx context.Context, actor Actor, id uuid.UUID) (Result, error) {
	var body []byte
	err := s.store.Within(ctx, func(tx repository.Tx) error {
		cap, err := s.ownCapacity(ctx, tx, actor.TenantID, id)
		if err != nil {
			return err
		}
		body, err = domain.MarshalCapacity(cap)
		return err
	})
	if err != nil {
		return Result{}, err
	}
	return Result{Status: 200, Body: body, AggregateID: id}, nil
}

func (s *Service) ListOwnCapacities(ctx context.Context, actor Actor, limit, offset int) (Result, error) {
	return s.listCapacities(ctx, actor, limit, offset, false)
}

func (s *Service) GetMarketplaceCapacity(ctx context.Context, actor Actor, id uuid.UUID) (Result, error) {
	var body []byte
	err := s.store.Within(ctx, func(tx repository.Tx) error {
		cap, err := tx.GetCapacity(ctx, id)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return apperrors.NotFound("capacity not found")
			}
			return err
		}
		if !domain.CapacityVisible(cap, actor.TenantID) {
			return apperrors.NotFound("capacity not found")
		}
		body, err = domain.MarshalMarketplaceCapacity(cap)
		return err
	})
	if err != nil {
		return Result{}, err
	}
	return Result{Status: 200, Body: body, AggregateID: id}, nil
}

func (s *Service) ListMarketplaceCapacities(ctx context.Context, actor Actor, limit, offset int) (Result, error) {
	return s.listCapacities(ctx, actor, limit, offset, true)
}

func (s *Service) listCapacities(ctx context.Context, actor Actor, limit, offset int, marketplace bool) (Result, error) {
	limit, offset, err := normalizePage(limit, offset)
	if err != nil {
		return Result{}, err
	}
	var items []json.RawMessage
	err = s.store.Within(ctx, func(tx repository.Tx) error {
		var rows []domain.Capacity
		var err error
		if marketplace {
			rows, err = tx.ListMarketplaceCapacities(ctx, actor.TenantID, limit, offset)
		} else {
			rows, err = tx.ListOwnCapacities(ctx, actor.TenantID, limit, offset)
		}
		if err != nil {
			return err
		}
		items = make([]json.RawMessage, 0, len(rows))
		for _, row := range rows {
			var raw []byte
			if marketplace {
				raw, err = domain.MarshalMarketplaceCapacity(row)
			} else {
				raw, err = domain.MarshalCapacity(row)
			}
			if err != nil {
				return err
			}
			items = append(items, raw)
		}
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	body, err := marshalList(items, limit, offset)
	if err != nil {
		return Result{}, err
	}
	return Result{Status: 200, Body: body}, nil
}

func (s *Service) mutate(ctx context.Context, tenant uuid.UUID, key, hash string, fn func(repository.Tx, *Result) error) (Result, error) {
	var result Result
	err := s.store.Within(ctx, func(tx repository.Tx) error {
		return fn(tx, &result)
	})
	if errors.Is(err, repository.ErrIdempotencyRace) {
		return s.readReplay(ctx, tenant, key, hash)
	}
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

func (s *Service) readReplay(ctx context.Context, tenant uuid.UUID, key, hash string) (Result, error) {
	var rec repository.IdempotencyRecord
	err := s.store.Within(ctx, func(tx repository.Tx) error {
		var getErr error
		rec, getErr = tx.GetIdempotency(ctx, tenant, key)
		return getErr
	})
	if errors.Is(err, repository.ErrNotFound) {
		return Result{}, apperrors.Conflict("idempotency key is in progress", nil)
	}
	if err != nil {
		return Result{}, apperrors.Internal("idempotency lookup failed", err)
	}
	if rec.Hash != hash {
		return Result{}, apperrors.Conflict("idempotency key was reused with a different request", nil)
	}
	return Result{Status: rec.Status, Body: rec.Body, Replay: true}, nil
}

func (s *Service) ownLoad(ctx context.Context, tx repository.Tx, tenant, id uuid.UUID) (domain.LoadOpportunity, error) {
	load, err := tx.GetLoad(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domain.LoadOpportunity{}, apperrors.NotFound("load opportunity not found")
		}
		return domain.LoadOpportunity{}, err
	}
	if load.OwnerTenantID != tenant {
		return domain.LoadOpportunity{}, apperrors.NotFound("load opportunity not found")
	}
	return load, nil
}

func (s *Service) ownCapacity(ctx context.Context, tx repository.Tx, tenant, id uuid.UUID) (domain.Capacity, error) {
	cap, err := tx.GetCapacity(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domain.Capacity{}, apperrors.NotFound("capacity not found")
		}
		return domain.Capacity{}, err
	}
	if cap.OwnerTenantID != tenant {
		return domain.Capacity{}, apperrors.NotFound("capacity not found")
	}
	return cap, nil
}

func (s *Service) audit(ctx context.Context, tx repository.Tx, actor Actor, aggregateType string, aggregateID uuid.UUID, action, from, to, visibility, sourceType string, sourceID *uuid.UUID, now time.Time) error {
	return tx.InsertAudit(ctx, repository.AuditEvent{
		ID: uuid.New(), ActorUserID: actor.UserID, TenantID: actor.TenantID,
		AggregateType: aggregateType, AggregateID: aggregateID, Action: action,
		FromStatus: from, ToStatus: to, VisibilityScope: visibility,
		SourceType: sourceType, SourceID: sourceID, OccurredAt: now,
	})
}

func (s *Service) emit(ctx context.Context, tx repository.Tx, name string, tenant, aggregate uuid.UUID, version int, status, visibility string, locationID *uuid.UUID, now time.Time) error {
	id := uuid.New()
	payloadMap := map[string]any{
		"eventId": id, "eventName": name, "schemaVersion": 1,
		"tenantId": tenant, "aggregateId": aggregate, "aggregateVersion": version,
		"occurredAt": now.Format(time.RFC3339Nano), "status": status, "visibilityScope": visibility,
	}
	if locationID != nil {
		payloadMap["locationId"] = locationID.String()
	}
	payload, err := json.Marshal(payloadMap)
	if err != nil {
		return err
	}
	return tx.InsertOutbox(ctx, repository.OutboxEvent{
		ID: id, EventName: name, SchemaVersion: 1, TenantID: tenant,
		AggregateID: aggregate, AggregateVersion: version, OccurredAt: now, Payload: payload,
	})
}

func takeIdempotency(ctx context.Context, tx repository.Tx, tenant uuid.UUID, key, hash string, result *Result) (bool, error) {
	if key == "" {
		return false, nil
	}
	rec, err := tx.GetIdempotency(ctx, tenant, key)
	if errors.Is(err, repository.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if rec.Hash != hash {
		return false, apperrors.Conflict("idempotency key was reused with a different request", nil)
	}
	result.Status = rec.Status
	result.Body = append([]byte(nil), rec.Body...)
	result.Replay = true
	return true, nil
}

func saveIdempotency(ctx context.Context, tx repository.Tx, tenant uuid.UUID, key, hash string, status int, body []byte) error {
	if key == "" {
		return nil
	}
	return tx.PutIdempotency(ctx, tenant, repository.IdempotencyRecord{Key: key, Hash: hash, Status: status, Body: body})
}

func mapWrite(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrConflict):
		return apperrors.Conflict("version conflict", nil)
	case errors.Is(err, repository.ErrNotFound):
		return apperrors.NotFound("record not found")
	default:
		return err
	}
}

func applyLoadPatch(load *domain.LoadOpportunity, patch LoadPatch) {
	if patch.VisibilityScope != nil {
		load.VisibilityScope = *patch.VisibilityScope
	}
	if patch.Invited != nil {
		load.InvitedCarrierCompanyIDs = append([]uuid.UUID(nil), (*patch.Invited)...)
	}
	if patch.Pickup != nil {
		load.Pickup = *patch.Pickup
	}
	if patch.PickupWindow != nil {
		load.PickupWindow = *patch.PickupWindow
	}
	if patch.Delivery != nil {
		load.Delivery = *patch.Delivery
	}
	if patch.DeliveryWindow != nil {
		load.DeliveryWindow = *patch.DeliveryWindow
	}
	if patch.WeightKg != nil {
		load.WeightKg = patch.WeightKg
	}
	if patch.VolumeM3 != nil {
		load.VolumeM3 = patch.VolumeM3
	}
	if patch.BodyType != nil {
		load.BodyType = *patch.BodyType
	}
	if patch.Equipment != nil {
		load.Equipment = append([]string(nil), (*patch.Equipment)...)
	}
	if patch.Cargo != nil {
		load.Cargo = *patch.Cargo
	}
	if patch.Commercial != nil {
		load.Commercial = *patch.Commercial
	}
}

func applyCapacityPatch(cap *domain.Capacity, patch CapacityPatch) {
	if patch.CarrierCompanyID != nil {
		cap.CarrierCompanyID = patch.CarrierCompanyID
	}
	if patch.VehicleID != nil {
		cap.VehicleID = patch.VehicleID
	}
	if patch.LocationID != nil {
		cap.LocationID = patch.LocationID
	}
	if patch.LocationLabel != nil {
		cap.LocationLabel = *patch.LocationLabel
	}
	if patch.CountryCode != nil {
		cap.CountryCode = *patch.CountryCode
	}
	if patch.Region != nil {
		cap.Region = *patch.Region
	}
	if patch.City != nil {
		cap.City = *patch.City
	}
	if patch.Latitude != nil {
		cap.Latitude = patch.Latitude
	}
	if patch.Longitude != nil {
		cap.Longitude = patch.Longitude
	}
	if patch.AvailableFrom != nil {
		cap.AvailableFrom = *patch.AvailableFrom
	}
	if patch.AvailableUntil != nil {
		cap.AvailableUntil = *patch.AvailableUntil
	}
	if patch.BodyType != nil {
		cap.BodyType = *patch.BodyType
	}
	if patch.Equipment != nil {
		cap.Equipment = append([]string(nil), (*patch.Equipment)...)
	}
	if patch.PayloadRemainingKg != nil {
		cap.PayloadRemainingKg = patch.PayloadRemainingKg
	}
	if patch.VolumeRemainingM3 != nil {
		cap.VolumeRemainingM3 = patch.VolumeRemainingM3
	}
	if patch.VisibilityScope != nil {
		cap.VisibilityScope = *patch.VisibilityScope
	}
	if patch.Audience != nil {
		cap.AudienceTenantIDs = append([]uuid.UUID(nil), (*patch.Audience)...)
	}
}

func normalizePage(limit, offset int) (int, int, error) {
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 100 || offset < 0 {
		return 0, 0, apperrors.Validation("limit must be 1..100 and offset must be >= 0", nil)
	}
	return limit, offset, nil
}

func marshalList(items []json.RawMessage, limit, offset int) ([]byte, error) {
	if items == nil {
		items = []json.RawMessage{}
	}
	return json.Marshal(struct {
		Items  []json.RawMessage `json:"items"`
		Limit  int               `json:"limit"`
		Offset int               `json:"offset"`
	}{Items: items, Limit: limit, Offset: offset})
}

func (s *Service) bindLoadGeography(ctx context.Context, tenant uuid.UUID, load *domain.LoadOpportunity) error {
	if s.directory == nil {
		return nil
	}
	origin, destination, err := s.directory.SourceEndpoints(ctx, tenant, load.SourceType, load.SourceID)
	if errors.Is(err, locationclient.ErrNotFound) {
		return apperrors.NotFound("source location is not available")
	}
	if err != nil {
		return apperrors.ServiceUnavailable("source location could not be resolved")
	}
	if load.Pickup.LocationID != nil && *load.Pickup.LocationID != origin {
		return apperrors.NotFound("location not found")
	}
	if load.Delivery.LocationID != nil && *load.Delivery.LocationID != destination {
		return apperrors.NotFound("location not found")
	}
	pickup, err := s.directory.Projection(ctx, tenant, origin)
	if errors.Is(err, locationclient.ErrNotFound) {
		return apperrors.NotFound("location not found")
	}
	if err != nil {
		return apperrors.ServiceUnavailable("source location could not be resolved")
	}
	delivery, err := s.directory.Projection(ctx, tenant, destination)
	if errors.Is(err, locationclient.ErrNotFound) {
		return apperrors.NotFound("location not found")
	}
	if err != nil {
		return apperrors.ServiceUnavailable("source location could not be resolved")
	}
	load.Pickup = domain.ApplyPlaceSnapshot(load.Pickup, pickup)
	load.Delivery = domain.ApplyPlaceSnapshot(load.Delivery, delivery)
	return nil
}

func (s *Service) bindCapacityLocation(ctx context.Context, tenant uuid.UUID, cap *domain.Capacity) error {
	if cap.LocationID == nil {
		return nil
	}
	if s.directory == nil {
		return apperrors.ServiceUnavailable("location could not be resolved")
	}
	snap, err := s.directory.Projection(ctx, tenant, *cap.LocationID)
	if errors.Is(err, locationclient.ErrNotFound) {
		return apperrors.NotFound("location not found")
	}
	if err != nil {
		return apperrors.ServiceUnavailable("location could not be resolved")
	}
	domain.ApplyCapacitySnapshot(cap, snap)
	return nil
}

func (s *Service) SaveCarrierPolicy(ctx context.Context, tenant uuid.UUID, policy domain.NextLoadSearchPolicy) error {
	if s.policies == nil {
		return apperrors.ServiceUnavailable("search policy store is unavailable")
	}
	if err := policy.Validate(); err != nil {
		return apperrors.Validation(err.Error(), nil)
	}
	return s.policies.UpsertCarrierPolicy(ctx, tenant, policy)
}

func (s *Service) SaveCapacityPolicy(ctx context.Context, tenant, capacityID uuid.UUID, policy domain.NextLoadSearchPolicy) error {
	if s.policies == nil {
		return apperrors.ServiceUnavailable("search policy store is unavailable")
	}
	if err := policy.ValidateOverride(); err != nil {
		return apperrors.Validation(err.Error(), nil)
	}
	if err := s.policies.UpsertCapacityPolicy(ctx, tenant, capacityID, policy); err != nil {
		if errors.Is(err, repository.ErrNotFound) || errors.Is(err, repository.ErrConflict) {
			return apperrors.NotFound("capacity is not available")
		}
		return err
	}
	return nil
}

func (s *Service) RoutingProvider() routing.Provider { return s.routes }
