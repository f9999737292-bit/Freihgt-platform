package domain

import "github.com/google/uuid"

// RfxEventProvenance describes clone origin when an event was created from a template (E5/E6).
type RfxEventProvenance struct {
	SourceTemplateID        uuid.UUID
	SourceTemplateVersionID uuid.UUID
	SourceVersionNumber     int
	SourceVersionStatus     string
	TemplateCode            string
	NameI18nJSON            []byte
	SourceVersionWarning    bool
	TemplateOwnerCompanyID  uuid.UUID
}
