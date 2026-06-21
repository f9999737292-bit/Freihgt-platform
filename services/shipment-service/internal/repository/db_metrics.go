package repository

import "github.com/freight-platform/shared-go/metrics"

const serviceName = "shipment-service"

func measureDB(repository, operation string, fn func() error) error {
	return metrics.MeasureDBQuery(serviceName, repository, operation, fn)
}
