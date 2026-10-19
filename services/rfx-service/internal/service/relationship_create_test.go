package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func TestRelationshipCreateReusesSourceAuthorityAndOmitsTokenOnReplay(t *testing.T) {
	award := awardedServiceRow()
	link := linkedServiceRow()
	reader := &scriptRfx{award: award, link: link, actor: buyerActor(), carrier: true}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Service-Name") != "" {
			t.Fatal("service name header")
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"relationship_request_id":"` + uuid.NewString() + `","request_status":"PENDING","acceptance_token":"once"}`))
	}))
	defer server.Close()
	initiator := NewRfxRelationshipCreateInitiator(reader, NewCompanyRelationshipCreateClient(server.URL, "dedicated", nil))
	actor := RfxRelationshipActor{UserID: uuid.New(), TenantID: award.TenantID}
	created, err := initiator.Create(context.Background(), actor, domain.RfxSourceTypeAward, award.ID, "same-key")
	if err != nil || !created.Created || created.AcceptanceToken != "once" {
		t.Fatalf("award create: %v %+v", err, created)
	}
	linked, err := initiator.Create(context.Background(), actor, domain.RfxSourceTypeAwardLink, link.ID, "link-key")
	if err != nil || !linked.Created || linked.AcceptanceToken != "once" {
		t.Fatalf("link create: %v %+v", err, linked)
	}
	replayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"relationship_request_id":"` + created.RequestID.String() + `","request_status":"PENDING","acceptance_token":"again"}`))
	}))
	defer replayServer.Close()
	replayInitiator := NewRfxRelationshipCreateInitiator(reader, NewCompanyRelationshipCreateClient(replayServer.URL, "dedicated", nil))
	replay, err := replayInitiator.Create(context.Background(), actor, domain.RfxSourceTypeAward, award.ID, "same-key")
	if err != nil || replay.Created || replay.AcceptanceToken != "" || replay.RequestID != created.RequestID {
		t.Fatalf("replay: %v %+v", err, replay)
	}
}

func TestRelationshipCreateRefusesBeforeCompanyCall(t *testing.T) {
	award := awardedServiceRow()
	link := linkedServiceRow()
	actor := RfxRelationshipActor{UserID: uuid.New(), TenantID: award.TenantID}
	stale := award
	stale.Version++
	wrongBuyer := link
	wrongBuyer.BuyerCompanyID = uuid.New()
	cases := []struct {
		name       string
		reader     *scriptRfx
		actor      RfxRelationshipActor
		sourceType string
		sourceID   uuid.UUID
		code       apperrors.Code
	}{
		{name: "missing award", reader: &scriptRfx{awardMissing: true}, actor: actor, sourceType: domain.RfxSourceTypeAward, sourceID: award.ID, code: apperrors.CodeNotFound},
		{name: "missing link", reader: &scriptRfx{linkMissing: true}, actor: actor, sourceType: domain.RfxSourceTypeAwardLink, sourceID: link.ID, code: apperrors.CodeNotFound},
		{name: "foreign tenant", reader: &scriptRfx{award: award, actor: buyerActor(), carrier: true}, actor: RfxRelationshipActor{UserID: actor.UserID, TenantID: uuid.New()}, sourceType: domain.RfxSourceTypeAward, sourceID: award.ID, code: apperrors.CodeNotFound},
		{name: "deleted event", reader: &scriptRfx{award: deletedAward(award), actor: buyerActor(), carrier: true}, actor: actor, sourceType: domain.RfxSourceTypeAward, sourceID: award.ID, code: apperrors.CodeNotFound},
		{name: "not awarded", reader: &scriptRfx{award: unpublishedAward(award), actor: buyerActor(), carrier: true}, actor: actor, sourceType: domain.RfxSourceTypeAward, sourceID: award.ID, code: apperrors.CodeNotFound},
		{name: "wrong buyer", reader: &scriptRfx{link: wrongBuyer, actor: buyerActor(), carrier: true}, actor: actor, sourceType: domain.RfxSourceTypeAwardLink, sourceID: link.ID, code: apperrors.CodeNotFound},
		{name: "wrong carrier", reader: &scriptRfx{award: award, actor: buyerActor(), carrier: false}, actor: actor, sourceType: domain.RfxSourceTypeAward, sourceID: award.ID, code: apperrors.CodeNotFound},
		{name: "inactive actor", reader: &scriptRfx{award: award, actor: domain.RfxCreatorAuthority{}, carrier: true}, actor: actor, sourceType: domain.RfxSourceTypeAward, sourceID: award.ID, code: apperrors.CodeRelationshipRequestForbidden},
		{name: "stale version", reader: &scriptRfx{award: award, nextAward: &stale, actor: buyerActor(), carrier: true}, actor: actor, sourceType: domain.RfxSourceTypeAward, sourceID: award.ID, code: apperrors.CodeRelationshipRequestStale},
		{name: "shipment", reader: &scriptRfx{award: award, actor: buyerActor(), carrier: true}, actor: actor, sourceType: "SHIPMENT", sourceID: award.ID, code: apperrors.CodeRelationshipRequestForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("company-service was called")
			}))
			defer server.Close()
			initiator := NewRfxRelationshipCreateInitiator(tc.reader, NewCompanyRelationshipCreateClient(server.URL, "dedicated", nil))
			_, err := initiator.Create(context.Background(), tc.actor, tc.sourceType, tc.sourceID, "key")
			if codeOf(err) != tc.code {
				t.Fatalf("got %v want %s", err, tc.code)
			}
		})
	}
}

func TestRelationshipCreateEmptyClientFailsClosed(t *testing.T) {
	award := awardedServiceRow()
	initiator := NewRfxRelationshipCreateInitiator(&scriptRfx{award: award, actor: buyerActor(), carrier: true}, NewCompanyRelationshipCreateClient("", "", nil))
	_, err := initiator.Create(context.Background(), RfxRelationshipActor{UserID: uuid.New(), TenantID: award.TenantID}, domain.RfxSourceTypeAward, award.ID, "key")
	if codeOf(err) != apperrors.CodeRelationshipSourceUnavailable {
		t.Fatalf("empty configuration %v", err)
	}
}

type scriptRfx struct {
	award        domain.RfxAwardSourceRecord
	nextAward    *domain.RfxAwardSourceRecord
	awardReads   int
	awardMissing bool
	link         domain.RfxAwardLinkSourceRecord
	nextLink     *domain.RfxAwardLinkSourceRecord
	linkReads    int
	linkMissing  bool
	actor        domain.RfxCreatorAuthority
	carrier      bool
}

func (s *scriptRfx) ReadAwardSource(context.Context, uuid.UUID) (domain.RfxAwardSourceRecord, error) {
	if s.awardMissing {
		return domain.RfxAwardSourceRecord{}, domain.ErrRfxSourceMissing
	}
	s.awardReads++
	if s.awardReads > 1 && s.nextAward != nil {
		return *s.nextAward, nil
	}
	return s.award, nil
}

func (s *scriptRfx) ReadAwardLinkSource(context.Context, uuid.UUID) (domain.RfxAwardLinkSourceRecord, error) {
	if s.linkMissing {
		return domain.RfxAwardLinkSourceRecord{}, domain.ErrRfxSourceMissing
	}
	s.linkReads++
	if s.linkReads > 1 && s.nextLink != nil {
		return *s.nextLink, nil
	}
	return s.link, nil
}

func (s *scriptRfx) ReadRfxCreator(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.RfxCreatorAuthority, error) {
	return s.actor, nil
}

func (s *scriptRfx) ActiveCarrierInTenant(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return s.carrier, nil
}

func deletedAward(row domain.RfxAwardSourceRecord) domain.RfxAwardSourceRecord {
	row.EventLive = false
	return row
}

func unpublishedAward(row domain.RfxAwardSourceRecord) domain.RfxAwardSourceRecord {
	row.EventStatus = "PUBLISHED"
	return row
}
