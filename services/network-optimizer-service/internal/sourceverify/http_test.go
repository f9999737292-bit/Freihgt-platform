package sourceverify

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	"github.com/freight-platform/shared-go/lowcode"
)

func TestHTTPVerifierUsesActorTenantOnly(t *testing.T) {
	tenant := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	other := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	sourceID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	var sawTenant, sawPath string
	var forbidden []string
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawTenant = r.Header.Get(lowcode.HeaderTenantID)
		sawPath = r.URL.Path
		for _, header := range []string{"Authorization", lowcode.HeaderUserID, "X-User-Email", "X-Platform-Admin", "X-Internal-Service-Token", "X-Company-ID"} {
			if r.Header.Get(header) != "" {
				forbidden = append(forbidden, header)
			}
		}
		if r.URL.Query().Get("mode") == "mismatch" {
			_, _ = io.WriteString(w, `{"tenant_id":"`+other.String()+`"}`)
			return
		}
		_, _ = io.WriteString(w, `{"tenant_id":"`+tenant.String()+`","shipper_name":"must-not-be-stored"}`)
	}))
	defer downstream.Close()

	verifier := NewHTTP(downstream.URL, downstream.URL)
	owned, err := verifier.Owns(context.Background(), tenant, domain.SourceTransportOrder, sourceID)
	if err != nil || !owned {
		t.Fatalf("owned=%v err=%v", owned, err)
	}
	if sawTenant != tenant.String() || sawPath != "/v1/transport-orders/"+sourceID.String() || len(forbidden) != 0 {
		t.Fatalf("tenant=%s path=%s forbidden=%v", sawTenant, sawPath, forbidden)
	}

	shipmentOwned, err := verifier.Owns(context.Background(), tenant, domain.SourceShipment, sourceID)
	if err != nil || !shipmentOwned || sawPath != "/v1/shipments/"+sourceID.String() {
		t.Fatalf("shipment owned=%v err=%v path=%s", shipmentOwned, err, sawPath)
	}

	mismatch := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(lowcode.HeaderTenantID) != tenant.String() {
			t.Errorf("mismatch probe used header %s", r.Header.Get(lowcode.HeaderTenantID))
		}
		_, _ = io.WriteString(w, `{"tenant_id":"`+other.String()+`"}`)
	}))
	defer mismatch.Close()
	owned, err = NewHTTP(mismatch.URL, "").Owns(context.Background(), tenant, domain.SourceTransportOrder, sourceID)
	if err != nil || owned {
		t.Fatalf("mismatched downstream tenant owned=%v err=%v", owned, err)
	}

	missing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"id":"`+sourceID.String()+`"}`)
	}))
	defer missing.Close()
	owned, err = NewHTTP(missing.URL, "").Owns(context.Background(), tenant, domain.SourceTransportOrder, sourceID)
	if err != nil || owned {
		t.Fatalf("missing tenant_id owned=%v err=%v", owned, err)
	}

	notFound := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer notFound.Close()
	owned, err = NewHTTP(notFound.URL, "").Owns(context.Background(), tenant, domain.SourceTransportOrder, sourceID)
	if err != nil || owned {
		t.Fatalf("not found owned=%v err=%v", owned, err)
	}
}

func TestHTTPVerifierDoesNotFollowRedirect(t *testing.T) {
	tenant := uuid.New()
	sourceID := uuid.New()
	var redirected bool
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/stolen") {
			redirected = true
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"tenant_id":"`+tenant.String()+`"}`)
			return
		}
		http.Redirect(w, r, "/stolen", http.StatusFound)
	}))
	defer downstream.Close()

	owned, err := NewHTTP(downstream.URL, "").Owns(context.Background(), tenant, domain.SourceTransportOrder, sourceID)
	if !errors.Is(err, ErrUnavailable) || owned || redirected {
		t.Fatalf("owned=%v err=%v redirected=%v", owned, err, redirected)
	}
}

func TestHTTPVerifierEmptyURLFailsClosed(t *testing.T) {
	owned, err := NewHTTP("", "").Owns(context.Background(), uuid.New(), domain.SourceTransportOrder, uuid.New())
	if !errors.Is(err, ErrUnavailable) || owned {
		t.Fatalf("owned=%v err=%v", owned, err)
	}
}
