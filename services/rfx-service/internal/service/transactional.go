package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/repository"
)

type transactionalRfxStore interface {
	RfxStore
}

type transactionalAuditRecorder interface {
	AuditRecorder
}

type atomicServices struct {
	rfxRepo   *repository.RfxRepository
	auditRepo *repository.AuditRepository
	bidRepo   *repository.BidRepository
	frRepo    *repository.FreightRequestRepository
	tx        *repository.TransactionRunner
}

func newAtomicServices(
	pool *pgxpool.Pool,
	rfxRepo *repository.RfxRepository,
	auditRepo *repository.AuditRepository,
	bidRepo *repository.BidRepository,
	frRepo *repository.FreightRequestRepository,
) *atomicServices {
	return &atomicServices{
		rfxRepo:   rfxRepo,
		auditRepo: auditRepo,
		bidRepo:   bidRepo,
		frRepo:    frRepo,
		tx:        repository.NewTransactionRunner(pool),
	}
}

func (a *atomicServices) runRfx(ctx context.Context, fn func(rfx RfxStore, audit AuditRecorder) error) error {
	if a == nil || a.tx == nil {
		return fn(a.rfxRepo, a.auditRepo)
	}
	return a.tx.Run(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(a.rfxRepo.WithTx(tx), a.auditRepo.WithTx(tx))
	})
}

func (a *atomicServices) runBid(ctx context.Context, fn func(bids BidStore, requests FreightRequestStore, audit AuditRecorder) error) error {
	if a == nil || a.tx == nil {
		return fn(a.bidRepo, a.frRepo, a.auditRepo)
	}
	return a.tx.Run(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(a.bidRepo.WithTx(tx), a.frRepo.WithTx(tx), a.auditRepo.WithTx(tx))
	})
}

func (a *atomicServices) runAutoClose(ctx context.Context, fn func(rfx RfxStore, audit AuditRecorder) error) error {
	return a.runRfx(ctx, fn)
}

func verifiedActorCompany(actor domain.ActorContext, verifiedCompanyID uuid.UUID) *uuid.UUID {
	if verifiedCompanyID == uuid.Nil {
		return nil
	}
	id := verifiedCompanyID
	return &id
}
