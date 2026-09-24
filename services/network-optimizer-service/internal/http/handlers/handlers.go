package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	apperrors "github.com/freight-platform/network-optimizer-service/internal/platform/errors"
	bnometrics "github.com/freight-platform/network-optimizer-service/internal/platform/metrics"
	"github.com/freight-platform/network-optimizer-service/internal/platform/respond"
	"github.com/freight-platform/network-optimizer-service/internal/reference"
	"github.com/freight-platform/network-optimizer-service/internal/service"
	"github.com/freight-platform/shared-go/lowcode"
	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
)

const headerCompanyID = "X-Company-ID"

type Handler struct {
	log     *slog.Logger
	svc     *service.Service
	catalog reference.Catalog
}

func New(log *slog.Logger, svc *service.Service) *Handler {
	if log == nil {
		log = slog.Default()
	}
	return &Handler{log: log, svc: svc, catalog: reference.NewMemoryCatalog()}
}

func (h *Handler) CreateLoad(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, "create_load", func(actor service.Actor, raw []byte, key, hash string) (service.Result, error) {
		var body createLoadBody
		if err := decode(raw, &body); err != nil {
			return service.Result{}, err
		}
		load, err := body.toDomain()
		if err != nil {
			return service.Result{}, err
		}
		return h.svc.CreateLoad(r.Context(), actor, key, hash, service.CreateLoadCommand{Load: load, Publish: body.Publish})
	})
}

func (h *Handler) PublishLoad(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, "publish_load", func(actor service.Actor, raw []byte, key, hash string) (service.Result, error) {
		id, version, err := idAndVersion(r, raw)
		if err != nil {
			return service.Result{}, err
		}
		return h.svc.PublishLoad(r.Context(), actor, id, version, key, hash)
	})
}

func (h *Handler) UpdateLoad(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, "update_load", func(actor service.Actor, raw []byte, key, hash string) (service.Result, error) {
		id, err := parseID(chi.URLParam(r, "id"))
		if err != nil {
			return service.Result{}, err
		}
		var body updateLoadBody
		if err := decode(raw, &body); err != nil {
			return service.Result{}, err
		}
		return h.svc.UpdateLoad(r.Context(), actor, id, body.patch(), key, hash)
	})
}

func (h *Handler) WithdrawLoad(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, "withdraw_load", func(actor service.Actor, raw []byte, key, hash string) (service.Result, error) {
		id, version, err := idAndVersion(r, raw)
		if err != nil {
			return service.Result{}, err
		}
		return h.svc.WithdrawLoad(r.Context(), actor, id, version, key, hash)
	})
}

func (h *Handler) GetOwnLoad(w http.ResponseWriter, r *http.Request) {
	h.read(w, r, "get_load", func(actor service.Actor) (service.Result, error) {
		id, err := parseID(chi.URLParam(r, "id"))
		if err != nil {
			return service.Result{}, err
		}
		return h.svc.GetOwnLoad(r.Context(), actor, id)
	})
}

func (h *Handler) ListOwnLoads(w http.ResponseWriter, r *http.Request) {
	h.read(w, r, "list_loads", func(actor service.Actor) (service.Result, error) {
		limit, offset, err := pageQuery(r)
		if err != nil {
			return service.Result{}, err
		}
		return h.svc.ListOwnLoads(r.Context(), actor, limit, offset)
	})
}

func (h *Handler) GetMarketplaceLoad(w http.ResponseWriter, r *http.Request) {
	h.read(w, r, "get_marketplace_load", func(actor service.Actor) (service.Result, error) {
		id, err := parseID(chi.URLParam(r, "id"))
		if err != nil {
			return service.Result{}, err
		}
		return h.svc.GetMarketplaceLoad(r.Context(), actor, id)
	})
}

func (h *Handler) ListMarketplaceLoads(w http.ResponseWriter, r *http.Request) {
	h.read(w, r, "list_marketplace_loads", func(actor service.Actor) (service.Result, error) {
		limit, offset, err := pageQuery(r)
		if err != nil {
			return service.Result{}, err
		}
		return h.svc.ListMarketplaceLoads(r.Context(), actor, limit, offset)
	})
}

func (h *Handler) CreateCapacity(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, "create_capacity", func(actor service.Actor, raw []byte, key, hash string) (service.Result, error) {
		var body createCapacityBody
		if err := decode(raw, &body); err != nil {
			return service.Result{}, err
		}
		cap, err := body.toDomain()
		if err != nil {
			return service.Result{}, err
		}
		return h.svc.CreateCapacity(r.Context(), actor, key, hash, service.CreateCapacityCommand{Capacity: cap})
	})
}

func (h *Handler) UpdateCapacity(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, "update_capacity", func(actor service.Actor, raw []byte, key, hash string) (service.Result, error) {
		id, err := parseID(chi.URLParam(r, "id"))
		if err != nil {
			return service.Result{}, err
		}
		var body updateCapacityBody
		if err := decode(raw, &body); err != nil {
			return service.Result{}, err
		}
		return h.svc.UpdateCapacity(r.Context(), actor, id, body.patch(), key, hash)
	})
}

func (h *Handler) WithdrawCapacity(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, "withdraw_capacity", func(actor service.Actor, raw []byte, key, hash string) (service.Result, error) {
		id, version, err := idAndVersion(r, raw)
		if err != nil {
			return service.Result{}, err
		}
		return h.svc.WithdrawCapacity(r.Context(), actor, id, version, key, hash)
	})
}

func (h *Handler) GetOwnCapacity(w http.ResponseWriter, r *http.Request) {
	h.read(w, r, "get_capacity", func(actor service.Actor) (service.Result, error) {
		id, err := parseID(chi.URLParam(r, "id"))
		if err != nil {
			return service.Result{}, err
		}
		return h.svc.GetOwnCapacity(r.Context(), actor, id)
	})
}

func (h *Handler) ListOwnCapacities(w http.ResponseWriter, r *http.Request) {
	h.read(w, r, "list_capacities", func(actor service.Actor) (service.Result, error) {
		limit, offset, err := pageQuery(r)
		if err != nil {
			return service.Result{}, err
		}
		return h.svc.ListOwnCapacities(r.Context(), actor, limit, offset)
	})
}

func (h *Handler) GetMarketplaceCapacity(w http.ResponseWriter, r *http.Request) {
	h.read(w, r, "get_marketplace_capacity", func(actor service.Actor) (service.Result, error) {
		id, err := parseID(chi.URLParam(r, "id"))
		if err != nil {
			return service.Result{}, err
		}
		return h.svc.GetMarketplaceCapacity(r.Context(), actor, id)
	})
}

func (h *Handler) ListMarketplaceCapacities(w http.ResponseWriter, r *http.Request) {
	h.read(w, r, "list_marketplace_capacities", func(actor service.Actor) (service.Result, error) {
		limit, offset, err := pageQuery(r)
		if err != nil {
			return service.Result{}, err
		}
		return h.svc.ListMarketplaceCapacities(r.Context(), actor, limit, offset)
	})
}

func (h *Handler) mutate(w http.ResponseWriter, r *http.Request, operation string, fn func(service.Actor, []byte, string, string) (service.Result, error)) {
	actor, err := actorFrom(r)
	if err != nil {
		h.finish(w, r, operation, uuid.Nil, err)
		return
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		h.finish(w, r, operation, uuid.Nil, apperrors.Validation("request body is invalid", nil))
		return
	}
	key, err := idempotencyKey(r, operation)
	if err != nil {
		h.finish(w, r, operation, uuid.Nil, err)
		return
	}
	result, err := fn(actor, raw, key, service.HashBody(raw))
	if err != nil {
		h.finish(w, r, operation, uuid.Nil, err)
		return
	}
	h.log.Info("bno request",
		slog.String("request_id", actor.RequestID),
		slog.String("tenant_id", actor.TenantID.String()),
		slog.String("aggregate_id", result.AggregateID.String()),
		slog.String("operation", operation),
		slog.Int("status", result.Status),
	)
	bnometrics.API(operation, "ok")
	respond.Bytes(w, result.Status, result.Body)
}

func (h *Handler) read(w http.ResponseWriter, r *http.Request, operation string, fn func(service.Actor) (service.Result, error)) {
	actor, err := actorFrom(r)
	if err != nil {
		h.finish(w, r, operation, uuid.Nil, err)
		return
	}
	result, err := fn(actor)
	if err != nil {
		h.finish(w, r, operation, uuid.Nil, err)
		return
	}
	h.log.Info("bno request",
		slog.String("request_id", actor.RequestID),
		slog.String("tenant_id", actor.TenantID.String()),
		slog.String("aggregate_id", result.AggregateID.String()),
		slog.String("operation", operation),
		slog.Int("status", result.Status),
	)
	bnometrics.API(operation, "ok")
	respond.Bytes(w, result.Status, result.Body)
}

func (h *Handler) finish(w http.ResponseWriter, r *http.Request, operation string, aggregate uuid.UUID, err error) {
	requestID := sharedmiddleware.RequestIDFromContext(r.Context())
	if requestID == "" {
		requestID = lowcode.RequestIDFromHeader(r.Header)
	}
	tenant := lowcode.TenantIDFromHeader(r.Header)
	h.log.Info("bno request",
		slog.String("request_id", requestID),
		slog.String("tenant_id", tenant),
		slog.String("aggregate_id", aggregate.String()),
		slog.String("operation", operation),
		slog.String("error_code", errorCode(err)),
	)
	bnometrics.API(operation, errorCode(err))
	respond.Error(w, err)
}

func actorFrom(r *http.Request) (service.Actor, error) {
	if q := strings.TrimSpace(r.URL.Query().Get("tenant_id")); q != "" && !strings.EqualFold(q, lowcode.TenantIDFromHeader(r.Header)) {
		return service.Actor{}, apperrors.Forbidden("tenant_id does not match authenticated tenant")
	}
	tenant, err := uuid.Parse(lowcode.TenantIDFromHeader(r.Header))
	if err != nil {
		return service.Actor{}, apperrors.Unauthorized("verified tenant context is required")
	}
	user, err := uuid.Parse(strings.TrimSpace(r.Header.Get(lowcode.HeaderUserID)))
	if err != nil {
		return service.Actor{}, apperrors.Unauthorized("verified user context is required")
	}
	var company *uuid.UUID
	if raw := strings.TrimSpace(r.Header.Get(headerCompanyID)); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			return service.Actor{}, apperrors.Validation("invalid X-Company-ID", map[string]any{"field": "X-Company-ID"})
		}
		company = &parsed
	}
	requestID := sharedmiddleware.RequestIDFromContext(r.Context())
	if requestID == "" {
		requestID = lowcode.RequestIDFromHeader(r.Header)
	}
	return service.Actor{TenantID: tenant, UserID: user, CompanyID: company, RequestID: requestID}, nil
}

func idempotencyKey(r *http.Request, operation string) (string, error) {
	if _, ok := r.Header["Idempotency-Key"]; !ok {
		return "", nil
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" || len(key) > 128 {
		return "", apperrors.Validation("Idempotency-Key must be 1..128 characters", nil)
	}
	return operation + ":" + key, nil
}

func decode(raw []byte, dest any) error {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		return apperrors.Validation("request body is invalid", nil)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return apperrors.Validation("request body is invalid", nil)
	}
	return nil
}

func parseID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, apperrors.Validation("id must be a uuid", nil)
	}
	return id, nil
}

func idAndVersion(r *http.Request, raw []byte) (uuid.UUID, int, error) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		return uuid.Nil, 0, err
	}
	var body struct {
		Version int `json:"version"`
	}
	if err := decode(raw, &body); err != nil {
		return uuid.Nil, 0, err
	}
	if body.Version < 1 {
		return uuid.Nil, 0, apperrors.Validation("version is required", nil)
	}
	if match := strings.Trim(r.Header.Get("If-Match"), `"`); match != "" && match != strconv.Itoa(body.Version) {
		return uuid.Nil, 0, apperrors.Conflict("version conflict", nil)
	}
	return id, body.Version, nil
}

func pageQuery(r *http.Request) (int, int, error) {
	limit, err := optionalInt(r.URL.Query().Get("limit"))
	if err != nil {
		return 0, 0, apperrors.Validation("limit is invalid", nil)
	}
	offset, err := optionalInt(r.URL.Query().Get("offset"))
	if err != nil {
		return 0, 0, apperrors.Validation("offset is invalid", nil)
	}
	return limit, offset, nil
}

func optionalInt(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	return strconv.Atoi(raw)
}

func errorCode(err error) string {
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		return string(appErr.Code)
	}
	return string(apperrors.CodeInternal)
}

type placeBody struct {
	LocationID *uuid.UUID `json:"location_id"`
	Label      string     `json:"label"`
	Latitude   *float64   `json:"latitude"`
	Longitude  *float64   `json:"longitude"`
}

type windowBody struct {
	Start *time.Time `json:"start"`
	End   *time.Time `json:"end"`
}

type createLoadBody struct {
	SourceType               string                  `json:"source_type"`
	SourceID                 uuid.UUID               `json:"source_id"`
	Pickup                   placeBody               `json:"pickup"`
	PickupWindow             windowBody              `json:"pickup_window"`
	Delivery                 placeBody               `json:"delivery"`
	DeliveryWindow           windowBody              `json:"delivery_window"`
	WeightKg                 *float64                `json:"weight_kg"`
	VolumeM3                 *float64                `json:"volume_m3"`
	BodyType                 string                  `json:"body_type"`
	Equipment                []string                `json:"equipment"`
	Cargo                    domain.CargoConstraints `json:"cargo"`
	Commercial               domain.Commercial       `json:"commercial"`
	VisibilityScope          string                  `json:"visibility_scope"`
	InvitedCarrierCompanyIDs []uuid.UUID             `json:"invited_carrier_company_ids"`
	Publish                  bool                    `json:"publish"`
}

func (b createLoadBody) toDomain() (domain.LoadOpportunity, error) {
	return domain.LoadOpportunity{
		SourceType: b.SourceType, SourceID: b.SourceID,
		Pickup: placeOf(b.Pickup), PickupWindow: windowOf(b.PickupWindow),
		Delivery: placeOf(b.Delivery), DeliveryWindow: windowOf(b.DeliveryWindow),
		WeightKg: b.WeightKg, VolumeM3: b.VolumeM3, BodyType: b.BodyType, Equipment: b.Equipment,
		Cargo: b.Cargo, Commercial: b.Commercial, VisibilityScope: b.VisibilityScope,
		InvitedCarrierCompanyIDs: b.InvitedCarrierCompanyIDs,
	}, nil
}

type updateLoadBody struct {
	Version                  int                      `json:"version"`
	VisibilityScope          *string                  `json:"visibility_scope"`
	InvitedCarrierCompanyIDs *[]uuid.UUID             `json:"invited_carrier_company_ids"`
	Pickup                   *placeBody               `json:"pickup"`
	PickupWindow             *windowBody              `json:"pickup_window"`
	Delivery                 *placeBody               `json:"delivery"`
	DeliveryWindow           *windowBody              `json:"delivery_window"`
	WeightKg                 *float64                 `json:"weight_kg"`
	VolumeM3                 *float64                 `json:"volume_m3"`
	BodyType                 *string                  `json:"body_type"`
	Equipment                *[]string                `json:"equipment"`
	Cargo                    *domain.CargoConstraints `json:"cargo"`
	Commercial               *domain.Commercial       `json:"commercial"`
}

func (b updateLoadBody) patch() service.LoadPatch {
	patch := service.LoadPatch{
		Version: b.Version, VisibilityScope: b.VisibilityScope, Invited: b.InvitedCarrierCompanyIDs,
		WeightKg: b.WeightKg, VolumeM3: b.VolumeM3, BodyType: b.BodyType, Equipment: b.Equipment,
		Cargo: b.Cargo, Commercial: b.Commercial,
	}
	if b.Pickup != nil {
		place := placeOf(*b.Pickup)
		patch.Pickup = &place
		patch.Changed = true
	}
	if b.PickupWindow != nil {
		window := windowOf(*b.PickupWindow)
		patch.PickupWindow = &window
		patch.Changed = true
	}
	if b.Delivery != nil {
		place := placeOf(*b.Delivery)
		patch.Delivery = &place
		patch.Changed = true
	}
	if b.DeliveryWindow != nil {
		window := windowOf(*b.DeliveryWindow)
		patch.DeliveryWindow = &window
		patch.Changed = true
	}
	patch.Changed = patch.Changed || b.VisibilityScope != nil || b.InvitedCarrierCompanyIDs != nil || b.WeightKg != nil || b.VolumeM3 != nil || b.BodyType != nil || b.Equipment != nil || b.Cargo != nil || b.Commercial != nil
	return patch
}

type createCapacityBody struct {
	CarrierCompanyID   *uuid.UUID  `json:"carrier_company_id"`
	VehicleID          *uuid.UUID  `json:"vehicle_id"`
	LocationLabel      string      `json:"location_label"`
	Latitude           *float64    `json:"latitude"`
	Longitude          *float64    `json:"longitude"`
	AvailableFrom      time.Time   `json:"available_from"`
	AvailableUntil     time.Time   `json:"available_until"`
	Source             string      `json:"source"`
	BodyType           string      `json:"body_type"`
	Equipment          []string    `json:"equipment"`
	PayloadRemainingKg *float64    `json:"payload_remaining_kg"`
	VolumeRemainingM3  *float64    `json:"volume_remaining_m3"`
	VisibilityScope    string      `json:"visibility_scope"`
	AudienceTenantIDs  []uuid.UUID `json:"audience_tenant_ids"`
}

func (b createCapacityBody) toDomain() (domain.Capacity, error) {
	return domain.Capacity{
		CarrierCompanyID: b.CarrierCompanyID, VehicleID: b.VehicleID, LocationLabel: b.LocationLabel,
		Latitude: b.Latitude, Longitude: b.Longitude, AvailableFrom: b.AvailableFrom, AvailableUntil: b.AvailableUntil,
		Source: b.Source, BodyType: b.BodyType, Equipment: b.Equipment,
		PayloadRemainingKg: b.PayloadRemainingKg, VolumeRemainingM3: b.VolumeRemainingM3,
		VisibilityScope: b.VisibilityScope, AudienceTenantIDs: b.AudienceTenantIDs,
	}, nil
}

type updateCapacityBody struct {
	Version            int          `json:"version"`
	CarrierCompanyID   *uuid.UUID   `json:"carrier_company_id"`
	VehicleID          *uuid.UUID   `json:"vehicle_id"`
	LocationLabel      *string      `json:"location_label"`
	Latitude           *float64     `json:"latitude"`
	Longitude          *float64     `json:"longitude"`
	AvailableFrom      *time.Time   `json:"available_from"`
	AvailableUntil     *time.Time   `json:"available_until"`
	BodyType           *string      `json:"body_type"`
	Equipment          *[]string    `json:"equipment"`
	PayloadRemainingKg *float64     `json:"payload_remaining_kg"`
	VolumeRemainingM3  *float64     `json:"volume_remaining_m3"`
	VisibilityScope    *string      `json:"visibility_scope"`
	AudienceTenantIDs  *[]uuid.UUID `json:"audience_tenant_ids"`
}

func (b updateCapacityBody) patch() service.CapacityPatch {
	patch := service.CapacityPatch{
		Version: b.Version, CarrierCompanyID: b.CarrierCompanyID, VehicleID: b.VehicleID,
		LocationLabel: b.LocationLabel, Latitude: b.Latitude, Longitude: b.Longitude,
		AvailableFrom: b.AvailableFrom, AvailableUntil: b.AvailableUntil, BodyType: b.BodyType,
		Equipment: b.Equipment, PayloadRemainingKg: b.PayloadRemainingKg, VolumeRemainingM3: b.VolumeRemainingM3,
		VisibilityScope: b.VisibilityScope, Audience: b.AudienceTenantIDs,
	}
	patch.Changed = b.CarrierCompanyID != nil || b.VehicleID != nil || b.LocationLabel != nil || b.Latitude != nil || b.Longitude != nil || b.AvailableFrom != nil || b.AvailableUntil != nil || b.BodyType != nil || b.Equipment != nil || b.PayloadRemainingKg != nil || b.VolumeRemainingM3 != nil || b.VisibilityScope != nil || b.AudienceTenantIDs != nil
	return patch
}

func placeOf(body placeBody) domain.Place {
	return domain.Place{LocationID: body.LocationID, Label: body.Label, Latitude: body.Latitude, Longitude: body.Longitude}
}

func windowOf(body windowBody) domain.TimeWindow {
	return domain.TimeWindow{Start: body.Start, End: body.End}
}
