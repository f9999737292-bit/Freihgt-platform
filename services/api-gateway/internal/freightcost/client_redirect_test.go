package freightcost

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/freight-platform/api-gateway/internal/platform/httpredirect"
)

func TestForwardRejectsMetadataRedirect(t *testing.T) {
	var host string
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		host = req.URL.Host
		header := make(http.Header)
		header.Set("Location", "http://169.254.169.254/latest/meta-data")
		return &http.Response{StatusCode: http.StatusFound, Header: header, Body: io.NopCloser(strings.NewReader("")), Request: req, Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1}, nil
	})
	client := NewClient(&http.Client{Transport: transport}, "http://cost.internal", "internal-secret")
	_, _, err := client.Forward(context.Background(), ForwardInput{Method: http.MethodGet, InternalPath: "/internal/v1/costs", TenantID: "tenant-secret", UserID: "user-secret", RequestID: "req-secret"})
	if !errors.Is(err, httpredirect.ErrRejected) || host != "cost.internal" || strings.Contains(err.Error(), "tenant-secret") {
		t.Fatalf("host=%s err=%v", host, err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
