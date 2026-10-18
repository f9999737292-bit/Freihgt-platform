package repository

import (
	"os"
	"strings"
	"testing"

	"github.com/freight-platform/identity-service/internal/domain"
)

// IUSR-0.1B-001: a surviving tenant-global role must not grant authority unless
// the user row exists in the verified tenant, is not deleted, and has the
// schema's existing active status.
func TestLookupPrincipalRequiresActiveUndeletedUser(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("authority_repository.go")
	if err != nil {
		t.Fatalf("read authority_repository.go: %v", err)
	}
	text := string(source)
	start := strings.Index(text, "func (r *AuthorityRepository) LookupPrincipal")
	end := strings.Index(text, "func (r *AuthorityRepository) TenantExists")
	if start < 0 || end < start {
		t.Fatal("LookupPrincipal source was not found")
	}
	method := text[start:end]

	required := []string{
		"INNER JOIN core.users u",
		"u.id = ur.user_id",
		"u.tenant_id = ur.tenant_id",
		"u.deleted_at IS NULL",
		"u.status = $3",
		"domain.UserStatusActive",
		"ur.user_id = $1",
		"ur.tenant_id = $2",
		"ur.company_id IS NULL",
		"r.deleted_at IS NULL",
	}
	for _, fragment := range required {
		if !strings.Contains(method, fragment) {
			t.Fatalf("LookupPrincipal missing %q", fragment)
		}
	}
	if strings.Contains(method, "FROM core.user_roles ur\n\t\tINNER JOIN core.roles r ON r.id = ur.role_id\n\t\tWHERE") {
		t.Fatal("role assignment is still selected without the user predicate")
	}
	if domain.UserStatusActive != "ACTIVE" {
		t.Fatalf("active user status = %s, schema active status is ACTIVE", domain.UserStatusActive)
	}
	for _, inactive := range []string{"INVITED", "BLOCKED", "DISABLED", "DELETED"} {
		if strings.Contains(method, "'"+inactive+"'") || strings.Contains(method, `"`+inactive+`"`) {
			t.Fatalf("LookupPrincipal treats %s as an authority status", inactive)
		}
	}
}
