//go:build integration

package bintranse2e

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBintransE2ETransportFlow(t *testing.T) {
	env := newFlowEnv(t)
	fix := seedFlowFixture(t, env.pool)
	ctx := context.Background()
	buyer := actorHeaders(fix.TenantID, fix.BuyerUser)
	carrier := actorHeaders(fix.TenantID, fix.CarrierUser)

	eventID, lotID := awardOneLot(t, env, fix, buyer, carrier)
	if countRows(t, env.pool, `SELECT COUNT(*) FROM transport.transport_orders`) != 0 {
		t.Fatal("award must not insert a transport order")
	}

	status, payload := env.post(t, env.rfx, "/v1/rfx-events/"+eventID+"/transport-orders", nil, buyer)
	if status != http.StatusCreated {
		t.Fatalf("convert status=%d body=%s\n%s", status, payload, env.logs())
	}
	var created convertView
	if err := json.Unmarshal(payload, &created); err != nil {
		t.Fatalf("convert json: %v", err)
	}
	if !created.Created || len(created.Items) != 1 {
		t.Fatalf("convert result: %+v", created)
	}
	item := created.Items[0]
	if item.RfxEventID != eventID || item.RfxLotID != lotID || item.CarrierCompanyID != fix.CarrierID.String() || item.TransportOrderID == "" || item.RfxAwardID == "" {
		t.Fatalf("link: %+v", item)
	}
	orderID, err := uuid.Parse(item.TransportOrderID)
	if err != nil {
		t.Fatalf("order id: %v", err)
	}

	var orderTenant, shipperID uuid.UUID
	var source string
	if err := env.pool.QueryRow(ctx, `
		SELECT tenant_id, shipper_company_id, source_system
		FROM transport.transport_orders WHERE id = $1`, orderID).Scan(&orderTenant, &shipperID, &source); err != nil {
		t.Fatalf("order row: %v", err)
	}
	if orderTenant != fix.TenantID || shipperID != fix.ShipperID || source != "rfx_award" {
		t.Fatalf("order tenant=%s shipper=%s source=%s", orderTenant, shipperID, source)
	}
	var idempotencyKey string
	if err := env.pool.QueryRow(ctx, `
		SELECT idempotency_key FROM transport.transport_order_create_idempotency
		WHERE transport_order_id = $1 AND tenant_id = $2`, orderID, fix.TenantID).Scan(&idempotencyKey); err != nil {
		t.Fatalf("idempotency row: %v", err)
	}
	wantKey := "award-conv:" + fix.TenantID.String() + ":" + eventID + ":" + lotID
	if idempotencyKey != wantKey {
		t.Fatalf("idempotency key=%s want=%s", idempotencyKey, wantKey)
	}
	if countRows(t, env.pool, `SELECT COUNT(*) FROM transport.transport_orders`) != 1 {
		t.Fatal("expected one transport order after conversion")
	}
	if countRows(t, env.pool, `SELECT COUNT(*) FROM transport.shipments`) != 0 {
		t.Fatal("conversion must not insert a shipment")
	}

	status, payload = env.post(t, env.shipment, "/v1/order-execution/transport-orders/"+item.TransportOrderID+"/execute?carrier_company_id="+fix.CarrierID.String(),
		map[string]string{"shipment_number": "SHP-E2E-1"}, carrier)
	if status != http.StatusCreated {
		t.Fatalf("execute status=%d body=%s\n%s", status, payload, env.logs())
	}
	var executed executeView
	if err := json.Unmarshal(payload, &executed); err != nil {
		t.Fatalf("execute json: %v body=%s", err, payload)
	}
	if !executed.Created || executed.TransportOrderID != item.TransportOrderID || executed.RfxEventID != eventID || executed.RfxAwardID != item.RfxAwardID {
		t.Fatalf("execute correlation: %+v", executed)
	}
	if executed.Shipment.TenantID != fix.TenantID.String() || deref(executed.Shipment.TransportOrderID) != item.TransportOrderID || deref(executed.Shipment.CarrierCompanyID) != fix.CarrierID.String() {
		t.Fatalf("shipment correlation: %+v", executed.Shipment)
	}
	shipmentID := executed.Shipment.ID

	deny := func(name string, status int, payload []byte, want int) {
		t.Helper()
		if status != want {
			t.Fatalf("%s status=%d want=%d body=%s\n%s", name, status, want, payload, env.logs())
		}
	}
	status, payload = env.post(t, env.shipment, "/v1/order-execution/transport-orders/"+item.TransportOrderID+"/execute?carrier_company_id="+fix.OtherCarrier.String(),
		map[string]string{"shipment_number": "SHP-E2E-OTHER"}, carrier)
	deny("other carrier execute", status, payload, http.StatusForbidden)
	status, payload = env.post(t, env.shipment, "/v1/order-execution/transport-orders/"+item.TransportOrderID+"/execute?carrier_company_id="+fix.ForeignCarrier.String(),
		map[string]string{"shipment_number": "SHP-E2E-FOREIGN"}, actorHeaders(fix.ForeignTenant, uuid.New()))
	deny("foreign tenant execute", status, payload, http.StatusNotFound)
	status, payload = env.post(t, env.shipment, "/v1/shipments/"+shipmentID+"/assign-driver",
		map[string]string{"driver_id": fix.OtherDriver.String()}, carrier)
	deny("other carrier driver", status, payload, http.StatusBadRequest)
	status, payload = env.post(t, env.shipment, "/v1/shipments/"+shipmentID+"/assign-vehicle",
		map[string]string{"vehicle_id": fix.OtherVehicle.String()}, carrier)
	deny("other carrier vehicle", status, payload, http.StatusBadRequest)
	status, payload = env.post(t, env.shipment, "/v1/shipments/"+shipmentID+"/assign-driver",
		map[string]string{"driver_id": fix.ForeignDriver.String()}, carrier)
	deny("foreign driver", status, payload, http.StatusNotFound)
	status, payload = env.post(t, env.shipment, "/v1/shipments/"+shipmentID+"/assign-vehicle",
		map[string]string{"vehicle_id": fix.ForeignVehicle.String()}, carrier)
	deny("foreign vehicle", status, payload, http.StatusNotFound)

	status, payload = env.get(t, env.shipment, "/v1/shipments/"+shipmentID, carrier)
	if status != http.StatusOK {
		t.Fatalf("get shipment status=%d body=%s", status, payload)
	}
	unchanged := decodeShipment(t, payload)
	if deref(unchanged.DriverID) != "" || deref(unchanged.VehicleID) != "" {
		t.Fatalf("rejected assignment changed the shipment: %+v", unchanged)
	}

	status, payload = env.post(t, env.shipment, "/v1/shipments/"+shipmentID+"/assign-vehicle",
		map[string]string{"vehicle_id": fix.VehicleID.String()}, carrier)
	if status != http.StatusOK {
		t.Fatalf("assign vehicle status=%d body=%s\n%s", status, payload, env.logs())
	}
	vehicleAssigned := decodeShipment(t, payload)
	if deref(vehicleAssigned.VehicleID) != fix.VehicleID.String() || deref(vehicleAssigned.TransportOrderID) != item.TransportOrderID {
		t.Fatalf("vehicle assignment: %+v", vehicleAssigned)
	}
	status, payload = env.post(t, env.shipment, "/v1/shipments/"+shipmentID+"/assign-driver",
		map[string]string{"driver_id": fix.DriverID.String()}, carrier)
	if status != http.StatusOK {
		t.Fatalf("assign driver status=%d body=%s\n%s", status, payload, env.logs())
	}
	driverAssigned := decodeShipment(t, payload)
	if deref(driverAssigned.DriverID) != fix.DriverID.String() || deref(driverAssigned.VehicleID) != fix.VehicleID.String() || deref(driverAssigned.CarrierCompanyID) != fix.CarrierID.String() {
		t.Fatalf("driver assignment: %+v", driverAssigned)
	}

	status, payload = env.post(t, env.rfx, "/v1/rfx-events/"+eventID+"/transport-orders", nil, buyer)
	if status != http.StatusOK {
		t.Fatalf("repeat convert status=%d body=%s\n%s", status, payload, env.logs())
	}
	var repeatedConvert convertView
	if err := json.Unmarshal(payload, &repeatedConvert); err != nil {
		t.Fatalf("repeat convert json: %v", err)
	}
	if repeatedConvert.Created || len(repeatedConvert.Items) != 1 || repeatedConvert.Items[0].TransportOrderID != item.TransportOrderID {
		t.Fatalf("repeat convert: %+v", repeatedConvert)
	}
	status, payload = env.post(t, env.shipment, "/v1/order-execution/transport-orders/"+item.TransportOrderID+"/execute?carrier_company_id="+fix.CarrierID.String(),
		map[string]string{"shipment_number": "SHP-E2E-1"}, carrier)
	if status != http.StatusOK {
		t.Fatalf("repeat execute status=%d body=%s\n%s", status, payload, env.logs())
	}
	var repeated executeView
	if err := json.Unmarshal(payload, &repeated); err != nil {
		t.Fatalf("repeat execute json: %v", err)
	}
	if repeated.Created || repeated.Shipment.ID != shipmentID {
		t.Fatalf("repeat execute created a shipment: %+v", repeated)
	}
	if countRows(t, env.pool, `SELECT COUNT(*) FROM transport.transport_orders`) != 1 ||
		countRows(t, env.pool, `SELECT COUNT(*) FROM transport.shipments`) != 1 ||
		countRows(t, env.pool, `SELECT COUNT(*) FROM rfx.rfx_award_transport_orders`) != 1 {
		t.Fatal("duplicate conversion or execution created another row")
	}

	status, payload = env.post(t, env.rfx, "/v1/rfx-events/"+eventID+"/transport-orders", nil, actorHeaders(fix.ForeignTenant, uuid.New()))
	deny("foreign tenant convert", status, payload, http.StatusNotFound)
	if countRows(t, env.pool, `SELECT COUNT(*) FROM transport.transport_orders`) != 1 {
		t.Fatal("foreign tenant conversion created an order")
	}

	status, payload = env.post(t, env.orders, "/v1/transport-orders", map[string]string{
		"tenant_id":    fix.ForeignTenant.String(),
		"order_number": "TO-MISMATCH",
	}, map[string]string{
		"X-Tenant-ID":     fix.TenantID.String(),
		"X-User-ID":       fix.BuyerUser.String(),
		"X-Company-ID":    fix.ShipperID.String(),
		"X-Actor-Kind":    "BUYER",
		"Idempotency-Key": "public-create-tenant-mismatch",
	})
	deny("public create tenant mismatch", status, payload, http.StatusForbidden)
	if countRows(t, env.pool, `SELECT COUNT(*) FROM transport.transport_orders`) != 1 {
		t.Fatal("mismatched public create inserted an order")
	}
}

func awardOneLot(t *testing.T, env *flowEnv, fix flowFixture, buyer, carrier map[string]string) (eventID, lotID string) {
	t.Helper()
	deadline := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
	status, payload := env.post(t, env.rfx, "/v1/rfx-events/", map[string]any{
		"tenant_id":         fix.TenantID.String(),
		"owner_company_id":  fix.ShipperID.String(),
		"title":             "BINTRANS E2E Lot",
		"rfx_type":          "SPOT_RFQ",
		"category":          "FREIGHT",
		"rfx_number":        "RFX-E2E-" + uuid.NewString()[:8],
		"currency_code":     "RUB",
		"response_deadline": deadline,
	}, buyer)
	if status != http.StatusCreated {
		t.Fatalf("create event status=%d body=%s\n%s", status, payload, env.logs())
	}
	eventID = jsonID(t, payload)

	status, payload = env.post(t, env.rfx, "/v1/rfx-events/"+eventID+"/participants", map[string]string{
		"tenant_id":        fix.TenantID.String(),
		"company_id":       fix.CarrierID.String(),
		"participant_type": "CARRIER",
	}, buyer)
	if status != http.StatusCreated && status != http.StatusOK {
		t.Fatalf("participant status=%d body=%s\n%s", status, payload, env.logs())
	}

	status, payload = env.post(t, env.rfx, "/v1/rfx-events/"+eventID+"/lots", map[string]string{
		"tenant_id":  fix.TenantID.String(),
		"lot_number": "LOT-1",
		"name":       "Single lot",
	}, buyer)
	if status != http.StatusCreated {
		t.Fatalf("lot status=%d body=%s\n%s", status, payload, env.logs())
	}
	lotID = jsonID(t, payload)

	status, payload = env.post(t, env.rfx, "/v1/rfx-lots/"+lotID+"/lanes", map[string]any{
		"tenant_id":               fix.TenantID.String(),
		"origin_location_id":      fix.OriginID.String(),
		"destination_location_id": fix.DestID.String(),
		"transport_mode":          "ROAD",
		"equipment_type":          "TAUTLINER",
	}, buyer)
	if status != http.StatusCreated {
		t.Fatalf("lane status=%d body=%s\n%s", status, payload, env.logs())
	}

	status, payload = env.post(t, env.rfx, "/v1/rfx-events/"+eventID+"/publish", nil, buyer)
	if status != http.StatusOK {
		t.Fatalf("publish status=%d body=%s\n%s", status, payload, env.logs())
	}

	status, payload = env.post(t, env.rfx, "/v1/rfx-events/"+eventID+"/responses", map[string]string{
		"tenant_id":              fix.TenantID.String(),
		"participant_company_id": fix.CarrierID.String(),
	}, carrier)
	if status != http.StatusCreated {
		t.Fatalf("response status=%d body=%s\n%s", status, payload, env.logs())
	}
	responseID := jsonID(t, payload)

	status, payload = env.patch(t, env.rfx, "/v1/rfx-responses/"+responseID, map[string]any{
		"offer_lines": []map[string]any{{
			"rfx_lot_id":    lotID,
			"amount":        100000,
			"currency_code": "RUB",
		}},
	}, carrier)
	if status != http.StatusOK {
		t.Fatalf("commercial status=%d body=%s\n%s", status, payload, env.logs())
	}

	status, payload = env.post(t, env.rfx, "/v1/rfx-responses/"+responseID+"/submit", nil, carrier)
	if status != http.StatusOK {
		t.Fatalf("submit status=%d body=%s\n%s", status, payload, env.logs())
	}

	status, payload = env.post(t, env.rfx, "/v1/rfx-events/"+eventID+"/award-response", map[string]string{
		"response_id": responseID,
	}, buyer)
	if status != http.StatusOK {
		t.Fatalf("award status=%d body=%s\n%s", status, payload, env.logs())
	}
	return eventID, lotID
}

func decodeShipment(t *testing.T, payload []byte) shipmentView {
	t.Helper()
	var body shipmentView
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("shipment json: %v body=%s", err, payload)
	}
	return body
}
