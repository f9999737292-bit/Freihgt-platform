//go:build integration

package studio

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/rfx-service/internal/http/handlers"
)

func newBrowserScoringV3Router(env *testEnv) http.Handler {
	qHandler := handlers.NewQuestionnaireHandler(env.qSvc)
	scoreHandler := handlers.NewScoreHandler(env.scoreModelSvc, env.scoringSvc, env.rfxSvc)
	rfxHandler := handlers.NewRfxHandler(env.rfxSvc)
	crHandler := handlers.NewCarrierResponseHandler(env.crSvc)
	r := chi.NewRouter()
	r.Use(captureBrowserDownstreamHeaders)
	r.Route("/v1/rfx-events", func(r chi.Router) {
		r.Get("/{id}", rfxHandler.GetEvent)
		r.Get("/{id}/responses", rfxHandler.ListEvaluationResponses)
		r.Get("/{id}/audit-events", rfxHandler.ListEventAuditEvents)
		r.Post("/{id}/evaluation/recalculate", rfxHandler.RecalculateEvaluation)
		r.Get("/{id}/transport-orders", rfxHandler.ListAwardTransportOrders)
		r.Get("/{id}/studio", qHandler.GetStudio)
		r.Get("/{id}/questionnaire", qHandler.GetQuestionnaire)
		r.Post("/{id}/save-draft", qHandler.SaveDraft)
		r.Post("/{id}/validate-publish", qHandler.ValidatePublish)
		r.Post("/{id}/sections", qHandler.CreateSection)
		r.Patch("/{id}/sections/{section_id}", qHandler.UpdateSection)
		r.Delete("/{id}/sections/{section_id}", qHandler.DeleteSection)
		r.Post("/{id}/sections/reorder", qHandler.ReorderSections)
		r.Post("/{id}/questions", qHandler.CreateQuestion)
		r.Patch("/{id}/questions/{question_id}", qHandler.UpdateQuestion)
		r.Delete("/{id}/questions/{question_id}", qHandler.DeleteQuestion)
		r.Post("/{id}/questions/{question_id}/duplicate", qHandler.DuplicateQuestion)
		r.Post("/{id}/questions/reorder", qHandler.ReorderQuestions)
		r.Post("/{id}/questions/{question_id}/options", qHandler.CreateOption)
		r.Patch("/{id}/questions/{question_id}/options/{option_id}", qHandler.UpdateOption)
		r.Delete("/{id}/questions/{question_id}/options/{option_id}", qHandler.DeleteOption)
		r.Post("/{id}/rules", qHandler.CreateRule)
		r.Patch("/{id}/rules/{rule_id}", qHandler.UpdateRule)
		r.Delete("/{id}/rules/{rule_id}", qHandler.DeleteRule)
		r.Get("/{id}/score-model", scoreHandler.GetScoreModel)
		r.Put("/{id}/score-model", scoreHandler.PutScoreModel)
		r.Post("/{id}/score-model/validate", scoreHandler.ValidateScoreModel)
		r.Post("/{id}/score-model/publish", scoreHandler.PublishScoreModel)
		r.Get("/{id}/responses/{response_id}/score", scoreHandler.GetResponseScore)
		r.Get("/{id}/responses/{response_id}/score/explanation", scoreHandler.GetResponseScoreExplanation)
		r.Get("/{id}/carrier-response", crHandler.GetCarrierResponse)
		r.Post("/{id}/carrier-response/start", crHandler.StartCarrierResponse)
		r.Patch("/{id}/carrier-response/answers", crHandler.PatchCarrierAnswers)
		r.Post("/{id}/carrier-response/validate", crHandler.ValidateCarrierResponse)
		r.Post("/{id}/carrier-response/submit", crHandler.SubmitCarrierResponse)
	})
	return r
}
