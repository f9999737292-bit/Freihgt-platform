package identityguard

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/freight-platform/api-gateway/internal/platform/httpredirect"
)

func TestVerifierRejectsForeignRedirect(t *testing.T) {
	var host string
	verifier := &HTTPVerifier{
		baseURL: "http://identity.internal",
		client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			host = req.URL.Host
			header := make(http.Header)
			header.Set("Location", "http://evil.example/roles")
			return &http.Response{StatusCode: http.StatusFound, Header: header, Body: io.NopCloser(strings.NewReader("")), Request: req, Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1}, nil
		})},
	}
	_, err := verifier.IsPlatformAdmin(context.Background(), "Bearer secret", "user-1", "tenant-secret")
	if !errors.Is(err, httpredirect.ErrRejected) || host != "identity.internal" || strings.Contains(err.Error(), "tenant-secret") {
		t.Fatalf("host=%s err=%v", host, err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
