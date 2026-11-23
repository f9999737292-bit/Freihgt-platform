package driver

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/freight-platform/api-gateway/internal/platform/httpredirect"
	"github.com/freight-platform/api-gateway/internal/tracking"
)

func TestProbeDriverRejectsForeignRedirect(t *testing.T) {
	t.Parallel()
	var forbidden atomic.Int32
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forbidden.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(foreign.Close)

	for _, status := range []int{http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			forbidden.Store(0)
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, foreign.URL+"/steal", status)
			}))
			defer origin.Close()

			caller := &http.Client{}
			client := NewClient(caller, origin.URL)
			_, statusCode, err := client.ArriveStop(context.Background(), RequestContext{
				TenantID:  "tenant-a",
				UserID:    "user-a",
				RequestID: "req-a",
				AuthToken: "Bearer secret-token",
			}, "stop-1", []byte(`{"keep":true}`), "idem-a")
			if !errors.Is(err, httpredirect.ErrRejected) {
				t.Fatalf("status=%d error=%v", statusCode, err)
			}
			if forbidden.Load() != 0 {
				t.Fatalf("forbidden requests=%d", forbidden.Load())
			}
			if caller.CheckRedirect != nil {
				t.Fatal("caller CheckRedirect was set")
			}
			if strings.Contains(err.Error(), "tenant-a") || strings.Contains(err.Error(), "idem-a") || strings.Contains(err.Error(), "secret-token") {
				t.Fatalf("error leaked details: %v", err)
			}
		})
	}
}

func TestProbeDriverSameOriginKeepsIdempotencyAndMethod(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var gotMethod, gotBody, gotTenant, gotIdem, gotAuth string
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasSuffix(r.URL.Path, "/landed") {
					http.Redirect(w, r, "http://"+r.Host+r.URL.Path+"/landed", status)
					return
				}
				gotMethod = r.Method
				raw, _ := io.ReadAll(r.Body)
				gotBody = string(raw)
				gotTenant = r.Header.Get("X-Tenant-ID")
				gotIdem = r.Header.Get("Idempotency-Key")
				gotAuth = r.Header.Get("Authorization")
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			defer origin.Close()

			caller := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) > 0 && via[0] != nil {
					via[0].Header.Set("X-Tenant-ID", "attacker-tenant")
					via[0].Header.Set("Idempotency-Key", "attacker-key")
					via[0].Header.Set("Authorization", "Bearer attacker")
				}
				req.Header.Set("X-Tenant-ID", "attacker-tenant")
				req.Header.Set("Idempotency-Key", "attacker-key")
				req.Header.Set("Authorization", "Bearer attacker")
				req.Host = "attacker.internal"
				return nil
			}}
			client := NewClient(caller, origin.URL)
			raw, statusCode, err := client.ArriveStop(context.Background(), RequestContext{
				TenantID:  "tenant-a",
				UserID:    "user-a",
				RequestID: "req-a",
				AuthToken: "Bearer secret-token",
			}, "stop-1", []byte(`{"keep":true}`), "idem-a")
			if err != nil || statusCode != http.StatusOK {
				t.Fatalf("status=%d body=%s error=%v", statusCode, raw, err)
			}
			if gotTenant != "tenant-a" || gotIdem != "idem-a" || gotAuth != "Bearer secret-token" {
				t.Fatalf("tenant=%q idem=%q auth=%q", gotTenant, gotIdem, gotAuth)
			}
			if status == http.StatusTemporaryRedirect || status == http.StatusPermanentRedirect {
				if gotMethod != http.MethodPost || gotBody != `{"keep":true}` {
					t.Fatalf("method=%s body=%q", gotMethod, gotBody)
				}
			} else if gotMethod != http.MethodGet {
				t.Fatalf("method=%s want GET", gotMethod)
			}
		})
	}
}

func TestProbeSharedDriverAndTrackingClientsStayOnTheirOrigins(t *testing.T) {
	t.Parallel()
	var foreign atomic.Int32
	var shipmentHits atomic.Int32
	var trackingHits atomic.Int32
	var trackingToken, trackingTenant, trackingDriver string

	foreignSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		foreign.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(foreignSrv.Close)

	shipment := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		shipmentHits.Add(1)
		if strings.Contains(r.Header.Get("X-Internal-Service-Token"), "internal-secret") {
			t.Errorf("shipment origin received tracking credential")
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(shipment.Close)

	trackingSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trackingHits.Add(1)
		if r.URL.Path == "/v1/shipments/s1/tracking" {
			http.Redirect(w, r, "http://"+r.Host+"/v1/shipments/s1/tracking-final", http.StatusFound)
			return
		}
		trackingToken = r.Header.Get("X-Internal-Service-Token")
		trackingTenant = r.Header.Get("X-Tenant-ID")
		trackingDriver = r.Header.Get("X-Driver-ID")
		_, _ = w.Write([]byte(`{"shipmentId":"s1","trackingStatus":"IN_TRANSIT","freshness":{"status":"FRESH"},"quality":{"status":"OK"}}`))
	}))
	t.Cleanup(trackingSrv.Close)

	shared := &http.Client{}
	driverClient := NewClient(shared, shipment.URL)
	trackingClient := tracking.NewClient(shared, trackingSrv.URL, "internal-secret")

	if _, _, err := driverClient.GetMe(context.Background(), RequestContext{TenantID: "tenant-a", UserID: "user-a", RequestID: "req-a"}); err != nil {
		t.Fatalf("driver happy path: %v", err)
	}
	summary, err := trackingClient.GetCurrent(context.Background(), "tenant-a", "req-a", "s1")
	if err != nil || summary == nil || summary.ShipmentID != "s1" {
		t.Fatalf("tracking same-origin redirect: summary=%v err=%v", summary, err)
	}
	if trackingTenant != "tenant-a" {
		t.Fatalf("tracking tenant=%q", trackingTenant)
	}
	if _, statusCode, err := trackingClient.IngestDriverLocation(context.Background(), "tenant-a", "req-a", "s1", "driver-a", "vehicle-a", []byte(`{"lat":1}`)); err != nil || statusCode != http.StatusOK {
		t.Fatalf("shared-client ingest status=%d err=%v", statusCode, err)
	}
	if trackingToken != "internal-secret" || trackingDriver != "driver-a" {
		t.Fatalf("token=%q driver=%q", trackingToken, trackingDriver)
	}
	if foreign.Load() != 0 {
		t.Fatalf("forbidden requests=%d", foreign.Load())
	}

	redirectShipment := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, foreignSrv.URL+"/from-driver", http.StatusFound)
	}))
	defer redirectShipment.Close()
	redirectTracking := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, shipment.URL+"/from-tracking", http.StatusTemporaryRedirect)
	}))
	defer redirectTracking.Close()

	sharedDriver := NewClient(shared, redirectShipment.URL)
	sharedTracking := tracking.NewClient(shared, redirectTracking.URL, "internal-secret")
	beforeShipment := shipmentHits.Load()
	if _, _, err := sharedDriver.GetMe(context.Background(), RequestContext{TenantID: "tenant-a", UserID: "user-a"}); !errors.Is(err, httpredirect.ErrRejected) {
		t.Fatalf("driver foreign redirect error=%v", err)
	}
	raw, status, err := sharedTracking.IngestDriverLocation(context.Background(), "tenant-a", "req-a", "s1", "driver-a", "vehicle-a", []byte(`{"lat":1}`))
	if !errors.Is(err, httpredirect.ErrRejected) {
		t.Fatalf("tracking cross-service redirect status=%d body=%s err=%v", status, raw, err)
	}
	if foreign.Load() != 0 {
		t.Fatalf("forbidden requests=%d", foreign.Load())
	}
	if shipmentHits.Load() != beforeShipment {
		t.Fatalf("tracking redirect reached shipment origin, hits %d -> %d", beforeShipment, shipmentHits.Load())
	}
	if shared.CheckRedirect != nil {
		t.Fatal("shared http.Client CheckRedirect was changed")
	}
}
