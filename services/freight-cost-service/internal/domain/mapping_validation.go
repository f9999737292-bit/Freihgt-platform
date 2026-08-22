package domain

import (
	"fmt"
	"strings"
)

var allowedMappingCategories = map[string]struct{}{
	"DETENTION":  {},
	"FUEL":       {},
	"WAITING":    {},
	"LUMPER":     {},
	"ACCESSORIAL": {},
	"OTHER":      {},
}

func ValidateMappingCategory(category string) error {
	normalized := strings.ToUpper(strings.TrimSpace(category))
	if normalized == "" {
		return fmt.Errorf("target category required")
	}
	if _, ok := allowedMappingCategories[normalized]; !ok {
		return fmt.Errorf("invalid target category")
	}
	return nil
}

func NormalizeMappingCategory(category string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(category))
	if err := ValidateMappingCategory(normalized); err != nil {
		return "", err
	}
	return normalized, nil
}
