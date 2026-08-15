package document

import (
	"bytes"
	"context"
	"encoding/json"
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
	return &Client{httpClient: httpClient, baseURL: strings.TrimRight(baseURL, "/")}
}

type CreatePODUploadRequest struct {
	TenantID       string
	ShipmentID     string
	DriverID       string
	OwnerCompanyID string
	MimeType       string
	FileName       string
	IdempotencyKey string
}

func (c *Client) CreatePODUpload(ctx context.Context, req CreatePODUploadRequest) (json.RawMessage, int, error) {
	body, _ := json.Marshal(map[string]string{
		"shipmentId": req.ShipmentID, "driverId": req.DriverID, "ownerCompanyId": req.OwnerCompanyID,
		"mimeType": req.MimeType, "fileName": req.FileName, "idempotencyKey": req.IdempotencyKey,
	})
	return c.do(ctx, http.MethodPost, "/internal/v1/pod-uploads", req.TenantID, body)
}

func (c *Client) UploadPODContent(ctx context.Context, tenantID, uploadID, token string, content []byte, mimeType string) (int, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/internal/v1/pod-uploads/"+uploadID+"/content", bytes.NewReader(content))
	if err != nil {
		return 0, err
	}
	httpReq.Header.Set("X-Tenant-ID", tenantID)
	httpReq.Header.Set("X-Upload-Token", token)
	if mimeType != "" {
		httpReq.Header.Set("Content-Type", mimeType)
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	return resp.StatusCode, nil
}

func (c *Client) CompletePODUpload(ctx context.Context, tenantID, uploadID, driverID, checksum string) (json.RawMessage, int, error) {
	body, _ := json.Marshal(map[string]string{"driverId": driverID, "checksumSha256": checksum})
	return c.do(ctx, http.MethodPost, "/internal/v1/pod-uploads/"+uploadID+"/complete", tenantID, body)
}

func (c *Client) do(ctx context.Context, method, path, tenantID string, body []byte) (json.RawMessage, int, error) {
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
	req.Header.Set("X-Tenant-ID", tenantID)
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

func WithRequestID(ctx context.Context, requestID string) context.Context {
	_ = sharedmiddleware.RequestIDHeader
	return ctx
}
