package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
)

func TestGetEventCreationChannelReturnsStoredValues(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	ownerCompanyID := uuid.New()
	eventID := uuid.New()
	for _, channel := range []string{
		domain.CreationChannelManual,
		domain.CreationChannelTemplate,
		domain.CreationChannelExcel,
		domain.CreationChannelERP,
	} {
		channel := channel
		t.Run(channel, func(t *testing.T) {
			t.Parallel()
			svc := NewRfxService(&mockRfxStore{
				getEventFn: func(_ context.Context, id, gotTenant uuid.UUID) (*domain.RfxEvent, error) {
					if id != eventID || gotTenant != tenantID {
						t.Fatalf("unexpected lookup id=%s tenant=%s", id, gotTenant)
					}
					return &domain.RfxEvent{ID: eventID, TenantID: tenantID, OwnerCompanyID: ownerCompanyID}, nil
				},
				getExchangeMetaFn: func(_ context.Context, id, gotTenant uuid.UUID) (*repository.EventExchangeMetadata, error) {
					if id != eventID || gotTenant != tenantID {
						t.Fatalf("unexpected metadata lookup id=%s tenant=%s", id, gotTenant)
					}
					return &repository.EventExchangeMetadata{CreationChannel: channel}, nil
				},
			}, nil, buyerMembershipResolver(ownerCompanyID))
			got, err := svc.GetEventCreationChannel(context.Background(), buyerTestActor(tenantID, uuid.New(), ownerCompanyID), eventID)
			if err != nil {
				t.Fatalf("GetEventCreationChannel: %v", err)
			}
			if got != channel {
				t.Fatalf("creation_channel=%q want %q", got, channel)
			}
		})
	}
}

func TestGetEventCreationChannelDoesNotInventHistoricalEmpty(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	ownerCompanyID := uuid.New()
	eventID := uuid.New()
	svc := NewRfxService(&mockRfxStore{
		getEventFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxEvent, error) {
			return &domain.RfxEvent{ID: eventID, TenantID: tenantID, OwnerCompanyID: ownerCompanyID}, nil
		},
		getExchangeMetaFn: func(context.Context, uuid.UUID, uuid.UUID) (*repository.EventExchangeMetadata, error) {
			return &repository.EventExchangeMetadata{CreationChannel: "  "}, nil
		},
	}, nil, buyerMembershipResolver(ownerCompanyID))
	got, err := svc.GetEventCreationChannel(context.Background(), buyerTestActor(tenantID, uuid.New(), ownerCompanyID), eventID)
	if err != nil {
		t.Fatalf("GetEventCreationChannel: %v", err)
	}
	if got != "" {
		t.Fatalf("empty stored channel must not be invented, got %q", got)
	}
}

func TestGetEventCreationChannelForeignTenantNotFound(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	foreignTenant := uuid.New()
	ownerCompanyID := uuid.New()
	eventID := uuid.New()
	svc := NewRfxService(&mockRfxStore{
		getEventFn: func(_ context.Context, _ uuid.UUID, gotTenant uuid.UUID) (*domain.RfxEvent, error) {
			if gotTenant != tenantID {
				return nil, apperrors.NotFound("rfx event not found")
			}
			return &domain.RfxEvent{ID: eventID, TenantID: tenantID, OwnerCompanyID: ownerCompanyID}, nil
		},
		getExchangeMetaFn: func(context.Context, uuid.UUID, uuid.UUID) (*repository.EventExchangeMetadata, error) {
			t.Fatal("metadata must not be read for a foreign tenant")
			return nil, nil
		},
	}, nil, buyerMembershipResolver(ownerCompanyID))
	_, err := svc.GetEventCreationChannel(context.Background(), buyerTestActor(foreignTenant, uuid.New(), ownerCompanyID), eventID)
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestGetEventCreationChannelForeignCompanyNotFound(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	ownerA := uuid.New()
	ownerB := uuid.New()
	eventID := uuid.New()
	svc := NewRfxService(&mockRfxStore{
		getEventFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxEvent, error) {
			return &domain.RfxEvent{ID: eventID, TenantID: tenantID, OwnerCompanyID: ownerA}, nil
		},
		getExchangeMetaFn: func(context.Context, uuid.UUID, uuid.UUID) (*repository.EventExchangeMetadata, error) {
			t.Fatal("metadata must not be read for a foreign company")
			return nil, nil
		},
	}, nil, buyerMembershipResolver(ownerB))
	_, err := svc.GetEventCreationChannel(context.Background(), buyerTestActor(tenantID, uuid.New(), ownerB), eventID)
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestGetEventCreationChannelCarrierDeniedWithoutParticipant(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	eventID := uuid.New()
	svc := NewRfxService(&mockRfxStore{
		getEventFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxEvent, error) {
			return &domain.RfxEvent{ID: eventID, TenantID: tenantID, OwnerCompanyID: uuid.New()}, nil
		},
		participantExistsFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (bool, error) {
			return false, nil
		},
		getExchangeMetaFn: func(context.Context, uuid.UUID, uuid.UUID) (*repository.EventExchangeMetadata, error) {
			t.Fatal("metadata must not be read for a non-participant carrier")
			return nil, nil
		},
	}, nil, carrierMembershipResolver(uuid.New()))
	_, err := svc.GetEventCreationChannel(context.Background(), carrierTestActor(tenantID, uuid.New()), eventID)
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
}
