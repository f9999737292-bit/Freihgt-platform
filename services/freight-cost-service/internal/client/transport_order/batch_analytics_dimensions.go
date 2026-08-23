package transport_order

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/freight-cost-service/internal/provider"
	apperrors "github.com/freight-platform/freight-cost-service/internal/platform/errors"
	"github.com/freight-platform/shared-go/internalauth"
)

const analyticsDimensionBatchSize = 500

type batchAnalyticsDimensionsRequest struct {
	TransportOrderIDs []string `json:"transport_order_ids"`
}

type batchAnalyticsDimensionItem struct {
	TransportOrderID   string  `json:"transport_order_id"`
	OriginCountry      string  `json:"origin_country"`
	OriginCity         *string `json:"origin_city,omitempty"`
	DestinationCountry string  `json:"destination_country"`
	DestinationCity    *string `json:"destination_city,omitempty"`
	TransportMode      string  `json:"transport_mode"`
	EquipmentType      *string `json:"equipment_type,omitempty"`
}

type batchAnalyticsDimensionsResponse struct {
	Items []batchAnalyticsDimensionItem `json:"items"`
}

func (c *Client) BatchGetAnalyticsDimensions(
	ctx context.Context,
	tenantID uuid.UUID,
	transportOrderIDs []uuid.UUID,
) (map[uuid.UUID]provider.TransportOrderAnalyticsDimension, error) {
	const operation = "batch_analytics_dimensions"
	result := make(map[uuid.UUID]provider.TransportOrderAnalyticsDimension, len(transportOrderIDs))
	if c == nil || c.baseURL == "" || len(transportOrderIDs) == 0 {
		return result, nil
	}
	for start := 0; start < len(transportOrderIDs); start += analyticsDimensionBatchSize {
		end := start + analyticsDimensionBatchSize
		if end > len(transportOrderIDs) {
			end = len(transportOrderIDs)
		}
		chunk := transportOrderIDs[start:end]
		ids := make([]string, len(chunk))
		for i, id := range chunk {
			ids[i] = id.String()
		}
		payload, err := json.Marshal(batchAnalyticsDimensionsRequest{TransportOrderIDs: ids})
		if err != nil {
			return nil, apperrors.Internal("marshal batch analytics dimensions request", err)
		}
		url := fmt.Sprintf("%s/internal/v1/transport-orders/batch-analytics-dimensions", c.baseURL)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return nil, apperrors.Internal("create transport order batch request", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", tenantID.String())
		if c.token != "" {
			req.Header.Set(internalauth.HeaderName, c.token)
		}
		resp, err := c.client.Do(req)
		if err != nil {
			c.observe(operation, "error", string(apperrors.CodeUnavailable))
			return nil, apperrors.Unavailable("transport order service unavailable", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if readErr != nil {
			c.observe(operation, "error", string(apperrors.CodeUnavailable))
			return nil, apperrors.Unavailable("transport order service unavailable", readErr)
		}
		if resp.StatusCode != http.StatusOK {
			c.observe(operation, "error", string(apperrors.CodeBadGateway))
			return nil, apperrors.BadGateway("transport order batch analytics dimensions failed", fmt.Errorf("status %d", resp.StatusCode))
		}
		var decoded batchAnalyticsDimensionsResponse
		if err := json.Unmarshal(body, &decoded); err != nil {
			c.observe(operation, "error", string(apperrors.CodeBadGateway))
			return nil, apperrors.BadGateway("invalid transport order batch response", err)
		}
		for _, item := range decoded.Items {
			id, err := uuid.Parse(strings.TrimSpace(item.TransportOrderID))
			if err != nil || id == uuid.Nil {
				continue
			}
			result[id] = provider.TransportOrderAnalyticsDimension{
				TransportOrderID:   id,
				OriginCountry:      item.OriginCountry,
				OriginCity:         item.OriginCity,
				DestinationCountry: item.DestinationCountry,
				DestinationCity:    item.DestinationCity,
				TransportMode:      item.TransportMode,
				EquipmentType:      item.EquipmentType,
			}
		}
	}
	c.observe(operation, "success", "")
	return result, nil
}

var _ provider.TransportDimensionReader = (*Client)(nil)
