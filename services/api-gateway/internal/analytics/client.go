package analytics

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const gatewayCaller = "api-gateway"

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewClient(httpClient *http.Client, baseURL, token string) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	dedicated := *httpClient
	dedicated.CheckRedirect = refuseAnalyticsRedirect
	return &Client{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:      token,
		httpClient: &dedicated,
	}
}

func refuseAnalyticsRedirect(*http.Request, []*http.Request) error {
	return errors.New("analytics service redirect is not followed")
}

func (c *Client) GetKPI(ctx context.Context, kpiID, tenantID, rawQuery string) (int, []byte, error) {
	if c == nil || c.baseURL == "" || strings.TrimSpace(c.token) == "" || strings.TrimSpace(tenantID) == "" {
		return 0, nil, errUnavailable
	}
	endpoint := c.baseURL + "/v1/analytics/kpis/" + url.PathEscape(kpiID)
	if rawQuery != "" {
		endpoint += "?" + rawQuery
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("X-Internal-Service-Token", c.token)
	req.Header.Set("X-Internal-Service-Name", gatewayCaller)
	req.Header.Set("X-Tenant-ID", tenantID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, body, nil
}

type unavailableError struct{}

func (unavailableError) Error() string { return "analytics dependency unavailable" }

var errUnavailable = unavailableError{}
