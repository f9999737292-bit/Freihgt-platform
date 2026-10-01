package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type HTTPShipmentOwner struct {
	BaseURL string
	Token   string
	Client  *http.Client
}

func (h HTTPShipmentOwner) Owns(ctx context.Context, tenantID, shipmentID uuid.UUID) (bool, error) {
	if h.BaseURL == "" || h.Token == "" {
		return false, nil
	}
	client := h.Client
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.BaseURL+"/internal/v1/shipments/"+shipmentID.String()+"/ownership", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("X-Tenant-ID", tenantID.String())
	req.Header.Set("X-Internal-Service-Token", h.Token)
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound, http.StatusForbidden, http.StatusUnauthorized:
		return false, nil
	default:
		return false, fmt.Errorf("shipment ownership check failed")
	}
}
