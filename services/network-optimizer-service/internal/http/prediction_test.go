package http_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	httpserver "github.com/freight-platform/network-optimizer-service/internal/http"
	"github.com/freight-platform/network-optimizer-service/internal/predict"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/service"
	"github.com/freight-platform/network-optimizer-service/internal/sourceverify"
)

type fakeSources struct {
	shipment predict.ShipmentFact
	vehicle  predict.VehicleFact
	eta      predict.ETAFact
}

func TestBNOPredictionGates(t *testing.T) {
	tenant := uuid.New()
	user := uuid.New()
	other := uuid.New()
	otherUser := uuid.New()
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	weight := 20000.0
	volume := 82.0
	lat, lon := 55.75, 37.62
	body := "TENT"
	mode := "ACTIVE"
	minC, maxC := -25.0, 20.0
	vehicleID := uuid.New()
	planned := time.Date(2026, 9, 24, 16, 0, 0, 0, time.UTC)
	fake := &fakeSources{
		shipment: predict.ShipmentFact{
			ID: uuid.New(), TenantID: tenant, Status: "IN_TRANSIT", Version: 4,
			VehicleID: &vehicleID, DestinationLocationID: uuid.New(),
			DestinationLatitude: &lat, DestinationLongitude: &lon, PlannedDeliveryAt: &planned,
		},
		vehicle: predict.VehicleFact{
			ID: vehicleID, Version: 2, BodyType: &body, LegacyEquipmentType: strPtr("curtain sider"),
			LoadingAccess: []string{"SIDE"}, UnloadingAccess: []string{"REAR"},
			CapacityWeightKg: &weight, CapacityVolumeM3: &volume,
			TemperatureControlMode: &mode, TemperatureCapabilityMinC: &minC, TemperatureCapabilityMaxC: &maxC,
			CombinationType: strPtr("TRUCK"),
		},
		eta: predict.ETAFact{Present: true, Arrival: time.Date(2026, 9, 24, 15, 20, 0, 0, time.UTC), ObservedAt: now.Add(-10 * time.Minute)},
	}
	store := repository.NewMemory()
	policy := predict.Policy{Unload: 35 * time.Minute, Uncertainty: 20 * time.Minute, MaxETAAge: 30 * time.Minute, ConfidenceFloor: 0.5}
	svc := service.New(store, sourceverify.MapVerifier{})
	svc.ConfigurePrediction(sources{fake}, policy)
	svc.SetClock(func() time.Time { return now })
	srv := httptest.NewServer(httpserver.NewRouter(slog.New(slog.DiscardHandler), svc, nil))
	t.Cleanup(srv.Close)
	api := &client{base: srv.URL, t: t}

	created := api.send(http.MethodPost, "/v1/network/shipments/"+fake.shipment.ID.String()+"/predicted-capacity", tenant, user, "", "", "")
	if created.status != http.StatusCreated {
		t.Fatalf("BNO28 status=%d body=%s", created.status, created.raw)
	}
	prediction := object(t, created.doc, "prediction")
	capacity := object(t, created.doc, "capacity")
	if prediction["prediction_method"] != "RULE_BASED" || prediction["is_current"] != true || capacity["status"] != "PREDICTED" || capacity["visibility_scope"] != "PRIVATE" || capacity["source"] != "CURRENT_SHIPMENT_PREDICTION" {
		t.Fatalf("BNO28/BNO41 %+v %+v", prediction, capacity)
	}
	if prediction["capacity_semantics"] != "NEXT_LOAD_FUTURE_CAPACITY" || prediction["capacity_weight_kg"] != 20000.0 || strings.Contains(created.raw, "8000") || strings.Contains(created.raw, "12000") {
		t.Fatalf("BNO78/BNO79 body=%s", created.raw)
	}
	if prediction["capacity_volume_m3"] != 82.0 || prediction["body_type"] != "TENT" {
		t.Fatalf("BNO33/BNO59 %+v", prediction)
	}
	if prediction["destination_location_id"] != fake.shipment.DestinationLocationID.String() || prediction["destination_latitude"] != lat {
		t.Fatalf("BNO35 %+v", prediction)
	}
	if prediction["predicted_available_at"] != "2026-09-24T16:35:00Z" {
		t.Fatalf("BNO36/BNO37 available=%v", prediction["predicted_available_at"])
	}
	if prediction["availability_window_start"] != "2026-09-24T16:15:00Z" || prediction["availability_window_end"] != "2026-09-24T16:55:00Z" {
		t.Fatalf("BNO38 %+v %+v", prediction["availability_window_start"], prediction["availability_window_end"])
	}
	confidence, _ := prediction["confidence"].(float64)
	if confidence <= 0 || confidence > 1 {
		t.Fatalf("BNO40 %v", confidence)
	}
	again := api.send(http.MethodPost, "/v1/network/shipments/"+fake.shipment.ID.String()+"/predicted-capacity", tenant, user, "", "", "")
	if again.status != http.StatusOK || object(t, again.doc, "prediction")["predicted_capacity_id"] != prediction["predicted_capacity_id"] || object(t, again.doc, "prediction")["confidence"] != confidence {
		t.Fatalf("BNO39/BNO46 status=%d body=%s", again.status, again.raw)
	}
	events, err := store.ListOutbox(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	predictedEvents := 0
	for _, event := range events {
		if event.EventName == domain.EventCapacityPredicted {
			predictedEvents++
			assertSafeEvent(t, event)
			if strings.Contains(string(event.Payload), "latitude") || strings.Contains(string(event.Payload), "vehicle_id") || strings.Contains(string(event.Payload), fake.vehicle.ID.String()) {
				t.Fatalf("BNO54 %s", event.Payload)
			}
		}
		if strings.Contains(event.EventName, "match") || strings.Contains(event.EventName, "chain") || strings.Contains(event.EventName, "route") || strings.Contains(event.EventName, "offer") {
			t.Fatalf("BNO57 %s", event.EventName)
		}
	}
	if predictedEvents != 1 {
		t.Fatalf("BNO53 count=%d", predictedEvents)
	}
	market := api.send(http.MethodGet, "/v1/network/marketplace/capacities/"+capacity["id"].(string), other, otherUser, "", "", "")
	if market.status != http.StatusNotFound {
		t.Fatalf("BNO42 status=%d", market.status)
	}
	foreign := api.send(http.MethodGet, "/v1/network/predicted-capacities/"+prediction["predicted_capacity_id"].(string), other, otherUser, "", "", "")
	if foreign.status != http.StatusNotFound {
		t.Fatalf("BNO43 status=%d", foreign.status)
	}
	denied := api.send(http.MethodPost, "/v1/network/shipments/"+uuid.NewString()+"/predicted-capacity", other, otherUser, "", "", "")
	if denied.status != http.StatusNotFound {
		t.Fatalf("BNO29 status=%d body=%s", denied.status, denied.raw)
	}

	svc.ConfigurePrediction(sources{fake}, predict.Policy{Unload: policy.Unload, Uncertainty: policy.Uncertainty, MaxETAAge: policy.MaxETAAge, ConfidenceFloor: 0.99})
	low := api.send(http.MethodPost, "/v1/network/predicted-capacities/"+prediction["predicted_capacity_id"].(string)+"/activate", tenant, user, "", `{"version":1}`, "")
	if low.status != http.StatusUnprocessableEntity || !strings.Contains(low.raw, "CONFIDENCE_BELOW_FLOOR") {
		t.Fatalf("BNO44 status=%d body=%s", low.status, low.raw)
	}
	svc.ConfigurePrediction(sources{fake}, policy)

	freshID := uuid.New()
	fake.shipment.ID = freshID
	missingETA := fake.eta
	fake.eta.Present = false
	noETA := api.send(http.MethodPost, "/v1/network/shipments/"+freshID.String()+"/predicted-capacity", tenant, user, "", "", "")
	if noETA.status != http.StatusUnprocessableEntity || !strings.Contains(noETA.raw, "ETA_UNAVAILABLE") {
		t.Fatalf("BNO30 status=%d body=%s", noETA.status, noETA.raw)
	}
	fake.eta = missingETA
	fake.eta.ObservedAt = now.Add(-2 * time.Hour)
	staleID := uuid.New()
	fake.shipment.ID = staleID
	stale := api.send(http.MethodPost, "/v1/network/shipments/"+staleID.String()+"/predicted-capacity", tenant, user, "", "", "")
	if stale.status != http.StatusUnprocessableEntity || !strings.Contains(stale.raw, "ETA_STALE") {
		t.Fatalf("BNO31 status=%d body=%s", stale.status, stale.raw)
	}
	fake.eta.ObservedAt = now.Add(-10 * time.Minute)
	unassignedID := uuid.New()
	fake.shipment.ID = unassignedID
	savedVehicle := fake.shipment.VehicleID
	fake.shipment.VehicleID = nil
	unassigned := api.send(http.MethodPost, "/v1/network/shipments/"+unassignedID.String()+"/predicted-capacity", tenant, user, "", "", "")
	if unassigned.status != http.StatusUnprocessableEntity || !strings.Contains(unassigned.raw, "VEHICLE_UNASSIGNED") {
		t.Fatalf("BNO32 status=%d body=%s", unassigned.status, unassigned.raw)
	}
	fake.shipment.VehicleID = savedVehicle
	ambiguousID := uuid.New()
	fake.shipment.ID = ambiguousID
	fake.shipment.OtherActiveAssignments = 1
	ambiguous := api.send(http.MethodPost, "/v1/network/shipments/"+ambiguousID.String()+"/predicted-capacity", tenant, user, "", "", "")
	if ambiguous.status != http.StatusUnprocessableEntity || !strings.Contains(ambiguous.raw, "VEHICLE_FUTURE_AVAILABILITY_AMBIGUOUS") || strings.Contains(ambiguous.raw, "20000") {
		t.Fatalf("BNO80 status=%d body=%s", ambiguous.status, ambiguous.raw)
	}
	fake.shipment.OtherActiveAssignments = 0

	unknownID := uuid.New()
	fake.shipment.ID = unknownID
	fake.vehicle.CapacityWeightKg = nil
	fake.vehicle.BodyType = nil
	fake.vehicle.LegacyEquipmentType = strPtr("curtain sider")
	fake.vehicle.LoadingAccess = nil
	unknown := api.send(http.MethodPost, "/v1/network/shipments/"+unknownID.String()+"/predicted-capacity", tenant, user, "", "", "")
	if unknown.status != http.StatusCreated {
		t.Fatalf("BNO34 status=%d body=%s", unknown.status, unknown.raw)
	}
	unknownPrediction := object(t, unknown.doc, "prediction")
	if unknownPrediction["capacity_weight_kg"] != nil || unknownPrediction["body_type"] != nil || unknownPrediction["loading_access"] != nil || strings.Contains(unknown.raw, `"capacity_weight_kg":0`) {
		t.Fatalf("BNO34/BNO67/BNO68/BNO69 %+v", unknownPrediction)
	}
	fake.vehicle.CapacityWeightKg = &weight
	fake.vehicle.BodyType = &body
	fake.vehicle.LoadingAccess = []string{"REAR", "SIDE", "TOP"}
	fake.vehicle.UnloadingAccess = []string{"REAR"}
	for _, token := range []string{"TENT", "CONTAINER", "ISOTHERMAL", "REFRIGERATOR"} {
		id := uuid.New()
		fake.shipment.ID = id
		value := token
		fake.vehicle.BodyType = &value
		if token == "REFRIGERATOR" {
			fake.vehicle.TemperatureCapabilityMinC = &minC
			fake.vehicle.TemperatureCapabilityMaxC = &maxC
		}
		res := api.send(http.MethodPost, "/v1/network/shipments/"+id.String()+"/predicted-capacity", tenant, user, "", "", "")
		got := object(t, res.doc, "prediction")
		if res.status != http.StatusCreated || got["body_type"] != token {
			t.Fatalf("body %s status=%d body=%s", token, res.status, res.raw)
		}
		if token == "REFRIGERATOR" && (got["temperature_capability_min_c"] != -25.0 || got["temperature_capability_max_c"] != 20.0) {
			t.Fatalf("BNO70 %+v", got)
		}
	}
	accessPrediction := object(t, api.send(http.MethodPost, "/v1/network/shipments/"+fake.shipment.ID.String()+"/predicted-capacity", tenant, user, "", "", "").doc, "prediction")
	loading, _ := accessPrediction["loading_access"].([]any)
	unloading, _ := accessPrediction["unloading_access"].([]any)
	if len(loading) != 3 || len(unloading) != 1 {
		t.Fatalf("BNO63-66 %+v %+v", loading, unloading)
	}
	emptyID := uuid.New()
	fake.shipment.ID = emptyID
	fake.vehicle.LoadingAccess = []string{}
	emptyAccess := api.send(http.MethodPost, "/v1/network/shipments/"+emptyID.String()+"/predicted-capacity", tenant, user, "", "", "")
	if !strings.Contains(emptyAccess.raw, `"loading_access":[]`) {
		t.Fatalf("known empty access body=%s", emptyAccess.raw)
	}

	fake.vehicle.LoadingAccess = []string{"SIDE"}
	fake.vehicle.BodyType = &body
	fake.shipment.ID = uuid.New()
	fake.eta.Arrival = fake.eta.Arrival.Add(time.Hour)
	refreshed := api.send(http.MethodPost, "/v1/network/shipments/"+fake.shipment.ID.String()+"/predicted-capacity", tenant, user, "", "", "")
	if refreshed.status != http.StatusCreated {
		t.Fatalf("refresh base status=%d body=%s", refreshed.status, refreshed.raw)
	}
	firstID := object(t, refreshed.doc, "prediction")["predicted_capacity_id"].(string)
	fake.eta.Arrival = fake.eta.Arrival.Add(time.Hour)
	next := api.send(http.MethodPost, "/v1/network/predicted-capacities/"+firstID+"/refresh", tenant, user, "", `{"version":1}`, "")
	if next.status != http.StatusCreated {
		t.Fatalf("BNO47 status=%d body=%s", next.status, next.raw)
	}
	second := object(t, next.doc, "prediction")
	if second["predicted_capacity_id"] == firstID || second["supersedes_prediction_id"] != firstID || second["is_current"] != true {
		t.Fatalf("BNO47 %+v", second)
	}
	oldActivate := api.send(http.MethodPost, "/v1/network/predicted-capacities/"+firstID+"/activate", tenant, user, "", `{"version":1}`, "")
	if oldActivate.status != http.StatusConflict || !strings.Contains(oldActivate.raw, "PREDICTION_SUPERSEDED") {
		t.Fatalf("BNO48 status=%d body=%s", oldActivate.status, oldActivate.raw)
	}
	activated := api.send(http.MethodPost, "/v1/network/predicted-capacities/"+second["predicted_capacity_id"].(string)+"/activate", tenant, user, "", `{"version":1}`, "")
	if activated.status != http.StatusOK || object(t, activated.doc, "capacity")["status"] != "AVAILABLE" || object(t, activated.doc, "capacity")["visibility_scope"] != "PRIVATE" {
		t.Fatalf("BNO45 status=%d body=%s", activated.status, activated.raw)
	}
	stillHidden := api.send(http.MethodGet, "/v1/network/marketplace/capacities/"+object(t, activated.doc, "capacity")["id"].(string), other, otherUser, "", "", "")
	if stillHidden.status != http.StatusNotFound {
		t.Fatalf("activated prediction remains marketplace-hidden status=%d", stillHidden.status)
	}
	fake.shipment.Status = "CANCELLED"
	cancelled := api.send(http.MethodPost, "/v1/network/shipments/"+fake.shipment.ID.String()+"/predicted-capacity", tenant, user, "", "", "")
	if cancelled.status != http.StatusUnprocessableEntity || !strings.Contains(cancelled.raw, "SHIPMENT_CANCELLED") {
		t.Fatalf("BNO49 status=%d body=%s", cancelled.status, cancelled.raw)
	}

	from := now.Format(time.RFC3339)
	until := now.Add(time.Hour).Format(time.RFC3339)
	manual := api.send(http.MethodPost, "/v1/network/capacities", tenant, user, "", fmt.Sprintf(`{"location_label":"Yard","available_from":"%s","available_until":"%s","visibility_scope":"PRIVATE"}`, from, until), "")
	if manual.status != http.StatusCreated || manual.doc["source"] != "MANUAL" {
		t.Fatalf("BNO55 status=%d body=%s", manual.status, manual.raw)
	}
	forged := api.send(http.MethodPost, "/v1/network/capacities", tenant, user, "", fmt.Sprintf(`{"location_label":"Yard","available_from":"%s","available_until":"%s","visibility_scope":"PRIVATE","source":"CURRENT_SHIPMENT_PREDICTION"}`, from, until), "")
	if forged.status != http.StatusUnprocessableEntity {
		t.Fatalf("BNO55 forged source status=%d", forged.status)
	}
	for _, path := range []string{"/v1/network/matches", "/v1/network/next-load", "/v1/network/recommendations", "/v1/network/top-n"} {
		if res := api.send(http.MethodGet, path, tenant, user, "", "", ""); res.status != http.StatusNotFound {
			t.Fatalf("BNO58 %s status=%d", path, res.status)
		}
	}
}

func TestBNO81AndBNO82CapacityEventAggregate(t *testing.T) {
	tenant := uuid.New()
	user := uuid.New()
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	weight := 20000.0
	volume := 82.0
	lat, lon := 55.75, 37.62
	body := "TENT"
	vehicleID := uuid.New()
	planned := time.Date(2026, 9, 24, 16, 0, 0, 0, time.UTC)
	fake := &fakeSources{
		shipment: predict.ShipmentFact{
			ID: uuid.New(), TenantID: tenant, Status: "IN_TRANSIT", Version: 4,
			VehicleID: &vehicleID, DestinationLocationID: uuid.New(),
			DestinationLatitude: &lat, DestinationLongitude: &lon, PlannedDeliveryAt: &planned,
		},
		vehicle: predict.VehicleFact{
			ID: vehicleID, Version: 2, BodyType: &body,
			LoadingAccess: []string{"SIDE"}, UnloadingAccess: []string{"REAR"},
			CapacityWeightKg: &weight, CapacityVolumeM3: &volume,
		},
		eta: predict.ETAFact{Present: true, Arrival: time.Date(2026, 9, 24, 15, 20, 0, 0, time.UTC), ObservedAt: now.Add(-10 * time.Minute)},
	}
	store := repository.NewMemory()
	policy := predict.Policy{Unload: 35 * time.Minute, Uncertainty: 20 * time.Minute, MaxETAAge: 30 * time.Minute, ConfidenceFloor: 0.5}
	svc := service.New(store, sourceverify.MapVerifier{})
	svc.ConfigurePrediction(sources{fake}, policy)
	svc.SetClock(func() time.Time { return now })
	srv := httptest.NewServer(httpserver.NewRouter(slog.New(slog.DiscardHandler), svc, nil))
	t.Cleanup(srv.Close)
	api := &client{base: srv.URL, t: t}

	created := api.send(http.MethodPost, "/v1/network/shipments/"+fake.shipment.ID.String()+"/predicted-capacity", tenant, user, "", "", "")
	if created.status != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.status, created.raw)
	}
	predictionID := object(t, created.doc, "prediction")["predicted_capacity_id"].(string)
	capacityID := object(t, created.doc, "capacity")["id"].(string)
	capacityVersion, _ := object(t, created.doc, "capacity")["version"].(float64)
	if predictionID == capacityID || capacityVersion != 1 {
		t.Fatalf("BNO81 fixture prediction=%s capacity=%s version=%v", predictionID, capacityID, capacityVersion)
	}
	replay := api.send(http.MethodPost, "/v1/network/shipments/"+fake.shipment.ID.String()+"/predicted-capacity", tenant, user, "", "", "")
	if replay.status != http.StatusOK || object(t, replay.doc, "prediction")["predicted_capacity_id"] != predictionID {
		t.Fatalf("idempotent replay status=%d body=%s", replay.status, replay.raw)
	}
	predicted := requireCapacityEvent(t, store, domain.EventCapacityPredicted, capacityID)
	if predicted.AggregateID.String() == predictionID || predicted.AggregateVersion != 1 {
		t.Fatalf("BNO81 aggregate=%s version=%d prediction=%s", predicted.AggregateID, predicted.AggregateVersion, predictionID)
	}
	payload := eventPayload(t, predicted)
	if payload["aggregateId"] != capacityID || payload["capacityId"] != capacityID || payload["predictionId"] != predictionID {
		t.Fatalf("BNO81 payload=%v", payload)
	}
	if payload["aggregateVersion"] != capacityVersion {
		t.Fatalf("BNO81 payload version=%v capacity version=%v", payload["aggregateVersion"], capacityVersion)
	}
	assertSafeEvent(t, predicted)
	if predictedCount(t, store, domain.EventCapacityPredicted, capacityID) != 1 {
		t.Fatal("replay emitted a second predicted event")
	}

	activated := api.send(http.MethodPost, "/v1/network/predicted-capacities/"+predictionID+"/activate", tenant, user, "", `{"version":1}`, "")
	if activated.status != http.StatusOK {
		t.Fatalf("activate status=%d body=%s", activated.status, activated.raw)
	}
	activatedVersion, _ := object(t, activated.doc, "capacity")["version"].(float64)
	updated := requireCapacityEvent(t, store, domain.EventCapacityUpdated, capacityID)
	if updated.AggregateID != predicted.AggregateID || updated.AggregateVersion != int(activatedVersion) || updated.AggregateVersion <= predicted.AggregateVersion {
		t.Fatalf("BNO82 predicted=%s/%d updated=%s/%d", predicted.AggregateID, predicted.AggregateVersion, updated.AggregateID, updated.AggregateVersion)
	}
	if object(t, activated.doc, "capacity")["visibility_scope"] != "PRIVATE" {
		t.Fatal("activation changed visibility")
	}

	fake.shipment.Status = "CANCELLED"
	cancelled := api.send(http.MethodPost, "/v1/network/shipments/"+fake.shipment.ID.String()+"/predicted-capacity", tenant, user, "", "", "")
	if cancelled.status != http.StatusUnprocessableEntity {
		t.Fatalf("cancel status=%d body=%s", cancelled.status, cancelled.raw)
	}
	withdrawn := requireCapacityEvent(t, store, domain.EventCapacityWithdrawn, capacityID)
	if withdrawn.AggregateID != predicted.AggregateID || withdrawn.AggregateVersion <= updated.AggregateVersion {
		t.Fatalf("BNO82 withdrawn=%s/%d updated=%d", withdrawn.AggregateID, withdrawn.AggregateVersion, updated.AggregateVersion)
	}
}

func requireCapacityEvent(t *testing.T, store *repository.Memory, name, capacityID string) repository.OutboxEvent {
	t.Helper()
	events, err := store.ListOutbox(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.EventName == name && event.AggregateID.String() == capacityID {
			return event
		}
	}
	t.Fatalf("missing %s for capacity %s", name, capacityID)
	return repository.OutboxEvent{}
}

func predictedCount(t *testing.T, store *repository.Memory, name, capacityID string) int {
	t.Helper()
	events, err := store.ListOutbox(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, event := range events {
		if event.EventName == name && event.AggregateID.String() == capacityID {
			count++
		}
	}
	return count
}

func eventPayload(t *testing.T, event repository.OutboxEvent) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

type sources struct{ *fakeSources }

func (s sources) Shipment(_ context.Context, tenant, id uuid.UUID) (predict.ShipmentFact, error) {
	if s.shipment.TenantID != tenant || s.shipment.ID != id {
		return predict.ShipmentFact{}, predict.ErrNotFound
	}
	return s.shipment, nil
}

func (s sources) Vehicle(_ context.Context, tenant, id uuid.UUID) (predict.VehicleFact, error) {
	if s.vehicle.ID != id {
		return predict.VehicleFact{}, predict.ErrNotFound
	}
	return s.vehicle, nil
}

func (s sources) ETA(_ context.Context, _, _ uuid.UUID) (predict.ETAFact, error) {
	return s.eta, nil
}

func object(t *testing.T, doc map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := doc[key].(map[string]any)
	if !ok {
		t.Fatalf("missing %s in %v", key, doc)
	}
	return value
}

func strPtr(value string) *string { return &value }
