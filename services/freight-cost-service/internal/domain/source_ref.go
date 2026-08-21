package domain

import "github.com/google/uuid"

const (
	SourceServiceTransportOrder = "transport-order-service"
	SourceTypeTORateSnapshot    = "TO_RATE_SNAPSHOT"
	PricingModelVersionSnapshot = "SNAPSHOT_V1"
)

type CanonicalSourceRef struct {
	SourceService       string
	SourceType          string
	SourceID            uuid.UUID
	SourceVersion       *int
	PricingModelVersion *string
}
