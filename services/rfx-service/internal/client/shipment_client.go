package client

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
)

type ShipmentClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewShipmentClient(baseURL string) *ShipmentClient {
	return &ShipmentClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type CreateShipmentFromBidRequest struct {
	ShipmentNumber   string `json:"shipment_number"`
	BidID            string `json:"bid_id"`
	TransportOrderID string `json:"transport_order_id"`
}

type CreateShipmentFromBidResponse struct {
	ID               string `json:"id"`
	ShipmentNumber   string `json:"shipment_number"`
	TransportOrderID string `json:"transport_order_id"`
	Status           string `json:"status"`
}

func (c *ShipmentClient) CreateFromBid(ctx context.Context, tenantID, userID uuid.UUID, req CreateShipmentFromBidRequest) (*CreateShipmentFromBidResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/shipments/from-bid", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Tenant-ID", tenantID.String())
	httpReq.Header.Set("X-User-ID", userID.String())

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("shipment service request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("shipment service unavailable: status=%d body=%s", resp.StatusCode, string(raw))
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("shipment service error: status=%d body=%s", resp.StatusCode, string(raw))
	}

	var out CreateShipmentFromBidResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
