package executionproj

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/shared-go/internalauth"
	"github.com/freight-platform/shared-go/lowcode"
)

func TestHTTPProjectSendsInternalAuthAndValidatesCorrelation(t *testing.T) {
	tenant := uuid.New()
	activation := uuid.New()
	plan := uuid.New()
	execution := uuid.New()
	revision := uuid.New()
	var gotName, gotTenant, gotToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotName = r.Header.Get("X-Internal-Service-Name")
		gotTenant = r.Header.Get(lowcode.HeaderTenantID)
		gotToken = r.Header.Get(internalauth.HeaderName)
		if r.URL.Path != path || r.Method != http.MethodPost {
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var cmd Command
		if err := json.Unmarshal(body, &cmd); err != nil || cmd.ActivationID != activation {
			t.Errorf("body %s", body)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Ack{
			OperatingTenantID: tenant, ActivationID: activation, RoutePlanID: plan,
			ExecutionID: execution, RevisionID: revision,
		})
	}))
	defer server.Close()
	ack, err := NewHTTP(server.URL, "test-token").Project(context.Background(), tenant, Command{
		ActivationID: activation, OperatingTenantID: tenant, RoutePlanID: plan, CarrierCompanyID: uuid.New(),
		ExecutionSubjects: []Subject{}, Stops: []Stop{}, Actions: []Action{},
	})
	if err != nil || ack.ExecutionID != execution || ack.RevisionID != revision || ack.RoutePlanID != plan {
		t.Fatalf("%+v %v", ack, err)
	}
	if gotName != callerName || gotTenant != tenant.String() || gotToken != "test-token" {
		t.Fatalf("name %s tenant %s token set %v", gotName, gotTenant, gotToken != "")
	}
}

func TestHTTPProjectClassifiesLostResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()
	_, err := NewHTTP(server.URL, "test-token").Project(context.Background(), uuid.New(), Command{})
	callErr, ok := err.(*Error)
	if !ok || !callErr.Temporary {
		t.Fatal(err)
	}
	_, err = NewHTTP("", "").Project(context.Background(), uuid.New(), Command{})
	callErr, ok = err.(*Error)
	if !ok || !callErr.Temporary {
		t.Fatal(err)
	}
}
