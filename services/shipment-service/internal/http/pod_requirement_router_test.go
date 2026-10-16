package http_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	shipmenthttp "github.com/freight-platform/shipment-service/internal/http"
)

func TestPODRequirementRouteRequiresInternalToken(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := shipmenthttp.NewRouter(log, podQueryDB{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "pod-token", nil)
	path := "/internal/v1/transport-executions/" + uuid.NewString() + "/actions/" + uuid.NewString() + "/pod-requirement"
	missing := httptest.NewRequest(http.MethodGet, path, nil)
	missing.Header.Set("X-Tenant-ID", uuid.NewString())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, missing)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing token %d", rec.Code)
	}
	wrong := httptest.NewRequest(http.MethodGet, path, nil)
	wrong.Header.Set("X-Internal-Service-Token", "other")
	wrong.Header.Set("X-Tenant-ID", uuid.NewString())
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, wrong)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token %d", rec.Code)
	}
	public := httptest.NewRecorder()
	router.ServeHTTP(public, httptest.NewRequest(http.MethodGet, "/v1/transport-executions/"+uuid.NewString()+"/actions/"+uuid.NewString()+"/pod-requirement", nil))
	if public.Code != http.StatusNotFound {
		t.Fatalf("public route %d", public.Code)
	}
}

func TestPODRequirementDependencyFailureDoesNotInventKnownFalse(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := shipmenthttp.NewRouter(log, podFailDB{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "pod-token", nil)
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/transport-executions/"+uuid.NewString()+"/actions/"+uuid.NewString()+"/pod-requirement", nil)
	req.Header.Set("X-Internal-Service-Token", "pod-token")
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, `"pod_required":false`) || strings.Contains(body, `"decision_state":"KNOWN"`) || strings.Contains(body, `"pod_required": false`) {
		t.Fatalf("dependency failure invented a decision: %s", body)
	}
}

type podQueryDB struct{}

func (podQueryDB) Ping(context.Context) error { return nil }

func (podQueryDB) QueryRow(context.Context, string, ...any) pgx.Row { return podFailRow{} }

type podFailDB struct{}

func (podFailDB) Ping(context.Context) error { return nil }

func (podFailDB) QueryRow(context.Context, string, ...any) pgx.Row { return podFailRow{} }

type podFailRow struct{}

func (podFailRow) Scan(...any) error { return errors.New("store down") }
