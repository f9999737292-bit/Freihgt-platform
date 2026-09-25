package twogis

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/freight-platform/network-optimizer-service/internal/routing"
)

func TestTwoGISProviderName(t *testing.T) {
	adapter := New("https://routing.api.2gis.com", "test-key", nil)
	if adapter.ProviderName() != "2GIS" {
		t.Fatalf("provider %s", adapter.ProviderName())
	}
}

func TestBNO180TwoGISMapping(t *testing.T) {
	var seenKey string
	var routeBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenKey = r.URL.Query().Get("key")
		body := decodeProviderBody(t, r)
		if strings.Contains(r.URL.Path, "get_dist_matrix") {
			if body["transport"] != "truck" {
				t.Fatalf("BNO180 matrix transport %v", body["transport"])
			}
			_, _ = w.Write([]byte(`{"routes":[{"source_id":0,"target_id":1,"distance":80000,"duration":4800,"status":"OK"}]}`))
			return
		}
		routeBody = body
		_, _ = w.Write([]byte(`{"status":"OK","result":[{"id":"route-1","total_distance":82000,"total_duration":5400,"maneuvers":[{"outcoming_path":{"geometry":[{"selection":"LINESTRING(60.597 56.838, 37.617 55.756)"}]}}]}]}`))
	}))
	t.Cleanup(srv.Close)
	adapter := New(srv.URL, "test-key", slog.New(slog.DiscardHandler))
	result, err := adapter.Route(context.Background(), routing.RouteRequest{
		Origin: routing.Point{Latitude: 56.838, Longitude: 60.597}, Destination: routing.Point{Latitude: 55.756, Longitude: 37.617},
	})
	if err != nil || result.Provider != "2GIS" || result.DistanceM != 82000 || result.DurationSeconds != 5400 || result.ProviderDefaultUsed != true {
		t.Fatalf("BNO180 %+v %v", result, err)
	}
	if result.Geometry.Type != "LineString" || len(result.Geometry.Coordinates) != 2 || result.RequestFingerprint == "" {
		t.Fatalf("BNO180 geometry %+v", result.Geometry)
	}
	if seenKey != "test-key" {
		t.Fatal("BNO180 key was not sent to the provider")
	}
	if routeBody["transport"] != "truck" || routeBody["output"] != "detailed" || routeBody["traffic_mode"] != "jam" || routeBody["route_mode"] != "fastest" {
		t.Fatalf("BNO180 request %+v", routeBody)
	}
	if _, ok := routeBody["truck"]; ok {
		t.Fatal("BNO180 sent a top-level truck object")
	}
	matrix, err := adapter.Matrix(context.Background(), routing.MatrixRequest{
		Origins: []routing.Point{{Latitude: 56.8, Longitude: 60.6}}, Destinations: []routing.Point{{Latitude: 55.7, Longitude: 37.6}},
	})
	if err != nil || len(matrix.Cells) != 1 || matrix.Cells[0].DistanceM != 80000 || matrix.Cells[0].DurationSeconds != 4800 {
		t.Fatalf("BNO180 matrix %+v %v", matrix, err)
	}
}

func TestBNO181TimeoutFailsClosed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	t.Cleanup(srv.Close)
	adapter := New(srv.URL, "test-key", slog.New(slog.DiscardHandler))
	adapter.client.Timeout = 20 * time.Millisecond
	_, err := adapter.Route(context.Background(), routing.RouteRequest{})
	if err != routing.ErrTimeout {
		t.Fatalf("BNO181 %v", err)
	}
}

func TestBNO182RouteNotFoundFailsClosed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"FAIL"}`))
	}))
	t.Cleanup(srv.Close)
	adapter := New(srv.URL, "test-key", slog.New(slog.DiscardHandler))
	_, err := adapter.Route(context.Background(), routing.RouteRequest{})
	if err != routing.ErrRouteNotFound {
		t.Fatalf("BNO182 %v", err)
	}
}

func TestBNO194RoutingSecretNotLogged(t *testing.T) {
	secret := "super-secret-2gis-key"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)
	var buf bytes.Buffer
	adapter := New(srv.URL, secret, slog.New(slog.NewJSONHandler(&buf, nil)))
	_, err := adapter.Route(context.Background(), routing.RouteRequest{})
	if err != routing.ErrProviderUnavailable {
		t.Fatalf("BNO194 %v", err)
	}
	if strings.Contains(buf.String(), secret) || strings.Contains(err.Error(), secret) {
		t.Fatalf("BNO194 secret logged %s", buf.String())
	}
}

func TestBNO195TwoGISRouteRequestSchema(t *testing.T) {
	var body map[string]any
	adapter := routeAdapter(t, func(got map[string]any) { body = got })
	mass := 20000.0
	height := 4.0
	width := 2.5
	length := 13.6
	axle := 8000.0
	danger := true
	when := time.Date(2026, 9, 25, 12, 7, 0, 0, time.UTC)
	result, err := adapter.Route(context.Background(), routing.RouteRequest{
		Origin: routing.Point{Latitude: 56.838, Longitude: 60.597}, Destination: routing.Point{Latitude: 55.756, Longitude: 37.617},
		DepartureAt: &when, TrafficMode: routing.TrafficStatistical, RouteMode: routing.RouteFastest,
		VehicleProfile: routing.VehicleProfile{GrossWeightKg: &mass, HeightM: &height, WidthM: &width, LengthM: &length, AxleLoadKg: &axle, DangerousCargo: &danger},
	})
	if err != nil || result.ProviderDefaultUsed {
		t.Fatalf("BNO195 %+v %v", result, err)
	}
	if body["transport"] != "truck" || body["output"] != "detailed" || body["traffic_mode"] != "statistics" || body["route_mode"] != "fastest" {
		t.Fatalf("BNO195 envelope %+v", body)
	}
	if _, ok := body["truck"]; ok {
		t.Fatal("BNO195 top-level truck")
	}
	if body["utc"] != float64(when.Unix()) {
		t.Fatalf("BNO195 utc %v", body["utc"])
	}
	params, _ := body["params"].(map[string]any)
	truck, _ := params["truck"].(map[string]any)
	if truck["mass"] != 20.0 || truck["height"] != 4.0 || truck["width"] != 2.5 || truck["length"] != 13.6 || truck["axle_load"] != 8.0 || truck["dangerous_cargo"] != true {
		t.Fatalf("BNO195 truck %+v", truck)
	}
	if _, ok := truck["max_perm_mass"]; ok {
		t.Fatal("BNO195 invented max_perm_mass")
	}
}

func TestBNO196TwoGISMatrixRequestSchema(t *testing.T) {
	var body map[string]any
	adapter := matrixAdapter(t, func(got map[string]any) { body = got }, `{"routes":[{"source_id":0,"target_id":1,"distance":80000,"duration":4800,"status":"OK"}]}`)
	mass := 18500.0
	result, err := adapter.Matrix(context.Background(), routing.MatrixRequest{
		Origins: []routing.Point{{Latitude: 56.8, Longitude: 60.6}}, Destinations: []routing.Point{{Latitude: 55.7, Longitude: 37.6}},
		VehicleProfile: routing.VehicleProfile{GrossWeightKg: &mass},
	})
	if err != nil || result.Provider != "2GIS" || result.ProviderDefaultUsed != true || result.RequestFingerprint == "" || result.ExpiresAt.IsZero() {
		t.Fatalf("BNO196 %+v %v", result, err)
	}
	if body["transport"] != "truck" {
		t.Fatalf("BNO196 transport %v", body["transport"])
	}
	if _, ok := body["params"]; ok || body["truck"] != nil {
		t.Fatalf("BNO196 used the route schema %+v", body)
	}
	truck, _ := body["truck_params"].(map[string]any)
	if truck["mass"] != 18.5 {
		t.Fatalf("BNO196 mass %+v", truck)
	}
	for _, absent := range []string{"height", "width", "length", "axle_load", "dangerous_cargo", "max_perm_mass"} {
		if _, ok := truck[absent]; ok {
			t.Fatalf("BNO196 sent unknown %s", absent)
		}
	}
}

func TestBNO197PartialTruckProfileStaysTruck(t *testing.T) {
	var routeBody, matrixBody map[string]any
	route := routeAdapter(t, func(got map[string]any) { routeBody = got })
	matrix := matrixAdapter(t, func(got map[string]any) { matrixBody = got }, `{"routes":[{"source_id":0,"target_id":1,"distance":1,"duration":1,"status":"OK"}]}`)
	mass := 12000.0
	profile := routing.VehicleProfile{GrossWeightKg: &mass}
	routeResult, err := route.Route(context.Background(), routing.RouteRequest{VehicleProfile: profile})
	if err != nil || !routeResult.ProviderDefaultUsed || routeBody["transport"] != "truck" {
		t.Fatalf("BNO197 route %+v %v %+v", routeResult, err, routeBody)
	}
	if routeBody["transport"] == "driving" || matrixBody != nil && matrixBody["transport"] == "driving" {
		t.Fatal("BNO197 fell back to driving")
	}
	matrixResult, err := matrix.Matrix(context.Background(), routing.MatrixRequest{
		Origins: []routing.Point{{Latitude: 1, Longitude: 2}}, Destinations: []routing.Point{{Latitude: 3, Longitude: 4}},
		VehicleProfile: profile,
	})
	if err != nil || !matrixResult.ProviderDefaultUsed || matrixBody["transport"] != "truck" {
		t.Fatalf("BNO197 matrix %+v %v %+v", matrixResult, err, matrixBody)
	}
	complete := routing.VehicleProfile{GrossWeightKg: &mass, HeightM: f64(3.8), WidthM: f64(2.4), LengthM: f64(12), AxleLoadKg: f64(7000), DangerousCargo: b(false)}
	completeRoute := routeAdapter(t, func(map[string]any) {})
	completeResult, err := completeRoute.Route(context.Background(), routing.RouteRequest{VehicleProfile: complete})
	if err != nil || completeResult.ProviderDefaultUsed {
		t.Fatalf("BNO197 complete %+v %v", completeResult, err)
	}
}

func TestBNO198ProviderTrafficModeTranslation(t *testing.T) {
	var seen []string
	adapter := routeAdapter(t, func(body map[string]any) {
		raw, _ := json.Marshal(body)
		seen = append(seen, string(raw))
	})
	if _, err := adapter.Route(context.Background(), routing.RouteRequest{TrafficMode: routing.TrafficCurrent}); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Route(context.Background(), routing.RouteRequest{TrafficMode: routing.TrafficStatistical, DepartureAt: timePtr(time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC))}); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(seen, "\n")
	if strings.Contains(joined, `"STATIC"`) || strings.Contains(joined, `"LIVE"`) || strings.Contains(joined, `"CURRENT"`) || strings.Contains(joined, `"STATISTICAL"`) {
		t.Fatalf("BNO198 internal mode leaked %s", joined)
	}
	if !strings.Contains(seen[0], `"traffic_mode":"jam"`) || !strings.Contains(seen[1], `"traffic_mode":"statistics"`) {
		t.Fatalf("BNO198 mapping %s", joined)
	}
	if _, err := adapter.Route(context.Background(), routing.RouteRequest{TrafficMode: "STATIC"}); err != routing.ErrInvalidResponse {
		t.Fatalf("BNO198 STATIC accepted %v", err)
	}
	if _, err := adapter.Route(context.Background(), routing.RouteRequest{TrafficMode: "LIVE"}); err != routing.ErrInvalidResponse {
		t.Fatalf("BNO198 LIVE accepted %v", err)
	}
}

func TestBNO199DepartureTimeSentToProvider(t *testing.T) {
	var routeBody, matrixBody map[string]any
	route := routeAdapter(t, func(got map[string]any) { routeBody = got })
	matrix := matrixAdapter(t, func(got map[string]any) { matrixBody = got }, `{"routes":[{"source_id":0,"target_id":1,"distance":10,"duration":10,"status":"OK"}]}`)
	early := time.Date(2026, 9, 25, 12, 7, 0, 0, time.UTC)
	sameBucket := time.Date(2026, 9, 25, 12, 14, 0, 0, time.UTC)
	nextBucket := time.Date(2026, 9, 25, 12, 16, 0, 0, time.UTC)
	req := func(at time.Time) routing.RouteRequest {
		return routing.RouteRequest{TrafficMode: routing.TrafficStatistical, DepartureAt: &at}
	}
	first, err := route.Route(context.Background(), req(early))
	if err != nil || routeBody["utc"] != float64(early.Unix()) {
		t.Fatalf("BNO199 route utc %v %v", routeBody["utc"], err)
	}
	second, err := route.Route(context.Background(), req(sameBucket))
	if err != nil || first.RequestFingerprint != second.RequestFingerprint {
		t.Fatalf("BNO199 same bucket %s %s %v", first.RequestFingerprint, second.RequestFingerprint, err)
	}
	third, err := route.Route(context.Background(), req(nextBucket))
	if err != nil || third.RequestFingerprint == first.RequestFingerprint || routeBody["utc"] != float64(nextBucket.Unix()) {
		t.Fatalf("BNO199 bucket change %s %s utc %v", first.RequestFingerprint, third.RequestFingerprint, routeBody["utc"])
	}
	matrixReq := routing.MatrixRequest{
		Origins: []routing.Point{{Latitude: 1, Longitude: 2}}, Destinations: []routing.Point{{Latitude: 3, Longitude: 4}},
		TrafficMode: routing.TrafficStatistical, DepartureAt: &early,
	}
	if _, err := matrix.Matrix(context.Background(), matrixReq); err != nil || matrixBody["start_time"] != early.Format(time.RFC3339) {
		t.Fatalf("BNO199 matrix start_time %v %v", matrixBody["start_time"], err)
	}
	current := routeAdapter(t, func(got map[string]any) { routeBody = got })
	currentEarly, err := current.Route(context.Background(), routing.RouteRequest{TrafficMode: routing.TrafficCurrent, DepartureAt: &early})
	currentLate, err2 := current.Route(context.Background(), routing.RouteRequest{TrafficMode: routing.TrafficCurrent, DepartureAt: &nextBucket})
	_, sent := routeBody["utc"]
	if err != nil || err2 != nil || sent || currentEarly.RequestFingerprint != currentLate.RequestFingerprint {
		t.Fatalf("BNO199 current traffic fingerprinted an unsent time %+v %s %s", routeBody, currentEarly.RequestFingerprint, currentLate.RequestFingerprint)
	}
}

func TestBNO200MatrixFailedCellNotZeroDistance(t *testing.T) {
	adapter := matrixAdapter(t, func(map[string]any) {}, `{"routes":[
		{"source_id":0,"target_id":1,"distance":9999,"duration":100,"status":"ROUTE_NOT_FOUND"},
		{"source_id":0,"target_id":2,"distance":0,"duration":0,"status":"ROUTE_DOES_NOT_EXISTS"},
		{"source_id":0,"target_id":3,"distance":50,"duration":5,"status":"POINT_EXCLUDED"},
		{"source_id":0,"target_id":4,"distance":70,"duration":7,"status":"ATTRACT_FAIL"},
		{"source_id":0,"target_id":5,"distance":12000,"duration":900,"status":"OK"}
	]}`)
	result, err := adapter.Matrix(context.Background(), routing.MatrixRequest{
		Origins:      []routing.Point{{Latitude: 1, Longitude: 2}},
		Destinations: []routing.Point{{}, {}, {}, {}, {}},
	})
	if err != nil || len(result.Cells) != 5 {
		t.Fatalf("BNO200 %+v %v", result, err)
	}
	for i := 0; i < 4; i++ {
		cell := result.Cells[i]
		if cell.Err != routing.ErrRouteNotFound || cell.DistanceM != 0 || cell.DurationSeconds != 0 {
			t.Fatalf("BNO200 failed cell kept a distance %+v", cell)
		}
	}
	if result.Cells[4].Err != nil || result.Cells[4].DistanceM != 12000 {
		t.Fatalf("BNO200 success cell %+v", result.Cells[4])
	}
}

func TestBNO202MatrixCurrentSendsJam(t *testing.T) {
	var seen []capturedMatrix
	adapter := capturingMatrix(t, &seen)
	when := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	points := routing.MatrixRequest{
		Origins: []routing.Point{{Latitude: 56.8, Longitude: 60.6}}, Destinations: []routing.Point{{Latitude: 55.7, Longitude: 37.6}},
	}
	current := points
	current.TrafficMode = routing.TrafficCurrent
	current.RouteMode = routing.RouteFastest
	current.DepartureAt = &when
	first, err := adapter.Matrix(context.Background(), current)
	if err != nil {
		t.Fatal(err)
	}
	implicit := points
	second, err := adapter.Matrix(context.Background(), implicit)
	if err != nil {
		t.Fatal(err)
	}
	body := seen[0].body
	if body["transport"] != "truck" || body["type"] != "jam" || seen[0].version != "2.0" {
		t.Fatalf("BNO202 %+v version %s", body, seen[0].version)
	}
	if _, sent := body["start_time"]; sent {
		t.Fatal("BNO202 sent start_time for current traffic")
	}
	assertNoInternalMatrixEnums(t, body)
	if first.RequestFingerprint != second.RequestFingerprint || first.TrafficMode != routing.TrafficCurrent || first.RouteMode != routing.RouteFastest {
		t.Fatalf("BNO202 fingerprint treated an unsent field as distinct %s %s", first.RequestFingerprint, second.RequestFingerprint)
	}
}

func TestBNO203MatrixStatisticalSendsStatistics(t *testing.T) {
	var seen []capturedMatrix
	adapter := capturingMatrix(t, &seen)
	when := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	result, err := adapter.Matrix(context.Background(), routing.MatrixRequest{
		Origins: []routing.Point{{Latitude: 56.8, Longitude: 60.6}}, Destinations: []routing.Point{{Latitude: 55.7, Longitude: 37.6}},
		TrafficMode: routing.TrafficStatistical, RouteMode: routing.RouteFastest, DepartureAt: &when,
	})
	if err != nil {
		t.Fatal(err)
	}
	body := seen[0].body
	if body["type"] != "statistics" || body["start_time"] != "2026-09-25T12:00:00Z" || body["transport"] != "truck" {
		t.Fatalf("BNO203 %+v", body)
	}
	assertNoInternalMatrixEnums(t, body)
	jam, err := adapter.Matrix(context.Background(), routing.MatrixRequest{
		Origins: []routing.Point{{Latitude: 56.8, Longitude: 60.6}}, Destinations: []routing.Point{{Latitude: 55.7, Longitude: 37.6}},
		TrafficMode: routing.TrafficCurrent, RouteMode: routing.RouteFastest,
	})
	if err != nil || result.RequestFingerprint == jam.RequestFingerprint || seen[1].body["type"] != "jam" {
		t.Fatalf("BNO203 jam and statistics share a fingerprint %s %s", result.RequestFingerprint, jam.RequestFingerprint)
	}
}

func TestBNO204MatrixStatisticalWithoutTimeFailsClosed(t *testing.T) {
	var seen []capturedMatrix
	adapter := capturingMatrix(t, &seen)
	_, err := adapter.Matrix(context.Background(), routing.MatrixRequest{
		Origins: []routing.Point{{Latitude: 1, Longitude: 2}}, Destinations: []routing.Point{{Latitude: 3, Longitude: 4}},
		TrafficMode: routing.TrafficStatistical, RouteMode: routing.RouteFastest,
	})
	if err != routing.ErrInvalidResponse || len(seen) != 0 {
		t.Fatalf("BNO204 %v calls %d", err, len(seen))
	}
}

func TestBNO205MatrixShortestNotSilentlyJam(t *testing.T) {
	var seen []capturedMatrix
	adapter := capturingMatrix(t, &seen)
	when := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	base := routing.MatrixRequest{
		Origins: []routing.Point{{Latitude: 56.8, Longitude: 60.6}}, Destinations: []routing.Point{{Latitude: 55.7, Longitude: 37.6}},
		RouteMode: routing.RouteShortest,
	}
	shortest, err := adapter.Matrix(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	planned := base
	planned.TrafficMode = routing.TrafficStatistical
	planned.DepartureAt = &when
	same, err := adapter.Matrix(context.Background(), planned)
	if err != nil {
		t.Fatal(err)
	}
	if seen[0].body["type"] != "shortest" || seen[1].body["type"] != "shortest" {
		t.Fatalf("BNO205 types %v %v", seen[0].body["type"], seen[1].body["type"])
	}
	if _, sent := seen[0].body["start_time"]; sent {
		t.Fatal("BNO205 shortest sent start_time")
	}
	if seen[1].body["type"] == "jam" || seen[0].body["type"] == "jam" {
		t.Fatal("BNO205 shortest became jam")
	}
	assertNoInternalMatrixEnums(t, seen[0].body)
	if shortest.RequestFingerprint != same.RequestFingerprint || shortest.TrafficMode != "" || shortest.RouteMode != routing.RouteShortest {
		t.Fatalf("BNO205 metadata claimed traffic %+v %+v", shortest, same)
	}
}

type capturedMatrix struct {
	body    map[string]any
	version string
}

func capturingMatrix(t *testing.T, seen *[]capturedMatrix) *Adapter {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = append(*seen, capturedMatrix{body: decodeProviderBody(t, r), version: r.URL.Query().Get("version")})
		_, _ = w.Write([]byte(`{"routes":[{"source_id":0,"target_id":1,"distance":80000,"duration":4800,"status":"OK"}]}`))
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL, "test-key", slog.New(slog.DiscardHandler))
}

func assertNoInternalMatrixEnums(t *testing.T, body map[string]any) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, forbidden := range []string{`"CURRENT"`, `"STATISTICAL"`, `"FASTEST"`, `"STATIC"`, `"LIVE"`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("internal enum %s in %s", forbidden, text)
		}
	}
}

func routeAdapter(t *testing.T, capture func(map[string]any)) *Adapter {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capture(decodeProviderBody(t, r))
		_, _ = w.Write([]byte(`{"status":"OK","result":[{"id":"route-1","total_distance":82000,"total_duration":5400,"maneuvers":[{"outcoming_path":{"geometry":[{"selection":"LINESTRING(60.597 56.838, 37.617 55.756)"}]}}]}]}`))
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL, "test-key", slog.New(slog.DiscardHandler))
}

func TestBNO232TwoGISMatrixBatchesStayWithinSyncLimit(t *testing.T) {
	calls := 0
	maxTargets := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := decodeProviderBody(t, r)
		calls++
		sources, _ := body["sources"].([]any)
		targets, _ := body["targets"].([]any)
		points, _ := body["points"].([]any)
		if len(sources) > routing.SyncMatrixLimit || len(targets) > routing.SyncMatrixLimit {
			t.Fatalf("BNO232 sources %d targets %d", len(sources), len(targets))
		}
		if len(targets) > maxTargets {
			maxTargets = len(targets)
		}
		routes := make([]map[string]any, 0, len(sources)*len(targets))
		for _, source := range sources {
			for _, target := range targets {
				sourceID := int(source.(float64))
				targetID := int(target.(float64))
				lon := points[targetID].(map[string]any)["lon"].(float64)
				routes = append(routes, map[string]any{
					"source_id": sourceID, "target_id": targetID, "distance": int(lon * 1000), "duration": 60, "status": "OK",
				})
			}
		}
		raw, _ := json.Marshal(map[string]any{"routes": routes})
		_, _ = w.Write(raw)
	}))
	t.Cleanup(srv.Close)
	adapter := New(srv.URL, "test-key", slog.New(slog.DiscardHandler))
	destinations := make([]routing.Point, 30)
	for i := range destinations {
		destinations[i] = routing.Point{Longitude: float64(i + 1)}
	}
	result, err := adapter.Matrix(context.Background(), routing.MatrixRequest{
		Origins: []routing.Point{{Latitude: 56.8, Longitude: 60.6}}, Destinations: destinations,
		RouteMode: routing.RouteFastest, TrafficMode: routing.TrafficStatistical, DepartureAt: timePtr(time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC)),
	})
	if err != nil || calls != 2 || maxTargets != routing.SyncMatrixLimit || len(result.Cells) != 30 {
		t.Fatalf("BNO232 calls=%d max=%d cells=%d err=%v", calls, maxTargets, len(result.Cells), err)
	}
	seen := map[int]int{}
	for _, cell := range result.Cells {
		seen[cell.DestinationIndex] = cell.DistanceM
	}
	if seen[0] != 1000 || seen[25] != 26000 || seen[29] != 30000 {
		t.Fatalf("BNO233 %+v", seen)
	}
}

func matrixAdapter(t *testing.T, capture func(map[string]any), response string) *Adapter {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capture(decodeProviderBody(t, r))
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL, "test-key", slog.New(slog.DiscardHandler))
}

func decodeProviderBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"STATIC"`) || strings.Contains(string(raw), `"LIVE"`) {
		t.Fatalf("literal traffic mode sent: %s", raw)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func f64(v float64) *float64         { return &v }
func b(v bool) *bool                 { return &v }
func timePtr(v time.Time) *time.Time { return &v }
