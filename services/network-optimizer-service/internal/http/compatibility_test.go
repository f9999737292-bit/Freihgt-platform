package http_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	httpserver "github.com/freight-platform/network-optimizer-service/internal/http"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/service"
	"github.com/freight-platform/network-optimizer-service/internal/sourceverify"
)

func TestBNO140NoMarketplaceScanOrMatching(t *testing.T) {
	srv := httptest.NewServer(httpserver.NewRouter(nil, service.New(repository.NewMemory(), sourceverify.Unavailable{}), nil))
	defer srv.Close()
	for _, path := range []string{"/v1/network/matches", "/v1/network/next-load", "/v1/network/recommendations", "/v1/network/top-n"} {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusNotFound {
			t.Fatalf("%s status %d", path, res.StatusCode)
		}
	}
	tenant := uuid.New()
	user := uuid.New()
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/network/compatibility/cargo-equipment/evaluate", bytes.NewBufferString(`{"cargo":{"id":"a","weight_kg":1000},"equipment":{"payload_kg":22000},"access":{}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenant.String())
	req.Header.Set("X-User-ID", user.String())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("evaluate status %d", res.StatusCode)
	}
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(res.Body)
	if !strings.Contains(buf.String(), `"status":"COMPATIBLE"`) {
		t.Fatalf("body %s", buf.String())
	}
}
