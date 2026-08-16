package controltower

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/freight-platform/api-gateway/internal/config"
	"github.com/freight-platform/api-gateway/internal/controltower/legacyaggregate"
	"github.com/freight-platform/api-gateway/internal/controltowerreadmodel"
	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/tracking"
)

type Service struct {
	client                 *DownstreamClient
	thresholds             SLAThresholds
	readModel              *controltowerreadmodel.Client
	readModelCfg           controltowerreadmodel.Config
	readModelLog           *slog.Logger
	metrics                *controltowerreadmodel.Metrics
	legacyAggregate        *legacyaggregate.Client
	legacyAggregateTimeout time.Duration
	legacyMetrics          *legacyaggregate.Metrics
	legacyLog              *slog.Logger
	trackingClient         *tracking.Client
}

func NewService(cfg config.Config, client *DownstreamClient, log *slog.Logger) *Service {
	metrics := controltowerreadmodel.NewMetrics()
	legacyMetrics := legacyaggregate.NewMetrics()

	var readModelClient *controltowerreadmodel.Client
	if cfg.ControlTower.ReadModel.Mode.Enabled() {
		readModelClient = controltowerreadmodel.NewClient(
			&http.Client{Timeout: cfg.ControlTower.ReadModel.Timeout},
			cfg.ControlTower.ReadModel,
			metrics,
		)
	}

	legacyTimeout := cfg.ControlTower.LegacyStatusTimeout
	legacyClient := legacyaggregate.NewClient(
		&http.Client{Timeout: legacyTimeout},
		legacyaggregate.Config{
			BaseURL:          cfg.Services.Shipment,
			Timeout:          legacyTimeout,
			MaxResponseBytes: 256 * 1024,
		},
		legacyMetrics,
	)

	return &Service{
		client: client,
		thresholds: SLAThresholds{
			AtRiskMinutes:        cfg.ControlTower.AtRiskMinutes,
			CriticalDelayMinutes: cfg.ControlTower.CriticalDelayMinutes,
			StaleWarningMinutes:  cfg.ControlTower.StaleWarningMinutes,
			StaleCriticalMinutes: cfg.ControlTower.StaleCriticalMinutes,
		},
		readModel:              readModelClient,
		readModelCfg:           cfg.ControlTower.ReadModel,
		readModelLog:           log,
		metrics:                metrics,
		legacyAggregate:        legacyClient,
		legacyAggregateTimeout: legacyTimeout,
		legacyMetrics:          legacyMetrics,
		legacyLog:              log,
		trackingClient: tracking.NewClient(
			&http.Client{Timeout: time.Duration(cfg.ProxyTimeoutSeconds) * time.Second},
			cfg.Services.Tracking,
			cfg.TrackingInternalToken,
		),
	}
}

func (s *Service) GetSummary(ctx context.Context, reqCtx RequestContext, query ListQuery) (SummaryResponse, error) {
	now := time.Now().UTC()
	freshness := DataFreshness{Warnings: []string{}}

	shipmentsRaw, shipmentsTotal, err := s.client.FetchShipments(ctx, reqCtx)
	if err != nil {
		return SummaryResponse{}, apperrors.ControlTowerShipmentsUnavailable("required shipment data is temporarily unavailable")
	}
	freshness.ShipmentsLoaded = true

	var (
		transportOrders []rawTransportOrder
		companies       []rawCompany
		documents       []rawDocument
		wg              sync.WaitGroup
		mu              sync.Mutex
	)

	wg.Add(3)
	go func() {
		defer wg.Done()
		items, err := s.client.FetchTransportOrders(ctx, reqCtx)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			freshness.Warnings = append(freshness.Warnings, WarningTransportOrdersUnavailable)
			return
		}
		transportOrders = items
		freshness.TransportOrdersLoaded = true
	}()
	go func() {
		defer wg.Done()
		items, err := s.client.FetchCompanies(ctx, reqCtx)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			freshness.Warnings = append(freshness.Warnings, WarningCompaniesUnavailable)
			return
		}
		companies = items
		freshness.CompaniesLoaded = true
	}()
	go func() {
		defer wg.Done()
		items, err := s.client.FetchDocuments(ctx, reqCtx)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			freshness.Warnings = append(freshness.Warnings, WarningDocumentsUnavailable)
			return
		}
		documents = items
		freshness.DocumentsLoaded = true
	}()
	wg.Wait()

	if shipmentsTotal > len(shipmentsRaw) {
		freshness.Partial = true
		freshness.Warnings = append(freshness.Warnings, WarningKPILimitedDataset)
	}
	if !freshness.CompaniesLoaded {
		freshness.Partial = true
	}
	if !freshness.TransportOrdersLoaded {
		freshness.Partial = true
	}
	if !freshness.DocumentsLoaded {
		freshness.Partial = true
	}
	if len(freshness.Warnings) > 0 {
		freshness.Partial = true
	}

	orderByID := map[string]rawTransportOrder{}
	for _, order := range transportOrders {
		orderByID[order.ID] = order
	}
	companyByID := map[string]rawCompany{}
	for _, company := range companies {
		companyByID[company.ID] = company
	}
	shipmentIDsWithDocs := shipmentDocumentIDs(documents)

	allRows := make([]ControlTowerShipment, 0, len(shipmentsRaw))
	rawByID := make(map[string]rawShipment, len(shipmentsRaw))
	for _, raw := range shipmentsRaw {
		rawByID[raw.ID] = raw
		allRows = append(allRows, s.mapShipment(raw, orderByID, companyByID, shipmentIDsWithDocs, now))
	}

	s.enrichShipmentsWithTracking(ctx, reqCtx, allRows)
	s.enrichShipmentsWithETA(ctx, reqCtx, allRows)
	s.enrichShipmentsWithSlots(ctx, reqCtx, allRows)

	filtered := ApplyFilters(allRows, query)
	kpi := CalculateKPI(filtered)
	page := Paginate(filtered, query.Page, query.Limit)
	shipmentByID := make(map[string]ControlTowerShipment, len(allRows))
	for _, row := range allRows {
		shipmentByID[row.ID] = row
	}
	criticalEvents := BuildCriticalEvents(filtered, shipmentIDsWithDocs, s.thresholds, now)
	s.mergeDriverCriticalEvents(ctx, reqCtx, &criticalEvents, shipmentByID)
	s.enrichCriticalEventWorkflows(ctx, reqCtx, &criticalEvents)
	SortCriticalEvents(criticalEvents)
	criticalEvents = FilterCriticalEvents(criticalEvents, query)
	exceptionKPI := CalculateExceptionKPI(criticalEvents)

	s.evaluateAndPersistRisks(ctx, reqCtx, allRows, rawByID, criticalEvents, now)
	shipmentNumbers := make(map[string]string, len(allRows))
	for _, row := range allRows {
		shipmentNumbers[row.ID] = row.ShipmentNumber
	}
	shipmentRisks, riskKPI := s.loadShipmentRisks(ctx, reqCtx, query, shipmentNumbers)

	filters := BuildFilterOptions(allRows, companies, freshness.CompaniesLoaded)
	if !freshness.CompaniesLoaded {
		freshness.Warnings = appendUniqueWarning(freshness.Warnings, WarningFilterOptionsIncomplete)
	}

	legacyInput := s.resolveLegacyStatusInput(ctx, reqCtx, shipmentsRaw, shipmentsTotal)

	response := SummaryResponse{
		GeneratedAt:    now,
		DataFreshness:  freshness,
		KPI:            kpi,
		ExceptionKPI:   exceptionKPI,
		RiskKPI:        riskKPI,
		Shipments:      page,
		CriticalEvents: criticalEvents,
		ShipmentRisks:  shipmentRisks,
		Filters:        filters,
	}

	s.applyReadModelStatusSummary(ctx, &response, reqCtx, legacyInput)
	return response, nil
}

func (s *Service) mapShipment(
	raw rawShipment,
	orderByID map[string]rawTransportOrder,
	companyByID map[string]rawCompany,
	shipmentIDsWithDocs map[string]struct{},
	now time.Time,
) ControlTowerShipment {
	lastUpdated := raw.UpdatedAt
	if lastUpdated == nil {
		lastUpdated = raw.CreatedAt
	}

	sla := ComputeSLA(SLAInput{
		Status:            raw.Status,
		PlannedPickupAt:   raw.PlannedPickupAt,
		PlannedDeliveryAt: raw.PlannedDeliveryAt,
		ActualPickupAt:    raw.ActualPickupAt,
		ActualDeliveryAt:  raw.ActualDeliveryAt,
		LastUpdatedAt:     lastUpdated,
		Now:               now,
		Thresholds:        s.thresholds,
	})

	row := ControlTowerShipment{
		ID:                raw.ID,
		ShipmentNumber:    raw.ShipmentNumber,
		Status:            raw.Status,
		SLAStatus:         sla.Status,
		SLAReason:         &sla.Reason,
		DelayMinutes:      sla.DelayMinutes,
		PlannedPickupAt:   raw.PlannedPickupAt,
		PlannedDeliveryAt: raw.PlannedDeliveryAt,
		ActualPickupAt:    raw.ActualPickupAt,
		ActualDeliveryAt:  raw.ActualDeliveryAt,
		LastUpdatedAt:     lastUpdated,
		DocumentsComplete: isDocumentsComplete(raw, shipmentIDsWithDocs),
		ReadyForBilling:   raw.Status == "READY_FOR_BILLING",
		DriverID:          raw.DriverID,
		VehicleID:         raw.VehicleID,
	}

	if raw.TransportOrderID != nil {
		row.TransportOrderID = raw.TransportOrderID
		if order, ok := orderByID[*raw.TransportOrderID]; ok {
			number := order.OrderNumber
			row.TransportOrderNumber = &number
		}
	}

	if raw.ShipperCompanyID != "" {
		id := raw.ShipperCompanyID
		row.ShipperID = &id
		if company, ok := companyByID[id]; ok {
			name := companyLabel(company)
			row.ShipperName = &name
		}
	}
	if raw.CarrierCompanyID != nil {
		row.CarrierID = raw.CarrierCompanyID
		if company, ok := companyByID[*raw.CarrierCompanyID]; ok {
			name := companyLabel(company)
			row.CarrierName = &name
		}
	}
	if raw.OriginLocationID != "" {
		id := raw.OriginLocationID
		name := shortID(id)
		row.OriginID = &id
		row.OriginName = &name
	}
	if raw.DestinationLocationID != "" {
		id := raw.DestinationLocationID
		name := shortID(id)
		row.DestinationID = &id
		row.DestinationName = &name
	}

	lastEvent := deriveLastEvent(raw)
	row.LastEvent = &lastEvent
	return row
}

func companyLabel(company rawCompany) string {
	if company.ShortName != nil && *company.ShortName != "" {
		return *company.ShortName
	}
	return company.LegalName
}

func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

func deriveLastEvent(raw rawShipment) string {
	if raw.Status == "CANCELLED" {
		return "CANCELLED"
	}
	if raw.ActualDeliveryAt != nil {
		return "DELIVERED"
	}
	if raw.ActualPickupAt != nil {
		return "PICKED_UP"
	}
	return raw.Status
}

func shipmentDocumentIDs(documents []rawDocument) map[string]struct{} {
	result := map[string]struct{}{}
	for _, doc := range documents {
		if doc.RelatedEntityType == nil || *doc.RelatedEntityType != "SHIPMENT" {
			continue
		}
		if doc.RelatedEntityID == nil || *doc.RelatedEntityID == "" {
			continue
		}
		if doc.DocumentStatus == "SIGNED" || doc.DocumentStatus == "ACCEPTED" || doc.DocumentStatus == "ARCHIVED" {
			result[*doc.RelatedEntityID] = struct{}{}
		}
	}
	return result
}

func isDocumentsComplete(raw rawShipment, shipmentIDsWithDocs map[string]struct{}) bool {
	if raw.Status == "DOCUMENTS_COMPLETED" || raw.Status == "READY_FOR_BILLING" || raw.Status == "INCLUDED_IN_BILLING_REGISTER" || raw.Status == "FINANCIALLY_CLOSED" {
		return true
	}
	_, ok := shipmentIDsWithDocs[raw.ID]
	return ok
}

func appendUniqueWarning(warnings []string, code string) []string {
	for _, existing := range warnings {
		if existing == code {
			return warnings
		}
	}
	return append(warnings, code)
}
