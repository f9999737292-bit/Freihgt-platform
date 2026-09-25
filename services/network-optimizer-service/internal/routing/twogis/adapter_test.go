package twogis

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/freight-platform/network-optimizer-service/internal/routing"
)

func TestBNO180TwoGISMapping(t *testing.T) {
	var seenKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenKey = r.URL.Query().Get("key")
		if strings.Contains(r.URL.Path, "get_dist_matrix") {
			_, _ = w.Write([]byte(`{"routes":[{"source_id":0,"target_id":1,"distance":80000,"duration":4800,"status":"OK"}]}`))
			return
		}
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
