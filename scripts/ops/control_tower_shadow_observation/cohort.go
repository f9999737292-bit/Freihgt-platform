package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type CohortEntry struct {
	Alias    string `json:"alias"`
	TenantID string `json:"tenantId"`
	Email    string `json:"email,omitempty"`
	Password string `json:"password,omitempty"`
	Category string `json:"category,omitempty"`
}

type TenantBaseline struct {
	Alias              string `json:"alias"`
	Category           string `json:"category,omitempty"`
	PublicSource       string `json:"publicSource"`
	ComparisonCategory string `json:"comparisonCategory"`
	LegacyComplete     bool   `json:"legacyComplete"`
	LimitedDataset     bool   `json:"limitedDataset"`
	FallbackUsed       bool   `json:"fallbackUsed"`
	LegacyTotal        int64  `json:"legacyTotal"`
	ReadModelTotal     int64  `json:"readModelTotal"`
	IncompleteCount    int64  `json:"incompleteCount"`
}

func loadCohort(path string) ([]CohortEntry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read cohort manifest: %w", err)
	}
	var entries []CohortEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("parse cohort manifest: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("cohort manifest is empty")
	}
	for _, e := range entries {
		if e.Alias == "" || e.TenantID == "" {
			return nil, fmt.Errorf("cohort entry requires alias and tenantId")
		}
	}
	return entries, nil
}

func redactJWT(s string) string {
	if s == "" {
		return ""
	}
	return "[REDACTED]"
}

func redactTenantID(_ string) string {
	return "[REDACTED]"
}
