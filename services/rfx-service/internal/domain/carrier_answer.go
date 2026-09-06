package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	CarrierResponseStatusNotStarted = "NOT_STARTED"
	CarrierResponseStatusInProgress = "IN_PROGRESS"
	CarrierResponseStatusSubmitted  = "SUBMITTED"

	AnswerSourceCarrierDeclared = "CARRIER_DECLARED"

	HiddenAnswerPolicy = "IGNORE_ON_SAVE"
)

type CarrierAnswer struct {
	ID                uuid.UUID
	TenantID          uuid.UUID
	RfxResponseID     uuid.UUID
	QuestionID        uuid.UUID
	AnswerValueJSON   json.RawMessage
	AnswerSource      string
	ValidationVersion int
	RuleVersion       *int
	UpdatedBy         *uuid.UUID
	UpdatedAt         time.Time
	Version           int
}

// RfxAnswer is the persisted carrier questionnaire answer entity.
type RfxAnswer = CarrierAnswer

type AnswerPatchItem struct {
	SectionID  uuid.UUID
	QuestionID uuid.UUID
	Field      string
	Value      json.RawMessage
}

// AnswerPatchInput is a single answer patch in a carrier autosave batch.
type AnswerPatchInput = AnswerPatchItem

type AnswerBatchPatchInput struct {
	ExpectedSaveVersion int64
	Answers             []AnswerPatchItem
}

type ResponseSaveResult struct {
	ResponseID        uuid.UUID
	SaveVersion       int64
	LastSavedAt       time.Time
	LastSavedBy       uuid.UUID
	CompletionPercent float64
	Warnings          []ValidationOutcome
}

type ResponseValidationResult struct {
	Valid              bool
	BlockingErrorCount int
	Errors             []ValidationErrorDetail
	Warnings           []ValidationOutcome
	CompletionPercent  float64
}

type ValidationErrorDetail struct {
	SectionID  uuid.UUID
	QuestionID uuid.UUID
	Field      string
	Rule       string
	MessageKey string
	Params     map[string]any
}

// ValidationErrorItem is the domain validation error payload for carrier responses.
type ValidationErrorItem = ValidationErrorDetail

type ValidationOutcome struct {
	Code       string
	MessageKey string
	Params     map[string]any
}

type ResponseConflict struct {
	ExpectedSaveVersion int64
	CurrentSaveVersion  int64
	LastSavedAt         *time.Time
}

type SubmitResult struct {
	ResponseID  uuid.UUID
	Status      string
	SubmittedAt time.Time
	SaveVersion int64
}

type CarrierResponseWorkspace struct {
	Response      RfxResponse
	Questionnaire QuestionnaireDefinition
	Answers       []CarrierAnswer
}

func MapResponseStatusToProduct(status string) string {
	switch status {
	case RfxResponseStatusDraft:
		return CarrierResponseStatusInProgress
	case RfxResponseStatusSubmitted:
		return CarrierResponseStatusSubmitted
	default:
		return status
	}
}

func ValidateUpdateQuestionnaireResponse(status string) error {
	return ValidateUpdateDraftResponse(status)
}
