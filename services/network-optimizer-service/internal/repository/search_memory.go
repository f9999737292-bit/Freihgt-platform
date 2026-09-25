package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
)

func (m *Memory) SaveSearch(_ context.Context, run SearchRun, candidates []StoredCandidate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.searchRuns == nil {
		m.searchRuns = map[uuid.UUID]SearchRun{}
		m.searchCandidates = map[uuid.UUID][]StoredCandidate{}
	}
	m.searchRuns[run.ID] = run
	copied := append([]StoredCandidate(nil), candidates...)
	m.searchCandidates[run.ID] = copied
	return nil
}

func (m *Memory) GetSearch(_ context.Context, tenant, id uuid.UUID) (SearchRun, []StoredCandidate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	run, ok := m.searchRuns[id]
	if !ok || run.TenantID != tenant {
		return SearchRun{}, nil, ErrNotFound
	}
	return run, append([]StoredCandidate(nil), m.searchCandidates[id]...), nil
}

func (m *Memory) CurrentPredictionForCapacity(_ context.Context, tenant, capacityID uuid.UUID) (domain.PredictedCapacity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, prediction := range m.preds {
		if prediction.CapacityID == capacityID && prediction.OwnerTenantID == tenant && prediction.IsCurrent {
			return prediction, nil
		}
	}
	return domain.PredictedCapacity{}, ErrNotFound
}
