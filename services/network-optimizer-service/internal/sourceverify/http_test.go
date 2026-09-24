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
	"github.com/freight-platform/shared-go/integrationauth"
	"github.com/freight-platform/shared-go/internalauth"
	"github.com/freight-platform/shared-go/lowcode"
)

const testServiceToken = "test-internal-token"

func TestBNO23ServiceAuthRequired(t *testing.T) {
	called := false
	downstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
		t.Fatal("unauthenticated verifier must not call downstream")
	}))
	defer downstream.Close()
	owned, err := NewHTTP(downstream.URL, downstream.URL, "").Owns(context.Background(), uuid.New(), domain.SourceTransportOrder, uuid.New())
	if !errors.Is(err, ErrUnavailable) || owned || called {
		t.Fatalf("BNO23 owned=%v err=%v called=%v", owned, err, called)
	}
}

func TestBNO24InvalidServiceCredentialDenied(t *testing.T) {
	tenant := uuid.New()
	sourceID := uuid.New()
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(internalauth.HeaderName) != testServiceToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer downstream.Close()
	owned, err := NewHTTP(downstream.URL, "", "wrong-token").Owns(context.Background(), tenant, domain.SourceTransportOrder, sourceID)
	if !errors.Is(err, ErrUnavailable) || owned {
		t.Fatalf("BNO24 owned=%v err=%v", owned, err)
	}
}

func TestBNO25AuthenticatedOtherTenantDenied(t *testing.T) {
	tenant := uuid.New()
	sourceID := uuid.New()
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(internalauth.HeaderName) != testServiceToken {
			t.Errorf("missing service token")
		}
		http.NotFound(w, r)
	}))
	defer downstream.Close()
	owned, err := NewHTTP(downstream.URL, "", testServiceToken).Owns(context.Background(), tenant, domain.SourceTransportOrder, sourceID)
	if err != nil || owned {
		t.Fatalf("BNO25 owned=%v err=%v", owned, err)
	}
}

func TestBNO26ClientHeadersAreNotServiceIdentity(t *testing.T) {
	tenant := uuid.New()
	sourceID := uuid.New()
	var sawTenant, sawToken, sawPath string
	var forged []string
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawTenant = r.Header.Get(lowcode.HeaderTenantID)
		sawToken = r.Header.Get(internalauth.HeaderName)
		sawPath = r.URL.Path
		for _, header := range []string{
			integrationauth.HeaderIntegrationPrincipalID,
			integrationauth.HeaderIntegrationScopes,
			integrationauth.HeaderAuthScheme,
			integrationauth.HeaderActorKind,
			"Authorization",
			lowcode.HeaderUserID,
		} {
			if r.Header.Get(header) != "" {
				forged = append(forged, header)
			}
		}
		_, _ = io.WriteString(w, `{"id":"`+sourceID.String()+`","tenant_id":"`+tenant.String()+`"}`)
	}))
	defer downstream.Close()

	owned, err := NewHTTP(downstream.URL, downstream.URL, testServiceToken).Owns(context.Background(), tenant, domain.SourceTransportOrder, sourceID)
	if err != nil || !owned {
		t.Fatalf("BNO26 owned=%v err=%v", owned, err)
	}
	if sawTenant != tenant.String() || sawToken != testServiceToken || len(forged) != 0 {
		t.Fatalf("BNO26 tenant=%s token=%s forged=%v", sawTenant, sawToken, forged)
	}
	if sawPath != "/internal/v1/transport-orders/"+sourceID.String()+"/ownership" {
		t.Fatalf("BNO26 path=%s", sawPath)
	}
	shipmentOwned, err := NewHTTP("", downstream.URL, testServiceToken).Owns(context.Background(), tenant, domain.SourceShipment, sourceID)
	if err != nil || !shipmentOwned || sawPath != "/internal/v1/shipments/"+sourceID.String()+"/ownership" {
		t.Fatalf("shipment owned=%v err=%v path=%s", shipmentOwned, err, sawPath)
	}
}

func TestBNO27ValidServiceAuthOwnTenant(t *testing.T) {
	tenant := uuid.New()
	other := uuid.New()
	sourceID := uuid.New()
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(internalauth.HeaderName) != testServiceToken || r.Header.Get(lowcode.HeaderTenantID) != tenant.String() {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, `{"id":"`+sourceID.String()+`","tenant_id":"`+tenant.String()+`"}`)
	}))
	defer downstream.Close()
	owned, err := NewHTTP(downstream.URL, "", testServiceToken).Owns(context.Background(), tenant, domain.SourceTransportOrder, sourceID)
	if err != nil || !owned {
		t.Fatalf("BNO27 owned=%v err=%v", owned, err)
	}

	mismatch := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"id":"`+sourceID.String()+`","tenant_id":"`+other.String()+`"}`)
	}))
	defer mismatch.Close()
	owned, err = NewHTTP(mismatch.URL, "", testServiceToken).Owns(context.Background(), tenant, domain.SourceTransportOrder, sourceID)
	if err != nil || owned {
		t.Fatalf("tenant mismatch owned=%v err=%v", owned, err)
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
			return
		}
		http.Redirect(w, r, "/stolen", http.StatusFound)
	}))
	defer downstream.Close()
	owned, err := NewHTTP(downstream.URL, "", testServiceToken).Owns(context.Background(), tenant, domain.SourceTransportOrder, sourceID)
	if !errors.Is(err, ErrUnavailable) || owned || redirected {
		t.Fatalf("owned=%v err=%v redirected=%v", owned, err, redirected)
	}
}

func TestHTTPVerifierEmptyURLFailsClosed(t *testing.T) {
	owned, err := NewHTTP("", "", testServiceToken).Owns(context.Background(), uuid.New(), domain.SourceTransportOrder, uuid.New())
	if !errors.Is(err, ErrUnavailable) || owned {
		t.Fatalf("owned=%v err=%v", owned, err)
	}
}
