package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type CohortManifestFile struct {
	Environment string        `json:"environment,omitempty"`
	Tenants     []CohortEntry `json:"tenants,omitempty"`
}

type CohortEntry struct {
	Alias                 string `json:"alias"`
	TenantID              string `json:"tenantId,omitempty"`
	TenantIDFromSecretRef string `json:"tenantIdFromSecretRef,omitempty"`
	Email                 string `json:"email,omitempty"`
	Password              string `json:"password,omitempty"`
	EmailFromSecretRef    string `json:"emailFromSecretRef,omitempty"`
	PasswordFromSecretRef string `json:"passwordFromSecretRef,omitempty"`
	Category              string `json:"category,omitempty"`
	Approved              *bool  `json:"approved,omitempty"`
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

func (e CohortEntry) resolveTenantID() (string, error) {
	if v := stringsTrim(e.TenantID); v != "" {
		return v, nil
	}
	if ref := stringsTrim(e.TenantIDFromSecretRef); ref != "" {
		v := stringsTrim(os.Getenv(ref))
		if v == "" {
			return "", fmt.Errorf("alias %s: env %s is not set", e.Alias, ref)
		}
		return v, nil
	}
	return "", fmt.Errorf("alias %s: tenantId or tenantIdFromSecretRef required", e.Alias)
}

func (e CohortEntry) resolveEmail() string {
	if v := stringsTrim(e.Email); v != "" {
		return v
	}
	if ref := stringsTrim(e.EmailFromSecretRef); ref != "" {
		return stringsTrim(os.Getenv(ref))
	}
	return ""
}

func (e CohortEntry) resolvePassword() string {
	if v := stringsTrim(e.Password); v != "" {
		return v
	}
	if ref := stringsTrim(e.PasswordFromSecretRef); ref != "" {
		return stringsTrim(os.Getenv(ref))
	}
	return ""
}

func (e CohortEntry) isApproved() bool {
	if e.Approved == nil {
		return true
	}
	return *e.Approved
}

func stringsTrim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\n') {
		s = s[:len(s)-1]
	}
	return s
}

func loadCohort(path string) ([]CohortEntry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read cohort manifest: %w", err)
	}
	var wrapped CohortManifestFile
	if err := json.Unmarshal(raw, &wrapped); err == nil && len(wrapped.Tenants) > 0 {
		return validateCohortEntries(wrapped.Tenants)
	}
	var entries []CohortEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("parse cohort manifest: %w", err)
	}
	return validateCohortEntries(entries)
}

func validateCohortEntries(entries []CohortEntry) ([]CohortEntry, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("cohort manifest is empty")
	}
	out := make([]CohortEntry, 0, len(entries))
	for _, e := range entries {
		if e.Alias == "" {
			return nil, fmt.Errorf("cohort entry requires alias")
		}
		if !e.isApproved() {
			continue
		}
		if _, err := e.resolveTenantID(); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("cohort manifest has no approved tenants")
	}
	return out, nil
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
