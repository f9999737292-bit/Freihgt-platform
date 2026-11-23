package controltower

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/freight-platform/api-gateway/internal/platform/httpredirect"
)

type oneHop struct {
	host string
}

func (t *oneHop) RoundTrip(req *http.Request) (*http.Response, error) {
	t.host = req.URL.Host
	header := make(http.Header)
	header.Set("Location", "http://169.254.169.254/latest/meta-data")
	return &http.Response{
		StatusCode: http.StatusFound, Header: header, Body: io.NopCloser(strings.NewReader("")),
		Request: req, Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
	}, nil
}

func TestDownstreamClientRejectsMetadataRedirect(t *testing.T) {
	transport := &oneHop{}
	client := NewDownstreamClient(&http.Client{Transport: transport}, "http://identity.internal", "", "", "", "", 10)
	_, err := client.FetchUserRoles(context.Background(), RequestContext{TenantID: "tenant-secret", UserID: "user-secret", RequestID: "req-secret", AuthToken: "Bearer secret"})
	if !errors.Is(err, httpredirect.ErrRejected) {
		t.Fatalf("err = %v", err)
	}
	if transport.host != "identity.internal" {
		t.Fatalf("host = %s", transport.host)
	}
	if strings.Contains(err.Error(), "tenant-secret") {
		t.Fatalf("leak: %s", err)
	}
}
