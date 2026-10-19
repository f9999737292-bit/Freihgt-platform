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

func TestRfxSourceAuthorityRedirectFailsClosed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sink" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"decision":"CONFIRMED"}`))
			return
		}
		http.Redirect(w, r, "/sink", http.StatusFound)
	}))
	defer server.Close()
	client := NewRfxSourceAuthorityClient(server.URL, "revalidation-token", nil)
	decision, err := client.ConfirmCreate(context.Background(), domain.SourceCreateSubject{
		SourceType: domain.SourceTypeRfxAward, SourceID: uuid.New(),
	})
	if err != nil || decision != domain.SourceCreateUnavailable {
		t.Fatalf("redirect confirm = %s %v", decision, err)
	}
}

func TestWireRfxSourceAuthorityStaysFailClosed(t *testing.T) {
	for _, pair := range [][2]string{{"", "token"}, {"http://rfx.internal", ""}, {"", ""}} {
		_, authority := WireRfxSourceAuthority(pair[0], pair[1])
		decision, err := authority.ConfirmCreate(context.Background(), domain.SourceCreateSubject{
			SourceType: domain.SourceTypeAwardLink, SourceID: uuid.New(),
		})
		if err != nil || decision != domain.SourceCreateUnavailable {
			t.Fatalf("pair %q confirm = %s %v", pair, decision, err)
		}
	}
}

func TestRfxSourceAuthorityServiceDownFailsClosed(t *testing.T) {
	client := NewRfxSourceAuthorityClient("http://127.0.0.1:1", "revalidation-token", &http.Client{Timeout: 200 * time.Millisecond})
	subject := domain.SourceCreateSubject{SourceType: domain.SourceTypeRfxAward, SourceID: uuid.New(), SourceTenantID: uuid.New()}
	decision, err := client.ConfirmCreate(context.Background(), subject)
	if err != nil || decision != domain.SourceCreateUnavailable {
		t.Fatalf("down confirm = %s %v", decision, err)
	}
	revalidated, err := client.Revalidate(context.Background(), domain.SourceSnapshot{SourceType: domain.SourceTypeAwardLink, SourceID: subject.SourceID, SourceTenantID: subject.SourceTenantID})
	if err != nil || revalidated != domain.SourceUnavailable {
		t.Fatalf("down revalidate = %s %v", revalidated, err)
	}
}

func TestRfxSourceAuthorityClient(t *testing.T) {
	var calls int
	var confirmed map[string]string
	subject := domain.SourceCreateSubject{
		SourceType: domain.SourceTypeAwardLink, SourceID: uuid.New(), SourceTenantID: uuid.New(),
		Fingerprint: "fingerprint-v2", ShipperCarrierCompanyID: uuid.New(), RequestingCompanyID: uuid.New(), RequestingUserID: uuid.New(),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("X-Internal-Service-Name") != "" || r.Header.Get("X-Internal-Service-Token") != "" || r.Header.Get("Authorization") != "" {
			t.Errorf("shared internal credentials must not be presented")
		}
		if r.Header.Get(rfxSourceAuthorityHeader) != "revalidation-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path == rfxSourceAuthorityRevalidatePath {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"decision":"MATCH"}`))
			return
		}
		if r.URL.Path != rfxSourceAuthorityConfirmPath || json.Unmarshal(body, &confirmed) != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"decision":"UNAVAILABLE"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"decision":"CONFIRMED"}`))
	}))
	defer server.Close()

	client := NewRfxSourceAuthorityClient(server.URL+"/", "revalidation-token", server.Client())
	decision, err := client.ConfirmCreate(context.Background(), subject)
	if err != nil || decision != domain.SourceCreateConfirmed {
		t.Fatalf("confirm = %s %v", decision, err)
	}
	if confirmed["source_type"] != domain.SourceTypeAwardLink || confirmed["source_version_or_fingerprint"] != subject.Fingerprint || confirmed["requesting_user_id"] != subject.RequestingUserID.String() || confirmed["source_tenant_id"] != subject.SourceTenantID.String() {
		t.Fatalf("confirm payload = %#v", confirmed)
	}
	revalidated, err := client.Revalidate(context.Background(), domain.SourceSnapshot{
		SourceType: domain.SourceTypeRfxAward, SourceID: uuid.New(), SourceTenantID: uuid.New(),
		Fingerprint: "abc", ShipperCarrierCompanyID: uuid.New(),
	})
	if err != nil || revalidated != domain.SourceMatch {
		t.Fatalf("match revalidate = %s %v", revalidated, err)
	}
	shipment, err := client.ConfirmCreate(context.Background(), domain.SourceCreateSubject{SourceType: domain.SourceTypeShipment, SourceID: uuid.New()})
	if err != nil || shipment != domain.SourceCreateUnavailable {
		t.Fatalf("shipment confirm = %s %v", shipment, err)
	}
	if calls != 2 {
		t.Fatalf("shipment source must not call the rfx adapter, calls=%d", calls)
	}

	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"decision":"MAYBE"}`))
	})
	unknownConfirm, err := client.ConfirmCreate(context.Background(), subject)
	if err != nil || unknownConfirm != domain.SourceCreateUnavailable {
		t.Fatalf("unknown confirm decision = %s %v", unknownConfirm, err)
	}
	unknownRevalidate, err := client.Revalidate(context.Background(), domain.SourceSnapshot{
		SourceType: domain.SourceTypeRfxAward, SourceID: uuid.New(),
	})
	if err != nil || unknownRevalidate != domain.SourceUnavailable {
		t.Fatalf("unknown revalidate decision = %s %v", unknownRevalidate, err)
	}
}

func TestRelationshipSourceDispatchKeepsOwnersSeparate(t *testing.T) {
	var shipmentCalls, rfxCalls int
	shipmentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		shipmentCalls++
		if r.URL.Path != "/internal/v1/shipment-source-authority/confirm" {
			t.Errorf("shipment path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"decision":"CONFIRMED"}`))
	}))
	defer shipmentServer.Close()
	rfxServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rfxCalls++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/internal/v1/rfx-source-authority/revalidate" {
			_, _ = w.Write([]byte(`{"decision":"CHANGED"}`))
			return
		}
		if r.URL.Path != "/internal/v1/rfx-source-authority/confirm" {
			t.Errorf("rfx path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"decision":"CONFIRMED"}`))
	}))
	defer rfxServer.Close()

	revalidator, authority := WireRelationshipSources(shipmentServer.URL, rfxServer.URL, "revalidation-token")
	confirmed, err := authority.ConfirmCreate(context.Background(), domain.SourceCreateSubject{SourceType: domain.SourceTypeShipment, SourceID: uuid.New()})
	if err != nil || confirmed != domain.SourceCreateConfirmed || shipmentCalls != 1 || rfxCalls != 0 {
		t.Fatalf("shipment confirm = %s calls shipment=%d rfx=%d err=%v", confirmed, shipmentCalls, rfxCalls, err)
	}
	changed, err := revalidator.Revalidate(context.Background(), domain.SourceSnapshot{SourceType: domain.SourceTypeRfxAward, SourceID: uuid.New()})
	if err != nil || changed != domain.SourceChanged || rfxCalls != 1 || shipmentCalls != 1 {
		t.Fatalf("rfx revalidate = %s calls shipment=%d rfx=%d err=%v", changed, shipmentCalls, rfxCalls, err)
	}
	link, err := authority.ConfirmCreate(context.Background(), domain.SourceCreateSubject{SourceType: domain.SourceTypeAwardLink, SourceID: uuid.New()})
	if err != nil || link != domain.SourceCreateConfirmed || rfxCalls != 2 {
		t.Fatalf("link confirm = %s calls=%d err=%v", link, rfxCalls, err)
	}

	_, closed := WireRelationshipSources(shipmentServer.URL, "", "revalidation-token")
	unavailable, err := closed.ConfirmCreate(context.Background(), domain.SourceCreateSubject{SourceType: domain.SourceTypeRfxAward, SourceID: uuid.New()})
	if err != nil || unavailable != domain.SourceCreateUnavailable {
		t.Fatalf("unconfigured rfx = %s %v", unavailable, err)
	}
	if rfxCalls != 2 {
		t.Fatal("unconfigured rfx must not call the rfx adapter")
	}
}
