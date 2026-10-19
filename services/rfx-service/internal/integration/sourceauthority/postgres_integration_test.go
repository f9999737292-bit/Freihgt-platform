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

	"github.com/freight-platform/rfx-service/internal/domain"
	rfxhttp "github.com/freight-platform/rfx-service/internal/http"
	"github.com/freight-platform/rfx-service/internal/http/sourceauth"
	"github.com/freight-platform/rfx-service/internal/repository"
	"github.com/freight-platform/rfx-service/internal/service"
)

func TestRfxSourceAuthorityPostgres(t *testing.T) {
	pool := setupPostgres(t)
	ctx := context.Background()
	var pgVersion, dbName string
	if err := pool.QueryRow(ctx, `SELECT current_setting('server_version'), current_database()`).Scan(&pgVersion, &dbName); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(dbName, "rfx_src_") {
		t.Fatalf("refusing non-isolated database %s", dbName)
	}
	t.Logf("POSTGRES_VERSION=%s", pgVersion)
	t.Logf("TEST_DATABASE_ISOLATED=%s", dbName)

	fix := seedSource(t, pool)
	repo := repository.NewRfxSourceAuthorityRepository(pool)
	authority := service.NewRfxSourceAuthority(repo)
	before := sideCounts(t, pool)
	var transportOrders int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM transport.transport_orders`).Scan(&transportOrders); err != nil {
		t.Fatal(err)
	}
	if transportOrders != 0 {
		t.Fatalf("transport orders before authority calls = %d", transportOrders)
	}

	award := awardSubject(t, repo, fix.AwardID, fix.Procurement)
	if got := authority.Confirm(ctx, award); got != domain.RfxSourceCreateConfirmed {
		t.Fatalf("award confirm = %s", got)
	}
	if got := authority.Revalidate(ctx, snapshotOf(award)); got != domain.RfxSourceMatch {
		t.Fatalf("award revalidate = %s", got)
	}
	link := linkSubject(t, repo, fix.LinkID, fix.Procurement)
	if got := authority.Confirm(ctx, link); got != domain.RfxSourceCreateConfirmed {
		t.Fatalf("link confirm = %s", got)
	}
	if got := authority.Revalidate(ctx, snapshotOf(link)); got != domain.RfxSourceMatch {
		t.Fatalf("link revalidate = %s", got)
	}
	if transportOrdersAfter(t, pool) != 0 {
		t.Fatal("RFx confirmation must not require or create a transport order row")
	}

	versioned := awardSubject(t, repo, fix.VersionAwardID, fix.Procurement)
	if got := authority.Revalidate(ctx, snapshotOf(versioned)); got != domain.RfxSourceMatch {
		t.Fatalf("version revalidate before change = %s", got)
	}
	if _, err := pool.Exec(ctx, `UPDATE rfx.rfx_awards SET version = version + 1 WHERE id = $1`, fix.VersionAwardID); err != nil {
		t.Fatal(err)
	}
	if got := authority.Confirm(ctx, versioned); got != domain.RfxSourceCreateStale {
		t.Fatalf("stale award confirm = %s", got)
	}
	if got := authority.Revalidate(ctx, snapshotOf(versioned)); got != domain.RfxSourceChanged {
		t.Fatalf("stale award revalidate = %s", got)
	}
	reread := awardSubject(t, repo, fix.VersionAwardID, fix.Procurement)
	if reread.Fingerprint == versioned.Fingerprint {
		t.Fatal("revalidate must observe the current award version")
	}
	if got := authority.Revalidate(ctx, snapshotOf(reread)); got != domain.RfxSourceMatch {
		t.Fatalf("reread award revalidate = %s", got)
	}

	if _, err := pool.Exec(ctx, `UPDATE rfx.rfx_events SET deleted_at = now() WHERE id = $1`, fix.DeletedEventID); err != nil {
		t.Fatal(err)
	}
	deleted := awardSubject(t, repo, fix.DeletedAwardID, fix.Procurement)
	if got := authority.Confirm(ctx, deleted); got != domain.RfxSourceCreateNotFound {
		t.Fatalf("deleted event confirm = %s", got)
	}
	if got := authority.Revalidate(ctx, snapshotOf(deleted)); got != domain.RfxSourceChanged {
		t.Fatalf("deleted event revalidate = %s", got)
	}

	if _, err := pool.Exec(ctx, `UPDATE rfx.rfx_events SET status = 'ARCHIVED' WHERE id = $1`, fix.ArchivedEventID); err != nil {
		t.Fatal(err)
	}
	archived := awardSubject(t, repo, fix.ArchivedAwardID, fix.Procurement)
	if got := authority.Confirm(ctx, archived); got != domain.RfxSourceCreateNotFound {
		t.Fatalf("archived event confirm = %s", got)
	}
	if got := authority.Revalidate(ctx, snapshotOf(archived)); got != domain.RfxSourceChanged {
		t.Fatalf("archived event revalidate = %s", got)
	}

	foreign := award
	foreign.SourceTenantID = fix.TenantB
	if got := authority.Confirm(ctx, foreign); got != domain.RfxSourceCreateTenantMismatch {
		t.Fatalf("foreign tenant confirm = %s", got)
	}
	if got := authority.Revalidate(ctx, snapshotOf(foreign)); got != domain.RfxSourceNotFound {
		t.Fatalf("foreign tenant revalidate = %s", got)
	}
	otherTenant := awardSubject(t, repo, fix.TenantBAwardID, fix.TenantBProcurement)
	otherTenant.SourceTenantID = fix.TenantA
	if got := authority.Confirm(ctx, otherTenant); got != domain.RfxSourceCreateTenantMismatch {
		t.Fatalf("other tenant award confirm = %s", got)
	}

	wrongCarrier := award
	wrongCarrier.ShipperCarrierCompanyID = fix.OtherCarrierID
	if got := authority.Confirm(ctx, wrongCarrier); got != domain.RfxSourceCreateStale {
		t.Fatalf("wrong carrier confirm = %s", got)
	}
	if got := authority.Revalidate(ctx, snapshotOf(wrongCarrier)); got != domain.RfxSourceChanged {
		t.Fatalf("wrong carrier revalidate = %s", got)
	}

	for _, userID := range []uuid.UUID{fix.BlockedUser, fix.DeletedUser, fix.CarrierAdmin, fix.PlatformNoMembership, fix.InactiveMembership, fix.CompanyScopedPlatformAdmin} {
		attempt := award
		attempt.RequestingUserID = userID
		if got := authority.Confirm(ctx, attempt); got != domain.RfxSourceCreateActorMismatch {
			t.Fatalf("actor %s confirm = %s", userID, got)
		}
	}
	otherCompany := award
	otherCompany.RequestingCompanyID = fix.OtherCarrierID
	if got := authority.Confirm(ctx, otherCompany); got != domain.RfxSourceCreateActorMismatch {
		t.Fatalf("other company confirm = %s", got)
	}
	for _, userID := range []uuid.UUID{fix.Procurement, fix.ShipperAdmin, fix.ShipperLogist, fix.ForwarderManager, fix.PlatformWithMembership} {
		attempt := award
		attempt.RequestingUserID = userID
		if got := authority.Confirm(ctx, attempt); got != domain.RfxSourceCreateConfirmed {
			t.Fatalf("authorized actor %s confirm = %s", userID, got)
		}
	}

	foreignParent := linkSubject(t, repo, fix.ForeignParentLinkID, fix.Procurement)
	if got := authority.Confirm(ctx, foreignParent); got != domain.RfxSourceCreateNotFound {
		t.Fatalf("foreign parent confirm = %s", got)
	}
	if got := authority.Revalidate(ctx, snapshotOf(foreignParent)); got != domain.RfxSourceChanged {
		t.Fatalf("foreign parent revalidate = %s", got)
	}

	if _, err := pool.Exec(ctx, `UPDATE rfx.rfx_award_transport_orders SET carrier_company_id = $2 WHERE id = $1`, fix.CarrierMismatchLinkID, fix.OtherCarrierID); err != nil {
		t.Fatal(err)
	}
	carrierMismatch := linkSubject(t, repo, fix.CarrierMismatchLinkID, fix.Procurement)
	if got := authority.Confirm(ctx, carrierMismatch); got != domain.RfxSourceCreateNotFound {
		t.Fatalf("link carrier mismatch confirm = %s", got)
	}
	if got := authority.Revalidate(ctx, snapshotOf(carrierMismatch)); got != domain.RfxSourceChanged {
		t.Fatalf("link carrier mismatch revalidate = %s", got)
	}

	if _, err := pool.Exec(ctx, `UPDATE rfx.rfx_award_transport_orders SET buyer_company_id = $2 WHERE id = $1`, fix.BuyerMismatchLinkID, fix.OtherCarrierID); err != nil {
		t.Fatal(err)
	}
	buyerMismatch := linkSubject(t, repo, fix.BuyerMismatchLinkID, fix.Procurement)
	if got := authority.Confirm(ctx, buyerMismatch); got != domain.RfxSourceCreateNotFound {
		t.Fatalf("buyer mismatch confirm = %s", got)
	}
	if got := authority.Revalidate(ctx, snapshotOf(buyerMismatch)); got != domain.RfxSourceChanged {
		t.Fatalf("buyer mismatch revalidate = %s", got)
	}

	staleLink := linkSubject(t, repo, fix.OrderLinkID, fix.Procurement)
	nextOrder := uuid.New()
	if _, err := pool.Exec(ctx, `UPDATE rfx.rfx_award_transport_orders SET transport_order_id = $2 WHERE id = $1`, fix.OrderLinkID, nextOrder); err != nil {
		t.Fatal(err)
	}
	if got := authority.Confirm(ctx, staleLink); got != domain.RfxSourceCreateStale {
		t.Fatalf("changed transport order confirm = %s", got)
	}
	if got := authority.Revalidate(ctx, snapshotOf(staleLink)); got != domain.RfxSourceChanged {
		t.Fatalf("changed transport order revalidate = %s", got)
	}
	currentOrder := linkSubject(t, repo, fix.OrderLinkID, fix.Procurement)
	if got := authority.Revalidate(ctx, snapshotOf(currentOrder)); got != domain.RfxSourceMatch {
		t.Fatalf("current transport order provenance revalidate = %s", got)
	}
	if transportOrdersAfter(t, pool) != 0 {
		t.Fatal("transport order provenance must remain the RFx link column")
	}

	staleVersion := linkSubject(t, repo, fix.VersionLinkID, fix.Procurement)
	if _, err := pool.Exec(ctx, `UPDATE rfx.rfx_award_transport_orders SET version = version + 1 WHERE id = $1`, fix.VersionLinkID); err != nil {
		t.Fatal(err)
	}
	if got := authority.Confirm(ctx, staleVersion); got != domain.RfxSourceCreateStale {
		t.Fatalf("link version confirm = %s", got)
	}
	if got := authority.Revalidate(ctx, snapshotOf(staleVersion)); got != domain.RfxSourceChanged {
		t.Fatalf("link version revalidate = %s", got)
	}

	if _, err := pool.Exec(ctx, `UPDATE rfx.rfx_events SET deleted_at = now() WHERE id = $1`, fix.DeletedLinkEventID); err != nil {
		t.Fatal(err)
	}
	deletedLink := linkSubject(t, repo, fix.DeletedLinkID, fix.Procurement)
	if got := authority.Confirm(ctx, deletedLink); got != domain.RfxSourceCreateNotFound {
		t.Fatalf("deleted parent link confirm = %s", got)
	}
	if got := authority.Revalidate(ctx, snapshotOf(deletedLink)); got != domain.RfxSourceChanged {
		t.Fatalf("deleted parent link revalidate = %s", got)
	}
	if _, err := pool.Exec(ctx, `UPDATE rfx.rfx_events SET status = 'CANCELLED' WHERE id = $1`, fix.InvalidLinkEventID); err != nil {
		t.Fatal(err)
	}
	invalidLink := linkSubject(t, repo, fix.InvalidLinkID, fix.Procurement)
	if got := authority.Confirm(ctx, invalidLink); got != domain.RfxSourceCreateNotFound {
		t.Fatalf("invalid parent link confirm = %s", got)
	}
	if got := authority.Revalidate(ctx, snapshotOf(invalidLink)); got != domain.RfxSourceChanged {
		t.Fatalf("invalid parent link revalidate = %s", got)
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if got := authority.Confirm(cancelled, award); got != domain.RfxSourceCreateUnavailable {
		t.Fatalf("sql error confirm = %s", got)
	}
	if got := authority.Revalidate(cancelled, snapshotOf(link)); got != domain.RfxSourceUnavailable {
		t.Fatalf("sql error revalidate = %s", got)
	}
	if sideCounts(t, pool) != before {
		t.Fatalf("authority calls wrote relationship rows: before=%s after=%s", before, sideCounts(t, pool))
	}

	const revalidationToken = "dedicated-revalidation-token"
	const shipmentCredential = "shipment-relationship-credential"
	handler := rfxhttp.WithRfxSourceAuthority(http.NotFoundHandler(), revalidationToken, authority)
	payload, _ := json.Marshal(map[string]string{
		"source_type": award.SourceType, "source_id": award.SourceID.String(), "source_tenant_id": award.SourceTenantID.String(),
		"source_version_or_fingerprint": award.Fingerprint, "shipper_carrier_company_id": award.ShipperCarrierCompanyID.String(),
		"requesting_company_id": award.RequestingCompanyID.String(), "requesting_user_id": award.RequestingUserID.String(),
	})
	ok := postAuthority(handler, domain.RfxSourceConfirmPath, payload, revalidationToken)
	if ok.Code != http.StatusOK || !strings.Contains(ok.Body.String(), `"decision":"CONFIRMED"`) || strings.Contains(ok.Body.String(), "transport_order") {
		t.Fatalf("http confirm = %d %s", ok.Code, ok.Body.String())
	}
	for _, presented := range []string{"", shipmentCredential, revalidationToken + "-no"} {
		rec := postAuthority(handler, domain.RfxSourceConfirmPath, payload, presented)
		if rec.Code != http.StatusUnauthorized || rec.Body.Len() != 0 {
			t.Fatalf("credential %q = %d %s", presented, rec.Code, rec.Body.String())
		}
	}
	spoof := httptest.NewRequest(http.MethodPost, domain.RfxSourceConfirmPath, bytes.NewReader(payload))
	spoof.Header.Set("X-Internal-Service-Token", shipmentCredential)
	spoof.Header.Set("X-Internal-Service-Name", "shipment-service")
	spoofed := httptest.NewRecorder()
	handler.ServeHTTP(spoofed, spoof)
	if spoofed.Code != http.StatusUnauthorized || spoofed.Body.Len() != 0 {
		t.Fatalf("shipment credential = %d %s", spoofed.Code, spoofed.Body.String())
	}
	closed := rfxhttp.WithRfxSourceAuthority(http.NotFoundHandler(), "", authority)
	if refused := postAuthority(closed, domain.RfxSourceRevalidatePath, payload, revalidationToken); refused.Code != http.StatusUnauthorized {
		t.Fatalf("empty configured credential = %d", refused.Code)
	}
	if sourceauth.Header() != "X-Company-Relationship-Revalidation-Token" {
		t.Fatal("revalidation header drifted")
	}
	t.Log("POSTGRES_INTEGRATION=PASS")
}

type sourceFix struct {
	TenantA, TenantB, OwnerID, CarrierID, OtherCarrierID uuid.UUID
	AwardID, VersionAwardID, DeletedAwardID              uuid.UUID
	ArchivedAwardID, TenantBAwardID                      uuid.UUID
	DeletedEventID, ArchivedEventID                      uuid.UUID
	LinkID, ForeignParentLinkID                          uuid.UUID
	CarrierMismatchLinkID, BuyerMismatchLinkID           uuid.UUID
	OrderLinkID, VersionLinkID                           uuid.UUID
	DeletedLinkID, InvalidLinkID                         uuid.UUID
	DeletedLinkEventID, InvalidLinkEventID               uuid.UUID
	Procurement, ShipperAdmin, ShipperLogist             uuid.UUID
	ForwarderManager, CarrierAdmin                       uuid.UUID
	BlockedUser, DeletedUser                             uuid.UUID
	PlatformNoMembership, PlatformWithMembership         uuid.UUID
	InactiveMembership, CompanyScopedPlatformAdmin       uuid.UUID
	TenantBProcurement                                   uuid.UUID
}

func awardSubject(t *testing.T, repo *repository.RfxSourceAuthorityRepository, awardID, userID uuid.UUID) domain.RfxSourceSubject {
	t.Helper()
	row, err := repo.ReadAwardSource(context.Background(), awardID)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, err := domain.RfxAwardSourceFingerprint(row)
	if err != nil {
		t.Fatal(err)
	}
	return domain.RfxSourceSubject{
		SourceType: domain.RfxSourceTypeAward, SourceID: row.ID, SourceTenantID: row.TenantID,
		Fingerprint: fingerprint, ShipperCarrierCompanyID: row.CarrierCompanyID,
		RequestingCompanyID: row.OwnerCompanyID, RequestingUserID: userID,
	}
}

func linkSubject(t *testing.T, repo *repository.RfxSourceAuthorityRepository, linkID, userID uuid.UUID) domain.RfxSourceSubject {
	t.Helper()
	row, err := repo.ReadAwardLinkSource(context.Background(), linkID)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, err := domain.RfxAwardLinkSourceFingerprint(row)
	if err != nil {
		t.Fatal(err)
	}
	owner := row.OwnerCompanyID
	if owner == uuid.Nil {
		owner = row.BuyerCompanyID
	}
	return domain.RfxSourceSubject{
		SourceType: domain.RfxSourceTypeAwardLink, SourceID: row.ID, SourceTenantID: row.TenantID,
		Fingerprint: fingerprint, ShipperCarrierCompanyID: row.CarrierCompanyID,
		RequestingCompanyID: owner, RequestingUserID: userID,
	}
}

func snapshotOf(subject domain.RfxSourceSubject) domain.RfxSourceSnapshot {
	return domain.RfxSourceSnapshot{
		SourceType: subject.SourceType, SourceID: subject.SourceID, SourceTenantID: subject.SourceTenantID,
		Fingerprint: subject.Fingerprint, ShipperCarrierCompanyID: subject.ShipperCarrierCompanyID,
	}
}

func sideCounts(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var requests, events, idempotency, audit, outbox, relationships int
	err := pool.QueryRow(context.Background(), `
		SELECT
			(SELECT count(*) FROM company.carrier_relationship_requests),
			(SELECT count(*) FROM company.carrier_relationship_request_events),
			(SELECT count(*) FROM company.carrier_relationship_request_idempotency),
			(SELECT count(*) FROM company.carrier_relationship_audit),
			(SELECT count(*) FROM company.carrier_relationship_outbox),
			(SELECT count(*) FROM company.carrier_relationships)
	`).Scan(&requests, &events, &idempotency, &audit, &outbox, &relationships)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("requests=%d events=%d idempotency=%d audit=%d outbox=%d relationships=%d", requests, events, idempotency, audit, outbox, relationships)
}

func transportOrdersAfter(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM transport.transport_orders`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func seedSource(t *testing.T, pool *pgxpool.Pool) sourceFix {
	t.Helper()
	ctx := context.Background()
	fix := sourceFix{
		TenantA: uuid.New(), TenantB: uuid.New(), OwnerID: uuid.New(), CarrierID: uuid.New(), OtherCarrierID: uuid.New(),
		Procurement: uuid.New(), ShipperAdmin: uuid.New(), ShipperLogist: uuid.New(), ForwarderManager: uuid.New(),
		CarrierAdmin: uuid.New(), BlockedUser: uuid.New(), DeletedUser: uuid.New(), PlatformNoMembership: uuid.New(),
		PlatformWithMembership: uuid.New(), InactiveMembership: uuid.New(), CompanyScopedPlatformAdmin: uuid.New(),
		TenantBProcurement: uuid.New(),
	}
	for _, row := range []struct {
		id   uuid.UUID
		code string
	}{{fix.TenantA, "a"}, {fix.TenantB, "b"}} {
		if _, err := pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1, $2, $3)`, row.id, row.code+"-"+row.id.String()[:8], row.code); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status) VALUES
		($1,$2,'SHIPPER',$3,'ACTIVE'), ($4,$2,'CARRIER',$5,'ACTIVE'), ($6,$2,'CARRIER',$7,'ACTIVE')
	`, fix.OwnerID, fix.TenantA, fix.OwnerID.String(), fix.CarrierID, fix.CarrierID.String(), fix.OtherCarrierID, fix.OtherCarrierID.String()); err != nil {
		t.Fatal(err)
	}
	insertUser(t, pool, fix.Procurement, fix.TenantA, "ACTIVE", false)
	insertUser(t, pool, fix.ShipperAdmin, fix.TenantA, "ACTIVE", false)
	insertUser(t, pool, fix.ShipperLogist, fix.TenantA, "ACTIVE", false)
	insertUser(t, pool, fix.ForwarderManager, fix.TenantA, "ACTIVE", false)
	insertUser(t, pool, fix.CarrierAdmin, fix.TenantA, "ACTIVE", false)
	insertUser(t, pool, fix.BlockedUser, fix.TenantA, "BLOCKED", false)
	insertUser(t, pool, fix.DeletedUser, fix.TenantA, "ACTIVE", true)
	insertUser(t, pool, fix.PlatformNoMembership, fix.TenantA, "ACTIVE", false)
	insertUser(t, pool, fix.PlatformWithMembership, fix.TenantA, "ACTIVE", false)
	insertUser(t, pool, fix.InactiveMembership, fix.TenantA, "ACTIVE", false)
	insertUser(t, pool, fix.CompanyScopedPlatformAdmin, fix.TenantA, "ACTIVE", false)
	insertUser(t, pool, fix.TenantBProcurement, fix.TenantB, "ACTIVE", false)
	for _, userID := range []uuid.UUID{fix.Procurement, fix.ShipperAdmin, fix.ShipperLogist, fix.ForwarderManager, fix.CarrierAdmin, fix.BlockedUser, fix.PlatformWithMembership, fix.CompanyScopedPlatformAdmin} {
		insertMembership(t, pool, fix.TenantA, fix.OwnerID, userID, "ACTIVE", false)
	}
	insertMembership(t, pool, fix.TenantA, fix.OwnerID, fix.InactiveMembership, "SUSPENDED", false)
	grantRole(t, pool, fix.TenantA, fix.Procurement, &fix.OwnerID, "PROCUREMENT_MANAGER")
	grantRole(t, pool, fix.TenantA, fix.ShipperAdmin, &fix.OwnerID, "SHIPPER_ADMIN")
	grantRole(t, pool, fix.TenantA, fix.ShipperLogist, &fix.OwnerID, "SHIPPER_LOGIST")
	grantRole(t, pool, fix.TenantA, fix.ForwarderManager, &fix.OwnerID, "FORWARDER_MANAGER")
	grantRole(t, pool, fix.TenantA, fix.CarrierAdmin, &fix.OwnerID, "CARRIER_ADMIN")
	grantRole(t, pool, fix.TenantA, fix.BlockedUser, &fix.OwnerID, "PROCUREMENT_MANAGER")
	grantRole(t, pool, fix.TenantA, fix.InactiveMembership, &fix.OwnerID, "PROCUREMENT_MANAGER")
	grantRole(t, pool, fix.TenantA, fix.PlatformNoMembership, nil, "PLATFORM_ADMIN")
	grantRole(t, pool, fix.TenantA, fix.PlatformWithMembership, nil, "PLATFORM_ADMIN")
	grantRole(t, pool, fix.TenantA, fix.CompanyScopedPlatformAdmin, &fix.OwnerID, "PLATFORM_ADMIN")

	fix.AwardID = insertAward(t, pool, fix.TenantA, fix.OwnerID, fix.CarrierID, "AWARDED")
	fix.VersionAwardID = insertAward(t, pool, fix.TenantA, fix.OwnerID, fix.CarrierID, "AWARDED")
	fix.DeletedEventID, fix.DeletedAwardID = insertAwardWithEvent(t, pool, fix.TenantA, fix.OwnerID, fix.CarrierID, "AWARDED")
	fix.ArchivedEventID, fix.ArchivedAwardID = insertAwardWithEvent(t, pool, fix.TenantA, fix.OwnerID, fix.CarrierID, "AWARDED")
	ownerB := uuid.New()
	carrierB := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status) VALUES
		($1,$2,'SHIPPER',$3,'ACTIVE'), ($4,$2,'CARRIER',$5,'ACTIVE')
	`, ownerB, fix.TenantB, ownerB.String(), carrierB, carrierB.String()); err != nil {
		t.Fatal(err)
	}
	insertMembership(t, pool, fix.TenantB, ownerB, fix.TenantBProcurement, "ACTIVE", false)
	grantRole(t, pool, fix.TenantB, fix.TenantBProcurement, &ownerB, "PROCUREMENT_MANAGER")
	fix.TenantBAwardID = insertAward(t, pool, fix.TenantB, ownerB, carrierB, "AWARDED")

	fix.LinkID = insertLink(t, pool, fix.TenantA, fix.OwnerID, fix.CarrierID, uuid.Nil)
	fix.ForeignParentLinkID = insertForeignParentLink(t, pool, fix.TenantA, fix.OwnerID, fix.CarrierID, fix.TenantBAwardID)
	fix.CarrierMismatchLinkID = insertLink(t, pool, fix.TenantA, fix.OwnerID, fix.CarrierID, uuid.Nil)
	fix.BuyerMismatchLinkID = insertLink(t, pool, fix.TenantA, fix.OwnerID, fix.CarrierID, uuid.Nil)
	fix.OrderLinkID = insertLink(t, pool, fix.TenantA, fix.OwnerID, fix.CarrierID, uuid.Nil)
	fix.VersionLinkID = insertLink(t, pool, fix.TenantA, fix.OwnerID, fix.CarrierID, uuid.Nil)
	fix.DeletedLinkEventID, fix.DeletedLinkID = insertLinkWithEvent(t, pool, fix.TenantA, fix.OwnerID, fix.CarrierID)
	fix.InvalidLinkEventID, fix.InvalidLinkID = insertLinkWithEvent(t, pool, fix.TenantA, fix.OwnerID, fix.CarrierID)
	return fix
}

func insertAward(t *testing.T, pool *pgxpool.Pool, tenant, owner, carrier uuid.UUID, status string) uuid.UUID {
	t.Helper()
	_, awardID := insertAwardWithEvent(t, pool, tenant, owner, carrier, status)
	return awardID
}

func insertAwardWithEvent(t *testing.T, pool *pgxpool.Pool, tenant, owner, carrier uuid.UUID, status string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	eventID, responseID := insertEvent(t, pool, tenant, owner, carrier, status)
	awardID := uuid.New()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO rfx.rfx_awards (id, tenant_id, rfx_event_id, rfx_response_id, carrier_company_id, version)
		VALUES ($1,$2,$3,$4,$5,4)
	`, awardID, tenant, eventID, responseID, carrier)
	if err != nil {
		t.Fatal(err)
	}
	return eventID, awardID
}

func insertLink(t *testing.T, pool *pgxpool.Pool, tenant, owner, carrier, awardID uuid.UUID) uuid.UUID {
	t.Helper()
	_, linkID := insertLinkWithEvent(t, pool, tenant, owner, carrier)
	return linkID
}

func insertLinkWithEvent(t *testing.T, pool *pgxpool.Pool, tenant, owner, carrier uuid.UUID) (uuid.UUID, uuid.UUID) {
	t.Helper()
	eventID, awardID := insertAwardWithEvent(t, pool, tenant, owner, carrier, "AWARDED")
	var responseID uuid.UUID
	if err := pool.QueryRow(context.Background(), `SELECT rfx_response_id FROM rfx.rfx_awards WHERE id = $1`, awardID).Scan(&responseID); err != nil {
		t.Fatal(err)
	}
	linkID := uuid.New()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO rfx.rfx_award_transport_orders (
			id, tenant_id, rfx_event_id, rfx_award_id, rfx_response_id, transport_order_id,
			carrier_company_id, buyer_company_id, amount, currency_code, version
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,10,'RUB',2)
	`, linkID, tenant, eventID, awardID, responseID, uuid.New(), carrier, owner)
	if err != nil {
		t.Fatal(err)
	}
	return eventID, linkID
}

func insertForeignParentLink(t *testing.T, pool *pgxpool.Pool, tenant, owner, carrier, foreignAwardID uuid.UUID) uuid.UUID {
	t.Helper()
	eventID, responseID := insertEvent(t, pool, tenant, owner, carrier, "AWARDED")
	linkID := uuid.New()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO rfx.rfx_award_transport_orders (
			id, tenant_id, rfx_event_id, rfx_award_id, rfx_response_id, transport_order_id,
			carrier_company_id, buyer_company_id, amount, currency_code, version
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,10,'RUB',2)
	`, linkID, tenant, eventID, foreignAwardID, responseID, uuid.New(), carrier, owner)
	if err != nil {
		t.Fatal(err)
	}
	return linkID
}

func insertEvent(t *testing.T, pool *pgxpool.Pool, tenant, owner, carrier uuid.UUID, status string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	eventID, responseID := uuid.New(), uuid.New()
	number := "RFX-" + eventID.String()[:8]
	_, err := pool.Exec(context.Background(), `
		INSERT INTO rfx.rfx_events (id, tenant_id, rfx_number, rfx_type, category, title, owner_company_id, status)
		VALUES ($1,$2,$3,'RFQ','ROAD',$4,$5,$6)
	`, eventID, tenant, number, number, owner, status)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(context.Background(), `
		INSERT INTO rfx.rfx_responses (id, tenant_id, rfx_event_id, participant_company_id, status)
		VALUES ($1,$2,$3,$4,'SUBMITTED')
	`, responseID, tenant, eventID, carrier)
	if err != nil {
		t.Fatal(err)
	}
	return eventID, responseID
}

func insertUser(t *testing.T, pool *pgxpool.Pool, id, tenant uuid.UUID, status string, deleted bool) {
	t.Helper()
	var deletedAt any
	if deleted {
		deletedAt = time.Now().UTC()
	}
	_, err := pool.Exec(context.Background(), `
		INSERT INTO core.users (id, tenant_id, email, full_name, status, deleted_at)
		VALUES ($1,$2,$3,'User',$4,$5)
	`, id, tenant, id.String()+"@example.test", status, deletedAt)
	if err != nil {
		t.Fatal(err)
	}
}

func insertMembership(t *testing.T, pool *pgxpool.Pool, tenant, company, user uuid.UUID, status string, deleted bool) {
	t.Helper()
	var deletedAt any
	if deleted {
		deletedAt = time.Now().UTC()
	}
	_, err := pool.Exec(context.Background(), `
		INSERT INTO core.company_memberships (tenant_id, company_id, user_id, status, deleted_at)
		VALUES ($1,$2,$3,$4,$5)
	`, tenant, company, user, status, deletedAt)
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
		if os.Getenv("REQUIRE_TEST_DATABASE") == "1" || strings.EqualFold(strings.TrimSpace(os.Getenv("CI")), "true") {
			t.Fatal("TEST_DATABASE_URL is required for the PostgreSQL gate")
		}
		adminURL = startEmbeddedPostgres(t)
	}
	dbName := "rfx_src_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
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
