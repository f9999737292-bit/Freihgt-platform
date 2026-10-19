package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

func TestCompanyRelationshipClientFailClosed(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	defer server.Close()
	request := shipmentCreateRequest()
	for _, client := range []*CompanyRelationshipCreateClient{
		nil,
		NewCompanyRelationshipCreateClient("", "dedicated", nil),
		NewCompanyRelationshipCreateClient(server.URL, "", nil),
	} {
		if _, err := client.Create(context.Background(), request); err == nil {
			t.Fatal("expected fail closed")
		}
	}
	if _, err := NewCompanyRelationshipCreateClient(server.URL, "dedicated", nil).Create(nil, request); err == nil {
		t.Fatal("nil context was accepted")
	}
	request.SourceType = "RFX_AWARD"
	if _, err := NewCompanyRelationshipCreateClient(server.URL, "dedicated", nil).Create(context.Background(), request); err == nil {
		t.Fatal("rfx source was accepted")
	}
	request.SourceType = "AWARD_TRANSPORT_ORDER_LINK"
	if _, err := NewCompanyRelationshipCreateClient(server.URL, "dedicated", nil).Create(context.Background(), request); err == nil {
		t.Fatal("award link source was accepted")
	}
	if called {
		t.Fatal("fail-closed client called company-service")
	}
}

func TestCompanyRelationshipClientUsesDedicatedCredentialOnce(t *testing.T) {
	var seen http.Header
	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Clone()
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		if r.URL.Path != companyRelationshipCreatePath {
			t.Fatalf("path %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"relationship_request_id":"` + uuid.NewString() + `","request_status":"PENDING","acceptance_token":"one-time-token"}`))
	}))
	defer server.Close()
	client := NewCompanyRelationshipCreateClient(server.URL, "dedicated-shipment-token", nil)
	out, err := client.Create(context.Background(), shipmentCreateRequest())
	if err != nil || !out.Created || out.AcceptanceToken != "one-time-token" {
		t.Fatalf("create: %v %+v", err, out)
	}
	if seen.Get("X-Internal-Service-Token") != "dedicated-shipment-token" {
		t.Fatalf("credential %q", seen.Get("X-Internal-Service-Token"))
	}
	if seen.Get("X-Internal-Service-Name") != "" {
		t.Fatal("client sent X-Internal-Service-Name")
	}
	if strings.Contains(body, "RFX") || !strings.Contains(body, `"source_type":"SHIPMENT"`) {
		t.Fatalf("body %s", body)
	}
	replayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"relationship_request_id":"` + out.RequestID.String() + `","request_status":"PENDING","acceptance_token":"must-not-return"}`))
	}))
	defer replayServer.Close()
	replay, err := NewCompanyRelationshipCreateClient(replayServer.URL, "dedicated-shipment-token", nil).Create(context.Background(), shipmentCreateRequest())
	if err != nil || replay.Created || replay.AcceptanceToken != "" {
		t.Fatalf("replay leaked token: %v %+v", err, replay)
	}
}

func TestCompanyRelationshipClientLostTransportDoesNotRetry(t *testing.T) {
	calls := 0
	client := NewCompanyRelationshipCreateClient("http://company.invalid", "dedicated", &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			calls++
			return nil, errors.New("connection reset")
		}),
	})
	_, err := client.Create(context.Background(), shipmentCreateRequest())
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeRelationshipSourceUnavailable || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestCompanyRelationshipClientRejectsRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://127.0.0.1/token", http.StatusFound)
	}))
	defer server.Close()
	_, err := NewCompanyRelationshipCreateClient(server.URL, "dedicated", nil).Create(context.Background(), shipmentCreateRequest())
	if err == nil {
		t.Fatal("redirect was followed")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func shipmentCreateRequest() CompanyRelationshipCreateRequest {
	return CompanyRelationshipCreateRequest{
		SourceType: domain.ShipmentSourceType, SourceID: uuid.New(), SourceTenantID: uuid.New(),
		Fingerprint: "abc", ShipperCarrierCompanyID: uuid.New(), RequestingCompanyID: uuid.New(),
		RequestingUserID: uuid.New(), IdempotencyKey: "key-1",
	}
}
