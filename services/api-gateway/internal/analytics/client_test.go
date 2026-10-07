package analytics

import (
	"net/http"
	"testing"
	"time"
)

func TestNewClientDeniesRedirectsWithoutMutatingCaller(t *testing.T) {
	caller := &http.Client{Timeout: 3 * time.Second}
	client := NewClient(caller, "http://analytics.test", "token")
	if caller.CheckRedirect != nil {
		t.Fatal("caller HTTP client was mutated")
	}
	if client.httpClient == caller {
		t.Fatal("analytics client must not share the caller pointer")
	}
	if client.httpClient.Timeout != caller.Timeout || client.httpClient.Transport != caller.Transport || client.httpClient.Jar != caller.Jar {
		t.Fatal("analytics client must keep the caller timeout, transport, and cookie jar")
	}
	sameHost, err := http.NewRequest(http.MethodGet, "http://analytics.test/v1/analytics/kpis/OPS_SHIPMENTS_TOTAL", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.httpClient.CheckRedirect(sameHost, []*http.Request{sameHost}); err == nil {
		t.Fatal("same-host redirect must be rejected")
	}
	otherHost, err := http.NewRequest(http.MethodGet, "http://other.test/stolen", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.httpClient.CheckRedirect(otherHost, []*http.Request{sameHost}); err == nil {
		t.Fatal("cross-host redirect must be rejected")
	}
	if http.DefaultClient.CheckRedirect != nil {
		t.Fatal("default client must stay unchanged")
	}
	fromDefault := NewClient(nil, "http://analytics.test", "token")
	if http.DefaultClient.CheckRedirect != nil || fromDefault.httpClient == http.DefaultClient {
		t.Fatal("nil caller must not mutate or reuse http.DefaultClient")
	}
}
