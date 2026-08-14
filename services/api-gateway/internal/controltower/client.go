package controltower

import (
	"context"
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
	company    string
	transport  string
	shipment   string
	document   string
	maxFetch   int
}

func NewDownstreamClient(
	httpClient *http.Client,
	identityURL, companyURL, transportURL, shipmentURL, documentURL string,
	maxFetch int,
) *DownstreamClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	if maxFetch <= 0 {
		maxFetch = 100
	}
	return &DownstreamClient{
		httpClient: httpClient,
		identity:   strings.TrimRight(identityURL, "/"),
		company:    strings.TrimRight(companyURL, "/"),
		transport:  strings.TrimRight(transportURL, "/"),
		shipment:   strings.TrimRight(shipmentURL, "/"),
		document:   strings.TrimRight(documentURL, "/"),
		maxFetch:   maxFetch,
	}
}

type listResult[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
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

func (c *DownstreamClient) FetchShipments(ctx context.Context, reqCtx RequestContext) ([]rawShipment, int, error) {
	endpoint, err := c.listURL(c.shipment+"/v1/shipments", reqCtx.TenantID, c.maxFetch, 0)
	if err != nil {
		return nil, 0, err
	}
	var payload listResult[map[string]any]
	if err := c.getJSON(ctx, endpoint, reqCtx, &payload); err != nil {
		return nil, 0, err
	}
	items := make([]rawShipment, 0, len(payload.Items))
	for _, item := range payload.Items {
		parsed, ok := parseRawShipment(item)
		if ok {
			items = append(items, parsed)
		}
	}
	total := payload.Total
	if total == 0 {
		total = len(items)
	}
	return items, total, nil
}

func (c *DownstreamClient) FetchTransportOrders(ctx context.Context, reqCtx RequestContext) ([]rawTransportOrder, error) {
	endpoint, err := c.listURL(c.transport+"/v1/transport-orders", reqCtx.TenantID, c.maxFetch, 0)
	if err != nil {
		return nil, err
	}
	var payload listResult[map[string]any]
	if err := c.getJSON(ctx, endpoint, reqCtx, &payload); err != nil {
		return nil, err
	}
	items := make([]rawTransportOrder, 0, len(payload.Items))
	for _, item := range payload.Items {
		id := stringField(item, "id")
		if id == "" {
			continue
		}
		items = append(items, rawTransportOrder{
			ID:          id,
			OrderNumber: stringField(item, "order_number"),
		})
	}
	return items, nil
}

func (c *DownstreamClient) FetchCompanies(ctx context.Context, reqCtx RequestContext) ([]rawCompany, error) {
	endpoint, err := c.listURL(c.company+"/v1/companies", reqCtx.TenantID, c.maxFetch, 0)
	if err != nil {
		return nil, err
	}
	var payload listResult[map[string]any]
	if err := c.getJSON(ctx, endpoint, reqCtx, &payload); err != nil {
		return nil, err
	}
	items := make([]rawCompany, 0, len(payload.Items))
	for _, item := range payload.Items {
		id := stringField(item, "id")
		if id == "" {
			continue
		}
		items = append(items, rawCompany{
			ID:          id,
			LegalName:   stringField(item, "legal_name"),
			ShortName:   stringPtrField(item, "short_name"),
			CompanyType: stringField(item, "company_type"),
		})
	}
	return items, nil
}

func (c *DownstreamClient) FetchDocuments(ctx context.Context, reqCtx RequestContext) ([]rawDocument, error) {
	endpoint, err := c.listURL(c.document+"/v1/documents", reqCtx.TenantID, c.maxFetch, 0)
	if err != nil {
		return nil, err
	}
	var payload listResult[map[string]any]
	if err := c.getJSON(ctx, endpoint, reqCtx, &payload); err != nil {
		return nil, err
	}
	items := make([]rawDocument, 0, len(payload.Items))
	for _, item := range payload.Items {
		items = append(items, rawDocument{
			RelatedEntityType: stringPtrField(item, "related_entity_type"),
			RelatedEntityID:   stringPtrField(item, "related_entity_id"),
			DocumentStatus:    stringField(item, "document_status"),
		})
	}
	return items, nil
}

func (c *DownstreamClient) listURL(base, tenantID string, limit, offset int) (string, error) {
	parsed, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("tenant_id", tenantID)
	query.Set("limit", fmt.Sprintf("%d", limit))
	query.Set("offset", fmt.Sprintf("%d", offset))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
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
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("downstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}

func (c *DownstreamClient) applyHeaders(req *http.Request, reqCtx RequestContext) {
	if reqCtx.AuthToken != "" {
		req.Header.Set("Authorization", reqCtx.AuthToken)
	}
	if reqCtx.TenantID != "" && !requestHasTenantQuery(req) {
		req.Header.Set("X-Tenant-ID", reqCtx.TenantID)
	}
	if reqCtx.UserID != "" {
		req.Header.Set("X-User-ID", reqCtx.UserID)
	}
	if reqCtx.RequestID != "" {
		req.Header.Set(sharedmiddleware.RequestIDHeader, reqCtx.RequestID)
	}
}

func requestHasTenantQuery(req *http.Request) bool {
	if req == nil || req.URL == nil {
		return false
	}
	return strings.Contains(req.URL.RawQuery, "tenant_id=")
}

func parseRawShipment(item map[string]any) (rawShipment, bool) {
	id := stringField(item, "id")
	if id == "" {
		return rawShipment{}, false
	}
	return rawShipment{
		ID:                    id,
		ShipmentNumber:        stringField(item, "shipment_number"),
		TransportOrderID:      stringPtrField(item, "transport_order_id"),
		ShipperCompanyID:      stringField(item, "shipper_company_id"),
		CarrierCompanyID:      stringPtrField(item, "carrier_company_id"),
		OriginLocationID:      stringField(item, "origin_location_id"),
		DestinationLocationID: stringField(item, "destination_location_id"),
		Status:                stringField(item, "status"),
		PlannedPickupAt:       timePtrField(item, "planned_pickup_at"),
		PlannedDeliveryAt:     timePtrField(item, "planned_delivery_at"),
		ActualPickupAt:        timePtrField(item, "actual_pickup_at"),
		ActualDeliveryAt:      timePtrField(item, "actual_delivery_at"),
		UpdatedAt:             timePtrField(item, "updated_at"),
		CreatedAt:             timePtrField(item, "created_at"),
		DriverID:              stringPtrField(item, "driver_id"),
		VehicleID:             stringPtrField(item, "vehicle_id"),
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

func stringPtrField(item map[string]any, key string) *string {
	value := stringField(item, key)
	if value == "" {
		return nil
	}
	return &value
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
