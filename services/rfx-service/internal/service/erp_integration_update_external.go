package service

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/erpjson"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
)

func linkCurrentRevision(link *domain.ExternalObjectLink) string {
	if link == nil {
		return ""
	}
	revision := strings.TrimSpace(link.ExternalRevision)
	if revision == "" {
		revision = strings.TrimSpace(link.ExternalVersion)
	}
	if revision == "" {
		return "1"
	}
	return revision
}

func normalizeUpdateExternalRef(ext *erpjson.ExternalRef) *erpjson.ExternalRef {
	if ext == nil {
		return nil
	}
	return &erpjson.ExternalRef{
		System:   strings.ToUpper(strings.TrimSpace(ext.System)),
		ObjectID: norm.NFC.String(strings.TrimSpace(ext.ObjectID)),
		Revision: strings.TrimSpace(ext.Revision),
	}
}

func externalRevisionValidation(message string) error {
	return apperrors.Validation(message, map[string]any{"field": "external.revision"})
}

func (s *ErpIntegrationService) loadEventExternalLink(
	ctx context.Context,
	actor IntegrationActor,
	eventID uuid.UUID,
) (*domain.ExternalObjectLink, error) {
	if s.linkRepo == nil {
		return nil, apperrors.Internal("erp update external link repository is not configured", nil)
	}
	link, err := s.linkRepo.GetByRfxEventID(ctx, actor.TenantID, actor.PrincipalID, eventID)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return link, nil
}

func (s *ErpIntegrationService) validateUpdatePreviewExternalPolicy(
	ctx context.Context,
	actor IntegrationActor,
	eventID uuid.UUID,
	parsed *erpjson.ParsedPreview,
) (*domain.ExternalObjectLink, string, error) {
	existing, err := s.loadEventExternalLink(ctx, actor, eventID)
	if err != nil {
		return nil, "", err
	}
	identity := normalizeUpdateExternalRef(parsed.External)

	if existing == nil {
		if identity == nil || (identity.System == "" && identity.ObjectID == "" && identity.Revision == "") {
			return nil, "", nil
		}
		if err := validateUpdateExternalIdentity(identity, false); err != nil {
			return nil, "", err
		}
		revision := identity.Revision
		if revision == "" {
			revision = "1"
		}
		if err := validateExternalRevisionValue(revision); err != nil {
			return nil, "", err
		}
		claimed, claimErr := s.linkRepo.GetByStableIdentity(
			ctx, actor.TenantID, actor.PrincipalID,
			identity.System, domain.ExternalObjectTypeRfxEvent, identity.ObjectID,
		)
		if claimErr != nil && !isNotFound(claimErr) {
			return nil, "", claimErr
		}
		if claimed != nil && claimed.RfxEventID != eventID {
			return nil, "", commitConflict(domain.MachineCodeExternalIDConflict, "stable external identity already exists")
		}
		if parsed.External != nil {
			parsed.External.System = identity.System
			parsed.External.ObjectID = identity.ObjectID
			parsed.External.Revision = revision
		}
		return nil, revision, nil
	}

	if identity == nil {
		return nil, "", externalRevisionValidation("external.revision is required when the event already has an external link")
	}
	if err := validateUpdateExternalIdentity(identity, true); err != nil {
		return nil, "", err
	}
	if identity.System != strings.TrimSpace(existing.ExternalSystem) {
		return nil, "", apperrors.Validation("external.system cannot change the stable identity of an existing link", map[string]any{"field": "external.system"})
	}
	if identity.ObjectID != strings.TrimSpace(existing.ExternalObjectID) {
		return nil, "", apperrors.Validation("external.object_id cannot change the stable identity of an existing link", map[string]any{"field": "external.object_id"})
	}
	if identity.Revision == "" {
		return nil, "", externalRevisionValidation("external.revision is required when the event already has an external link")
	}
	if err := validateExternalRevisionValue(identity.Revision); err != nil {
		return nil, "", err
	}
	current := linkCurrentRevision(existing)
	if identity.Revision == current {
		return nil, "", externalRevisionValidation("external.revision must differ from the current link revision")
	}
	used, err := s.linkRepo.RevisionExists(ctx, actor.TenantID, existing.ID, identity.Revision)
	if err != nil {
		return nil, "", err
	}
	if used {
		return nil, "", externalRevisionValidation("external.revision was already recorded for this external link")
	}
	if parsed.External != nil {
		parsed.External.System = identity.System
		parsed.External.ObjectID = identity.ObjectID
		parsed.External.Revision = identity.Revision
	}
	return existing, identity.Revision, nil
}

func validateUpdateExternalIdentity(identity *erpjson.ExternalRef, existingLink bool) error {
	if identity == nil {
		return externalRevisionValidation("external.revision is required when the event already has an external link")
	}
	suffix := "to bind an external identity"
	if existingLink {
		suffix = "when the event already has an external link"
	}
	if identity.System == "" || !validExternalSystem(identity.System) {
		return apperrors.Validation("external.system is required "+suffix, map[string]any{"field": "external.system"})
	}
	if identity.ObjectID == "" || utf8.RuneCountInString(identity.ObjectID) > 256 {
		return apperrors.Validation("external.object_id is required "+suffix, map[string]any{"field": "external.object_id"})
	}
	return nil
}

func validExternalSystem(system string) bool {
	if system == "" || len(system) > 64 {
		return false
	}
	for _, r := range system {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

func validateExternalRevisionValue(revision string) error {
	if revision == "" || utf8.RuneCountInString(revision) > 64 {
		return externalRevisionValidation("external.revision must be 1-64 characters")
	}
	return nil
}

func (s *ErpIntegrationService) assertUpdateExternalLinkBaseline(
	ctx context.Context,
	linkRepo *repository.ExternalObjectLinkRepository,
	actor IntegrationActor,
	eventID uuid.UUID,
	stored *erpjson.StoredUpdateCanonical,
) error {
	current, err := linkRepo.GetByRfxEventID(ctx, actor.TenantID, actor.PrincipalID, eventID)
	if err != nil && !isNotFound(err) {
		return err
	}
	expectedID := ""
	expectedRevision := ""
	if stored != nil {
		expectedID = strings.TrimSpace(stored.BaselineExternalLinkID)
		expectedRevision = strings.TrimSpace(stored.BaselineExternalRevision)
	}
	if expectedID == "" {
		if current != nil {
			return commitConflict(domain.MachineCodeStaleTarget, "external link changed after preview")
		}
		return nil
	}
	if current == nil || current.ID.String() != expectedID || linkCurrentRevision(current) != expectedRevision {
		return commitConflict(domain.MachineCodeStaleTarget, "external link revision changed after preview")
	}
	return nil
}

func (s *ErpIntegrationService) applyUpdateExternalLink(
	ctx context.Context,
	linkRepo *repository.ExternalObjectLinkRepository,
	actor IntegrationActor,
	eventID uuid.UUID,
	payloadHash string,
	stored *erpjson.StoredUpdateCanonical,
) error {
	var identity *erpjson.ExternalRef
	if stored != nil && stored.External != nil &&
		strings.TrimSpace(stored.External.System) != "" &&
		strings.TrimSpace(stored.External.ObjectID) != "" {
		identity = normalizeUpdateExternalRef(stored.External)
	}

	existing, err := linkRepo.GetByRfxEventID(ctx, actor.TenantID, actor.PrincipalID, eventID)
	if err != nil && !isNotFound(err) {
		return err
	}
	if existing == nil {
		if identity == nil {
			return nil
		}
		claimed, claimErr := linkRepo.GetByStableIdentity(
			ctx, actor.TenantID, actor.PrincipalID,
			identity.System, domain.ExternalObjectTypeRfxEvent, identity.ObjectID,
		)
		if claimErr != nil && !isNotFound(claimErr) {
			return claimErr
		}
		if claimed != nil {
			return commitConflict(domain.MachineCodeExternalIDConflict, "stable external identity already exists")
		}
		revision := identity.Revision
		if revision == "" {
			revision = "1"
		}
		created, insertErr := linkRepo.InsertLink(ctx, domain.ExternalObjectLink{
			TenantID:               actor.TenantID,
			IntegrationPrincipalID: actor.PrincipalID,
			ExternalSystem:         identity.System,
			ExternalObjectType:     domain.ExternalObjectTypeRfxEvent,
			ExternalObjectID:       identity.ObjectID,
			ExternalVersion:        revision,
			ExternalRevision:       revision,
			PayloadHash:            payloadHash,
			RfxEventID:             eventID,
		})
		if insertErr != nil {
			return insertErr
		}
		_, err = linkRepo.RecordRevision(ctx, domain.ExternalObjectLinkRevision{
			TenantID:         actor.TenantID,
			LinkID:           created.ID,
			ExternalRevision: revision,
			PayloadHash:      payloadHash,
		})
		return err
	}

	if identity == nil {
		return commitConflict(domain.MachineCodeStaleTarget, "existing external link requires a new revision")
	}
	if identity.System != strings.TrimSpace(existing.ExternalSystem) ||
		identity.ObjectID != strings.TrimSpace(existing.ExternalObjectID) {
		return commitConflict(domain.MachineCodeExternalIDConflict, "stable external identity already exists")
	}
	revision := identity.Revision
	if err := validateExternalRevisionValue(revision); err != nil {
		return err
	}
	current := linkCurrentRevision(existing)
	if revision == current {
		return externalRevisionValidation("external.revision must differ from the current link revision")
	}
	used, err := linkRepo.RevisionExists(ctx, actor.TenantID, existing.ID, revision)
	if err != nil {
		return err
	}
	if used {
		return externalRevisionValidation("external.revision was already recorded for this external link")
	}

	previousRecorded, err := linkRepo.RevisionExists(ctx, actor.TenantID, existing.ID, current)
	if err != nil {
		return err
	}
	if !previousRecorded {
		if _, err := linkRepo.RecordRevision(ctx, domain.ExternalObjectLinkRevision{
			TenantID:         actor.TenantID,
			LinkID:           existing.ID,
			ExternalRevision: current,
			PayloadHash:      existing.PayloadHash,
		}); err != nil {
			return err
		}
	}
	if _, err := linkRepo.RecordRevision(ctx, domain.ExternalObjectLinkRevision{
		TenantID:         actor.TenantID,
		LinkID:           existing.ID,
		ExternalRevision: revision,
		PayloadHash:      payloadHash,
	}); err != nil {
		return err
	}
	_, err = linkRepo.UpdateLinkMetadataInPlace(
		ctx, existing.ID, actor.TenantID, actor.PrincipalID, eventID, revision, payloadHash,
	)
	return err
}

func updatePreviewBaselineTokens(
	eventVersion, draftVersion int,
	lotsFingerprint string,
	existing *domain.ExternalObjectLink,
) erpjson.UpdateBaselineTokens {
	tokens := erpjson.UpdateBaselineTokens{
		EventRowVersion:         eventVersion,
		DraftRowVersion:         draftVersion,
		BaselineLotsFingerprint: lotsFingerprint,
	}
	if existing != nil && existing.ID != uuid.Nil {
		tokens.BaselineExternalLinkID = existing.ID.String()
		tokens.BaselineExternalRevision = linkCurrentRevision(existing)
	}
	return tokens
}
