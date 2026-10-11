//go:build integration

package podupload

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
	"github.com/freight-platform/document-service/internal/http/handlers"
	"github.com/freight-platform/document-service/internal/repository"
	"github.com/freight-platform/document-service/internal/service"
)

func TestCarrierDocumentSourceContract(t *testing.T) {
	env := setupTestEnv(t)
	applyAttachmentMigrations(t, env)
	fix := seedFixtures(t, env.pool)
	ctx := context.Background()

	var carrierA, shipperA, consigneeA uuid.UUID
	if err := env.pool.QueryRow(ctx, `
		SELECT carrier_company_id, shipper_company_id, consignee_company_id
		FROM transport.shipments WHERE id = $1`, fix.ShipmentA).Scan(&carrierA, &shipperA, &consigneeA); err != nil {
		t.Fatal(err)
	}
	forwarder := uuid.New()
	otherCarrier := uuid.New()
	otherShipment := uuid.New()
	deletedShipment := uuid.New()
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id,tenant_id,legal_name,company_type) VALUES ($1,$2,'Forwarder','FORWARDER'), ($3,$2,'Other carrier','CARRIER')`,
		forwarder, fix.TenantA, otherCarrier); err != nil {
		t.Fatal(err)
	}
	copyShipment := func(id, carrier uuid.UUID) {
		t.Helper()
		_, err := env.pool.Exec(ctx, `
			INSERT INTO transport.shipments (
				id, tenant_id, shipment_number, transport_order_id, shipper_company_id, consignee_company_id,
				carrier_company_id, driver_id, origin_location_id, destination_location_id, transport_mode, status, version
			)
			SELECT $1, tenant_id, $2, transport_order_id, shipper_company_id, consignee_company_id,
				$3, driver_id, origin_location_id, destination_location_id, transport_mode, status, version
			FROM transport.shipments WHERE id = $4`, id, "SHP-"+id.String()[:8], carrier, fix.ShipmentA)
		if err != nil {
			t.Fatal(err)
		}
	}
	copyShipment(otherShipment, otherCarrier)
	copyShipment(deletedShipment, carrierA)

	docRepo := repository.NewDocumentRepository(env.pool)
	docs := service.NewDocumentService(docRepo)
	rel := domain.RelatedEntityTypeShipment
	create := func(number, docType string, owner, shipment uuid.UUID, status string) uuid.UUID {
		t.Helper()
		doc, err := docs.Create(ctx, domain.CreateDocumentInput{
			TenantID: fix.TenantA, DocumentNumber: number, DocumentType: docType,
			OwnerCompanyID: owner, RelatedEntityType: &rel, RelatedEntityID: &shipment, LegalLanguage: "ru-RU",
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := env.pool.Exec(ctx, `UPDATE documents.documents SET document_status = $2 WHERE id = $1`, doc.ID, status); err != nil {
			t.Fatal(err)
		}
		return doc.ID
	}

	shipperOwned := create("POD-SHIPPER-OWNER", "POD", shipperA, fix.ShipmentA, domain.DocumentStatusSigned)
	var waybillID uuid.UUID
	for _, docType := range []string{"ETRN", "EPD", "WAYBILL", "DISCREPANCY_ACT", "ECMR"} {
		id := create(docType+"-1", docType, carrierA, fix.ShipmentA, domain.DocumentStatusSigned)
		if docType == "WAYBILL" {
			waybillID = id
		}
	}
	for _, status := range domain.CarrierVisibleDocumentStatuses() {
		if status == domain.DocumentStatusSigned {
			continue
		}
		create("POD-"+status, "POD", shipperA, fix.ShipmentA, status)
	}
	create("POD-DRAFT", "POD", carrierA, fix.ShipmentA, domain.DocumentStatusDraft)
	create("POD-CANCELLED", "POD", carrierA, fix.ShipmentA, domain.DocumentStatusCancelled)
	for _, docType := range []string{"INVOICE", "VAT_INVOICE", "ACT", "UPD", "CLAIM"} {
		create(docType+"-HIDDEN", docType, carrierA, fix.ShipmentA, domain.DocumentStatusSigned)
	}
	ownerOnly := create("POD-OTHER-CARRIER", "POD", carrierA, otherShipment, domain.DocumentStatusSigned)
	deletedDoc := create("POD-DELETED", "POD", carrierA, fix.ShipmentA, domain.DocumentStatusReadyForSigning)
	if _, err := env.pool.Exec(ctx, `UPDATE documents.documents SET deleted_at = now() WHERE id = $1`, deletedDoc); err != nil {
		t.Fatal(err)
	}
	deletedShipmentDoc := create("POD-DELETED-SHIPMENT", "POD", carrierA, deletedShipment, domain.DocumentStatusSigned)
	if _, err := env.pool.Exec(ctx, `UPDATE transport.shipments SET deleted_at = now() WHERE id = $1`, deletedShipment); err != nil {
		t.Fatal(err)
	}

	store := &countingStore{inner: env.storage}
	attachmentRepo := repository.NewAttachmentRepository(env.pool)
	attachmentSvc := service.NewAttachmentService(docRepo, store, attachmentRepo)
	carrier := service.NewCarrierDocumentService(docRepo, attachmentSvc)
	handler := handlers.NewCarrierDocumentHandler(carrier)
	router := chi.NewRouter()
	router.Get("/v1/carrier/documents", handler.List)
	router.Get("/v1/carrier/documents/{documentId}", handler.Get)
	router.Get("/v1/carrier/documents/{documentId}/attachments", handler.ListAttachments)
	router.Get("/v1/carrier/documents/{documentId}/attachments/{attachmentId}", handler.GetAttachment)
	router.Get("/v1/carrier/documents/{documentId}/attachments/{attachmentId}/content", handler.Content)

	listPath := "/v1/carrier/documents?carrier_company_id=" + carrierA.String() + "&shipment_id=" + fix.ShipmentA.String()
	list := doGet(router, listPath, fix.TenantA.String())
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "POD-SHIPPER-OWNER") {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	for _, hidden := range []string{"POD-DRAFT", "POD-CANCELLED", "INVOICE-HIDDEN", "VAT_INVOICE-HIDDEN", "ACT-HIDDEN", "UPD-HIDDEN", "CLAIM-HIDDEN", "POD-OTHER-CARRIER", "POD-DELETED", "POD-DELETED-SHIPMENT"} {
		if strings.Contains(list.Body.String(), hidden) {
			t.Fatalf("list exposed %s", hidden)
		}
	}
	for _, status := range domain.CarrierVisibleDocumentStatuses() {
		if status == domain.DocumentStatusSigned {
			continue
		}
		if !strings.Contains(list.Body.String(), "POD-"+status) {
			t.Fatalf("list hid %s", status)
		}
	}
	assertNoLeak(t, list.Body.String())

	for _, company := range []uuid.UUID{otherCarrier, shipperA, consigneeA, forwarder} {
		rec := doGet(router, "/v1/carrier/documents?carrier_company_id="+company.String()+"&shipment_id="+fix.ShipmentA.String(), fix.TenantA.String())
		if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "POD-SHIPPER-OWNER") {
			t.Fatalf("company %s status=%d", company, rec.Code)
		}
	}
	if doGet(router, listPath, fix.TenantB.String()).Code != http.StatusNotFound {
		t.Fatal("wrong tenant listed documents")
	}
	if doGet(router, "/v1/carrier/documents?carrier_company_id="+carrierA.String()+"&shipment_id="+deletedShipment.String(), fix.TenantA.String()).Code != http.StatusNotFound {
		t.Fatal("deleted shipment was listed")
	}

	detail := doGet(router, "/v1/carrier/documents/"+shipperOwned.String()+"?carrier_company_id="+carrierA.String(), fix.TenantA.String())
	if detail.Code != http.StatusOK {
		t.Fatalf("detail status=%d body=%s", detail.Code, detail.Body.String())
	}
	assertNoLeak(t, detail.Body.String())
	if doGet(router, "/v1/carrier/documents/"+ownerOnly.String()+"?carrier_company_id="+carrierA.String(), fix.TenantA.String()).Code != http.StatusNotFound {
		t.Fatal("owner company authorized a document on another carrier shipment")
	}
	if doGet(router, "/v1/carrier/documents/"+ownerOnly.String()+"?carrier_company_id="+otherCarrier.String(), fix.TenantA.String()).Code != http.StatusOK {
		t.Fatal("participating carrier cannot read an operational document it does not own")
	}
	for _, id := range []uuid.UUID{shipperOwned, deletedDoc, deletedShipmentDoc} {
		if doGet(router, "/v1/carrier/documents/"+id.String()+"?carrier_company_id="+otherCarrier.String(), fix.TenantA.String()).Code != http.StatusNotFound {
			t.Fatalf("foreign carrier read %s", id)
		}
	}
	if doGet(router, "/v1/carrier/documents/"+shipperOwned.String()+"?carrier_company_id="+carrierA.String(), fix.TenantB.String()).Code != http.StatusNotFound {
		t.Fatal("wrong tenant can read the document")
	}
	if doGet(router, "/v1/carrier/documents/"+deletedDoc.String()+"?carrier_company_id="+carrierA.String(), fix.TenantA.String()).Code != http.StatusNotFound {
		t.Fatal("deleted document is visible")
	}
	for _, number := range []string{"INVOICE-HIDDEN", "VAT_INVOICE-HIDDEN", "ACT-HIDDEN", "UPD-HIDDEN", "CLAIM-HIDDEN", "POD-DRAFT", "POD-CANCELLED"} {
		var id uuid.UUID
		if err := env.pool.QueryRow(ctx, `SELECT id FROM documents.documents WHERE document_number = $1 AND tenant_id = $2`, number, fix.TenantA).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if doGet(router, "/v1/carrier/documents/"+id.String()+"?carrier_company_id="+carrierA.String(), fix.TenantA.String()).Code != http.StatusNotFound {
			t.Fatalf("detail exposed %s", number)
		}
	}

	body := []byte("%PDF-carrier")
	sum := domain.SHA256Hex(body)
	finalID := uuid.New()
	draftID := uuid.New()
	finalKey := "tenants/" + fix.TenantA.String() + "/documents/" + shipperOwned.String() + "/attachments/" + finalID.String()
	draftKey := "tenants/" + fix.TenantA.String() + "/documents/" + shipperOwned.String() + "/attachments/" + draftID.String()
	if _, err := env.storage.Put(ctx, finalKey, bytes.NewReader(body), int64(len(body)+1)); err != nil {
		t.Fatal(err)
	}
	if _, err := env.storage.Put(ctx, draftKey, bytes.NewReader(body), int64(len(body)+1)); err != nil {
		t.Fatal(err)
	}
	insertAttachment := func(id uuid.UUID, key, status string, finalized *time.Time) {
		t.Helper()
		_, err := env.pool.Exec(ctx, `
			INSERT INTO documents.document_attachments (
				id, document_id, tenant_id, file_name, media_type, size_bytes, sha256, storage_key, status, finalized_at
			) VALUES ($1,$2,$3,'pod.pdf','application/pdf',$4,$5,$6,$7,$8)`,
			id, shipperOwned, fix.TenantA, len(body), sum, key, status, finalized)
		if err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC()
	insertAttachment(finalID, finalKey, domain.AttachmentStatusFinalized, &now)
	insertAttachment(draftID, draftKey, domain.AttachmentStatusDraft, nil)
	before := store.gets
	if doGet(router, "/v1/carrier/documents/"+shipperOwned.String()+"/attachments/"+draftID.String()+"/content?carrier_company_id="+carrierA.String(), fix.TenantA.String()).Code == http.StatusOK || store.gets != before {
		t.Fatalf("draft content was readable gets=%d", store.gets)
	}
	if doGet(router, "/v1/carrier/documents/"+shipperOwned.String()+"/attachments/"+finalID.String()+"/content?carrier_company_id="+otherCarrier.String(), fix.TenantA.String()).Code == http.StatusOK || store.gets != before {
		t.Fatal("foreign carrier content read storage")
	}
	if doGet(router, "/v1/carrier/documents/"+waybillID.String()+"/attachments/"+finalID.String()+"/content?carrier_company_id="+carrierA.String(), fix.TenantA.String()).Code == http.StatusOK || store.gets != before {
		t.Fatal("attachment id was readable through another document")
	}
	content := doGet(router, "/v1/carrier/documents/"+shipperOwned.String()+"/attachments/"+finalID.String()+"/content?carrier_company_id="+carrierA.String(), fix.TenantA.String())
	if content.Code != http.StatusOK || content.Header().Get("Content-Type") != "application/pdf" || content.Header().Get("X-Content-SHA256") != sum || store.gets != before+1 {
		t.Fatalf("content status=%d type=%s gets=%d", content.Code, content.Header().Get("Content-Type"), store.gets)
	}
	if !strings.Contains(content.Header().Get("Content-Disposition"), `filename="pod.pdf"`) || strings.Contains(content.Header().Get("Content-Disposition"), finalKey) {
		t.Fatalf("disposition=%s", content.Header().Get("Content-Disposition"))
	}
	attList := doGet(router, "/v1/carrier/documents/"+shipperOwned.String()+"/attachments?carrier_company_id="+carrierA.String(), fix.TenantA.String())
	if attList.Code != http.StatusOK || strings.Contains(attList.Body.String(), draftID.String()) || !strings.Contains(attList.Body.String(), finalID.String()) {
		t.Fatalf("attachment list status=%d body=%s", attList.Code, attList.Body.String())
	}
	assertNoLeak(t, attList.Body.String())
}
