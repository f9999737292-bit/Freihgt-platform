package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/compat"
	"github.com/freight-platform/network-optimizer-service/internal/domain"
	httpserver "github.com/freight-platform/network-optimizer-service/internal/http"
	"github.com/freight-platform/network-optimizer-service/internal/reference"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
	"github.com/freight-platform/network-optimizer-service/internal/service"
)

func TestBNO255RuntimeCatalogWired(t *testing.T) {
	fx := newRuntimeFixture(t)
	unwired := fx.service()
	_ = httpserver.NewRouter(slog.New(slog.DiscardHandler), unwired, nil)
	doc := fx.search(t, unwired)
	if doc.EligibleCandidateCount != 0 || doc.RejectionCountsByReason[domain.ReasonCargoIndeterminate] != 1 {
		t.Fatalf("unwired %+v", doc.RejectionCountsByReason)
	}

	wired := fx.service()
	_ = httpserver.NewRouterWithCatalog(slog.New(slog.DiscardHandler), wired, nil, fx.catalog)
	doc = fx.search(t, wired)
	if doc.EligibleCandidateCount != 1 || len(doc.Candidates) != 1 || doc.Candidates[0].Compatibility != compat.StatusCompatible {
		t.Fatalf("wired %+v %+v", doc.Candidates, doc.RejectionCountsByReason)
	}

	failed := fx.service()
	_ = httpserver.NewRouterWithCatalog(slog.New(slog.DiscardHandler), failed, nil, failCatalog{fx.catalog})
	doc = fx.search(t, failed)
	if doc.EligibleCandidateCount != 0 || doc.RejectionCountsByReason[domain.ReasonCargoIndeterminate] != 1 {
		t.Fatalf("catalog failure %+v", doc.RejectionCountsByReason)
	}
}

func TestBNO256ActiveCatalogRuleAppliedInSearch(t *testing.T) {
	fx := newRuntimeFixture(t)
	svc := fx.service()
	_ = httpserver.NewRouterWithCatalog(slog.New(slog.DiscardHandler), svc, nil, fx.catalog)
	set, err := fx.catalog.CreateRuleSet(context.Background(), uuid.New(), fx.carrier)
	if err != nil {
		t.Fatal(err)
	}
	if err := fx.catalog.AddRule(context.Background(), uuid.New(), fx.carrier, set.ID, reference.Rule{
		RuleCode: "REEFER_GENERAL_DENY", RuleKind: compat.KindCargoEquipment, Layer: compat.LayerTenant,
		LeftSelectorType: "CARGO_TYPE", LeftSelectorValue: "GENERAL_CARGO",
		RightSelectorType: "EQUIPMENT_TYPE", RightSelectorValue: "SEMITRAILER_REEFER",
		Decision: compat.DecisionDeny, ReasonCode: "TENANT_EQUIPMENT_DENY", Severity: compat.SeverityHard,
	}); err != nil {
		t.Fatal(err)
	}
	if err := fx.catalog.ActivateRuleSet(context.Background(), uuid.New(), fx.carrier, set.ID); err != nil {
		t.Fatal(err)
	}
	doc := fx.search(t, svc)
	if doc.EligibleCandidateCount != 0 || doc.RejectionCountsByReason[domain.ReasonCargoIncompatible] != 1 {
		t.Fatalf("active rule %+v", doc.RejectionCountsByReason)
	}
}

type runtimeFixture struct {
	store    *repository.Memory
	catalog  *reference.MemoryCatalog
	carrier  uuid.UUID
	capacity uuid.UUID
}

func newRuntimeFixture(t *testing.T) runtimeFixture {
	t.Helper()
	store := repository.NewMemory()
	catalog := reference.NewMemoryCatalog()
	carrier := uuid.New()
	shipper := uuid.New()
	now := time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC)
	capacityID := uuid.New()
	lat, lon := 0.0, 0.0
	pickupLon := 0.02
	load := domain.LoadOpportunity{
		ID: uuid.New(), OwnerTenantID: shipper, SourceType: domain.SourceTransportOrder, SourceID: uuid.New(),
		Pickup:          domain.Place{Latitude: &lat, Longitude: &pickupLon, City: "P", CountryCode: "RU"},
		Delivery:        domain.Place{Latitude: &lat, Longitude: &lon, City: "D", CountryCode: "RU"},
		VisibilityScope: domain.VisMarketplace,
		Status:          domain.LoadPublished,
		Version:         1,
		Cargo:           domain.CargoConstraints{CargoTypeCode: strPtr("GENERAL_CARGO"), RequiredBodyTypes: []string{"REFRIGERATOR"}},
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	capacity := domain.Capacity{
		ID: capacityID, OwnerTenantID: carrier, Latitude: &lat, Longitude: &lon,
		AvailableFrom: now, AvailableUntil: now.Add(24 * time.Hour), Source: domain.SourceManual,
		Equipment: []string{"SEMITRAILER_REEFER"}, VisibilityScope: domain.CapVisPrivate,
		Status: domain.CapacityAvailable, Version: 1,
	}
	if err := store.Within(context.Background(), func(tx repository.Tx) error {
		if err := tx.InsertCapacity(context.Background(), capacity); err != nil {
			return err
		}
		return tx.InsertLoad(context.Background(), load)
	}); err != nil {
		t.Fatal(err)
	}
	return runtimeFixture{store: store, catalog: catalog, carrier: carrier, capacity: capacityID}
}

func (fx runtimeFixture) service() *service.Service {
	svc := service.New(fx.store, nil)
	svc.UseRouting(flatRoads{})
	svc.UsePolicies(fx.store)
	return svc
}

func (fx runtimeFixture) search(t *testing.T, svc *service.Service) searchDoc {
	t.Helper()
	srv := httptest.NewServer(httpserver.NewRouter(slog.New(slog.DiscardHandler), svc, nil))
	t.Cleanup(srv.Close)
	body := map[string]any{
		"capacity_id": fx.capacity.String(),
		"policy":      map[string]any{"search_mode": "RADIUS", "radius_km": 100, "objective_profile": "MIN_DEADHEAD"},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/v1/network/next-load/search", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", fx.carrier.String())
	req.Header.Set("X-User-ID", uuid.NewString())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	payload, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d %s", res.StatusCode, payload)
	}
	var doc searchDoc
	if err := json.Unmarshal(payload, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

type searchDoc struct {
	EligibleCandidateCount  int            `json:"eligible_candidate_count"`
	RejectionCountsByReason map[string]int `json:"rejection_counts_by_reason"`
	Candidates              []struct {
		Compatibility string `json:"compatibility"`
	} `json:"candidates"`
}

type flatRoads struct{}

func (flatRoads) Route(context.Context, routing.RouteRequest) (routing.RouteResult, error) {
	return routing.RouteResult{}, nil
}

func (flatRoads) Matrix(_ context.Context, req routing.MatrixRequest) (routing.MatrixResult, error) {
	cells := make([]routing.MatrixCell, 0, len(req.Origins)*len(req.Destinations))
	for i := range req.Origins {
		for j := range req.Destinations {
			cells = append(cells, routing.MatrixCell{OriginIndex: i, DestinationIndex: j, DistanceM: 10000, DurationSeconds: 600})
		}
	}
	return routing.MatrixResult{Cells: cells}, nil
}

type failCatalog struct{ *reference.MemoryCatalog }

func (failCatalog) Evaluation(context.Context, uuid.UUID) (compat.Context, error) {
	return compat.Context{}, errors.New("catalog unavailable")
}
