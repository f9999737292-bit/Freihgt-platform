package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
)

type fakeProjector struct {
	cmd    domain.ProjectionCommand
	result domain.ProjectionResult
	err    error
}

func (f *fakeProjector) CreateExecutionProjectionFromActivation(_ context.Context, cmd domain.ProjectionCommand) (domain.ProjectionResult, error) {
	f.cmd = cmd
	return f.result, f.err
}

func TestProjectionRejectsBrowserAndForeignCaller(t *testing.T) {
	handler := NewTransportExecutionHandler(&fakeProjector{})
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/transport-executions/projections", strings.NewReader(`{}`))
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	rec := httptest.NewRecorder()
	handler.CreateFromActivation(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestProjectionIgnoresClientTenantHeader(t *testing.T) {
	operating := uuid.New()
	fake := &fakeProjector{result: domain.ProjectionResult{
		ExecutionID:  uuid.New(),
		RevisionID:   uuid.New(),
		ActivationID: uuid.New(),
		Created:      true,
	}}
	handler := NewTransportExecutionHandler(fake)
	body := `{
		"activation_id":"` + fake.result.ActivationID.String() + `",
		"activation_version":1,
		"activation_status":"PENDING_EXECUTION",
		"route_plan_id":"` + uuid.NewString() + `",
		"route_plan_version":1,
		"planning_mode":"DEPOT_START",
		"operating_tenant_id":"` + operating.String() + `",
		"carrier_company_id":"` + uuid.NewString() + `",
		"evaluation_fingerprint":"fp",
		"execution_subjects":[],
		"stops":[],
		"actions":[]
	}`
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/transport-executions/projections", strings.NewReader(body))
	req.Header.Set(headerInternalServiceName, domain.TrustedProjectionCaller)
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	rec := httptest.NewRecorder()
	handler.CreateFromActivation(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fake.cmd.OperatingTenantID != operating {
		t.Fatalf("operating tenant = %s", fake.cmd.OperatingTenantID)
	}
}
