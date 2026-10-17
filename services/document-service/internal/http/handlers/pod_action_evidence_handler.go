package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
	"github.com/freight-platform/document-service/internal/platform/respond"
	"github.com/freight-platform/document-service/internal/service"
)

const (
	HeaderInternalServiceName   = "X-Internal-Service-Name"
	AuthorizedPODEvidenceCaller = "shipment-service"
)

// RequireShipmentServiceCaller allows only the shipment-service internal caller.
func RequireShipmentServiceCaller(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(r.Header.Get(HeaderInternalServiceName)) != AuthorizedPODEvidenceCaller {
			respond.Error(w, apperrors.Forbidden("caller is not allowed to bind POD action evidence"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

type PODActionEvidenceHandler struct {
	svc *service.PODActionEvidenceService
}

func NewPODActionEvidenceHandler(svc *service.PODActionEvidenceService) *PODActionEvidenceHandler {
	return &PODActionEvidenceHandler{svc: svc}
}

func (h *PODActionEvidenceHandler) CreateIntent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ActionType        string `json:"actionType"`
		DocumentTenantID  string `json:"documentTenantId"`
		OperatingTenantID string `json:"operatingTenantId"`
		ExecutionID       string `json:"executionId"`
		ExecutionActionID string `json:"executionActionId"`
		ExecutionStopID   string `json:"executionStopId"`
		ShipmentID        string `json:"shipmentId"`
		ShipmentTenantID  string `json:"shipmentTenantId"`
		CargoID           string `json:"cargoId"`
		OwnerCompanyID    string `json:"ownerCompanyId"`
		AuthorDriverID    string `json:"authorDriverId"`
		AuthorCompanyID   string `json:"authorCompanyId"`
		MimeType          string `json:"mimeType"`
		FileName          string `json:"fileName"`
		IdempotencyKey    string `json:"idempotencyKey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, apperrors.Validation("invalid body", nil))
		return
	}
	in, err := parseCreateBody(body.ActionType, body.DocumentTenantID, body.OperatingTenantID, body.ExecutionID, body.ExecutionActionID, body.ExecutionStopID, body.ShipmentID, body.ShipmentTenantID, body.CargoID, body.OwnerCompanyID, body.AuthorDriverID, body.AuthorCompanyID, body.MimeType, body.FileName, body.IdempotencyKey)
	if err != nil {
		respond.Error(w, err)
		return
	}
	intent, err := h.svc.CreateUploadIntent(r.Context(), in)
	if err != nil {
		respond.Error(w, err)
		return
	}
	status := http.StatusCreated
	if intent.Replayed {
		status = http.StatusOK
	}
	respond.JSON(w, status, intent)
}

func (h *PODActionEvidenceHandler) UploadContent(w http.ResponseWriter, r *http.Request) {
	uploadID, err := uuid.Parse(chi.URLParam(r, "uploadId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("uploadId required", nil))
		return
	}
	token := strings.TrimSpace(r.Header.Get("X-Upload-Token"))
	confirm, err := confirmationFromHeaders(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if err := h.svc.UploadContent(r.Context(), uploadID, token, io.LimitReader(r.Body, 11<<20), confirm); err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "uploaded"})
}

func (h *PODActionEvidenceHandler) Complete(w http.ResponseWriter, r *http.Request) {
	uploadID, err := uuid.Parse(chi.URLParam(r, "uploadId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("uploadId required", nil))
		return
	}
	var body struct {
		ChecksumSHA256    string `json:"checksumSha256"`
		DocumentTenantID  string `json:"documentTenantId"`
		OperatingTenantID string `json:"operatingTenantId"`
		ExecutionID       string `json:"executionId"`
		ExecutionActionID string `json:"executionActionId"`
		ExecutionStopID   string `json:"executionStopId"`
		ShipmentID        string `json:"shipmentId"`
		ShipmentTenantID  string `json:"shipmentTenantId"`
		CargoID           string `json:"cargoId"`
		OwnerCompanyID    string `json:"ownerCompanyId"`
		AuthorDriverID    string `json:"authorDriverId"`
		AuthorCompanyID   string `json:"authorCompanyId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, apperrors.Validation("invalid body", nil))
		return
	}
	confirm, err := confirmationFromStrings(body.DocumentTenantID, body.OperatingTenantID, body.ExecutionID, body.ExecutionActionID, body.ExecutionStopID, body.ShipmentID, body.ShipmentTenantID, body.CargoID, body.OwnerCompanyID, body.AuthorDriverID, body.AuthorCompanyID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	result, err := h.svc.CompleteUpload(r.Context(), uploadID, body.ChecksumSHA256, confirm)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, result)
}

func (h *PODActionEvidenceHandler) GetEvidence(w http.ResponseWriter, r *http.Request) {
	operatingID, err := uuid.Parse(chi.URLParam(r, "operatingTenantId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("operatingTenantId required", nil))
		return
	}
	executionID, err := uuid.Parse(chi.URLParam(r, "executionId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("executionId required", nil))
		return
	}
	actionID, err := uuid.Parse(chi.URLParam(r, "executionActionId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("executionActionId required", nil))
		return
	}
	rows, err := h.svc.GetEvidence(r.Context(), operatingID, executionID, actionID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"evidence": rows})
}

func parseCreateBody(actionType, documentTenant, operating, execution, action, stop, shipment, shipmentTenant, cargo, owner, driver, authorCompany, mime, fileName, key string) (service.CreateActionPODIntentInput, error) {
	parse := func(raw, field string) (uuid.UUID, error) {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			return uuid.Nil, apperrors.Validation(field+" is required", map[string]any{"field": field})
		}
		return id, nil
	}
	var in service.CreateActionPODIntentInput
	var err error
	in.ActionType = actionType
	if in.DocumentTenantID, err = parse(documentTenant, "documentTenantId"); err != nil {
		return in, err
	}
	if in.OperatingTenantID, err = parse(operating, "operatingTenantId"); err != nil {
		return in, err
	}
	if in.ExecutionID, err = parse(execution, "executionId"); err != nil {
		return in, err
	}
	if in.ExecutionActionID, err = parse(action, "executionActionId"); err != nil {
		return in, err
	}
	if in.ExecutionStopID, err = parse(stop, "executionStopId"); err != nil {
		return in, err
	}
	if in.ShipmentID, err = parse(shipment, "shipmentId"); err != nil {
		return in, err
	}
	if in.ShipmentTenantID, err = parse(shipmentTenant, "shipmentTenantId"); err != nil {
		return in, err
	}
	if in.CargoID, err = parse(cargo, "cargoId"); err != nil {
		return in, err
	}
	if in.OwnerCompanyID, err = parse(owner, "ownerCompanyId"); err != nil {
		return in, err
	}
	if in.AuthorDriverID, err = parse(driver, "authorDriverId"); err != nil {
		return in, err
	}
	if in.AuthorCompanyID, err = parse(authorCompany, "authorCompanyId"); err != nil {
		return in, err
	}
	in.MimeType = mime
	in.FileName = fileName
	in.IdempotencyKey = key
	return in, nil
}

func confirmationFromHeaders(r *http.Request) (domain.SubjectConfirmation, error) {
	var confirm domain.SubjectConfirmation
	var err error
	if confirm.DocumentTenantID, err = optionalHeaderUUID(r, "X-POD-Document-Tenant-ID"); err != nil {
		return confirm, err
	}
	if confirm.OperatingTenantID, err = optionalHeaderUUID(r, "X-POD-Operating-Tenant-ID"); err != nil {
		return confirm, err
	}
	if confirm.ExecutionID, err = optionalHeaderUUID(r, "X-POD-Execution-ID"); err != nil {
		return confirm, err
	}
	if confirm.ExecutionActionID, err = optionalHeaderUUID(r, "X-POD-Execution-Action-ID"); err != nil {
		return confirm, err
	}
	if confirm.ExecutionStopID, err = optionalHeaderUUID(r, "X-POD-Execution-Stop-ID"); err != nil {
		return confirm, err
	}
	if confirm.ShipmentID, err = optionalHeaderUUID(r, "X-POD-Shipment-ID"); err != nil {
		return confirm, err
	}
	if confirm.ShipmentTenantID, err = optionalHeaderUUID(r, "X-POD-Shipment-Tenant-ID"); err != nil {
		return confirm, err
	}
	if confirm.CargoID, err = optionalHeaderUUID(r, "X-POD-Cargo-ID"); err != nil {
		return confirm, err
	}
	if confirm.OwnerCompanyID, err = optionalHeaderUUID(r, "X-POD-Owner-Company-ID"); err != nil {
		return confirm, err
	}
	if confirm.AuthorDriverID, err = optionalHeaderUUID(r, "X-POD-Author-Driver-ID"); err != nil {
		return confirm, err
	}
	if confirm.AuthorCompanyID, err = optionalHeaderUUID(r, "X-POD-Author-Company-ID"); err != nil {
		return confirm, err
	}
	return confirm, nil
}

func confirmationFromStrings(documentTenant, operating, execution, action, stop, shipment, shipmentTenant, cargo, owner, driver, authorCompany string) (domain.SubjectConfirmation, error) {
	var confirm domain.SubjectConfirmation
	var err error
	if confirm.DocumentTenantID, err = optionalBodyUUID(documentTenant, "documentTenantId"); err != nil {
		return confirm, err
	}
	if confirm.OperatingTenantID, err = optionalBodyUUID(operating, "operatingTenantId"); err != nil {
		return confirm, err
	}
	if confirm.ExecutionID, err = optionalBodyUUID(execution, "executionId"); err != nil {
		return confirm, err
	}
	if confirm.ExecutionActionID, err = optionalBodyUUID(action, "executionActionId"); err != nil {
		return confirm, err
	}
	if confirm.ExecutionStopID, err = optionalBodyUUID(stop, "executionStopId"); err != nil {
		return confirm, err
	}
	if confirm.ShipmentID, err = optionalBodyUUID(shipment, "shipmentId"); err != nil {
		return confirm, err
	}
	if confirm.ShipmentTenantID, err = optionalBodyUUID(shipmentTenant, "shipmentTenantId"); err != nil {
		return confirm, err
	}
	if confirm.CargoID, err = optionalBodyUUID(cargo, "cargoId"); err != nil {
		return confirm, err
	}
	if confirm.OwnerCompanyID, err = optionalBodyUUID(owner, "ownerCompanyId"); err != nil {
		return confirm, err
	}
	if confirm.AuthorDriverID, err = optionalBodyUUID(driver, "authorDriverId"); err != nil {
		return confirm, err
	}
	if confirm.AuthorCompanyID, err = optionalBodyUUID(authorCompany, "authorCompanyId"); err != nil {
		return confirm, err
	}
	return confirm, nil
}

func optionalBodyUUID(raw, field string) (*uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, apperrors.Validation(field+" is invalid", map[string]any{"field": field})
	}
	return &id, nil
}
