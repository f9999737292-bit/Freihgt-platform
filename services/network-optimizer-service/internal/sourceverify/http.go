package sourceverify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	"github.com/freight-platform/shared-go/lowcode"
)

type HTTPVerifier struct {
	client            *http.Client
	transportOrderURL string
	shipmentURL       string
}

func NewHTTP(transportOrderURL, shipmentURL string) *HTTPVerifier {
	return &HTTPVerifier{
		client:            &http.Client{Timeout: 5 * time.Second},
		transportOrderURL: strings.TrimRight(transportOrderURL, "/"),
		shipmentURL:       strings.TrimRight(shipmentURL, "/"),
	}
}

func (v *HTTPVerifier) Owns(ctx context.Context, tenantID uuid.UUID, sourceType string, sourceID uuid.UUID) (bool, error) {
	var endpoint string
	switch sourceType {
	case domain.SourceTransportOrder:
		if v.transportOrderURL == "" {
			return false, ErrUnavailable
		}
		endpoint = fmt.Sprintf("%s/v1/transport-orders/%s", v.transportOrderURL, sourceID)
	case domain.SourceShipment:
		if v.shipmentURL == "" {
			return false, ErrUnavailable
		}
		endpoint = fmt.Sprintf("%s/v1/shipments/%s", v.shipmentURL, sourceID)
	default:
		return false, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, ErrUnavailable
	}
	req.Header.Set(lowcode.HeaderTenantID, tenantID.String())
	resp, err := v.client.Do(req)
	if err != nil {
		return false, ErrUnavailable
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return false, ErrUnavailable
	}
	switch resp.StatusCode {
	case http.StatusOK:
		var probe struct {
			TenantID *uuid.UUID `json:"tenant_id"`
		}
		if json.Unmarshal(body, &probe) == nil && probe.TenantID != nil && *probe.TenantID != tenantID {
			return false, nil
		}
		return true, nil
	case http.StatusNotFound, http.StatusForbidden:
		return false, nil
	default:
		return false, ErrUnavailable
	}
}
