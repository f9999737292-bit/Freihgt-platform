//go:build integration

package bintranse2e

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func continueFinancialClosing(t *testing.T, env *flowEnv, fix flowFixture, shipmentID string) {
	t.Helper()
	orderID := singleText(t, env, `SELECT id::text FROM transport.transport_orders`)
	if decodeShipment(t, mustGetShipment(t, env, shipmentID, actorHeaders(fix.TenantID, fix.CarrierUser))).Status != "DELIVERED" {
		t.Fatal("financial closing requires the original shipment to stay DELIVERED")
	}
	if countRows(t, env.pool, `SELECT COUNT(*) FROM documents.document_upload_intent WHERE tenant_id=$1 AND shipment_id=$2 AND status='completed'`, fix.TenantID, shipmentID) != 1 {
		t.Fatal("financial closing requires the document-service POD upload to be completed")
	}

	carrier := billingHeaders(fix.TenantID, fix.CarrierUser, fix.CarrierID, "CARRIER")
	buyer := billingHeaders(fix.TenantID, fix.BuyerUser, fix.ShipperID, "BUYER")
	otherCarrier := billingHeaders(fix.TenantID, fix.OtherCarrierUser, fix.OtherCarrier, "CARRIER")
	otherBuyer := billingHeaders(fix.TenantID, fix.OtherBuyerUser, fix.OtherShipper, "BUYER")
	foreignCarrier := billingHeaders(fix.ForeignTenant, fix.ForeignCarrierUser, fix.ForeignCarrier, "CARRIER")
	deny := func(name string, status int, payload []byte, want int) {
		t.Helper()
		if status != want {
			t.Fatalf("%s status=%d want=%d body=%s\n%s", name, status, want, payload, env.logs())
		}
	}
	assertChain := func() {
		t.Helper()
		if countRows(t, env.pool, `SELECT COUNT(*) FROM transport.transport_orders`) != 1 ||
			countRows(t, env.pool, `SELECT COUNT(*) FROM transport.shipments`) != 1 ||
			singleText(t, env, `SELECT id::text FROM transport.transport_orders`) != orderID ||
			singleText(t, env, `SELECT id::text FROM transport.shipments`) != shipmentID {
			t.Fatal("financial calls changed the original order or shipment")
		}
		if countRows(t, env.pool, `SELECT COUNT(*) FROM documents.documents WHERE related_entity_id=$1 AND document_type='POD'`, shipmentID) != 1 {
			t.Fatal("financial calls changed the document-service POD")
		}
		if decodeShipment(t, mustGetShipment(t, env, shipmentID, actorHeaders(fix.TenantID, fix.CarrierUser))).Status != "DELIVERED" {
			t.Fatal("financial calls moved the shipment off DELIVERED")
		}
	}

	createBody := map[string]string{
		"shipment_id":     shipmentID,
		"idempotency_key": "e2e-settlement-" + shipmentID,
	}
	status, payload := env.post(t, env.billing, "/v1/freight-settlements", createBody, foreignCarrier)
	deny("foreign tenant settlement create", status, payload, http.StatusNotFound)
	status, payload = env.post(t, env.billing, "/v1/freight-settlements", createBody, otherCarrier)
	deny("other carrier settlement create", status, payload, http.StatusForbidden)
	assertFinanceShape(t, env, 0, 0, 0, 0, 0, 0, 0)
	assertChain()

	status, payload = env.post(t, env.billing, "/v1/freight-settlements", createBody, carrier)
	if status != http.StatusCreated {
		t.Fatalf("create settlement status=%d body=%s\n%s", status, payload, env.logs())
	}
	created := decodeSettlement(t, payload)
	if created.Status != "DRAFT" || created.ShipmentID != shipmentID || created.TransportOrderID != orderID || created.TenantID != fix.TenantID.String() || created.BuyerCompanyID != fix.ShipperID.String() || created.CarrierCompanyID != fix.CarrierID.String() {
		t.Fatalf("created settlement: %+v", created)
	}
	status, payload = env.post(t, env.billing, "/v1/freight-settlements", createBody, carrier)
	if status != http.StatusCreated {
		t.Fatalf("replay create status=%d body=%s\n%s", status, payload, env.logs())
	}
	replayedCreate := decodeSettlement(t, payload)
	if replayedCreate.ID != created.ID || replayedCreate.Status != "DRAFT" {
		t.Fatalf("replay create: %+v", replayedCreate)
	}
	if countRows(t, env.pool, `SELECT COUNT(*) FROM billing.freight_settlements`) != 1 {
		t.Fatal("replayed create inserted another settlement")
	}

	status, payload = env.get(t, env.billing, "/v1/freight-settlements/"+created.ID, foreignCarrier)
	deny("foreign tenant settlement read", status, payload, http.StatusNotFound)
	status, payload = env.get(t, env.billing, "/v1/freight-settlements/"+created.ID, otherCarrier)
	deny("other carrier settlement read", status, payload, http.StatusForbidden)
	status, payload = env.get(t, env.billing, "/v1/freight-settlements/"+created.ID, otherBuyer)
	deny("other buyer settlement read", status, payload, http.StatusForbidden)
	if countRows(t, env.pool, `SELECT COUNT(*) FROM billing.freight_settlements`) != 1 {
		t.Fatal("denied settlement access inserted a row")
	}

	status, payload = env.post(t, env.billing, "/v1/freight-settlements/"+created.ID+"/submit-for-review", map[string]any{}, carrier)
	submitted := expectSettlement(t, env, status, payload, http.StatusOK, "UNDER_REVIEW", created.ID)
	if submitted.ShipmentID != shipmentID {
		t.Fatalf("submitted settlement shipment: %+v", submitted)
	}
	status, payload = env.post(t, env.billing, "/v1/freight-settlements/"+created.ID+"/submit-for-review", map[string]any{}, carrier)
	deny("replay settlement submit", status, payload, http.StatusBadRequest)
	if countRows(t, env.pool, `SELECT COUNT(*) FROM billing.settlement_audit_events WHERE settlement_id=$1 AND event_type='SETTLEMENT_STATUS_UNDER_REVIEW'`, created.ID) != 1 {
		t.Fatal("replayed submit wrote another audit event")
	}

	status, payload = env.post(t, env.billing, "/v1/freight-settlements/"+created.ID+"/approve", map[string]any{}, otherBuyer)
	deny("other buyer settlement approve", status, payload, http.StatusForbidden)
	status, payload = env.post(t, env.billing, "/v1/freight-settlements/"+created.ID+"/approve", map[string]any{}, buyer)
	approved := expectSettlement(t, env, status, payload, http.StatusOK, "APPROVED", created.ID)
	if approved.TransportOrderID != orderID || approved.BuyerCompanyID != fix.ShipperID.String() || approved.CarrierCompanyID != fix.CarrierID.String() {
		t.Fatalf("approved settlement identity: %+v", approved)
	}
	status, payload = env.post(t, env.billing, "/v1/freight-settlements/"+created.ID+"/approve", map[string]any{}, buyer)
	deny("replay settlement approve", status, payload, http.StatusBadRequest)
	if countRows(t, env.pool, `SELECT COUNT(*) FROM billing.settlement_audit_events WHERE settlement_id=$1 AND event_type='SETTLEMENT_STATUS_APPROVED'`, created.ID) != 1 ||
		countRows(t, env.pool, `SELECT COUNT(*) FROM billing.freight_settlements`) != 1 {
		t.Fatal("replayed settlement approve wrote another financial row")
	}

	status, payload = env.post(t, env.billing, "/v1/freight-settlements/"+created.ID+"/include-in-register", map[string]any{}, buyer)
	included := expectSettlement(t, env, status, payload, http.StatusOK, "APPROVED", created.ID)
	if included.BillingRegisterID == nil || included.BillingRegisterItemID == nil || *included.BillingRegisterID == "" || *included.BillingRegisterItemID == "" {
		t.Fatalf("include result: %+v", included)
	}
	registerID := *included.BillingRegisterID
	itemID := *included.BillingRegisterItemID
	beforeReplay := readIncludeFingerprint(t, env, fix.TenantID, created.ID)
	status, payload = env.post(t, env.billing, "/v1/freight-settlements/"+created.ID+"/include-in-register", map[string]any{}, otherBuyer)
	denyIncludePayload(t, env, "other buyer include replay", status, payload, http.StatusForbidden, registerID, itemID)
	status, payload = env.post(t, env.billing, "/v1/freight-settlements/"+created.ID+"/include-in-register", map[string]any{}, otherCarrier)
	denyIncludePayload(t, env, "other carrier include replay", status, payload, http.StatusForbidden, registerID, itemID)
	status, payload = env.post(t, env.billing, "/v1/freight-settlements/"+created.ID+"/include-in-register", map[string]any{}, foreignCarrier)
	denyIncludePayload(t, env, "foreign tenant include replay", status, payload, http.StatusNotFound, registerID, itemID)
	if got := readIncludeFingerprint(t, env, fix.TenantID, created.ID); got != beforeReplay {
		t.Fatalf("denied include replay changed financial state before=%+v after=%+v", beforeReplay, got)
	}
	status, payload = env.post(t, env.billing, "/v1/freight-settlements/"+created.ID+"/include-in-register", map[string]any{}, buyer)
	includedAgain := expectSettlement(t, env, status, payload, http.StatusOK, "APPROVED", created.ID)
	if includedAgain.BillingRegisterID == nil || *includedAgain.BillingRegisterID != registerID || includedAgain.BillingRegisterItemID == nil || *includedAgain.BillingRegisterItemID != itemID {
		t.Fatalf("include replay: %+v", includedAgain)
	}
	if got := readIncludeFingerprint(t, env, fix.TenantID, created.ID); got != beforeReplay {
		t.Fatalf("buyer include replay changed financial state before=%+v after=%+v", beforeReplay, got)
	}
	assertFinanceShape(t, env, 1, 1, 1, 0, 0, 0, 0)

	mismatch := map[string]string{"tenant_id": fix.ForeignTenant.String()}
	status, payload = env.post(t, env.billing, "/v1/billing-registers/"+registerID+"/calculate", mismatch, buyer)
	deny("calculate tenant mismatch", status, payload, http.StatusForbidden)
	if registerStatus(t, env, registerID) != "DRAFT" {
		t.Fatal("rejected calculate changed the register")
	}
	status, payload = env.post(t, env.billing, "/v1/billing-registers/"+registerID+"/calculate", nil, buyer)
	calculated := expectRegister(t, env, status, payload, http.StatusOK, "CALCULATED", registerID)
	status, payload = env.post(t, env.billing, "/v1/billing-registers/"+registerID+"/calculate", nil, buyer)
	calculatedAgain := expectRegister(t, env, status, payload, http.StatusOK, "CALCULATED", registerID)
	if calculatedAgain.Version != calculated.Version || calculatedAgain.TotalWithVAT != calculated.TotalWithVAT {
		t.Fatalf("calculate replay changed the register: %+v -> %+v", calculated, calculatedAgain)
	}

	status, payload = env.post(t, env.billing, "/v1/billing-registers/"+registerID+"/approve", mismatch, buyer)
	deny("register approve tenant mismatch", status, payload, http.StatusForbidden)
	if registerStatus(t, env, registerID) != "CALCULATED" {
		t.Fatal("rejected register approve changed the register")
	}
	status, payload = env.post(t, env.billing, "/v1/billing-registers/"+registerID+"/approve", map[string]any{}, buyer)
	registerApproved := expectRegister(t, env, status, payload, http.StatusOK, "APPROVED", registerID)
	status, payload = env.post(t, env.billing, "/v1/billing-registers/"+registerID+"/approve", map[string]any{}, buyer)
	registerApprovedAgain := expectRegister(t, env, status, payload, http.StatusOK, "APPROVED", registerID)
	if registerApprovedAgain.Version != registerApproved.Version || registerApprovedAgain.Status != "APPROVED" {
		t.Fatalf("register approve replay: %+v", registerApprovedAgain)
	}

	status, payload = env.post(t, env.billing, "/v1/billing-registers/"+registerID+"/closing-document-package", mismatch, buyer)
	deny("package tenant mismatch", status, payload, http.StatusForbidden)
	assertFinanceShape(t, env, 1, 1, 1, 0, 0, 0, 0)
	status, payload = env.post(t, env.billing, "/v1/billing-registers/"+registerID+"/closing-document-package", map[string]any{}, buyer)
	if status != http.StatusCreated {
		t.Fatalf("package status=%d body=%s\n%s", status, payload, env.logs())
	}
	pkg := decodePackage(t, payload)
	if pkg.Package.ID == "" || pkg.Package.RegisterID != registerID || pkg.Package.PackageType != "ACT_PLUS_VAT_INVOICE" || pkg.Act == nil || pkg.Act.ID == "" || pkg.VATInvoice == nil || pkg.VATInvoice.ID == "" {
		t.Fatalf("package: %+v", pkg)
	}
	if registerStatus(t, env, registerID) != "CLOSING_DOCUMENTS_CREATED" {
		t.Fatalf("register status after package=%s", registerStatus(t, env, registerID))
	}
	status, payload = env.post(t, env.billing, "/v1/billing-registers/"+registerID+"/closing-document-package", map[string]any{}, buyer)
	if status != http.StatusCreated {
		t.Fatalf("package replay status=%d body=%s\n%s", status, payload, env.logs())
	}
	pkgAgain := decodePackage(t, payload)
	if pkgAgain.Package.ID != pkg.Package.ID || pkgAgain.Act == nil || pkgAgain.Act.ID != pkg.Act.ID || pkgAgain.VATInvoice == nil || pkgAgain.VATInvoice.ID != pkg.VATInvoice.ID {
		t.Fatalf("package replay: %+v", pkgAgain)
	}
	assertFinanceShape(t, env, 1, 1, 1, 1, 1, 1, 0)
	if countRows(t, env.pool, `SELECT COUNT(*) FROM billing.billing_registers WHERE status IN ('PAID','CLOSED')`) != 0 {
		t.Fatal("register reached PAID or CLOSED")
	}
	if countRows(t, env.pool, `SELECT COUNT(*) FROM billing.settlement_audit_events WHERE settlement_id=$1 AND event_type='SETTLEMENT_CREATED'`, created.ID) != 1 {
		t.Fatal("missing settlement create audit event")
	}
	if countRows(t, env.pool, `SELECT COUNT(*) FROM billing.freight_cost_outbox WHERE tenant_id=$1 AND aggregate_id=$2 AND status='PENDING'`, fix.TenantID, created.ID) == 0 {
		t.Fatal("freight cost outbox has no pending settlement event")
	}
	assertRateSnapshot(t, env, fix, created.ID, shipmentID, orderID, registerID, itemID)
	status, payload = env.get(t, env.documents, "/v1/documents/"+singleText(t, env, `SELECT id::text FROM documents.documents WHERE related_entity_id=$1 AND document_type='POD'`, shipmentID), map[string]string{"X-Tenant-ID": fix.ForeignTenant.String()})
	deny("foreign tenant document after billing", status, payload, http.StatusNotFound)
	assertChain()
}

type includeFingerprint struct {
	registers, items, audits, outbox, version  int
	baseAmount, settlementTotal, registerTotal string
}

func readIncludeFingerprint(t *testing.T, env *flowEnv, tenantID uuid.UUID, settlementID string) includeFingerprint {
	t.Helper()
	var got includeFingerprint
	err := env.pool.QueryRow(context.Background(), `
		SELECT
			(SELECT COUNT(*) FROM billing.billing_registers),
			(SELECT COUNT(*) FROM billing.billing_register_items),
			(SELECT COUNT(*) FROM billing.settlement_audit_events WHERE settlement_id = fs.id),
			(SELECT COUNT(*) FROM billing.freight_cost_outbox WHERE tenant_id = fs.tenant_id AND aggregate_id = fs.id),
			fs.version,
			fs.base_freight_amount::text,
			fs.total_with_vat::text,
			br.total_with_vat::text
		FROM billing.freight_settlements fs
		JOIN billing.billing_registers br ON br.id = fs.billing_register_id AND br.tenant_id = fs.tenant_id
		WHERE fs.id = $1 AND fs.tenant_id = $2`, settlementID, tenantID).Scan(
		&got.registers, &got.items, &got.audits, &got.outbox, &got.version,
		&got.baseAmount, &got.settlementTotal, &got.registerTotal,
	)
	if err != nil {
		t.Fatalf("include fingerprint: %v", err)
	}
	return got
}

func denyIncludePayload(t *testing.T, env *flowEnv, name string, status int, payload []byte, want int, registerID, itemID string) {
	t.Helper()
	if status != want {
		t.Fatalf("%s status=%d want=%d body=%s\n%s", name, status, want, payload, env.logs())
	}
	body := string(payload)
	for _, secret := range []string{registerID, itemID, "base_freight_amount", "billing_register_id", "total_with_vat", "vat_amount"} {
		if strings.Contains(body, secret) {
			t.Fatalf("%s leaked %s in %s", name, secret, body)
		}
	}
}

func assertFinanceShape(t *testing.T, env *flowEnv, settlements, registers, items, packages, acts, vatInvoices, payments int) {
	t.Helper()
	checks := []struct {
		name  string
		query string
		want  int
	}{
		{"settlements", `SELECT COUNT(*) FROM billing.freight_settlements`, settlements},
		{"registers", `SELECT COUNT(*) FROM billing.billing_registers`, registers},
		{"items", `SELECT COUNT(*) FROM billing.billing_register_items`, items},
		{"packages", `SELECT COUNT(*) FROM billing.closing_document_packages`, packages},
		{"acts", `SELECT COUNT(*) FROM billing.acts`, acts},
		{"vat invoices", `SELECT COUNT(*) FROM billing.vat_invoices`, vatInvoices},
		{"payments", `SELECT COUNT(*) FROM billing.payment_obligations`, payments},
	}
	for _, check := range checks {
		if got := countRows(t, env.pool, check.query); got != check.want {
			t.Fatalf("%s rows=%d want=%d", check.name, got, check.want)
		}
	}
}

func assertRateSnapshot(t *testing.T, env *flowEnv, fix flowFixture, settlementID, shipmentID, orderID, registerID, itemID string) {
	t.Helper()
	var (
		pricingModel, currency                  string
		baseMatches, currencyMatches, vatIsZero bool
		buyerMatches, carrierMatches            bool
		orderMatches, shipmentMatches           bool
		itemShipment, itemBase, itemVAT         bool
		registerCustomer, registerContractor    bool
		registerMoney, accessorials             int
	)
	err := env.pool.QueryRow(context.Background(), `
		SELECT o.pricing_model_version,
			fs.currency_code,
			fs.base_freight_amount = snap.total_amount,
			fs.currency_code = snap.currency_code,
			fs.vat_amount = 0,
			fs.buyer_company_id = $2 AND snap.buyer_company_id = $2,
			fs.carrier_company_id = $3 AND snap.carrier_company_id = $3 AND s.carrier_company_id = $3,
			fs.transport_order_id = $4 AND snap.transport_order_id = $4,
			fs.shipment_id = $5 AND s.id = $5 AND s.status = 'DELIVERED',
			(SELECT COUNT(*) FROM billing.settlement_accessorials WHERE settlement_id = fs.id)
		FROM billing.freight_settlements fs
		JOIN transport.transport_order_rate_snapshots snap ON snap.id = fs.rate_snapshot_id AND snap.tenant_id = fs.tenant_id
		JOIN transport.transport_orders o ON o.id = fs.transport_order_id AND o.tenant_id = fs.tenant_id
		JOIN transport.shipments s ON s.id = fs.shipment_id AND s.tenant_id = fs.tenant_id
		WHERE fs.id = $1 AND fs.tenant_id = $6`,
		settlementID, fix.ShipperID, fix.CarrierID, orderID, shipmentID, fix.TenantID).Scan(
		&pricingModel, &currency, &baseMatches, &currencyMatches, &vatIsZero,
		&buyerMatches, &carrierMatches, &orderMatches, &shipmentMatches, &accessorials,
	)
	if err != nil {
		t.Fatalf("rate snapshot identity: %v", err)
	}
	if pricingModel != "SNAPSHOT_V1" || currency == "" || !baseMatches || !currencyMatches || !vatIsZero || !buyerMatches || !carrierMatches || !orderMatches || !shipmentMatches || accessorials != 0 {
		t.Fatalf("snapshot identity model=%s currency=%s base=%v currencyMatch=%v vatZero=%v buyer=%v carrier=%v order=%v shipment=%v accessorials=%d",
			pricingModel, currency, baseMatches, currencyMatches, vatIsZero, buyerMatches, carrierMatches, orderMatches, shipmentMatches, accessorials)
	}
	err = env.pool.QueryRow(context.Background(), `
		SELECT bri.shipment_id = $2,
			bri.base_amount = snap.total_amount,
			bri.vat_amount = 0,
			br.customer_company_id = $3,
			br.contractor_company_id = $4,
			CASE WHEN br.total_without_vat = snap.total_amount AND br.vat_amount = 0 AND br.total_with_vat = snap.total_amount AND br.status = 'CLOSING_DOCUMENTS_CREATED' THEN 1 ELSE 0 END
		FROM billing.billing_register_items bri
		JOIN billing.billing_registers br ON br.id = bri.register_id
		JOIN billing.freight_settlements fs ON fs.id = bri.settlement_id
		JOIN transport.transport_order_rate_snapshots snap ON snap.id = fs.rate_snapshot_id
		WHERE bri.id = $1 AND br.id = $5`,
		itemID, shipmentID, fix.ShipperID, fix.CarrierID, registerID).Scan(
		&itemShipment, &itemBase, &itemVAT, &registerCustomer, &registerContractor, &registerMoney,
	)
	if err != nil {
		t.Fatalf("register item identity: %v", err)
	}
	if !itemShipment || !itemBase || !itemVAT || !registerCustomer || !registerContractor || registerMoney != 1 {
		t.Fatalf("register money itemShipment=%v itemBase=%v itemVAT=%v customer=%v contractor=%v money=%d",
			itemShipment, itemBase, itemVAT, registerCustomer, registerContractor, registerMoney)
	}
}

func registerStatus(t *testing.T, env *flowEnv, registerID string) string {
	t.Helper()
	return singleText(t, env, `SELECT status FROM billing.billing_registers WHERE id=$1`, registerID)
}

func singleText(t *testing.T, env *flowEnv, query string, args ...any) string {
	t.Helper()
	var value string
	if err := env.pool.QueryRow(context.Background(), query, args...).Scan(&value); err != nil {
		t.Fatalf("query %s: %v", query, err)
	}
	return value
}

func expectSettlement(t *testing.T, env *flowEnv, status int, payload []byte, want int, wantStatus, wantID string) settlementHTTP {
	t.Helper()
	if status != want {
		t.Fatalf("settlement status=%d want=%d body=%s\n%s", status, want, payload, env.logs())
	}
	body := decodeSettlement(t, payload)
	if body.ID != wantID || body.Status != wantStatus {
		t.Fatalf("settlement result: %+v", body)
	}
	return body
}

func expectRegister(t *testing.T, env *flowEnv, status int, payload []byte, want int, wantStatus, wantID string) registerHTTP {
	t.Helper()
	if status != want {
		t.Fatalf("register status=%d want=%d body=%s\n%s", status, want, payload, env.logs())
	}
	body := decodeRegister(t, payload)
	if body.ID != wantID || body.Status != wantStatus {
		t.Fatalf("register result: %+v", body)
	}
	return body
}

func decodeSettlement(t *testing.T, payload []byte) settlementHTTP {
	t.Helper()
	var body settlementHTTP
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("settlement json: %v body=%s", err, payload)
	}
	return body
}

func decodeRegister(t *testing.T, payload []byte) registerHTTP {
	t.Helper()
	var body registerHTTP
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("register json: %v body=%s", err, payload)
	}
	return body
}

func decodePackage(t *testing.T, payload []byte) packageHTTP {
	t.Helper()
	var body packageHTTP
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("package json: %v body=%s", err, payload)
	}
	return body
}

type settlementHTTP struct {
	ID                    string  `json:"id"`
	TenantID              string  `json:"tenant_id"`
	ShipmentID            string  `json:"shipment_id"`
	TransportOrderID      string  `json:"transport_order_id"`
	BuyerCompanyID        string  `json:"buyer_company_id"`
	CarrierCompanyID      string  `json:"carrier_company_id"`
	Status                string  `json:"status"`
	BillingRegisterID     *string `json:"billing_register_id"`
	BillingRegisterItemID *string `json:"billing_register_item_id"`
}

type registerHTTP struct {
	ID           string  `json:"id"`
	Status       string  `json:"status"`
	Version      int     `json:"version"`
	TotalWithVAT float64 `json:"total_with_vat"`
}

type packageHTTP struct {
	Package struct {
		ID          string `json:"id"`
		RegisterID  string `json:"register_id"`
		PackageType string `json:"package_type"`
	} `json:"package"`
	Act *struct {
		ID string `json:"id"`
	} `json:"act"`
	VATInvoice *struct {
		ID string `json:"id"`
	} `json:"vat_invoice"`
}
