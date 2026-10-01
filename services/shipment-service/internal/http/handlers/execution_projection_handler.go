package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/platform/respond"
)

const (
	// HeaderInternalServiceName names the internal caller. It is accepted only after the internal token check.
	HeaderInternalServiceName = "X-Internal-Service-Name"
	// AuthorizedProjectionCaller is the only service allowed to open a transport execution from a route-plan activation.
	AuthorizedProjectionCaller = "network-optimizer-service"
)

// ExecutionProjectionWriter commits a route-plan activation into one transport execution.
type ExecutionProjectionWriter interface {
	AcceptRoutePlanActivation(ctx context.Context, cmd domain.ProjectionCommand) (domain.ProjectionAck, error)
}

// RequireNetworkOptimizerCaller rejects any caller other than network-optimizer-service.
func RequireNetworkOptimizerCaller(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(r.Header.Get(HeaderInternalServiceName)) != AuthorizedProjectionCaller {
			respond.Error(w, apperrors.Forbidden("caller is not allowed to project an execution"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

type ExecutionProjectionHandler struct {
	writer ExecutionProjectionWriter
}

func NewExecutionProjectionHandler(writer ExecutionProjectionWriter) *ExecutionProjectionHandler {
	return &ExecutionProjectionHandler{writer: writer}
}

func (h *ExecutionProjectionHandler) Create(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.writer == nil {
		respond.Error(w, apperrors.NotFound("transport execution not found"))
		return
	}
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var cmd domain.ProjectionCommand
	if err := decodeStrictJSON(r, &cmd); err != nil {
		respond.Error(w, err)
		return
	}
	if cmd.OperatingTenantID != tenantID {
		respond.Error(w, domain.ExecutionCommandError(domain.ReasonTenantDenied, true))
		return
	}
	cmd.OperatingTenantID = tenantID
	ack, err := h.writer.AcceptRoutePlanActivation(r.Context(), cmd)
	if err != nil {
		respond.Error(w, err)
		return
	}
	status := http.StatusOK
	if ack.Created {
		status = http.StatusCreated
	}
	respond.JSON(w, status, ack)
}
