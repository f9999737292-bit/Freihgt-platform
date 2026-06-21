package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/platform/respond"
	"github.com/freight-platform/shipment-service/internal/service"
)

type DriverHandler struct {
	service *service.DriverService
}

func NewDriverHandler(svc *service.DriverService) *DriverHandler {
	return &DriverHandler{service: svc}
}

type createDriverRequest struct {
	TenantID         string  `json:"tenant_id"`
	CarrierCompanyID string  `json:"carrier_company_id"`
	UserID           *string `json:"user_id"`
	FullName         string  `json:"full_name"`
	Phone            *string `json:"phone"`
	LicenseNumber    *string `json:"license_number"`
	LicenseCountry   string  `json:"license_country"`
	PreferredLocale  string  `json:"preferred_locale"`
}

func (h *DriverHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createDriverRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	input, err := parseCreateDriverRequest(req)
	if err != nil {
		respond.Error(w, err)
		return
	}
	driver, err := h.service.Create(r.Context(), input)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toDriverResponse(driver))
}

func (h *DriverHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	driver, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toDriverResponse(driver))
}

func (h *DriverHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, err := domain.ParseUUID(r.URL.Query().Get("tenant_id"), "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	filter := domain.ListDriversFilter{
		TenantID: tenantID,
		Limit:    parseLimit(r),
		Offset:   parseOffset(r),
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("carrier_company_id")); raw != "" {
		id, err := domain.ParseUUID(raw, "carrier_company_id")
		if err != nil {
			respond.Error(w, err)
			return
		}
		filter.CarrierCompanyID = &id
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("status")); raw != "" {
		filter.Status = &raw
	}

	drivers, total, err := h.service.List(r.Context(), filter)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items := make([]map[string]any, 0, len(drivers))
	for i := range drivers {
		items = append(items, toDriverResponse(&drivers[i]))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func parseCreateDriverRequest(req createDriverRequest) (domain.CreateDriverInput, error) {
	tenantID, err := domain.ParseUUID(req.TenantID, "tenant_id")
	if err != nil {
		return domain.CreateDriverInput{}, err
	}
	carrierCompanyID, err := domain.ParseUUID(req.CarrierCompanyID, "carrier_company_id")
	if err != nil {
		return domain.CreateDriverInput{}, err
	}
	userID, err := domain.ParseOptionalUUID(derefString(req.UserID), "user_id")
	if err != nil {
		return domain.CreateDriverInput{}, err
	}
	return domain.CreateDriverInput{
		TenantID:         tenantID,
		CarrierCompanyID: carrierCompanyID,
		UserID:           userID,
		FullName:         req.FullName,
		Phone:            req.Phone,
		LicenseNumber:    req.LicenseNumber,
		LicenseCountry:   req.LicenseCountry,
		PreferredLocale:  req.PreferredLocale,
	}, nil
}

func toDriverResponse(d *domain.Driver) map[string]any {
	return map[string]any{
		"id":                 d.ID.String(),
		"tenant_id":          d.TenantID.String(),
		"carrier_company_id": d.CarrierCompanyID.String(),
		"user_id":            optionalUUIDString(d.UserID),
		"full_name":          d.FullName,
		"phone":              d.Phone,
		"license_number":     d.LicenseNumber,
		"license_country":    d.LicenseCountry,
		"preferred_locale":   d.PreferredLocale,
		"status":             d.Status,
	}
}
