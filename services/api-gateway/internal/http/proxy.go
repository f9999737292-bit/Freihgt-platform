package http

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/freight-platform/api-gateway/internal/config"
	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/platform/respond"
	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
)

type Route struct {
	Prefix  string
	Service string
	Target  *url.URL
}

type ProxyHandler struct {
	routes  []Route
	timeout time.Duration
}

func NewProxyHandler(cfg config.Config) (*ProxyHandler, error) {
	routeDefs := []struct {
		prefix  string
		service string
		baseURL string
	}{
		{"/api/v1/auth", "identity-service", cfg.Services.Identity},
		{"/api/v1/users", "identity-service", cfg.Services.Identity},
		{"/api/v1/roles", "identity-service", cfg.Services.Identity},
		{"/api/v1/companies", "company-service", cfg.Services.Company},
		{"/api/v1/locations", "transport-order-service", cfg.Services.TransportOrder},
		{"/api/v1/cargoes", "transport-order-service", cfg.Services.TransportOrder},
		{"/api/v1/transport-orders", "transport-order-service", cfg.Services.TransportOrder},
		{"/api/v1/rfx-events", "rfx-service", cfg.Services.RFX},
		{"/api/v1/rfx-lots", "rfx-service", cfg.Services.RFX},
		{"/api/v1/rfx-responses", "rfx-service", cfg.Services.RFX},
		{"/api/v1/freight-requests", "rfx-service", cfg.Services.RFX},
		{"/api/v1/bids", "rfx-service", cfg.Services.RFX},
		{"/api/v1/shipments", "shipment-service", cfg.Services.Shipment},
		{"/api/v1/drivers", "shipment-service", cfg.Services.Shipment},
		{"/api/v1/vehicles", "shipment-service", cfg.Services.Shipment},
		{"/api/v1/documents", "document-service", cfg.Services.Document},
		{"/api/v1/signing-sessions", "document-service", cfg.Services.Document},
		{"/api/v1/billing-registers", "billing-register-service", cfg.Services.BillingRegister},
	}

	routes := make([]Route, 0, len(routeDefs))
	for _, def := range routeDefs {
		target, err := url.Parse(strings.TrimRight(def.baseURL, "/"))
		if err != nil {
			return nil, fmt.Errorf("parse service url for %s: %w", def.service, err)
		}
		routes = append(routes, Route{
			Prefix:  def.prefix,
			Service: def.service,
			Target:  target,
		})
	}

	return &ProxyHandler{
		routes:  routes,
		timeout: time.Duration(cfg.ProxyTimeoutSeconds) * time.Second,
	}, nil
}

func (p *ProxyHandler) Routes() []Route {
	return append([]Route(nil), p.routes...)
}

func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	route := MatchRoute(r.URL.Path, p.routes)
	if route == nil {
		respond.Error(w, apperrors.RouteNotFound("no route found for path"))
		return
	}

	rewrittenPath, ok := RewritePath(r.URL.Path)
	if !ok {
		respond.Error(w, apperrors.RouteNotFound("no route found for path"))
		return
	}

	ctx := sharedmiddleware.WithTargetService(r.Context(), route.Service)
	r = r.WithContext(ctx)

	proxy := httputil.NewSingleHostReverseProxy(route.Target)
	proxy.Transport = &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ResponseHeaderTimeout: p.timeout,
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		respond.Error(w, apperrors.ServiceUnavailable("target service is unavailable", route.Service))
	}

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.URL.Path = rewrittenPath
		req.URL.RawPath = rewrittenPath
		req.Host = route.Target.Host
		if requestID := sharedmiddleware.RequestIDFromContext(r.Context()); requestID != "" {
			req.Header.Set(sharedmiddleware.RequestIDHeader, requestID)
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), p.timeout)
	defer cancel()
	r = r.WithContext(ctx)

	proxy.ServeHTTP(w, r)
}

func MatchRoute(path string, routes []Route) *Route {
	var matched *Route
	bestLen := 0
	for i := range routes {
		if strings.HasPrefix(path, routes[i].Prefix) {
			if len(routes[i].Prefix) > bestLen {
				matched = &routes[i]
				bestLen = len(routes[i].Prefix)
			}
		}
	}
	return matched
}

func RewritePath(path string) (string, bool) {
	if !strings.HasPrefix(path, "/api") {
		return "", false
	}
	rewritten := strings.TrimPrefix(path, "/api")
	if rewritten == "" {
		rewritten = "/"
	}
	return rewritten, true
}

func RouteTarget(route Route) string {
	rewritten, ok := RewritePath(route.Prefix)
	if !ok {
		return route.Target.String()
	}
	return strings.TrimRight(route.Target.String(), "/") + rewritten
}

type downstreamService struct {
	name string
	url  string
}

func downstreamServices(cfg config.Config) []downstreamService {
	return []downstreamService{
		{name: "identity-service", url: cfg.Services.Identity},
		{name: "company-service", url: cfg.Services.Company},
		{name: "transport-order-service", url: cfg.Services.TransportOrder},
		{name: "rfx-service", url: cfg.Services.RFX},
		{name: "shipment-service", url: cfg.Services.Shipment},
		{name: "document-service", url: cfg.Services.Document},
		{name: "billing-register-service", url: cfg.Services.BillingRegister},
	}
}

func CheckServiceHealth(ctx context.Context, client *http.Client, baseURL string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/ready", nil)
	if err != nil {
		return "down"
	}

	resp, err := client.Do(req)
	if err != nil {
		return "down"
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "down"
	}
	return "ok"
}

func ReadyStatus(ctx context.Context, cfg config.Config) (status string, httpStatus int, services map[string]string) {
	services = make(map[string]string, 7)
	client := &http.Client{Timeout: time.Duration(cfg.ReadyCheckTimeoutMS) * time.Millisecond}

	allOK := true
	for _, svc := range downstreamServices(cfg) {
		checkCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.ReadyCheckTimeoutMS)*time.Millisecond)
		state := CheckServiceHealth(checkCtx, client, svc.url)
		cancel()
		services[svc.name] = state
		if state != "ok" {
			allOK = false
		}
	}

	if allOK {
		return "ready", http.StatusOK, services
	}
	return "not_ready", http.StatusServiceUnavailable, services
}
