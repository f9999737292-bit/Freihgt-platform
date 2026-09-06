//go:build integration

package studio

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

func newBrowserQuestionnaireRouter(env *testEnv) http.Handler {
	h := handlers.NewQuestionnaireHandler(env.qSvc)
	r := chi.NewRouter()
	r.Use(captureBrowserDownstreamHeaders)
	r.Route("/v1/rfx-events", func(r chi.Router) {
		r.Get("/{id}/studio", h.GetStudio)
		r.Get("/{id}/questionnaire", h.GetQuestionnaire)
		r.Post("/{id}/save-draft", h.SaveDraft)
		r.Post("/{id}/validate-publish", h.ValidatePublish)
		r.Post("/{id}/sections", h.CreateSection)
		r.Patch("/{id}/sections/{section_id}", h.UpdateSection)
		r.Delete("/{id}/sections/{section_id}", h.DeleteSection)
		r.Post("/{id}/sections/reorder", h.ReorderSections)
		r.Post("/{id}/questions", h.CreateQuestion)
		r.Patch("/{id}/questions/{question_id}", h.UpdateQuestion)
		r.Delete("/{id}/questions/{question_id}", h.DeleteQuestion)
		r.Post("/{id}/questions/{question_id}/duplicate", h.DuplicateQuestion)
		r.Post("/{id}/questions/reorder", h.ReorderQuestions)
		r.Post("/{id}/questions/{question_id}/options", h.CreateOption)
		r.Patch("/{id}/questions/{question_id}/options/{option_id}", h.UpdateOption)
		r.Delete("/{id}/questions/{question_id}/options/{option_id}", h.DeleteOption)
		r.Post("/{id}/rules", h.CreateRule)
		r.Patch("/{id}/rules/{rule_id}", h.UpdateRule)
		r.Delete("/{id}/rules/{rule_id}", h.DeleteRule)
	})
	return r
}
