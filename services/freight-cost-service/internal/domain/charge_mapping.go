package domain

import (
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	MappingScopePlatform = "PLATFORM"
	MappingScopeTenant   = "TENANT"
	CategoryOther        = "OTHER"
)

const MaxChargeCodeLength = 50

func NormalizeChargeCode(raw string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(raw))
	if normalized == "" {
		return "", ErrInvalidChargeCode
	}
	if utf8.RuneCountInString(normalized) > MaxChargeCodeLength {
		return "", ErrInvalidChargeCode
	}
	return normalized, nil
}

var ErrInvalidChargeCode = fmtError("invalid charge_code")

type fmtError string

func (e fmtError) Error() string { return string(e) }

type ChargeCodeMapping struct {
	MappingScope               string
	TenantID                   *uuid.UUID
	SourceChargeCodeNormalized string
	NormalizedCategory         string
	MappingVersion             int64
}

func ResolveChargeCategory(sourceCode string, platform, tenant []ChargeCodeMapping) string {
	normalized, err := NormalizeChargeCode(sourceCode)
	if err != nil {
		return CategoryOther
	}
	for _, m := range tenant {
		if m.SourceChargeCodeNormalized == normalized {
			return m.NormalizedCategory
		}
	}
	for _, m := range platform {
		if m.SourceChargeCodeNormalized == normalized {
			return m.NormalizedCategory
		}
	}
	return CategoryOther
}
