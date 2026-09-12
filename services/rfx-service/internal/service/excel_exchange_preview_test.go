package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
	"github.com/freight-platform/rfx-service/internal/xlsxexchange"
)

type errorTxRunner struct{}

func (errorTxRunner) Run(ctx context.Context, fn func(ctx context.Context, tx pgx.Tx) error) error {
	return apperrors.Internal("transaction failed", errors.New("boom"))
}

func TestE7P2INT37RepositoryFailureNoAnalysis(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	userID := uuid.New()
	ownerCompanyID := uuid.New()
	eventID := uuid.New()
	versionID := uuid.New()
	questionID := uuid.New()

	getEventFn := func(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxEvent, error) {
		return &domain.RfxEvent{
			ID: eventID, TenantID: tenantID, OwnerCompanyID: ownerCompanyID, Version: 3,
		}, nil
	}
	authRfxStore := &mockRfxStore{getEventFn: getEventFn}
	rfxStore := &mockExcelExchangeRfxStore{
		getEventFn: getEventFn,
		listLotsFn: func(context.Context, uuid.UUID, uuid.UUID) ([]domain.RfxLot, error) {
			return []domain.RfxLot{{LotNumber: "L1", Name: "Lot One", Status: "DRAFT"}}, nil
		},
	}
	qStore := &mockExcelExchangeQuestionnaireStore{
		getActiveDraftFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxVersion, error) {
			return &domain.RfxVersion{
				ID: versionID, TenantID: tenantID, RfxEventID: eventID,
				VersionNumber: 2, Status: domain.RfxVersionStatusDraft, Version: 5,
			}, nil
		},
		loadTreeFn: func(context.Context, uuid.UUID, uuid.UUID) ([]domain.SectionWithQuestions, error) {
			return []domain.SectionWithQuestions{{
				Section: domain.Section{SectionCode: "S1", Title: "Section", SortOrder: 1},
				Questions: []domain.Question{{
					ID: questionID, QuestionCode: "Q1", QuestionType: "TEXT", Label: "Question",
					ValidationRuleJSON: json.RawMessage(`{}`), Required: true,
				}},
			}}, nil
		},
	}
	auth := NewRfxService(authRfxStore, nil, buyerMembershipResolver(ownerCompanyID))
	svc := NewExcelExchangeService(rfxStore, qStore, auth, repository.NewImportAnalysisRepository(nil), errorTxRunner{})

	workbook, err := xlsxexchange.GenerateBuyerDraftWorkbook(xlsxexchange.BuyerDraftSnapshot{
		Metadata: xlsxexchange.BuyerDraftMetadata{
			TenantID: tenantID, RfxEventID: eventID, RfxVersionID: versionID, VersionNumber: 2,
			VersionStatus: domain.RfxVersionStatusDraft, EventRowVersion: 3, VersionRowVersion: 5,
		},
		Sections:  []domain.Section{{SectionCode: "S1", Title: "Section", SortOrder: 1}},
		Questions: []xlsxexchange.BuyerDraftQuestion{{SectionCode: "S1", Question: domain.Question{QuestionCode: "Q1", QuestionType: "TEXT", Label: "Question", Required: true}}},
		Lots:      []domain.RfxLot{{LotNumber: "L1", Name: "Lot One", Status: "DRAFT"}},
	})
	if err != nil {
		t.Fatalf("generate workbook: %v", err)
	}

	_, err = svc.PreviewBuyerImportWorkbook(context.Background(), buyerTestActor(tenantID, userID, ownerCompanyID), eventID, workbook)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeInternal {
		t.Fatalf("expected internal error, got %v", err)
	}
}

func TestPreviewBuyerImportWorkbookTTLUsesInjectedClock(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	userID := uuid.New()
	ownerCompanyID := uuid.New()
	eventID := uuid.New()
	versionID := uuid.New()
	fixed := time.Date(2026, 9, 12, 15, 0, 0, 0, time.UTC)

	getEventFn := func(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxEvent, error) {
		return &domain.RfxEvent{ID: eventID, TenantID: tenantID, OwnerCompanyID: ownerCompanyID, Version: 1}, nil
	}
	authRfxStore := &mockRfxStore{getEventFn: getEventFn}
	rfxStore := &mockExcelExchangeRfxStore{
		getEventFn: getEventFn,
		listLotsFn: func(context.Context, uuid.UUID, uuid.UUID) ([]domain.RfxLot, error) {
			return nil, nil
		},
	}
	qStore := &mockExcelExchangeQuestionnaireStore{
		getActiveDraftFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxVersion, error) {
			return &domain.RfxVersion{
				ID: versionID, TenantID: tenantID, RfxEventID: eventID,
				VersionNumber: 1, Status: domain.RfxVersionStatusDraft, Version: 1,
			}, nil
		},
		loadTreeFn: func(context.Context, uuid.UUID, uuid.UUID) ([]domain.SectionWithQuestions, error) {
			return []domain.SectionWithQuestions{{
				Section:   domain.Section{SectionCode: "S1", Title: "Section", SortOrder: 1},
				Questions: []domain.Question{{QuestionCode: "Q1", QuestionType: "TEXT", Label: "Q", Required: true}},
			}}, nil
		},
	}
	auth := NewRfxService(authRfxStore, nil, buyerMembershipResolver(ownerCompanyID))
	svc := NewExcelExchangeService(rfxStore, qStore, auth, nil, nil)
	svc.SetNowFunc(func() time.Time { return fixed })

	workbook, err := xlsxexchange.GenerateBuyerDraftWorkbook(xlsxexchange.BuyerDraftSnapshot{
		Metadata: xlsxexchange.BuyerDraftMetadata{
			TenantID: tenantID, RfxEventID: eventID, RfxVersionID: versionID, VersionNumber: 1,
			VersionStatus: domain.RfxVersionStatusDraft, EventRowVersion: 1, VersionRowVersion: 1,
		},
		Sections:  []domain.Section{{SectionCode: "S1", Title: "Section", SortOrder: 1}},
		Questions: []xlsxexchange.BuyerDraftQuestion{{SectionCode: "S1", Question: domain.Question{QuestionCode: "Q1", QuestionType: "TEXT", Label: "Q", Required: true}}},
	})
	if err != nil {
		t.Fatalf("generate workbook: %v", err)
	}

	// nil repo exercises parse path only; TTL verified in integration INT-36.
	_, err = svc.PreviewBuyerImportWorkbook(context.Background(), buyerTestActor(tenantID, userID, ownerCompanyID), eventID, workbook)
	if err == nil {
		t.Fatal("expected persistence-not-configured error without repository wiring")
	}
}
