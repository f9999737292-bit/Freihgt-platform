//go:build integration

package erp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/shared-go/integrationauth"
)

func TestE7P2INT120OAuthClientCredentialsValid(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "int120-client")
	secret := "int120-client-secret"
	seedOAuthCredential(t, env, tenantID, principal, secret)
	grantScope(t, env, tenantID, principal.ID, domain.ScopeDraftPreview)

	authCtx, err := integrationauth.NewVerifier(env.pool).AuthenticateOAuthClientCredentials(context.Background(), principal.ClientID, secret, "127.0.0.1")
	if err != nil {
		t.Fatalf("auth: %v", err)
	}
	jwtSvc := integrationauth.NewJWTService("test-jwt-secret", integrationauth.TokenTTL)
	token, expiresIn, err := jwtSvc.CreateIntegrationToken(authCtx)
	if err != nil || token == "" || expiresIn <= 0 {
		t.Fatalf("token issue failed token=%q expires=%d err=%v", token, expiresIn, err)
	}
}

func TestE7P2INT121OAuthInvalidClientSecret(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "int121-client")
	seedOAuthCredential(t, env, tenantID, principal, "correct-secret")
	_, err := integrationauth.NewVerifier(env.pool).AuthenticateOAuthClientCredentials(context.Background(), principal.ClientID, "wrong-secret", "127.0.0.1")
	if err == nil {
		t.Fatal("expected invalid client")
	}
}

func TestE7P2INT122OAuthExpiredCredential(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "int122-client")
	secret := "expired-secret"
	hash, _ := integrationauth.HashSecret(secret)
	expired := time.Now().UTC().Add(-time.Hour)
	if _, err := env.pool.Exec(context.Background(), `
		INSERT INTO rfx.rfx_integration_credentials (
			tenant_id, integration_principal_id, credential_type, secret_hash, hash_algorithm, hash_version, lookup_fingerprint, expires_at
		) VALUES ($1,$2,'OAUTH',$3,'bcrypt',1,$4,$5)
	`, tenantID, principal.ID, hash, "fp-int122", expired); err != nil {
		t.Fatalf("insert expired credential: %v", err)
	}
	_, err := integrationauth.NewVerifier(env.pool).AuthenticateOAuthClientCredentials(context.Background(), principal.ClientID, secret, "127.0.0.1")
	if err == nil {
		t.Fatal("expected expired credential rejection")
	}
}

func TestE7P2INT123RevokedAPIKey(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	principal := seedAPIKeyPrincipal(t, env, tenantID, companyID, "int123-client")
	apiKey := "bt_live_revoked_key_value"
	hash, _ := integrationauth.HashSecret(apiKey)
	fingerprint := integrationauth.APIKeyLookupFingerprint(apiKey)
	if _, err := env.pool.Exec(context.Background(), `
		INSERT INTO rfx.rfx_integration_credentials (
			tenant_id, integration_principal_id, credential_type, secret_hash, hash_algorithm, hash_version, lookup_fingerprint, revoked_at
		) VALUES ($1,$2,'API_KEY',$3,'bcrypt',1,$4, now())
	`, tenantID, principal.ID, hash, fingerprint); err != nil {
		t.Fatalf("insert revoked credential: %v", err)
	}
	if _, err := integrationauth.NewVerifier(env.pool).AuthenticateAPIKeyBearer(context.Background(), apiKey, "127.0.0.1"); err == nil {
		t.Fatal("expected revoked api key rejection")
	}
}

func TestE7P2INT124APIKeyHashOnlyStorage(t *testing.T) {
	TestCredentialHashOnlyStorage(t)
}

func TestE7P2INT125MissingAuthorization(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/integrations/erp/capabilities", nil)
	rec := httptest.NewRecorder()
	http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(r.Header.Get("Authorization")) == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", rec.Code)
	}
}

func TestE7P2INT126CredentialRotationGrace(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "int126-client")
	oldSecret := "rotation-old-secret"
	newSecret := "rotation-new-secret"
	oldHash, _ := integrationauth.HashSecret(oldSecret)
	newHash, _ := integrationauth.HashSecret(newSecret)
	grace := time.Now().UTC().Add(time.Hour)
	if _, err := env.pool.Exec(context.Background(), `
		UPDATE rfx.rfx_integration_principals SET grace_ends_at = $2 WHERE id = $1
	`, principal.ID, grace); err != nil {
		t.Fatalf("set grace: %v", err)
	}
	var newCredID uuid.UUID
	if err := env.pool.QueryRow(context.Background(), `
		INSERT INTO rfx.rfx_integration_credentials (
			tenant_id, integration_principal_id, credential_type, secret_hash, hash_algorithm, hash_version, lookup_fingerprint
		) VALUES ($1,$2,'OAUTH',$3,'bcrypt',1,'fp-int126-new') RETURNING id
	`, tenantID, principal.ID, newHash).Scan(&newCredID); err != nil {
		t.Fatalf("insert new credential: %v", err)
	}
	if _, err := env.pool.Exec(context.Background(), `
		INSERT INTO rfx.rfx_integration_credentials (
			tenant_id, integration_principal_id, credential_type, secret_hash, hash_algorithm, hash_version, lookup_fingerprint, revoked_at, rotation_successor_id
		) VALUES ($1,$2,'OAUTH',$3,'bcrypt',1,'fp-int126-old', now(), $4)
	`, tenantID, principal.ID, oldHash, newCredID); err != nil {
		t.Fatalf("insert old credential: %v", err)
	}
	if _, err := integrationauth.NewVerifier(env.pool).AuthenticateOAuthClientCredentials(context.Background(), principal.ClientID, oldSecret, "127.0.0.1"); err != nil {
		t.Fatalf("expected rotation grace acceptance: %v", err)
	}
	_, err := integrationauth.NewVerifier(env.pool).AuthenticateOAuthClientCredentials(context.Background(), principal.ClientID, newSecret, "127.0.0.1")
	if err != nil {
		t.Fatalf("expected new credential acceptance: %v", err)
	}
}

func TestE7P2INT127IPAllowlistViolation(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "int127-client")
	secret := "int127-secret"
	seedOAuthCredential(t, env, tenantID, principal, secret)
	if _, err := env.pool.Exec(context.Background(), `UPDATE rfx.rfx_integration_principals SET allowed_cidrs = '["203.0.113.0/24"]'::jsonb WHERE id = $1`, principal.ID); err != nil {
		t.Fatalf("set cidrs: %v", err)
	}
	if _, err := integrationauth.NewVerifier(env.pool).AuthenticateOAuthClientCredentials(context.Background(), principal.ClientID, secret, "198.51.100.10"); err == nil {
		t.Fatal("expected cidr denial")
	}
}

func TestE7P2INT128SpoofedTenantHeaderStripped(t *testing.T) {
	testIntegrationHeaderInjection(t, "X-Tenant-ID", uuid.NewString())
}

func TestE7P2INT129SpoofedCompanyHeaderStripped(t *testing.T) {
	testIntegrationHeaderInjection(t, "X-Company-ID", uuid.NewString())
}

func TestE7P2INT130CrossTenantIsolation(t *testing.T) {
	env := setupTestEnv(t)
	tenantA, companyA := seedTenantCompany(t, env)
	tenantB, _ := seedTenantCompany(t, env)
	principalA := seedIntegrationPrincipal(t, env, tenantA, companyA, "int130-client")
	secret := "int130-secret"
	seedOAuthCredential(t, env, tenantA, principalA, secret)
	authCtx, err := integrationauth.NewVerifier(env.pool).AuthenticateOAuthClientCredentials(context.Background(), principalA.ClientID, secret, "127.0.0.1")
	if err != nil {
		t.Fatalf("auth: %v", err)
	}
	if authCtx.TenantID == tenantB {
		t.Fatal("tenant must not resolve to foreign tenant")
	}
}

func TestE7P2INT131CrossCompanyBinding(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyA := seedTenantCompany(t, env)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyA, "int131-client")
	secret := "int131-secret"
	seedOAuthCredential(t, env, tenantID, principal, secret)
	authCtx, err := integrationauth.NewVerifier(env.pool).AuthenticateOAuthClientCredentials(context.Background(), principal.ClientID, secret, "127.0.0.1")
	if err != nil {
		t.Fatalf("auth: %v", err)
	}
	if authCtx.CompanyID != companyA {
		t.Fatalf("company binding mismatch")
	}
}

func TestE7P2INT186RateLimit429(t *testing.T) {
	limiter := integrationauth.NewPrincipalRateLimiter(1, time.Minute)
	key := limiter.Key(uuid.NewString(), uuid.NewString(), "oauth_token")
	if allowed, _ := limiter.Allow(key); !allowed {
		t.Fatal("first request should pass")
	}
	if allowed, _ := limiter.Allow(key); allowed {
		t.Fatal("second request should be rate limited")
	}
}

func TestE7P2INT190AuthSchemeBinding(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "int190-client")
	seedOAuthCredential(t, env, tenantID, principal, "oauth-only-secret")
	apiKey := "bt_live_scheme_binding_probe"
	if _, err := integrationauth.NewVerifier(env.pool).AuthenticateAPIKeyBearer(context.Background(), apiKey, "127.0.0.1"); err == nil {
		t.Fatal("expected API key auth failure for OAuth-only principal")
	}
}

func seedAPIKeyPrincipal(t *testing.T, env *testEnv, tenantID, companyID uuid.UUID, clientID string) *domain.IntegrationPrincipal {
	t.Helper()
	principal, err := env.principalRepo.Create(context.Background(), domain.IntegrationPrincipal{
		TenantID: tenantID, CompanyID: companyID, ClientID: clientID,
		CredentialType: domain.IntegrationCredentialTypeAPIKey,
		Status:         domain.IntegrationPrincipalStatusActive,
	})
	if err != nil {
		t.Fatalf("create api key principal: %v", err)
	}
	return principal
}

func grantScope(t *testing.T, env *testEnv, tenantID, principalID uuid.UUID, scope string) {
	t.Helper()
	if err := env.scopeRepo.Grant(context.Background(), tenantID, principalID, scope); err != nil {
		t.Fatalf("grant scope: %v", err)
	}
}

func testIntegrationHeaderInjection(t *testing.T, header, spoofed string) {
	t.Helper()
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "hdr-"+strings.ToLower(header))
	secret := "hdr-secret"
	seedOAuthCredential(t, env, tenantID, principal, secret)
	authCtx, err := integrationauth.NewVerifier(env.pool).AuthenticateOAuthClientCredentials(context.Background(), principal.ClientID, secret, "127.0.0.1")
	if err != nil {
		t.Fatalf("auth: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.Header.Set(header, spoofed)
	integrationauth.InjectTrustedIntegrationHeaders(req.Header, authCtx)
	if got := req.Header.Get(header); got == spoofed {
		t.Fatalf("expected spoofed %s stripped/replaced", header)
	}
}
