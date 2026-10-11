package service

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
)

type carrierDocFake struct {
	visible        bool
	listed         int
	seen           domain.CarrierDocumentFilter
	allowedCarrier uuid.UUID
	docs           map[uuid.UUID]*domain.Document
}

func (f *carrierDocFake) CarrierShipmentVisible(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (bool, error) {
	return f.visible, nil
}

func (f *carrierDocFake) ListCarrierDocuments(_ context.Context, filter domain.CarrierDocumentFilter) ([]domain.Document, int, error) {
	f.listed++
	f.seen = filter
	return nil, 0, nil
}

func (f *carrierDocFake) GetCarrierDocument(_ context.Context, _, carrierID, documentID uuid.UUID) (*domain.Document, error) {
	doc, ok := f.docs[documentID]
	if !ok || doc == nil || (f.allowedCarrier != uuid.Nil && carrierID != f.allowedCarrier) {
		return nil, apperrors.NotFound("document not found")
	}
	return doc, nil
}

type carrierFileFake struct {
	lists int
	gets  int
	opens int
	item  *domain.Attachment
}

func (f *carrierFileFake) ListFinalized(context.Context, uuid.UUID, uuid.UUID) ([]domain.Attachment, error) {
	f.lists++
	if f.item == nil || f.item.Status != domain.AttachmentStatusFinalized {
		return nil, nil
	}
	return []domain.Attachment{*f.item}, nil
}

func (f *carrierFileFake) GetFinalized(_ context.Context, _, _, attachmentID uuid.UUID) (*domain.Attachment, error) {
	f.gets++
	if f.item == nil || f.item.ID != attachmentID || f.item.Status != domain.AttachmentStatusFinalized {
		return nil, apperrors.NotFound("attachment not found")
	}
	return f.item, nil
}

func (f *carrierFileFake) OpenFinalized(ctx context.Context, tenantID, documentID, attachmentID uuid.UUID) (*domain.Attachment, io.ReadCloser, error) {
	item, err := f.GetFinalized(ctx, tenantID, documentID, attachmentID)
	if err != nil {
		return nil, nil, err
	}
	f.opens++
	return item, io.NopCloser(strings.NewReader("pdf")), nil
}

func TestCarrierListHidesForeignShipmentBeforeQuery(t *testing.T) {
	docs := &carrierDocFake{visible: false}
	svc := NewCarrierDocumentService(docs, &carrierFileFake{})
	_, _, err := svc.List(context.Background(), domain.CarrierDocumentFilter{
		TenantID: uuid.New(), CarrierCompanyID: uuid.New(), ShipmentID: uuid.New(), Limit: 20,
	})
	if err == nil || docs.listed != 0 {
		t.Fatalf("err=%v listed=%d", err, docs.listed)
	}
}

func TestCarrierListDefaultsLimitBeforeQuery(t *testing.T) {
	docs := &carrierDocFake{visible: true}
	svc := NewCarrierDocumentService(docs, &carrierFileFake{})
	_, _, err := svc.List(context.Background(), domain.CarrierDocumentFilter{
		TenantID: uuid.New(), CarrierCompanyID: uuid.New(), ShipmentID: uuid.New(),
	})
	if err != nil || docs.listed != 1 || docs.seen.Limit != 20 {
		t.Fatalf("err=%v listed=%d limit=%d", err, docs.listed, docs.seen.Limit)
	}
}

func TestCarrierListRejectsUnsafeFiltersBeforeQuery(t *testing.T) {
	docs := &carrierDocFake{visible: true}
	svc := NewCarrierDocumentService(docs, &carrierFileFake{})
	invoice := "INVOICE"
	base := domain.CarrierDocumentFilter{TenantID: uuid.New(), CarrierCompanyID: uuid.New(), ShipmentID: uuid.New(), Limit: 20}
	over := base
	over.Limit = 51
	neg := base
	neg.Offset = -1
	typed := base
	typed.DocumentType = &invoice
	for _, filter := range []domain.CarrierDocumentFilter{over, neg, typed} {
		if _, _, err := svc.List(context.Background(), filter); err == nil || docs.listed != 0 {
			t.Fatalf("err=%v listed=%d", err, docs.listed)
		}
	}
}

func TestCarrierGetDeniesForeignCarrier(t *testing.T) {
	tenant := uuid.New()
	carrier := uuid.New()
	docID := uuid.New()
	docs := &carrierDocFake{
		allowedCarrier: carrier,
		docs: map[uuid.UUID]*domain.Document{
			docID: {ID: docID, TenantID: tenant},
		},
	}
	svc := NewCarrierDocumentService(docs, &carrierFileFake{})
	if _, err := svc.Get(context.Background(), tenant, uuid.New(), docID); err == nil {
		t.Fatal("foreign carrier read was allowed")
	}
	if _, err := svc.Get(context.Background(), tenant, carrier, docID); err != nil {
		t.Fatal(err)
	}
}

func TestCarrierAttachmentsRequireDocumentProofBeforeStorage(t *testing.T) {
	tenant := uuid.New()
	carrier := uuid.New()
	docID := uuid.New()
	other := uuid.New()
	att := uuid.New()
	docs := &carrierDocFake{docs: map[uuid.UUID]*domain.Document{
		docID: {ID: docID, TenantID: tenant, DocumentType: "POD", DocumentStatus: domain.DocumentStatusSigned},
	}}
	files := &carrierFileFake{item: &domain.Attachment{ID: att, Status: domain.AttachmentStatusDraft}}
	svc := NewCarrierDocumentService(docs, files)

	if _, err := svc.Get(context.Background(), tenant, carrier, other); err == nil {
		t.Fatal("foreign document was found")
	}
	if _, err := svc.ListAttachments(context.Background(), tenant, carrier, other); err == nil || files.lists != 0 {
		t.Fatalf("foreign list attachments lists=%d", files.lists)
	}
	if _, err := svc.GetAttachment(context.Background(), tenant, carrier, other, att); err == nil || files.gets != 0 {
		t.Fatalf("foreign get attachment gets=%d", files.gets)
	}
	if _, _, err := svc.OpenContent(context.Background(), tenant, carrier, other, att); err == nil || files.opens != 0 || files.gets != 0 {
		t.Fatalf("foreign open opens=%d gets=%d", files.opens, files.gets)
	}
	if _, _, err := svc.OpenContent(context.Background(), tenant, carrier, docID, att); err == nil || files.opens != 0 {
		t.Fatalf("draft open opens=%d", files.opens)
	}
	if _, _, err := svc.OpenContent(context.Background(), tenant, carrier, docID, uuid.New()); err == nil || files.opens != 0 {
		t.Fatalf("cross attachment open opens=%d", files.opens)
	}
}
