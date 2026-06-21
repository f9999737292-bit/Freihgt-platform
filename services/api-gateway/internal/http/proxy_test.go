package http_test

import (
	"net/url"
	"testing"

	gatewayhttp "github.com/freight-platform/api-gateway/internal/http"
)

func TestRewritePath(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantOK  bool
	}{
		{"/api/v1/companies", "/v1/companies", true},
		{"/api/v1/auth/login", "/v1/auth/login", true},
		{"/api", "/", true},
		{"/v1/companies", "", false},
		{"/health", "", false},
	}

	for _, tt := range tests {
		got, ok := gatewayhttp.RewritePath(tt.in)
		if ok != tt.wantOK {
			t.Fatalf("RewritePath(%q) ok=%v want %v", tt.in, ok, tt.wantOK)
		}
		if got != tt.want {
			t.Fatalf("RewritePath(%q)=%q want %q", tt.in, got, tt.want)
		}
	}
}

func TestMatchRouteLongestPrefix(t *testing.T) {
	routes := []gatewayhttp.Route{
		{Prefix: "/api/v1/auth", Service: "identity-service"},
		{Prefix: "/api/v1/users", Service: "identity-service"},
		{Prefix: "/api/v1/companies", Service: "company-service"},
	}

	if route := gatewayhttp.MatchRoute("/api/v1/auth/login", routes); route == nil || route.Service != "identity-service" {
		t.Fatalf("expected identity auth route")
	}
	if route := gatewayhttp.MatchRoute("/api/v1/companies/123", routes); route == nil || route.Service != "company-service" {
		t.Fatalf("expected company route")
	}
	if route := gatewayhttp.MatchRoute("/api/v1/unknown", routes); route != nil {
		t.Fatalf("expected no route match")
	}
}

func TestRouteTarget(t *testing.T) {
	target, err := parseURL("http://localhost:8082")
	if err != nil {
		t.Fatal(err)
	}

	route := gatewayhttp.Route{
		Prefix:  "/api/v1/companies",
		Service: "company-service",
		Target:  target,
	}

	got := gatewayhttp.RouteTarget(route)
	want := "http://localhost:8082/v1/companies"
	if got != want {
		t.Fatalf("RouteTarget()=%q want %q", got, want)
	}
}

func parseURL(raw string) (*url.URL, error) {
	return url.Parse(raw)
}
