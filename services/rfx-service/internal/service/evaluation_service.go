package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
)

type evaluationStore interface {
	LotBelongsToEvent(ctx context.Context, lotID, eventID, tenantID uuid.UUID) (bool, error)
	ReplaceOfferLines(ctx context.Context, responseID, tenantID uuid.UUID, lines []domain.UpsertOfferLineInput) ([]domain.RfxResponseOfferLine, error)
	ListOfferLinesByResponse(ctx context.Context, responseID, tenantID uuid.UUID) ([]domain.RfxResponseOfferLine, error)
	ListSubmittedResponsesByEvent(ctx context.Context, eventID, tenantID uuid.UUID) ([]domain.RfxResponse, error)
	UpdateResponseEvaluation(ctx context.Context, responseID, tenantID uuid.UUID, commercialScore, manualScore, totalScore *float64, rank *int) error
	UpdateParticipantStatus(ctx context.Context, eventID, companyID, tenantID uuid.UUID, status string) error
	GetAwardByEvent(ctx context.Context, eventID, tenantID uuid.UUID) (*domain.RfxAward, error)
	GetAwardForCarrier(ctx context.Context, eventID, carrierCompanyID, tenantID uuid.UUID) (*domain.RfxAward, error)
	CreateAwardTransactional(ctx context.Context, in domain.CreateRfxAwardInput, newEventStatus string, preCommit func(context.Context, pgx.Tx) error) (*domain.RfxAward, error)
}

func (s *RfxService) evalStore() evaluationStore {
	if store, ok := s.repo.(evaluationStore); ok {
		return store
	}
	return nil
}

func (s *RfxService) auditReader() AuditReader {
	if reader, ok := s.audit.(AuditReader); ok {
		return reader
	}
	return nil
}

func (s *RfxService) UpdateResponseCommercial(ctx context.Context, actor domain.ActorContext, responseID uuid.UUID, lines []domain.UpsertOfferLineInput) (*domain.RfxResponse, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	store := s.evalStore()
	if store == nil {
		return nil, apperrors.Internal("evaluation store unavailable", nil)
	}
	response, err := s.repo.GetResponseByID(ctx, responseID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	event, err := s.repo.GetEventByID(ctx, response.RfxEventID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	_, carrierIDs, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	if !carrierCanViewResponse(carrierIDs, response.ParticipantCompanyID) {
		return nil, apperrors.NotFound("rfx response not found")
	}
	if err := domain.ValidateUpdateDraftResponse(response.Status); err != nil {
		return nil, err
	}
	lotCount, err := s.repo.CountLotsByEvent(ctx, event.ID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	eventCurrency := ""
	if event.CurrencyCode != nil {
		eventCurrency = *event.CurrencyCode
	}
	for _, line := range lines {
		if err := domain.ValidateOfferLineInput(line, lotCount); err != nil {
			return nil, err
		}
		ok, err := store.LotBelongsToEvent(ctx, line.RfxLotID, event.ID, actor.TenantID)
		if err != nil {
			return nil, err
		}
		if line.RfxLotID != uuid.Nil && !ok {
			return nil, apperrors.NotFound("rfx lot not found")
		}
	}
	if err := domain.ValidateOfferLinesForEventCurrency(lines, eventCurrency); err != nil {
		return nil, err
	}
	offerLines, err := store.ReplaceOfferLines(ctx, responseID, actor.TenantID, lines)
	if err != nil {
		return nil, err
	}
	response.OfferLines = offerLines
	return response, nil
}

func (s *RfxService) ListEvaluationResponses(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID) ([]domain.EvaluationResponseView, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	event, _, err := s.requireBuyerEventAccess(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	store := s.evalStore()
	if store == nil {
		return nil, apperrors.Internal("evaluation store unavailable", nil)
	}
	responses, err := store.ListSubmittedResponsesByEvent(ctx, eventID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	participants, err := s.repo.ListParticipants(ctx, eventID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	participantByCompany := map[uuid.UUID]domain.RfxParticipant{}
	for _, p := range participants {
		participantByCompany[p.CompanyID] = p
	}
	lotCount, err := s.repo.CountLotsByEvent(ctx, eventID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	eventCurrency := ""
	if event.CurrencyCode != nil {
		eventCurrency = *event.CurrencyCode
	}
	var award *domain.RfxAward
	if a, err := store.GetAwardByEvent(ctx, eventID, actor.TenantID); err == nil {
		award = a
	}
	views := make([]domain.EvaluationResponseView, 0, len(responses))
	for _, response := range responses {
		lines, err := store.ListOfferLinesByResponse(ctx, response.ID, actor.TenantID)
		if err != nil {
			return nil, err
		}
		response.OfferLines = lines
		total, currency, currencyOK := domain.ResponseCommercialSummary(lines, eventCurrency)
		complete := domain.ResponseOfferComplete(lotCount, lines)
		participant := participantByCompany[response.ParticipantCompanyID]
		view := domain.EvaluationResponseView{
			Response:          response,
			ParticipantStatus: participant.Status,
			TotalAmount:       total,
			CurrencyCode:      currency,
			Comparable:        currencyOK && complete && total > 0,
			Shortlisted:       participant.Status == domain.ParticipantStatusShortlisted,
			Awarded:           award != nil && award.RfxResponseID == response.ID,
			OfferComplete:     complete,
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *RfxService) RecalculateEvaluation(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID) ([]domain.EvaluationResponseView, error) {
	views, err := s.ListEvaluationResponses(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	store := s.evalStore()
	if store == nil {
		return nil, apperrors.Internal("evaluation store unavailable", nil)
	}
	candidates := make([]domain.EvaluationCandidate, 0, len(views))
	for _, view := range views {
		if !domain.IsResponseEligibleForEvaluation(view.Response.Status) {
			continue
		}
		candidates = append(candidates, domain.EvaluationCandidate{
			ResponseID:           view.Response.ID,
			ParticipantCompanyID: view.Response.ParticipantCompanyID,
			TotalAmount:          view.TotalAmount,
			CurrencyCode:         view.CurrencyCode,
			ManualScore:          view.Response.ManualScore,
			ParticipantStatus:    view.ParticipantStatus,
			Comparable:           view.Comparable,
		})
	}
	scored := domain.ComputeCommercialScores(candidates)
	ranked := domain.RankEvaluationCandidates(scored)
	rankByResponse := map[uuid.UUID]domain.EvaluationCandidate{}
	for _, c := range ranked {
		rankByResponse[c.ResponseID] = c
	}
	for i := range views {
		c, ok := rankByResponse[views[i].Response.ID]
		if !ok {
			continue
		}
		commercial := c.CommercialScore
		total := c.TotalScore
		rank := c.Rank
		var manual *float64
		if c.ManualScore != nil {
			manual = c.ManualScore
		}
		if err := store.UpdateResponseEvaluation(ctx, views[i].Response.ID, actor.TenantID, &commercial, manual, &total, &rank); err != nil {
			return nil, err
		}
		views[i].Response.CommercialScore = &commercial
		views[i].Response.TotalScore = &total
		views[i].Response.EvaluationRank = &rank
		if manual != nil {
			views[i].Response.ManualScore = manual
		}
		views[i].Comparable = c.Comparable
	}
	return views, nil
}

func (s *RfxService) UpdateManualScore(ctx context.Context, actor domain.ActorContext, responseID uuid.UUID, score float64) (*domain.RfxResponse, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	if err := domain.ValidateManualScore(score); err != nil {
		return nil, err
	}
	response, err := s.repo.GetResponseByID(ctx, responseID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if _, _, err := s.requireBuyerEventAccess(ctx, actor, response.RfxEventID); err != nil {
		return nil, err
	}
	if !domain.IsResponseEligibleForEvaluation(response.Status) {
		return nil, apperrors.Validation("only submitted responses can be scored", map[string]any{"field": "status"})
	}
	store := s.evalStore()
	if store == nil {
		return nil, apperrors.Internal("evaluation store unavailable", nil)
	}
	lines, err := store.ListOfferLinesByResponse(ctx, responseID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	event, err := s.repo.GetEventByID(ctx, response.RfxEventID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	eventCurrency := ""
	if event.CurrencyCode != nil {
		eventCurrency = *event.CurrencyCode
	}
	total, currency, currencyOK := domain.ResponseCommercialSummary(lines, eventCurrency)
	manual := score
	var commercial float64
	var totalScore float64
	if currencyOK && total > 0 {
		commercial = 100
		totalScore = domain.CombineScoresPublic(commercial, &manual)
	} else {
		totalScore = manual
	}
	if err := store.UpdateResponseEvaluation(ctx, responseID, actor.TenantID, &commercial, &manual, &totalScore, response.EvaluationRank); err != nil {
		return nil, err
	}
	response.ManualScore = &manual
	response.CommercialScore = &commercial
	response.TotalScore = &totalScore
	response.OfferLines = lines
	_ = currency
	return response, nil
}

func (s *RfxService) AddToShortlist(ctx context.Context, actor domain.ActorContext, responseID uuid.UUID) error {
	response, err := s.getEligibleBuyerResponse(ctx, actor, responseID)
	if err != nil {
		return err
	}
	return s.runRfx(ctx, func(rfx RfxStore, audit AuditRecorder) error {
		eval, ok := rfx.(evaluationStore)
		if !ok {
			return apperrors.Internal("evaluation store unavailable", nil)
		}
		if err := eval.UpdateParticipantStatus(ctx, response.RfxEventID, response.ParticipantCompanyID, actor.TenantID, domain.ParticipantStatusShortlisted); err != nil {
			return err
		}
		event, _ := s.repo.GetEventByID(ctx, response.RfxEventID, actor.TenantID)
		ownerCompanyID := event.OwnerCompanyID
		return recordAudit(ctx, audit, actor, ownerCompanyID, "rfx_event", response.RfxEventID, "shortlist_response", map[string]any{
			"response_id": response.ID.String(),
		})
	})
}

func (s *RfxService) RemoveFromShortlist(ctx context.Context, actor domain.ActorContext, responseID uuid.UUID) error {
	response, err := s.getEligibleBuyerResponse(ctx, actor, responseID)
	if err != nil {
		return err
	}
	return s.runRfx(ctx, func(rfx RfxStore, audit AuditRecorder) error {
		eval, ok := rfx.(evaluationStore)
		if !ok {
			return apperrors.Internal("evaluation store unavailable", nil)
		}
		if err := eval.UpdateParticipantStatus(ctx, response.RfxEventID, response.ParticipantCompanyID, actor.TenantID, domain.ParticipantStatusResponseSubmitted); err != nil {
			return err
		}
		event, _ := s.repo.GetEventByID(ctx, response.RfxEventID, actor.TenantID)
		return recordAudit(ctx, audit, actor, event.OwnerCompanyID, "rfx_event", response.RfxEventID, "unshortlist_response", map[string]any{
			"response_id": response.ID.String(),
		})
	})
}

func (s *RfxService) AwardResponse(ctx context.Context, actor domain.ActorContext, eventID, responseID uuid.UUID) (*domain.RfxAward, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	event, ownerCompanyID, err := s.requireBuyerEventAccess(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	response, err := s.repo.GetResponseByID(ctx, responseID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if response.RfxEventID != eventID {
		return nil, apperrors.NotFound("rfx response not found")
	}
	if !domain.IsResponseEligibleForEvaluation(response.Status) {
		return nil, apperrors.Validation("response is not eligible for award", map[string]any{"field": "status"})
	}
	store := s.evalStore()
	if store == nil {
		return nil, apperrors.Internal("evaluation store unavailable", nil)
	}
	lines, err := store.ListOfferLinesByResponse(ctx, responseID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	lotCount, err := s.repo.CountLotsByEvent(ctx, eventID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if !domain.ResponseOfferComplete(lotCount, lines) {
		return nil, apperrors.Validation("response commercial offer is incomplete", map[string]any{"field": "offer_lines"})
	}
	eventCurrency := ""
	if event.CurrencyCode != nil {
		eventCurrency = *event.CurrencyCode
	}
	total, currency, currencyOK := domain.ResponseCommercialSummary(lines, eventCurrency)
	if !currencyOK || total <= 0 {
		return nil, apperrors.Validation("response is not commercially comparable", map[string]any{"field": "currency_code"})
	}
	totalCopy := total
	currencyCopy := currency
	input := domain.CreateRfxAwardInput{
		TenantID:         actor.TenantID,
		RfxEventID:       eventID,
		RfxResponseID:    responseID,
		CarrierCompanyID: response.ParticipantCompanyID,
		TotalAmount:      &totalCopy,
		CurrencyCode:     &currencyCopy,
		AwardedBy:        actor.UserID,
	}
	preCommit := func(ctx context.Context, tx pgx.Tx) error {
		recorder := s.audit
		if auditRepo, ok := s.audit.(*repository.AuditRepository); ok {
			recorder = auditRepo.WithTx(tx)
		}
		return recordAudit(ctx, recorder, actor, ownerCompanyID, "rfx_event", eventID, "award_response", map[string]any{
			"response_id":        responseID.String(),
			"carrier_company_id": response.ParticipantCompanyID.String(),
			"total_amount":       totalCopy,
			"currency_code":      currencyCopy,
		})
	}
	award, err := store.CreateAwardTransactional(ctx, input, domain.RfxStatusAwarded, preCommit)
	if err != nil {
		return nil, err
	}
	return award, nil
}

func (s *RfxService) ListEventAuditEvents(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID, limit int) ([]repository.AuditEvent, error) {
	if _, _, err := s.requireBuyerEventAccess(ctx, actor, eventID); err != nil {
		return nil, err
	}
	reader := s.auditReader()
	if reader == nil {
		return nil, apperrors.Internal("audit reader unavailable", nil)
	}
	return reader.ListByEntity(ctx, actor.TenantID, "rfx_event", eventID, limit)
}

func (s *RfxService) GetOwnAward(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID, requestedCarrierCompanyID uuid.UUID) (*domain.RfxAward, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	carrierCompanyID, err := s.requireCarrierEventAccess(ctx, actor, eventID, requestedCarrierCompanyID)
	if err != nil {
		return nil, err
	}
	store := s.evalStore()
	if store == nil {
		return nil, apperrors.Internal("evaluation store unavailable", nil)
	}
	award, err := store.GetAwardForCarrier(ctx, eventID, carrierCompanyID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if award.CarrierCompanyID != carrierCompanyID {
		return nil, apperrors.NotFound("rfx award not found")
	}
	return award, nil
}

func (s *RfxService) GetResponseDetail(ctx context.Context, actor domain.ActorContext, responseID uuid.UUID) (*domain.EvaluationResponseView, error) {
	response, err := s.GetResponse(ctx, actor, responseID)
	if err != nil {
		return nil, err
	}
	kind, _, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	if kind != domain.ActorKindBuyer {
		store := s.evalStore()
		if store == nil {
			return nil, apperrors.Internal("evaluation store unavailable", nil)
		}
		lines, err := store.ListOfferLinesByResponse(ctx, responseID, actor.TenantID)
		if err != nil {
			return nil, err
		}
		response.OfferLines = lines
		return &domain.EvaluationResponseView{Response: *response}, nil
	}
	views, err := s.ListEvaluationResponses(ctx, actor, response.RfxEventID)
	if err != nil {
		return nil, err
	}
	for _, view := range views {
		if view.Response.ID == responseID {
			return &view, nil
		}
	}
	return nil, apperrors.NotFound("rfx response not found")
}

func (s *RfxService) requireBuyerEventAccess(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID) (*domain.RfxEvent, uuid.UUID, error) {
	event, err := s.repo.GetEventByID(ctx, eventID, actor.TenantID)
	if err != nil {
		return nil, uuid.Nil, err
	}
	ownerCompanyID, err := s.requireOwnerCompanyAccess(ctx, actor, event.OwnerCompanyID)
	if err != nil {
		return nil, uuid.Nil, err
	}
	return event, ownerCompanyID, nil
}

func (s *RfxService) getEligibleBuyerResponse(ctx context.Context, actor domain.ActorContext, responseID uuid.UUID) (*domain.RfxResponse, error) {
	response, err := s.repo.GetResponseByID(ctx, responseID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if _, _, err := s.requireBuyerEventAccess(ctx, actor, response.RfxEventID); err != nil {
		return nil, err
	}
	if !domain.IsResponseEligibleForEvaluation(response.Status) {
		return nil, apperrors.Validation("response is not eligible for evaluation action", map[string]any{"field": "status"})
	}
	return response, nil
}
