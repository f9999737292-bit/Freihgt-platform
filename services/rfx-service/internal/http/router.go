package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/rfx-service/internal/http/handlers"
	"github.com/freight-platform/rfx-service/internal/service"
	"github.com/freight-platform/shared-go/metrics"
	"github.com/freight-platform/shared-go/observability"
)

const serviceName = "rfx-service"

func NewRouter(
	log *slog.Logger,
	db observability.DatabasePinger,
	rfxSvc *service.RfxService,
	frSvc *service.FreightRequestService,
	bidSvc *service.BidService,
	bidRevSvc *service.BidRevisionService,
	respBidSvc *service.ResponseBidService,
	evalSvc *service.EvaluationService,
	authz handlers.PermissionChecker,
) http.Handler {
	rfxHandler := handlers.NewRfxHandler(rfxSvc)
	frHandler := handlers.NewFreightRequestHandler(frSvc)
	bidHandler := handlers.NewBidHandler(bidSvc)
	bidRevHandler := handlers.NewBidRevisionHandler(bidRevSvc)
	respBidHandler := handlers.NewResponseBidHandler(respBidSvc)
	evalHandler := handlers.NewEvaluationHandler(evalSvc, authz)

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
		r.Post("/{id}/lots", rfxHandler.CreateLot)
		r.Get("/{id}/lots", rfxHandler.ListLots)
		r.Post("/{id}/participants", rfxHandler.AddParticipant)
		r.Get("/{id}/participants", rfxHandler.ListParticipants)
		r.Post("/{id}/responses", rfxHandler.CreateResponse)
		r.Get("/{id}/bids", respBidHandler.ListEventBids)
		r.Get("/{id}/responses/mine", respBidHandler.GetCurrent)
		r.Post("/{id}/responses/{response_id}/revisions", respBidHandler.SubmitRevision)
		r.Get("/{id}/responses/{response_id}/revisions", respBidHandler.ListRevisions)
		r.Post("/{id}/evaluate", evalHandler.RunEvaluation)
	})

	r.Post("/v1/scoring-templates", evalHandler.CreateScoringTemplate)
	r.Post("/v1/allocation-scenarios", evalHandler.RunAllocationScenario)
	r.Post("/v1/award-proposals", evalHandler.CreateAwardProposal)
	r.Post("/v1/award-proposals/{proposal_id}/submit", evalHandler.SubmitAwardProposal)
	r.Post("/v1/award-proposals/{proposal_id}/approve", evalHandler.ApproveAwardProposal)
	r.Post("/v1/award-proposals/{proposal_id}/reject", evalHandler.RejectAwardProposal)
	r.Post("/v1/award-proposals/{proposal_id}/finalize", evalHandler.FinalizeAward)

	r.Post("/v1/rfx-lots/{lot_id}/lanes", rfxHandler.CreateLane)
	r.Post("/v1/rfx-responses/{response_id}/submit", rfxHandler.SubmitResponse)

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
		r.Post("/{id}/revisions", bidRevHandler.SubmitRevision)
		r.Get("/{id}/revisions/current", bidRevHandler.GetCurrent)
		r.Get("/{id}/revisions", bidRevHandler.ListRevisions)
	})

	return r
}
