package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/company-service/internal/domain"
	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
	"github.com/freight-platform/company-service/internal/platform/respond"
	"github.com/freight-platform/company-service/internal/service"
)

type CompanyHandler struct {
	service *service.CompanyService
}

func NewCompanyHandler(svc *service.CompanyService) *CompanyHandler {
	return &CompanyHandler{service: svc}
}

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type createCompanyRequest struct {
	TenantID           string  `json:"tenant_id"`
	LegalName          string  `json:"legal_name"`
	ShortName          *string `json:"short_name"`
	LegalNameEN        *string `json:"legal_name_en"`
	LegalNameZH        *string `json:"legal_name_zh"`
	CompanyType        string  `json:"company_type"`
	TaxID              *string `json:"tax_id"`
	RegistrationNumber *string `json:"registration_number"`
	CountryCode        string  `json:"country_code"`
	PreferredLocale    string  `json:"preferred_locale"`
}

type updateCompanyRequest struct {
	LegalName          *string `json:"legal_name"`
	ShortName          *string `json:"short_name"`
	LegalNameEN        *string `json:"legal_name_en"`
	LegalNameZH        *string `json:"legal_name_zh"`
	CompanyType        *string `json:"company_type"`
	TaxID              *string `json:"tax_id"`
	RegistrationNumber *string `json:"registration_number"`
	CountryCode        *string `json:"country_code"`
	PreferredLocale    *string `json:"preferred_locale"`
	Status             *string `json:"status"`
}

type companyResponse struct {
	ID                 string  `json:"id"`
	TenantID           string  `json:"tenant_id"`
	LegalName          string  `json:"legal_name"`
	ShortName          *string `json:"short_name,omitempty"`
	LegalNameEN        *string `json:"legal_name_en,omitempty"`
	LegalNameZH        *string `json:"legal_name_zh,omitempty"`
	CompanyType        string  `json:"company_type"`
	TaxID              *string `json:"tax_id,omitempty"`
	RegistrationNumber *string `json:"registration_number,omitempty"`
	CountryCode        string  `json:"country_code"`
	PreferredLocale    string  `json:"preferred_locale"`
	Status             string  `json:"status"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at,omitempty"`
	Version            int     `json:"version,omitempty"`
}

type listCompaniesResponse struct {
	Items  []companyResponse `json:"items"`
	Total  int               `json:"total"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}

func (h *CompanyHandler) Health(w http.ResponseWriter, _ *http.Request) {
	respond.JSON(w, http.StatusOK, healthResponse{
		Status:  "ok",
		Service: "company-service",
	})
}

func (h *CompanyHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createCompanyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}

	tenantID, err := domain.ParseUUID(req.TenantID, "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	company, err := h.service.Create(r.Context(), domain.CreateCompanyInput{
		TenantID:           tenantID,
		LegalName:          req.LegalName,
		ShortName:          req.ShortName,
		LegalNameEN:        req.LegalNameEN,
		LegalNameZH:        req.LegalNameZH,
		CompanyType:        req.CompanyType,
		TaxID:              req.TaxID,
		RegistrationNumber: req.RegistrationNumber,
		CountryCode:        req.CountryCode,
		PreferredLocale:    req.PreferredLocale,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusCreated, toCompanyResponse(company))
}

func (h *CompanyHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	company, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, toCompanyResponse(company))
}

func (h *CompanyHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, err := domain.ParseUUID(r.URL.Query().Get("tenant_id"), "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			respond.Error(w, apperrors.Validation("invalid limit", map[string]any{"field": "limit"}))
			return
		}
		limit = parsed
	}

	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			respond.Error(w, apperrors.Validation("invalid offset", map[string]any{"field": "offset"}))
			return
		}
		offset = parsed
	}

	filter := domain.ListCompaniesFilter{
		TenantID: tenantID,
		Limit:    limit,
		Offset:   offset,
	}

	if raw := strings.TrimSpace(r.URL.Query().Get("company_type")); raw != "" {
		filter.CompanyType = &raw
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("status")); raw != "" {
		filter.Status = &raw
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("search")); raw != "" {
		filter.Search = &raw
	}

	companies, total, err := h.service.List(r.Context(), filter)
	if err != nil {
		respond.Error(w, err)
		return
	}

	items := make([]companyResponse, 0, len(companies))
	for i := range companies {
		items = append(items, toCompanyResponse(&companies[i]))
	}

	respond.JSON(w, http.StatusOK, listCompaniesResponse{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

func (h *CompanyHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	var req updateCompanyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}

	company, err := h.service.Update(r.Context(), id, domain.UpdateCompanyInput{
		LegalName:          req.LegalName,
		ShortName:          req.ShortName,
		LegalNameEN:        req.LegalNameEN,
		LegalNameZH:        req.LegalNameZH,
		CompanyType:        req.CompanyType,
		TaxID:              req.TaxID,
		RegistrationNumber: req.RegistrationNumber,
		CountryCode:        req.CountryCode,
		PreferredLocale:    req.PreferredLocale,
		Status:             req.Status,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, toCompanyResponse(company))
}

func (h *CompanyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	if err := h.service.Delete(r.Context(), id); err != nil {
		respond.Error(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func toCompanyResponse(company *domain.Company) companyResponse {
	return companyResponse{
		ID:                 company.ID.String(),
		TenantID:           company.TenantID.String(),
		LegalName:          company.LegalName,
		ShortName:          company.ShortName,
		LegalNameEN:        company.LegalNameEN,
		LegalNameZH:        company.LegalNameZH,
		CompanyType:        company.CompanyType,
		TaxID:              company.TaxID,
		RegistrationNumber: company.RegistrationNumber,
		CountryCode:        company.CountryCode,
		PreferredLocale:    company.PreferredLocale,
		Status:             company.Status,
		CreatedAt:          company.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:          company.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		Version:            company.Version,
	}
}
