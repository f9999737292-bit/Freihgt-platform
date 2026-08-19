package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type BillingSyncHTTPError struct {
	StatusCode int
	Body       string
}

func (e *BillingSyncHTTPError) Error() string {
	if e == nil {
		return "billing sync-paid failed"
	}
	if e.Body == "" {
		return fmt.Sprintf("billing sync-paid failed: status=%d", e.StatusCode)
	}
	return fmt.Sprintf("billing sync-paid failed: status=%d body=%s", e.StatusCode, e.Body)
}

type BillingRegisterHTTPClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewBillingRegisterHTTPClient(baseURL, token string) *BillingRegisterHTTPClient {
	return &BillingRegisterHTTPClient{
		baseURL: baseURL,
		token:   token,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *BillingRegisterHTTPClient) SyncRegisterPaid(ctx context.Context, tenantID, registerID uuid.UUID) error {
	if c == nil || c.baseURL == "" {
		return fmt.Errorf("billing register service url is not configured")
	}
	body, err := json.Marshal(map[string]string{"tenant_id": tenantID.String()})
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/internal/v1/billing-registers/%s/sync-paid", c.baseURL, registerID.String())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID.String())
	if c.token != "" {
		req.Header.Set("X-Internal-Service-Token", c.token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return &BillingSyncHTTPError{StatusCode: resp.StatusCode, Body: string(raw)}
	}
	return nil
}
