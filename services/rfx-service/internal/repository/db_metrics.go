package repository

// TODO: extend DB metrics validation to rfx-service (see docs/OBSERVABILITY.md).

import "github.com/freight-platform/shared-go/metrics"

const serviceName = "rfx-service"

func measureDB(repository, operation string, fn func() error) error {
	return metrics.MeasureDBQuery(serviceName, repository, operation, fn)
}
