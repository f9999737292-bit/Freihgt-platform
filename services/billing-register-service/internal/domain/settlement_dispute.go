package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

const (
	DisputeStatusOpen       = "OPEN"
	DisputeStatusResolved   = "RESOLVED"
	DisputeStatusWithdrawn  = "WITHDRAWN"
)

type SettlementDispute struct {
	ID                 uuid.UUID
	TenantID           uuid.UUID
	SettlementID       uuid.UUID
	AccessorialID      *uuid.UUID
	Reason             string
	RaisedBy           uuid.UUID
	RaisedByCompanyID  uuid.UUID
	Status             string
	ResolutionNote     *string
	ResolvedBy         *uuid.UUID
	ResolvedAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type RaiseDisputeInput struct {
	SettlementActorInput
	AccessorialID  *uuid.UUID
	Reason         string
}

type ResolveDisputeInput struct {
	SettlementActorInput
	ResolutionNote string
}

func ValidateRaiseDisputeInput(in RaiseDisputeInput) error {
	if err := ValidateSettlementActor(in.SettlementActorInput); err != nil {
		return err
	}
	if strings.TrimSpace(in.Reason) == "" {
		return apperrors.Validation("reason is required", map[string]any{"field": "reason"})
	}
	return nil
}
