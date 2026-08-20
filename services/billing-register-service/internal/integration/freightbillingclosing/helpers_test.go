//go:build integration

package freightbillingclosing

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/billing-register-service/internal/domain"
	"github.com/freight-platform/billing-register-service/internal/repository"
	"github.com/freight-platform/billing-register-service/internal/service"
)

const maxMigrationFile = "000052_freight_settlement_snapshot_v2.0C.up.sql"

type env struct {
	pool        *pgxpool.Pool
	settlements *service.FreightSettlementService
	settlementRepo *repository.FreightSettlementRepository
	registers   *service.BillingRegisterService
	registerRepo *repository.BillingRegisterRepository
	closing     *service.ClosingDocumentService
}

type fixture struct {
	TenantID      uuid.UUID
	OtherTenantID uuid.UUID
	BuyerID       uuid.UUID
	ForeignBuyer  uuid.UUID
	CarrierA      uuid.UUID
	CarrierB      uuid.UUID
	OrderID       uuid.UUID
	EventID       uuid.UUID
	ResponseID    uuid.UUID
	AwardID       uuid.UUID
	AwardLinkID   uuid.UUID
	UserID        uuid.UUID
	BuyerUserID   uuid.UUID
	OriginID      uuid.UUID
	DestID        uuid.UUID
	CargoID       uuid.UUID
	ShipmentID    uuid.UUID
	AwardAmount   float64
	OfferAmount   float64
}

func setupEnv(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	url := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	settlementRepo := repository.NewFreightSettlementRepository(pool)
	registerRepo := repository.NewBillingRegisterRepository(pool)
	closingRepo := repository.NewClosingDocumentRepository(pool)
	return &env{
		pool:           pool,
		settlements:    service.NewFreightSettlementService(settlementRepo),
		settlementRepo: settlementRepo,
		registers:      service.NewBillingRegisterService(registerRepo),
		registerRepo:   registerRepo,
		closing:        service.NewClosingDocumentService(registerRepo, closingRepo),
	}
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	dir, err := locateMigrationsDir()
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		if strings.Compare(filepath.Base(file), maxMigrationFile) > 0 {
			break
		}
		content, readErr := os.ReadFile(file)
		if readErr != nil {
			return readErr
		}
		if _, execErr := pool.Exec(ctx, string(content)); execErr != nil {
			msg := execErr.Error()
			if strings.Contains(msg, "already exists") || strings.Contains(msg, "duplicate key") {
				continue
			}
			return execErr
		}
	}
	return nil
}

func locateMigrationsDir() (string, error) {
	candidates := []string{
		filepath.Join("..", "..", "..", "..", "..", "infrastructure", "migrations"),
		filepath.Join("..", "..", "..", "..", "infrastructure", "migrations"),
	}
	for _, candidate := range candidates {
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate, nil
		}
	}
	return "", os.ErrNotExist
}

func seedFixture(t *testing.T, pool *pgxpool.Pool) fixture {
	t.Helper()
	ctx := context.Background()
	fix := fixture{
		TenantID:      uuid.New(),
		OtherTenantID: uuid.New(),
		BuyerID:       uuid.New(),
		ForeignBuyer:  uuid.New(),
		CarrierA:      uuid.New(),
		CarrierB:      uuid.New(),
		OrderID:       uuid.New(),
		EventID:       uuid.New(),
		ResponseID:    uuid.New(),
		AwardID:       uuid.New(),
		AwardLinkID:   uuid.New(),
		UserID:        uuid.New(),
		BuyerUserID:   uuid.New(),
		OriginID:      uuid.New(),
		DestID:        uuid.New(),
		CargoID:       uuid.New(),
		ShipmentID:    uuid.New(),
		AwardAmount:   100000,
		OfferAmount:   45000,
	}
	for _, row := range []struct {
		tenant uuid.UUID
		code   string
		name   string
	}{
		{fix.TenantID, "t-" + fix.TenantID.String()[:8], "Billing Tenant"},
		{fix.OtherTenantID, "t-" + fix.OtherTenantID.String()[:8], "Other Tenant"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1,$2,$3)`,
			row.tenant, row.code, row.name); err != nil {
			t.Fatalf("tenant: %v", err)
		}
	}
	for _, row := range []struct {
		id, tenant uuid.UUID
		typ, name  string
	}{
		{fix.BuyerID, fix.TenantID, "SHIPPER", "Buyer Co"},
		{fix.ForeignBuyer, fix.TenantID, "SHIPPER", "Foreign Buyer"},
		{fix.CarrierA, fix.TenantID, "CARRIER", "Carrier A"},
		{fix.CarrierB, fix.TenantID, "CARRIER", "Carrier B"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status)
			VALUES ($1,$2,$3,$4,'ACTIVE')`, row.id, row.tenant, row.typ, row.name); err != nil {
			t.Fatalf("company: %v", err)
		}
	}
	for _, loc := range []struct {
		id   uuid.UUID
		name string
	}{
		{fix.OriginID, "Origin"},
		{fix.DestID, "Destination"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO transport.locations (id, tenant_id, company_id, location_type, name, country_code, city)
			VALUES ($1,$2,$3,'WAREHOUSE',$4,'RU','Moscow')`, loc.id, fix.TenantID, fix.BuyerID, loc.name); err != nil {
			t.Fatalf("location: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO transport.cargoes (id, tenant_id, cargo_type, description, gross_weight)
		VALUES ($1,$2,'GENERAL','Cargo',1000)`, fix.CargoID, fix.TenantID); err != nil {
		t.Fatalf("cargo: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO transport.transport_orders (
		id, tenant_id, order_number, shipper_company_id, consignee_company_id,
		origin_location_id, destination_location_id, cargo_id, transport_mode, status, source_system, external_reference
	) VALUES ($1,$2,$3,$4,$4,$5,$6,$7,'ROAD','CONVERTED_TO_SHIPMENT','rfx_award',$8)`,
		fix.OrderID, fix.TenantID, "TO-"+fix.OrderID.String()[:8], fix.BuyerID, fix.OriginID, fix.DestID, fix.CargoID,
		"CLIENT_AMOUNT:50000"); err != nil {
		t.Fatalf("order: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO rfx.rfx_events (id, tenant_id, owner_company_id, title, rfx_type, category, rfx_number, status)
		VALUES ($1,$2,$3,'Award Event','SPOT_RFQ','FREIGHT',$4,'AWARDED')`,
		fix.EventID, fix.TenantID, fix.BuyerID, "RFX-"+fix.EventID.String()[:8]); err != nil {
		t.Fatalf("event: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO rfx.rfx_responses (id, tenant_id, rfx_event_id, participant_company_id, status)
		VALUES ($1,$2,$3,$4,'SUBMITTED')`, fix.ResponseID, fix.TenantID, fix.EventID, fix.CarrierA); err != nil {
		t.Fatalf("response: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO rfx.rfx_response_offer_lines (id, tenant_id, rfx_response_id, amount, currency_code)
		VALUES ($1,$2,$3,$4,'RUB')`, uuid.New(), fix.TenantID, fix.ResponseID, fix.OfferAmount); err != nil {
		t.Fatalf("offer line: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO rfx.rfx_awards (id, tenant_id, rfx_event_id, rfx_response_id, carrier_company_id, total_amount, currency_code, awarded_at)
		VALUES ($1,$2,$3,$4,$5,$6,'RUB',now())`, fix.AwardID, fix.TenantID, fix.EventID, fix.ResponseID, fix.CarrierA, fix.OfferAmount); err != nil {
		t.Fatalf("award: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO rfx.rfx_award_transport_orders (
		id, tenant_id, rfx_event_id, rfx_award_id, rfx_response_id, transport_order_id,
		carrier_company_id, buyer_company_id, amount, currency_code
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'RUB')`,
		fix.AwardLinkID, fix.TenantID, fix.EventID, fix.AwardID, fix.ResponseID, fix.OrderID, fix.CarrierA, fix.BuyerID, fix.AwardAmount); err != nil {
		t.Fatalf("award link: %v", err)
	}
	seedDeliveredShipment(t, pool, fix)
	seedPODDocument(t, pool, fix.TenantID, fix.CarrierA, fix.ShipmentID, "POD-"+fix.ShipmentID.String()[:8])
	seedBillingMemberships(t, pool, fix)
	return fix
}

func seedBillingMemberships(t *testing.T, pool *pgxpool.Pool, fix fixture) {
	t.Helper()
	ctx := context.Background()
	for _, row := range []struct {
		id    uuid.UUID
		email string
	}{
		{fix.UserID, "carrier-user@" + fix.TenantID.String()[:8] + ".test"},
		{fix.BuyerUserID, "buyer-user@" + fix.TenantID.String()[:8] + ".test"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO core.users (id, tenant_id, email, full_name, status)
			VALUES ($1,$2,$3,$4,'ACTIVE') ON CONFLICT (id) DO NOTHING`, row.id, fix.TenantID, row.email, row.email); err != nil {
			t.Fatalf("user: %v", err)
		}
	}
	for _, row := range []struct {
		userID, companyID uuid.UUID
	}{
		{fix.UserID, fix.CarrierA},
		{fix.BuyerUserID, fix.BuyerID},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id, status)
			VALUES ($1,$2,$3,'ACTIVE') ON CONFLICT (company_id, user_id) DO NOTHING`,
			fix.TenantID, row.companyID, row.userID); err != nil {
			t.Fatalf("membership: %v", err)
		}
	}
	var carrierRoleID, buyerRoleID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code='CARRIER_ADMIN' LIMIT 1`).Scan(&carrierRoleID); err != nil {
		t.Fatalf("carrier role: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code='SHIPPER_ADMIN' LIMIT 1`).Scan(&buyerRoleID); err != nil {
		t.Fatalf("buyer role: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id)
		VALUES ($1,$2,$3,$4) ON CONFLICT (user_id, company_id, role_id) DO NOTHING`, fix.TenantID, fix.UserID, fix.CarrierA, carrierRoleID); err != nil {
		t.Fatalf("carrier user role: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id)
		VALUES ($1,$2,$3,$4) ON CONFLICT (user_id, company_id, role_id) DO NOTHING`, fix.TenantID, fix.BuyerUserID, fix.BuyerID, buyerRoleID); err != nil {
		t.Fatalf("buyer user role: %v", err)
	}
}

func seedDeliveredShipment(t *testing.T, pool *pgxpool.Pool, fix fixture) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO transport.shipments (
		id, tenant_id, shipment_number, transport_order_id, shipper_company_id, consignee_company_id,
		carrier_company_id, origin_location_id, destination_location_id, cargo_id, transport_mode, status,
		actual_delivery_at
	) VALUES ($1,$2,$3,$4,$5,$5,$6,$7,$8,$9,'ROAD',$10,now())`,
		fix.ShipmentID, fix.TenantID, "SHP-"+fix.ShipmentID.String()[:8], fix.OrderID, fix.BuyerID, fix.CarrierA,
		fix.OriginID, fix.DestID, fix.CargoID, domain.ShipmentStatusDelivered); err != nil {
		t.Fatalf("shipment: %v", err)
	}
}

func seedPODDocument(t *testing.T, pool *pgxpool.Pool, tenantID, ownerCompanyID, shipmentID uuid.UUID, docNumber string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	docID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO documents.documents (
			id, tenant_id, document_number, document_type, document_status,
			owner_company_id, related_entity_type, related_entity_id, created_at, updated_at
		) VALUES ($1,$2,$3,'POD','SIGNED',$4,'SHIPMENT',$5,now(),now())`,
		docID, tenantID, docNumber, ownerCompanyID, shipmentID); err != nil {
		t.Fatalf("seed pod document: %v", err)
	}
	return docID
}

func carrierActor(fix fixture) domain.SettlementActorInput {
	return domain.SettlementActorInput{
		TenantID: fix.TenantID, ActorCompanyID: fix.CarrierA,
		ActorKind: domain.SettlementActorCarrier, ActorUserID: fix.UserID,
	}
}

func buyerActor(fix fixture) domain.SettlementActorInput {
	return domain.SettlementActorInput{
		TenantID: fix.TenantID, ActorCompanyID: fix.BuyerID,
		ActorKind: domain.SettlementActorBuyer, ActorUserID: fix.BuyerUserID,
	}
}

func foreignBuyerActor(fix fixture) domain.SettlementActorInput {
	return domain.SettlementActorInput{
		TenantID: fix.TenantID, ActorCompanyID: fix.ForeignBuyer,
		ActorKind: domain.SettlementActorBuyer, ActorUserID: fix.BuyerUserID,
	}
}

func carrierBActor(fix fixture) domain.SettlementActorInput {
	return domain.SettlementActorInput{
		TenantID: fix.TenantID, ActorCompanyID: fix.CarrierB,
		ActorKind: domain.SettlementActorCarrier, ActorUserID: fix.UserID,
	}
}

func createSettlementInput(fix fixture, shipmentID uuid.UUID, idempotencyKey string) domain.CreateFreightSettlementInput {
	return domain.CreateFreightSettlementInput{
		TenantID: fix.TenantID, ShipmentID: shipmentID,
		ActorCompanyID: fix.CarrierA, ActorKind: domain.SettlementActorCarrier, ActorUserID: fix.UserID,
		IdempotencyKey: idempotencyKey, SettlementNumber: "FS-" + shipmentID.String()[:8],
	}
}

func createSettlement(t *testing.T, env *env, fix fixture, idempotencyKey string) *domain.FreightSettlement {
	t.Helper()
	settlement, err := env.settlements.Create(context.Background(), createSettlementInput(fix, fix.ShipmentID, idempotencyKey))
	if err != nil {
		t.Fatalf("create settlement: %v", err)
	}
	return settlement
}

func submitAndApproveSettlement(t *testing.T, env *env, fix fixture, settlementID uuid.UUID) *domain.FreightSettlement {
	t.Helper()
	ctx := context.Background()
	if _, err := env.settlements.SubmitForReview(ctx, settlementID, carrierActor(fix)); err != nil {
		t.Fatalf("submit for review: %v", err)
	}
	approved, err := env.settlements.Approve(ctx, settlementID, buyerActor(fix))
	if err != nil {
		t.Fatalf("approve settlement: %v", err)
	}
	return approved
}

func createRegisterInput(fix fixture, number string) domain.CreateBillingRegisterInput {
	period := time.Now().UTC()
	return domain.CreateBillingRegisterInput{
		TenantID: fix.TenantID, RegisterNumber: number,
		CustomerCompanyID: fix.BuyerID, ContractorCompanyID: fix.CarrierA,
		PeriodFrom: period, PeriodTo: period, CurrencyCode: "RUB",
	}
}

func createRegister(t *testing.T, env *env, fix fixture, number string) *domain.BillingRegister {
	t.Helper()
	reg, err := env.registers.CreateForActor(context.Background(), createRegisterInput(fix, number), buyerActor(fix))
	if err != nil {
		t.Fatalf("create register: %v", err)
	}
	return reg
}

func includeSettlement(t *testing.T, env *env, fix fixture, registerID, settlementID uuid.UUID) *repository.IncludeSettlementResult {
	t.Helper()
	result, err := env.registers.IncludeSettlement(context.Background(), registerID, settlementID, buyerActor(fix))
	if err != nil {
		t.Fatalf("include settlement: %v", err)
	}
	return result
}

func approvedSettlement(t *testing.T, env *env, fix fixture, key string) *domain.FreightSettlement {
	t.Helper()
	settlement := createSettlement(t, env, fix, key)
	return submitAndApproveSettlement(t, env, fix, settlement.ID)
}

func countRegisterItems(t *testing.T, pool *pgxpool.Pool, registerID uuid.UUID) int {
	t.Helper()
	var count int
	err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM billing.billing_register_items WHERE register_id=$1`, registerID).Scan(&count)
	if err != nil {
		t.Fatalf("count register items: %v", err)
	}
	return count
}

func countRegisterAuditEvents(t *testing.T, pool *pgxpool.Pool, registerID uuid.UUID) int {
	t.Helper()
	var count int
	err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM billing.billing_register_audit_events WHERE register_id=$1`, registerID).Scan(&count)
	if err != nil {
		t.Fatalf("count register audit: %v", err)
	}
	return count
}

func scanRegisterStatus(t *testing.T, pool *pgxpool.Pool, registerID uuid.UUID) string {
	t.Helper()
	var status string
	err := pool.QueryRow(context.Background(), `
		SELECT status FROM billing.billing_registers WHERE id=$1`, registerID).Scan(&status)
	if err != nil {
		t.Fatalf("register status: %v", err)
	}
	return status
}

func prepareApprovedRegister(t *testing.T, env *env, fix fixture, key, registerNumber string) *domain.BillingRegister {
	t.Helper()
	settlement := approvedSettlement(t, env, fix, key)
	reg := createRegister(t, env, fix, registerNumber)
	includeSettlement(t, env, fix, reg.ID, settlement.ID)
	if _, err := env.registers.Calculate(context.Background(), reg.ID, buyerActor(fix)); err != nil {
		t.Fatalf("calculate: %v", err)
	}
	approved, err := env.registers.Approve(context.Background(), reg.ID, buyerActor(fix))
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	return approved
}

func countPackages(t *testing.T, pool *pgxpool.Pool, registerID uuid.UUID) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM billing.closing_document_packages WHERE register_id=$1`, registerID).Scan(&count); err != nil {
		t.Fatalf("count packages: %v", err)
	}
	return count
}

func countDocuments(t *testing.T, pool *pgxpool.Pool, table string, registerID uuid.UUID) int {
	t.Helper()
	var count int
	query := "SELECT COUNT(*) FROM billing." + table + " WHERE register_id=$1"
	if err := pool.QueryRow(context.Background(), query, registerID).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

func hasAuditEvent(t *testing.T, pool *pgxpool.Pool, registerID uuid.UUID, eventType string) bool {
	t.Helper()
	var count int
	err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM billing.billing_register_audit_events
		WHERE register_id=$1 AND event_type=$2`, registerID, eventType).Scan(&count)
	if err != nil {
		t.Fatalf("audit lookup: %v", err)
	}
	return count > 0
}

func scanApprovedBy(t *testing.T, pool *pgxpool.Pool, registerID uuid.UUID) uuid.UUID {
	t.Helper()
	var approvedBy uuid.UUID
	err := pool.QueryRow(context.Background(), `
		SELECT COALESCE(approved_by, '00000000-0000-0000-0000-000000000000'::uuid)
		FROM billing.billing_registers WHERE id=$1`, registerID).Scan(&approvedBy)
	if err != nil {
		t.Fatalf("approved_by: %v", err)
	}
	return approvedBy
}
