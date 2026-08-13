package controltower

import "time"

type ControlTowerShipment struct {
	ID                   string     `json:"id"`
	ShipmentNumber       string     `json:"shipmentNumber"`
	TransportOrderID     *string    `json:"transportOrderId,omitempty"`
	TransportOrderNumber *string    `json:"transportOrderNumber,omitempty"`
	ShipperID            *string    `json:"shipperId,omitempty"`
	ShipperName          *string    `json:"shipperName,omitempty"`
	CarrierID            *string    `json:"carrierId,omitempty"`
	CarrierName          *string    `json:"carrierName,omitempty"`
	OriginID             *string    `json:"originId,omitempty"`
	OriginName           *string    `json:"originName,omitempty"`
	DestinationID        *string    `json:"destinationId,omitempty"`
	DestinationName      *string    `json:"destinationName,omitempty"`
	PlannedPickupAt      *time.Time `json:"plannedPickupAt,omitempty"`
	ActualPickupAt       *time.Time `json:"actualPickupAt,omitempty"`
	PlannedDeliveryAt    *time.Time `json:"plannedDeliveryAt,omitempty"`
	ActualDeliveryAt     *time.Time `json:"actualDeliveryAt,omitempty"`
	Status               string     `json:"status"`
	SLAStatus            SLAStatus  `json:"slaStatus"`
	SLAReason            *string    `json:"slaReason,omitempty"`
	DelayMinutes         *int64     `json:"delayMinutes,omitempty"`
	LastEvent            *string    `json:"lastEvent,omitempty"`
	LastUpdatedAt        *time.Time `json:"lastUpdatedAt,omitempty"`
	DocumentsComplete    bool       `json:"documentsComplete"`
	ReadyForBilling      bool       `json:"readyForBilling"`
}

type ControlTowerEvent struct {
	ID              string                            `json:"id"`
	ShipmentID      string                            `json:"shipmentId"`
	ShipmentNumber  string                            `json:"shipmentNumber"`
	Type            string                            `json:"type"`
	Severity        string                            `json:"severity"`
	OccurredAt      time.Time                         `json:"occurredAt"`
	Description     *string                           `json:"description,omitempty"`
	Source          string                            `json:"source"`
	Acknowledgement *ControlTowerEventAckSummary      `json:"acknowledgement,omitempty"`
}

type ControlTowerEventAckSummary struct {
	AcknowledgedAt time.Time                    `json:"acknowledgedAt"`
	AcknowledgedBy ControlTowerEventAcknowledgedBy `json:"acknowledgedBy"`
}

type ControlTowerEventAcknowledgedBy struct {
	UserID      string  `json:"userId"`
	DisplayName *string `json:"displayName,omitempty"`
}

type ControlTowerEventAcknowledgement struct {
	EventID        string                          `json:"eventId"`
	ShipmentID     string                          `json:"shipmentId"`
	EventType      string                          `json:"eventType"`
	OccurredAt     time.Time                       `json:"occurredAt"`
	Source         string                          `json:"source"`
	AcknowledgedAt time.Time                       `json:"acknowledgedAt"`
	AcknowledgedBy ControlTowerEventAcknowledgedBy `json:"acknowledgedBy"`
}

const (
	EventTypePickupDelay       = "PICKUP_DELAY"
	EventTypeDeliveryDelay     = "DELIVERY_DELAY"
	EventTypeStaleUpdates      = "STALE_UPDATES"
	EventTypeMissingDocuments  = "MISSING_DOCUMENTS"
	EventTypeShipmentCancelled = "SHIPMENT_CANCELLED"
	EventTypeTechnicalProblem  = "TECHNICAL_PROBLEM"
	EventTypeUnknownCritical   = "UNKNOWN_CRITICAL_EVENT"
	EventSeverityInfo          = "INFO"
	EventSeverityWarning       = "WARNING"
	EventSeverityCritical      = "CRITICAL"
	EventSourceControlTower    = "control-tower"
)

type DataFreshness struct {
	ShipmentsLoaded       bool     `json:"shipmentsLoaded"`
	TransportOrdersLoaded bool     `json:"transportOrdersLoaded"`
	CompaniesLoaded       bool     `json:"companiesLoaded"`
	DocumentsLoaded       bool     `json:"documentsLoaded"`
	Partial               bool     `json:"partial"`
	Warnings              []string `json:"warnings"`
}

type KPI struct {
	Active            int `json:"active"`
	OnTime            int `json:"onTime"`
	AtRisk            int `json:"atRisk"`
	Delayed           int `json:"delayed"`
	Critical          int `json:"critical"`
	AwaitingDocuments int `json:"awaitingDocuments"`
	ReadyForBilling   int `json:"readyForBilling"`
}

type ShipmentsPage struct {
	Items   []ControlTowerShipment `json:"items"`
	Page    int                    `json:"page"`
	Limit   int                    `json:"limit"`
	Total   int                    `json:"total"`
	HasNext bool                   `json:"hasNext"`
}

type FilterOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type FiltersResponse struct {
	Statuses []FilterOption `json:"statuses"`
	Shippers []FilterOption `json:"shippers"`
	Carriers []FilterOption `json:"carriers"`
}

type SummaryResponse struct {
	GeneratedAt            time.Time                    `json:"generatedAt"`
	DataFreshness          DataFreshness                `json:"dataFreshness"`
	KPI                    KPI                          `json:"kpi"`
	Shipments              ShipmentsPage                `json:"shipments"`
	CriticalEvents         []ControlTowerEvent          `json:"criticalEvents"`
	Filters                FiltersResponse              `json:"filters"`
	StatusSummary          *StatusSummaryBlock          `json:"statusSummary,omitempty"`
	StatusSummaryFreshness *StatusSummaryFreshnessBlock `json:"statusSummaryFreshness,omitempty"`
}

type StatusSummaryBlock struct {
	TotalShipments        int64            `json:"totalShipments"`
	CountedShipments      int64            `json:"countedShipments,omitempty"`
	ByStatus              map[string]int64 `json:"byStatus"`
	IncompleteProjections int64            `json:"incompleteProjections"`
	Source                string           `json:"source"`
	LimitedDataset        bool             `json:"limitedDataset,omitempty"`
}

type StatusSummaryFreshnessBlock struct {
	Loaded                  bool    `json:"loaded"`
	FallbackUsed            bool    `json:"fallbackUsed"`
	Partial                 bool    `json:"partial"`
	Source                  string  `json:"source,omitempty"`
	ConsumerRunning         *bool   `json:"consumerRunning,omitempty"`
	LastRecordReceivedAt    *string `json:"lastRecordReceivedAt,omitempty"`
	LastProjectionAppliedAt *string `json:"lastProjectionAppliedAt,omitempty"`
	LegacyAggregateLoaded   *bool   `json:"legacyAggregateLoaded,omitempty"`
}

const (
	WarningReadModelUnavailable        = "CONTROL_TOWER_READ_MODEL_UNAVAILABLE"
	WarningReadModelConsumerNotRunning = "CONTROL_TOWER_READ_MODEL_CONSUMER_NOT_RUNNING"
	WarningReadModelPartial            = "CONTROL_TOWER_READ_MODEL_PARTIAL"
	WarningReadModelFallbackUsed       = "CONTROL_TOWER_READ_MODEL_FALLBACK_USED"
	WarningLegacyStatusSummaryLimited  = "CONTROL_TOWER_LEGACY_STATUS_SUMMARY_LIMITED"
	StatusSummarySourceLegacy          = "LEGACY"
	StatusSummarySourceReadModel       = "READ_MODEL"
)

const (
	WarningTransportOrdersUnavailable = "TRANSPORT_ORDERS_UNAVAILABLE"
	WarningCompaniesUnavailable       = "COMPANIES_UNAVAILABLE"
	WarningDocumentsUnavailable       = "DOCUMENTS_UNAVAILABLE"
	WarningKPILimitedDataset          = "KPI_CALCULATED_FROM_LIMITED_DATASET"
	WarningFilterOptionsIncomplete    = "FILTER_OPTIONS_INCOMPLETE"
)

type ListQuery struct {
	Q            string
	Status       string
	SLAStatus    string
	ShipperID    string
	CarrierID    string
	DateFrom     *time.Time
	DateTo       *time.Time
	CriticalOnly bool
	Page         int
	Limit        int
}

type RequestContext struct {
	TenantID  string
	UserID    string
	AuthToken string
	RequestID string
}

type rawShipment struct {
	ID                    string
	ShipmentNumber        string
	TransportOrderID      *string
	ShipperCompanyID      string
	CarrierCompanyID      *string
	OriginLocationID      string
	DestinationLocationID string
	Status                string
	PlannedPickupAt       *time.Time
	PlannedDeliveryAt     *time.Time
	ActualPickupAt        *time.Time
	ActualDeliveryAt      *time.Time
	UpdatedAt             *time.Time
	CreatedAt             *time.Time
}

type rawTransportOrder struct {
	ID          string
	OrderNumber string
}

type rawCompany struct {
	ID          string
	LegalName   string
	ShortName   *string
	CompanyType string
}

type rawDocument struct {
	RelatedEntityType *string
	RelatedEntityID   *string
	DocumentStatus    string
}
