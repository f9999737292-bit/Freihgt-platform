package http_test

import (
	"context"
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
	"github.com/freight-platform/network-optimizer-service/internal/locationclient"
	"github.com/freight-platform/network-optimizer-service/internal/predict"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/service"
	"github.com/freight-platform/network-optimizer-service/internal/sourceverify"
)

type mapDirectory struct {
	snaps   map[string]domain.LocationSnapshot
	sources map[string][2]uuid.UUID
}

func (m mapDirectory) Projection(_ context.Context, tenantID, locationID uuid.UUID) (domain.LocationSnapshot, error) {
	snap, ok := m.snaps[tenantID.String()+"|"+locationID.String()]
	if !ok {
		return domain.LocationSnapshot{}, locationclient.ErrNotFound
	}
	return snap, nil
}

func (m mapDirectory) SourceEndpoints(_ context.Context, tenantID uuid.UUID, sourceType string, sourceID uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	pair, ok := m.sources[fmt.Sprintf("%s|%s|%s", tenantID, sourceType, sourceID)]
	if !ok {
		return uuid.Nil, uuid.Nil, locationclient.ErrNotFound
	}
	return pair[0], pair[1], nil
}

func TestBNO170To178Geography(t *testing.T) {
	tenant := uuid.New()
	user := uuid.New()
	other := uuid.New()
	otherUser := uuid.New()
	locationID := uuid.New()
	foreign := uuid.New()
	origin := uuid.New()
	destination := uuid.New()
	lat, lon := 56.838, 60.597
	cityLat, cityLon := 55.7558, 37.6173
	directory := mapDirectory{
		sources: map[string][2]uuid.UUID{},
		snaps: map[string]domain.LocationSnapshot{
			tenant.String() + "|" + locationID.String():  {ID: locationID, CountryCode: "RU", Region: "Sverdlovsk", City: "Ekaterinburg", Latitude: &lat, Longitude: &lon, Status: "ACTIVE"},
			tenant.String() + "|" + origin.String():      {ID: origin, CountryCode: "RU", Region: "Sverdlovsk", City: "Ekaterinburg", Latitude: &lat, Longitude: &lon, Status: "ACTIVE"},
			tenant.String() + "|" + destination.String(): {ID: destination, CountryCode: "RU", Region: "Moscow", City: "Moscow", Latitude: &cityLat, Longitude: &cityLon, Status: "ACTIVE"},
		},
	}
	store := repository.NewMemory()
	verifier := sourceverify.MapVerifier{Owned: map[string]bool{}}
	svc := service.New(store, verifier)
	svc.UseDirectory(directory)
	srv := httptest.NewServer(httpserver.NewRouter(slog.New(slog.DiscardHandler), svc, nil))
	t.Cleanup(srv.Close)
	api := &client{base: srv.URL, t: t}
	from := time.Now().UTC().Format(time.RFC3339)
	until := time.Now().UTC().Add(4 * time.Hour).Format(time.RFC3339)

	created := api.send(http.MethodPost, "/v1/network/capacities", tenant, user, "", fmt.Sprintf(`{"location_id":"%s","location_label":"caller yard","latitude":1,"longitude":2,"available_from":"%s","available_until":"%s","visibility_scope":"PRIVATE"}`, locationID, from, until), "")
	if created.status != http.StatusCreated || created.doc["location_id"] != locationID.String() || created.doc["city"] != "Ekaterinburg" || created.doc["latitude"] != lat {
		t.Fatalf("BNO170 %d %s", created.status, created.raw)
	}
	if _, ok := created.doc["address_line"]; ok || strings.Contains(created.raw, "postal_code") {
		t.Fatalf("BNO170 copied facility metadata %s", created.raw)
	}

	missing := api.send(http.MethodPost, "/v1/network/capacities", tenant, user, "", fmt.Sprintf(`{"location_id":"%s","available_from":"%s","available_until":"%s","visibility_scope":"PRIVATE"}`, foreign, from, until), "")
	if missing.status != http.StatusNotFound {
		t.Fatalf("BNO172 %d %s", missing.status, missing.raw)
	}

	source := uuid.New()
	verifier.Owned[fmt.Sprintf("%s|TRANSPORT_ORDER|%s", tenant, source)] = true
	directory.sources[fmt.Sprintf("%s|TRANSPORT_ORDER|%s", tenant, source)] = [2]uuid.UUID{origin, destination}
	load := api.send(http.MethodPost, "/v1/network/load-opportunities", tenant, user, "", fmt.Sprintf(`{"source_type":"TRANSPORT_ORDER","source_id":"%s","pickup":{"label":"Dock","latitude":1,"longitude":2},"delivery":{"label":"Door","latitude":3,"longitude":4},"visibility_scope":"ANONYMIZED_MARKETPLACE","publish":true}`, source), "")
	if load.status != http.StatusCreated {
		t.Fatalf("load %d %s", load.status, load.raw)
	}
	pickup := load.doc["pickup"].(map[string]any)
	delivery := load.doc["delivery"].(map[string]any)
	if pickup["location_id"] != origin.String() || pickup["latitude"] != lat || pickup["city"] != "Ekaterinburg" {
		t.Fatalf("BNO173 %+v", pickup)
	}
	if delivery["location_id"] != destination.String() || delivery["city"] != "Moscow" || delivery["latitude"] != cityLat {
		t.Fatalf("BNO174 %+v", delivery)
	}
	view := api.send(http.MethodGet, "/v1/network/marketplace/load-opportunities/"+load.doc["id"].(string), other, otherUser, "", "", "")
	if view.status != http.StatusOK {
		t.Fatalf("marketplace %d %s", view.status, view.raw)
	}
	shown := view.doc["pickup"].(map[string]any)
	if _, ok := shown["location_id"]; ok || shown["latitude"] != nil || shown["label"] != nil || strings.Contains(view.raw, "60.597") || strings.Contains(view.raw, "Dock") {
		t.Fatalf("BNO175 %s", view.raw)
	}
	if shown["city"] != "Ekaterinburg" || shown["country_code"] != "RU" {
		t.Fatalf("BNO176 %+v", shown)
	}
	if view.doc["delivery"].(map[string]any)["city"] != "Moscow" {
		t.Fatalf("BNO178 delivery display %+v", view.doc["delivery"])
	}
	owner := api.send(http.MethodGet, "/v1/network/load-opportunities/"+load.doc["id"].(string), tenant, user, "", "", "")
	ownerPickup := owner.doc["pickup"].(map[string]any)
	if ownerPickup["latitude"] != lat || ownerPickup["location_id"] != origin.String() {
		t.Fatalf("BNO178 search geo lost %+v", ownerPickup)
	}

	regionOnly := uuid.New()
	regionSource := uuid.New()
	verifier.Owned[fmt.Sprintf("%s|SHIPMENT|%s", tenant, regionSource)] = true
	directory.snaps[tenant.String()+"|"+regionOnly.String()] = domain.LocationSnapshot{ID: regionOnly, CountryCode: "RU", Region: "Sverdlovsk", Status: "ACTIVE"}
	directory.sources[fmt.Sprintf("%s|SHIPMENT|%s", tenant, regionSource)] = [2]uuid.UUID{regionOnly, destination}
	regionLoad := api.send(http.MethodPost, "/v1/network/load-opportunities", tenant, user, "", fmt.Sprintf(`{"source_type":"SHIPMENT","source_id":"%s","pickup":{"label":"Hidden"},"delivery":{"label":"Door"},"visibility_scope":"ANONYMIZED_MARKETPLACE","publish":true}`, regionSource), "")
	if regionLoad.status != http.StatusCreated {
		t.Fatalf("region load %d %s", regionLoad.status, regionLoad.raw)
	}
	regionView := api.send(http.MethodGet, "/v1/network/marketplace/load-opportunities/"+regionLoad.doc["id"].(string), other, otherUser, "", "", "")
	regionPickup := regionView.doc["pickup"].(map[string]any)
	if _, ok := regionPickup["city"]; ok || regionPickup["region"] != "Sverdlovsk" || regionPickup["country_code"] != "RU" {
		t.Fatalf("BNO177 %+v", regionPickup)
	}
}

func TestBNO171PredictionLocationPropagates(t *testing.T) {
	tenant := uuid.New()
	user := uuid.New()
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	weight, volume := 20000.0, 82.0
	lat, lon := 56.83, 60.60
	body := "TENT"
	vehicleID := uuid.New()
	destination := uuid.New()
	planned := now.Add(4 * time.Hour)
	fake := &fakeSources{
		shipment: predict.ShipmentFact{
			ID: uuid.New(), TenantID: tenant, Status: "IN_TRANSIT", Version: 1,
			VehicleID: &vehicleID, DestinationLocationID: destination,
			DestinationLatitude: &lat, DestinationLongitude: &lon, PlannedDeliveryAt: &planned,
		},
		vehicle: predict.VehicleFact{ID: vehicleID, Version: 1, BodyType: &body, CapacityWeightKg: &weight, CapacityVolumeM3: &volume},
		eta:     predict.ETAFact{Present: true, Arrival: now.Add(3 * time.Hour), ObservedAt: now.Add(-5 * time.Minute)},
	}
	store := repository.NewMemory()
	svc := service.New(store, sourceverify.MapVerifier{})
	svc.ConfigurePrediction(sources{fake}, predict.Policy{Unload: 30 * time.Minute, Uncertainty: 15 * time.Minute, MaxETAAge: time.Hour, ConfidenceFloor: 0.5})
	svc.SetClock(func() time.Time { return now })
	srv := httptest.NewServer(httpserver.NewRouter(slog.New(slog.DiscardHandler), svc, nil))
	t.Cleanup(srv.Close)
	api := &client{base: srv.URL, t: t}
	created := api.send(http.MethodPost, "/v1/network/shipments/"+fake.shipment.ID.String()+"/predicted-capacity", tenant, user, "", "", "")
	if created.status != http.StatusCreated {
		t.Fatalf("BNO171 %d %s", created.status, created.raw)
	}
	capacity := object(t, created.doc, "capacity")
	if capacity["location_id"] != destination.String() || capacity["latitude"] != lat {
		t.Fatalf("BNO171 capacity %+v", capacity)
	}
}
