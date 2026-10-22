package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
)

type recordingProjectionWriter struct {
	called bool
}

func (w *recordingProjectionWriter) AcceptRoutePlanActivation(context.Context, domain.ProjectionCommand) (domain.ProjectionAck, error) {
	w.called = true
	return domain.ProjectionAck{}, nil
}

func TestProjectionRejectsRelationshipAuthorityBeforeTransaction(t *testing.T) {
	writer := &recordingProjectionWriter{}
	handler := NewExecutionProjectionHandler(writer)
	tenantID := uuid.New()
	body := []byte(`{"relationship_version_id":"` + uuid.NewString() + `","operating_tenant_id":"` + tenantID.String() + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/transport-executions/from-route-plan-activation", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", tenantID.String())
	rec := httptest.NewRecorder()
	handler.Create(rec, req)
	if rec.Code != http.StatusBadRequest || !bytes.Contains(rec.Body.Bytes(), []byte(domain.ReasonRelationshipVersionNotAuthoritative)) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if writer.called {
		t.Fatal("relationship authority opened a transaction")
	}
}

func TestProjectionRejectsForeignTenantBeforeTransaction(t *testing.T) {
	writer := &recordingProjectionWriter{}
	handler := NewExecutionProjectionHandler(writer)
	body := []byte(`{"operating_tenant_id":"` + uuid.NewString() + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/transport-executions/from-route-plan-activation", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	rec := httptest.NewRecorder()
	handler.Create(rec, req)
	if rec.Code != http.StatusForbidden || !bytes.Contains(rec.Body.Bytes(), []byte(domain.ReasonTenantDenied)) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if writer.called {
		t.Fatal("foreign tenant opened a transaction")
	}
}
