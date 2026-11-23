package companycontext

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/freight-platform/api-gateway/internal/platform/httpredirect"
	"github.com/freight-platform/api-gateway/internal/routeauth"
)

func TestCompanyContextRejectsForeignRedirect(t *testing.T) {
	var host string
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		host = req.URL.Host
		header := make(http.Header)
		header.Set("Location", "http://evil.example/companies")
		return &http.Response{StatusCode: http.StatusFound, Header: header, Body: io.NopCloser(strings.NewReader("")), Request: req, Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1}, nil
	})
	client := NewIdentityClient(&http.Client{Transport: transport}, "http://identity.internal")
	_, err := client.ListUserCompanies(context.Background(), routeauth.RequestContext{TenantID: "tenant-secret", UserID: "user-secret", AuthToken: "Bearer secret"}, "user-1")
	if !errors.Is(err, httpredirect.ErrRejected) || host != "identity.internal" || strings.Contains(err.Error(), "tenant-secret") {
		t.Fatalf("host=%s err=%v", host, err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
