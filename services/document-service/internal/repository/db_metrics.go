package repository

// TODO: extend DB metrics validation to document-service (see docs/OBSERVABILITY.md).

import "github.com/freight-platform/shared-go/metrics"

const serviceName = "document-service"

func measureDB(repository, operation string, fn func() error) error {
	return metrics.MeasureDBQuery(serviceName, repository, operation, fn)
}
