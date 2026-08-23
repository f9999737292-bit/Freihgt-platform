package billing_register

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	"github.com/freight-platform/freight-cost-service/internal/provider"
	apperrors "github.com/freight-platform/freight-cost-service/internal/platform/errors"
	"github.com/freight-platform/shared-go/internalauth"
)

const settlementBatchSize = 500

type batchSettlementsRequest struct {
	TransportOrderIDs []string `json:"transport_order_ids"`
}

type batchAccessorialLineResponse struct {
	AccessorialID string `json:"accessorial_id"`
	ChargeCode    string `json:"charge_code"`
	Amount        string `json:"amount"`
	Status        string `json:"status"`
	CurrencyCode  string `json:"currency_code"`
}

type batchSettlementItemResponse struct {
	TransportOrderID         string                         `json:"transport_order_id"`
	SettlementID             string                         `json:"settlement_id"`
	BuyerCompanyID           string                         `json:"buyer_company_id"`
	CurrencyCode             string                         `json:"currency_code"`
	ApprovedAccessorialTotal string                         `json:"approved_accessorial_total"`
	Accessorials             []batchAccessorialLineResponse `json:"accessorials"`
}

type batchSettlementsResponse struct {
	Items []batchSettlementItemResponse `json:"items"`
}

func (c *Client) BatchGetSettlementsByTransportOrder(
	ctx context.Context,
	tenantID uuid.UUID,
	transportOrderIDs []uuid.UUID,
) (map[uuid.UUID]provider.SettlementAccessorialBatchItem, error) {
	const operation = "batch_settlements_by_transport_order"
	result := make(map[uuid.UUID]provider.SettlementAccessorialBatchItem)
	if c == nil || c.baseURL == "" || len(transportOrderIDs) == 0 {
		return result, nil
	}
	for start := 0; start < len(transportOrderIDs); start += settlementBatchSize {
		end := start + settlementBatchSize
		if end > len(transportOrderIDs) {
			end = len(transportOrderIDs)
		}
		chunk := transportOrderIDs[start:end]
		ids := make([]string, len(chunk))
		for i, id := range chunk {
			ids[i] = id.String()
		}
		payload, err := json.Marshal(batchSettlementsRequest{TransportOrderIDs: ids})
		if err != nil {
			return nil, apperrors.Internal("marshal settlement batch request", err)
		}
		url := fmt.Sprintf("%s/internal/v1/freight-settlements/batch-by-transport-order", c.baseURL)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return nil, apperrors.Internal("create settlement batch request", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", tenantID.String())
		if c.token != "" {
			req.Header.Set(internalauth.HeaderName, c.token)
		}
		resp, err := c.client.Do(req)
		if err != nil {
			c.observe(operation, "error", string(apperrors.CodeUnavailable))
			return nil, apperrors.Unavailable("billing register service unavailable", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if readErr != nil {
			c.observe(operation, "error", string(apperrors.CodeUnavailable))
			return nil, apperrors.Unavailable("billing register service unavailable", readErr)
		}
		if resp.StatusCode != http.StatusOK {
			c.observe(operation, "error", string(apperrors.CodeBadGateway))
			return nil, apperrors.BadGateway("billing settlement batch failed", fmt.Errorf("status %d", resp.StatusCode))
		}
		var decoded batchSettlementsResponse
		if err := json.Unmarshal(body, &decoded); err != nil {
			c.observe(operation, "error", string(apperrors.CodeBadGateway))
			return nil, apperrors.BadGateway("invalid settlement batch response", err)
		}
		for _, item := range decoded.Items {
			transportOrderID, err := parseUUID("transport_order_id", item.TransportOrderID)
			if err != nil {
				continue
			}
			settlementID, err := parseUUID("settlement_id", item.SettlementID)
			if err != nil {
				continue
			}
			buyerID, err := parseUUID("buyer_company_id", item.BuyerCompanyID)
			if err != nil {
				continue
			}
			approvedTotal, err := domain.ParseMoneyAmount(item.ApprovedAccessorialTotal)
			if err != nil {
				continue
			}
			batchItem := provider.SettlementAccessorialBatchItem{
				TransportOrderID:         transportOrderID,
				SettlementID:             settlementID,
				BuyerCompanyID:           buyerID,
				CurrencyCode:             strings.ToUpper(strings.TrimSpace(item.CurrencyCode)),
				ApprovedAccessorialTotal: approvedTotal,
			}
			for _, line := range item.Accessorials {
				accessorialID, err := parseUUID("accessorial_id", line.AccessorialID)
				if err != nil {
					continue
				}
				amount, err := domain.ParseMoneyAmount(line.Amount)
				if err != nil {
					continue
				}
				batchItem.Accessorials = append(batchItem.Accessorials, provider.SettlementAccessorialLine{
					AccessorialID: accessorialID,
					ChargeCode:    line.ChargeCode,
					Amount:        amount,
					Status:        strings.ToUpper(strings.TrimSpace(line.Status)),
					CurrencyCode:  strings.ToUpper(strings.TrimSpace(line.CurrencyCode)),
				})
			}
			result[transportOrderID] = batchItem
		}
	}
	c.observe(operation, "success", "")
	return result, nil
}

var _ provider.SettlementAccessorialReader = (*Client)(nil)
