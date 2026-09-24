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
	"github.com/freight-platform/shared-go/internalauth"
	"github.com/freight-platform/shared-go/lowcode"
)

type HTTPVerifier struct {
	client            *http.Client
	transportOrderURL string
	shipmentURL       string
	serviceToken      string
}

func NewHTTP(transportOrderURL, shipmentURL, serviceToken string) *HTTPVerifier {
	return &HTTPVerifier{
		client: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		transportOrderURL: strings.TrimRight(transportOrderURL, "/"),
		shipmentURL:       strings.TrimRight(shipmentURL, "/"),
		serviceToken:      strings.TrimSpace(serviceToken),
	}
}

func (v *HTTPVerifier) Owns(ctx context.Context, tenantID uuid.UUID, sourceType string, sourceID uuid.UUID) (bool, error) {
	if strings.TrimSpace(v.serviceToken) == "" {
		return false, ErrUnavailable
	}
	var endpoint string
	switch sourceType {
	case domain.SourceTransportOrder:
		if v.transportOrderURL == "" {
			return false, ErrUnavailable
		}
		endpoint = fmt.Sprintf("%s/internal/v1/transport-orders/%s/ownership", v.transportOrderURL, sourceID)
	case domain.SourceShipment:
		if v.shipmentURL == "" {
			return false, ErrUnavailable
		}
		endpoint = fmt.Sprintf("%s/internal/v1/shipments/%s/ownership", v.shipmentURL, sourceID)
	default:
		return false, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, ErrUnavailable
	}
	// Fresh request. The token is the configured platform service credential.
	// tenantID is the actor already established for this call.
	// Inbound client headers, including integration identity headers, are not copied.
	req.Header.Set(lowcode.HeaderTenantID, tenantID.String())
	req.Header.Set(internalauth.HeaderName, v.serviceToken)
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
			ID       *uuid.UUID `json:"id"`
			TenantID *uuid.UUID `json:"tenant_id"`
		}
		if json.Unmarshal(body, &probe) != nil || probe.ID == nil || probe.TenantID == nil || *probe.ID != sourceID || *probe.TenantID != tenantID {
			return false, nil
		}
		return true, nil
	case http.StatusNotFound, http.StatusForbidden:
		return false, nil
	default:
		return false, ErrUnavailable
	}
}
