//go:build integration

package analytics

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"

	"github.com/freight-platform/freight-cost-service/internal/provider"
)

var errInjectedSettlementFailure = errors.New("injected settlement reader failure")

type failingSettlementReader struct {
	inner      provider.SettlementAccessorialReader
	mu         sync.Mutex
	failNext   bool
	failCount  int
	callCount  int
}

func newFailingSettlementReader(inner provider.SettlementAccessorialReader) *failingSettlementReader {
	return &failingSettlementReader{inner: inner}
}

func (f *failingSettlementReader) setFailNext(fail bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failNext = fail
}

func (f *failingSettlementReader) BatchGetSettlementsByTransportOrder(
	ctx context.Context,
	tenantID uuid.UUID,
	transportOrderIDs []uuid.UUID,
) (map[uuid.UUID]provider.SettlementAccessorialBatchItem, error) {
	f.mu.Lock()
	f.callCount++
	shouldFail := f.failNext
	if shouldFail {
		f.failNext = false
		f.failCount++
	}
	f.mu.Unlock()
	if shouldFail {
		return nil, errInjectedSettlementFailure
	}
	if f.inner == nil {
		return map[uuid.UUID]provider.SettlementAccessorialBatchItem{}, nil
	}
	return f.inner.BatchGetSettlementsByTransportOrder(ctx, tenantID, transportOrderIDs)
}
