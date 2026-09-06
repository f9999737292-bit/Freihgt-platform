//go:build integration

package carrierresponse

import (
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/rfx-service/internal/http/handlers"
)

var (
	browserDownstreamHeadersMu sync.Mutex
	browserDownstreamHeaders   http.Header
)

func captureBrowserDownstreamHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		browserDownstreamHeadersMu.Lock()
		browserDownstreamHeaders = r.Header.Clone()
		browserDownstreamHeadersMu.Unlock()
		next.ServeHTTP(w, r)
	})
}

func lastBrowserDownstreamHeaders() http.Header {
	browserDownstreamHeadersMu.Lock()
	defer browserDownstreamHeadersMu.Unlock()
	if browserDownstreamHeaders == nil {
		return http.Header{}
	}
	return browserDownstreamHeaders.Clone()
}

func newBrowserCarrierRouter(env *testEnv) http.Handler {
	rfxHandler := handlers.NewRfxHandler(env.rfxSvc)
	crHandler := handlers.NewCarrierResponseHandler(env.crSvc)
	r := chi.NewRouter()
	r.Use(captureBrowserDownstreamHeaders)
	r.Route("/v1/rfx-events", func(r chi.Router) {
		r.Get("/{id}", rfxHandler.GetEvent)
		r.Get("/{id}/carrier-response", crHandler.GetCarrierResponse)
		r.Post("/{id}/carrier-response/start", crHandler.StartCarrierResponse)
		r.Patch("/{id}/carrier-response/answers", crHandler.PatchCarrierAnswers)
		r.Post("/{id}/carrier-response/validate", crHandler.ValidateCarrierResponse)
		r.Post("/{id}/carrier-response/submit", crHandler.SubmitCarrierResponse)
		r.Get("/{id}/carrier-response/summary", crHandler.GetCarrierResponseSummary)
	})
	r.Route("/v1/carrier/rfx-events", func(r chi.Router) {
		r.Get("/", rfxHandler.ListCarrierInvitedEvents)
	})
	return r
}
