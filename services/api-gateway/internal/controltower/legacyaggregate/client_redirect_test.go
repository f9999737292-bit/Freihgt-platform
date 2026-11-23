package legacyaggregate

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/freight-platform/api-gateway/internal/platform/httpredirect"
)

func TestLegacyAggregateRejectsForeignRedirect(t *testing.T) {
	var host string
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		host = req.URL.Host
		header := make(http.Header)
		header.Set("Location", "http://evil.example/collect")
		return &http.Response{StatusCode: http.StatusFound, Header: header, Body: io.NopCloser(strings.NewReader("")), Request: req, Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1}, nil
	})
	client := NewClient(&http.Client{Transport: transport}, Config{BaseURL: "http://read.internal:8080", Timeout: time.Second}, NewMetrics())
	_, err := client.FetchStatusSummary(context.Background(), "shadow", "tenant-secret", "req-secret")
	if err == nil || !errors.Is(err.Err, httpredirect.ErrRejected) {
		t.Fatalf("err = %#v", err)
	}
	if host != "read.internal:8080" || strings.Contains(err.Error(), "tenant-secret") || strings.Contains(err.Error(), "evil.example") {
		t.Fatalf("host=%s err=%s", host, err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
