package repository

import (
	"os"
	"strings"
	"testing"
)

func TestGetUserCompaniesQueryScopesTenantAndUser(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("membership_repository.go")
	if err != nil {
		t.Fatalf("read membership_repository.go: %v", err)
	}
	text := string(source)
	if !strings.Contains(text, "cm.user_id = $1 AND cm.tenant_id = $2") {
		t.Fatal("expected user and tenant scoped membership query")
	}
}
