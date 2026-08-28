package contractrate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/transport-order-service/internal/domain"
)

func TestResolveMissingURLFailsClearly(t *testing.T) {
	t.Parallel()
	client := New(Config{BaseURL: ""})
	actor := domain.InternalActor{
		TenantID:  uuid.New(),
		UserID:    uuid.New(),
		CompanyID: uuid.New(),
		ActorKind: "BUYER",
	}
	carrierID := uuid.New()
	in := domain.CreatePricedTransportOrderInput{
		CreateTransportOrderInput: domain.CreateTransportOrderInput{
			TenantID:              actor.TenantID,
			ShipperCompanyID:      actor.CompanyID,
			OriginLocationID:      uuid.New(),
			DestinationLocationID: uuid.New(),
			TransportMode:         "ROAD",
		},
		Actor: actor,
		PricingContext: domain.PricingContext{
			CarrierCompanyID: carrierID,
		},
	}
	_, err := client.Resolve(context.Background(), in, time.Now().UTC())
	if err == nil {
		t.Fatal("expected error for missing contract rate service url")
	}
	if !strings.Contains(err.Error(), "contract rate service url is not configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveConfiguredURLSucceeds(t *testing.T) {
	t.Parallel()
	carrierID := uuid.New()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/v1/rates/resolve" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-Actor-Kind") != "BUYER" {
			t.Fatalf("missing actor kind header")
		}
		total := "1500.00"
		_ = json.NewEncoder(w).Encode(domain.ResolveRateResult{
			Status:           "MATCHED",
			PricingSource:    "CONTRACT",
			TotalAmount:      &total,
			CurrencyCode:     strPtr("RUB"),
			PricingDate:      time.Now().UTC().Format("2006-01-02"),
			ResolvedAt:       time.Now().UTC(),
			CarrierCompanyID: &carrierID,
		})
	}))
	defer srv.Close()

	client := New(Config{BaseURL: srv.URL, InternalServiceToken: "test-token"})
	actor := domain.InternalActor{
		TenantID:  uuid.New(),
		UserID:    uuid.New(),
		CompanyID: uuid.New(),
		ActorKind: "BUYER",
	}
	equip := "TAUTLINER"
	in := domain.CreatePricedTransportOrderInput{
		CreateTransportOrderInput: domain.CreateTransportOrderInput{
			TenantID:              actor.TenantID,
			ShipperCompanyID:      actor.CompanyID,
			OriginLocationID:      uuid.New(),
			DestinationLocationID: uuid.New(),
			EquipmentType:         &equip,
			TransportMode:         "ROAD",
		},
		Actor: actor,
		PricingContext: domain.PricingContext{
			CarrierCompanyID: carrierID,
		},
	}
	result, err := client.Resolve(context.Background(), in, time.Now().UTC())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if result.Status != "MATCHED" {
		t.Fatalf("status=%q", result.Status)
	}
	if result.TotalAmount == nil || *result.TotalAmount != "1500.00" {
		t.Fatalf("total=%v", result.TotalAmount)
	}
}

func TestBuildSnapshotFromResolve(t *testing.T) {
	t.Parallel()
	total := "2000.00"
	base := "1800.00"
	carrierID := uuid.New()
	in := domain.CreatePricedTransportOrderInput{
		CreateTransportOrderInput: domain.CreateTransportOrderInput{
			TenantID:              uuid.New(),
			ShipperCompanyID:      uuid.New(),
			OriginLocationID:      uuid.New(),
			DestinationLocationID: uuid.New(),
			TransportMode:         "ROAD",
		},
	}
	result := domain.ResolveRateResult{
		Status:        "MATCHED",
		PricingSource: "CONTRACT",
		TotalAmount:   &total,
		BaseAmount:    &base,
		PricingDate:   time.Now().UTC().Format("2006-01-02"),
		ResolvedAt:    time.Now().UTC(),
	}
	snap, err := BuildSnapshotFromResolve(in, result, strings.Repeat("a", 64), carrierID)
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	if !snap.TotalAmount.Equal(decimal.RequireFromString("2000.00")) {
		t.Fatalf("total=%s", snap.TotalAmount)
	}
}

func strPtr(v string) *string { return &v }
