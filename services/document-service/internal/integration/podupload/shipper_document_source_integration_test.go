//go:build integration

package podupload

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
	"github.com/freight-platform/document-service/internal/http/handlers"
	"github.com/freight-platform/document-service/internal/platform/storage"
	"github.com/freight-platform/document-service/internal/repository"
	"github.com/freight-platform/document-service/internal/service"
)

type countingStore struct {
	inner storage.ObjectStore
	gets  int
}

func (s *countingStore) Put(ctx context.Context, key string, reader io.Reader, maxBytes int64) (int64, error) {
	return s.inner.Put(ctx, key, reader, maxBytes)
}
func (s *countingStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	s.gets++
	return s.inner.Get(ctx, key)
}
func (s *countingStore) Exists(ctx context.Context, key string) (bool, error) {
	return s.inner.Exists(ctx, key)
}
func (s *countingStore) Metadata(ctx context.Context, key string) (storage.ObjectMetadata, error) {
	return s.inner.Metadata(ctx, key)
}
func (s *countingStore) Delete(ctx context.Context, key string) error {
	return s.inner.Delete(ctx, key)
}

func TestShipperDocumentSourceContract(t *testing.T) {
	env := setupTestEnv(t)
	applyAttachmentMigrations(t, env)
	fix := seedFixtures(t, env.pool)
	ctx := context.Background()
	var shipperA uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT shipper_company_id FROM transport.shipments WHERE id = $1`, fix.ShipmentA).Scan(&shipperA); err != nil {
		t.Fatal(err)
	}
	otherShipper := uuid.New()
	otherShipment := uuid.New()
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id,tenant_id,legal_name,company_type) VALUES ($1,$2,'Other shipper','SHIPPER')`, otherShipper, fix.TenantA); err != nil {
		t.Fatal(err)
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO transport.shipments (
			id, tenant_id, shipment_number, transport_order_id, shipper_company_id, consignee_company_id,
			carrier_company_id, driver_id, origin_location_id, destination_location_id, transport_mode, status, version
		)
		SELECT $1, tenant_id, 'SHP-OTHER', transport_order_id, $2, consignee_company_id,
			carrier_company_id, driver_id, origin_location_id, destination_location_id, transport_mode, status, version
		FROM transport.shipments WHERE id = $3`, otherShipment, otherShipper, fix.ShipmentA); err != nil {
		t.Fatal(err)
	}

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

	carrierOwned := create("POD-CARRIER", "POD", fix.CompanyA, fix.ShipmentA, domain.DocumentStatusSigned)
	var waybillID uuid.UUID
	for _, docType := range []string{"ETRN", "EPD", "WAYBILL", "DISCREPANCY_ACT", "ECMR"} {
		id := create(docType+"-1", docType, fix.CompanyA, fix.ShipmentA, domain.DocumentStatusSigned)
		if docType == "WAYBILL" {
			waybillID = id
		}
	}
	for _, status := range domain.ShipperVisibleDocumentStatuses() {
		if status == domain.DocumentStatusSigned {
			continue
		}
		create("POD-"+status, "POD", fix.CompanyA, fix.ShipmentA, status)
	}
	create("POD-DRAFT", "POD", shipperA, fix.ShipmentA, domain.DocumentStatusDraft)
	create("POD-CANCELLED", "POD", shipperA, fix.ShipmentA, domain.DocumentStatusCancelled)
	for _, docType := range []string{"INVOICE", "VAT_INVOICE", "ACT", "UPD", "CLAIM"} {
		create(docType+"-HIDDEN", docType, shipperA, fix.ShipmentA, domain.DocumentStatusSigned)
	}
	ownerOnly := create("POD-OTHER-SHIPMENT", "POD", shipperA, otherShipment, domain.DocumentStatusSigned)

	store := &countingStore{inner: env.storage}
	attachmentRepo := repository.NewAttachmentRepository(env.pool)
	attachmentSvc := service.NewAttachmentService(docRepo, store, attachmentRepo)
	shipper := service.NewShipperDocumentService(docRepo, attachmentSvc)
	handler := handlers.NewShipperDocumentHandler(shipper)
	router := chi.NewRouter()
	router.Get("/v1/shipper/documents", handler.List)
	router.Get("/v1/shipper/documents/{documentId}", handler.Get)
	router.Get("/v1/shipper/documents/{documentId}/attachments", handler.ListAttachments)
	router.Get("/v1/shipper/documents/{documentId}/attachments/{attachmentId}", handler.GetAttachment)
	router.Get("/v1/shipper/documents/{documentId}/attachments/{attachmentId}/content", handler.Content)

	list := doGet(router, "/v1/shipper/documents?shipper_company_id="+shipperA.String()+"&shipment_id="+fix.ShipmentA.String(), fix.TenantA.String())
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "POD-CARRIER") {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	for _, hidden := range []string{"POD-DRAFT", "POD-CANCELLED", "INVOICE-HIDDEN", "VAT_INVOICE-HIDDEN", "ACT-HIDDEN", "UPD-HIDDEN", "CLAIM-HIDDEN", "POD-OTHER-SHIPMENT"} {
		if strings.Contains(list.Body.String(), hidden) {
			t.Fatalf("list exposed %s", hidden)
		}
	}
	assertNoLeak(t, list.Body.String())
	foreignList := doGet(router, "/v1/shipper/documents?shipper_company_id="+otherShipper.String()+"&shipment_id="+fix.ShipmentA.String(), fix.TenantA.String())
	if foreignList.Code != http.StatusNotFound || strings.Contains(foreignList.Body.String(), "POD-CARRIER") {
		t.Fatalf("foreign shipment list status=%d", foreignList.Code)
	}
	detail := doGet(router, "/v1/shipper/documents/"+carrierOwned.String()+"?shipper_company_id="+shipperA.String(), fix.TenantA.String())
	if detail.Code != http.StatusOK || strings.Contains(detail.Body.String(), "payload") {
		t.Fatalf("detail status=%d", detail.Code)
	}
	assertNoLeak(t, detail.Body.String())
	if doGet(router, "/v1/shipper/documents/"+ownerOnly.String()+"?shipper_company_id="+shipperA.String(), fix.TenantA.String()).Code != http.StatusNotFound {
		t.Fatal("owner company authorized a document on another shipper shipment")
	}
	if doGet(router, "/v1/shipper/documents/"+carrierOwned.String()+"?shipper_company_id="+otherShipper.String(), fix.TenantA.String()).Code != http.StatusNotFound {
		t.Fatal("foreign shipper can read the document")
	}
	if doGet(router, "/v1/shipper/documents/"+carrierOwned.String()+"?shipper_company_id="+shipperA.String(), fix.TenantB.String()).Code != http.StatusNotFound {
		t.Fatal("wrong tenant can read the document")
	}

	body := []byte("%PDF-shipper")
	sum := domain.SHA256Hex(body)
	finalID := uuid.New()
	draftID := uuid.New()
	finalKey := "tenants/" + fix.TenantA.String() + "/documents/" + carrierOwned.String() + "/attachments/" + finalID.String()
	draftKey := "tenants/" + fix.TenantA.String() + "/documents/" + carrierOwned.String() + "/attachments/" + draftID.String()
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
			id, carrierOwned, fix.TenantA, len(body), sum, key, status, finalized)
		if err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC()
	insertAttachment(finalID, finalKey, domain.AttachmentStatusFinalized, &now)
	insertAttachment(draftID, draftKey, domain.AttachmentStatusDraft, nil)
	before := store.gets
	if doGet(router, "/v1/shipper/documents/"+carrierOwned.String()+"/attachments/"+draftID.String()+"/content?shipper_company_id="+shipperA.String(), fix.TenantA.String()).Code == http.StatusOK || store.gets != before {
		t.Fatalf("draft content was readable gets=%d", store.gets)
	}
	if doGet(router, "/v1/shipper/documents/"+carrierOwned.String()+"/attachments/"+finalID.String()+"/content?shipper_company_id="+otherShipper.String(), fix.TenantA.String()).Code == http.StatusOK || store.gets != before {
		t.Fatal("foreign shipper content read storage")
	}
	if doGet(router, "/v1/shipper/documents/"+waybillID.String()+"/attachments/"+finalID.String()+"/content?shipper_company_id="+shipperA.String(), fix.TenantA.String()).Code == http.StatusOK || store.gets != before {
		t.Fatal("attachment id was readable through another document")
	}
	content := doGet(router, "/v1/shipper/documents/"+carrierOwned.String()+"/attachments/"+finalID.String()+"/content?shipper_company_id="+shipperA.String(), fix.TenantA.String())
	if content.Code != http.StatusOK || content.Header().Get("X-Content-SHA256") != sum || store.gets != before+1 {
		t.Fatalf("content status=%d gets=%d", content.Code, store.gets)
	}
	if strings.Contains(content.Header().Get("Content-Disposition"), finalKey) {
		t.Fatal("content header discloses storage key")
	}
	attList := doGet(router, "/v1/shipper/documents/"+carrierOwned.String()+"/attachments?shipper_company_id="+shipperA.String(), fix.TenantA.String())
	if attList.Code != http.StatusOK || strings.Contains(attList.Body.String(), draftID.String()) || !strings.Contains(attList.Body.String(), finalID.String()) {
		t.Fatalf("attachment list status=%d body=%s", attList.Code, attList.Body.String())
	}
	assertNoLeak(t, attList.Body.String())
}

func applyAttachmentMigrations(t *testing.T, env *testEnv) {
	t.Helper()
	dir, err := locateMigrationsDir()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"000087_edo_0_3_i1_schema_foundation.up.sql",
		"000095_edo_0_3_i2_signed_file_attachment.up.sql",
	} {
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := env.pool.Exec(context.Background(), string(content)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func assertNoLeak(t *testing.T, body string) {
	t.Helper()
	for _, token := range []string{"storage_key", "storageKey", "bucket_name", "object_key", "payload_json", "idempotency_key"} {
		if strings.Contains(body, token) {
			t.Fatalf("body contains %s", token)
		}
	}
}
