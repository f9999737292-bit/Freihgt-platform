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
)
