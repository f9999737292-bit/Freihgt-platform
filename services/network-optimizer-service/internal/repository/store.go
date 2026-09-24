package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrConflict        = errors.New("conflict")
	ErrDuplicateSource = errors.New("duplicate source")
	ErrIdempotencyRace = errors.New("idempotency race")
)

type IdempotencyRecord struct {
	Key    string
	Hash   string
	Status int
	Body   []byte
}

type AuditEvent struct {
	ID              uuid.UUID
	ActorUserID     uuid.UUID
	TenantID        uuid.UUID
	AggregateType   string
	AggregateID     uuid.UUID
	Action          string
	FromStatus      string
	ToStatus        string
	VisibilityScope string
	SourceType      string
	SourceID        *uuid.UUID
	OccurredAt      time.Time
}

type OutboxEvent struct {
	ID               uuid.UUID
	EventName        string
	SchemaVersion    int
	TenantID         uuid.UUID
	AggregateID      uuid.UUID
	AggregateVersion int
	OccurredAt       time.Time
	Payload          []byte
}

type Tx interface {
	InsertLoad(context.Context, domain.LoadOpportunity) error
	UpdateLoad(context.Context, domain.LoadOpportunity, int) error
	GetLoad(context.Context, uuid.UUID) (domain.LoadOpportunity, error)
	ListOwnLoads(context.Context, uuid.UUID, int, int) ([]domain.LoadOpportunity, error)
	ListMarketplaceLoads(context.Context, uuid.UUID, *uuid.UUID, int, int) ([]domain.LoadOpportunity, error)
	ActiveLoadBySource(context.Context, uuid.UUID, string, uuid.UUID) (domain.LoadOpportunity, error)

	InsertCapacity(context.Context, domain.Capacity) error
	UpdateCapacity(context.Context, domain.Capacity, int) error
	GetCapacity(context.Context, uuid.UUID) (domain.Capacity, error)
	ListOwnCapacities(context.Context, uuid.UUID, int, int) ([]domain.Capacity, error)
	ListMarketplaceCapacities(context.Context, uuid.UUID, int, int) ([]domain.Capacity, error)

	GetIdempotency(context.Context, uuid.UUID, string) (IdempotencyRecord, error)
	PutIdempotency(context.Context, uuid.UUID, IdempotencyRecord) error
	InsertAudit(context.Context, AuditEvent) error
	InsertOutbox(context.Context, OutboxEvent) error
}

type Store interface {
	Within(context.Context, func(Tx) error) error
	Ping(context.Context) error
	ListOutbox(context.Context) ([]OutboxEvent, error)
	ListAudit(context.Context) ([]AuditEvent, error)
}
