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

type shipperDocFake struct {
	visible bool
	listed  int
	docs    map[uuid.UUID]*domain.Document
}

func (f *shipperDocFake) ShipperShipmentVisible(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (bool, error) {
	return f.visible, nil
}

func (f *shipperDocFake) ListShipperDocuments(context.Context, domain.ShipperDocumentFilter) ([]domain.Document, int, error) {
	f.listed++
	return nil, 0, nil
}

func (f *shipperDocFake) GetShipperDocument(_ context.Context, _, _, documentID uuid.UUID) (*domain.Document, error) {
	doc, ok := f.docs[documentID]
	if !ok {
		return nil, apperrors.NotFound("document not found")
	}
	return doc, nil
}

type shipperFileFake struct {
	opens int
	item  *domain.Attachment
}

func (f *shipperFileFake) ListFinalized(context.Context, uuid.UUID, uuid.UUID) ([]domain.Attachment, error) {
	if f.item == nil || f.item.Status != domain.AttachmentStatusFinalized {
		return nil, nil
	}
	return []domain.Attachment{*f.item}, nil
}

func (f *shipperFileFake) GetFinalized(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*domain.Attachment, error) {
	if f.item == nil || f.item.Status != domain.AttachmentStatusFinalized {
		return nil, apperrors.NotFound("attachment not found")
	}
	return f.item, nil
}

func (f *shipperFileFake) OpenFinalized(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*domain.Attachment, io.ReadCloser, error) {
	if f.item == nil || f.item.Status != domain.AttachmentStatusFinalized {
		return nil, nil, apperrors.NotFound("attachment not found")
	}
	f.opens++
	return f.item, io.NopCloser(strings.NewReader("pdf")), nil
}

func TestShipperListHidesForeignShipmentBeforeQuery(t *testing.T) {
	docs := &shipperDocFake{visible: false}
	svc := NewShipperDocumentService(docs, &shipperFileFake{})
	_, _, err := svc.List(context.Background(), domain.ShipperDocumentFilter{
		TenantID: uuid.New(), ShipperCompanyID: uuid.New(), ShipmentID: uuid.New(), Limit: 20,
	})
	if err == nil || docs.listed != 0 {
		t.Fatalf("err=%v listed=%d", err, docs.listed)
	}
}

func TestShipperFinancialFilterRejectedBeforeQuery(t *testing.T) {
	docs := &shipperDocFake{visible: true}
	svc := NewShipperDocumentService(docs, &shipperFileFake{})
	invoice := "INVOICE"
	_, _, err := svc.List(context.Background(), domain.ShipperDocumentFilter{
		TenantID: uuid.New(), ShipperCompanyID: uuid.New(), ShipmentID: uuid.New(),
		DocumentType: &invoice, Limit: 20,
	})
	if err == nil || docs.listed != 0 {
		t.Fatalf("err=%v listed=%d", err, docs.listed)
	}
}

func TestShipperContentDoesNotOpenForeignOrDraft(t *testing.T) {
	tenant := uuid.New()
	shipper := uuid.New()
	docID := uuid.New()
	other := uuid.New()
	att := uuid.New()
	docs := &shipperDocFake{docs: map[uuid.UUID]*domain.Document{
		docID: {ID: docID, TenantID: tenant, DocumentType: "POD", DocumentStatus: domain.DocumentStatusSigned},
	}}
	files := &shipperFileFake{item: &domain.Attachment{ID: att, Status: domain.AttachmentStatusDraft}}
	svc := NewShipperDocumentService(docs, files)
	if _, _, err := svc.OpenContent(context.Background(), tenant, shipper, other, att); err == nil || files.opens != 0 {
		t.Fatalf("foreign open err=%v opens=%d", err, files.opens)
	}
	if _, _, err := svc.OpenContent(context.Background(), tenant, shipper, docID, att); err == nil || files.opens != 0 {
		t.Fatalf("draft open err=%v opens=%d", err, files.opens)
	}
}
