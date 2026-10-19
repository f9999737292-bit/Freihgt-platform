package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

func TestRelationshipCreateReusesSourceAuthorityAndOmitsTokenOnReplay(t *testing.T) {
	row := relationshipLiveRow()
	reader := &scriptReader{row: row, actor: authorizedActor(), carrier: true}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Service-Name") != "" {
			t.Fatal("service name header")
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"relationship_request_id":"` + uuid.NewString() + `","request_status":"PENDING","acceptance_token":"once"}`))
	}))
	defer server.Close()
	initiator := NewRelationshipCreateInitiator(reader, NewCompanyRelationshipCreateClient(server.URL, "dedicated", nil))
	created, err := initiator.Create(context.Background(), RelationshipActor{UserID: uuid.New(), TenantID: row.TenantID}, row.ID, "same-key")
	if err != nil || !created.Created || created.AcceptanceToken != "once" {
		t.Fatalf("create: %v %+v", err, created)
	}
	replayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"relationship_request_id":"` + created.RequestID.String() + `","request_status":"PENDING","acceptance_token":"again"}`))
	}))
	defer replayServer.Close()
	replayInitiator := NewRelationshipCreateInitiator(reader, NewCompanyRelationshipCreateClient(replayServer.URL, "dedicated", nil))
	replay, err := replayInitiator.Create(context.Background(), RelationshipActor{UserID: uuid.New(), TenantID: row.TenantID}, row.ID, "same-key")
	if err != nil || replay.Created || replay.AcceptanceToken != "" || replay.RequestID != created.RequestID {
		t.Fatalf("replay: %v %+v", err, replay)
	}
}

func TestRelationshipCreateRefusesBeforeCompanyCall(t *testing.T) {
	row := relationshipLiveRow()
	cases := []struct {
		name   string
		reader *scriptReader
		actor  RelationshipActor
		code   apperrors.Code
	}{
		{name: "missing", reader: &scriptReader{missing: true}, actor: RelationshipActor{UserID: uuid.New(), TenantID: row.TenantID}, code: apperrors.CodeNotFound},
		{name: "foreign tenant", reader: &scriptReader{row: row, actor: authorizedActor(), carrier: true}, actor: RelationshipActor{UserID: uuid.New(), TenantID: uuid.New()}, code: apperrors.CodeNotFound},
		{name: "deleted", reader: &scriptReader{row: deletedRow(row), actor: authorizedActor(), carrier: true}, actor: RelationshipActor{UserID: uuid.New(), TenantID: row.TenantID}, code: apperrors.CodeNotFound},
		{name: "no carrier", reader: &scriptReader{row: unassignedRow(row), actor: authorizedActor(), carrier: true}, actor: RelationshipActor{UserID: uuid.New(), TenantID: row.TenantID}, code: apperrors.CodeNotFound},
		{name: "inactive actor", reader: &scriptReader{row: row, actor: domain.ShipmentCreatorAuthority{}, carrier: true}, actor: RelationshipActor{UserID: uuid.New(), TenantID: row.TenantID}, code: apperrors.CodeRelationshipRequestForbidden},
		{name: "stale", reader: &scriptReader{row: row, next: versioned(row, row.Version+1), actor: authorizedActor(), carrier: true}, actor: RelationshipActor{UserID: uuid.New(), TenantID: row.TenantID}, code: apperrors.CodeRelationshipRequestStale},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("company-service was called")
			}))
			defer server.Close()
			initiator := NewRelationshipCreateInitiator(tc.reader, NewCompanyRelationshipCreateClient(server.URL, "dedicated", nil))
			_, err := initiator.Create(context.Background(), tc.actor, row.ID, "key")
			assertCode(t, err, tc.code)
		})
	}
}

func TestRelationshipCreateEmptyClientFailsClosed(t *testing.T) {
	row := relationshipLiveRow()
	initiator := NewRelationshipCreateInitiator(&scriptReader{row: row, actor: authorizedActor(), carrier: true}, NewCompanyRelationshipCreateClient("", "", nil))
	_, err := initiator.Create(context.Background(), RelationshipActor{UserID: uuid.New(), TenantID: row.TenantID}, row.ID, "key")
	assertCode(t, err, apperrors.CodeRelationshipSourceUnavailable)
}

type scriptReader struct {
	row     domain.ShipmentSourceRecord
	next    *domain.ShipmentSourceRecord
	missing bool
	actor   domain.ShipmentCreatorAuthority
	carrier bool
	reads   int
}

func (s *scriptReader) ReadShipmentSource(context.Context, uuid.UUID) (domain.ShipmentSourceRecord, error) {
	if s.missing {
		return domain.ShipmentSourceRecord{}, domain.ErrShipmentSourceMissing
	}
	s.reads++
	if s.reads > 1 && s.next != nil {
		return *s.next, nil
	}
	return s.row, nil
}

func (s *scriptReader) ReadShipmentCreator(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.ShipmentCreatorAuthority, error) {
	return s.actor, nil
}

func (s *scriptReader) ActiveCarrierInTenant(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return s.carrier, nil
}

func relationshipLiveRow() domain.ShipmentSourceRecord {
	return domain.ShipmentSourceRecord{
		TenantID: uuid.New(), ID: uuid.New(), Version: 4,
		ShipperCompanyID: uuid.New(), CarrierCompanyID: uuid.New(),
		CarrierAssigned: true, Live: true,
	}
}

func deletedRow(row domain.ShipmentSourceRecord) domain.ShipmentSourceRecord {
	row.Live = false
	return row
}

func unassignedRow(row domain.ShipmentSourceRecord) domain.ShipmentSourceRecord {
	row.CarrierAssigned = false
	row.CarrierCompanyID = uuid.Nil
	return row
}

func versioned(row domain.ShipmentSourceRecord, version int) *domain.ShipmentSourceRecord {
	row.Version = version
	return &row
}

func authorizedActor() domain.ShipmentCreatorAuthority {
	return domain.ShipmentCreatorAuthority{Active: true, ActiveMembership: true, CompanyRoles: []string{"SHIPPER_ADMIN"}}
}

func assertCode(t *testing.T, err error, code apperrors.Code) {
	t.Helper()
	var appErr *apperrors.AppError
	if err == nil || !errors.As(err, &appErr) || appErr.Code != code {
		t.Fatalf("got %v want %s", err, code)
	}
}
