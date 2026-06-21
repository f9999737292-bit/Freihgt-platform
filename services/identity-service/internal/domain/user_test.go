package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestValidateCreateUserInput(t *testing.T) {
	t.Parallel()

	validTenant := mustUUID(t, "11111111-1111-1111-1111-111111111111")

	if err := ValidateCreateUserInput(CreateUserInput{
		TenantID:        validTenant,
		Email:           "user@example.com",
		Password:        "StrongPassword123!",
		FullName:        "Иван Иванов",
		PreferredLocale: "ru-RU",
	}); err != nil {
		t.Fatalf("expected valid input, got %v", err)
	}

	if err := ValidateCreateUserInput(CreateUserInput{
		Email:    "user@example.com",
		Password: "StrongPassword123!",
		FullName: "Иван Иванов",
	}); err == nil {
		t.Fatalf("expected tenant validation error")
	}
}

func TestValidateLoginInput(t *testing.T) {
	t.Parallel()

	validTenant := mustUUID(t, "11111111-1111-1111-1111-111111111111")

	if err := ValidateLoginInput(LoginInput{
		TenantID: validTenant,
		Email:    "user@example.com",
		Password: "StrongPassword123!",
	}); err != nil {
		t.Fatalf("expected valid login input, got %v", err)
	}

	if err := ValidateLoginInput(LoginInput{
		TenantID: validTenant,
		Email:    "user@example.com",
	}); err == nil {
		t.Fatalf("expected password validation error")
	}
}

func mustUUID(t *testing.T, value string) uuid.UUID {
	t.Helper()
	id, err := ParseUUID(value, "tenant_id")
	if err != nil {
		t.Fatalf("parse uuid: %v", err)
	}
	return id
}
