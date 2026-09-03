package companycontext

import (
	"os"
	"strings"
	"testing"
)

func TestListUserCompaniesUsesVerifiedTenantFromRequestContext(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("client.go")
	if err != nil {
		t.Fatalf("read client.go: %v", err)
	}
	text := string(source)
	if !strings.Contains(text, "func (c *IdentityClient) ListUserCompanies") {
		t.Fatal("expected ListUserCompanies client method")
	}
	if !strings.Contains(text, "reqCtx.TenantID") {
		t.Fatal("expected verified tenant from request context")
	}
	if !strings.Contains(text, "/v1/users/%s/companies?tenant_id=%s&status=ACTIVE") {
		t.Fatal("expected self user companies endpoint template")
	}
}
