package driver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
}

func NewClient(httpClient *http.Client, baseURL string) *Client {
	return &Client{
		httpClient: httpClient,
		baseURL:    strings.TrimRight(baseURL, "/"),
	}
}

type RequestContext struct {
	TenantID  string
	UserID    string
	RequestID string
	AuthToken string
}

func (c *Client) GetMe(ctx context.Context, reqCtx RequestContext) (json.RawMessage, int, error) {
	return c.doJSON(ctx, reqCtx, http.MethodGet, "/v1/driver/me", nil)
}

func (c *Client) ListShipments(ctx context.Context, reqCtx RequestContext, query string) (json.RawMessage, int, error) {
	path := "/v1/driver/me/shipments"
	if query != "" {
		path += "?" + query
	}
	return c.doJSON(ctx, reqCtx, http.MethodGet, path, nil)
}

func (c *Client) GetShipment(ctx context.Context, reqCtx RequestContext, shipmentID string) (json.RawMessage, int, error) {
	return c.doJSON(ctx, reqCtx, http.MethodGet, "/v1/driver/me/shipments/"+shipmentID, nil)
}

func (c *Client) RecordEvent(ctx context.Context, reqCtx RequestContext, shipmentID string, body []byte) (json.RawMessage, int, error) {
	return c.doJSON(ctx, reqCtx, http.MethodPost, "/v1/driver/me/shipments/"+shipmentID+"/events", body)
}

func (c *Client) ReportException(ctx context.Context, reqCtx RequestContext, shipmentID string, body []byte) (json.RawMessage, int, error) {
	return c.doJSON(ctx, reqCtx, http.MethodPost, "/v1/driver/me/shipments/"+shipmentID+"/exceptions", body)
}

func (c *Client) doJSON(ctx context.Context, reqCtx RequestContext, method, path string, body []byte) (json.RawMessage, int, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Tenant-ID", reqCtx.TenantID)
	if reqCtx.UserID != "" {
		req.Header.Set("X-User-ID", reqCtx.UserID)
	}
	if reqCtx.AuthToken != "" {
		req.Header.Set("Authorization", reqCtx.AuthToken)
	}
	if reqCtx.RequestID != "" {
		req.Header.Set(sharedmiddleware.RequestIDHeader, reqCtx.RequestID)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if len(raw) == 0 {
		return json.RawMessage("{}"), resp.StatusCode, nil
	}
	return json.RawMessage(raw), resp.StatusCode, nil
}

func MapDependencyStatus(status int) error {
	return fmt.Errorf("shipment service status %d", status)
}
