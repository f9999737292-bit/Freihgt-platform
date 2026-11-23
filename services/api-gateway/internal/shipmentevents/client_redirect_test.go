package shipmentevents

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/freight-platform/api-gateway/internal/platform/httpredirect"
)

func TestShipmentEventsRejectMetadataRedirect(t *testing.T) {
	var host string
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		host = req.URL.Host
		header := make(http.Header)
		header.Set("Location", "http://169.254.169.254/latest/meta-data")
		return &http.Response{StatusCode: http.StatusFound, Header: header, Body: io.NopCloser(strings.NewReader("")), Request: req, Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1}, nil
	})
	client := NewDownstreamClient(&http.Client{Transport: transport}, "", "http://shipment.internal", "", "", 10)
	_, err := client.FetchShipment(context.Background(), RequestContext{TenantID: "tenant-secret", AuthToken: "Bearer secret", RequestID: "req-secret"}, "s1")
	if !errors.Is(err, httpredirect.ErrRejected) || host != "shipment.internal" || strings.Contains(err.Error(), "tenant-secret") {
		t.Fatalf("host=%s err=%v", host, err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
