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

func validSnapshotPayload(tenantID, transportOrderID uuid.UUID, overrides map[string]any) map[string]any {
	payload := map[string]any{
		"transport_order_id":    transportOrderID.String(),
		"tenant_id":             tenantID.String(),
		"buyer_company_id":      uuid.New().String(),
		"carrier_company_id":    uuid.New().String(),
		"snapshot_id":           uuid.New().String(),
		"currency_code":         "RUB",
		"total_amount":          "150000.00",
		"pricing_source":        "CONTRACT_RATE",
		"pricing_model_version": "SNAPSHOT_V1",
		"resolved_at":           "2026-08-21T12:00:00Z",
	}
	for key, value := range overrides {
		payload[key] = value
	}
	return payload
}

func snapshotTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server, NewClient(server.URL, "token", fcmetrics.New())
}

func TestFC_A_SRC_001_SnapshotTotalDecimalStringPreserved(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	transportOrderID := uuid.New()
	_, client := snapshotTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(validSnapshotPayload(tenantID, transportOrderID, nil))
	})

	fact, err := client.GetRateSnapshot(context.Background(), tenantID, transportOrderID)
	if err != nil {
		t.Fatalf("get snapshot: %v", err)
	}
	if got := fact.TotalAmount.StringFixed(2); got != "150000.00" {
		t.Fatalf("total = %s", got)
	}
}

func TestFC_A_SRC_002_DownstreamUnavailableReturns503(t *testing.T) {
	t.Parallel()

	_, client := snapshotTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})

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

	tenantID := uuid.New()
	transportOrderID := uuid.New()
	_, client := snapshotTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(validSnapshotPayload(tenantID, transportOrderID, map[string]any{
			"total_amount": "1e3",
		}))
	})

	_, err := client.GetRateSnapshot(context.Background(), tenantID, transportOrderID)
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
	_, client := snapshotTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/v1/transport-orders/" + transportOrderID.String() + "/rate-snapshot":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusConflict)
		}
	})

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

func TestFC_A_SRC_005_DownstreamTenantMismatchReturns502(t *testing.T) {
	t.Parallel()

	requestedTenant := uuid.New()
	otherTenant := uuid.New()
	transportOrderID := uuid.New()
	_, client := snapshotTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(validSnapshotPayload(otherTenant, transportOrderID, nil))
	})

	_, err := client.GetRateSnapshot(context.Background(), requestedTenant, transportOrderID)
	assertBadGateway(t, err)
}

func TestFC_A_SRC_006_DownstreamTransportOrderMismatchReturns502(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	requestedOrder := uuid.New()
	otherOrder := uuid.New()
	_, client := snapshotTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(validSnapshotPayload(tenantID, otherOrder, nil))
	})

	_, err := client.GetRateSnapshot(context.Background(), tenantID, requestedOrder)
	assertBadGateway(t, err)
}

func TestFC_A_SRC_007_ZeroCanonicalUUIDReturns502(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	transportOrderID := uuid.New()
	cases := []struct {
		name     string
		override map[string]any
	}{
		{name: "buyer_company_id", override: map[string]any{"buyer_company_id": uuid.Nil.String()}},
		{name: "carrier_company_id", override: map[string]any{"carrier_company_id": uuid.Nil.String()}},
		{name: "snapshot_id", override: map[string]any{"snapshot_id": uuid.Nil.String()}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, client := snapshotTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(validSnapshotPayload(tenantID, transportOrderID, tc.override))
			})
			_, err := client.GetRateSnapshot(context.Background(), tenantID, transportOrderID)
			assertBadGateway(t, err)
		})
	}
}

func TestFC_A_SRC_008_InvalidPricingModelVersionReturns502(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	transportOrderID := uuid.New()
	cases := []string{"", " SNAPSHOT_V2 ", "legacy", "unknown"}

	for _, modelVersion := range cases {
		modelVersion := modelVersion
		t.Run(modelVersion, func(t *testing.T) {
			t.Parallel()
			_, client := snapshotTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(validSnapshotPayload(tenantID, transportOrderID, map[string]any{
					"pricing_model_version": modelVersion,
				}))
			})
			_, err := client.GetRateSnapshot(context.Background(), tenantID, transportOrderID)
			assertBadGateway(t, err)
		})
	}
}

func TestFC_A_SRC_009_EmptyPricingSourceReturns502(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	transportOrderID := uuid.New()
	_, client := snapshotTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(validSnapshotPayload(tenantID, transportOrderID, map[string]any{
			"pricing_source": "   ",
		}))
	})

	_, err := client.GetRateSnapshot(context.Background(), tenantID, transportOrderID)
	assertBadGateway(t, err)
}

func TestFC_A_SRC_010_NegativeTotalAmountReturns502(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	transportOrderID := uuid.New()
	_, client := snapshotTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(validSnapshotPayload(tenantID, transportOrderID, map[string]any{
			"total_amount": "-0.01",
		}))
	})

	_, err := client.GetRateSnapshot(context.Background(), tenantID, transportOrderID)
	assertBadGateway(t, err)
}

func TestFC_A_SRC_011_ZeroTotalAmountAccepted(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	transportOrderID := uuid.New()
	_, client := snapshotTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(validSnapshotPayload(tenantID, transportOrderID, map[string]any{
			"total_amount": "0.00",
		}))
	})

	fact, err := client.GetRateSnapshot(context.Background(), tenantID, transportOrderID)
	if err != nil {
		t.Fatalf("get snapshot: %v", err)
	}
	if got := fact.TotalAmount.StringFixed(2); got != "0.00" {
		t.Fatalf("total = %s", got)
	}
}

func assertBadGateway(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.AppError
	if err == nil {
		t.Fatal("expected error")
	}
	if !errorsAs(err, &appErr) || appErr.Code != apperrors.CodeBadGateway {
		t.Fatalf("expected BAD_GATEWAY, got %v", err)
	}
	if appErr.Code == apperrors.CodeForbidden {
		t.Fatal("tenant mismatch must not map to FORBIDDEN")
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
