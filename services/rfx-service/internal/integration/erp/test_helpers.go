//go:build integration

package erp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/repository"
	"github.com/freight-platform/shared-go/integrationauth"
)

type testEnv struct {
	pool             *pgxpool.Pool
	principalRepo    *repository.IntegrationPrincipalRepository
	credentialRepo   *repository.IntegrationCredentialRepository
	scopeRepo        *repository.IntegrationScopeRepository
	mappingRepo      *repository.ReferenceMappingRepository
	analysisRepo     *repository.ImportAnalysisRepository
	idemRepo         *repository.IdempotencyRepository
	externalLinkRepo *repository.ExternalObjectLinkRepository
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		if os.Getenv("REQUIRE_TEST_DATABASE") == "1" || strings.EqualFold(strings.TrimSpace(os.Getenv("CI")), "true") {
			t.Fatal("TEST_DATABASE_URL is required in CI")
		}
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	dbName, testURL, dropDB, err := createTempDatabase(ctx, adminURL)
	if err != nil {
		t.Fatalf("isolated postgres unavailable: %v", err)
	}
	t.Logf("isolated database=%s", dbName)
	pool, err := pgxpool.New(ctx, testURL)
	if err != nil {
		dropDB(context.Background())
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		dropDB(context.Background())
	})
	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	return &testEnv{
		pool:             pool,
		principalRepo:    repository.NewIntegrationPrincipalRepository(pool),
		credentialRepo:   repository.NewIntegrationCredentialRepository(pool),
		scopeRepo:        repository.NewIntegrationScopeRepository(pool),
		mappingRepo:      repository.NewReferenceMappingRepository(pool),
		analysisRepo:     repository.NewImportAnalysisRepository(pool),
		idemRepo:         repository.NewIdempotencyRepository(pool),
		externalLinkRepo: repository.NewExternalObjectLinkRepository(pool),
	}
}

func seedTenantCompany(t *testing.T, env *testEnv) (tenantID, companyID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	tenantID = uuid.New()
	companyID = uuid.New()
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1, $2, $3)`,
		tenantID, "erp-"+tenantID.String()[:8], "ERP Test Tenant"); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status)
		VALUES ($1, $2, 'SHIPPER', 'ERP Test Company', 'ACTIVE')
	`, companyID, tenantID); err != nil {
		t.Fatalf("seed company: %v", err)
	}
	return tenantID, companyID
}

func seedOAuthCredential(t *testing.T, env *testEnv, tenantID uuid.UUID, principal *domain.IntegrationPrincipal, secret string) {
	t.Helper()
	hash, err := integrationauth.HashSecret(secret)
	if err != nil {
		t.Fatalf("hash secret: %v", err)
	}
	if _, err := env.credentialRepo.StoreHash(context.Background(), domain.IntegrationCredential{
		TenantID:               tenantID,
		IntegrationPrincipalID: principal.ID,
		CredentialType:         domain.IntegrationCredentialTypeOAuth,
		SecretHash:             hash,
		HashAlgorithm:          integrationauth.HashAlgorithmBcrypt,
		HashVersion:            1,
		LookupFingerprint:      "oauth-" + principal.ClientID,
	}); err != nil {
		t.Fatalf("store oauth credential: %v", err)
	}
}

func seedAPIKeyCredential(t *testing.T, env *testEnv, tenantID uuid.UUID, principal *domain.IntegrationPrincipal, apiKey string) {
	t.Helper()
	hash, err := integrationauth.HashSecret(apiKey)
	if err != nil {
		t.Fatalf("hash api key: %v", err)
	}
	if _, err := env.credentialRepo.StoreHash(context.Background(), domain.IntegrationCredential{
		TenantID:               tenantID,
		IntegrationPrincipalID: principal.ID,
		CredentialType:         domain.IntegrationCredentialTypeAPIKey,
		SecretHash:             hash,
		HashAlgorithm:          integrationauth.HashAlgorithmBcrypt,
		HashVersion:            1,
		LookupFingerprint:      integrationauth.APIKeyLookupFingerprint(apiKey),
	}); err != nil {
		t.Fatalf("store api key credential: %v", err)
	}
}

func seedIntegrationPrincipal(t *testing.T, env *testEnv, tenantID, companyID uuid.UUID, clientID string) *domain.IntegrationPrincipal {
	t.Helper()
	principal, err := env.principalRepo.Create(context.Background(), domain.IntegrationPrincipal{
		TenantID:       tenantID,
		CompanyID:      companyID,
		ClientID:       clientID,
		CredentialType: domain.IntegrationCredentialTypeOAuth,
		Status:         domain.IntegrationPrincipalStatusActive,
	})
	if err != nil {
		t.Fatalf("create principal: %v", err)
	}
	return principal
}

func seedRfxEvent(t *testing.T, env *testEnv, tenantID, companyID uuid.UUID) uuid.UUID {
	t.Helper()
	eventID := uuid.New()
	_, err := env.pool.Exec(context.Background(), `
		INSERT INTO rfx.rfx_events (
			id, tenant_id, rfx_number, rfx_type, category, title, owner_company_id, status
		) VALUES ($1, $2, $3, 'SPOT_RFQ', 'FREIGHT', 'ERP Event', $4, 'DRAFT')
	`, eventID, tenantID, "ERP-"+eventID.String()[:8], companyID)
	if err != nil {
		t.Fatalf("seed event: %v", err)
	}
	return eventID
}

func createTempDatabase(ctx context.Context, adminURL string) (dbName string, testURL string, cleanup func(context.Context), err error) {
	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		return "", "", nil, fmt.Errorf("parse database url: %w", err)
	}
	dbName = "rfx_erp_e1_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	adminCfg := cfg.Copy()
	adminCfg.ConnConfig.Database = "postgres"
	adminPool, err := pgxpool.NewWithConfig(ctx, adminCfg)
	if err != nil {
		return "", "", nil, err
	}
	defer adminPool.Close()
	if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize()); err != nil {
		return "", "", nil, err
	}
	testCfg := cfg.Copy()
	testCfg.ConnConfig.Database = dbName
	testURL = buildDSN(testCfg)
	cleanup = func(cctx context.Context) {
		cadmin, cerr := pgxpool.NewWithConfig(cctx, adminCfg)
		if cerr != nil {
			return
		}
		defer cadmin.Close()
		_, _ = cadmin.Exec(cctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize()+" WITH (FORCE)")
	}
	return dbName, testURL, cleanup, nil
}

func connectPool(ctx context.Context, testURL string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, testURL)
}

func buildDSN(cfg *pgxpool.Config) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		url.QueryEscape(cfg.ConnConfig.User),
		url.QueryEscape(cfg.ConnConfig.Password),
		cfg.ConnConfig.Host, cfg.ConnConfig.Port, cfg.ConnConfig.Database)
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	migrationsDir, err := locateMigrationsDir()
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		content, readErr := os.ReadFile(file)
		if readErr != nil {
			return readErr
		}
		if _, execErr := pool.Exec(ctx, string(content)); execErr != nil {
			return fmt.Errorf("migration %s: %w", filepath.Base(file), execErr)
		}
	}
	return nil
}

func applyMigrationFile(ctx context.Context, pool *pgxpool.Pool, filename string) error {
	migrationsDir, err := locateMigrationsDir()
	if err != nil {
		return err
	}
	content, err := os.ReadFile(filepath.Join(migrationsDir, filename))
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, string(content))
	return err
}

func locateMigrationsDir() (string, error) {
	candidates := []string{
		filepath.Join("..", "..", "..", "..", "infrastructure", "migrations"),
		filepath.Join("..", "..", "..", "..", "..", "infrastructure", "migrations"),
	}
	for _, candidate := range candidates {
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate, nil
		}
	}
	wd, _ := os.Getwd()
	return "", fmt.Errorf("migrations dir not found from %s", wd)
}

func canonicalHashPayload(payload []byte) string {
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}
