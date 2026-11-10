package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/compat"
	"github.com/freight-platform/network-optimizer-service/internal/config"
	"github.com/freight-platform/network-optimizer-service/internal/currenttrip"
	"github.com/freight-platform/network-optimizer-service/internal/domain"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
	"github.com/freight-platform/network-optimizer-service/internal/sourceclient"
)

func TestNLO10A_VehicleCapabilityReachesCompatibility(t *testing.T) {
	h := startShipmentHarness(t)
	client := sourceclient.New(h.URL, "", h.Token)
	provider := currenttrip.NewProvider(client, client, client, client, client, client)

	compatible := h.scenario("compatible")
	trip := mustBuild(t, provider, h.TenantID, compatible.ShipmentID)
	if trip.VehicleCapability == nil {
		t.Fatal("vehicle capability missing after real handler read")
	}
	assertCapabilityMatchesFixture(t, *trip.VehicleCapability, compatible)
	equipment := equipmentFromVehicle(trip.VehicleCapability)
	assertEquipmentMatchesFixture(t, equipment, compatible)

	pass := compat.EvaluateCargoEquipment(compatibleCargo(), equipment, compat.AccessNeed{}, compat.Context{})
	if pass.Status != compat.StatusCompatible {
		t.Fatalf("compatible status %s hard %#v indeterminate %#v", pass.Status, pass.HardRejects, pass.IndeterminateReasons)
	}
	height := fillCandidate{}
	addHeight(&height, loadWithHeight(2000), trip)
	if len(height.hard) != 0 || len(height.indeterminate) != 0 {
		t.Fatalf("compatible height hard %#v indeterminate %#v", height.hard, height.indeterminate)
	}

	assertHard(t, provider, h, "short", cargoWithHeight(2500), "HEIGHT_EXCEEDED")
	assertHard(t, provider, h, "light", cargoWithWeight(5000), "PAYLOAD_EXCEEDED")
	assertHard(t, provider, h, "small-volume", cargoWithVolume(12), "VOLUME_EXCEEDED")
	assertHard(t, provider, h, "few-pallets", cargoWithPallets(8, "EUR"), "PALLET_POSITIONS_EXCEEDED")
	assertHard(t, provider, h, "warm", cargoWithTemperature(2, 8), "TEMPERATURE_RANGE_UNSUPPORTED")
	assertHard(t, provider, h, "no-adr", cargoWithADR(), "ADR_INCOMPATIBLE")
	assertHard(t, provider, h, "no-food", cargoWithFood(), "FOOD_GRADE_REQUIRED")
	assertHard(t, provider, h, "zero-weight", cargoWithWeight(100), "PAYLOAD_EXCEEDED")

	unknownHeight := mustTrip(t, provider, h, "height-unknown")
	if unknownHeight.VehicleCapability.InternalHeightMM != nil {
		t.Fatal("absent height was decoded as a value")
	}
	unknownEval := compat.EvaluateCargoEquipment(cargoWithHeight(1800), equipmentFromVehicle(unknownHeight.VehicleCapability), compat.AccessNeed{}, compat.Context{})
	requireIndeterminate(t, unknownEval, "HEIGHT_UNKNOWN")
	if hasHard(unknownEval, "HEIGHT_EXCEEDED") {
		t.Fatal("absent height became a measured exceedance")
	}

	unknownFood := mustTrip(t, provider, h, "food-unknown")
	foodEval := compat.EvaluateCargoEquipment(cargoWithFood(), equipmentFromVehicle(unknownFood.VehicleCapability), compat.AccessNeed{}, compat.Context{})
	requireIndeterminate(t, foodEval, "FOOD_GRADE_CAPABILITY_UNKNOWN")
	if hasHard(foodEval, "FOOD_GRADE_REQUIRED") {
		t.Fatal("unknown food-grade capability was treated as known false")
	}

	unknownADR := mustTrip(t, provider, h, "adr-unknown")
	adrEval := compat.EvaluateCargoEquipment(cargoWithADR(), equipmentFromVehicle(unknownADR.VehicleCapability), compat.AccessNeed{}, compat.Context{})
	requireIndeterminate(t, adrEval, "ADR_CAPABILITY_UNKNOWN")
	if hasHard(adrEval, "ADR_INCOMPATIBLE") {
		t.Fatal("unknown ADR capability was treated as known false")
	}
}

func TestNLO10A_ForeignTenantDoesNotEnterCurrentTrip(t *testing.T) {
	h := startShipmentHarness(t)
	client := sourceclient.New(h.URL, "", h.Token)
	provider := currenttrip.NewProvider(client, client, client, client, client, client)
	foreign := h.scenario("foreign")

	_, err := client.VehicleCapability(context.Background(), h.TenantID, foreign.ID)
	if !errors.Is(err, currenttrip.ErrNotFound) {
		t.Fatalf("caller reading foreign vehicle: %v", err)
	}
	seen, err := client.VehicleCapability(context.Background(), foreign.TenantID, foreign.ID)
	if err != nil {
		t.Fatal(err)
	}
	if seen.InternalHeightMM == nil || *seen.InternalHeightMM != 4242 || seen.TenantID != foreign.TenantID {
		t.Fatalf("foreign owner read %+v", seen)
	}

	_, err = provider.Build(context.Background(), h.TenantID, foreign.ShipmentID)
	if !errors.Is(err, currenttrip.ErrNotFound) {
		t.Fatalf("caller build of foreign shipment: %v", err)
	}
	own := mustBuild(t, provider, h.TenantID, h.scenario("compatible").ShipmentID)
	if own.VehicleCapability != nil && own.VehicleCapability.InternalHeightMM != nil && *own.VehicleCapability.InternalHeightMM == 4242 {
		t.Fatal("foreign vehicle height entered the caller current-trip context")
	}
}

func TestNLO10A_EmptyURLMakesNoRequest(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits++ }))
	t.Cleanup(server.Close)
	client := sourceclient.New("", "", "nlo-1.0a-harness-token")
	_, err := client.VehicleCapability(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, currenttrip.ErrUnavailable) {
		t.Fatalf("empty shipment url: %v", err)
	}
	_, err = client.TrackingState(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, currenttrip.ErrUnavailable) {
		t.Fatalf("empty tracking url: %v", err)
	}
	_, err = client.TrackingETA(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, currenttrip.ErrUnavailable) {
		t.Fatalf("empty eta url: %v", err)
	}
	if hits != 0 {
		t.Fatalf("empty url issued %d requests to %s", hits, server.URL)
	}
}

func TestNLO10A_ProtocolFailuresStayDistinct(t *testing.T) {
	h := startShipmentHarness(t)
	vehicle := h.scenario("compatible")

	wrong := sourceclient.New(h.URL, "", "wrong-token")
	_, err := wrong.VehicleCapability(context.Background(), h.TenantID, vehicle.ID)
	if !errors.Is(err, currenttrip.ErrUnavailable) {
		t.Fatalf("wrong token: %v", err)
	}

	missingAuth, err := http.NewRequest(http.MethodGet, h.URL+"/internal/v1/vehicles/"+vehicle.ID.String()+"/capability", nil)
	if err != nil {
		t.Fatal(err)
	}
	missingAuth.Header.Set("X-Tenant-ID", h.TenantID.String())
	authResp, err := http.DefaultClient.Do(missingAuth)
	if err != nil {
		t.Fatal(err)
	}
	authResp.Body.Close()
	if authResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("missing internal token status %d", authResp.StatusCode)
	}

	missingTenant, err := http.NewRequest(http.MethodGet, h.URL+"/internal/v1/vehicles/"+vehicle.ID.String()+"/capability", nil)
	if err != nil {
		t.Fatal(err)
	}
	missingTenant.Header.Set("X-Internal-Service-Token", h.Token)
	tenantResp, err := http.DefaultClient.Do(missingTenant)
	if err != nil {
		t.Fatal(err)
	}
	tenantResp.Body.Close()
	if tenantResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("missing tenant status %d", tenantResp.StatusCode)
	}

	client := sourceclient.New(h.URL, "", h.Token)
	_, err = client.VehicleCapability(context.Background(), h.TenantID, uuid.New())
	if !errors.Is(err, currenttrip.ErrNotFound) {
		t.Fatalf("unknown vehicle: %v", err)
	}

	mismatch := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"`+uuid.NewString()+`","tenant_id":"`+h.TenantID.String()+`","version":1}`)
	}))
	t.Cleanup(mismatch.Close)
	_, err = sourceclient.New(mismatch.URL, "", h.Token).VehicleCapability(context.Background(), h.TenantID, vehicle.ID)
	if !errors.Is(err, currenttrip.ErrUnavailable) {
		t.Fatalf("mismatched id on 200: %v", err)
	}

	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{`)
	}))
	t.Cleanup(broken.Close)
	_, err = sourceclient.New(broken.URL, "", h.Token).VehicleCapability(context.Background(), h.TenantID, vehicle.ID)
	if !errors.Is(err, currenttrip.ErrUnavailable) {
		t.Fatalf("malformed json: %v", err)
	}

	slow := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	t.Cleanup(slow.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = sourceclient.New(slow.URL, "", h.Token).VehicleCapability(ctx, h.TenantID, vehicle.ID)
	if !errors.Is(err, currenttrip.ErrUnavailable) {
		t.Fatalf("timeout: %v", err)
	}

	absent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"items":{}}`)
	}))
	t.Cleanup(absent.Close)
	position, err := sourceclient.New("", absent.URL, h.Token).TrackingState(context.Background(), h.TenantID, vehicle.ShipmentID)
	if err != nil {
		t.Fatal(err)
	}
	if position.Latitude != nil || position.Longitude != nil || position.Freshness != "UNKNOWN" {
		t.Fatalf("missing tracking item became a position %+v", position)
	}
	eta, err := sourceclient.New("", absent.URL, h.Token).TrackingETA(context.Background(), h.TenantID, vehicle.ShipmentID)
	if err != nil {
		t.Fatal(err)
	}
	if eta.EstimatedArrivalAt != nil || eta.FreshnessStatus != "UNKNOWN" {
		t.Fatalf("missing eta item became a time %+v", eta)
	}
}

func TestNLO10A_RoadUnknownWithout2GIS(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://unused")
	t.Setenv("BNO_ROUTING_PROVIDER", "")
	t.Setenv("BNO_2GIS_ROUTING_BASE_URL", "")
	t.Setenv("BNO_2GIS_API_KEY", "")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RoutingProvider != "" {
		t.Fatalf("routing provider %q", cfg.RoutingProvider)
	}

	w := newWorld(t)
	w.svc.UseRouting(nil)
	capacity := w.capacity(domain.CapacityAvailable, domain.SourceManual, 55.75, 37.61)
	pickup := place(56.50, 38.20, "Noginsk")
	delivery := place(56.80, 38.40, "Elektrostal")
	windowStart := w.at.Add(2 * time.Hour)
	windowEnd := w.at.Add(3 * time.Hour)
	load := w.load(domain.VisMarketplace, pickup, delivery, window(windowStart, windowEnd))
	straight := routing.StraightLineKm(55.75, 37.61, 56.50, 38.20)
	if straight <= 1 {
		t.Fatalf("straight line %v", straight)
	}

	result, err := w.svc.SearchNextLoad(context.Background(), w.actor(), SearchCommand{
		CapacityID: capacity.ID,
		Policy: domain.NextLoadSearchPolicy{
			SearchMode: domain.SearchRadius, RadiusKm: f64(2000), ObjectiveProfile: "MIN_DEADHEAD",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	body := string(result.Body)
	if strings.Contains(body, "road_deadhead_km") || strings.Contains(body, "road_deadhead_minutes") {
		t.Fatalf("road measurement present %s", body)
	}
	var doc SearchResponse
	if err := json.Unmarshal(result.Body, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.RejectionCountsByReason[domain.ReasonRoadDistanceUnknown] < 1 {
		t.Fatalf("rejection counts %#v", doc.RejectionCountsByReason)
	}
	if doc.EligibleCandidateCount != 0 {
		t.Fatalf("unknown road became eligible %d", doc.EligibleCandidateCount)
	}
	for _, candidate := range doc.Candidates {
		if candidate.LoadOpportunityID == load.ID && (candidate.Timing == "FEASIBLE" || candidate.RoadDeadheadKm != nil || candidate.WaitingMinutes != nil) {
			t.Fatalf("window treated as reachable %+v", candidate)
		}
	}
	if strings.Contains(body, "2gis") {
		t.Fatalf("provider name leaked %s", body)
	}
}

func assertHard(t *testing.T, provider *currenttrip.Provider, h harness, scenario string, cargo compat.Cargo, code string) {
	t.Helper()
	trip := mustTrip(t, provider, h, scenario)
	got := compat.EvaluateCargoEquipment(cargo, equipmentFromVehicle(trip.VehicleCapability), compat.AccessNeed{}, compat.Context{})
	if !hasHard(got, code) {
		t.Fatalf("%s: missing %s status %s hard %#v indeterminate %#v", scenario, code, got.Status, got.HardRejects, got.IndeterminateReasons)
	}
}

func mustTrip(t *testing.T, provider *currenttrip.Provider, h harness, scenario string) currenttrip.CurrentTripContext {
	t.Helper()
	row := h.scenario(scenario)
	return mustBuild(t, provider, row.TenantID, row.ShipmentID)
}

func mustBuild(t *testing.T, provider *currenttrip.Provider, tenant, shipment uuid.UUID) currenttrip.CurrentTripContext {
	t.Helper()
	trip, err := provider.Build(context.Background(), tenant, shipment)
	if err != nil {
		t.Fatal(err)
	}
	if trip.VehicleCapability == nil {
		t.Fatal("build returned no vehicle")
	}
	return trip
}

func hasHard(got compat.Result, code string) bool {
	for _, reason := range got.HardRejects {
		if reason.ReasonCode == code {
			return true
		}
	}
	return false
}

func requireIndeterminate(t *testing.T, got compat.Result, code string) {
	t.Helper()
	for _, reason := range got.IndeterminateReasons {
		if reason.ReasonCode == code {
			return
		}
	}
	t.Fatalf("missing indeterminate %s status %s hard %#v indeterminate %#v", code, got.Status, got.HardRejects, got.IndeterminateReasons)
}

func assertCapabilityMatchesFixture(t *testing.T, got currenttrip.VehicleCapability, want vehicleFixture) {
	t.Helper()
	if got.ID != want.ID || got.TenantID != want.TenantID || got.Version != want.Version {
		t.Fatalf("identity got %+v want %+v", got, want)
	}
	sameF(t, "weight", got.CapacityWeight, want.CapacityWeight)
	sameF(t, "volume", got.CapacityVolume, want.CapacityVolume)
	sameI(t, "pallets", got.PalletPositions, want.PalletPositions)
	sameI(t, "length", got.InternalLengthMM, want.InternalLengthMM)
	sameI(t, "width", got.InternalWidthMM, want.InternalWidthMM)
	sameI(t, "height", got.InternalHeightMM, want.InternalHeightMM)
	sameS(t, "temperature mode", got.TemperatureControlMode, want.TemperatureControlMode)
	sameF(t, "temperature min", got.TemperatureCapabilityMinC, want.TemperatureCapabilityMinC)
	sameF(t, "temperature max", got.TemperatureCapabilityMaxC, want.TemperatureCapabilityMaxC)
	sameB(t, "food", got.FoodGradeCapability, want.FoodGradeCapability)
	sameB(t, "adr", got.ADRCapability, want.ADRCapability)
}

func assertEquipmentMatchesFixture(t *testing.T, got compat.Equipment, want vehicleFixture) {
	t.Helper()
	sameF(t, "equipment weight", got.PayloadKg, want.CapacityWeight)
	sameF(t, "equipment volume", got.VolumeM3, want.CapacityVolume)
	sameI(t, "equipment pallets", got.PalletPositions, want.PalletPositions)
	sameI(t, "equipment height", got.InternalHeightMM, want.InternalHeightMM)
	sameF(t, "equipment temp min", got.TemperatureMinC, want.TemperatureCapabilityMinC)
	sameF(t, "equipment temp max", got.TemperatureMaxC, want.TemperatureCapabilityMaxC)
	sameB(t, "equipment food", got.FoodGradeCapability, want.FoodGradeCapability)
	sameB(t, "equipment adr", got.ADRCapability, want.ADRCapability)
}

func sameF(t *testing.T, name string, got, want *float64) {
	t.Helper()
	if (got == nil) != (want == nil) || (got != nil && *got != *want) {
		t.Fatalf("%s got %v want %v", name, got, want)
	}
}

func sameI(t *testing.T, name string, got, want *int) {
	t.Helper()
	if (got == nil) != (want == nil) || (got != nil && *got != *want) {
		t.Fatalf("%s got %v want %v", name, got, want)
	}
}

func sameB(t *testing.T, name string, got, want *bool) {
	t.Helper()
	if (got == nil) != (want == nil) || (got != nil && *got != *want) {
		t.Fatalf("%s got %v want %v", name, got, want)
	}
}

func sameS(t *testing.T, name string, got, want *string) {
	t.Helper()
	if (got == nil) != (want == nil) || (got != nil && *got != *want) {
		t.Fatalf("%s got %v want %v", name, got, want)
	}
}

func compatibleCargo() compat.Cargo {
	return compat.Cargo{ID: "compatible", WeightKg: f64(1000), VolumeM3: f64(10), PalletCount: ip(4), PalletTypeCode: sp("EUR"), MaxLoadedHeightMM: ip(2000)}
}

func cargoWithHeight(mm int) compat.Cargo {
	return compat.Cargo{ID: "height", MaxLoadedHeightMM: ip(mm)}
}
func cargoWithWeight(kg float64) compat.Cargo {
	return compat.Cargo{ID: "weight", WeightKg: f64(kg)}
}
func cargoWithVolume(m3 float64) compat.Cargo { return compat.Cargo{ID: "volume", VolumeM3: f64(m3)} }
func cargoWithPallets(n int, code string) compat.Cargo {
	return compat.Cargo{ID: "pallets", PalletCount: ip(n), PalletTypeCode: sp(code)}
}
func cargoWithTemperature(low, high float64) compat.Cargo {
	required := true
	return compat.Cargo{ID: "temp", TemperatureRequired: &required, TemperatureMinC: f64(low), TemperatureMaxC: f64(high)}
}
func cargoWithADR() compat.Cargo {
	danger := true
	return compat.Cargo{ID: "adr", DangerousGoods: &danger, HazardClasses: []string{"3"}}
}
func cargoWithFood() compat.Cargo {
	required := true
	return compat.Cargo{ID: "food", FoodGradeRequired: &required}
}
func loadWithHeight(mm int) domain.LoadOpportunity {
	return domain.LoadOpportunity{Cargo: domain.CargoConstraints{MaxLoadedHeightMM: ip(mm)}}
}

func ip(v int) *int       { return &v }
func sp(v string) *string { return &v }

type vehicleFixture struct {
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

type harness struct {
	URL      string
	Token    string
	TenantID uuid.UUID
	rows     []vehicleFixture
}

func (h harness) scenario(name string) vehicleFixture {
	for _, row := range h.rows {
		if row.Scenario == name {
			return row
		}
	}
	panic("missing scenario " + name)
}

var (
	harnessOnce sync.Once
	harnessLive harness
	harnessErr  error
	harnessStop func()
)

func startShipmentHarness(t *testing.T) harness {
	t.Helper()
	harnessOnce.Do(func() {
		harnessLive, harnessStop, harnessErr = launchHarness()
	})
	if harnessErr != nil {
		t.Fatal(harnessErr)
	}
	return harnessLive
}

func launchHarness() (harness, func(), error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return harness{}, nil, errors.New("caller")
	}
	dir := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "shipment-service"))
	cmd := exec.Command("go", "run", "./cmd/nlo10a-contract-harness")
	cmd.Dir = dir
	cmd.Env = os.Environ()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return harness{}, nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return harness{}, nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return harness{}, nil, err
	}
	stop := func() {
		_ = stdin.Close()
		done := make(chan struct{})
		go func() { _ = cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
		}
	}
	reader := bufio.NewReader(stdout)
	var url string
	var raw []byte
	for {
		line, err := reader.ReadString('\n')
		if strings.HasPrefix(line, "NLO10A_URL=") {
			url = strings.TrimSpace(strings.TrimPrefix(line, "NLO10A_URL="))
		}
		if strings.HasPrefix(line, "NLO10A_FIXTURE=") {
			raw = []byte(strings.TrimSpace(strings.TrimPrefix(line, "NLO10A_FIXTURE=")))
		}
		if url != "" && len(raw) > 0 {
			break
		}
		if err != nil {
			stop()
			return harness{}, nil, errors.New(strings.TrimSpace(stderr.String()) + " " + err.Error())
		}
	}
	go io.Copy(io.Discard, reader)
	var doc struct {
		Token    string           `json:"token"`
		TenantID uuid.UUID        `json:"tenant_id"`
		Vehicles []vehicleFixture `json:"vehicles"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		stop()
		return harness{}, nil, err
	}
	return harness{URL: url, Token: doc.Token, TenantID: doc.TenantID, rows: doc.Vehicles}, stop, nil
}

func TestMain(m *testing.M) {
	code := m.Run()
	if harnessStop != nil {
		harnessStop()
	}
	os.Exit(code)
}
