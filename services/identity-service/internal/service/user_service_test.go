package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/identity-service/internal/domain"
)

type mockUserStore struct {
	createFn func(ctx context.Context, in domain.CreateUserInput, passwordHash string) (*domain.User, error)
}

func (m *mockUserStore) Create(ctx context.Context, in domain.CreateUserInput, passwordHash string) (*domain.User, error) {
	return m.createFn(ctx, in, passwordHash)
}

func (m *mockUserStore) GetByID(context.Context, uuid.UUID) (*domain.User, error) { return nil, nil }
func (m *mockUserStore) GetByTenantAndEmail(context.Context, uuid.UUID, string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserStore) List(context.Context, domain.ListUsersFilter) ([]domain.User, int, error) {
	return nil, 0, nil
}
func (m *mockUserStore) Update(context.Context, uuid.UUID, domain.UpdateUserInput) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserStore) SoftDelete(context.Context, uuid.UUID) error { return nil }
func (m *mockUserStore) UpdateLastLogin(context.Context, uuid.UUID) error { return nil }

func TestUserServiceCreate(t *testing.T) {
	t.Parallel()

	svc := NewUserService(&mockUserStore{
		createFn: func(_ context.Context, in domain.CreateUserInput, passwordHash string) (*domain.User, error) {
			if passwordHash == "" {
				t.Fatalf("expected password hash")
			}
			if in.Password != "" {
				t.Fatalf("password must not be passed to repository")
			}
			return &domain.User{
				ID:              uuid.New(),
				TenantID:        in.TenantID,
				Email:           in.Email,
				FullName:        in.FullName,
				PreferredLocale: in.PreferredLocale,
				Status:          domain.UserStatusActive,
				CreatedAt:       time.Now().UTC(),
				UpdatedAt:       time.Now().UTC(),
				Version:         1,
			}, nil
		},
	})

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	user, err := svc.Create(context.Background(), domain.CreateUserInput{
		TenantID:        tenantID,
		Email:           "user@example.com",
		Password:        "StrongPassword123!",
		FullName:        "Иван Иванов",
		PreferredLocale: "ru-RU",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Email != "user@example.com" {
		t.Fatalf("unexpected email: %s", user.Email)
	}
}
