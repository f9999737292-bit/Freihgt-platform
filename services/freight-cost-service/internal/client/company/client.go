package company

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/freight-cost-service/internal/provider"
	apperrors "github.com/freight-platform/freight-cost-service/internal/platform/errors"
	fcmetrics "github.com/freight-platform/freight-cost-service/internal/platform/metrics"
	"github.com/freight-platform/shared-go/internalauth"
)

const batchSize = 500

type Client struct {
	baseURL string
	token   string
	client  *http.Client
	metrics *fcmetrics.Metrics
}

func NewClient(baseURL, token string, metrics *fcmetrics.Metrics) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		client:  &http.Client{Timeout: 10 * time.Second},
		metrics: metrics,
	}
}

type batchRequest struct {
	CompanyIDs []string `json:"company_ids"`
}

type batchItem struct {
	CompanyID string  `json:"company_id"`
	LegalName string  `json:"legal_name"`
	ShortName *string `json:"short_name,omitempty"`
	Status    string  `json:"status"`
}

type batchResponse struct {
	Items []batchItem `json:"items"`
}

func (c *Client) BatchGetCompanyDisplay(
	ctx context.Context,
	tenantID uuid.UUID,
	companyIDs []uuid.UUID,
) (map[uuid.UUID]provider.CompanyDisplay, error) {
	const operation = "batch_company_display"
	result := make(map[uuid.UUID]provider.CompanyDisplay)
	if c == nil || c.baseURL == "" || len(companyIDs) == 0 {
		return result, nil
	}
	for start := 0; start < len(companyIDs); start += batchSize {
		end := start + batchSize
		if end > len(companyIDs) {
			end = len(companyIDs)
		}
		chunk := companyIDs[start:end]
		ids := make([]string, len(chunk))
		for i, id := range chunk {
			ids[i] = id.String()
		}
		payload, err := json.Marshal(batchRequest{CompanyIDs: ids})
		if err != nil {
			return nil, apperrors.Internal("marshal company batch request", err)
		}
		url := fmt.Sprintf("%s/internal/v1/companies/batch-get", c.baseURL)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return nil, apperrors.Internal("create company batch request", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", tenantID.String())
		if c.token != "" {
			req.Header.Set(internalauth.HeaderName, c.token)
		}
		resp, err := c.client.Do(req)
		if err != nil {
			c.observe(operation, "error")
			return nil, apperrors.Unavailable("company service unavailable", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if readErr != nil {
			c.observe(operation, "error")
			return nil, apperrors.Unavailable("company service unavailable", readErr)
		}
		if resp.StatusCode != http.StatusOK {
			c.observe(operation, "error")
			return nil, apperrors.BadGateway("company batch-get failed", fmt.Errorf("status %d", resp.StatusCode))
		}
		var decoded batchResponse
		if err := json.Unmarshal(body, &decoded); err != nil {
			c.observe(operation, "error")
			return nil, apperrors.BadGateway("invalid company batch response", err)
		}
		for _, item := range decoded.Items {
			id, err := uuid.Parse(strings.TrimSpace(item.CompanyID))
			if err != nil || id == uuid.Nil {
				continue
			}
			result[id] = provider.CompanyDisplay{
				CompanyID: id,
				LegalName: item.LegalName,
				ShortName: item.ShortName,
				Status:    item.Status,
			}
		}
	}
	c.observe(operation, "success")
	return result, nil
}

func (c *Client) observe(operation, result string) {
	if c.metrics != nil {
		c.metrics.ObserveSourceRequest("company-service", operation, result)
	}
}

var _ provider.CompanyDisplayReader = (*Client)(nil)
