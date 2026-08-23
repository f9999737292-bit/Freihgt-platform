package freightcost

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

func NewClient(httpClient *http.Client, baseURL, internalToken string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		httpClient: httpClient,
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      internalToken,
	}
}

type ForwardInput struct {
	Method       string
	InternalPath string
	Query        string
	TenantID     string
	UserID       string
	CompanyID    string
	ActorKind    string
	RequestID    string
}

var (
	downstreamMetricsOnce sync.Once
	downstreamRequestsTotal *prometheus.CounterVec
)

func initDownstreamMetrics() {
	downstreamMetricsOnce.Do(func() {
		downstreamRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "freight_cost_downstream_requests_total",
			Help: "Freight-cost downstream requests by operation and result.",
		}, []string{"operation", "result"})
		prometheus.MustRegister(downstreamRequestsTotal)
	})
}

func recordDownstream(operation, result string) {
	initDownstreamMetrics()
	downstreamRequestsTotal.WithLabelValues(operation, result).Inc()
}

func (c *Client) Forward(ctx context.Context, in ForwardInput) (status int, body []byte, err error) {
	operation := in.Method + " " + in.InternalPath
	endpoint := c.baseURL + in.InternalPath
	if in.Query != "" {
		endpoint += "?" + in.Query
	}

	req, err := http.NewRequestWithContext(ctx, in.Method, endpoint, nil)
	if err != nil {
		recordDownstream(operation, "error")
		return 0, nil, err
	}

	req.Header.Set("Accept", "application/json")
	if in.RequestID != "" {
		req.Header.Set(sharedmiddleware.RequestIDHeader, in.RequestID)
	}
	req.Header.Set("X-Tenant-ID", in.TenantID)
	req.Header.Set("X-User-ID", in.UserID)
	req.Header.Set("X-Company-ID", in.CompanyID)
	req.Header.Set("X-Actor-Kind", in.ActorKind)
	if c.token != "" {
		req.Header.Set("X-Internal-Service-Token", c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		recordDownstream(operation, "transport_error")
		return 0, nil, err
	}
	defer resp.Body.Close()

	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if readErr != nil {
		recordDownstream(operation, "read_error")
		return resp.StatusCode, nil, readErr
	}

	result := fmt.Sprintf("%d", resp.StatusCode)
	if resp.StatusCode == http.StatusUnauthorized && looksLikeInternalAuthFailure(raw) {
		recordDownstream(operation, "internal_auth")
	} else if resp.StatusCode >= 500 {
		recordDownstream(operation, "server_error")
	} else {
		recordDownstream(operation, result)
	}
	return resp.StatusCode, raw, nil
}

func looksLikeInternalAuthFailure(body []byte) bool {
	var payload struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(payload.Error.Message), "internal service authentication")
}
