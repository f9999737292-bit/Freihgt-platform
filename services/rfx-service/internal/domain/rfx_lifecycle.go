package domain

import (
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

const (
	RfxStatusInvitationSent       = "INVITATION_SENT"
	RfxStatusQuestionsOpen        = "QUESTIONS_OPEN"
	RfxStatusResponsesClosed      = "RESPONSES_CLOSED"
	RfxStatusEvaluationInProgress = "EVALUATION_IN_PROGRESS"
	RfxStatusShortlisted          = "SHORTLISTED"
	RfxStatusAwarded              = "AWARDED"
	RfxStatusPartiallyAwarded     = "PARTIALLY_AWARDED"
	RfxStatusArchived             = "ARCHIVED"
)

type LifecycleProfile string

const (
	LifecycleProfileSpot      LifecycleProfile = "SPOT"
	LifecycleProfileLongForm  LifecycleProfile = "LONG_FORM"
)

var spotRfxTypes = map[string]struct{}{
	"SPOT_RFQ": {}, "MINI_TENDER": {},
}

var longFormRfxTypes = map[string]struct{}{
	"RFI": {}, "RFQ": {}, "RFP": {}, "RFG": {}, "RFT": {},
	"LANE_TENDER": {}, "CONTRACT_TENDER": {}, "SEASONAL_TENDER": {},
	"PROJECT_TENDER": {}, "REVERSE_AUCTION": {},
}

func LifecycleProfileForType(rfxType string) LifecycleProfile {
	rfxType = strings.TrimSpace(rfxType)
	if _, ok := spotRfxTypes[rfxType]; ok {
		return LifecycleProfileSpot
	}
	if _, ok := longFormRfxTypes[rfxType]; ok {
		return LifecycleProfileLongForm
	}
	return LifecycleProfileLongForm
}

type RfxTransitionCommand string

const (
	RfxCommandPublish          RfxTransitionCommand = "publish"
	RfxCommandSendInvitations  RfxTransitionCommand = "send-invitations"
	RfxCommandOpenQuestions    RfxTransitionCommand = "open-questions"
	RfxCommandOpenResponses    RfxTransitionCommand = "open-responses"
	RfxCommandCloseResponses   RfxTransitionCommand = "close-responses"
	RfxCommandStartEvaluation  RfxTransitionCommand = "start-evaluation"
	RfxCommandShortlist        RfxTransitionCommand = "shortlist"
	RfxCommandAward            RfxTransitionCommand = "award"
	RfxCommandPartiallyAward   RfxTransitionCommand = "partially-award"
	RfxCommandArchive          RfxTransitionCommand = "archive"
	RfxCommandCancel           RfxTransitionCommand = "cancel"
	RfxCommandReopenResponses  RfxTransitionCommand = "reopen-responses"
)

var rfxTransitionTargets = map[RfxTransitionCommand]string{
	RfxCommandPublish:         RfxStatusPublished,
	RfxCommandSendInvitations: RfxStatusInvitationSent,
	RfxCommandOpenQuestions:   RfxStatusQuestionsOpen,
	RfxCommandOpenResponses:   RfxStatusResponsesOpen,
	RfxCommandCloseResponses:  RfxStatusResponsesClosed,
	RfxCommandStartEvaluation: RfxStatusEvaluationInProgress,
	RfxCommandShortlist:       RfxStatusShortlisted,
	RfxCommandAward:           RfxStatusAwarded,
	RfxCommandPartiallyAward:  RfxStatusPartiallyAwarded,
	RfxCommandArchive:         RfxStatusArchived,
	RfxCommandCancel:          RfxStatusCancelled,
	RfxCommandReopenResponses: RfxStatusResponsesOpen,
}

var spotAllowedTransitions = map[string]map[string]struct{}{
	RfxStatusDraft:                 {RfxStatusPublished: {}, RfxStatusCancelled: {}},
	RfxStatusPublished:             {RfxStatusResponsesOpen: {}, RfxStatusCancelled: {}},
	RfxStatusResponsesOpen:         {RfxStatusResponsesClosed: {}, RfxStatusCancelled: {}},
	RfxStatusResponsesClosed:       {RfxStatusEvaluationInProgress: {}},
	RfxStatusEvaluationInProgress:  {RfxStatusAwarded: {}, RfxStatusPartiallyAwarded: {}},
	RfxStatusAwarded:               {RfxStatusArchived: {}},
	RfxStatusPartiallyAwarded:      {RfxStatusArchived: {}},
	RfxStatusCancelled:             {},
	RfxStatusArchived:              {},
}

var longFormAllowedTransitions = map[string]map[string]struct{}{
	RfxStatusDraft:                 {RfxStatusPublished: {}, RfxStatusCancelled: {}},
	RfxStatusPublished:             {RfxStatusInvitationSent: {}, RfxStatusResponsesOpen: {}, RfxStatusCancelled: {}},
	RfxStatusInvitationSent:        {RfxStatusQuestionsOpen: {}, RfxStatusResponsesOpen: {}, RfxStatusCancelled: {}},
	RfxStatusQuestionsOpen:         {RfxStatusResponsesOpen: {}, RfxStatusCancelled: {}},
	RfxStatusResponsesOpen:         {RfxStatusResponsesClosed: {}, RfxStatusCancelled: {}},
	RfxStatusResponsesClosed:       {RfxStatusEvaluationInProgress: {}, RfxStatusResponsesOpen: {}},
	RfxStatusEvaluationInProgress:  {RfxStatusShortlisted: {}},
	RfxStatusShortlisted:           {RfxStatusAwarded: {}, RfxStatusPartiallyAwarded: {}},
	RfxStatusAwarded:               {RfxStatusArchived: {}},
	RfxStatusPartiallyAwarded:      {RfxStatusArchived: {}},
	RfxStatusCancelled:             {},
	RfxStatusArchived:              {},
}

func ResolveRfxTransitionTarget(profile LifecycleProfile, currentStatus string, command RfxTransitionCommand) (string, error) {
	target, ok := rfxTransitionTargets[command]
	if !ok {
		return "", apperrors.Validation("unsupported rfx transition command", map[string]any{"command": command})
	}
	if !CanTransitionRfxStatus(profile, currentStatus, target) {
		return "", apperrors.Conflict("invalid rfx status transition", map[string]any{
			"from":    currentStatus,
			"to":      target,
			"command": command,
		})
	}
	return target, nil
}

func CanTransitionRfxStatus(profile LifecycleProfile, from, to string) bool {
	table := longFormAllowedTransitions
	if profile == LifecycleProfileSpot {
		table = spotAllowedTransitions
	}
	targets, ok := table[from]
	if !ok {
		return false
	}
	_, ok = targets[to]
	return ok
}

func ValidateDeadlineExtensionStatus(status string) error {
	switch status {
	case RfxStatusDraft, RfxStatusPublished, RfxStatusInvitationSent, RfxStatusQuestionsOpen, RfxStatusResponsesOpen:
		return nil
	default:
		return apperrors.Conflict("response deadline cannot be changed in current status", map[string]any{"field": "status", "status": status})
	}
}

func ValidatePublishRfxEventWithLots(event *RfxEvent, lotCount int) error {
	if err := ValidatePublishRfxEvent(event.Status); err != nil {
		return err
	}
	if strings.TrimSpace(event.Title) == "" {
		return apperrors.Validation("title is required before publish", map[string]any{"field": "title"})
	}
	if event.OwnerCompanyID == uuid.Nil {
		return apperrors.Validation("owner_company_id is required before publish", map[string]any{"field": "owner_company_id"})
	}
	profile := LifecycleProfileForType(event.RfxType)
	if profile == LifecycleProfileLongForm && lotCount == 0 {
		return apperrors.Validation("at least one lot is required before publish for long-form tenders", map[string]any{"field": "lots"})
	}
	return nil
}

func ValidateCancelRfxEventExtended(status string) error {
	switch status {
	case RfxStatusDraft, RfxStatusPublished, RfxStatusInvitationSent, RfxStatusQuestionsOpen, RfxStatusResponsesOpen:
		return nil
	default:
		return apperrors.Conflict("rfx event cannot be cancelled in current status", map[string]any{"field": "status", "status": status})
	}
}
