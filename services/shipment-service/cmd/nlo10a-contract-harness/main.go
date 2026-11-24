// Test-only loopback process for NLO-1.0A.
// It mounts the production shipment handlers and internal-auth middleware.
// The repository below those handlers is an in-memory fixture. It does not
// open a database, read an environment file, or change product behavior.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/shared-go/internalauth"
	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/http/handlers"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

const serviceToken = "nlo-1.0a-harness-token"

type fixture struct {
	Token         string     `json:"token"`
	TenantID      uuid.UUID  `json:"tenant_id"`
	ForeignTenant uuid.UUID  `json:"foreign_tenant_id"`
	Vehicles      []vehicle  `json:"vehicles"`
	Shipments     []shipment `json:"shipments"`
}

type vehicle struct {
	Scenario                  string    `json:"scenario"`
	ID                        uuid.UUID `json:"id"`
	TenantID                  uuid.UUID `json:"tenant_id"`
	Version                   int       `json:"version"`
	ShipmentID                uuid.UUID `json:"shipment_id"`
	CapacityWeight            *float64  `json:"capacity_weight"`
	CapacityVolume            *float64  `json:"capacity_volume"`
	PalletPositions           *int      `json:"pallet_positions"`
	InternalLengthMM          *int      `json:"internal_length_mm"`
	InternalWidthMM           *int      `json:"internal_width_mm"`
	InternalHeightMM          *int      `json:"internal_height_mm"`
	TemperatureControlMode    *string   `json:"temperature_control_mode"`
	TemperatureCapabilityMinC *float64  `json:"temperature_capability_min_c"`
	TemperatureCapabilityMaxC *float64  `json:"temperature_capability_max_c"`
	FoodGradeCapability       *bool     `json:"food_grade_capability"`
	ADRCapability             *bool     `json:"adr_capability"`
}

type shipment struct {
	ID        uuid.UUID `json:"id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	VehicleID uuid.UUID `json:"vehicle_id"`
	Version   int       `json:"version"`
}

func main() {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	tenant := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa1")
	foreign := uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbb1")
	rows, ships := scenarios(tenant, foreign)
	store := newStore(rows, ships)
	router := productionRoutes(store, serviceToken)
	go func() { _ = http.Serve(ln, router) }()

	doc := fixture{Token: serviceToken, TenantID: tenant, ForeignTenant: foreign, Vehicles: publicVehicles(rows, ships), Shipments: publicShipments(ships)}
	raw, err := json.Marshal(doc)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("NLO10A_URL=http://%s\n", ln.Addr().String())
	fmt.Printf("NLO10A_FIXTURE=%s\n", raw)
	_, _ = io.Copy(io.Discard, os.Stdin)
	_ = ln.Close()
}

func productionRoutes(store *memoryStore, token string) http.Handler {
	auth := internalauth.Config{Token: token}
	evidence := handlers.NewExecutionEvidenceHandler(store)
	capability := handlers.NewVehicleCapabilityHandler(store)
	r := chi.NewRouter()
	r.Route("/internal/v1/shipments", func(r chi.Router) {
		r.With(auth.Middleware).Get("/{shipmentId}/execution-context", evidence.GetExecutionContext)
		r.With(auth.Middleware).Get("/{shipmentId}/onboard-cargo", evidence.GetOnboardCargo)
	})
	r.Route("/internal/v1/vehicles", func(r chi.Router) {
		r.With(auth.Middleware).Get("/{id}/capability", capability.Get)
	})
	return r
}

type storedVehicle struct {
	scenario string
	row      domain.Vehicle
}

type storedShipment struct {
	row domain.Shipment
}

type memoryStore struct {
	vehicles  map[uuid.UUID]map[uuid.UUID]storedVehicle
	shipments map[uuid.UUID]map[uuid.UUID]storedShipment
}

func newStore(rows []storedVehicle, ships []storedShipment) *memoryStore {
	out := &memoryStore{
		vehicles:  map[uuid.UUID]map[uuid.UUID]storedVehicle{},
		shipments: map[uuid.UUID]map[uuid.UUID]storedShipment{},
	}
	for _, row := range rows {
		bucket := out.vehicles[row.row.TenantID]
		if bucket == nil {
			bucket = map[uuid.UUID]storedVehicle{}
			out.vehicles[row.row.TenantID] = bucket
		}
		bucket[row.row.ID] = row
	}
	for _, row := range ships {
		bucket := out.shipments[row.row.TenantID]
		if bucket == nil {
			bucket = map[uuid.UUID]storedShipment{}
			out.shipments[row.row.TenantID] = bucket
		}
		bucket[row.row.ID] = row
	}
	return out
}

func (s *memoryStore) GetByIDAndTenant(_ context.Context, tenantID, id uuid.UUID) (*domain.Vehicle, error) {
	row, ok := s.vehicles[tenantID][id]
	if !ok {
		return nil, apperrors.NotFound("vehicle not found")
	}
	copy := row.row
	return &copy, nil
}

func (s *memoryStore) ExecutionContext(_ context.Context, tenantID, shipmentID uuid.UUID) (domain.ShipmentExecutionContext, error) {
	row, ok := s.shipments[tenantID][shipmentID]
	if !ok {
		return domain.ShipmentExecutionContext{}, apperrors.NotFound("shipment not found")
	}
	return domain.ExecutionContextFromShipment(row.row), nil
}

func (s *memoryStore) OnboardCargo(_ context.Context, tenantID, shipmentID uuid.UUID) (domain.OnboardCargoView, error) {
	row, ok := s.shipments[tenantID][shipmentID]
	if !ok {
		return domain.OnboardCargoView{}, apperrors.NotFound("shipment not found")
	}
	return domain.ResolveOnboardCargo(row.row, nil), nil
}

func scenarios(tenant, foreign uuid.UUID) ([]storedVehicle, []storedShipment) {
	origin := uuid.MustParse("cccccccc-cccc-4ccc-8ccc-ccccccccccc1")
	dest := uuid.MustParse("dddddddd-dddd-4ddd-8ddd-ddddddddddd1")
	specs := []struct {
		name string
		id   string
		own  uuid.UUID
		fill func(*domain.Vehicle)
	}{
		{name: "compatible", id: "10000000-0000-4000-8000-000000000001", own: tenant, fill: compatibleFill},
		{name: "short", id: "10000000-0000-4000-8000-000000000002", own: tenant, fill: func(v *domain.Vehicle) { compatibleFill(v); v.InternalHeightMM = i(2100) }},
		{name: "light", id: "10000000-0000-4000-8000-000000000003", own: tenant, fill: func(v *domain.Vehicle) { compatibleFill(v); v.CapacityWeight = f(800) }},
		{name: "small-volume", id: "10000000-0000-4000-8000-000000000004", own: tenant, fill: func(v *domain.Vehicle) { compatibleFill(v); v.CapacityVolume = f(4) }},
		{name: "few-pallets", id: "10000000-0000-4000-8000-000000000005", own: tenant, fill: func(v *domain.Vehicle) { compatibleFill(v); v.PalletPositions = i(2) }},
		{name: "warm", id: "10000000-0000-4000-8000-000000000006", own: tenant, fill: func(v *domain.Vehicle) {
			compatibleFill(v)
			v.TemperatureCapabilityMaxC = f(0)
		}},
		{name: "no-adr", id: "10000000-0000-4000-8000-000000000007", own: tenant, fill: func(v *domain.Vehicle) { compatibleFill(v); v.ADRCapability = b(false) }},
		{name: "adr-unknown", id: "10000000-0000-4000-8000-000000000008", own: tenant, fill: func(v *domain.Vehicle) { compatibleFill(v); v.ADRCapability = nil }},
		{name: "no-food", id: "10000000-0000-4000-8000-000000000009", own: tenant, fill: func(v *domain.Vehicle) { compatibleFill(v); v.FoodGradeCapability = b(false) }},
		{name: "food-unknown", id: "10000000-0000-4000-8000-00000000000a", own: tenant, fill: func(v *domain.Vehicle) { compatibleFill(v); v.FoodGradeCapability = nil }},
		{name: "height-unknown", id: "10000000-0000-4000-8000-00000000000b", own: tenant, fill: func(v *domain.Vehicle) { compatibleFill(v); v.InternalHeightMM = nil }},
		{name: "zero-weight", id: "10000000-0000-4000-8000-00000000000c", own: tenant, fill: func(v *domain.Vehicle) { compatibleFill(v); v.CapacityWeight = f(0) }},
		{name: "foreign", id: "10000000-0000-4000-8000-00000000000d", own: foreign, fill: func(v *domain.Vehicle) { compatibleFill(v); v.InternalHeightMM = i(4242); v.Version = 13 }},
	}
	rows := make([]storedVehicle, 0, len(specs))
	ships := make([]storedShipment, 0, len(specs))
	for n, spec := range specs {
		id := uuid.MustParse(spec.id)
		vehicleID := id
		shipmentID := uuid.MustParse(fmt.Sprintf("20000000-0000-4000-8000-%012d", n+1))
		row := domain.Vehicle{
			ID: vehicleID, TenantID: spec.own, CarrierCompanyID: uuid.MustParse("eeeeeeee-eeee-4eee-8eee-eeeeeeeeeee1"),
			PlateNumber: spec.name, VehicleType: domain.VehicleTypeTruck, Version: 7,
		}
		spec.fill(&row)
		rows = append(rows, storedVehicle{scenario: spec.name, row: row})
		ships = append(ships, storedShipment{row: domain.Shipment{
			ID: shipmentID, TenantID: spec.own, Version: 4, Status: domain.ShipmentStatusLoaded,
			VehicleID: &vehicleID, OriginLocationID: origin, DestinationLocationID: dest,
		}})
	}
	return rows, ships
}

func compatibleFill(v *domain.Vehicle) {
	mode := "ACTIVE"
	v.CapacityWeight = f(18000)
	v.CapacityVolume = f(82)
	v.PalletPositions = i(33)
	v.InternalLengthMM = i(13600)
	v.InternalWidthMM = i(2450)
	v.InternalHeightMM = i(2700)
	v.TemperatureControlMode = &mode
	v.TemperatureCapabilityMinC = f(-25)
	v.TemperatureCapabilityMaxC = f(20)
	v.FoodGradeCapability = b(true)
	v.ADRCapability = b(true)
	v.Version = 7
}

func publicVehicles(rows []storedVehicle, ships []storedShipment) []vehicle {
	out := make([]vehicle, 0, len(rows))
	for i, row := range rows {
		out = append(out, vehicle{
			Scenario: row.scenario, ID: row.row.ID, TenantID: row.row.TenantID, Version: row.row.Version,
			ShipmentID:     ships[i].row.ID,
			CapacityWeight: row.row.CapacityWeight, CapacityVolume: row.row.CapacityVolume,
			PalletPositions:  row.row.PalletPositions,
			InternalLengthMM: row.row.InternalLengthMM, InternalWidthMM: row.row.InternalWidthMM, InternalHeightMM: row.row.InternalHeightMM,
			TemperatureControlMode:    row.row.TemperatureControlMode,
			TemperatureCapabilityMinC: row.row.TemperatureCapabilityMinC, TemperatureCapabilityMaxC: row.row.TemperatureCapabilityMaxC,
			FoodGradeCapability: row.row.FoodGradeCapability, ADRCapability: row.row.ADRCapability,
		})
	}
	return out
}

func publicShipments(ships []storedShipment) []shipment {
	out := make([]shipment, 0, len(ships))
	for _, row := range ships {
		out = append(out, shipment{ID: row.row.ID, TenantID: row.row.TenantID, VehicleID: *row.row.VehicleID, Version: row.row.Version})
	}
	return out
}

func f(v float64) *float64 { return &v }
func i(v int) *int         { return &v }
func b(v bool) *bool       { return &v }
