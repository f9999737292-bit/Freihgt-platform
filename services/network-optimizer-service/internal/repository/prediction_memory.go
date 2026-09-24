package repository

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
)

func (t *memTx) InsertPrediction(_ context.Context, prediction domain.PredictedCapacity) error {
	if prediction.IsCurrent {
		for _, existing := range t.preds {
			if existing.IsCurrent && existing.OwnerTenantID == prediction.OwnerTenantID && existing.ShipmentID == prediction.ShipmentID {
				return ErrConflict
			}
		}
	}
	if t.preds == nil {
		t.preds = map[uuid.UUID]domain.PredictedCapacity{}
	}
	t.preds[prediction.ID] = copyPrediction(prediction)
	return nil
}

func (t *memTx) UpdatePrediction(_ context.Context, prediction domain.PredictedCapacity) error {
	if _, ok := t.preds[prediction.ID]; !ok {
		return ErrNotFound
	}
	if prediction.IsCurrent {
		for id, existing := range t.preds {
			if id != prediction.ID && existing.IsCurrent && existing.OwnerTenantID == prediction.OwnerTenantID && existing.ShipmentID == prediction.ShipmentID {
				return ErrConflict
			}
		}
	}
	t.preds[prediction.ID] = copyPrediction(prediction)
	return nil
}

func (t *memTx) GetPrediction(_ context.Context, id uuid.UUID) (domain.PredictedCapacity, error) {
	prediction, ok := t.preds[id]
	if !ok {
		return domain.PredictedCapacity{}, ErrNotFound
	}
	return copyPrediction(prediction), nil
}

func (t *memTx) CurrentPredictionByShipment(_ context.Context, tenant, shipment uuid.UUID) (domain.PredictedCapacity, error) {
	for _, prediction := range t.preds {
		if prediction.IsCurrent && prediction.OwnerTenantID == tenant && prediction.ShipmentID == shipment {
			return copyPrediction(prediction), nil
		}
	}
	return domain.PredictedCapacity{}, ErrNotFound
}

func (t *memTx) ListOwnPredictions(_ context.Context, tenant uuid.UUID, limit, offset int) ([]domain.PredictedCapacity, error) {
	rows := make([]domain.PredictedCapacity, 0)
	for _, prediction := range t.preds {
		if prediction.OwnerTenantID == tenant {
			rows = append(rows, copyPrediction(prediction))
		}
	}
	slices.SortFunc(rows, func(a, b domain.PredictedCapacity) int {
		if a.GeneratedAt.Equal(b.GeneratedAt) {
			return compareUUID(a.ID, b.ID)
		}
		if a.GeneratedAt.After(b.GeneratedAt) {
			return -1
		}
		return 1
	})
	if offset > len(rows) {
		return []domain.PredictedCapacity{}, nil
	}
	rows = rows[offset:]
	if limit < len(rows) {
		rows = rows[:limit]
	}
	return rows, nil
}

func copyPrediction(prediction domain.PredictedCapacity) domain.PredictedCapacity {
	prediction.LoadingAccess = copyStringSlice(prediction.LoadingAccess)
	prediction.UnloadingAccess = copyStringSlice(prediction.UnloadingAccess)
	return prediction
}

func copyStringSlice(values []string) []string {
	if values == nil {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

func clonePreds(in map[uuid.UUID]domain.PredictedCapacity) map[uuid.UUID]domain.PredictedCapacity {
	out := make(map[uuid.UUID]domain.PredictedCapacity, len(in))
	for id, prediction := range in {
		out[id] = copyPrediction(prediction)
	}
	return out
}
