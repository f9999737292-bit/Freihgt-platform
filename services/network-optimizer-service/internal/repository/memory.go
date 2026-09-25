package repository

import (
	"context"
	"slices"
	"sync"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
)

type Memory struct {
	mu               sync.Mutex
	loads            map[uuid.UUID]domain.LoadOpportunity
	caps             map[uuid.UUID]domain.Capacity
	preds            map[uuid.UUID]domain.PredictedCapacity
	idem             map[string]IdempotencyRecord
	audits           []AuditEvent
	outbox           []OutboxEvent
	carrierPolicies  map[uuid.UUID]domain.NextLoadSearchPolicy
	capacityPolicies map[uuid.UUID]storedCapacityPolicy
}

func NewMemory() *Memory {
	return &Memory{
		loads: map[uuid.UUID]domain.LoadOpportunity{},
		caps:  map[uuid.UUID]domain.Capacity{},
		preds: map[uuid.UUID]domain.PredictedCapacity{},
		idem:  map[string]IdempotencyRecord{},
	}
}

func (m *Memory) Ping(context.Context) error { return nil }

func (m *Memory) Within(_ context.Context, fn func(Tx) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	tx := &memTx{
		loads:  cloneLoads(m.loads),
		caps:   cloneCaps(m.caps),
		preds:  clonePreds(m.preds),
		idem:   cloneIdem(m.idem),
		audits: append([]AuditEvent(nil), m.audits...),
		outbox: append([]OutboxEvent(nil), m.outbox...),
	}
	if err := fn(tx); err != nil {
		return err
	}
	m.loads = tx.loads
	m.caps = tx.caps
	m.preds = tx.preds
	m.idem = tx.idem
	m.audits = tx.audits
	m.outbox = tx.outbox
	return nil
}

func (m *Memory) ListOutbox(context.Context) ([]OutboxEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]OutboxEvent(nil), m.outbox...), nil
}

func (m *Memory) ListAudit(context.Context) ([]AuditEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]AuditEvent(nil), m.audits...), nil
}

type memTx struct {
	loads  map[uuid.UUID]domain.LoadOpportunity
	caps   map[uuid.UUID]domain.Capacity
	preds  map[uuid.UUID]domain.PredictedCapacity
	idem   map[string]IdempotencyRecord
	audits []AuditEvent
	outbox []OutboxEvent
}

func (t *memTx) InsertLoad(_ context.Context, load domain.LoadOpportunity) error {
	for _, existing := range t.loads {
		if existing.OwnerTenantID == load.OwnerTenantID && existing.SourceType == load.SourceType && existing.SourceID == load.SourceID &&
			(existing.Status == domain.LoadDraft || existing.Status == domain.LoadPublished) {
			return ErrDuplicateSource
		}
	}
	t.loads[load.ID] = copyLoad(load)
	return nil
}

func (t *memTx) UpdateLoad(_ context.Context, load domain.LoadOpportunity, expected int) error {
	current, ok := t.loads[load.ID]
	if !ok {
		return ErrNotFound
	}
	if current.Version != expected || load.Version != expected+1 {
		return ErrConflict
	}
	t.loads[load.ID] = copyLoad(load)
	return nil
}

func (t *memTx) GetLoad(_ context.Context, id uuid.UUID) (domain.LoadOpportunity, error) {
	load, ok := t.loads[id]
	if !ok {
		return domain.LoadOpportunity{}, ErrNotFound
	}
	return copyLoad(load), nil
}

func (t *memTx) ListOwnLoads(_ context.Context, tenant uuid.UUID, limit, offset int) ([]domain.LoadOpportunity, error) {
	var rows []domain.LoadOpportunity
	for _, load := range t.loads {
		if load.OwnerTenantID == tenant {
			rows = append(rows, copyLoad(load))
		}
	}
	sortLoads(rows)
	return pageLoads(rows, limit, offset), nil
}

func (t *memTx) ListMarketplaceLoads(_ context.Context, viewer uuid.UUID, company *uuid.UUID, limit, offset int) ([]domain.LoadOpportunity, error) {
	var rows []domain.LoadOpportunity
	for _, load := range t.loads {
		if ok, _ := domain.LoadVisible(load, viewer, company); ok {
			rows = append(rows, copyLoad(load))
		}
	}
	sortLoads(rows)
	return pageLoads(rows, limit, offset), nil
}

func (t *memTx) ActiveLoadBySource(_ context.Context, tenant uuid.UUID, sourceType string, sourceID uuid.UUID) (domain.LoadOpportunity, error) {
	for _, load := range t.loads {
		if load.OwnerTenantID == tenant && load.SourceType == sourceType && load.SourceID == sourceID &&
			(load.Status == domain.LoadDraft || load.Status == domain.LoadPublished) {
			return copyLoad(load), nil
		}
	}
	return domain.LoadOpportunity{}, ErrNotFound
}

func (t *memTx) InsertCapacity(_ context.Context, cap domain.Capacity) error {
	t.caps[cap.ID] = copyCap(cap)
	return nil
}

func (t *memTx) UpdateCapacity(_ context.Context, cap domain.Capacity, expected int) error {
	current, ok := t.caps[cap.ID]
	if !ok {
		return ErrNotFound
	}
	if current.Version != expected || cap.Version != expected+1 {
		return ErrConflict
	}
	t.caps[cap.ID] = copyCap(cap)
	return nil
}

func (t *memTx) GetCapacity(_ context.Context, id uuid.UUID) (domain.Capacity, error) {
	cap, ok := t.caps[id]
	if !ok {
		return domain.Capacity{}, ErrNotFound
	}
	return copyCap(cap), nil
}

func (t *memTx) ListOwnCapacities(_ context.Context, tenant uuid.UUID, limit, offset int) ([]domain.Capacity, error) {
	var rows []domain.Capacity
	for _, cap := range t.caps {
		if cap.OwnerTenantID == tenant {
			rows = append(rows, copyCap(cap))
		}
	}
	sortCaps(rows)
	return pageCaps(rows, limit, offset), nil
}

func (t *memTx) ListMarketplaceCapacities(_ context.Context, viewer uuid.UUID, limit, offset int) ([]domain.Capacity, error) {
	var rows []domain.Capacity
	for _, cap := range t.caps {
		if domain.CapacityVisible(cap, viewer) {
			rows = append(rows, copyCap(cap))
		}
	}
	sortCaps(rows)
	return pageCaps(rows, limit, offset), nil
}

func (t *memTx) GetIdempotency(_ context.Context, tenant uuid.UUID, key string) (IdempotencyRecord, error) {
	rec, ok := t.idem[idemKey(tenant, key)]
	if !ok {
		return IdempotencyRecord{}, ErrNotFound
	}
	rec.Body = append([]byte(nil), rec.Body...)
	return rec, nil
}

func (t *memTx) PutIdempotency(_ context.Context, tenant uuid.UUID, rec IdempotencyRecord) error {
	key := idemKey(tenant, rec.Key)
	if _, ok := t.idem[key]; ok {
		return ErrIdempotencyRace
	}
	rec.Body = append([]byte(nil), rec.Body...)
	t.idem[key] = rec
	return nil
}

func (t *memTx) InsertAudit(_ context.Context, event AuditEvent) error {
	t.audits = append(t.audits, event)
	return nil
}

func (t *memTx) InsertOutbox(_ context.Context, event OutboxEvent) error {
	event.Payload = append([]byte(nil), event.Payload...)
	t.outbox = append(t.outbox, event)
	return nil
}

func idemKey(tenant uuid.UUID, key string) string { return tenant.String() + "|" + key }

func copyLoad(load domain.LoadOpportunity) domain.LoadOpportunity {
	load.Equipment = append([]string(nil), load.Equipment...)
	load.InvitedCarrierCompanyIDs = append([]uuid.UUID(nil), load.InvitedCarrierCompanyIDs...)
	return load
}

func copyCap(cap domain.Capacity) domain.Capacity {
	cap.Equipment = append([]string(nil), cap.Equipment...)
	cap.AudienceTenantIDs = append([]uuid.UUID(nil), cap.AudienceTenantIDs...)
	return cap
}

func cloneLoads(in map[uuid.UUID]domain.LoadOpportunity) map[uuid.UUID]domain.LoadOpportunity {
	out := make(map[uuid.UUID]domain.LoadOpportunity, len(in))
	for id, load := range in {
		out[id] = copyLoad(load)
	}
	return out
}

func cloneCaps(in map[uuid.UUID]domain.Capacity) map[uuid.UUID]domain.Capacity {
	out := make(map[uuid.UUID]domain.Capacity, len(in))
	for id, cap := range in {
		out[id] = copyCap(cap)
	}
	return out
}

func cloneIdem(in map[string]IdempotencyRecord) map[string]IdempotencyRecord {
	out := make(map[string]IdempotencyRecord, len(in))
	for key, rec := range in {
		rec.Body = append([]byte(nil), rec.Body...)
		out[key] = rec
	}
	return out
}

func sortLoads(rows []domain.LoadOpportunity) {
	slices.SortFunc(rows, func(a, b domain.LoadOpportunity) int {
		if a.CreatedAt.Equal(b.CreatedAt) {
			return compareUUID(a.ID, b.ID)
		}
		if a.CreatedAt.After(b.CreatedAt) {
			return -1
		}
		return 1
	})
}

func sortCaps(rows []domain.Capacity) {
	slices.SortFunc(rows, func(a, b domain.Capacity) int {
		if a.CreatedAt.Equal(b.CreatedAt) {
			return compareUUID(a.ID, b.ID)
		}
		if a.CreatedAt.After(b.CreatedAt) {
			return -1
		}
		return 1
	})
}

func compareUUID(a, b uuid.UUID) int {
	as, bs := a.String(), b.String()
	if as < bs {
		return -1
	}
	if as > bs {
		return 1
	}
	return 0
}

func pageLoads(rows []domain.LoadOpportunity, limit, offset int) []domain.LoadOpportunity {
	if offset >= len(rows) {
		return []domain.LoadOpportunity{}
	}
	rows = rows[offset:]
	if limit < len(rows) {
		rows = rows[:limit]
	}
	return rows
}

func pageCaps(rows []domain.Capacity, limit, offset int) []domain.Capacity {
	if offset >= len(rows) {
		return []domain.Capacity{}
	}
	rows = rows[offset:]
	if limit < len(rows) {
		rows = rows[:limit]
	}
	return rows
}
