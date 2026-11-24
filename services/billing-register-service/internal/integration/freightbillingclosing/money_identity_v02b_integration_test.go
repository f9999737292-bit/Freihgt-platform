//go:build integration

package freightbillingclosing

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/billing-register-service/internal/domain"
)

const moneyIdentityAmount = "1234.56"

func TestMoneyIdentitySnapshotToClosingDocuments(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	snapshotID := attachRateSnapshot(t, env.pool, fix, moneyIdentityAmount)
	ctx := context.Background()

	first := createSettlement(t, env, fix, "money-identity-0.2b")
	second := createSettlement(t, env, fix, "money-identity-0.2b")
	if first.ID != second.ID {
		t.Fatalf("settlement replay created %s and %s", first.ID, second.ID)
	}
	if countTenantRows(t, env.pool, "billing.freight_settlements", fix.TenantID) != 1 {
		t.Fatal("settlement replay created a second row")
	}
	approved := submitAndApproveSettlement(t, env, fix, first.ID)
	if approved.Status != domain.SettlementStatusApproved {
		t.Fatalf("settlement status %s", approved.Status)
	}

	if _, err := env.settlements.GetDetail(ctx, first.ID, fix.OtherTenantID, fix.BuyerID, domain.SettlementActorBuyer); err == nil {
		t.Fatal("foreign tenant read the settlement")
	}
	if countTenantRows(t, env.pool, "billing.freight_settlements", fix.TenantID) != 1 {
		t.Fatal("foreign tenant read created a settlement")
	}

	reg := createRegister(t, env, fix, "BR-MI-"+fix.OrderID.String()[:8])
	versionBefore := scanRegisterVersion(t, env.pool, reg.ID)
	auditBefore := countRegisterAuditEvents(t, env.pool, reg.ID)
	outboxBefore := countOutbox(t, env.pool, fix.TenantID, first.ID)

	if _, err := env.registers.IncludeSettlement(ctx, reg.ID, first.ID, foreignBuyerActor(fix)); err == nil {
		t.Fatal("foreign company included the settlement")
	}
	spoofed := buyerActor(fix)
	spoofed.TenantID = fix.OtherTenantID
	if _, err := env.registers.IncludeSettlement(ctx, reg.ID, first.ID, spoofed); err == nil {
		t.Fatal("spoofed tenant included the settlement")
	}
	if countRegisterItems(t, env.pool, reg.ID) != 0 {
		t.Fatal("denied inclusion created a register item")
	}
	if scanRegisterVersion(t, env.pool, reg.ID) != versionBefore || countRegisterAuditEvents(t, env.pool, reg.ID) != auditBefore || countOutbox(t, env.pool, fix.TenantID, first.ID) != outboxBefore {
		t.Fatal("denied inclusion changed version, audit, or outbox")
	}

	included := includeSettlement(t, env, fix, reg.ID, first.ID)
	if _, err := env.registers.IncludeSettlement(ctx, reg.ID, first.ID, buyerActor(fix)); err != nil {
		t.Fatalf("include replay: %v", err)
	}
	if countRegisterItems(t, env.pool, reg.ID) != 1 {
		t.Fatal("include replay created a second register item")
	}
	if _, err := env.registers.Calculate(ctx, reg.ID, buyerActor(fix)); err != nil {
		t.Fatalf("calculate: %v", err)
	}
	if _, err := env.registers.Approve(ctx, reg.ID, buyerActor(fix)); err != nil {
		t.Fatalf("approve register: %v", err)
	}

	packageInput := domain.CreateClosingDocumentPackageInput{
		TenantID:      fix.OtherTenantID,
		PackageNumber: "PKG-MI-" + fix.OrderID.String()[:8],
		PackageType:   domain.ClosingPackageTypeActPlusVATInvoice,
	}
	created, err := env.closing.CreatePackageForActor(ctx, reg.ID, packageInput, buyerActor(fix))
	if err != nil {
		t.Fatalf("closing package: %v", err)
	}
	replayed, err := env.closing.CreatePackageForActor(ctx, reg.ID, packageInput, buyerActor(fix))
	if err != nil {
		t.Fatalf("closing package replay: %v", err)
	}
	if created.Act == nil || created.VATInvoice == nil || replayed.Act == nil || replayed.VATInvoice == nil || created.Package == nil {
		t.Fatal("closing package did not return an act and a VAT invoice")
	}
	if created.Package.TenantID != fix.TenantID {
		t.Fatal("package body tenant replaced the trusted tenant")
	}
	if created.Package.ID != replayed.Package.ID || created.Act.ID != replayed.Act.ID || created.VATInvoice.ID != replayed.VATInvoice.ID {
		t.Fatal("closing package replay created another act or VAT invoice")
	}
	if countDocuments(t, env.pool, "acts", reg.ID) != 1 || countDocuments(t, env.pool, "vat_invoices", reg.ID) != 1 || countPackages(t, env.pool, reg.ID) != 1 {
		t.Fatal("closing package replay created duplicate financial documents")
	}
	if scanRegisterStatus(t, env.pool, reg.ID) != "CLOSING_DOCUMENTS_CREATED" {
		t.Fatalf("register status %s", scanRegisterStatus(t, env.pool, reg.ID))
	}
	if countTenantRows(t, env.pool, "billing.payment_obligations", fix.TenantID) != 0 {
		t.Fatal("payment obligation was created")
	}

	row := readMoneyIdentity(t, env.pool, fix.TenantID, snapshotID, first.ID, reg.ID)
	if row.AwardAmount == moneyIdentityAmount {
		t.Fatal("award amount collided with the snapshot amount; choose another fixture amount")
	}
	assertDecimalEqual(t, "snapshot to settlement base", moneyIdentityAmount, row.SnapshotTotal, row.SettlementBase)
	assertDecimalEqual(t, "settlement base to register item", moneyIdentityAmount, row.SettlementBase, row.ItemBase)
	assertDecimalEqual(t, "register base to VAT invoice base", moneyIdentityAmount, row.RegisterWithoutVAT, row.VATInvoiceWithoutVAT)
	assertDecimalEqual(t, "act total to register total with VAT", row.RegisterWithVAT, row.ActTotal)
	assertDecimalEqual(t, "VAT invoice total to register total with VAT", row.RegisterWithVAT, row.VATInvoiceWithVAT)
	for _, amount := range []string{row.SettlementAccessorial, row.ItemExtra, row.ItemPenalties, row.SettlementVAT, row.RegisterVAT, row.VATInvoiceVAT} {
		assertDecimalEqual(t, "zero addition", "0.00", amount)
	}
	if row.SettlementVATRate != "" || row.RegisterVATRate != "" || row.ItemVATRate != "" || row.VATInvoiceVATRate != "" {
		t.Fatalf("a VAT rate was stored: settlement=%q register=%q item=%q invoice=%q", row.SettlementVATRate, row.RegisterVATRate, row.ItemVATRate, row.VATInvoiceVATRate)
	}
	currencies := []string{row.SnapshotCurrency, row.SettlementCurrency, row.RegisterCurrency, row.ActCurrency}
	for _, currency := range currencies {
		if currency != "RUB" {
			t.Fatalf("currency %q", currency)
		}
	}
	if row.PricingSource != "RFQ_AWARD" || row.PricingModel != "SNAPSHOT_V1" || row.SnapshotID != snapshotID.String() {
		t.Fatalf("pricing identity source=%s model=%s snapshot=%s", row.PricingSource, row.PricingModel, row.SnapshotID)
	}
	if row.ActSeller != fix.CarrierA || row.ActBuyer != fix.BuyerID || row.VATSeller != fix.CarrierA || row.VATBuyer != fix.BuyerID {
		t.Fatal("closing document parties do not match the register companies")
	}
	if included.Item == nil || included.Item.ID.String() != row.ItemID {
		t.Fatalf("included item %v stored item %s", included.Item, row.ItemID)
	}
	t.Logf("MONEY_IDENTITY_IDS snapshot=%s settlement=%s register=%s item=%s act=%s vat_invoice=%s",
		row.SnapshotID, first.ID, reg.ID, row.ItemID, created.Act.ID, created.VATInvoice.ID)
}

func attachRateSnapshot(t *testing.T, pool *pgxpool.Pool, fix fixture, amount string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		UPDATE transport.transport_orders
		SET pricing_model_version = 'SNAPSHOT_V1'
		WHERE id = $1 AND tenant_id = $2`, fix.OrderID, fix.TenantID); err != nil {
		t.Fatalf("mark snapshot order: %v", err)
	}
	snapshotID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO transport.transport_order_rate_snapshots (
			id, tenant_id, transport_order_id, buyer_company_id, carrier_company_id,
			pricing_source, award_link_id, rfx_event_id,
			origin_location_id, destination_location_id, equipment_type, transport_mode,
			currency_code, component_breakdown_status, components, accessorial_rules,
			base_amount, total_amount, pricing_date, resolved_at,
			resolved_by_service, resolver_version, resolution_request_hash
		) VALUES (
			$1,$2,$3,$4,$5,
			'RFQ_AWARD',$6,$7,
			$8,$9,'TENT','ROAD',
			'RUB','UNAVAILABLE','[]'::jsonb,'[]'::jsonb,
			NULL,$10::numeric,CURRENT_DATE,now(),
			'finance-pilot-money-identity-v0.2b','TEST',$11
		)`,
		snapshotID, fix.TenantID, fix.OrderID, fix.BuyerID, fix.CarrierA,
		fix.AwardLinkID, fix.EventID, fix.OriginID, fix.DestID, amount, strings.Repeat("a", 64),
	); err != nil {
		t.Fatalf("insert rate snapshot: %v", err)
	}
	return snapshotID
}

type moneyIdentityRow struct {
	SnapshotID            string
	PricingSource         string
	PricingModel          string
	SnapshotCurrency      string
	SettlementCurrency    string
	RegisterCurrency      string
	ActCurrency           string
	SnapshotTotal         string
	SettlementBase        string
	SettlementAccessorial string
	SettlementVAT         string
	SettlementVATRate     string
	ItemID                string
	ItemBase              string
	ItemExtra             string
	ItemPenalties         string
	ItemVATRate           string
	RegisterWithoutVAT    string
	RegisterVAT           string
	RegisterVATRate       string
	RegisterWithVAT       string
	ActTotal              string
	ActSeller             uuid.UUID
	ActBuyer              uuid.UUID
	VATInvoiceWithoutVAT  string
	VATInvoiceVAT         string
	VATInvoiceVATRate     string
	VATInvoiceWithVAT     string
	VATSeller             uuid.UUID
	VATBuyer              uuid.UUID
	AwardAmount           string
}

func readMoneyIdentity(t *testing.T, pool *pgxpool.Pool, tenantID, snapshotID, settlementID, registerID uuid.UUID) moneyIdentityRow {
	t.Helper()
	var row moneyIdentityRow
	var settlementRate, registerRate, itemRate, invoiceRate *string
	err := pool.QueryRow(context.Background(), `
		SELECT snap.id::text, snap.pricing_source, o.pricing_model_version,
			btrim(snap.currency_code), btrim(fs.currency_code), btrim(br.currency_code), btrim(act.currency_code),
			snap.total_amount::text, fs.base_freight_amount::text, fs.approved_accessorial_total::text,
			fs.vat_amount::text, fs.vat_rate::text,
			bri.id::text, bri.base_amount::text, bri.extra_charges::text, bri.penalties::text, bri.vat_rate::text,
			br.total_without_vat::text, br.vat_amount::text, br.vat_rate::text, br.total_with_vat::text,
			act.total_amount::text, act.seller_company_id, act.buyer_company_id,
			vi.amount_without_vat::text, vi.vat_amount::text, vi.vat_rate::text, vi.amount_with_vat::text,
			vi.seller_company_id, vi.buyer_company_id,
			award.amount::text
		FROM transport.transport_order_rate_snapshots snap
		JOIN transport.transport_orders o ON o.id = snap.transport_order_id AND o.tenant_id = snap.tenant_id
		JOIN billing.freight_settlements fs ON fs.rate_snapshot_id = snap.id AND fs.tenant_id = snap.tenant_id
		JOIN billing.billing_registers br ON br.id = fs.billing_register_id AND br.tenant_id = fs.tenant_id
		JOIN billing.billing_register_items bri ON bri.id = fs.billing_register_item_id AND bri.tenant_id = fs.tenant_id
		JOIN billing.acts act ON act.register_id = br.id AND act.tenant_id = br.tenant_id
		JOIN billing.vat_invoices vi ON vi.register_id = br.id AND vi.tenant_id = br.tenant_id
		JOIN rfx.rfx_award_transport_orders award ON award.id = snap.award_link_id AND award.tenant_id = snap.tenant_id
		WHERE snap.id = $1 AND snap.tenant_id = $2 AND fs.id = $3 AND br.id = $4`,
		snapshotID, tenantID, settlementID, registerID).Scan(
		&row.SnapshotID, &row.PricingSource, &row.PricingModel,
		&row.SnapshotCurrency, &row.SettlementCurrency, &row.RegisterCurrency, &row.ActCurrency,
		&row.SnapshotTotal, &row.SettlementBase, &row.SettlementAccessorial,
		&row.SettlementVAT, &settlementRate,
		&row.ItemID, &row.ItemBase, &row.ItemExtra, &row.ItemPenalties, &itemRate,
		&row.RegisterWithoutVAT, &row.RegisterVAT, &registerRate, &row.RegisterWithVAT,
		&row.ActTotal, &row.ActSeller, &row.ActBuyer,
		&row.VATInvoiceWithoutVAT, &row.VATInvoiceVAT, &invoiceRate, &row.VATInvoiceWithVAT,
		&row.VATSeller, &row.VATBuyer, &row.AwardAmount,
	)
	if err != nil {
		t.Fatalf("read stored money: %v", err)
	}
	row.SettlementVATRate = deref(settlementRate)
	row.RegisterVATRate = deref(registerRate)
	row.ItemVATRate = deref(itemRate)
	row.VATInvoiceVATRate = deref(invoiceRate)
	return row
}

func assertDecimalEqual(t *testing.T, name string, values ...string) {
	t.Helper()
	if len(values) < 2 {
		t.Fatalf("%s needs two amounts", name)
	}
	want, err := decimal.NewFromString(values[0])
	if err != nil {
		t.Fatalf("%s parse %s: %v", name, values[0], err)
	}
	for _, raw := range values[1:] {
		got, parseErr := decimal.NewFromString(raw)
		if parseErr != nil {
			t.Fatalf("%s parse %s: %v", name, raw, parseErr)
		}
		if !want.Equal(got) {
			t.Fatalf("%s expected %s actual %s", name, want.StringFixed(2), got.StringFixed(2))
		}
	}
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func countTenantRows(t *testing.T, pool *pgxpool.Pool, table string, tenantID uuid.UUID) int {
	t.Helper()
	var count int
	query := "SELECT COUNT(*) FROM " + table + " WHERE tenant_id=$1"
	if err := pool.QueryRow(context.Background(), query, tenantID).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

func countOutbox(t *testing.T, pool *pgxpool.Pool, tenantID, settlementID uuid.UUID) int {
	t.Helper()
	var count int
	err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM billing.freight_cost_outbox
		WHERE tenant_id=$1 AND aggregate_id=$2`, tenantID, settlementID).Scan(&count)
	if err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	return count
}

func scanRegisterVersion(t *testing.T, pool *pgxpool.Pool, registerID uuid.UUID) int {
	t.Helper()
	var version int
	if err := pool.QueryRow(context.Background(), `
		SELECT version FROM billing.billing_registers WHERE id=$1`, registerID).Scan(&version); err != nil {
		t.Fatalf("register version: %v", err)
	}
	return version
}
