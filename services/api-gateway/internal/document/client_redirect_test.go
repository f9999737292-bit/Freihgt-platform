package document

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/freight-platform/api-gateway/internal/platform/httpredirect"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestUploadRedirectDoesNotSendBodyOffOrigin(t *testing.T) {
	var hosts []string
	var bodies []string
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		hosts = append(hosts, req.URL.Host)
		raw, _ := io.ReadAll(req.Body)
		bodies = append(bodies, string(raw))
		header := make(http.Header)
		header.Set("Location", "http://169.254.169.254/collect")
		return &http.Response{StatusCode: http.StatusPermanentRedirect, Header: header, Body: io.NopCloser(strings.NewReader("")), Request: req, Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1}, nil
	})
	client := NewClient(&http.Client{Transport: transport}, "http://document.internal")
	_, err := client.UploadPODContent(context.Background(), "tenant-secret", "upload-1", "upload-token-secret", []byte("pod-body-secret"), "image/jpeg")
	if !errors.Is(err, httpredirect.ErrRejected) {
		t.Fatalf("err = %v", err)
	}
	if len(hosts) != 1 || hosts[0] != "document.internal" || !strings.Contains(bodies[0], "pod-body-secret") {
		t.Fatalf("hosts=%v bodies=%v", hosts, bodies)
	}
}

func TestShipperProxyRejectsForeignRedirect(t *testing.T) {
	var host string
	handler := &ShipperHandler{
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			host = req.URL.Host
			header := make(http.Header)
			header.Set("Location", "http://evil.example/docs")
			return &http.Response{StatusCode: http.StatusFound, Header: header, Body: io.NopCloser(strings.NewReader("")), Request: req, Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1}, nil
		})},
		baseURL:     "http://document.internal",
		devTenantID: "tenant-secret",
	}
	shipmentID := uuid.NewString()
	req := httptest.NewRequest(http.MethodGet, "/v1/shipper/documents?shipment_id="+shipmentID, nil)
	route := chi.NewRouter()
	route.Get("/v1/shipper/documents", handler.List)
	rec := httptest.NewRecorder()
	route.ServeHTTP(rec, req)
	if host != "document.internal" {
		t.Fatalf("host = %s status=%d", host, rec.Code)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
