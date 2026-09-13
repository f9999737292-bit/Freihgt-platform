package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
)

type mockExcelExchangeRfxStore struct {
	getEventFn            func(ctx context.Context, id, tenantID uuid.UUID) (*domain.RfxEvent, error)
	listLotsFn            func(ctx context.Context, eventID, tenantID uuid.UUID) ([]domain.RfxLot, error)
	getExchangeMetadataFn func(ctx context.Context, eventID, tenantID uuid.UUID) (*repository.EventExchangeMetadata, error)
}

func (m *mockExcelExchangeRfxStore) GetEventByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.RfxEvent, error) {
	if m.getEventFn != nil {
		return m.getEventFn(ctx, id, tenantID)
	}
	return nil, apperrors.NotFound("rfx event not found")
}

func (m *mockExcelExchangeRfxStore) ListLotsByEvent(ctx context.Context, eventID, tenantID uuid.UUID) ([]domain.RfxLot, error) {
	if m.listLotsFn != nil {
		return m.listLotsFn(ctx, eventID, tenantID)
	}
	return nil, nil
}

func (m *mockExcelExchangeRfxStore) GetEventExchangeMetadata(ctx context.Context, eventID, tenantID uuid.UUID) (*repository.EventExchangeMetadata, error) {
	if m.getExchangeMetadataFn != nil {
		return m.getExchangeMetadataFn(ctx, eventID, tenantID)
	}
	return &repository.EventExchangeMetadata{CreationChannel: domain.CreationChannelManual}, nil
}

type mockExcelExchangeQuestionnaireStore struct {
	getActiveDraftFn func(ctx context.Context, tenantID, eventID uuid.UUID) (*domain.RfxVersion, error)
	loadTreeFn       func(ctx context.Context, versionID, tenantID uuid.UUID) ([]domain.SectionWithQuestions, error)
	listRulesFn      func(ctx context.Context, versionID, tenantID uuid.UUID) ([]domain.QuestionRule, error)
}

func (m *mockExcelExchangeQuestionnaireStore) GetActiveDraftVersion(ctx context.Context, tenantID, eventID uuid.UUID) (*domain.RfxVersion, error) {
	if m.getActiveDraftFn != nil {
		return m.getActiveDraftFn(ctx, tenantID, eventID)
	}
	return nil, apperrors.Conflict("draft questionnaire version not found", map[string]any{"field": "draft_version"})
}

func (m *mockExcelExchangeQuestionnaireStore) LoadQuestionnaireTree(ctx context.Context, versionID, tenantID uuid.UUID) ([]domain.SectionWithQuestions, error) {
	if m.loadTreeFn != nil {
		return m.loadTreeFn(ctx, versionID, tenantID)
	}
	return nil, nil
}

func (m *mockExcelExchangeQuestionnaireStore) ListRulesByVersion(ctx context.Context, versionID, tenantID uuid.UUID) ([]domain.QuestionRule, error) {
	if m.listRulesFn != nil {
		return m.listRulesFn(ctx, versionID, tenantID)
	}
	return nil, nil
}

func TestExcelExchangeServiceExportBuyerDraftWorkbookSuccess(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	userID := uuid.New()
	ownerCompanyID := uuid.New()
	eventID := uuid.New()
	versionID := uuid.New()
	questionID := uuid.New()
	fixedNow := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)

	getEventFn := func(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxEvent, error) {
		return &domain.RfxEvent{
			ID:             eventID,
			TenantID:       tenantID,
			OwnerCompanyID: ownerCompanyID,
			Version:        3,
		}, nil
	}
	authRfxStore := &mockRfxStore{getEventFn: getEventFn}
	rfxStore := &mockExcelExchangeRfxStore{
		getEventFn: getEventFn,
		listLotsFn: func(context.Context, uuid.UUID, uuid.UUID) ([]domain.RfxLot, error) {
			return []domain.RfxLot{{LotNumber: "L1", Name: "Lot One", Status: "DRAFT"}}, nil
		},
		getExchangeMetadataFn: func(context.Context, uuid.UUID, uuid.UUID) (*repository.EventExchangeMetadata, error) {
			return &repository.EventExchangeMetadata{CreationChannel: domain.CreationChannelManual}, nil
		},
	}
	qStore := &mockExcelExchangeQuestionnaireStore{
		getActiveDraftFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxVersion, error) {
			return &domain.RfxVersion{
				ID:            versionID,
				TenantID:      tenantID,
				RfxEventID:    eventID,
				VersionNumber: 2,
				Status:        domain.RfxVersionStatusDraft,
				Version:       5,
			}, nil
		},
		loadTreeFn: func(context.Context, uuid.UUID, uuid.UUID) ([]domain.SectionWithQuestions, error) {
			return []domain.SectionWithQuestions{{
				Section: domain.Section{SectionCode: "S1", Title: "Section", SortOrder: 1},
				Questions: []domain.Question{{
					ID:                 questionID,
					QuestionCode:       "Q1",
					QuestionType:       "TEXT",
					Label:              "Question",
					ValidationRuleJSON: json.RawMessage(`{}`),
				}},
			}}, nil
		},
	}

	auth := NewRfxService(authRfxStore, nil, buyerMembershipResolver(ownerCompanyID))
	svc := NewExcelExchangeService(rfxStore, qStore, auth, nil, nil)
	svc.SetNowFunc(func() time.Time { return fixedNow })

	data, filename, err := svc.ExportBuyerDraftWorkbook(context.Background(), buyerTestActor(tenantID, userID, ownerCompanyID), eventID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty workbook")
	}
	wantFilename := "bintrans-rfx-" + eventID.String() + "-draft-v2.xlsx"
	if filename != wantFilename {
		t.Fatalf("filename: got %q want %q", filename, wantFilename)
	}
}

func TestExcelExchangeServiceExportBuyerDraftWorkbookMissingDraft(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	userID := uuid.New()
	ownerCompanyID := uuid.New()
	eventID := uuid.New()

	getEventFn := func(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxEvent, error) {
		return &domain.RfxEvent{ID: eventID, TenantID: tenantID, OwnerCompanyID: ownerCompanyID}, nil
	}
	authRfxStore := &mockRfxStore{getEventFn: getEventFn}
	rfxStore := &mockExcelExchangeRfxStore{getEventFn: getEventFn}
	qStore := &mockExcelExchangeQuestionnaireStore{
		getActiveDraftFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxVersion, error) {
			return nil, apperrors.Conflict("draft questionnaire version not found", map[string]any{"field": "draft_version"})
		},
	}
	auth := NewRfxService(authRfxStore, nil, buyerMembershipResolver(ownerCompanyID))
	svc := NewExcelExchangeService(rfxStore, qStore, auth, nil, nil)

	_, _, err := svc.ExportBuyerDraftWorkbook(context.Background(), buyerTestActor(tenantID, userID, ownerCompanyID), eventID)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeConflict {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestExcelExchangeServiceExportBuyerDraftWorkbookRequiresBuyerManage(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	userID := uuid.New()
	ownerCompanyID := uuid.New()
	eventID := uuid.New()

	getEventFn := func(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxEvent, error) {
		return &domain.RfxEvent{ID: eventID, TenantID: tenantID, OwnerCompanyID: ownerCompanyID}, nil
	}
	authRfxStore := &mockRfxStore{getEventFn: getEventFn}
	rfxStore := &mockExcelExchangeRfxStore{getEventFn: getEventFn}
	qStore := &mockExcelExchangeQuestionnaireStore{}
	resolver := buyerMembershipResolver(ownerCompanyID)
	resolver.roles = []string{"SHIPPER_LOGIST"}
	auth := NewRfxService(authRfxStore, nil, resolver)
	svc := NewExcelExchangeService(rfxStore, qStore, auth, nil, nil)

	_, _, err := svc.ExportBuyerDraftWorkbook(context.Background(), buyerTestActor(tenantID, userID, ownerCompanyID), eventID)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
}
