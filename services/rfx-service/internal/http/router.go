package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/rfx-service/internal/config"
	"github.com/freight-platform/rfx-service/internal/http/handlers"
	"github.com/freight-platform/rfx-service/internal/service"
	"github.com/freight-platform/shared-go/internalauth"
	"github.com/freight-platform/shared-go/metrics"
	"github.com/freight-platform/shared-go/observability"
)

const serviceName = "rfx-service"

func NewRouter(
	log *slog.Logger,
	db observability.DatabasePinger,
	cfg config.Config,
	rfxSvc *service.RfxService,
	qSvc *service.QuestionnaireService,
	crSvc *service.CarrierResponseService,
	scoreModelSvc *service.ScoreModelService,
	scoringSvc *service.ScoringService,
	frSvc *service.FreightRequestService,
	bidSvc *service.BidService,
	pricingSvc *service.PricingService,
) http.Handler {
	rfxHandler := handlers.NewRfxHandler(rfxSvc)
	qHandler := handlers.NewQuestionnaireHandler(qSvc)
	crHandler := handlers.NewCarrierResponseHandler(crSvc)
	scoreHandler := handlers.NewScoreHandler(scoreModelSvc, scoringSvc, rfxSvc)
	frHandler := handlers.NewFreightRequestHandler(frSvc)
	bidHandler := handlers.NewBidHandler(bidSvc)
	pricingHandler := handlers.NewPricingHandler(pricingSvc)
	internalAuth := internalauth.Config{Token: cfg.InternalServiceToken, Environment: cfg.Environment}

	r := chi.NewRouter()
	observability.Mount(r, observability.MountOptions{
		ServiceName: serviceName,
		Log:         log,
		Metrics:     metrics.New(serviceName),
		DB:          db,
	})

	r.Route("/v1/rfx-events", func(r chi.Router) {
		r.Post("/", rfxHandler.CreateEvent)
		r.Get("/", rfxHandler.ListEvents)
		r.Get("/{id}", rfxHandler.GetEvent)
		r.Patch("/{id}", rfxHandler.UpdateEvent)
		r.Post("/{id}/publish", rfxHandler.PublishEvent)
		r.Post("/{id}/cancel", rfxHandler.CancelEvent)
		r.Post("/{id}/transitions/{command}", rfxHandler.TransitionEvent)
		r.Post("/{id}/send-invitations", rfxHandler.SendInvitations)
		r.Post("/{id}/open-questions", rfxHandler.OpenQuestions)
		r.Post("/{id}/open-responses", rfxHandler.OpenResponses)
		r.Post("/{id}/close-responses", rfxHandler.CloseResponses)
		r.Post("/{id}/start-evaluation", rfxHandler.StartEvaluation)
		r.Post("/{id}/shortlist", rfxHandler.Shortlist)
		r.Post("/{id}/award", rfxHandler.AwardEvent)
		r.Post("/{id}/archive", rfxHandler.ArchiveEvent)
		r.Post("/{id}/extend-deadline", rfxHandler.ExtendDeadline)
		r.Post("/{id}/lots", rfxHandler.CreateLot)
		r.Get("/{id}/lots", rfxHandler.ListLots)
		r.Post("/{id}/participants", rfxHandler.AddParticipant)
		r.Get("/{id}/participants", rfxHandler.ListParticipants)
		r.Post("/{id}/responses", rfxHandler.CreateResponse)
		r.Get("/{id}/own-response", rfxHandler.GetOwnResponse)
		r.Get("/{id}/own-participant", rfxHandler.GetCarrierParticipant)
		r.Get("/{id}/responses", rfxHandler.ListEvaluationResponses)
		r.Post("/{id}/evaluation/recalculate", rfxHandler.RecalculateEvaluation)
		r.Post("/{id}/award-response", rfxHandler.AwardResponse)
		r.Get("/{id}/audit-events", rfxHandler.ListEventAuditEvents)
		r.Get("/{id}/own-award", rfxHandler.GetOwnAward)
		r.Post("/{id}/transport-orders", rfxHandler.ConvertAwardToTransportOrders)
		r.Get("/{id}/transport-orders", rfxHandler.ListAwardTransportOrders)

		// RFx v3.0B questionnaire studio
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

		// RFx v3.0C carrier questionnaire response
		r.Get("/{id}/carrier-response", crHandler.GetCarrierResponse)
		r.Post("/{id}/carrier-response/start", crHandler.StartCarrierResponse)
		r.Patch("/{id}/carrier-response/answers", crHandler.PatchCarrierAnswers)
		r.Post("/{id}/carrier-response/validate", crHandler.ValidateCarrierResponse)
		r.Post("/{id}/carrier-response/submit", crHandler.SubmitCarrierResponse)
		r.Get("/{id}/carrier-response/summary", crHandler.GetCarrierResponseSummary)

		// RFx v3.0D scoring
		r.Get("/{id}/score-model", scoreHandler.GetScoreModel)
		r.Put("/{id}/score-model", scoreHandler.PutScoreModel)
		r.Post("/{id}/score-model/validate", scoreHandler.ValidateScoreModel)
		r.Post("/{id}/score-model/publish", scoreHandler.PublishScoreModel)
		r.Get("/{id}/responses/{response_id}/score", scoreHandler.GetResponseScore)
		r.Get("/{id}/responses/{response_id}/score/explanation", scoreHandler.GetResponseScoreExplanation)
	})

	r.Route("/v1/carrier/rfx-events", func(r chi.Router) {
		r.Get("/", rfxHandler.ListCarrierInvitedEvents)
	})

	r.Post("/v1/rfx-lots/{lot_id}/lanes", rfxHandler.CreateLane)
	r.Get("/v1/rfx-lots/{lot_id}/lanes", rfxHandler.ListLanes)

	r.Post("/v1/rfx-responses/{response_id}/submit", rfxHandler.SubmitResponse)
	r.Get("/v1/rfx-responses/{response_id}", rfxHandler.GetResponse)
	r.Patch("/v1/rfx-responses/{response_id}", rfxHandler.UpdateResponseCommercial)
	r.Patch("/v1/rfx-responses/{response_id}/evaluation", rfxHandler.UpdateManualScore)
	r.Post("/v1/rfx-responses/{response_id}/shortlist", rfxHandler.AddResponseToShortlist)
	r.Delete("/v1/rfx-responses/{response_id}/shortlist", rfxHandler.RemoveResponseFromShortlist)

	r.Route("/v1/freight-requests", func(r chi.Router) {
		r.Post("/from-transport-order", frHandler.CreateFromTransportOrder)
		r.Get("/", frHandler.List)
		r.Get("/{id}", frHandler.GetByID)
		r.Post("/{id}/publish", frHandler.Publish)
		r.Post("/{id}/bids", bidHandler.CreateBid)
		r.Get("/{id}/bids", bidHandler.ListBids)
	})

	r.Route("/v1/bids", func(r chi.Router) {
		r.Get("/{id}", bidHandler.GetByID)
		r.Post("/{id}/submit", bidHandler.SubmitBid)
		r.Post("/{id}/accept", bidHandler.AcceptBid)
	})

	r.Route("/internal/v1/pricing", func(r chi.Router) {
		r.Use(internalAuth.Middleware)
		r.Get("/award-context/{sourceId}", pricingHandler.GetAwardLinkContext)
		r.Get("/award-scope/{eventId}", pricingHandler.GetAwardScopeContext)
		r.Get("/bid-context/{bidId}", pricingHandler.GetBidContext)
	})

	return r
}
