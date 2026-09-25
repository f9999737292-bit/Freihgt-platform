package service

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	apperrors "github.com/freight-platform/network-optimizer-service/internal/platform/errors"
	"github.com/freight-platform/network-optimizer-service/internal/reference"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
)

func TestBNO241OpenAPISingleRequestBody(t *testing.T) {
	for _, path := range []string{
		"../../../../packages/openapi/network-optimizer-service.yaml",
		"../../../../packages/openapi/openapi.yaml",
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		block := operationBlock(string(raw), "/api/v1/network/next-load/search")
		if strings.Count(block, "\n      requestBody:") != 1 {
			t.Fatalf("%s requestBody count %d", path, strings.Count(block, "\n      requestBody:"))
		}
		head := block
		if idx := strings.Index(block, "\n      responses:"); idx >= 0 {
			head = block[:idx]
		}
		if strings.Contains(head, "additionalProperties: true") || !strings.Contains(block, "#/components/schemas/NextLoadSearchRequest") {
			t.Fatalf("%s request schema %s", path, head)
		}
	}
}

func TestBNO242To254CompatibilityAndLimit(t *testing.T) {
	t.Run("BNO242_TEMPERATURE_REQUIREMENT_REJECT", func(t *testing.T) {
		w, cap := manualSearch(t)
		load := w.load(domain.VisMarketplace, place(0, kmDeg(2), "Cold"), place(0, 1, "D"), nil)
		load.Cargo.TemperatureRequired = boolPtr(true)
		load.Cargo.TemperatureMinC = f64(2)
		load.Cargo.TemperatureMaxC = f64(8)
		updateLoad(t, w, load)
		doc := w.search(w.actor(), cap.ID, radiusPolicy(100, 0))
		if doc.EligibleCandidateCount != 0 || doc.RejectionCountsByReason[domain.ReasonCargoIncompatible] != 1 {
			t.Fatalf("%+v", doc.RejectionCountsByReason)
		}
	})

	t.Run("BNO243_LOADING_ACCESS_REJECT", func(t *testing.T) {
		w, cap := predictedSearch(t, []string{"SIDE"}, []string{"REAR"}, "TENT", false)
		load := w.load(domain.VisMarketplace, place(0, kmDeg(2), "Side"), place(0, 1, "D"), feasibleWindow(w))
		load.Cargo.RequiredLoadingAccess = []string{"REAR"}
		updateLoad(t, w, load)
		doc := w.search(w.actor(), cap.ID, radiusPolicy(100, 0))
		if doc.EligibleCandidateCount != 0 || doc.RejectionCountsByReason[domain.ReasonCargoIncompatible] != 1 {
			t.Fatalf("%+v", doc.RejectionCountsByReason)
		}
	})

	t.Run("BNO244_UNLOADING_ACCESS_REJECT", func(t *testing.T) {
		w, cap := predictedSearch(t, []string{"REAR"}, []string{"SIDE"}, "TENT", false)
		load := w.load(domain.VisMarketplace, place(0, kmDeg(2), "Rear"), place(0, 1, "D"), feasibleWindow(w))
		load.Cargo.RequiredLoadingAccess = []string{"REAR"}
		load.Cargo.RequiredUnloadingAccess = []string{"REAR"}
		updateLoad(t, w, load)
		doc := w.search(w.actor(), cap.ID, radiusPolicy(100, 0))
		if doc.EligibleCandidateCount != 0 || doc.RejectionCountsByReason[domain.ReasonCargoIncompatible] != 1 {
			t.Fatalf("%+v", doc.RejectionCountsByReason)
		}
	})

	t.Run("BNO245_ADR_UNKNOWN_FAILS_CLOSED", func(t *testing.T) {
		w, cap := manualSearch(t)
		load := w.load(domain.VisMarketplace, place(0, kmDeg(2), "DG"), place(0, 1, "D"), nil)
		load.Cargo.Dangerous = boolPtr(true)
		load.Cargo.HazardClasses = []string{"3"}
		updateLoad(t, w, load)
		assertIndeterminate(t, w, cap.ID)
	})

	t.Run("BNO246_FOOD_GRADE_UNKNOWN_FAILS_CLOSED", func(t *testing.T) {
		w, cap := manualSearch(t)
		load := w.load(domain.VisMarketplace, place(0, kmDeg(2), "Food"), place(0, 1, "D"), nil)
		load.Cargo.FoodGradeRequired = boolPtr(true)
		updateLoad(t, w, load)
		assertIndeterminate(t, w, cap.ID)
	})

	t.Run("BNO247_PALLET_CAPACITY_INDETERMINATE", func(t *testing.T) {
		w, cap := manualSearch(t)
		load := w.load(domain.VisMarketplace, place(0, kmDeg(2), "Pallets"), place(0, 1, "D"), nil)
		load.Cargo.PalletCount = intPtr(4)
		load.Cargo.PalletTypeCode = strPtr("EUR")
		updateLoad(t, w, load)
		assertIndeterminate(t, w, cap.ID)
	})

	t.Run("BNO248_LINEAR_METERS_INDETERMINATE", func(t *testing.T) {
		w, cap := manualSearch(t)
		load := w.load(domain.VisMarketplace, place(0, kmDeg(2), "Linear"), place(0, 1, "D"), nil)
		load.Cargo.LinearMeters = f64(6.5)
		updateLoad(t, w, load)
		assertIndeterminate(t, w, cap.ID)
	})

	t.Run("BNO249_HEIGHT_INDETERMINATE", func(t *testing.T) {
		w, cap := manualSearch(t)
		load := w.load(domain.VisMarketplace, place(0, kmDeg(2), "Tall"), place(0, 1, "D"), nil)
		load.Cargo.MaxLoadedHeightMM = intPtr(2400)
		updateLoad(t, w, load)
		assertIndeterminate(t, w, cap.ID)
	})

	t.Run("BNO250_PREDICTED_CAPABILITY_FACTS_USED", func(t *testing.T) {
		w, cap := predictedSearch(t, []string{"REAR"}, []string{"SIDE"}, "TENT", true)
		load := w.load(domain.VisMarketplace, place(0, kmDeg(2), "Fit"), place(0, 1, "D"), feasibleWindow(w))
		load.WeightKg = f64(1000)
		load.VolumeM3 = f64(4)
		load.Cargo.RequiredBodyTypes = []string{"TENT"}
		load.Cargo.RequiredLoadingAccess = []string{"REAR"}
		load.Cargo.RequiredUnloadingAccess = []string{"SIDE"}
		load.Cargo.TemperatureRequired = boolPtr(true)
		load.Cargo.TemperatureMinC = f64(2)
		load.Cargo.TemperatureMaxC = f64(6)
		updateLoad(t, w, load)
		doc := w.search(w.actor(), cap.ID, radiusPolicy(100, 0))
		if doc.EligibleCandidateCount != 1 || len(doc.Candidates) != 1 || doc.Candidates[0].Compatibility != "COMPATIBLE" {
			t.Fatalf("predicted facts %+v %+v", doc.Candidates, doc.RejectionCountsByReason)
		}
	})

	t.Run("BNO250_CATALOG_RESOLVED_BODY", func(t *testing.T) {
		w, cap := manualSearch(t)
		cap.Equipment = []string{"SEMITRAILER_REEFER"}
		cap.Version = 2
		if err := w.store.Within(context.Background(), func(tx repository.Tx) error {
			return tx.UpdateCapacity(context.Background(), cap, 1)
		}); err != nil {
			t.Fatal(err)
		}
		load := w.load(domain.VisMarketplace, place(0, kmDeg(2), "Reefer"), place(0, 1, "D"), nil)
		load.Cargo.RequiredBodyTypes = []string{"REFRIGERATOR"}
		updateLoad(t, w, load)
		without := w.search(w.actor(), cap.ID, radiusPolicy(100, 0))
		if without.RejectionCountsByReason[domain.ReasonCargoIndeterminate] != 1 {
			t.Fatalf("without catalog %+v", without.RejectionCountsByReason)
		}
		w.svc.UseCatalog(reference.NewMemoryCatalog())
		with := w.search(w.actor(), cap.ID, radiusPolicy(100, 0))
		if with.EligibleCandidateCount != 1 || with.Candidates[0].Compatibility != "COMPATIBLE" {
			t.Fatalf("catalog %+v %+v", with.Candidates, with.RejectionCountsByReason)
		}
	})

	t.Run("BNO254_NEGATIVE_CANDIDATE_LIMIT_REJECTED", func(t *testing.T) {
		w, cap := manualSearch(t)
		limit := -1
		_, err := w.svc.SearchNextLoad(context.Background(), w.actor(), SearchCommand{CapacityID: cap.ID, Policy: radiusPolicy(100, 0), CandidateLimit: &limit})
		var app *apperrors.AppError
		if !errors.As(err, &app) || app.Code != apperrors.CodeValidation || app.Details["field"] != "candidate_limit" {
			t.Fatalf("%v", err)
		}
		doc := w.search(w.actor(), cap.ID, radiusPolicy(100, 0))
		run, _, err := w.store.GetSearch(context.Background(), w.carrier, doc.SearchID)
		if err != nil || run.RoutingProvider != "UNSPECIFIED" {
			t.Fatalf("provider %q %v", run.RoutingProvider, err)
		}
	})
}

func TestRoutingProviderName(t *testing.T) {
	if routingProviderName(nil) != "UNCONFIGURED" {
		t.Fatal("missing provider")
	}
	if routingProviderName(&scripted{}) != "UNSPECIFIED" {
		t.Fatal("unnamed provider")
	}
	if routingProviderName(namedProvider{name: "2GIS"}) != "2GIS" {
		t.Fatal("named provider")
	}
}

type namedProvider struct{ name string }

func (namedProvider) Route(context.Context, routing.RouteRequest) (routing.RouteResult, error) {
	return routing.RouteResult{}, nil
}
func (namedProvider) Matrix(context.Context, routing.MatrixRequest) (routing.MatrixResult, error) {
	return routing.MatrixResult{}, nil
}
func (n namedProvider) ProviderName() string { return n.name }

func manualSearch(t *testing.T) (*world, domain.Capacity) {
	t.Helper()
	w := newWorld(t)
	w.routes.defaultM = 10000
	w.routes.defaultSec = 600
	return w, w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
}

func predictedSearch(t *testing.T, loading, unloading []string, body string, withTemperature bool) (*world, domain.Capacity) {
	t.Helper()
	w := newWorld(t)
	w.routes.defaultM = 10000
	w.routes.defaultSec = 600
	cap := w.capacity(domain.CapacityAvailable, domain.SourceCurrentShipmentPrediction, 0, 0)
	mode := "NONE"
	prediction := domain.PredictedCapacity{
		ID: uuid.New(), CapacityID: cap.ID, OwnerTenantID: w.carrier, IsCurrent: true,
		PredictedAvailableAt: w.at, AvailabilityWindowStart: w.at, AvailabilityWindowEnd: w.at.Add(12 * time.Hour),
		CombinationType: strPtr("ARTICULATED"), BodyType: strPtr(body),
		LoadingAccess: append([]string(nil), loading...), UnloadingAccess: append([]string(nil), unloading...),
		LegacyEquipmentType: strPtr("CURTAIN"), ContainerSize: strPtr("40FT"),
		TemperatureControlMode: &mode,
	}
	if withTemperature {
		active := "ACTIVE"
		prediction.TemperatureControlMode = &active
		prediction.TemperatureCapabilityMinC = f64(-20)
		prediction.TemperatureCapabilityMaxC = f64(8)
		prediction.TemperatureZoneCount = intPtr(1)
		prediction.IndependentTemperatureControl = boolPtr(false)
		prediction.CapacityWeightKg = f64(20000)
		prediction.CapacityVolumeM3 = f64(80)
	}
	if err := w.store.Within(context.Background(), func(tx repository.Tx) error {
		return tx.InsertPrediction(context.Background(), prediction)
	}); err != nil {
		t.Fatal(err)
	}
	return w, cap
}

func feasibleWindow(w *world) *domain.TimeWindow {
	return window(w.at.Add(-time.Hour), w.at.Add(4*time.Hour))
}

func updateLoad(t *testing.T, w *world, load domain.LoadOpportunity) {
	t.Helper()
	load.Version = 2
	if err := w.store.Within(context.Background(), func(tx repository.Tx) error {
		return tx.UpdateLoad(context.Background(), load, 1)
	}); err != nil {
		t.Fatal(err)
	}
}

func assertIndeterminate(t *testing.T, w *world, capacityID uuid.UUID) {
	t.Helper()
	doc := w.search(w.actor(), capacityID, radiusPolicy(100, 0))
	if doc.EligibleCandidateCount != 0 || doc.RejectionCountsByReason[domain.ReasonCargoIndeterminate] != 1 {
		t.Fatalf("%+v", doc.RejectionCountsByReason)
	}
}

func operationBlock(text, path string) string {
	marker := "  " + path + ":"
	start := strings.Index(text, marker)
	if start < 0 {
		return ""
	}
	rest := text[start+len(marker):]
	if next := strings.Index(rest, "\n  /"); next >= 0 {
		rest = rest[:next]
	}
	return rest
}

func boolPtr(v bool) *bool    { return &v }
func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }
