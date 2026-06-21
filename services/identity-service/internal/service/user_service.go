package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/identity-service/internal/domain"
	"github.com/freight-platform/identity-service/internal/platform/security"
	apperrors "github.com/freight-platform/identity-service/internal/platform/errors"
)

type UserStore interface {
	Create(ctx context.Context, in domain.CreateUserInput, passwordHash string) (*domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	List(ctx context.Context, filter domain.ListUsersFilter) ([]domain.User, int, error)
	Update(ctx context.Context, id uuid.UUID, in domain.UpdateUserInput) (*domain.User, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

type UserService struct {
	repo UserStore
}

func NewUserService(repo UserStore) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) Create(ctx context.Context, in domain.CreateUserInput) (*domain.User, error) {
	in.Email = domain.NormalizeEmail(in.Email)
	in.PreferredLocale = domain.NormalizePreferredLocale(in.PreferredLocale)
	if err := domain.ValidateCreateUserInput(in); err != nil {
		return nil, err
	}

	hash, err := security.HashPassword(in.Password)
	if err != nil {
		return nil, apperrors.Internal("failed to hash password", err)
	}

	in.Password = ""
	return s.repo.Create(ctx, in, hash)
}

func (s *UserService) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	return s.repo.GetByID(ctx, id)
}

func (s *UserService) List(ctx context.Context, filter domain.ListUsersFilter) ([]domain.User, int, error) {
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if err := domain.ValidateListUsersFilter(filter); err != nil {
		return nil, 0, err
	}
	return s.repo.List(ctx, filter)
}

func (s *UserService) Update(ctx context.Context, id uuid.UUID, in domain.UpdateUserInput) (*domain.User, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateUpdateUserInput(in); err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, id, in)
}

func (s *UserService) Delete(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	return s.repo.SoftDelete(ctx, id)
}
