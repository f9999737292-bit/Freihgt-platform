//go:build integration

package relationship

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/company-service/internal/domain"
	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
	"github.com/freight-platform/company-service/internal/repository"
	"github.com/freight-platform/company-service/internal/service"
)

func TestCompanyRuntimeLoginFoundation(t *testing.T) {
	admin := setupPostgres(t)
	ctx := context.Background()
	before := snapshotRole(t, admin, "freight")
	if err := cycleRoleFoundationMigration(ctx, admin); err != nil {
		t.Fatal(err)
	}
	after := snapshotRole(t, admin, "freight")
	if before != after {
		t.Fatalf("freight role changed before=%+v after=%+v", before, after)
	}
	assertNoLoginRuntime(t, admin, "company_runtime")
	assertNoLoginRuntime(t, admin, "shipment_runtime")
	if triggerCount(t, admin) != 0 {
		t.Fatal("role foundation installed the enforcement trigger")
	}

	fix := seedFixture(t, admin)
	login := connectRuntimeLogin(t, admin, "company_login", "company_runtime")
	assertDedicatedSession(t, login, "company_login")

	companies := repository.NewCompanyRepository(login)
	memberships := repository.NewMembershipRepository(login)
	created := createCompanyAsLogin(t, companies, fix.TenantA)
	renamed := "Renamed " + created.ID.String()[:8]
	updated, err := companies.Update(ctx, fix.TenantA, created.ID, domain.UpdateCompanyInput{LegalName: &renamed})
	if err != nil || updated.LegalName != renamed {
		t.Fatalf("update company: %v %+v", err, updated)
	}
	if _, err := companies.GetByID(ctx, fix.TenantB, created.ID); err == nil {
		t.Fatal("company read crossed tenants")
	}
	var foreignName string
	if err := admin.QueryRow(ctx, `SELECT legal_name FROM core.companies WHERE id = $1`, created.ID).Scan(&foreignName); err != nil {
		t.Fatal(err)
	}
	if _, err := companies.Update(ctx, fix.TenantB, created.ID, domain.UpdateCompanyInput{LegalName: stringPtrLocal("other tenant")}); err == nil {
		t.Fatal("company update crossed tenants")
	}
	if foreignName != renamed {
		t.Fatalf("cross-tenant update changed the row to %s", foreignName)
	}
	var still string
	if err := admin.QueryRow(ctx, `SELECT legal_name FROM core.companies WHERE id = $1`, created.ID).Scan(&still); err != nil || still != renamed {
		t.Fatalf("company row after denied update=%s err=%v", still, err)
	}

	userID := uuid.New()
	insertUser(t, admin, userID, fix.TenantA, "ACTIVE", nil)
	position := "dispatcher"
	member, err := memberships.CreateMembership(ctx, domain.CreateMembershipInput{
		TenantID: fix.TenantA, CompanyID: created.ID, UserID: userID, Position: &position,
	})
	if err != nil || member.CompanyID != created.ID {
		t.Fatalf("create membership: %v %+v", err, member)
	}
	nextPosition := "lead"
	member, err = memberships.UpdateMembership(ctx, member.ID, created.ID, domain.UpdateMembershipInput{Position: &nextPosition})
	if err != nil || member.Position == nil || *member.Position != nextPosition {
		t.Fatalf("update membership: %v %+v", err, member)
	}
	var roleID uuid.UUID
	if err := login.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = 'SHIPPER_ADMIN' AND deleted_at IS NULL`).Scan(&roleID); err != nil {
		t.Fatal(err)
	}
	if err := memberships.AddUserRoleForCompany(ctx, fix.TenantA, userID, created.ID, roleID); err != nil {
		t.Fatalf("assign role: %v", err)
	}
	if err := memberships.RemoveUserRolesForCompany(ctx, fix.TenantA, userID, created.ID); err != nil {
		t.Fatalf("remove role: %v", err)
	}
	var left int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM core.user_roles WHERE user_id = $1 AND company_id = $2`, userID, created.ID).Scan(&left); err != nil || left != 0 {
		t.Fatalf("user roles left=%d err=%v", left, err)
	}

	expectSQLState(t, login, `UPDATE core.user_roles SET role_id = role_id WHERE false`, "42501")
	expectSQLState(t, login, `INSERT INTO core.tenants DEFAULT VALUES`, "42501")
	expectSQLState(t, login, `DELETE FROM core.companies WHERE false`, "42501")
	expectSQLState(t, login, `INSERT INTO company.carrier_relationship_activation_authorizations DEFAULT VALUES`, "42501")
	expectSQLState(t, login, `INSERT INTO company.carrier_relationship_activation_subjects DEFAULT VALUES`, "42501")
	expectSQLState(t, login, `DELETE FROM transport.transport_execution_carrier_relationship_pins WHERE false`, "42501")

	repo := repository.NewRelationshipRepository(login)
	cat := newCatalog()
	svc := service.NewRelationshipService(repo, repo, postgresRelationshipConfig(), matchSource{}, cat)
	pending := createOK(t, svc, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, "login-cancel", "login-cancel")
	cancelled, err := svc.Cancel(ctx, cancelInput(fix.ShipperAdmin, fix.TenantA, pending.SourceID, pending.RequestID, "login-cancel-do", "withdrawn"))
	if err != nil || cancelled.RequestID != pending.RequestID {
		t.Fatalf("cancel: %v %+v", err, cancelled)
	}
	relationshipID, accepted := acceptCarrier(t, svc, cat, fix, fix.CarrierA, "login-accept")
	revoked, err := svc.Revoke(ctx, revokeInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, relationshipID, "login-revoke", "ended"))
	if err != nil || revoked.RelationshipID != relationshipID {
		t.Fatalf("revoke: %v %+v", err, revoked)
	}
	assertRevokedHead(t, admin, relationshipID, revoked.RelationshipVersion)
	_, err = svc.Get(ctx, domain.Principal{UserID: fix.CarrierAdmin, TenantID: fix.TenantB}, fix.OperatingB2, accepted.RequestID)
	assertCode(t, err, apperrors.CodeRelationshipRequestNotFound)
	var audits, outbox int
	if err := admin.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM company.carrier_relationship_audit),
			(SELECT count(*) FROM company.carrier_relationship_outbox)
	`).Scan(&audits, &outbox); err != nil || audits == 0 || outbox == 0 {
		t.Fatalf("audit=%d outbox=%d err=%v", audits, outbox, err)
	}
}

func cycleRoleFoundationMigration(ctx context.Context, pool *pgxpool.Pool) error {
	if err := execMigration(ctx, pool, "000105_platform_postgres_role_foundation_v0_2_i1.down.sql"); err != nil {
		return fmt.Errorf("down: %w", err)
	}
	if companyInsertGranted(ctx, pool) {
		return errors.New("down left company insert")
	}
	if err := execMigration(ctx, pool, "000105_platform_postgres_role_foundation_v0_2_i1.up.sql"); err != nil {
		return fmt.Errorf("up after down: %w", err)
	}
	if err := execMigration(ctx, pool, "000105_platform_postgres_role_foundation_v0_2_i1.down.sql"); err != nil {
		return fmt.Errorf("second down: %w", err)
	}
	if err := execMigration(ctx, pool, "000105_platform_postgres_role_foundation_v0_2_i1.up.sql"); err != nil {
		return fmt.Errorf("second up: %w", err)
	}
	if !companyInsertGranted(ctx, pool) {
		return errors.New("up/down/up did not restore company insert")
	}
	return nil
}

func companyInsertGranted(ctx context.Context, pool *pgxpool.Pool) bool {
	var ok bool
	_ = pool.QueryRow(ctx, `SELECT has_table_privilege('company_runtime', 'core.companies', 'INSERT')`).Scan(&ok)
	return ok
}

type roleSnapshot struct {
	exists bool
	login  bool
	super  bool
	bypass bool
}

func snapshotRole(t *testing.T, pool *pgxpool.Pool, name string) roleSnapshot {
	t.Helper()
	var snap roleSnapshot
	err := pool.QueryRow(context.Background(), `
		SELECT rolcanlogin, rolsuper, rolbypassrls
		FROM pg_roles WHERE rolname = $1
	`, name).Scan(&snap.login, &snap.super, &snap.bypass)
	if errors.Is(err, pgx.ErrNoRows) {
		return roleSnapshot{}
	}
	if err != nil {
		t.Fatal(err)
	}
	snap.exists = true
	return snap
}

func assertNoLoginRuntime(t *testing.T, pool *pgxpool.Pool, name string) {
	t.Helper()
	snap := snapshotRole(t, pool, name)
	if !snap.exists || snap.login || snap.super || snap.bypass {
		t.Fatalf("%s attributes %+v", name, snap)
	}
}

func connectRuntimeLogin(t *testing.T, admin *pgxpool.Pool, loginName, runtimeName string) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	password := strings.ReplaceAll(uuid.NewString(), "-", "")
	var stmt string
	if err := admin.QueryRow(ctx, `
		SELECT format(
			'CREATE ROLE %I LOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD %L',
			$1::text, $2::text)
	`, loginName, password).Scan(&stmt); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, stmt); err != nil {
		t.Fatal(err)
	}
	grant := fmt.Sprintf("GRANT %s TO %s", runtimeName, loginName)
	if _, err := admin.Exec(ctx, grant); err != nil {
		_, _ = admin.Exec(ctx, "DROP ROLE "+loginName)
		t.Fatal(err)
	}
	var db string
	if err := admin.QueryRow(ctx, `SELECT current_database()`).Scan(&db); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO %s", quoteIdent(db), loginName)); err != nil {
		t.Fatal(err)
	}
	cfg := admin.Config().Copy()
	cfg.ConnConfig.User = loginName
	cfg.ConnConfig.Password = password
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP OWNED BY "+loginName)
		_, _ = admin.Exec(context.Background(), "DROP ROLE IF EXISTS "+loginName)
	})
	return pool
}

func quoteIdent(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func assertDedicatedSession(t *testing.T, pool *pgxpool.Pool, loginName string) {
	t.Helper()
	var current string
	var super, bypass bool
	if err := pool.QueryRow(context.Background(), `
		SELECT current_user, rolsuper, rolbypassrls
		FROM pg_roles WHERE rolname = current_user
	`).Scan(&current, &super, &bypass); err != nil {
		t.Fatal(err)
	}
	if current != loginName || super || bypass {
		t.Fatalf("session current=%s super=%v bypass=%v", current, super, bypass)
	}
}

func createCompanyAsLogin(t *testing.T, companies *repository.CompanyRepository, tenantID uuid.UUID) *domain.Company {
	t.Helper()
	created, err := companies.Create(context.Background(), domain.CreateCompanyInput{
		TenantID: tenantID, LegalName: "Login Carrier " + uuid.NewString()[:8], CompanyType: "CARRIER",
		CountryCode: "RU", PreferredLocale: "ru-RU",
	})
	if err != nil || created.TenantID != tenantID {
		t.Fatalf("create company: %v %+v", err, created)
	}
	return created
}

func expectSQLState(t *testing.T, pool *pgxpool.Pool, query, code string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), query)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code {
		t.Fatalf("query %s state=%v err=%v", query, pgErr, err)
	}
}

func stringPtrLocal(value string) *string { return &value }
