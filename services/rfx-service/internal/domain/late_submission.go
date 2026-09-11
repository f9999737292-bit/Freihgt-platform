package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type LateSubmissionStatus string

const (
	LateSubmissionStatusRequested LateSubmissionStatus = "REQUESTED"
	LateSubmissionStatusApproved  LateSubmissionStatus = "APPROVED"
	LateSubmissionStatusRejected  LateSubmissionStatus = "REJECTED"
	LateSubmissionStatusExpired   LateSubmissionStatus = "EXPIRED"
	LateSubmissionStatusConsumed  LateSubmissionStatus = "CONSUMED"
)

type LateSubmissionReasonCode string

const (
	LateSubmissionReasonTechnicalFailure    LateSubmissionReasonCode = "TECHNICAL_FAILURE"
	LateSubmissionReasonOrganizationalDelay LateSubmissionReasonCode = "ORGANIZATIONAL_DELAY"
	LateSubmissionReasonBuyerRequest        LateSubmissionReasonCode = "BUYER_REQUEST"
	LateSubmissionReasonForceMajeure        LateSubmissionReasonCode = "FORCE_MAJEURE"
	LateSubmissionReasonOther               LateSubmissionReasonCode = "OTHER"
)

const (
	LateSubmissionOperationCreateRequest = "LATE_SUBMISSION_CREATE_REQUEST"
	LateSubmissionOperationApprove       = "LATE_SUBMISSION_APPROVE"
	LateSubmissionOperationReject        = "LATE_SUBMISSION_REJECT"
	LateSubmissionOperationSubmit        = "LATE_SUBMISSION_CARRIER_SUBMIT"
)

const (
	LateSubmissionAuditRequested = "rfx.late_submission.requested.v1"
	LateSubmissionAuditApproved  = "rfx.late_submission.approved.v1"
	LateSubmissionAuditRejected  = "rfx.late_submission.rejected.v1"
	LateSubmissionAuditExpired   = "rfx.late_submission.expired.v1"
	LateSubmissionAuditConsumed  = "rfx.late_submission.consumed.v1"
)

type LateSubmissionRequest struct {
	ID                 uuid.UUID
	TenantID           uuid.UUID
	RfxEventID         uuid.UUID
	CarrierCompanyID   uuid.UUID
	ParticipantID      *uuid.UUID
	ReasonCode         LateSubmissionReasonCode
	ReasonText         string
	RequestedUntil     time.Time
	Status             LateSubmissionStatus
	ApprovedValidFrom  *time.Time
	ApprovedValidUntil *time.Time
	DecisionComment    *string
	RequestedBy        uuid.UUID
	DecidedBy          *uuid.UUID
	DecidedAt          *time.Time
	ConsumedAt         *time.Time
	Version            int
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type CreateLateSubmissionRequestInput struct {
	ReasonCode     LateSubmissionReasonCode `json:"reason_code"`
	ReasonText     string                   `json:"reason_text"`
	RequestedUntil time.Time                `json:"requested_until"`
}

type ApproveLateSubmissionInput struct {
	ExpectedVersion    int       `json:"expected_version"`
	ApprovedValidFrom  time.Time `json:"approved_valid_from"`
	ApprovedValidUntil time.Time `json:"approved_valid_until"`
	DecisionComment    string    `json:"decision_comment"`
}

type RejectLateSubmissionInput struct {
	ExpectedVersion int    `json:"expected_version"`
	DecisionComment string `json:"decision_comment"`
}

func ParseLateSubmissionReasonCode(raw string) (LateSubmissionReasonCode, error) {
	code := LateSubmissionReasonCode(strings.ToUpper(strings.TrimSpace(raw)))
	switch code {
	case LateSubmissionReasonTechnicalFailure,
		LateSubmissionReasonOrganizationalDelay,
		LateSubmissionReasonBuyerRequest,
		LateSubmissionReasonForceMajeure,
		LateSubmissionReasonOther:
		return code, nil
	default:
		return "", apperrors.Validation("unknown late submission reason code", map[string]any{"field": "reason_code"})
	}
}

func ValidateCreateLateSubmissionInput(in CreateLateSubmissionRequestInput) error {
	if _, err := ParseLateSubmissionReasonCode(string(in.ReasonCode)); err != nil {
		return err
	}
	if strings.TrimSpace(in.ReasonText) == "" {
		return apperrors.Validation("reason text is required", map[string]any{"field": "reason_text"})
	}
	if in.RequestedUntil.IsZero() {
		return apperrors.Validation("requested_until is required", map[string]any{"field": "requested_until"})
	}
	return nil
}

func ResponseDeadlinePassed(deadline *time.Time, now time.Time) bool {
	if deadline == nil {
		return false
	}
	return !now.Before(deadline.UTC())
}

func ValidateLateRequestEligibility(deadline *time.Time, now time.Time) error {
	if !ResponseDeadlinePassed(deadline, now) {
		return apperrors.Validation("late submission request is allowed only after response deadline", map[string]any{"field": "response_deadline"})
	}
	return nil
}

func ValidateCarrierMutationDeadline(deadline *time.Time, now time.Time, permission *LateSubmissionRequest) error {
	if !ResponseDeadlinePassed(deadline, now) {
		return nil
	}
	if permission == nil || permission.Status != LateSubmissionStatusApproved {
		return apperrors.Unprocessable("carrier response mutation is not allowed after response deadline without approved late submission permission", map[string]any{"field": "response_deadline"})
	}
	if err := ValidateApprovedWindowActive(permission, now); err != nil {
		return err
	}
	return nil
}

func ValidateApprovedWindowInput(from, until time.Time) error {
	from = from.UTC()
	until = until.UTC()
	if !until.After(from) {
		return apperrors.Validation("approved_valid_until must be after approved_valid_from", map[string]any{"field": "approved_valid_until"})
	}
	return nil
}

func ValidateApprovedWindowActive(req *LateSubmissionRequest, now time.Time) error {
	if req == nil || req.Status != LateSubmissionStatusApproved {
		return apperrors.Unprocessable("approved late submission permission is required", map[string]any{"field": "late_submission_status"})
	}
	if req.ApprovedValidFrom == nil || req.ApprovedValidUntil == nil {
		return apperrors.Unprocessable("approved late submission window is incomplete", map[string]any{"field": "approved_valid_until"})
	}
	from := req.ApprovedValidFrom.UTC()
	until := req.ApprovedValidUntil.UTC()
	now = now.UTC()
	if now.Before(from) {
		return apperrors.Unprocessable("late submission window has not started yet", map[string]any{"field": "approved_valid_from"})
	}
	if !now.Before(until) {
		return apperrors.Unprocessable("late submission window has expired", map[string]any{"field": "approved_valid_until"})
	}
	return nil
}

func ValidateLateSubmissionTransition(current LateSubmissionStatus, next LateSubmissionStatus) error {
	switch current {
	case LateSubmissionStatusRequested:
		switch next {
		case LateSubmissionStatusApproved, LateSubmissionStatusRejected, LateSubmissionStatusExpired:
			return nil
		}
	case LateSubmissionStatusApproved:
		if next == LateSubmissionStatusConsumed {
			return nil
		}
	}
	return apperrors.Conflict("invalid late submission status transition", map[string]any{
		"field":   "status",
		"current": string(current),
		"next":    string(next),
	})
}

func IsLateSubmissionApprovedExpired(req *LateSubmissionRequest, now time.Time) bool {
	if req == nil || req.Status != LateSubmissionStatusApproved {
		return false
	}
	if req.ApprovedValidUntil == nil {
		return false
	}
	return !now.Before(req.ApprovedValidUntil.UTC())
}

func EffectiveLateSubmissionStatus(req *LateSubmissionRequest, now time.Time) LateSubmissionStatus {
	if req == nil {
		return ""
	}
	if IsLateSubmissionApprovedExpired(req, now) {
		return LateSubmissionStatusExpired
	}
	return req.Status
}

func CanCreateLateSubmissionRequest(existing *LateSubmissionRequest, now time.Time) error {
	if existing == nil {
		return nil
	}
	status := EffectiveLateSubmissionStatus(existing, now)
	switch status {
	case LateSubmissionStatusRequested, LateSubmissionStatusApproved:
		return apperrors.Conflict("active late submission request already exists", map[string]any{"field": "late_submission_request"})
	case LateSubmissionStatusRejected, LateSubmissionStatusExpired, LateSubmissionStatusConsumed:
		return nil
	default:
		return nil
	}
}
