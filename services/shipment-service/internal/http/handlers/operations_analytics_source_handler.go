package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/platform/respond"
)

const (
	AuthorizedAnalyticsCaller = "analytics-service"
	// OperationsFoundationV2Path is the extended source read. The original route stays
	// key-compatible because the Analytics-0.2 client rejects unknown JSON fields.
	OperationsFoundationV2Path = "/internal/v1/analytics/operations-foundation-v2"
)

type operationsFoundationV1Response struct {
	TenantID                  uuid.UUID `json:"tenantId"`
	ShipmentTotal             int64     `json:"shipmentTotal"`
	OnTimeDeliveryDenominator int64     `json:"onTimeDeliveryDenominator"`
	OnTimeDeliveryNumerator   int64     `json:"onTimeDeliveryNumerator"`
	ReturnCaseCount           int64     `json:"returnCaseCount"`
	RedirectCaseCount         int64     `json:"redirectCaseCount"`
}

// RequireAnalyticsCaller rejects any internal caller other than analytics-service.
// The shared internal token middleware must already have accepted the request.
func RequireAnalyticsCaller(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(r.Header.Get(HeaderInternalServiceName)) != AuthorizedAnalyticsCaller {
			respond.Error(w, apperrors.Forbidden("caller is not allowed to read analytics source facts"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

type OperationsAnalyticsSourceReader interface {
	OperationsFoundation(ctx context.Context, tenantID uuid.UUID) (domain.OperationsAnalyticsSourceSnapshot, error)
}

type OperationsAnalyticsSourceHandler struct {
	reader OperationsAnalyticsSourceReader
}

func NewOperationsAnalyticsSourceHandler(reader OperationsAnalyticsSourceReader) *OperationsAnalyticsSourceHandler {
	return &OperationsAnalyticsSourceHandler{reader: reader}
}

func (h *OperationsAnalyticsSourceHandler) Get(w http.ResponseWriter, r *http.Request) {
	snap, err := h.read(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, operationsFoundationV1Response{
		TenantID:                  snap.TenantID,
		ShipmentTotal:             snap.ShipmentTotal,
		OnTimeDeliveryDenominator: snap.OnTimeDeliveryDenominator,
		OnTimeDeliveryNumerator:   snap.OnTimeDeliveryNumerator,
		ReturnCaseCount:           snap.ReturnCaseCount,
		RedirectCaseCount:         snap.RedirectCaseCount,
	})
}

func (h *OperationsAnalyticsSourceHandler) GetExtended(w http.ResponseWriter, r *http.Request) {
	snap, err := h.read(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if snap.Carriers == nil {
		snap.Carriers = []domain.OperationsAnalyticsCarrierSource{}
	}
	respond.JSON(w, http.StatusOK, snap)
}

func (h *OperationsAnalyticsSourceHandler) read(r *http.Request) (domain.OperationsAnalyticsSourceSnapshot, error) {
	if h == nil || h.reader == nil {
		return domain.OperationsAnalyticsSourceSnapshot{}, apperrors.Internal("analytics source is not configured", nil)
	}
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		return domain.OperationsAnalyticsSourceSnapshot{}, err
	}
	return h.reader.OperationsFoundation(r.Context(), tenantID)
}
