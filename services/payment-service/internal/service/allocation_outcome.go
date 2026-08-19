package service

import "github.com/freight-platform/payment-service/internal/repository"

const (
	RegisterPaidProjectionSynced = "SYNCED"
	RegisterPaidProjectionFailed = "FAILED"

	OutboxProjectionPublished  = "PUBLISHED"
	OutboxProjectionMarkFailed = "MARK_FAILED"
)

type RegisterPaidProjection struct {
	Status    string `json:"status"`
	Retryable bool   `json:"retryable"`
	Message   string `json:"message,omitempty"`
}

type OutboxProjection struct {
	Status    string `json:"status"`
	Retryable bool   `json:"retryable"`
	Message   string `json:"message,omitempty"`
}

type AllocateOutcome struct {
	Result                 *repository.AllocateResult
	RegisterPaidProjection *RegisterPaidProjection
	OutboxProjection       *OutboxProjection
}
