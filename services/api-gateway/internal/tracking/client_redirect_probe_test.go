package tracking

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
)

func TestProbeTrackingRejectsForeignRedirect(t *testing.T) {
	t.Parallel()
	var forbidden atomic.Int32
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forbidden.Add(1)
		if r.Header.Get("X-Internal-Service-Token") != "" || r.Header.Get("X-Tenant-ID") != "" {
			t.Errorf("forbidden target received tenant or credential headers")
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(foreign.Close)

	statuses := []int{http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect}
	calls := []struct {
		name string
		call func(*Client) error
	}{
		{name: "current", call: func(c *Client) error {
			_, err := c.GetCurrent(context.Background(), "tenant-a", "req-a", "s1")
			return err
		}},
		{name: "eta", call: func(c *Client) error {
			_, err := c.GetETA(context.Background(), "tenant-a", "req-a", "s1", "")
			return err
		}},
		{name: "slots", call: func(c *Client) error {
			_, err := c.GetSlots(context.Background(), "tenant-a", "req-a", "s1", "")
			return err
		}},
		{name: "lookup", call: func(c *Client) error {
			_, err := c.LookupStates(context.Background(), "tenant-a", "req-a", []string{"s1"})
			return err
		}},
		{name: "proxy", call: func(c *Client) error {
			_, _, err := c.ProxyJSON(context.Background(), "tenant-a", "req-a", http.MethodGet, "/v1/tracking", "")
			return err
		}},
		{name: "ingest", call: func(c *Client) error {
			_, _, err := c.IngestDriverLocation(context.Background(), "tenant-a", "req-a", "s1", "driver-a", "vehicle-a", []byte(`{"lat":1}`))
			return err
		}},
	}

	for _, status := range statuses {
		for _, call := range calls {
			t.Run(http.StatusText(status)+"/"+call.name, func(t *testing.T) {
				forbidden.Store(0)
				origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Redirect(w, r, foreign.URL+"/steal", status)
				}))
				defer origin.Close()
				caller := &http.Client{}
				client := NewClient(caller, origin.URL, "internal-secret")
				err := call.call(client)
				if !errors.Is(err, httpredirect.ErrRejected) {
					t.Fatalf("error=%v", err)
				}
				if forbidden.Load() != 0 {
					t.Fatalf("forbidden requests=%d", forbidden.Load())
				}
				if caller.CheckRedirect != nil {
					t.Fatal("caller CheckRedirect was set")
				}
				if strings.Contains(err.Error(), "internal-secret") || strings.Contains(err.Error(), "tenant-a") || strings.Contains(err.Error(), foreign.URL) {
					t.Fatalf("error leaked details: %v", err)
				}
			})
		}
	}
}

func TestProbeTrackingSameOriginKeepsInternalCredential(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var gotToken, gotTenant, gotDriver, gotVehicle, gotMethod, gotBody string
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasSuffix(r.URL.Path, "/landed") {
					http.Redirect(w, r, "http://"+r.Host+r.URL.Path+"/landed", status)
					return
				}
				gotToken = r.Header.Get("X-Internal-Service-Token")
				gotTenant = r.Header.Get("X-Tenant-ID")
				gotDriver = r.Header.Get("X-Driver-ID")
				gotVehicle = r.Header.Get("X-Vehicle-ID")
				gotMethod = r.Method
				raw, _ := io.ReadAll(r.Body)
				gotBody = string(raw)
				w.WriteHeader(http.StatusAccepted)
			}))
			defer origin.Close()

			caller := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) > 0 && via[0] != nil {
					via[0].Header.Set("X-Internal-Service-Token", "attacker-token")
					via[0].Header.Set("X-Tenant-ID", "attacker-tenant")
					via[0].Header.Set("X-Driver-ID", "attacker-driver")
				}
				req.Header.Set("X-Internal-Service-Token", "attacker-token")
				req.Header.Set("X-Tenant-ID", "attacker-tenant")
				req.Header.Set("X-Driver-ID", "attacker-driver")
				req.Host = "attacker.internal"
				return nil
			}}
			client := NewClient(caller, origin.URL, "internal-secret")
			_, code, err := client.IngestDriverLocation(context.Background(), "tenant-a", "req-a", "s1", "driver-a", "vehicle-a", []byte(`{"lat":1}`))
			if err != nil || code != http.StatusAccepted {
				t.Fatalf("status=%d error=%v", code, err)
			}
			if gotToken != "internal-secret" || gotTenant != "tenant-a" || gotDriver != "driver-a" || gotVehicle != "vehicle-a" {
				t.Fatalf("token=%q tenant=%q driver=%q vehicle=%q", gotToken, gotTenant, gotDriver, gotVehicle)
			}
			if gotMethod != http.MethodPost || !strings.Contains(gotBody, `"lat":1`) {
				t.Fatalf("method=%s body=%q", gotMethod, gotBody)
			}
		})
	}
}
