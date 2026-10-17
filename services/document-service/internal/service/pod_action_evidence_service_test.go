package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
	"github.com/freight-platform/document-service/internal/platform/storage"
)

type actionWorld struct {
	operating, shipmentTenant, company, driver, authorCompany uuid.UUID
	execution, action, stop, shipment, cargo                  uuid.UUID
}

func newWorld() actionWorld {
	return actionWorld{
		operating: uuid.New(), shipmentTenant: uuid.New(), company: uuid.New(),
		driver: uuid.New(), authorCompany: uuid.New(), execution: uuid.New(),
		action: uuid.New(), stop: uuid.New(), shipment: uuid.New(), cargo: uuid.New(),
	}
}

func (w actionWorld) input(key string) CreateActionPODIntentInput {
	return CreateActionPODIntentInput{
		ActionType: domain.PODActionTypeDelivery, DocumentTenantID: w.operating, OperatingTenantID: w.operating,
		ExecutionID: w.execution, ExecutionActionID: w.action, ExecutionStopID: w.stop,
		ShipmentID: w.shipment, ShipmentTenantID: w.shipmentTenant, CargoID: w.cargo,
		OwnerCompanyID: w.company, AuthorDriverID: w.driver, AuthorCompanyID: w.authorCompany,
		MimeType: "image/jpeg", FileName: "pod.jpg", IdempotencyKey: key,
	}
}

type memActionStore struct {
	mu           sync.Mutex
	companies    map[string]struct{}
	shipments    map[uuid.UUID]uuid.UUID
	actions      map[string]*domain.DeliveryActionSubject
	intents      map[string]*domain.PODActionIntent
	byID         map[uuid.UUID]*domain.PODActionIntent
	evidence     map[uuid.UUID]*domain.PODActionEvidence
	files        map[uuid.UUID]struct{}
	deleted      map[uuid.UUID]struct{}
	inserts      int
	listErr      error
	requirements int
	provider     string
	bucket       *string
}

func newMemActionStore() *memActionStore {
	return &memActionStore{
		companies: map[string]struct{}{},
		shipments: map[uuid.UUID]uuid.UUID{},
		actions:   map[string]*domain.DeliveryActionSubject{},
		intents:   map[string]*domain.PODActionIntent{},
		byID:      map[uuid.UUID]*domain.PODActionIntent{},
		evidence:  map[uuid.UUID]*domain.PODActionEvidence{},
		files:     map[uuid.UUID]struct{}{},
		deleted:   map[uuid.UUID]struct{}{},
	}
}

func (m *memActionStore) seedDelivery(w actionWorld) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.companies[w.company.String()+"|"+w.operating.String()] = struct{}{}
	m.shipments[w.shipment] = w.shipmentTenant
	m.actions[actionKey(w.operating, w.execution, w.action)] = &domain.DeliveryActionSubject{
		ActionType: domain.PODActionTypeDelivery, OperatingTenantID: w.operating, ExecutionID: w.execution,
		ExecutionActionID: w.action, ExecutionStopID: w.stop, ShipmentID: w.shipment,
		ShipmentTenantID: w.shipmentTenant, CargoID: w.cargo, CarrierCompanyID: w.company,
	}
}

func actionKey(op, exec, action uuid.UUID) string {
	return op.String() + "|" + exec.String() + "|" + action.String()
}

func (m *memActionStore) CompanyExists(_ context.Context, companyID, tenantID uuid.UUID) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.companies[companyID.String()+"|"+tenantID.String()]
	return ok, nil
}

func (m *memActionStore) FindIntentByIdempotency(_ context.Context, documentTenantID uuid.UUID, key string) (*domain.PODActionIntent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	intent := m.intents[documentTenantID.String()+"|"+key]
	if intent == nil {
		return nil, nil
	}
	copy := *intent
	return &copy, nil
}

func (m *memActionStore) LoadIntent(_ context.Context, uploadID uuid.UUID) (*domain.PODActionIntent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	intent := m.byID[uploadID]
	if intent == nil {
		return nil, apperrors.NotFound("upload intent not found")
	}
	copy := *intent
	return &copy, nil
}

func (m *memActionStore) InsertIntent(_ context.Context, in domain.PODActionIntent) (domain.PODActionIntent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.intents[in.DocumentTenantID.String()+"|"+in.IdempotencyKey]; exists {
		return domain.PODActionIntent{}, apperrors.Conflict("record already exists", nil)
	}
	in.Status = "pending"
	in.DocumentVersionID = uuid.New()
	stored := in
	m.intents[in.DocumentTenantID.String()+"|"+in.IdempotencyKey] = &stored
	m.byID[in.ID] = &stored
	m.inserts++
	return stored, nil
}

func (m *memActionStore) RotateUploadToken(_ context.Context, uploadID uuid.UUID, tokenHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	intent := m.byID[uploadID]
	if intent == nil || intent.Status == "completed" {
		return apperrors.Conflict("completed upload does not rotate its token", nil)
	}
	intent.UploadTokenHash = tokenHash
	return nil
}

func (m *memActionStore) MarkIntentUploaded(_ context.Context, uploadID uuid.UUID, size int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	intent := m.byID[uploadID]
	if intent == nil || (intent.Status != "pending" && intent.Status != "uploaded") {
		return apperrors.Conflict("upload intent is not active", nil)
	}
	intent.Status = "uploaded"
	intent.ByteSize = size
	return nil
}

func (m *memActionStore) EvidenceByIntent(_ context.Context, uploadID uuid.UUID) (*domain.PODActionEvidence, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	item := m.evidence[uploadID]
	if item == nil {
		return nil, apperrors.Internal("completed upload has no action evidence binding", nil)
	}
	copy := *item
	return &copy, nil
}

func (m *memActionStore) CompleteIntent(_ context.Context, uploadID uuid.UUID, checksum, storageProvider string, bucketName *string) (*domain.PODActionEvidence, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.provider = storageProvider
	m.bucket = bucketName
	intent := m.byID[uploadID]
	if intent == nil {
		return nil, apperrors.NotFound("upload intent not found")
	}
	if intent.Status == "completed" {
		item := m.evidence[uploadID]
		if item == nil {
			return nil, apperrors.Internal("completed upload has no action evidence binding", nil)
		}
		copy := *item
		return &copy, nil
	}
	if intent.Status != "uploaded" {
		return nil, apperrors.Validation("upload content required before finalize", nil)
	}
	fileID := uuid.New()
	item := &domain.PODActionEvidence{
		ID: uuid.New(), UploadIntentID: intent.ID, DocumentTenantID: intent.DocumentTenantID,
		OperatingTenantID: intent.OperatingTenantID, ExecutionID: intent.ExecutionID,
		ExecutionActionID: intent.ExecutionActionID, ExecutionStopID: intent.ExecutionStopID,
		ShipmentID: intent.ShipmentID, ShipmentTenantID: intent.ShipmentTenantID, CargoID: intent.CargoID,
		OwnerCompanyID: intent.OwnerCompanyID, AuthorDriverID: intent.AuthorDriverID,
		AuthorCompanyID: intent.AuthorCompanyID, DocumentID: intent.DocumentID,
		DocumentVersionID: intent.DocumentVersionID, DocumentFileID: fileID,
		IdempotencyKey: intent.IdempotencyKey,
	}
	intent.Status = "completed"
	intent.ChecksumSHA256 = checksum
	m.files[fileID] = struct{}{}
	m.evidence[uploadID] = item
	copy := *item
	return &copy, nil
}

func (m *memActionStore) ListPresentEvidence(_ context.Context, operatingTenantID, executionID, actionID uuid.UUID) ([]domain.PODActionEvidence, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.listErr != nil {
		return nil, m.listErr
	}
	out := make([]domain.PODActionEvidence, 0)
	for _, item := range m.evidence {
		if item.OperatingTenantID != operatingTenantID || item.ExecutionID != executionID || item.ExecutionActionID != actionID {
			continue
		}
		intent := m.byID[item.UploadIntentID]
		if intent == nil || intent.Status != "completed" {
			continue
		}
		if _, ok := m.files[item.DocumentFileID]; !ok {
			continue
		}
		if _, gone := m.deleted[item.DocumentID]; gone {
			continue
		}
		out = append(out, *item)
	}
	return out, nil
}

func testActionService(t *testing.T, store *memActionStore) *PODActionEvidenceService {
	t.Helper()
	objects, err := storage.NewLocalObjectStore(filepath.Join(t.TempDir(), "objects"))
	if err != nil {
		t.Fatal(err)
	}
	return NewPODActionEvidenceService(store, objects, 1024, ActionObjectPlacement{StorageProvider: "local"})
}

func codeOf(err error) apperrors.Code {
	var app *apperrors.AppError
	if errors.As(err, &app) {
		return app.Code
	}
	return ""
}

func completeFlow(t *testing.T, svc *PODActionEvidenceService, in CreateActionPODIntentInput) *CompleteActionPODResult {
	t.Helper()
	ctx := context.Background()
	intent, err := svc.CreateUploadIntent(ctx, in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.UploadContent(ctx, intent.UploadID, intent.UploadToken, bytes.NewReader([]byte("jpeg")), domain.SubjectConfirmation{}); err != nil {
		t.Fatalf("content: %v", err)
	}
	done, err := svc.CompleteUpload(ctx, intent.UploadID, "abc", domain.SubjectConfirmation{})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	return done
}

func TestPOD_BIND_01(t *testing.T) {
	store := newMemActionStore()
	w := newWorld()
	store.seedDelivery(w)
	svc := testActionService(t, store)
	done := completeFlow(t, svc, w.input("k1"))
	rows, err := svc.GetEvidence(context.Background(), w.operating, w.execution, w.action)
	if err != nil || len(rows) != 1 || rows[0].DocumentID != done.DocumentID || rows[0].DocumentFileID != done.FileID {
		t.Fatalf("evidence=%v err=%v", rows, err)
	}
	if rows[0].DocumentTenantID != w.operating || rows[0].ShipmentTenantID != w.shipmentTenant {
		t.Fatalf("tenant pin document=%s shipment=%s", rows[0].DocumentTenantID, rows[0].ShipmentTenantID)
	}
}

func TestPOD_BIND_02(t *testing.T) {
	store := newMemActionStore()
	w := newWorld()
	store.seedDelivery(w)
	svc := testActionService(t, store)
	in := w.input("k2")
	in.ActionType = "PICKUP"
	if _, err := svc.CreateUploadIntent(context.Background(), in); codeOf(err) != apperrors.CodeValidation || store.inserts != 0 {
		t.Fatalf("command pickup: %v inserts=%d", err, store.inserts)
	}
}

func TestPOD_BIND_03(t *testing.T) {
	store := newMemActionStore()
	w := newWorld()
	store.seedDelivery(w)
	svc := testActionService(t, store)
	done := completeFlow(t, svc, w.input("k3"))
	rows, err := svc.GetEvidence(context.Background(), w.operating, uuid.New(), uuid.New())
	if err != nil || len(rows) != 0 {
		t.Fatalf("unknown scope: %+v %v", rows, err)
	}
	for _, row := range rows {
		if row.DocumentID == done.DocumentID {
			t.Fatal("unknown scope leaked document id")
		}
	}
}

func TestPOD_BIND_05(t *testing.T) {
	store := newMemActionStore()
	w := newWorld()
	store.seedDelivery(w)
	svc := testActionService(t, store)
	ctx := context.Background()
	in := w.input("k5")
	intent, err := svc.CreateUploadIntent(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	otherExec := uuid.New()
	in.ExecutionID = otherExec
	if _, err := svc.CreateUploadIntent(ctx, in); codeOf(err) != apperrors.CodeConflict {
		t.Fatalf("replay execution mismatch: %v", err)
	}
	bad := otherExec
	err = svc.UploadContent(ctx, intent.UploadID, intent.UploadToken, bytes.NewReader([]byte("x")), domain.SubjectConfirmation{ExecutionID: &bad})
	if codeOf(err) != apperrors.CodeConflict {
		t.Fatalf("content execution mismatch: %v", err)
	}
}

func TestPOD_BIND_06(t *testing.T) {
	store := newMemActionStore()
	w := newWorld()
	store.seedDelivery(w)
	svc := testActionService(t, store)
	ctx := context.Background()
	in := w.input("k6")
	intent, err := svc.CreateUploadIntent(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.UploadContent(ctx, intent.UploadID, intent.UploadToken, bytes.NewReader([]byte("x")), domain.SubjectConfirmation{}); err != nil {
		t.Fatal(err)
	}
	other := uuid.New()
	if _, err := svc.CompleteUpload(ctx, intent.UploadID, "", domain.SubjectConfirmation{ShipmentID: &other}); codeOf(err) != apperrors.CodeConflict || len(store.evidence) != 0 {
		t.Fatalf("complete shipment mismatch: %v evidence=%d", err, len(store.evidence))
	}
	in.ShipmentID = other
	if _, err := svc.CreateUploadIntent(ctx, in); codeOf(err) != apperrors.CodeConflict {
		t.Fatalf("replay shipment mismatch: %v", err)
	}
}

func TestPOD_BIND_07(t *testing.T) {
	store := newMemActionStore()
	w := newWorld()
	store.seedDelivery(w)
	svc := testActionService(t, store)
	done := completeFlow(t, svc, w.input("k7"))
	foreign := uuid.New()
	rows, err := svc.GetEvidence(context.Background(), foreign, w.execution, w.action)
	if err != nil || len(rows) != 0 {
		t.Fatalf("foreign read: %+v %v", rows, err)
	}
	for _, row := range rows {
		if row.DocumentID == done.DocumentID {
			t.Fatal("foreign operating tenant leaked document id")
		}
	}
	in := w.input("k7-missing-company")
	in.DocumentTenantID = foreign
	in.OperatingTenantID = foreign
	if _, err := svc.CreateUploadIntent(context.Background(), in); codeOf(err) != apperrors.CodeNotFound {
		t.Fatalf("company outside document tenant: %v", err)
	}
}

func TestPOD_BIND_08(t *testing.T) {
	store := newMemActionStore()
	w := newWorld()
	store.seedDelivery(w)
	svc := testActionService(t, store)
	in := w.input("k8")
	if _, err := svc.CreateUploadIntent(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	in.ShipmentTenantID = uuid.New()
	if _, err := svc.CreateUploadIntent(context.Background(), in); codeOf(err) != apperrors.CodeConflict || store.inserts != 1 {
		t.Fatalf("shipment tenant replay: %v inserts=%d", err, store.inserts)
	}
}

func TestPOD_BIND_09(t *testing.T) {
	store := newMemActionStore()
	w := newWorld()
	store.seedDelivery(w)
	svc := testActionService(t, store)
	ctx := context.Background()
	in := w.input("k9")
	if _, err := svc.CreateUploadIntent(ctx, in); err != nil {
		t.Fatal(err)
	}
	in.AuthorDriverID = uuid.New()
	if _, err := svc.CreateUploadIntent(ctx, in); codeOf(err) != apperrors.CodeConflict {
		t.Fatalf("driver mismatch: %v", err)
	}
	in.AuthorDriverID = w.driver
	in.AuthorCompanyID = uuid.New()
	if _, err := svc.CreateUploadIntent(ctx, in); codeOf(err) != apperrors.CodeConflict {
		t.Fatalf("author company mismatch: %v", err)
	}
}

func TestPOD_BIND_10(t *testing.T) {
	store := newMemActionStore()
	w := newWorld()
	store.seedDelivery(w)
	svc := testActionService(t, store)
	ctx := context.Background()
	first, err := svc.CreateUploadIntent(ctx, w.input("same"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.CreateUploadIntent(ctx, w.input("same"))
	if err != nil {
		t.Fatal(err)
	}
	if !second.Replayed || first.UploadID != second.UploadID || first.DocumentID != second.DocumentID || store.inserts != 1 {
		t.Fatalf("replay first=%s second=%s inserts=%d", first.UploadID, second.UploadID, store.inserts)
	}
}

func TestPOD_BIND_11(t *testing.T) {
	store := newMemActionStore()
	w := newWorld()
	other := newWorld()
	other.operating = w.operating
	other.company = w.company
	store.seedDelivery(w)
	store.seedDelivery(other)
	svc := testActionService(t, store)
	ctx := context.Background()
	if _, err := svc.CreateUploadIntent(ctx, w.input("shared")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateUploadIntent(ctx, other.input("shared")); codeOf(err) != apperrors.CodeConflict || store.inserts != 1 {
		t.Fatalf("cross-action key: %v inserts=%d", err, store.inserts)
	}
}

func TestPOD_BIND_11B(t *testing.T) {
	store := newMemActionStore()
	w := newWorld()
	store.seedDelivery(w)
	svc := testActionService(t, store)
	in := w.input(" ")
	if _, err := svc.CreateUploadIntent(context.Background(), in); codeOf(err) != apperrors.CodeValidation || store.inserts != 0 {
		t.Fatalf("blank key: %v inserts=%d", err, store.inserts)
	}
}

func TestPOD_BIND_12(t *testing.T) {
	store := newMemActionStore()
	w := newWorld()
	store.seedDelivery(w)
	svc := testActionService(t, store)
	done := completeFlow(t, svc, w.input("k12"))
	again, err := svc.CompleteUpload(context.Background(), done.UploadID, "abc", domain.SubjectConfirmation{})
	if err != nil {
		t.Fatal(err)
	}
	if again.EvidenceID != done.EvidenceID || again.FileID != done.FileID || len(store.evidence) != 1 {
		t.Fatalf("retry evidence=%s file=%s count=%d", again.EvidenceID, again.FileID, len(store.evidence))
	}
}

func TestPOD_BIND_13(t *testing.T) {
	store := newMemActionStore()
	first := newWorld()
	second := newWorld()
	second.operating = first.operating
	second.shipment = first.shipment
	second.shipmentTenant = first.shipmentTenant
	second.company = first.company
	store.seedDelivery(first)
	store.seedDelivery(second)
	svc := testActionService(t, store)
	a := completeFlow(t, svc, first.input("a"))
	b := completeFlow(t, svc, second.input("b"))
	ctx := context.Background()
	left, err := svc.GetEvidence(ctx, first.operating, first.execution, first.action)
	if err != nil || len(left) != 1 || left[0].DocumentID != a.DocumentID {
		t.Fatalf("left: %+v %v", left, err)
	}
	right, err := svc.GetEvidence(ctx, second.operating, second.execution, second.action)
	if err != nil || len(right) != 1 || right[0].DocumentID != b.DocumentID || right[0].DocumentID == a.DocumentID {
		t.Fatalf("right: %+v %v", right, err)
	}
}

func TestPOD_BIND_14(t *testing.T) {
	store := newMemActionStore()
	original := newWorld()
	successor := newWorld()
	successor.operating = original.operating
	successor.shipment = original.shipment
	successor.shipmentTenant = original.shipmentTenant
	successor.company = original.company
	store.seedDelivery(original)
	store.seedDelivery(successor)
	svc := testActionService(t, store)
	done := completeFlow(t, svc, original.input("orig"))
	rows, err := svc.GetEvidence(context.Background(), successor.operating, successor.execution, successor.action)
	if err != nil || len(rows) != 0 {
		t.Fatalf("successor inherited: %+v %v", rows, err)
	}
	if rows != nil {
		for _, row := range rows {
			if row.DocumentID == done.DocumentID {
				t.Fatal("successor returned the original document")
			}
		}
	}
}

func TestPOD_BIND_15(t *testing.T) {
	store := newMemActionStore()
	w := newWorld()
	store.seedDelivery(w)
	svc := testActionService(t, store)
	historical := uuid.New()
	rows, err := svc.GetEvidence(context.Background(), w.operating, w.execution, w.action)
	if err != nil || len(rows) != 0 {
		t.Fatalf("unbound subject: %+v %v", rows, err)
	}
	for _, row := range rows {
		if row.DocumentID == historical {
			t.Fatal("historical document was bound")
		}
	}
}

func TestPOD_BIND_16(t *testing.T) {
	store := newMemActionStore()
	w := newWorld()
	store.seedDelivery(w)
	svc := testActionService(t, store)
	before := store.requirements
	_ = completeFlow(t, svc, w.input("k16"))
	if _, err := svc.GetEvidence(context.Background(), w.operating, w.execution, w.action); err != nil {
		t.Fatal(err)
	}
	if store.requirements != before {
		t.Fatal("evidence presence changed a requirement counter")
	}
}

func TestPOD_BIND_17(t *testing.T) {
	store := newMemActionStore()
	w := newWorld()
	store.seedDelivery(w)
	svc := testActionService(t, store)
	done := completeFlow(t, svc, w.input("k17"))
	store.requirements = 1
	rows, err := svc.GetEvidence(context.Background(), w.operating, w.execution, w.action)
	if err != nil || len(rows) != 1 || rows[0].DocumentID != done.DocumentID {
		t.Fatalf("known-not-required removed evidence: %+v %v", rows, err)
	}
	if store.evidence[done.UploadID] == nil || store.requirements != 1 {
		t.Fatal("evidence or requirement marker changed")
	}
}

func TestPOD_BIND_18(t *testing.T) {
	store := newMemActionStore()
	w := newWorld()
	store.seedDelivery(w)
	store.requirements = 1
	svc := testActionService(t, store)
	rows, err := svc.GetEvidence(context.Background(), w.operating, w.execution, w.action)
	if err != nil || len(rows) != 0 || store.requirements != 1 {
		t.Fatalf("missing evidence mutated requirement: rows=%d err=%v marker=%d", len(rows), err, store.requirements)
	}
}

func TestPOD_BIND_19(t *testing.T) {
	store := newMemActionStore()
	w := newWorld()
	store.seedDelivery(w)
	store.listErr = apperrors.Internal("repository unavailable", nil)
	svc := testActionService(t, store)
	rows, err := svc.GetEvidence(context.Background(), w.operating, w.execution, w.action)
	if err == nil || rows != nil {
		t.Fatalf("outage returned rows=%v err=%v", rows, err)
	}
}

func TestPOD_BIND_20(t *testing.T) {
	store := newMemActionStore()
	w := newWorld()
	store.seedDelivery(w)
	svc := testActionService(t, store)
	done := completeFlow(t, svc, w.input("k20"))
	rows, err := svc.GetEvidence(context.Background(), w.shipmentTenant, w.execution, w.action)
	if err != nil || len(rows) != 0 {
		t.Fatalf("cross-tenant read: %+v %v", rows, err)
	}
	for _, row := range rows {
		if row.DocumentID == done.DocumentID {
			t.Fatal("cross-tenant read leaked document id")
		}
	}
}

func TestCrossShipperActionPOD(t *testing.T) {
	store := newMemActionStore()
	w := newWorld()
	if w.operating == w.shipmentTenant {
		t.Fatal("fixture tenants collided")
	}
	store.seedDelivery(w)
	svc := testActionService(t, store)
	done := completeFlow(t, svc, w.input("cross"))
	rows, err := svc.GetEvidence(context.Background(), w.operating, w.execution, w.action)
	if err != nil || len(rows) != 1 {
		t.Fatal(err)
	}
	if rows[0].DocumentTenantID != w.operating || rows[0].ShipmentTenantID != w.shipmentTenant || rows[0].DocumentID != done.DocumentID {
		t.Fatalf("cross-shipper pin %+v", rows[0])
	}
	if _, exists := store.companies[w.company.String()+"|"+w.shipmentTenant.String()]; exists {
		t.Fatal("owner company must not be registered in the shipment tenant for this case")
	}
}

func TestPendingUploadIsNotEvidence(t *testing.T) {
	store := newMemActionStore()
	w := newWorld()
	store.seedDelivery(w)
	svc := testActionService(t, store)
	if _, err := svc.CreateUploadIntent(context.Background(), w.input("pending")); err != nil {
		t.Fatal(err)
	}
	rows, err := svc.GetEvidence(context.Background(), w.operating, w.execution, w.action)
	if err != nil || len(rows) != 0 {
		t.Fatalf("pending intent counted as evidence: %+v %v", rows, err)
	}
}

func TestDeletedDocumentIsNotEvidence(t *testing.T) {
	store := newMemActionStore()
	w := newWorld()
	store.seedDelivery(w)
	svc := testActionService(t, store)
	done := completeFlow(t, svc, w.input("gone"))
	store.deleted[done.DocumentID] = struct{}{}
	rows, err := svc.GetEvidence(context.Background(), w.operating, w.execution, w.action)
	if err != nil || len(rows) != 0 {
		t.Fatalf("deleted document still present: %+v %v", rows, err)
	}
}

func TestActionStorageMetadata(t *testing.T) {
	store := newMemActionStore()
	w := newWorld()
	store.seedDelivery(w)
	objects, err := storage.NewLocalObjectStore(filepath.Join(t.TempDir(), "objects"))
	if err != nil {
		t.Fatal(err)
	}
	localSvc := NewPODActionEvidenceService(store, objects, 1024, ActionObjectPlacement{StorageProvider: "local"})
	completeFlow(t, localSvc, w.input("local-meta"))
	if store.provider != "local" || store.bucket != nil {
		t.Fatalf("local metadata provider=%s bucket=%v", store.provider, store.bucket)
	}
	bucket := "edo-pod-evidence"
	s3Svc := NewPODActionEvidenceService(store, objects, 1024, ActionObjectPlacement{StorageProvider: "s3", BucketName: &bucket})
	completeFlow(t, s3Svc, w.input("s3-meta"))
	if store.provider != "s3" || store.bucket == nil || *store.bucket != bucket {
		t.Fatalf("s3 metadata provider=%s bucket=%v", store.provider, store.bucket)
	}
	fromCfg := ActionObjectPlacementFromConfig(storage.Config{Provider: "s3", Bucket: bucket})
	if fromCfg.StorageProvider != "s3" || fromCfg.BucketName == nil || *fromCfg.BucketName != bucket {
		t.Fatalf("config placement %+v", fromCfg)
	}
	if got := ActionObjectPlacementFromConfig(storage.Config{Provider: "local"}); got.StorageProvider != "local" || got.BucketName != nil {
		t.Fatalf("local config placement %+v", got)
	}
}

func TestCreateReplayResumesWithRotatedToken(t *testing.T) {
	store := newMemActionStore()
	w := newWorld()
	store.seedDelivery(w)
	svc := testActionService(t, store)
	ctx := context.Background()
	first, err := svc.CreateUploadIntent(ctx, w.input("resume"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.CreateUploadIntent(ctx, w.input("resume"))
	if err != nil {
		t.Fatal(err)
	}
	if !second.Replayed || second.UploadID != first.UploadID || second.DocumentID != first.DocumentID || second.UploadToken == "" || second.UploadToken == first.UploadToken || store.inserts != 1 {
		t.Fatalf("replay first=%s second=%+v inserts=%d", first.UploadToken, second, store.inserts)
	}
	if err := svc.UploadContent(ctx, second.UploadID, first.UploadToken, bytes.NewReader([]byte("old")), domain.SubjectConfirmation{}); codeOf(err) != apperrors.CodeValidation {
		t.Fatalf("old token: %v", err)
	}
	if err := svc.UploadContent(ctx, second.UploadID, second.UploadToken, bytes.NewReader([]byte("new")), domain.SubjectConfirmation{}); err != nil {
		t.Fatal(err)
	}
	done := completeFlow(t, svc, w.input("resume-done"))
	replay, err := svc.CreateUploadIntent(ctx, w.input("resume-done"))
	if err != nil {
		t.Fatal(err)
	}
	if replay.UploadToken != "" || replay.DocumentID != done.DocumentID || replay.UploadID != done.UploadID || store.inserts != 2 {
		t.Fatalf("completed replay %+v inserts=%d", replay, store.inserts)
	}
}

func TestContentUploadRetryDoesNotOverwrite(t *testing.T) {
	store := newMemActionStore()
	w := newWorld()
	store.seedDelivery(w)
	root := filepath.Join(t.TempDir(), "objects")
	inner, err := storage.NewLocalObjectStore(root)
	if err != nil {
		t.Fatal(err)
	}
	guard := &actionCountingStore{inner: inner}
	svc := NewPODActionEvidenceService(store, guard, 1024, ActionObjectPlacement{StorageProvider: "local"})
	ctx := context.Background()
	intent, err := svc.CreateUploadIntent(ctx, w.input("retry-content"))
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.UploadContent(ctx, intent.UploadID, intent.UploadToken, bytes.NewReader([]byte("original")), domain.SubjectConfirmation{}); err != nil {
		t.Fatal(err)
	}
	if err := svc.UploadContent(ctx, intent.UploadID, intent.UploadToken, bytes.NewReader([]byte("replacement-bytes")), domain.SubjectConfirmation{}); err != nil {
		t.Fatal(err)
	}
	if guard.puts != 1 {
		t.Fatalf("puts=%d", guard.puts)
	}
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(intent.ObjectKey)))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "original" {
		t.Fatalf("object overwritten: %s", body)
	}
}

func TestContentRecoveryWhenObjectExistsAndIntentPending(t *testing.T) {
	store := newMemActionStore()
	w := newWorld()
	store.seedDelivery(w)
	inner, err := storage.NewLocalObjectStore(filepath.Join(t.TempDir(), "objects"))
	if err != nil {
		t.Fatal(err)
	}
	guard := &actionCountingStore{inner: inner}
	svc := NewPODActionEvidenceService(store, guard, 1024, ActionObjectPlacement{StorageProvider: "local"})
	ctx := context.Background()
	intent, err := svc.CreateUploadIntent(ctx, w.input("recover"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := inner.Put(ctx, intent.ObjectKey, bytes.NewReader([]byte("already")), 1024); err != nil {
		t.Fatal(err)
	}
	if err := svc.UploadContent(ctx, intent.UploadID, intent.UploadToken, bytes.NewReader([]byte("ignored")), domain.SubjectConfirmation{}); err != nil {
		t.Fatal(err)
	}
	if guard.puts != 0 {
		t.Fatalf("recovery called Put %d times", guard.puts)
	}
	loaded, err := store.LoadIntent(ctx, intent.UploadID)
	if err != nil || loaded.Status != "uploaded" || loaded.ByteSize != int64(len("already")) {
		t.Fatalf("recovered intent %+v %v", loaded, err)
	}
}

type actionCountingStore struct {
	inner storage.ObjectStore
	puts  int
}

func (c *actionCountingStore) Put(ctx context.Context, objectKey string, reader io.Reader, maxBytes int64) (int64, error) {
	c.puts++
	return c.inner.Put(ctx, objectKey, reader, maxBytes)
}
func (c *actionCountingStore) Get(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	return c.inner.Get(ctx, objectKey)
}
func (c *actionCountingStore) Exists(ctx context.Context, objectKey string) (bool, error) {
	return c.inner.Exists(ctx, objectKey)
}
func (c *actionCountingStore) Metadata(ctx context.Context, objectKey string) (storage.ObjectMetadata, error) {
	return c.inner.Metadata(ctx, objectKey)
}
func (c *actionCountingStore) Delete(ctx context.Context, objectKey string) error {
	return c.inner.Delete(ctx, objectKey)
}
