package shipmentevents

import "time"

const (
	CategoryShipment    = "SHIPMENT"
	CategoryOperation   = "OPERATION"
	CategoryDocument    = "DOCUMENT"
	CategorySLA         = "SLA"
	CategoryBilling     = "BILLING"
	CategoryTechnical   = "TECHNICAL"
	CategoryGeolocation = "GEOLOCATION"
	CategorySystem      = "SYSTEM"
)

const (
	SourceShipmentState = "SHIPMENT_STATE"
	SourceSLACalculator = "SLA_CALCULATOR"
	SourceDocumentState = "DOCUMENT_STATE"
	SourceBillingState  = "BILLING_STATE"
)

const (
	SeverityInfo     = "INFO"
	SeverityWarning  = "WARNING"
	SeverityCritical = "CRITICAL"
)

const (
	EventTypeShipmentCreated         = "SHIPMENT_CREATED"
	EventTypeShipmentStatusChanged   = "SHIPMENT_STATUS_CHANGED"
	EventTypeShipmentCancelled       = "SHIPMENT_CANCELLED"
	EventTypePickupPlanned           = "PICKUP_PLANNED"
	EventTypePickupCompleted         = "PICKUP_COMPLETED"
	EventTypeDeliveryPlanned         = "DELIVERY_PLANNED"
	EventTypeDeliveryCompleted       = "DELIVERY_COMPLETED"
	EventTypeDocumentCreated         = "DOCUMENT_CREATED"
	EventTypeDocumentSigned          = "DOCUMENT_SIGNED"
	EventTypeDocumentRejected        = "DOCUMENT_REJECTED"
	EventTypeDocumentsCompleted      = "DOCUMENTS_COMPLETED"
	EventTypeDocumentsMissing        = "DOCUMENTS_MISSING"
	EventTypeReadyForBilling         = "READY_FOR_BILLING"
	EventTypeBillingRegisterAdded    = "BILLING_REGISTER_ADDED"
	EventTypeClosingDocumentsCreated = "CLOSING_DOCUMENTS_CREATED"
	EventTypeSignedByCounterparty    = "SIGNED_BY_COUNTERPARTY"
	EventTypePaymentMarked           = "PAYMENT_MARKED"
	EventTypeFinanciallyClosed       = "FINANCIALLY_CLOSED"
	EventTypeSLAAtRisk               = "SLA_AT_RISK"
	EventTypeSLADelayed              = "SLA_DELAYED"
	EventTypeSLACritical             = "SLA_CRITICAL"
	EventTypeTechnicalProblem        = "TECHNICAL_PROBLEM"
	EventTypeGeolocationLost         = "GEOLOCATION_LOST"
	EventTypeRouteDeviation          = "ROUTE_DEVIATION"
	EventTypeUnknownEvent            = "UNKNOWN_EVENT"
)

const (
	WarningShipmentEventsUnavailable    = "SHIPMENT_EVENTS_UNAVAILABLE"
	WarningShipmentHistoryDerived       = "SHIPMENT_HISTORY_DERIVED_FROM_CURRENT_STATE"
	WarningDocumentEventsUnavailable    = "DOCUMENT_EVENTS_UNAVAILABLE"
	WarningBillingEventsUnavailable     = "BILLING_EVENTS_UNAVAILABLE"
	WarningTechnicalEventsUnavailable   = "TECHNICAL_EVENTS_UNAVAILABLE"
	WarningGeolocationEventsUnavailable = "GEOLOCATION_EVENTS_UNAVAILABLE"
	WarningTimelineLimitedDataset       = "TIMELINE_CALCULATED_FROM_LIMITED_DATASET"
	WarningFilterOptionsIncomplete      = "FILTER_OPTIONS_INCOMPLETE"
)

type ShipmentEventActor struct {
	Type string  `json:"type"`
	ID   *string `json:"id,omitempty"`
	Name *string `json:"name,omitempty"`
}

type ShipmentTimelineEvent struct {
	ID              string                 `json:"id"`
	ShipmentID      string                 `json:"shipmentId"`
	ShipmentNumber  string                 `json:"shipmentNumber"`
	Type            string                 `json:"type"`
	Category        string                 `json:"category"`
	Source          string                 `json:"source"`
	Severity        string                 `json:"severity"`
	TitleCode       string                 `json:"titleCode"`
	DescriptionCode *string                `json:"descriptionCode,omitempty"`
	OccurredAt      time.Time              `json:"occurredAt"`
	RecordedAt      *time.Time             `json:"recordedAt,omitempty"`
	Actor           *ShipmentEventActor    `json:"actor,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	Derived         bool                   `json:"derived"`
	CorrelationID   *string                `json:"correlationId,omitempty"`
	SourceEventID   *string                `json:"sourceEventId,omitempty"`
}

type ShipmentSummary struct {
	ID     string `json:"id"`
	Number string `json:"number"`
	Status string `json:"status"`
}

type DataFreshness struct {
	ShipmentLoaded        bool     `json:"shipmentLoaded"`
	ShipmentEventsLoaded  bool     `json:"shipmentEventsLoaded"`
	DocumentsLoaded       bool     `json:"documentsLoaded"`
	BillingLoaded         bool     `json:"billingLoaded"`
	TechnicalEventsLoaded bool     `json:"technicalEventsLoaded"`
	Partial               bool     `json:"partial"`
	Warnings              []string `json:"warnings"`
}

type TimelinePage struct {
	Items   []ShipmentTimelineEvent `json:"items"`
	Page    int                     `json:"page"`
	Limit   int                     `json:"limit"`
	Total   int                     `json:"total"`
	HasNext bool                    `json:"hasNext"`
}

type FilterOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type FiltersResponse struct {
	Types      []FilterOption `json:"types"`
	Categories []FilterOption `json:"categories"`
	Sources    []FilterOption `json:"sources"`
	Severities []FilterOption `json:"severities"`
}

type EventsResponse struct {
	Shipment      ShipmentSummary `json:"shipment"`
	GeneratedAt   time.Time       `json:"generatedAt"`
	DataFreshness DataFreshness   `json:"dataFreshness"`
	Timeline      TimelinePage    `json:"timeline"`
	Filters       FiltersResponse `json:"filters"`
}

type ListQuery struct {
	Type     string
	Category string
	Source   string
	Severity string
	DateFrom *time.Time
	DateTo   *time.Time
	Derived  *bool
	Order    string
	Page     int
	Limit    int
}

type RequestContext struct {
	TenantID  string
	UserID    string
	AuthToken string
	RequestID string
}

type rawShipment struct {
	ID                string
	TenantID          string
	ShipmentNumber    string
	Status            string
	PlannedPickupAt   *time.Time
	PlannedDeliveryAt *time.Time
	ActualPickupAt    *time.Time
	ActualDeliveryAt  *time.Time
	TechnicalProblem  bool
	CreatedAt         *time.Time
	UpdatedAt         *time.Time
}

type rawDocument struct {
	ID             string
	DocumentType   string
	DocumentStatus string
	CreatedAt      *time.Time
	SignedAt       *time.Time
	RejectedAt     *time.Time
}

type rawBillingItem struct {
	ID         string
	RegisterID string
	ShipmentID string
	CreatedAt  *time.Time
}

func titleCode(eventType string) string {
	return "shipment.timeline." + eventType + ".title"
}

func descriptionCode(eventType string) string {
	code := "shipment.timeline." + eventType + ".description"
	return code
}

func categoryForType(eventType string) string {
	switch eventType {
	case EventTypeDocumentCreated, EventTypeDocumentSigned, EventTypeDocumentRejected,
		EventTypeDocumentsCompleted, EventTypeDocumentsMissing:
		return CategoryDocument
	case EventTypeReadyForBilling, EventTypeBillingRegisterAdded, EventTypeClosingDocumentsCreated,
		EventTypeSignedByCounterparty, EventTypePaymentMarked, EventTypeFinanciallyClosed:
		return CategoryBilling
	case EventTypeSLAAtRisk, EventTypeSLADelayed, EventTypeSLACritical:
		return CategorySLA
	case EventTypePickupPlanned, EventTypePickupCompleted, EventTypeDeliveryPlanned, EventTypeDeliveryCompleted:
		return CategoryOperation
	case EventTypeTechnicalProblem:
		return CategoryTechnical
	case EventTypeGeolocationLost, EventTypeRouteDeviation:
		return CategoryGeolocation
	default:
		return CategoryShipment
	}
}

func severityForType(eventType string) string {
	switch eventType {
	case EventTypeSLACritical, EventTypeTechnicalProblem, EventTypeShipmentCancelled:
		return SeverityCritical
	case EventTypeSLADelayed, EventTypeSLAAtRisk, EventTypeDocumentsMissing, EventTypeDocumentRejected:
		return SeverityWarning
	default:
		return SeverityInfo
	}
}
