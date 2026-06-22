package repository

import "github.com/freight-platform/shared-go/metrics"

const serviceName = "low-code-service"

func measureDB(repository, operation string, fn func() error) error {
	return metrics.MeasureDBQuery(serviceName, repository, operation, fn)
}
