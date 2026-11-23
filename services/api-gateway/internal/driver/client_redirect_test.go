package driver

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/freight-platform/api-gateway/internal/platform/httpredirect"
	"github.com/freight-platform/api-gateway/internal/tracking"
)

type countTransport struct {
	mu       sync.Mutex
	hosts    []string
	location string
	status   int
	bodies   []string
}

func (t *countTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
	}
	t.mu.Lock()
	t.hosts = append(t.hosts, req.URL.Host)
	t.bodies = append(t.bodies, string(body))
	t.mu.Unlock()
	header := make(http.Header)
	if t.location != "" {
		header.Set("Location", t.location)
	}
	return &http.Response{
		StatusCode: t.status, Header: header, Body: io.NopCloser(strings.NewReader("")),
		Request: req, Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
	}, nil
}

func TestDriverRejectsForeignRedirect(t *testing.T) {
	for _, location := range []string{"http://evil.example/collect", "http://169.254.169.254/latest/meta-data"} {
		transport := &countTransport{status: http.StatusFound, location: location}
		client := NewClient(&http.Client{Transport: transport}, "http://shipment.internal:8080")
		_, _, err := client.GetMe(context.Background(), RequestContext{TenantID: "tenant-secret", UserID: "user-secret", RequestID: "req-secret", AuthToken: "Bearer secret"})
		if !errors.Is(err, httpredirect.ErrRejected) {
			t.Fatalf("err = %v", err)
		}
		if len(transport.hosts) != 1 || transport.hosts[0] != "shipment.internal:8080" {
			t.Fatalf("hosts = %v", transport.hosts)
		}
		if strings.Contains(err.Error(), "tenant-secret") || strings.Contains(err.Error(), location) {
			t.Fatalf("leak: %s", err)
		}
	}
}

func TestDriverPost307DoesNotLeaveOrigin(t *testing.T) {
	transport := &countTransport{status: http.StatusTemporaryRedirect, location: "http://169.254.169.254/collect"}
	client := NewClient(&http.Client{Transport: transport}, "http://shipment.internal:8080")
	_, _, err := client.SubmitTaskResponse(context.Background(), RequestContext{TenantID: "tenant-secret"}, "task-1", []byte(`{"note":"body-secret"}`), "idem-secret")
	if !errors.Is(err, httpredirect.ErrRejected) {
		t.Fatalf("err = %v", err)
	}
	if len(transport.hosts) != 1 || !strings.Contains(transport.bodies[0], "body-secret") {
		t.Fatalf("hosts=%v bodies=%v", transport.hosts, transport.bodies)
	}
}

func TestSharedDriverAndTrackingClientsDoNotCrossOrigins(t *testing.T) {
	shared := &http.Client{}
	driverTransport := &countTransport{status: http.StatusFound, location: "http://tracking.internal/stolen"}
	trackingTransport := &countTransport{status: http.StatusTemporaryRedirect, location: "http://shipment.internal:8080/stolen"}
	shared.Transport = driverTransport
	driverClient := NewClient(shared, "http://shipment.internal:8080")
	if _, _, err := driverClient.GetMe(context.Background(), RequestContext{TenantID: "tenant-a"}); !errors.Is(err, httpredirect.ErrRejected) {
		t.Fatal(err)
	}
	shared.Transport = trackingTransport
	trackingClient := tracking.NewClient(shared, "http://tracking.internal", "internal-secret")
	if _, err := trackingClient.GetCurrent(context.Background(), "tenant-a", "req-a", "s1"); !errors.Is(err, httpredirect.ErrRejected) {
		t.Fatal(err)
	}
	if shared.CheckRedirect != nil {
		t.Fatal("shared client policy was replaced")
	}
	if len(driverTransport.hosts) != 1 || driverTransport.hosts[0] != "shipment.internal:8080" {
		t.Fatalf("driver hosts = %v", driverTransport.hosts)
	}
	if len(trackingTransport.hosts) != 1 || trackingTransport.hosts[0] != "tracking.internal" {
		t.Fatalf("tracking hosts = %v", trackingTransport.hosts)
	}
}
