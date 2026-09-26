package repository

import (
	"context"

	"github.com/google/uuid"
)

func (m *Memory) SaveConsolidation(_ context.Context, run ConsolidationRun, candidates []ConsolidationCandidate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.consolidationRuns == nil {
		m.consolidationRuns = map[uuid.UUID]ConsolidationRun{}
		m.consolidationCandidates = map[uuid.UUID][]ConsolidationCandidate{}
	}
	copied := make([]ConsolidationCandidate, len(candidates))
	for i, candidate := range candidates {
		copied[i] = candidate
		copied[i].Members = append([]ConsolidationMember(nil), candidate.Members...)
		copied[i].HardRejectReasons = append([]string(nil), candidate.HardRejectReasons...)
		copied[i].IndeterminateReasonCodes = append([]string(nil), candidate.IndeterminateReasonCodes...)
	}
	m.consolidationRuns[run.ID] = run
	m.consolidationCandidates[run.ID] = copied
	return nil
}

func (m *Memory) GetConsolidation(_ context.Context, tenant, id uuid.UUID) (ConsolidationRun, []ConsolidationCandidate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	run, ok := m.consolidationRuns[id]
	if !ok || run.TenantID != tenant {
		return ConsolidationRun{}, nil, ErrNotFound
	}
	rows := m.consolidationCandidates[id]
	out := make([]ConsolidationCandidate, len(rows))
	for i, candidate := range rows {
		out[i] = candidate
		out[i].Members = append([]ConsolidationMember(nil), candidate.Members...)
	}
	return run, out, nil
}
