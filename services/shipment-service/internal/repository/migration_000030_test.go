package repository

import (
	"strings"
	"testing"
)

func TestMigration000030DriverMobilePlatform(t *testing.T) {
	up := strings.ToLower(readMigration(t, "000030_add_driver_mobile_platform_v0.1.up.sql"))
	if !strings.Contains(up, "transport.driver_operation_idempotency") {
		t.Fatal("migration must create driver_operation_idempotency")
	}
	if !strings.Contains(up, "transport.driver_reported_exception") {
		t.Fatal("migration must create driver_reported_exception")
	}
	if !strings.Contains(up, "uq_drivers_tenant_user_active") {
		t.Fatal("migration must enforce tenant user driver binding")
	}
}
