package xlsxexchange

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
)

type canonicalCreateEventShell struct {
	RfxNumber        string     `json:"rfx_number"`
	Title            string     `json:"title"`
	RfxType          string     `json:"rfx_type"`
	Category         string     `json:"category"`
	Description      string     `json:"description,omitempty"`
	ResponseDeadline *time.Time `json:"response_deadline,omitempty"`
	CurrencyCode     string     `json:"currency_code,omitempty"`
}

type canonicalCreateParserMarkers struct {
	ReadyToCommit       bool `json:"ready_to_commit"`
	FormulaDenied       bool `json:"formula_denied"`
	UnsafePackageDenied bool `json:"unsafe_package_denied"`
}

type canonicalCreatePayload struct {
	SchemaName     string                        `json:"schema_name"`
	SchemaVersion  string                        `json:"schema_version"`
	Mode           string                        `json:"mode"`
	OwnerCompanyID uuid.UUID                     `json:"owner_company_id"`
	EventShell     canonicalCreateEventShell     `json:"event_shell"`
	Lots           []canonicalLot                `json:"lots"`
	Questionnaire  canonicalQuestionnaire        `json:"questionnaire"`
	Counts         canonicalImportCounts         `json:"counts"`
	ParserMarkers  canonicalCreateParserMarkers  `json:"parser_markers"`
}

// StoredCreatePayload is the server-authoritative CREATE analysis payload.
type StoredCreatePayload struct {
	OwnerCompanyID uuid.UUID
	EventShell     canonicalCreateEventShell
	Lots           []canonicalLot
	Questionnaire  canonicalQuestionnaire
}

// BuyerCreateEventShell is the human-supplied event identity for CREATE_NEW_DRAFT.
type BuyerCreateEventShell struct {
	RfxNumber        string
	Title            string
	RfxType          string
	Category         string
	Description      *string
	ResponseDeadline *time.Time
	CurrencyCode     *string
	OwnerCompanyID   uuid.UUID
}

func CanonicalCreatePayloadJSON(
	shell BuyerCreateEventShell,
	preview BuyerCreatePreview,
) ([]byte, error) {
	payload, err := buildCanonicalCreatePayload(shell, preview)
	if err != nil {
		return nil, err
	}
	raw, err := marshalCanonical(payload)
	if err != nil {
		return nil, err
	}
	stored, _, err := StableStoredPayload(raw)
	return stored, err
}

func buildCanonicalCreatePayload(shell BuyerCreateEventShell, preview BuyerCreatePreview) (canonicalCreatePayload, error) {
	sections, questions, options := flattenCanonicalQuestionnaire(preview.Proposal.Questionnaire)
	lots := canonicalLots(preview.Proposal.Lots)
	return canonicalCreatePayload{
		SchemaName:     domain.SchemaVersionBuyerXLSXV1,
		SchemaVersion:  schemaVersionNumber,
		Mode:           BuyerImportModeCreateNewDraft,
		OwnerCompanyID: shell.OwnerCompanyID,
		EventShell: canonicalCreateEventShell{
			RfxNumber:        shell.RfxNumber,
			Title:            shell.Title,
			RfxType:          shell.RfxType,
			Category:         shell.Category,
			Description:      optionalString(shell.Description),
			ResponseDeadline: shell.ResponseDeadline,
			CurrencyCode:     optionalString(shell.CurrencyCode),
		},
		Lots: lots,
		Questionnaire: canonicalQuestionnaire{
			Sections:  sections,
			Questions: questions,
			Options:   options,
			Rules:     canonicalRules(preview.Proposal.Questionnaire.Rules, preview.Proposal.Questionnaire.Sections),
		},
		Counts: canonicalImportCounts{
			Errors:    len(preview.Errors),
			Warnings:  len(preview.Warnings),
			Lots:      len(preview.Proposal.Lots),
			Sections:  len(sections),
			Questions: len(questions),
			Options:   len(options),
			Rules:     len(preview.Proposal.Questionnaire.Rules),
		},
		ParserMarkers: canonicalCreateParserMarkers{
			ReadyToCommit:       preview.ReadyToCommit,
			FormulaDenied:       hasCreateMachineCode(preview.Errors, MachineCodeFormulaDenied),
			UnsafePackageDenied: hasCreateMachineCode(preview.Errors, MachineCodeUnsafePackage),
		},
	}, nil
}

func stableStoredCreatePayload(pgLike []byte) ([]byte, string, error) {
	var payload canonicalCreatePayload
	if err := json.Unmarshal(pgLike, &payload); err != nil {
		return nil, "", err
	}
	if err := normalizeCanonicalCreatePayload(&payload); err != nil {
		return nil, "", err
	}
	normalized, err := marshalCanonical(payload)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(normalized)
	return normalized, hex.EncodeToString(sum[:]), nil
}

func normalizeCanonicalCreatePayload(payload *canonicalCreatePayload) error {
	for idx := range payload.Questionnaire.Questions {
		normalized, err := normalizeJSONBytes(payload.Questionnaire.Questions[idx].ValidationJSON)
		if err != nil {
			return err
		}
		payload.Questionnaire.Questions[idx].ValidationJSON = normalized
	}
	for idx := range payload.Questionnaire.Rules {
		normalized, err := normalizeJSONBytes(payload.Questionnaire.Rules[idx].ConditionJSON)
		if err != nil {
			return err
		}
		payload.Questionnaire.Rules[idx].ConditionJSON = normalized
	}
	return nil
}

// ParseStoredCreatePayload decodes immutable CREATE analysis JSON.
func ParseStoredCreatePayload(payloadJSON []byte) (StoredCreatePayload, error) {
	stored, _, err := StableStoredPayload(payloadJSON)
	if err != nil {
		return StoredCreatePayload{}, err
	}
	var payload canonicalCreatePayload
	if err := json.Unmarshal(stored, &payload); err != nil {
		return StoredCreatePayload{}, err
	}
	if payload.Mode != BuyerImportModeCreateNewDraft {
		return StoredCreatePayload{}, fmt.Errorf("stored payload mode is not CREATE_NEW_DRAFT")
	}
	return StoredCreatePayload{
		OwnerCompanyID: payload.OwnerCompanyID,
		EventShell:     payload.EventShell,
		Lots:           payload.Lots,
		Questionnaire:  payload.Questionnaire,
	}, nil
}

func (s StoredCreatePayload) AsImportPayload() StoredImportPayload {
	return StoredImportPayload{
		Lots:          s.Lots,
		Questionnaire: s.Questionnaire,
	}
}

func ProposalFromStoredCreatePayload(stored StoredCreatePayload) BuyerImportProposal {
	return ProposalFromStoredPayload(stored.AsImportPayload(), TargetDraftBaseline{})
}

func EventShellToCreateInput(tenantID uuid.UUID, stored StoredCreatePayload) domain.CreateRfxEventInput {
	shell := stored.EventShell
	return domain.CreateRfxEventInput{
		TenantID:         tenantID,
		RfxNumber:        shell.RfxNumber,
		RfxType:          shell.RfxType,
		Category:         shell.Category,
		Title:            shell.Title,
		Description:      optionalStringPtr(shell.Description),
		OwnerCompanyID:   stored.OwnerCompanyID,
		CurrencyCode:     optionalStringPtr(shell.CurrencyCode),
		ResponseDeadline: shell.ResponseDeadline,
	}
}

func ValidateCreateCommitProposal(ctx context.Context, proposal BuyerImportProposal) []BuyerImportIssue {
	target := createParserValidationTarget()
	lots := append([]domain.RfxLot(nil), proposal.Lots...)
	for i := range lots {
		lots[i].TenantID = target.TenantID
		lots[i].RfxEventID = target.EventID
	}
	proposal.Lots = lots
	p := newBuyerImportParser(ctx, productionParseConfig())
	p.validateProposalGraph(target, proposal)
	return p.issues.errors
}

func hasCreateMachineCode(issues []BuyerImportIssue, code string) bool {
	for _, issue := range issues {
		if issue.MachineCode == code {
			return true
		}
	}
	return false
}
