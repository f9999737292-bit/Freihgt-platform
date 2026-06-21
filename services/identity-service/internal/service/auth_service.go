package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/freight-platform/identity-service/internal/domain"
	"github.com/freight-platform/identity-service/internal/platform/security"
	apperrors "github.com/freight-platform/identity-service/internal/platform/errors"
)

type AuthUserStore interface {
	GetByTenantAndEmail(ctx context.Context, tenantID uuid.UUID, email string) (*domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	UpdateLastLogin(ctx context.Context, id uuid.UUID) error
}

type LoginResult struct {
	AccessToken string
	TokenType   string
	ExpiresIn   int64
	User        *domain.User
}

type AuthService struct {
	users AuthUserStore
	jwt   *security.JWTService
}

func NewAuthService(users AuthUserStore, jwt *security.JWTService) *AuthService {
	return &AuthService{users: users, jwt: jwt}
}

func (s *AuthService) Login(ctx context.Context, in domain.LoginInput) (*LoginResult, error) {
	in.Email = domain.NormalizeEmail(in.Email)
	if err := domain.ValidateLoginInput(in); err != nil {
		return nil, err
	}

	user, err := s.users.GetByTenantAndEmail(ctx, in.TenantID, in.Email)
	if err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound {
			return nil, apperrors.Unauthorized("invalid credentials")
		}
		return nil, err
	}

	if !security.VerifyPassword(user.PasswordHash, in.Password) {
		return nil, apperrors.Unauthorized("invalid credentials")
	}

	if user.Status != domain.UserStatusActive {
		return nil, apperrors.Forbidden("user is not active")
	}

	token, expiresIn, err := s.jwt.CreateAccessToken(user.ID, user.TenantID, user.Email, user.PreferredLocale)
	if err != nil {
		return nil, apperrors.Internal("failed to create access token", err)
	}

	_ = s.users.UpdateLastLogin(ctx, user.ID)

	return &LoginResult{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   expiresIn,
		User:        user,
	}, nil
}

func (s *AuthService) GetCurrentUser(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	if userID == uuid.Nil {
		return nil, apperrors.Unauthorized("invalid token")
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound {
			return nil, apperrors.Unauthorized("invalid token")
		}
		return nil, err
	}
	return user, nil
}
