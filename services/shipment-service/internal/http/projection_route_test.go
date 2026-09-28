package http

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/freight-platform/shipment-service/internal/service"
)

func TestTransportExecutionProjectionIsInternal(t *testing.T) {
	router := NewRouter(
		slog.New(slog.DiscardHandler),
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		service.NewTransportExecutionService(nil),
		"test-token",
	)
	publicReq := httptest.NewRequest(http.MethodPost, "/v1/transport-executions/projections", strings.NewReader(`{}`))
	publicRec := httptest.NewRecorder()
	router.ServeHTTP(publicRec, publicReq)
	if publicRec.Code != http.StatusNotFound {
		t.Fatalf("public status=%d", publicRec.Code)
	}

	missingToken := httptest.NewRequest(http.MethodPost, "/internal/v1/transport-executions/projections", strings.NewReader(`{}`))
	missingRec := httptest.NewRecorder()
	router.ServeHTTP(missingRec, missingToken)
	if missingRec.Code != http.StatusUnauthorized {
		t.Fatalf("missing token status=%d body=%s", missingRec.Code, missingRec.Body.String())
	}
}
