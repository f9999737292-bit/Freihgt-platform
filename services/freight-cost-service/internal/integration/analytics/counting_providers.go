//go:build integration

package analytics

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"github.com/freight-platform/freight-cost-service/internal/provider"
)

type countingCompanyReader struct {
	inner provider.CompanyDisplayReader
	mu    sync.Mutex
	calls int
	ids   int
}

func wrapCountingCompany(inner provider.CompanyDisplayReader) *countingCompanyReader {
	return &countingCompanyReader{inner: inner}
}

func (c *countingCompanyReader) BatchGetCompanyDisplay(
	ctx context.Context,
	tenantID uuid.UUID,
	companyIDs []uuid.UUID,
) (map[uuid.UUID]provider.CompanyDisplay, error) {
	c.mu.Lock()
	c.calls++
	c.ids += len(companyIDs)
	c.mu.Unlock()
	if c.inner == nil {
		return map[uuid.UUID]provider.CompanyDisplay{}, nil
	}
	return c.inner.BatchGetCompanyDisplay(ctx, tenantID, companyIDs)
}

func (c *countingCompanyReader) snapshot() (calls, ids int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls, c.ids
}

type countingDimensionReader struct {
	inner provider.TransportDimensionReader
	mu    sync.Mutex
	calls int
	ids   int
}

func wrapCountingDimensions(inner provider.TransportDimensionReader) *countingDimensionReader {
	return &countingDimensionReader{inner: inner}
}

func (c *countingDimensionReader) BatchGetAnalyticsDimensions(
	ctx context.Context,
	tenantID uuid.UUID,
	transportOrderIDs []uuid.UUID,
) (map[uuid.UUID]provider.TransportOrderAnalyticsDimension, error) {
	c.mu.Lock()
	c.calls++
	c.ids += len(transportOrderIDs)
	c.mu.Unlock()
	if c.inner == nil {
		return map[uuid.UUID]provider.TransportOrderAnalyticsDimension{}, nil
	}
	return c.inner.BatchGetAnalyticsDimensions(ctx, tenantID, transportOrderIDs)
}

func (c *countingDimensionReader) snapshot() (calls, ids int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls, c.ids
}

type countingSettlementReader struct {
	inner provider.SettlementAccessorialReader
	mu    sync.Mutex
	calls int
	ids   int
}

func wrapCountingSettlements(inner provider.SettlementAccessorialReader) *countingSettlementReader {
	return &countingSettlementReader{inner: inner}
}

func (c *countingSettlementReader) BatchGetSettlementsByTransportOrder(
	ctx context.Context,
	tenantID uuid.UUID,
	transportOrderIDs []uuid.UUID,
) (map[uuid.UUID]provider.SettlementAccessorialBatchItem, error) {
	c.mu.Lock()
	c.calls++
	c.ids += len(transportOrderIDs)
	c.mu.Unlock()
	if c.inner == nil {
		return map[uuid.UUID]provider.SettlementAccessorialBatchItem{}, nil
	}
	return c.inner.BatchGetSettlementsByTransportOrder(ctx, tenantID, transportOrderIDs)
}

func (c *countingSettlementReader) snapshot() (calls, ids int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls, c.ids
}
