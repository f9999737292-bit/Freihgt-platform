package http_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	shipmenthttp "github.com/freight-platform/shipment-service/internal/http"
)

func TestOperationsAnalyticsRouteFollowsDatabase(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	plain := shipmenthttp.NewRouter(log, analyticsPingOnly{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "analytics-token", nil)
	if code := analyticsRouteStatus(plain); code != http.StatusNotFound {
		t.Fatalf("route without query database: %d", code)
	}
	wired := shipmenthttp.NewRouter(log, analyticsQueryDB{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "analytics-token", nil)
	if code := analyticsRouteStatus(wired); code == http.StatusNotFound {
		t.Fatal("query database did not register the internal route")
	}
	public := httptest.NewRecorder()
	wired.ServeHTTP(public, httptest.NewRequest(http.MethodGet, "/v1/analytics/operations-foundation", nil))
	if public.Code != http.StatusNotFound {
		t.Fatalf("public route %d", public.Code)
	}
}

func analyticsRouteStatus(router http.Handler) int {
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/analytics/operations-foundation", nil)
	req.Header.Set("X-Internal-Service-Token", "analytics-token")
	req.Header.Set("X-Internal-Service-Name", "analytics-service")
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Code
}

type analyticsPingOnly struct{}

func (analyticsPingOnly) Ping(context.Context) error { return nil }

type analyticsQueryDB struct{}

func (analyticsQueryDB) Ping(context.Context) error { return nil }

func (analyticsQueryDB) QueryRow(context.Context, string, ...any) pgx.Row {
	return analyticsFailRow{}
}

type analyticsFailRow struct{}

func (analyticsFailRow) Scan(...any) error { return errors.New("unused") }
