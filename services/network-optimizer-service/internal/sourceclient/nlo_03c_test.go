package sourceclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/predict"
)

func TestNLO03C_SourceReads(t *testing.T) {
	tenant := uuid.New()
	vehicleID := uuid.New()
	cargoID := uuid.New()
	shipmentID := uuid.New()
	recorded := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	observed := recorded.Add(time.Minute)
	arrival := recorded.Add(2 * time.Hour)
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/v1/vehicles/"+vehicleID.String()+"/capability", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"id":"` + vehicleID.String() + `","tenant_id":"` + tenant.String() + `","version":4,
			"equipment_type":"CURTAIN","combination_type":"RIGID","body_type":"BOX","equipment_unit_kind":"VEHICLE",
			"loading_access":["REAR"],"unloading_access":["SIDE"],
			"capacity_weight":12000,"capacity_volume":82,
			"pallet_positions":33,"usable_linear_meters":13.6,
			"internal_length_mm":13600,"internal_width_mm":2450,"internal_height_mm":2700,
			"temperature_control_mode":"NONE","temperature_capability_min_c":-20,"temperature_capability_max_c":20,
			"temperature_zone_count":1,"independent_temperature_control":false,
			"food_grade_capability":true,"adr_capability":false,"container_size":null
		}`))
	})
	mux.HandleFunc("/internal/v1/cargoes/"+cargoID.String()+"/planning-profile", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"` + cargoID.String() + `","tenant_id":"` + tenant.String() + `","version":9,"weight_kg":null,"volume_m3":null,"hazard_classes":["6.1","3"]}`))
	})
	mux.HandleFunc("/internal/v1/shipments/"+shipmentID.String()+"/execution-context", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"shipment_id":"` + shipmentID.String() + `","tenant_id":"` + tenant.String() + `","shipment_version":2,"shipment_status":"LOADED","vehicle_id":"` + vehicleID.String() + `","origin_location_id":"` + uuid.NewString() + `","destination_location_id":"` + uuid.NewString() + `","cargo_id":"` + cargoID.String() + `"}`))
	})
	mux.HandleFunc("/internal/v1/tracking/states/lookup", func(w http.ResponseWriter, r *http.Request) {
		status := r.URL.Query().Get("freshness")
		if status == "" {
			status = "FRESH"
		}
		_, _ = w.Write([]byte(`{"items":{"` + shipmentID.String() + `":{"trackingStatus":"ACTIVE","freshness":{"status":"` + status + `","ageSeconds":12},"lastKnownPosition":{"latitude":55.7,"longitude":37.6,"recordedAt":"` + recorded.Format(time.RFC3339) + `"}}}}`))
	})
	mux.HandleFunc("/internal/v1/tracking/eta/lookup", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":{"` + shipmentID.String() + `":{"status":"AVAILABLE","freshnessStatus":"STALE","ageSeconds":90,"estimatedArrivalAt":"` + arrival.Format(time.RFC3339) + `","sourceObservedAt":"` + observed.Format(time.RFC3339) + `","sourceType":"PROVIDER","provider":"tracking-service"}}}`))
	})
	upstream := httptest.NewServer(mux)
	t.Cleanup(upstream.Close)
	client := New(upstream.URL, upstream.URL, "token").WithTransportOrder(upstream.URL)

	t.Run("NLO03C_030_VEHICLE_VERSION_PINNED", func(t *testing.T) {
		got, err := client.VehicleCapability(context.Background(), tenant, vehicleID)
		if err != nil || got.Version != 4 || got.EquipmentUnitKind == nil || *got.EquipmentUnitKind != "VEHICLE" {
			t.Fatalf("%v %+v", err, got)
		}
	})
	t.Run("NLO03C_031_WEIGHT_CAPACITY_PRESERVED", func(t *testing.T) {
		got, _ := client.VehicleCapability(context.Background(), tenant, vehicleID)
		if got.CapacityWeight == nil || *got.CapacityWeight != 12000 {
			t.Fatal(got.CapacityWeight)
		}
	})
	t.Run("NLO03C_032_VOLUME_CAPACITY_PRESERVED", func(t *testing.T) {
		got, _ := client.VehicleCapability(context.Background(), tenant, vehicleID)
		if got.CapacityVolume == nil || *got.CapacityVolume != 82 {
			t.Fatal(got.CapacityVolume)
		}
	})
	t.Run("NLO03C_033_PALLET_POSITIONS_PRESERVED", func(t *testing.T) {
		got, _ := client.VehicleCapability(context.Background(), tenant, vehicleID)
		if got.PalletPositions == nil || *got.PalletPositions != 33 {
			t.Fatal(got.PalletPositions)
		}
	})
	t.Run("NLO03C_034_LINEAR_METRES_PRESERVED", func(t *testing.T) {
		got, _ := client.VehicleCapability(context.Background(), tenant, vehicleID)
		if got.UsableLinearMeters == nil || *got.UsableLinearMeters != 13.6 {
			t.Fatal(got.UsableLinearMeters)
		}
	})
	t.Run("NLO03C_035_HEIGHT_PRESERVED", func(t *testing.T) {
		got, _ := client.VehicleCapability(context.Background(), tenant, vehicleID)
		if got.InternalHeightMM == nil || *got.InternalHeightMM != 2700 {
			t.Fatal(got.InternalHeightMM)
		}
	})
	t.Run("NLO03C_036_TEMPERATURE_CAPABILITY_PRESERVED", func(t *testing.T) {
		got, _ := client.VehicleCapability(context.Background(), tenant, vehicleID)
		if got.TemperatureCapabilityMinC == nil || *got.TemperatureCapabilityMinC != -20 || got.TemperatureCapabilityMaxC == nil || *got.TemperatureCapabilityMaxC != 20 {
			t.Fatalf("%+v", got)
		}
	})
	t.Run("NLO03C_037_ADR_FOOD_CAPABILITY_PRESERVED", func(t *testing.T) {
		got, _ := client.VehicleCapability(context.Background(), tenant, vehicleID)
		if got.FoodGradeCapability == nil || !*got.FoodGradeCapability || got.ADRCapability == nil || *got.ADRCapability {
			t.Fatalf("%+v", got)
		}
	})
	for _, status := range []string{"FRESH", "STALE", "LOST", "UNKNOWN"} {
		name := map[string]string{"FRESH": "NLO03C_038_TRACKING_FRESH_STATUS_CONSUMED", "STALE": "NLO03C_039_TRACKING_STALE_STATUS_CONSUMED", "LOST": "NLO03C_040_TRACKING_LOST_STATUS_CONSUMED", "UNKNOWN": "NLO03C_041_TRACKING_UNKNOWN_STATUS_CONSUMED"}[status]
		t.Run(name, func(t *testing.T) {
			freshnessServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"items":{"` + shipmentID.String() + `":{"trackingStatus":"ACTIVE","freshness":{"status":"` + status + `","ageSeconds":12},"lastKnownPosition":{"latitude":55.7,"longitude":37.6,"recordedAt":"` + recorded.Format(time.RFC3339) + `"}}}}`))
			}))
			defer freshnessServer.Close()
			local := New(freshnessServer.URL, freshnessServer.URL, "token")
			got, err := local.TrackingState(context.Background(), tenant, shipmentID)
			if err != nil || got.Freshness != status || got.RecordedAt == nil {
				t.Fatalf("%v %+v", err, got)
			}
		})
	}
	t.Run("NLO03C_042_NO_BNO_LOCATION_THRESHOLD_DUPLICATION", func(t *testing.T) {
		raw, err := os.ReadFile("current_trip.go")
		if err != nil {
			t.Fatal(err)
		}
		src := string(raw)
		for _, forbidden := range []string{"10 * time.Minute", "30 * time.Minute", "15 * time.Minute", "60 * time.Minute"} {
			if strings.Contains(src, forbidden) {
				t.Fatal(forbidden)
			}
		}
	})
	t.Run("NLO03C_043_ETA_FRESHNESS_STATUS_PRESERVED", func(t *testing.T) {
		got, err := client.TrackingETA(context.Background(), tenant, shipmentID)
		if err != nil || got.FreshnessStatus != "STALE" || got.Status != "AVAILABLE" || got.Provider != "tracking-service" {
			t.Fatalf("%v %+v", err, got)
		}
	})
	t.Run("NLO03C_044_ETA_SOURCE_OBSERVED_AT_PRESERVED", func(t *testing.T) {
		got, err := client.TrackingETA(context.Background(), tenant, shipmentID)
		if err != nil || got.SourceObservedAt == nil || !got.SourceObservedAt.Equal(observed) || got.EstimatedArrivalAt == nil {
			t.Fatalf("%v %+v", err, got)
		}
	})
	t.Run("NLO03C_045_NLO_0_2_ETA_BEHAVIOR_REGRESSION_PASS", func(t *testing.T) {
		fact, err := client.ETA(context.Background(), tenant, shipmentID)
		if err != nil || !fact.Present || !fact.Arrival.Equal(arrival) || !fact.ObservedAt.Equal(observed) {
			t.Fatalf("%v %+v", err, fact)
		}
		if fact == (predict.ETAFact{}) {
			t.Fatal("empty fact")
		}
	})
}
