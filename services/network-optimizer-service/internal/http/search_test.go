package http_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	httpserver "github.com/freight-platform/network-optimizer-service/internal/http"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/service"
)

func TestNextLoadSearchRoute(t *testing.T) {
	srv := httptest.NewServer(httpserver.NewRouter(slog.New(slog.DiscardHandler), service.New(repository.NewMemory(), nil), nil))
	t.Cleanup(srv.Close)
	missingAuth, err := http.Post(srv.URL+"/v1/network/next-load/search", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer missingAuth.Body.Close()
	if missingAuth.StatusCode != http.StatusUnauthorized {
		t.Fatalf("auth %d", missingAuth.StatusCode)
	}
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/v1/network/next-load/search", strings.NewReader(`{"capacity_id":"`+uuid.NewString()+`","policy":{"search_mode":"RADIUS","radius_km":25,"objective_profile":"MIN_EMPTY"}}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	req.Header.Set("X-User-ID", uuid.NewString())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("missing capacity %d", res.StatusCode)
	}
}
