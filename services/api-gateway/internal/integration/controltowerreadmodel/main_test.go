//go:build integration

package controltowerreadmodelintegration

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if os.Getenv("TEST_DATABASE_URL") == "" {
		os.Exit(m.Run())
	}
	if err := warmIntegrationBinaries(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func warmIntegrationBinaries() error {
	if _, err := buildServiceBinaryOnceCached("services/shipment-service", shipmentServiceBinaryKey); err != nil {
		return err
	}
	_, err := buildServiceBinaryOnceCached("services/control-tower-read-model-service", readModelServiceBinaryKey)
	return err
}
