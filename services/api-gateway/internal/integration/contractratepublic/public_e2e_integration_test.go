//go:build integration

package contractratepublic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	crconfig "github.com/freight-platform/contract-rate-service/internal/config"
	crhttp "github.com/freight-platform/contract-rate-service/internal/http"
	"github.com/freight-platform/contract-rate-service/internal/http/handlers"
	"github.com/freight-platform/contract-rate-service/internal/repository"
	"github.com/freight-platform/contract-rate-service/internal/service"

	gwconfig "github.com/freight-platform/api-gateway/internal/config"
	gwhttp "github.com/freight-platform/api-gateway/internal/http"
)

const internalToken = "integration-internal-token"
const maxMigrationNumber = 53

type harness struct {
	pool      *pgxpool.Pool
	tenantID  uuid.UUID
	userID    uuid.UUID
	buyerID   uuid.UUID
	carrierID uuid.UUID
	gateway   http.Handler
	jwtSecret string
}

type membership struct {
	companyID   uuid.UUID
	companyType string
	roles       []string
}

func TestPublicE2E001BuyerCreateContract(t *testing.T) {
	h := newHarness(t)
	token := h.signToken(t)
	body := fmt.Sprintf(`{
		"buyer_company_id":"%s",
		"carrier_company_id":"%s",
		"contract_number":"PUB-E2E-001",
		"name":"Public E2E Contract",
		"valid_from":"2026-01-01",
		"currency_code":"RUB"
	}`, h.buyerID, h.carrierID)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transport-contracts", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Company-ID", h.buyerID.String())
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	h.gateway.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("E-E2E-001 create expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPublicE2E006CrossCompanyRoleBleed(t *testing.T) {
	pool := setupPool(t)
	ctx := context.Background()
	tenantID := uuid.New()
	adminUser := uuid.New()
	logistUser := uuid.New()
	buyerID := uuid.New()
	carrierID := uuid.New()
	seedTenantAndCompanies(t, ctx, pool, tenantID, buyerID, carrierID)
	seedCompanyRole(t, ctx, pool, tenantID, adminUser, buyerID, "SHIPPER_ADMIN")
	seedCompanyRole(t, ctx, pool, tenantID, logistUser, carrierID, "SHIPPER_LOGIST")

	h := newHarnessWithIdentity(t, pool, tenantID, logistUser, buyerID, carrierID, []membership{
		{companyID: buyerID, companyType: "SHIPPER", roles: []string{"SHIPPER_ADMIN"}},
		{companyID: carrierID, companyType: "SHIPPER", roles: []string{"SHIPPER_LOGIST"}},
	})

	token := h.signTokenFor(logistUser)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/transport-contracts/"+uuid.New().String(), strings.NewReader(`{"description":"x"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Company-ID", carrierID.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.gateway.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("E-E2E-006 expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	pool := setupPool(t)
	ctx := context.Background()
	tenantID := uuid.New()
	userID := uuid.New()
	buyerID := uuid.New()
	carrierID := uuid.New()
	seedTenantAndCompanies(t, ctx, pool, tenantID, buyerID, carrierID)
	seedCompanyRole(t, ctx, pool, tenantID, userID, buyerID, "PROCUREMENT_MANAGER")
	return newHarnessWithIdentity(t, pool, tenantID, userID, buyerID, carrierID, []membership{
		{companyID: buyerID, companyType: "SHIPPER", roles: []string{"PROCUREMENT_MANAGER"}},
	})
}

func newHarnessWithIdentity(t *testing.T, pool *pgxpool.Pool, tenantID, userID, buyerID, carrierID uuid.UUID, memberships []membership) *harness {
	t.Helper()
	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/companies"):
			items := make([]map[string]any, 0, len(memberships))
			for _, m := range memberships {
				roles := make([]map[string]any, 0, len(m.roles))
				for _, role := range m.roles {
					roles = append(roles, map[string]any{"code": role})
				}
				items = append(items, map[string]any{
					"company_id":   m.companyID.String(),
					"company_type": m.companyType,
					"roles":        roles,
				})
			}
			writeJSON(w, map[string]any{"items": items})
		case strings.HasSuffix(r.URL.Path, "/roles"):
			writeJSON(w, map[string]any{"items": []any{}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(identityServer.Close)

	audit := repository.NewAuditRepository()
	contracts := repository.NewContractRepository(pool, audit)
	rateCards := repository.NewRateCardRepository(pool, contracts, audit)
	locations := repository.NewLocationRepository(pool)
	rateLines := repository.NewRateLineRepository(pool, rateCards, locations, audit)
	rateComponents := repository.NewRateComponentRepository(pool, rateLines, rateCards, audit)
	resolutions := repository.NewResolutionRepository(pool, audit)
	membershipsRepo := repository.NewMembershipRepository(pool)
	actors := handlers.NewActorResolver(membershipsRepo)

	contractSvc := service.NewContractService(contracts, membershipsRepo)
	rateCardSvc := service.NewRateCardService(rateCards, contracts)
	rateLineSvc := service.NewRateLineService(rateLines, rateCards, contracts)
	rateComponentSvc := service.NewRateComponentService(rateComponents, rateLines, rateCards, contracts)
	resolutionSvc := service.NewResolutionService(resolutions, membershipsRepo, nil, nil)

	crRouter := crhttp.NewRouter(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		pool,
		crconfig.Config{InternalServiceToken: internalToken, Environment: "test"},
		contractSvc, rateCardSvc, rateLineSvc, rateComponentSvc, resolutionSvc, actors,
	)
	contractRateServer := httptest.NewServer(crRouter)
	t.Cleanup(contractRateServer.Close)

	cfg := gwconfig.Config{
		AuthEnabled:          true,
		JWTSecret:            "integration-jwt-secret",
		ProxyTimeoutSeconds:  10,
		InternalServiceToken: internalToken,
		Services: gwconfig.ServiceURLs{
			Identity:     identityServer.URL,
			ContractRate: contractRateServer.URL,
		},
	}
	proxy, err := gwhttp.NewProxyHandler(cfg)
	if err != nil {
		t.Fatalf("proxy: %v", err)
	}
	router := gwhttp.NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), cfg, proxy, nil, nil, nil, nil)
	return &harness{
		pool: pool, tenantID: tenantID, userID: userID, buyerID: buyerID, carrierID: carrierID,
		gateway: router, jwtSecret: cfg.JWTSecret,
	}
}

func (h *harness) signToken(t *testing.T) string {
	return h.signTokenFor(h.userID)
}

func (h *harness) signTokenFor(userID uuid.UUID) string {
	claims := jwt.MapClaims{
		"tenant_id": h.tenantID.String(),
		"email":     "user@example.test",
		"sub":       userID.String(),
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(h.jwtSecret))
	return signed
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func setupPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		if os.Getenv("REQUIRE_TEST_DATABASE") == "1" || strings.EqualFold(strings.TrimSpace(os.Getenv("CI")), "true") {
			t.Fatal("TEST_DATABASE_URL is required")
		}
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, cleanup := createTempDB(t, ctx, adminURL)
	t.Cleanup(cleanup)
	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	return pool
}

func createTempDB(t *testing.T, ctx context.Context, adminURL string) (*pgxpool.Pool, func()) {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	dbName := "freight_contract_rate_public_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	adminCfg := cfg.Copy()
	adminCfg.ConnConfig.Database = "postgres"
	adminPool, err := pgxpool.NewWithConfig(ctx, adminCfg)
	if err != nil {
		t.Fatalf("admin pool: %v", err)
	}
	if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+dbName); err != nil {
		adminPool.Close()
		t.Fatalf("create db: %v", err)
	}
	adminPool.Close()

	testCfg := cfg.Copy()
	testCfg.ConnConfig.Database = dbName
	pool, err := pgxpool.NewWithConfig(ctx, testCfg)
	if err != nil {
		t.Fatalf("test pool: %v", err)
	}
	cleanup := func() {
		pool.Close()
		adminPool, _ = pgxpool.NewWithConfig(context.Background(), adminCfg)
		if adminPool != nil {
			_, _ = adminPool.Exec(context.Background(), "DROP DATABASE IF EXISTS "+dbName+" WITH (FORCE)")
			adminPool.Close()
		}
	}
	return pool, cleanup
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	root, err := repoRoot()
	if err != nil {
		return err
	}
	dir := filepath.Join(root, "infrastructure", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var files []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		var num int
		if _, err := fmt.Sscanf(name, "%d", &num); err != nil || num > maxMigrationNumber {
			continue
		}
		files = append(files, filepath.Join(dir, name))
	}
	sort.Strings(files)
	for _, file := range files {
		sqlBytes, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
	}
	return nil
}

func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "infrastructure", "migrations")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("repo root not found")
		}
		dir = parent
	}
}

func seedTenantAndCompanies(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, buyerID, carrierID uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(ctx, `
		INSERT INTO core.tenants (id, code, name, status)
		VALUES ($1, $2, 'Test Tenant', 'ACTIVE')
		ON CONFLICT DO NOTHING`, tenantID, "T-"+tenantID.String()[:8])
	if err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	for _, row := range []struct {
		id, tenant uuid.UUID
		typ, name  string
	}{
		{buyerID, tenantID, "SHIPPER", "Buyer Co"},
		{carrierID, tenantID, "CARRIER", "Carrier Co"},
	} {
		_, err := pool.Exec(ctx, `
			INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status)
			VALUES ($1,$2,$3,$4,'ACTIVE')
			ON CONFLICT DO NOTHING`, row.id, row.tenant, row.typ, row.name)
		if err != nil {
			t.Fatalf("seed company: %v", err)
		}
	}
}

func seedUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, userID uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(ctx, `
		INSERT INTO core.users (id, tenant_id, email, full_name, status)
		VALUES ($1,$2,$3,'Test User','ACTIVE')
		ON CONFLICT DO NOTHING`, userID, tenantID, userID.String()+"@example.test")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

func seedCompanyMembership(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, userID, companyID uuid.UUID) {
	t.Helper()
	seedUser(t, ctx, pool, tenantID, userID)
	_, err := pool.Exec(ctx, `
		INSERT INTO core.company_memberships (tenant_id, company_id, user_id, status)
		VALUES ($1,$2,$3,'ACTIVE')
		ON CONFLICT (company_id, user_id) DO UPDATE SET status='ACTIVE', deleted_at=NULL`,
		tenantID, companyID, userID)
	if err != nil {
		t.Fatalf("seed company membership: %v", err)
	}
}

func resolveRoleID(t *testing.T, ctx context.Context, pool *pgxpool.Pool, roleCode string) uuid.UUID {
	t.Helper()
	var roleID uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO core.roles (code, name, scope, is_system)
		VALUES ($1,$1,'GLOBAL',true)
		ON CONFLICT DO NOTHING
		RETURNING id`, roleCode).Scan(&roleID)
	if err != nil {
		err = pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE code=$1 AND tenant_id IS NULL LIMIT 1`, roleCode).Scan(&roleID)
		if err != nil {
			t.Fatalf("resolve role %s: %v", roleCode, err)
		}
	}
	return roleID
}

func seedCompanyRole(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, userID, companyID uuid.UUID, roleCode string) {
	t.Helper()
	seedCompanyMembership(t, ctx, pool, tenantID, userID, companyID)
	roleID := resolveRoleID(t, ctx, pool, roleCode)
	_, err := pool.Exec(ctx, `
		INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT DO NOTHING`, tenantID, userID, companyID, roleID)
	if err != nil {
		t.Fatalf("seed company role: %v", err)
	}
}
