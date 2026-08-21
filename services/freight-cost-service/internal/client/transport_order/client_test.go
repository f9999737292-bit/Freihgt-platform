package transport_order

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/freight-cost-service/internal/platform/errors"
	fcmetrics "github.com/freight-platform/freight-cost-service/internal/platform/metrics"
)

func TestFC_A_SRC_001_SnapshotTotalDecimalStringPreserved(t *testing.T) {
	t.Parallel()

	transportOrderID := uuid.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"transport_order_id":    transportOrderID.String(),
			"tenant_id":             uuid.New().String(),
			"buyer_company_id":      uuid.New().String(),
			"carrier_company_id":    uuid.New().String(),
			"snapshot_id":           uuid.New().String(),
			"currency_code":         "RUB",
			"total_amount":          "150000.00",
			"pricing_source":        "CONTRACT_RATE",
			"pricing_model_version": "SNAPSHOT_V1",
			"resolved_at":           "2026-08-21T12:00:00Z",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", fcmetrics.New())
	fact, err := client.GetRateSnapshot(context.Background(), uuid.New(), transportOrderID)
	if err != nil {
		t.Fatalf("get snapshot: %v", err)
	}
	if got := fact.TotalAmount.StringFixed(2); got != "150000.00" {
		t.Fatalf("total = %s", got)
	}
}

func TestFC_A_SRC_002_DownstreamUnavailableReturns503(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", fcmetrics.New())
	_, err := client.GetRateSnapshot(context.Background(), uuid.New(), uuid.New())
	var appErr *apperrors.AppError
	if err == nil {
		t.Fatal("expected error")
	}
	if !errorsAs(err, &appErr) || appErr.Code != apperrors.CodeUnavailable {
		t.Fatalf("expected SERVICE_UNAVAILABLE, got %v", err)
	}
}

func TestFC_A_SRC_003_InvalidDownstreamDecimalReturns502(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"transport_order_id":    uuid.New().String(),
			"tenant_id":             uuid.New().String(),
			"buyer_company_id":      uuid.New().String(),
			"carrier_company_id":    uuid.New().String(),
			"snapshot_id":           uuid.New().String(),
			"currency_code":         "RUB",
			"total_amount":          "1e3",
			"pricing_source":        "CONTRACT_RATE",
			"pricing_model_version": "SNAPSHOT_V1",
			"resolved_at":           "2026-08-21T12:00:00Z",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", fcmetrics.New())
	_, err := client.GetRateSnapshot(context.Background(), uuid.New(), uuid.New())
	var appErr *apperrors.AppError
	if err == nil {
		t.Fatal("expected error")
	}
	if !errorsAs(err, &appErr) || appErr.Code != apperrors.CodeBadGateway {
		t.Fatalf("expected BAD_GATEWAY, got %v", err)
	}
}

func TestFC_A_SRC_004_TOMissing404VsUnpriced409(t *testing.T) {
	t.Parallel()

	transportOrderID := uuid.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/v1/transport-orders/" + transportOrderID.String() + "/rate-snapshot":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusConflict)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", fcmetrics.New())

	_, err := client.GetRateSnapshot(context.Background(), uuid.New(), transportOrderID)
	var notFound *apperrors.AppError
	if !errorsAs(err, &notFound) || notFound.Code != apperrors.CodeNotFound {
		t.Fatalf("expected NOT_FOUND, got %v", err)
	}

	_, err = client.GetRateSnapshot(context.Background(), uuid.New(), uuid.New())
	var conflict *apperrors.AppError
	if !errorsAs(err, &conflict) || conflict.Code != apperrors.CodeConflict {
		t.Fatalf("expected CONFLICT, got %v", err)
	}
}

func errorsAs(err error, target **apperrors.AppError) bool {
	if err == nil {
		return false
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok {
		return false
	}
	*target = appErr
	return true
}
