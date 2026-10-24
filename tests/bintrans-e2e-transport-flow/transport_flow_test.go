//go:build integration

package bintranse2e

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
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

	continueDeliveryAndPOD(t, env, fix, carrier, shipmentID)
	continueFinancialClosing(t, env, fix, shipmentID)
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

func continueDeliveryAndPOD(t *testing.T, env *flowEnv, fix flowFixture, carrier map[string]string, shipmentID string) {
	t.Helper()
	driver := actorHeaders(fix.TenantID, fix.DriverUser)
	otherDriver := actorHeaders(fix.TenantID, fix.OtherDriverUser)
	foreignDriver := actorHeaders(fix.ForeignTenant, fix.ForeignDriverUser)
	deny := func(name string, status int, payload []byte, want int) {
		t.Helper()
		if status != want {
			t.Fatalf("%s status=%d want=%d body=%s\n%s", name, status, want, payload, env.logs())
		}
	}

	status, payload := env.post(t, env.shipment, "/v1/driver/me/shipments/"+shipmentID+"/events", map[string]string{
		"type": "ARRIVED_AT_PICKUP", "idempotencyKey": "other-driver-pickup",
	}, otherDriver)
	deny("other driver event", status, payload, http.StatusNotFound)
	status, payload = env.post(t, env.shipment, "/v1/driver/me/shipments/"+shipmentID+"/events", map[string]string{
		"type": "ARRIVED_AT_PICKUP", "idempotencyKey": "foreign-driver-pickup",
	}, foreignDriver)
	deny("foreign tenant driver event", status, payload, http.StatusNotFound)
	if countRows(t, env.pool, `SELECT COUNT(*) FROM transport.shipment_status_history WHERE shipment_id=$1 AND to_status='IN_PICKUP'`, shipmentID) != 0 {
		t.Fatal("denied driver event wrote a pickup transition")
	}

	status, payload = env.patch(t, env.shipment, "/v1/shipments/"+shipmentID+"/status", map[string]string{"status": "PICKUP_SLOT_BOOKED"}, carrier)
	if status != http.StatusOK {
		t.Fatalf("book pickup status=%d body=%s\n%s", status, payload, env.logs())
	}

	steps := []struct {
		event  string
		status string
	}{
		{"ARRIVED_AT_PICKUP", "IN_PICKUP"},
		{"PICKUP_COMPLETED", "LOADED"},
		{"DEPARTED_PICKUP", "IN_TRANSIT"},
		{"ARRIVED_AT_DELIVERY", "ARRIVED_AT_CONSIGNEE"},
		{"UNLOADING_STARTED", "UNLOADING"},
		{"DELIVERY_COMPLETED", "DELIVERED"},
	}
	for _, step := range steps {
		occurred := time.Now().UTC().Format(time.RFC3339)
		status, payload = env.post(t, env.shipment, "/v1/driver/me/shipments/"+shipmentID+"/events", map[string]string{
			"type": step.event, "occurredAt": occurred, "idempotencyKey": "e2e-" + step.event,
		}, driver)
		if status != http.StatusOK {
			t.Fatalf("%s status=%d body=%s\n%s", step.event, status, payload, env.logs())
		}
		var recorded struct {
			ShipmentID     string `json:"shipmentId"`
			ShipmentStatus string `json:"shipmentStatus"`
			Replayed       bool   `json:"replayed"`
		}
		if err := json.Unmarshal(payload, &recorded); err != nil {
			t.Fatalf("%s json: %v body=%s", step.event, err, payload)
		}
		if recorded.ShipmentID != shipmentID || recorded.ShipmentStatus != step.status || recorded.Replayed {
			t.Fatalf("%s result: %+v", step.event, recorded)
		}
	}

	deliveredHistory := countRows(t, env.pool, `SELECT COUNT(*) FROM transport.shipment_status_history WHERE shipment_id=$1 AND to_status='DELIVERED'`, shipmentID)
	if deliveredHistory != 1 {
		t.Fatalf("delivered history rows=%d", deliveredHistory)
	}
	status, payload = env.post(t, env.shipment, "/v1/driver/me/shipments/"+shipmentID+"/events", map[string]string{
		"type": "DELIVERY_COMPLETED", "occurredAt": time.Now().UTC().Format(time.RFC3339), "idempotencyKey": "e2e-DELIVERY_COMPLETED",
	}, driver)
	if status != http.StatusOK {
		t.Fatalf("replay delivery status=%d body=%s\n%s", status, payload, env.logs())
	}
	var replayed struct {
		ShipmentID     string `json:"shipmentId"`
		ShipmentStatus string `json:"shipmentStatus"`
		Replayed       bool   `json:"replayed"`
	}
	if err := json.Unmarshal(payload, &replayed); err != nil {
		t.Fatalf("replay json: %v body=%s", err, payload)
	}
	if !replayed.Replayed || replayed.ShipmentID != shipmentID || replayed.ShipmentStatus != "DELIVERED" {
		t.Fatalf("replay result: %+v", replayed)
	}
	if countRows(t, env.pool, `SELECT COUNT(*) FROM transport.shipment_status_history WHERE shipment_id=$1 AND to_status='DELIVERED'`, shipmentID) != 1 {
		t.Fatal("replayed delivery wrote another history row")
	}

	waitPublished(t, env, shipmentID)
	assertBrokerDelivered(t, env.brokers, shipmentID)
	waitControlTowerDelivered(t, env, fix.TenantID.String(), shipmentID)
	if countRows(t, env.pool, `SELECT COUNT(*) FROM control_tower.shipment_status_projection WHERE tenant_id=$1 AND shipment_id=$2`, fix.TenantID, shipmentID) != 1 {
		t.Fatal("delivery projection is not one row for this tenant and shipment")
	}
	if countRows(t, env.pool, `SELECT COUNT(*) FROM control_tower.shipment_status_projection WHERE tenant_id=$1 AND shipment_id=$2`, fix.ForeignTenant, shipmentID) != 0 {
		t.Fatal("foreign tenant received a delivery projection")
	}
	status, payload = env.get(t, env.tower, "/internal/v1/control-tower/shipments/"+shipmentID+"/status", map[string]string{"X-Tenant-ID": fix.ForeignTenant.String()})
	deny("foreign tenant control tower", status, payload, http.StatusNotFound)
	if countRows(t, env.pool, `SELECT COUNT(*) FROM transport.shipment_event_outbox WHERE aggregate_id=$1 AND status='PUBLISHED' AND payload->'data'->>'toStatus'='DELIVERED'`, shipmentID) != 1 {
		t.Fatal("replayed delivery published another DELIVERED event")
	}

	status, payload = env.get(t, env.shipment, "/v1/shipments/"+shipmentID, carrier)
	if status != http.StatusOK {
		t.Fatalf("delivered shipment status=%d body=%s\n%s", status, payload, env.logs())
	}
	delivered := decodeShipment(t, payload)
	if delivered.ID != shipmentID || delivered.TenantID != fix.TenantID.String() || delivered.Status != "DELIVERED" || deref(delivered.DriverID) != fix.DriverID.String() || deref(delivered.CarrierCompanyID) != fix.CarrierID.String() {
		t.Fatalf("delivered shipment: %+v", delivered)
	}

	tenantHeader := map[string]string{"X-Tenant-ID": fix.TenantID.String()}
	foreignHeader := map[string]string{"X-Tenant-ID": fix.ForeignTenant.String()}
	status, payload = env.post(t, env.documents, "/internal/v1/pod-uploads", map[string]string{
		"shipmentId": shipmentID, "driverId": fix.OtherDriver.String(), "ownerCompanyId": fix.CarrierID.String(),
		"mimeType": "image/jpeg", "fileName": "other.jpg", "idempotencyKey": "pod-other-driver",
	}, tenantHeader)
	deny("other driver pod intent", status, payload, http.StatusNotFound)
	status, payload = env.post(t, env.documents, "/internal/v1/pod-uploads", map[string]string{
		"shipmentId": shipmentID, "driverId": fix.ForeignDriver.String(), "ownerCompanyId": fix.ForeignCarrier.String(),
		"mimeType": "image/jpeg", "fileName": "foreign.jpg", "idempotencyKey": "pod-foreign-driver",
	}, foreignHeader)
	deny("foreign tenant pod intent", status, payload, http.StatusNotFound)
	if countRows(t, env.pool, `SELECT COUNT(*) FROM documents.documents WHERE related_entity_id=$1 AND document_type='POD'`, shipmentID) != 0 {
		t.Fatal("denied pod intent inserted a document")
	}
	if decodeShipment(t, mustGetShipment(t, env, shipmentID, carrier)).Status != "DELIVERED" {
		t.Fatal("denied pod intent changed the shipment status")
	}

	status, payload = env.post(t, env.documents, "/internal/v1/pod-uploads", map[string]string{
		"shipmentId": shipmentID, "driverId": fix.DriverID.String(), "ownerCompanyId": fix.CarrierID.String(),
		"mimeType": "image/jpeg", "fileName": "pod.jpg", "idempotencyKey": "pod-" + shipmentID,
	}, tenantHeader)
	if status != http.StatusCreated {
		t.Fatalf("pod intent status=%d body=%s\n%s", status, payload, env.logs())
	}
	var intent struct {
		UploadID    string `json:"uploadId"`
		DocumentID  string `json:"documentId"`
		UploadToken string `json:"uploadToken"`
	}
	if err := json.Unmarshal(payload, &intent); err != nil || intent.UploadID == "" || intent.DocumentID == "" || intent.UploadToken == "" {
		t.Fatalf("pod intent json: %v body=%s", err, payload)
	}
	body := []byte("bintrans-e2e-pod")
	sum := sha256.Sum256(body)
	status, payload = env.putRaw(t, env.documents, "/internal/v1/pod-uploads/"+intent.UploadID+"/content", body, map[string]string{
		"X-Tenant-ID": fix.TenantID.String(), "X-Upload-Token": intent.UploadToken,
	})
	if status != http.StatusOK {
		t.Fatalf("pod upload status=%d body=%s\n%s", status, payload, env.logs())
	}
	status, payload = env.post(t, env.documents, "/internal/v1/pod-uploads/"+intent.UploadID+"/complete", map[string]string{
		"driverId": fix.DriverID.String(), "checksumSha256": hex.EncodeToString(sum[:]),
	}, tenantHeader)
	if status != http.StatusOK {
		t.Fatalf("pod complete status=%d body=%s\n%s", status, payload, env.logs())
	}
	var completed struct {
		DocumentID string `json:"documentId"`
		UploadID   string `json:"uploadId"`
		Status     string `json:"status"`
	}
	if err := json.Unmarshal(payload, &completed); err != nil {
		t.Fatalf("pod complete json: %v body=%s", err, payload)
	}
	if completed.DocumentID != intent.DocumentID || completed.UploadID != intent.UploadID || completed.Status != "completed" {
		t.Fatalf("pod complete: %+v", completed)
	}
	status, payload = env.post(t, env.documents, "/internal/v1/pod-uploads/"+intent.UploadID+"/complete", map[string]string{
		"driverId": fix.DriverID.String(), "checksumSha256": hex.EncodeToString(sum[:]),
	}, tenantHeader)
	if status != http.StatusOK {
		t.Fatalf("pod replay status=%d body=%s\n%s", status, payload, env.logs())
	}
	var completedAgain struct {
		DocumentID string `json:"documentId"`
		Status     string `json:"status"`
	}
	if err := json.Unmarshal(payload, &completedAgain); err != nil || completedAgain.DocumentID != intent.DocumentID || completedAgain.Status != "completed" {
		t.Fatalf("pod replay: %v %+v body=%s", err, completedAgain, payload)
	}
	if countRows(t, env.pool, `SELECT COUNT(*) FROM documents.documents WHERE tenant_id=$1 AND document_type='POD' AND related_entity_type='SHIPMENT' AND related_entity_id=$2`, fix.TenantID, shipmentID) != 1 {
		t.Fatal("replayed pod complete created another document")
	}
	if countRows(t, env.pool, `SELECT COUNT(*) FROM documents.document_files WHERE document_id=$1`, intent.DocumentID) != 1 {
		t.Fatal("replayed pod complete created another file")
	}

	status, payload = env.post(t, env.documents, "/internal/v1/pod-uploads/"+intent.UploadID+"/complete", map[string]string{
		"driverId": fix.OtherDriver.String(),
	}, tenantHeader)
	deny("other driver pod complete", status, payload, http.StatusBadRequest)
	status, payload = env.get(t, env.documents, "/v1/documents/"+intent.DocumentID, foreignHeader)
	deny("foreign tenant document", status, payload, http.StatusNotFound)
	if countRows(t, env.pool, `SELECT COUNT(*) FROM documents.documents WHERE related_entity_id=$1 AND document_type='POD'`, shipmentID) != 1 {
		t.Fatal("denied pod access created or removed a document")
	}

	status, payload = env.get(t, env.documents, "/v1/documents/"+intent.DocumentID, tenantHeader)
	if status != http.StatusOK {
		t.Fatalf("document lookup status=%d body=%s\n%s", status, payload, env.logs())
	}
	var document struct {
		TenantID          string  `json:"tenant_id"`
		DocumentType      string  `json:"document_type"`
		RelatedEntityType *string `json:"related_entity_type"`
		RelatedEntityID   *string `json:"related_entity_id"`
	}
	if err := json.Unmarshal(payload, &document); err != nil {
		t.Fatalf("document json: %v body=%s", err, payload)
	}
	if document.TenantID != fix.TenantID.String() || document.DocumentType != "POD" || document.RelatedEntityType == nil || *document.RelatedEntityType != "SHIPMENT" || document.RelatedEntityID == nil || *document.RelatedEntityID != shipmentID {
		t.Fatalf("document link: %+v", document)
	}
	finalShipment := decodeShipment(t, mustGetShipment(t, env, shipmentID, carrier))
	if finalShipment.ID != shipmentID || finalShipment.Status != "DELIVERED" {
		t.Fatalf("pod left shipment %+v", finalShipment)
	}
	if countRows(t, env.pool, `SELECT COUNT(*) FROM transport.shipments`) != 1 {
		t.Fatal("delivery or pod created another shipment")
	}
}

func waitPublished(t *testing.T, env *flowEnv, shipmentID string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		published := countRows(t, env.pool, `SELECT COUNT(*) FROM transport.shipment_event_outbox WHERE aggregate_id=$1 AND status='PUBLISHED' AND payload->'data'->>'toStatus'='DELIVERED'`, shipmentID)
		pending := countRows(t, env.pool, `SELECT COUNT(*) FROM transport.shipment_event_outbox WHERE aggregate_id=$1 AND status<>'PUBLISHED'`, shipmentID)
		if published == 1 && pending == 0 {
			return
		}
		time.Sleep(300 * time.Millisecond)
	}
	var rows string
	_ = env.pool.QueryRow(context.Background(), `
		SELECT COALESCE(string_agg(status || ':' || COALESCE(payload->'data'->>'toStatus', '?') || ':' || COALESCE(last_error_code, ''), ','), 'none')
		FROM transport.shipment_event_outbox WHERE aggregate_id=$1`, shipmentID).Scan(&rows)
	t.Fatalf("outbox did not publish the delivery event rows=%s\n%s", rows, env.logs())
}

func waitControlTowerDelivered(t *testing.T, env *flowEnv, tenantID, shipmentID string) {
	t.Helper()
	deadline := time.Now().Add(45 * time.Second)
	var status int
	var payload []byte
	for time.Now().Before(deadline) {
		status, payload = env.get(t, env.tower, "/internal/v1/control-tower/shipments/"+shipmentID+"/status", map[string]string{"X-Tenant-ID": tenantID})
		if status == http.StatusOK {
			var body struct {
				ShipmentID    string `json:"shipmentId"`
				CurrentStatus string `json:"currentStatus"`
			}
			if err := json.Unmarshal(payload, &body); err == nil && body.ShipmentID == shipmentID && body.CurrentStatus == "DELIVERED" {
				return
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatalf("control tower status=%d body=%s\n%s", status, payload, env.logs())
}

func assertBrokerDelivered(t *testing.T, brokers, shipmentID string) {
	t.Helper()
	client, err := kgo.NewClient(
		kgo.SeedBrokers(strings.Split(brokers, ",")...),
		kgo.ConsumeTopics("shipment.status.v1"),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		t.Fatalf("broker reader: %v", err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	seen := 0
	sample := ""
	found := false
	for {
		fetches := client.PollFetches(ctx)
		fetches.EachRecord(func(record *kgo.Record) {
			seen++
			value := string(record.Value)
			if sample == "" {
				sample = value
			}
			if strings.Contains(value, shipmentID) && strings.Contains(value, "toStatus") && strings.Contains(value, "DELIVERED") {
				found = true
			}
		})
		if found {
			return
		}
		if ctx.Err() != nil {
			t.Fatalf("broker records=%d sample=%s errors=%v", seen, sample, fetches.Errors())
		}
	}
}

func mustGetShipment(t *testing.T, env *flowEnv, shipmentID string, headers map[string]string) []byte {
	t.Helper()
	status, payload := env.get(t, env.shipment, "/v1/shipments/"+shipmentID, headers)
	if status != http.StatusOK {
		t.Fatalf("shipment lookup status=%d body=%s\n%s", status, payload, env.logs())
	}
	return payload
}

func decodeShipment(t *testing.T, payload []byte) shipmentView {
	t.Helper()
	var body shipmentView
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("shipment json: %v body=%s", err, payload)
	}
	return body
}
