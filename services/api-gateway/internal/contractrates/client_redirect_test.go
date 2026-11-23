package contractrates

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/freight-platform/api-gateway/internal/platform/httpredirect"
)

func TestForward307DoesNotSendBodyOffOrigin(t *testing.T) {
	var host, body string
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		host = req.URL.Host
		raw, _ := io.ReadAll(req.Body)
		body = string(raw)
		header := make(http.Header)
		header.Set("Location", "http://169.254.169.254/collect")
		return &http.Response{StatusCode: http.StatusTemporaryRedirect, Header: header, Body: io.NopCloser(strings.NewReader("")), Request: req, Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1}, nil
	})
	client := NewClient(&http.Client{Transport: transport}, "http://rates.internal", "internal-secret")
	_, _, err := client.Forward(context.Background(), ForwardInput{
		Method: http.MethodPost, InternalPath: "/internal/v1/rates", Body: []byte(`{"amount":"body-secret"}`),
		TenantID: "tenant-secret", UserID: "user-secret", RequestID: "req-secret",
	})
	if !errors.Is(err, httpredirect.ErrRejected) || host != "rates.internal" || !strings.Contains(body, "body-secret") {
		t.Fatalf("host=%s body=%s err=%v", host, body, err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
