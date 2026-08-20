package domain

const (
	ContractStatusDraft      = "DRAFT"
	ContractStatusActive     = "ACTIVE"
	ContractStatusSuspended  = "SUSPENDED"
	ContractStatusTerminated = "TERMINATED"
	ContractStatusExpired    = "EXPIRED"
	ContractStatusCancelled  = "CANCELLED"
)

const (
	RateVersionStatusDraft      = "DRAFT"
	RateVersionStatusActive     = "ACTIVE"
	RateVersionStatusSuperseded = "SUPERSEDED"
)

const (
	AuditEntityContract    = "TRANSPORT_CONTRACT"
	AuditEntityRateCard    = "RATE_CARD"
	AuditEntityRateVersion = "RATE_CARD_VERSION"

	AuditActionContractCreated      = "CONTRACT_CREATED"
	AuditActionContractUpdated      = "CONTRACT_UPDATED"
	AuditActionContractActivated    = "CONTRACT_ACTIVATED"
	AuditActionContractSuspended    = "CONTRACT_SUSPENDED"
	AuditActionContractReactivated  = "CONTRACT_REACTIVATED"
	AuditActionContractTerminated   = "CONTRACT_TERMINATED"
	AuditActionContractCancelled    = "CONTRACT_CANCELLED"
	AuditActionContractExpired      = "CONTRACT_EXPIRED"
	AuditActionRateCardCreated      = "RATE_CARD_CREATED"
	AuditActionRateCardUpdated      = "RATE_CARD_UPDATED"
	AuditActionRateVersionCreated   = "RATE_VERSION_DRAFT_CREATED"
	AuditActionRateVersionUpdated   = "RATE_VERSION_DRAFT_UPDATED"
	AuditActionRateVersionDiscarded = "RATE_VERSION_DRAFT_DISCARDED"
	AuditActionRateVersionActivated = "RATE_VERSION_ACTIVATED"
	AuditActionRateVersionSuperseded = "RATE_VERSION_SUPERSEDED"

	AuditEntityRateLine      = "RATE_LINE"
	AuditEntityRateComponent = "RATE_COMPONENT"

	AuditActionRateLineCreated   = "RATE_LINE_CREATED"
	AuditActionRateLineUpdated   = "RATE_LINE_UPDATED"
	AuditActionRateLineDeleted   = "RATE_LINE_DELETED"
	AuditActionRateComponentCreated = "RATE_COMPONENT_CREATED"
	AuditActionRateComponentUpdated = "RATE_COMPONENT_UPDATED"
	AuditActionRateComponentDeleted = "RATE_COMPONENT_DELETED"
	AuditActionManualSpotResolved   = "MANUAL_SPOT_RESOLVED"
)
