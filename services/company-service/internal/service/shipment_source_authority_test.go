package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/company-service/internal/domain"
)

func TestShipmentSourceAuthorityRedirectFailsClosed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sink" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"decision":"CONFIRMED"}`))
			return
		}
		http.Redirect(w, r, "/sink", http.StatusFound)
	}))
	defer server.Close()
	client := NewShipmentSourceAuthorityClient(server.URL, "revalidation-token", nil)
	decision, err := client.ConfirmCreate(context.Background(), domain.SourceCreateSubject{
		SourceType: domain.SourceTypeShipment, SourceID: uuid.New(),
	})
	if err != nil || decision != domain.SourceCreateUnavailable {
		t.Fatalf("redirect confirm = %s %v", decision, err)
	}
}

func TestWireShipmentSourceAuthorityStaysFailClosed(t *testing.T) {
	for _, pair := range [][2]string{{"", "token"}, {"http://shipment.internal", ""}, {"", ""}} {
		_, authority := WireShipmentSourceAuthority(pair[0], pair[1])
		decision, err := authority.ConfirmCreate(context.Background(), domain.SourceCreateSubject{
			SourceType: domain.SourceTypeShipment, SourceID: uuid.New(),
		})
		if err != nil || decision != domain.SourceCreateUnavailable {
			t.Fatalf("pair %q confirm = %s %v", pair, decision, err)
		}
	}
}

func TestShipmentSourceAuthorityServiceDownFailsClosed(t *testing.T) {
	client := NewShipmentSourceAuthorityClient("http://127.0.0.1:1", "revalidation-token", &http.Client{Timeout: 200 * time.Millisecond})
	subject := domain.SourceCreateSubject{SourceType: domain.SourceTypeShipment, SourceID: uuid.New(), SourceTenantID: uuid.New()}
	decision, err := client.ConfirmCreate(context.Background(), subject)
	if err != nil || decision != domain.SourceCreateUnavailable {
		t.Fatalf("down confirm = %s %v", decision, err)
	}
	revalidated, err := client.Revalidate(context.Background(), domain.SourceSnapshot{SourceType: domain.SourceTypeShipment, SourceID: subject.SourceID, SourceTenantID: subject.SourceTenantID})
	if err != nil || revalidated != domain.SourceUnavailable {
		t.Fatalf("down revalidate = %s %v", revalidated, err)
	}
}

func TestShipmentSourceAuthorityClient(t *testing.T) {
	var calls int
	var confirmed map[string]string
	subject := domain.SourceCreateSubject{
		SourceType: domain.SourceTypeShipment, SourceID: uuid.New(), SourceTenantID: uuid.New(),
		Fingerprint: "fingerprint-v4", ShipperCarrierCompanyID: uuid.New(), RequestingCompanyID: uuid.New(), RequestingUserID: uuid.New(),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("X-Internal-Service-Name") != "" || r.Header.Get("X-Internal-Service-Token") != "" || r.Header.Get("Authorization") != "" {
			t.Errorf("shared internal credentials must not be presented")
		}
		if r.Header.Get(shipmentSourceAuthorityHeader) != "revalidation-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path == shipmentSourceAuthorityRevalidatePath {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"decision":"CHANGED"}`))
			return
		}
		if r.URL.Path != shipmentSourceAuthorityConfirmPath || json.Unmarshal(body, &confirmed) != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"decision":"UNAVAILABLE"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"decision":"STALE"}`))
	}))
	defer server.Close()

	client := NewShipmentSourceAuthorityClient(server.URL+"/", "revalidation-token", server.Client())
	decision, err := client.ConfirmCreate(context.Background(), subject)
	if err != nil || decision != domain.SourceCreateStale {
		t.Fatalf("stale confirm = %s %v", decision, err)
	}
	if confirmed["source_version_or_fingerprint"] != subject.Fingerprint || confirmed["requesting_user_id"] != subject.RequestingUserID.String() || confirmed["requesting_company_id"] != subject.RequestingCompanyID.String() || confirmed["source_tenant_id"] != subject.SourceTenantID.String() || confirmed["shipper_carrier_company_id"] != subject.ShipperCarrierCompanyID.String() {
		t.Fatalf("confirm payload = %#v", confirmed)
	}
	revalidated, err := client.Revalidate(context.Background(), domain.SourceSnapshot{
		SourceType: domain.SourceTypeShipment, SourceID: uuid.New(), SourceTenantID: uuid.New(),
		Fingerprint: "abc", ShipperCarrierCompanyID: uuid.New(),
	})
	if err != nil || revalidated != domain.SourceChanged {
		t.Fatalf("changed revalidate = %s %v", revalidated, err)
	}
	rfx, err := client.ConfirmCreate(context.Background(), domain.SourceCreateSubject{SourceType: domain.SourceTypeRfxAward, SourceID: uuid.New()})
	if err != nil || rfx != domain.SourceCreateUnavailable {
		t.Fatalf("rfx confirm = %s %v", rfx, err)
	}
	if calls != 2 {
		t.Fatalf("rfx source must not call the shipment adapter, calls=%d", calls)
	}

	down := NewShipmentSourceAuthorityClient(server.URL, "revalidation-token", server.Client())
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"decision":"CONFIRMED"}`))
	})
	unknown, err := down.Revalidate(context.Background(), domain.SourceSnapshot{SourceType: domain.SourceTypeShipment, SourceID: uuid.New()})
	if err != nil || unknown != domain.SourceUnavailable {
		t.Fatalf("unknown revalidation decision = %s %v", unknown, err)
	}
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	refused, err := down.ConfirmCreate(context.Background(), domain.SourceCreateSubject{SourceType: domain.SourceTypeShipment, SourceID: uuid.New()})
	if err != nil || refused != domain.SourceCreateUnavailable {
		t.Fatalf("unauthorized callback = %s %v", refused, err)
	}
}
