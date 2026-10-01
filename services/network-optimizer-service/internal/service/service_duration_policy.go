package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/network-optimizer-service/internal/platform/errors"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
)

func (s *Service) CreateServiceDurationDraft(ctx context.Context, actor Actor, pickupSeconds, deliverySeconds int) (repository.ServiceDurationPolicy, error) {
	if pickupSeconds < 0 || deliverySeconds < 0 {
		return repository.ServiceDurationPolicy{}, apperrors.Validation("service duration seconds must be zero or greater", nil)
	}
	var created repository.ServiceDurationPolicy
	err := s.store.Within(ctx, func(tx repository.Tx) error {
		version, err := tx.NextServiceDurationVersion(ctx, actor.TenantID)
		if err != nil {
			return err
		}
		created = repository.ServiceDurationPolicy{
			ID: uuid.New(), TenantID: actor.TenantID, Version: version, Status: repository.DurationPolicyDraft,
			PickupSeconds: pickupSeconds, DeliverySeconds: deliverySeconds, CreatedAt: s.now(),
		}
		return tx.InsertServiceDurationDraft(ctx, created)
	})
	if err != nil {
		return repository.ServiceDurationPolicy{}, err
	}
	return created, nil
}

func (s *Service) UpdateServiceDurationDraft(ctx context.Context, actor Actor, id uuid.UUID, pickupSeconds, deliverySeconds int) error {
	if pickupSeconds < 0 || deliverySeconds < 0 {
		return apperrors.Validation("service duration seconds must be zero or greater", nil)
	}
	err := s.store.Within(ctx, func(tx repository.Tx) error {
		return tx.UpdateServiceDurationDraft(ctx, actor.TenantID, id, pickupSeconds, deliverySeconds)
	})
	if errors.Is(err, repository.ErrNotFound) {
		return apperrors.NotFound("service duration policy not found")
	}
	if errors.Is(err, repository.ErrConflict) {
		return apperrors.Conflict("active service duration policy is immutable", nil)
	}
	return err
}

func (s *Service) PublishServiceDurationPolicy(ctx context.Context, actor Actor, id uuid.UUID) (repository.ServiceDurationPolicy, error) {
	var published repository.ServiceDurationPolicy
	err := s.store.Within(ctx, func(tx repository.Tx) error {
		row, err := tx.PublishServiceDurationPolicy(ctx, actor.TenantID, id, s.now())
		if err != nil {
			return err
		}
		published = row
		return nil
	})
	if errors.Is(err, repository.ErrNotFound) {
		return repository.ServiceDurationPolicy{}, apperrors.NotFound("service duration policy not found")
	}
	if errors.Is(err, repository.ErrConflict) {
		return repository.ServiceDurationPolicy{}, apperrors.Conflict("service duration policy cannot be published", nil)
	}
	return published, err
}
