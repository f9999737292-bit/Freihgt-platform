package bintranse2e

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const standSchemaVersion = 105

type standSchemaState struct {
	Version             int
	Dirty               bool
	TriggerCount        int
	ExtraLoginRoles     int
	RuntimeRolesNoLogin bool
}

func validateStandDatabaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("stand mode requires TEST_DATABASE_URL")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" {
		return "", errors.New("stand mode rejected TEST_DATABASE_URL")
	}
	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return "", errors.New("stand mode rejected TEST_DATABASE_URL scheme")
	}
	if parsed.Hostname() != "postgres" {
		return "", errors.New("stand mode requires TEST_DATABASE_URL host postgres")
	}
	if port := parsed.Port(); port != "" && port != "5432" {
		return "", errors.New("stand mode requires TEST_DATABASE_URL port 5432")
	}
	return raw, nil
}

func evaluateStandSchema(state standSchemaState) error {
	if state.Version != standSchemaVersion || state.Dirty {
		return fmt.Errorf("schema_migrations version=%d dirty=%t, want %d clean", state.Version, state.Dirty, standSchemaVersion)
	}
	if state.TriggerCount != 0 {
		return errors.New("trg_new_activation_authorization is present")
	}
	if state.ExtraLoginRoles != 0 {
		return errors.New("stand database has a LOGIN role other than the ephemeral owner")
	}
	if !state.RuntimeRolesNoLogin {
		return errors.New("freight_migrations, company_runtime, and shipment_runtime must be NOLOGIN")
	}
	return nil
}
