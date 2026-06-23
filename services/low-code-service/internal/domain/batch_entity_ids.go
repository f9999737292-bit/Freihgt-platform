package domain

import "github.com/google/uuid"

// NormalizeBatchEntityIDs removes duplicate entity IDs while preserving first-seen order.
// Batch preview/execute treat duplicate IDs as a single entity (hardening v0.1).
func NormalizeBatchEntityIDs(entityIDs []uuid.UUID) []uuid.UUID {
	if len(entityIDs) <= 1 {
		return entityIDs
	}
	seen := make(map[uuid.UUID]struct{}, len(entityIDs))
	normalized := make([]uuid.UUID, 0, len(entityIDs))
	for _, id := range entityIDs {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized
}
