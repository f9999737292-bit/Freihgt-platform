package shipmentevents

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
)

type DownstreamClient struct {
	httpClient *http.Client
	identity   string
	shipment   string
	document   string
	billing    string
	maxFetch   int
}

func NewDownstreamClient(
	httpClient *http.Client,
	identityURL, shipmentURL, documentURL, billingURL string,
	maxFetch int,
) *DownstreamClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	if maxFetch <= 0 {
		maxFetch = 200
	}
	return &DownstreamClient{
		httpClient: httpClient,
		identity:   strings.TrimRight(identityURL, "/"),
		shipment:   strings.TrimRight(shipmentURL, "/"),
		document:   strings.TrimRight(documentURL, "/"),
		billing:    strings.TrimRight(billingURL, "/"),
		maxFetch:   maxFetch,
	}
}

type listResult struct {
	Items []map[string]any `json:"items"`
	Total int              `json:"total"`
}

type meResponse struct {
	Roles []string `json:"roles"`
}

func (c *DownstreamClient) FetchUserRoles(ctx context.Context, reqCtx RequestContext) ([]string, error) {
	endpoint := c.identity + "/v1/auth/me"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	c.applyHeaders(req, reqCtx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("unauthorized")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("identity service returned %d", resp.StatusCode)
	}

	var payload meResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Roles, nil
}

type shipmentFetchResult struct {
	Shipment *rawShipment
	NotFound bool
}

func (c *DownstreamClient) FetchShipment(ctx context.Context, reqCtx RequestContext, shipmentID string) (shipmentFetchResult, error) {
	values := url.Values{}
	values.Set("tenant_id", reqCtx.TenantID)
	endpoint := c.shipment + "/v1/shipments/" + url.PathEscape(shipmentID) + "?" + values.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return shipmentFetchResult{}, err
	}
	c.applyHeaders(req, reqCtx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return shipmentFetchResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return shipmentFetchResult{NotFound: true}, nil
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return shipmentFetchResult{}, fmt.Errorf("shipment service returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var item map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return shipmentFetchResult{}, err
	}
	shipment, ok := parseRawShipment(item)
	if !ok {
		return shipmentFetchResult{}, fmt.Errorf("invalid shipment payload")
	}
	return shipmentFetchResult{Shipment: &shipment}, nil
}

func (c *DownstreamClient) FetchDocuments(ctx context.Context, reqCtx RequestContext, shipmentID string) ([]rawDocument, bool, error) {
	values := url.Values{}
	values.Set("tenant_id", reqCtx.TenantID)
	values.Set("related_entity_type", "SHIPMENT")
	values.Set("related_entity_id", shipmentID)
	values.Set("limit", fmt.Sprintf("%d", c.maxFetch))
	values.Set("offset", "0")

	endpoint := c.document + "/v1/documents?" + values.Encode()
	var payload listResult
	if err := c.getJSON(ctx, endpoint, reqCtx, &payload); err != nil {
		return nil, false, err
	}

	items := make([]rawDocument, 0, len(payload.Items))
	for _, item := range payload.Items {
		doc, ok := parseRawDocument(item)
		if ok {
			items = append(items, doc)
		}
	}
	limited := payload.Total > len(items) || len(items) >= c.maxFetch
	return items, limited, nil
}

func (c *DownstreamClient) getJSON(ctx context.Context, endpoint string, reqCtx RequestContext, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	c.applyHeaders(req, reqCtx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("downstream returned %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}

func (c *DownstreamClient) applyHeaders(req *http.Request, reqCtx RequestContext) {
	if reqCtx.AuthToken != "" {
		req.Header.Set("Authorization", reqCtx.AuthToken)
	}
	if reqCtx.RequestID != "" {
		req.Header.Set(sharedmiddleware.RequestIDHeader, reqCtx.RequestID)
	}
}

func parseRawShipment(item map[string]any) (rawShipment, bool) {
	id := stringField(item, "id")
	if id == "" {
		return rawShipment{}, false
	}
	technicalProblem := false
	if v, ok := item["technical_problem"]; ok {
		switch typed := v.(type) {
		case bool:
			technicalProblem = typed
		}
	}
	return rawShipment{
		ID:                id,
		TenantID:          stringField(item, "tenant_id"),
		ShipmentNumber:    stringField(item, "shipment_number"),
		Status:            stringField(item, "status"),
		PlannedPickupAt:   timePtrField(item, "planned_pickup_at"),
		PlannedDeliveryAt: timePtrField(item, "planned_delivery_at"),
		ActualPickupAt:    timePtrField(item, "actual_pickup_at"),
		ActualDeliveryAt:  timePtrField(item, "actual_delivery_at"),
		TechnicalProblem:  technicalProblem,
		CreatedAt:         timePtrField(item, "created_at"),
		UpdatedAt:         timePtrField(item, "updated_at"),
	}, true
}

func parseRawDocument(item map[string]any) (rawDocument, bool) {
	id := stringField(item, "id")
	if id == "" {
		return rawDocument{}, false
	}
	return rawDocument{
		ID:             id,
		DocumentType:   stringField(item, "document_type"),
		DocumentStatus: stringField(item, "document_status"),
		CreatedAt:      timePtrField(item, "created_at"),
		SignedAt:       timePtrField(item, "signed_at"),
		RejectedAt:     timePtrField(item, "rejected_at"),
	}, true
}

func stringField(item map[string]any, key string) string {
	value, ok := item[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func timePtrField(item map[string]any, key string) *time.Time {
	raw := stringField(item, key)
	if raw == "" {
		return nil
	}
	layouts := []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02T15:04:05.000Z"}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			utc := parsed.UTC()
			return &utc
		}
	}
	return nil
}

func deterministicEventID(shipmentID, eventType string, occurredAt time.Time, relatedEntityID, source string) string {
	payload := strings.Join([]string{
		shipmentID,
		eventType,
		occurredAt.UTC().Format(time.RFC3339Nano),
		relatedEntityID,
		source,
	}, "|")
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:16])
}
