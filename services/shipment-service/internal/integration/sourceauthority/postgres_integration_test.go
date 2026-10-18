//go:build integration

package sourceauthority

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/shipment-service/internal/domain"
	shipmenthttp "github.com/freight-platform/shipment-service/internal/http"
	"github.com/freight-platform/shipment-service/internal/http/sourceauth"
	"github.com/freight-platform/shipment-service/internal/repository"
	"github.com/freight-platform/shipment-service/internal/service"
)

func TestShipmentSourceAuthorityPostgres(t *testing.T) {
	pool := setupPostgres(t)
	ctx := context.Background()
	var pgVersion, dbName string
	if err := pool.QueryRow(ctx, `SELECT current_setting('server_version'), current_database()`).Scan(&pgVersion, &dbName); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(dbName, "src_auth_") {
		t.Fatalf("refusing non-isolated database %s", dbName)
	}
	t.Logf("POSTGRES_VERSION=%s", pgVersion)
	t.Logf("TEST_DATABASE_ISOLATED=%s", dbName)

	fix := seedSource(t, pool)
	repo := repository.NewShipmentSourceAuthorityRepository(pool)
	authority := service.NewShipmentSourceAuthority(repo)
	before := sideCounts(t, pool)

	subject := subjectFor(t, pool, fix.ShipmentID, fix.ShipperAdmin)
	if got := authority.Confirm(ctx, subject); got != domain.ShipmentSourceCreateConfirmed {
		t.Fatalf("confirm existing = %s", got)
	}
	if got := authority.Revalidate(ctx, snapshotOf(subject)); got != domain.ShipmentSourceMatch {
		t.Fatalf("revalidate existing = %s", got)
	}
	if sideCounts(t, pool) != before {
		t.Fatalf("confirm wrote rows: before=%s after=%s", before, sideCounts(t, pool))
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE transport.shipments SET version = version + 1 WHERE id = $1`, fix.ShipmentID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if got := authority.Revalidate(ctx, snapshotOf(subject)); got != domain.ShipmentSourceMatch {
		t.Fatalf("rolled-back version change remained visible: %s", got)
	}

	if _, err := pool.Exec(ctx, `UPDATE transport.shipments SET version = version + 1 WHERE id = $1`, fix.ShipmentID); err != nil {
		t.Fatal(err)
	}
	if got := authority.Revalidate(ctx, snapshotOf(subject)); got != domain.ShipmentSourceChanged {
		t.Fatalf("committed version change = %s", got)
	}
	if got := authority.Confirm(ctx, subject); got != domain.ShipmentSourceCreateStale {
		t.Fatalf("stale confirm = %s", got)
	}
	current := subjectFor(t, pool, fix.ShipmentID, fix.ShipperAdmin)
	if got := authority.Confirm(ctx, current); got != domain.ShipmentSourceCreateConfirmed {
		t.Fatalf("confirm current fingerprint = %s", got)
	}
	if current.Fingerprint == subject.Fingerprint {
		t.Fatal("version change did not change the fingerprint")
	}

	if _, err := pool.Exec(ctx, `UPDATE transport.shipments SET deleted_at = clock_timestamp() WHERE id = $1`, fix.ShipmentID); err != nil {
		t.Fatal(err)
	}
	if got := authority.Confirm(ctx, current); got != domain.ShipmentSourceCreateNotFound {
		t.Fatalf("deleted confirm = %s", got)
	}
	if got := authority.Revalidate(ctx, snapshotOf(current)); got != domain.ShipmentSourceChanged {
		t.Fatalf("deleted revalidate = %s", got)
	}
	if _, err := pool.Exec(ctx, `UPDATE transport.shipments SET deleted_at = NULL WHERE id = $1`, fix.ShipmentID); err != nil {
		t.Fatal(err)
	}

	if _, err := pool.Exec(ctx, `UPDATE transport.shipments SET carrier_company_id = NULL WHERE id = $1`, fix.ShipmentID); err != nil {
		t.Fatal(err)
	}
	unassigned := current
	unassigned.Fingerprint = current.Fingerprint
	if got := authority.Confirm(ctx, unassigned); got != domain.ShipmentSourceCreateNotFound {
		t.Fatalf("unassigned confirm = %s", got)
	}
	if got := authority.Revalidate(ctx, snapshotOf(current)); got != domain.ShipmentSourceChanged {
		t.Fatalf("unassigned revalidate = %s", got)
	}
	if _, err := pool.Exec(ctx, `UPDATE transport.shipments SET carrier_company_id = $2 WHERE id = $1`, fix.ShipmentID, fix.CarrierID); err != nil {
		t.Fatal(err)
	}

	missing := current
	missing.SourceID = uuid.New()
	if got := authority.Confirm(ctx, missing); got != domain.ShipmentSourceCreateNotFound {
		t.Fatalf("missing confirm = %s", got)
	}
	if got := authority.Revalidate(ctx, snapshotOf(missing)); got != domain.ShipmentSourceNotFound {
		t.Fatalf("missing revalidate = %s", got)
	}

	live := subjectFor(t, pool, fix.ShipmentID, fix.ShipperAdmin)
	foreign := live
	foreign.SourceTenantID = fix.TenantB
	if got := authority.Confirm(ctx, foreign); got != domain.ShipmentSourceCreateTenantMismatch {
		t.Fatalf("foreign tenant confirm = %s", got)
	}
	if got := authority.Revalidate(ctx, snapshotOf(foreign)); got != domain.ShipmentSourceNotFound {
		t.Fatalf("foreign tenant revalidate = %s", got)
	}

	if _, err := pool.Exec(ctx, `UPDATE core.companies SET status = 'BLOCKED' WHERE id = $1`, fix.CarrierID); err != nil {
		t.Fatal(err)
	}
	if got := authority.Confirm(ctx, live); got != domain.ShipmentSourceCreateNotFound {
		t.Fatalf("inactive carrier confirm = %s", got)
	}
	if _, err := pool.Exec(ctx, `UPDATE core.companies SET status = 'ACTIVE' WHERE id = $1`, fix.CarrierID); err != nil {
		t.Fatal(err)
	}

	for _, userID := range []uuid.UUID{fix.BlockedUser, fix.DeletedUser, fix.CarrierAdmin, fix.PlatformNoMembership, fix.MemberNoRole} {
		attempt := subjectFor(t, pool, fix.ShipmentID, userID)
		if got := authority.Confirm(ctx, attempt); got != domain.ShipmentSourceCreateActorMismatch {
			t.Fatalf("actor %s confirm = %s", userID, got)
		}
	}
	for _, userID := range []uuid.UUID{fix.ShipperLogist, fix.ForwarderManager, fix.PlatformWithMembership} {
		attempt := subjectFor(t, pool, fix.ShipmentID, userID)
		if got := authority.Confirm(ctx, attempt); got != domain.ShipmentSourceCreateConfirmed {
			t.Fatalf("authorized actor %s confirm = %s", userID, got)
		}
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if got := authority.Confirm(cancelled, live); got != domain.ShipmentSourceCreateUnavailable {
		t.Fatalf("database error confirm = %s", got)
	}
	if got := authority.Revalidate(cancelled, snapshotOf(live)); got != domain.ShipmentSourceUnavailable {
		t.Fatalf("database error revalidate = %s", got)
	}
	if sideCounts(t, pool) != before {
		t.Fatalf("failed authority calls wrote rows: before=%s after=%s", before, sideCounts(t, pool))
	}

	const token = "dedicated-revalidation-token"
	handler := shipmenthttp.WithShipmentSourceAuthority(http.NotFoundHandler(), token, authority)
	payload, _ := json.Marshal(map[string]string{
		"source_type": live.SourceType, "source_id": live.SourceID.String(), "source_tenant_id": live.SourceTenantID.String(),
		"source_version_or_fingerprint": live.Fingerprint, "shipper_carrier_company_id": live.ShipperCarrierCompanyID.String(),
		"requesting_company_id": live.RequestingCompanyID.String(), "requesting_user_id": live.RequestingUserID.String(),
	})
	ok := postAuthority(handler, domain.ShipmentSourceConfirmPath, payload, token)
	if ok.Code != http.StatusOK || !strings.Contains(ok.Body.String(), `"decision":"CONFIRMED"`) {
		t.Fatalf("http confirm = %d %s", ok.Code, ok.Body.String())
	}
	for _, presented := range []string{"", "internal-service-token", token + "-no"} {
		rec := postAuthority(handler, domain.ShipmentSourceConfirmPath, payload, presented)
		if rec.Code != http.StatusUnauthorized || rec.Body.Len() != 0 {
			t.Fatalf("credential %q = %d %s", presented, rec.Code, rec.Body.String())
		}
	}
	spoof := httptest.NewRequest(http.MethodPost, domain.ShipmentSourceConfirmPath, bytes.NewReader(payload))
	spoof.Header.Set("X-Internal-Service-Token", token)
	spoof.Header.Set("X-Internal-Service-Name", "company-service")
	spoofed := httptest.NewRecorder()
	handler.ServeHTTP(spoofed, spoof)
	if spoofed.Code != http.StatusUnauthorized {
		t.Fatalf("generic internal credential = %d", spoofed.Code)
	}
	if sourceauth.Header() != "X-Company-Relationship-Revalidation-Token" {
		t.Fatal("revalidation header drifted")
	}
	t.Log("POSTGRES_INTEGRATION=PASS")
}

type sourceFix struct {
	TenantA, TenantB, ShipperID, CarrierID, ShipmentID uuid.UUID
	ShipperAdmin, ShipperLogist, ForwarderManager      uuid.UUID
	CarrierAdmin, BlockedUser, DeletedUser             uuid.UUID
	PlatformNoMembership, PlatformWithMembership       uuid.UUID
	MemberNoRole                                       uuid.UUID
}

func subjectFor(t *testing.T, pool *pgxpool.Pool, shipmentID, userID uuid.UUID) domain.ShipmentSourceSubject {
	t.Helper()
	row, err := repository.NewShipmentSourceAuthorityRepository(pool).ReadShipmentSource(context.Background(), shipmentID)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, err := domain.ShipmentSourceFingerprint(row)
	if err != nil {
		t.Fatal(err)
	}
	return domain.ShipmentSourceSubject{
		SourceType: domain.ShipmentSourceType, SourceID: row.ID, SourceTenantID: row.TenantID,
		Fingerprint: fingerprint, ShipperCarrierCompanyID: row.CarrierCompanyID,
		RequestingCompanyID: row.ShipperCompanyID, RequestingUserID: userID,
	}
}

func snapshotOf(subject domain.ShipmentSourceSubject) domain.ShipmentSourceSnapshot {
	return domain.ShipmentSourceSnapshot{
		SourceType: subject.SourceType, SourceID: subject.SourceID, SourceTenantID: subject.SourceTenantID,
		Fingerprint: subject.Fingerprint, ShipperCarrierCompanyID: subject.ShipperCarrierCompanyID,
	}
}

func sideCounts(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var shipments, requests, relationships, events int
	err := pool.QueryRow(context.Background(), `
		SELECT
			(SELECT count(*) FROM transport.shipments),
			(SELECT count(*) FROM company.carrier_relationship_requests),
			(SELECT count(*) FROM company.carrier_relationships),
			(SELECT count(*) FROM company.carrier_relationship_request_events)
	`).Scan(&shipments, &requests, &relationships, &events)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("shipments=%d requests=%d relationships=%d events=%d", shipments, requests, relationships, events)
}

func seedSource(t *testing.T, pool *pgxpool.Pool) sourceFix {
	t.Helper()
	ctx := context.Background()
	fix := sourceFix{
		TenantA: uuid.New(), TenantB: uuid.New(), ShipperID: uuid.New(), CarrierID: uuid.New(), ShipmentID: uuid.New(),
		ShipperAdmin: uuid.New(), ShipperLogist: uuid.New(), ForwarderManager: uuid.New(), CarrierAdmin: uuid.New(),
		BlockedUser: uuid.New(), DeletedUser: uuid.New(), PlatformNoMembership: uuid.New(), PlatformWithMembership: uuid.New(),
		MemberNoRole: uuid.New(),
	}
	for _, row := range []struct {
		id   uuid.UUID
		code string
	}{{fix.TenantA, "a"}, {fix.TenantB, "b"}} {
		if _, err := pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1, $2, $3)`, row.id, row.code+"-"+row.id.String()[:8], row.code); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status) VALUES ($1,$2,'SHIPPER',$3,'ACTIVE'), ($4,$2,'CARRIER',$5,'ACTIVE')`,
		fix.ShipperID, fix.TenantA, fix.ShipperID.String(), fix.CarrierID, fix.CarrierID.String()); err != nil {
		t.Fatal(err)
	}
	insertUser(t, pool, fix.ShipperAdmin, fix.TenantA, "ACTIVE", false)
	insertUser(t, pool, fix.ShipperLogist, fix.TenantA, "ACTIVE", false)
	insertUser(t, pool, fix.ForwarderManager, fix.TenantA, "ACTIVE", false)
	insertUser(t, pool, fix.CarrierAdmin, fix.TenantA, "ACTIVE", false)
	insertUser(t, pool, fix.BlockedUser, fix.TenantA, "BLOCKED", false)
	insertUser(t, pool, fix.DeletedUser, fix.TenantA, "ACTIVE", true)
	insertUser(t, pool, fix.PlatformNoMembership, fix.TenantA, "ACTIVE", false)
	insertUser(t, pool, fix.PlatformWithMembership, fix.TenantA, "ACTIVE", false)
	insertUser(t, pool, fix.MemberNoRole, fix.TenantA, "ACTIVE", false)
	for _, userID := range []uuid.UUID{fix.ShipperAdmin, fix.ShipperLogist, fix.ForwarderManager, fix.CarrierAdmin, fix.BlockedUser, fix.PlatformWithMembership, fix.MemberNoRole} {
		if _, err := pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id, status) VALUES ($1,$2,$3,'ACTIVE')`, fix.TenantA, fix.ShipperID, userID); err != nil {
			t.Fatal(err)
		}
	}
	grantRole(t, pool, fix.TenantA, fix.ShipperAdmin, &fix.ShipperID, "SHIPPER_ADMIN")
	grantRole(t, pool, fix.TenantA, fix.ShipperLogist, &fix.ShipperID, "SHIPPER_LOGIST")
	grantRole(t, pool, fix.TenantA, fix.ForwarderManager, &fix.ShipperID, "FORWARDER_MANAGER")
	grantRole(t, pool, fix.TenantA, fix.CarrierAdmin, &fix.ShipperID, "CARRIER_ADMIN")
	grantRole(t, pool, fix.TenantA, fix.BlockedUser, &fix.ShipperID, "SHIPPER_ADMIN")
	grantRole(t, pool, fix.TenantA, fix.PlatformNoMembership, nil, "PLATFORM_ADMIN")
	grantRole(t, pool, fix.TenantA, fix.PlatformWithMembership, nil, "PLATFORM_ADMIN")

	originID, destID := uuid.New(), uuid.New()
	for _, row := range []struct {
		id, name string
		value    uuid.UUID
	}{{originID.String(), "Origin", originID}, {destID.String(), "Destination", destID}} {
		if _, err := pool.Exec(ctx, `INSERT INTO transport.locations (id, tenant_id, location_type, name, country_code) VALUES ($1,$2,'WAREHOUSE',$3,'RU')`, row.value, fix.TenantA, row.name); err != nil {
			t.Fatal(err)
		}
	}
	orderID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO transport.transport_orders (
			id, tenant_id, order_number, status, shipper_company_id, consignee_company_id, origin_location_id, destination_location_id, transport_mode
		) VALUES ($1,$2,$3,'ASSIGNED',$4,$4,$5,$6,'ROAD')
	`, orderID, fix.TenantA, "TO-"+orderID.String()[:8], fix.ShipperID, originID, destID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO transport.shipments (
			id, tenant_id, shipment_number, transport_order_id, shipper_company_id, consignee_company_id,
			carrier_company_id, origin_location_id, destination_location_id, transport_mode, status, version
		) VALUES ($1,$2,$3,$4,$5,$5,$6,$7,$8,'ROAD','CARRIER_ASSIGNED',4)
	`, fix.ShipmentID, fix.TenantA, "SHP-"+fix.ShipmentID.String()[:8], orderID, fix.ShipperID, fix.CarrierID, originID, destID); err != nil {
		t.Fatal(err)
	}
	return fix
}

func insertUser(t *testing.T, pool *pgxpool.Pool, id, tenant uuid.UUID, status string, deleted bool) {
	t.Helper()
	var deletedAt any
	if deleted {
		deletedAt = time.Now().UTC()
	}
	_, err := pool.Exec(context.Background(), `
		INSERT INTO core.users (id, tenant_id, email, full_name, status, deleted_at)
		VALUES ($1, $2, $3, 'User', $4, $5)
	`, id, tenant, id.String()+"@example.test", status, deletedAt)
	if err != nil {
		t.Fatal(err)
	}
}

func grantRole(t *testing.T, pool *pgxpool.Pool, tenant, user uuid.UUID, company *uuid.UUID, code string) {
	t.Helper()
	var roleID uuid.UUID
	if err := pool.QueryRow(context.Background(), `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = $1 AND deleted_at IS NULL`, code).Scan(&roleID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1,$2,$3,$4)`, tenant, user, company, roleID); err != nil {
		t.Fatal(err)
	}
}

func postAuthority(handler http.Handler, path string, payload []byte, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	if token != "" {
		req.Header.Set(sourceauth.Header(), token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func setupPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	t.Cleanup(cancel)
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		adminURL = startEmbeddedPostgres(t)
	}
	dbName := "src_auth_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	testURL, cleanup := createDatabase(t, ctx, adminURL, dbName)
	t.Cleanup(cleanup)
	pool, err := pgxpool.New(ctx, testURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

func startEmbeddedPostgres(t *testing.T) string {
	t.Helper()
	port := freePort(t)
	pg := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Port(uint32(port)).
		RuntimePath(filepath.Join(t.TempDir(), "pg-runtime")).
		DataPath(filepath.Join(t.TempDir(), "pg-data")).
		Database("postgres").
		Username("freight").
		Password("freight").
		Version(embeddedpostgres.V16))
	if err := pg.Start(); err != nil {
		t.Fatalf("start embedded postgres: %v", err)
	}
	t.Cleanup(func() { _ = pg.Stop() })
	return fmt.Sprintf("postgres://freight:freight@127.0.0.1:%d/postgres?sslmode=disable", port)
}

func createDatabase(t *testing.T, ctx context.Context, adminURL, dbName string) (string, func()) {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	adminDB := cfg.ConnConfig.Database
	if adminDB == "" {
		adminDB = "postgres"
	}
	adminCfg := cfg.Copy()
	adminCfg.ConnConfig.Database = adminDB
	admin, err := pgxpool.NewWithConfig(ctx, adminCfg)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	testCfg := cfg.Copy()
	testCfg.ConnConfig.Database = dbName
	testURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		testCfg.ConnConfig.User, testCfg.ConnConfig.Password, testCfg.ConnConfig.Host, testCfg.ConnConfig.Port, dbName)
	return testURL, func() {
		cadmin, err := pgxpool.NewWithConfig(context.Background(), adminCfg)
		if err != nil {
			return
		}
		defer cadmin.Close()
		_, _ = cadmin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize()+" WITH (FORCE)")
	}
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	dir, err := migrationsDir()
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if _, execErr := pool.Exec(ctx, string(content)); execErr != nil {
			return fmt.Errorf("%s: %w", filepath.Base(file), execErr)
		}
	}
	return nil
}

func migrationsDir() (string, error) {
	candidate := filepath.Join("..", "..", "..", "..", "..", "infrastructure", "migrations")
	if st, err := os.Stat(candidate); err == nil && st.IsDir() {
		return candidate, nil
	}
	return "", os.ErrNotExist
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}
