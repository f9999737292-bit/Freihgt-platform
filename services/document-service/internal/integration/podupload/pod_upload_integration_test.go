//go:build integration

package podupload

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/service"
)

func TestPODUploadFullFlow(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedFixtures(t, env.pool)
	ctx := context.Background()

	intent, err := env.pod.CreateUploadIntent(ctx, createInput(fix, fix.ShipmentA, fix.DriverA, "pod-1"))
	if err != nil {
		t.Fatalf("create intent: %v", err)
	}
	if intent.UploadToken == "" {
		t.Fatal("expected upload token")
	}
	if !strings.Contains(intent.ObjectKey, fix.TenantA.String()) || !strings.Contains(intent.ObjectKey, fix.ShipmentA.String()) {
		t.Fatalf("server-controlled object key expected tenant/shipment prefix, got %q", intent.ObjectKey)
	}

	body := []byte("fake-jpeg-content")
	if err := env.pod.UploadContent(ctx, fix.TenantA, intent.UploadID, intent.UploadToken, bytes.NewReader(body)); err != nil {
		t.Fatalf("upload content: %v", err)
	}
	result, err := env.pod.CompleteUpload(ctx, service.CompletePODUploadInput{
		TenantID: fix.TenantA, UploadID: intent.UploadID, DriverID: fix.DriverA,
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("expected completed, got %q", result.Status)
	}

	var docType, relatedType string
	var relatedID uuid.UUID
	err = env.pool.QueryRow(ctx, `
		SELECT document_type, related_entity_type, related_entity_id
		FROM documents.documents WHERE id=$1 AND tenant_id=$2`,
		result.DocumentID, fix.TenantA).Scan(&docType, &relatedType, &relatedID)
	if err != nil {
		t.Fatalf("document row: %v", err)
	}
	if docType != "POD" || relatedType != "SHIPMENT" || relatedID != fix.ShipmentA {
		t.Fatalf("unexpected document linkage: type=%s related=%s/%s", docType, relatedType, relatedID)
	}

	result2, err := env.pod.CompleteUpload(ctx, service.CompletePODUploadInput{
		TenantID: fix.TenantA, UploadID: intent.UploadID, DriverID: fix.DriverA,
	})
	if err != nil {
		t.Fatalf("repeat complete: %v", err)
	}
	if result2.DocumentID != result.DocumentID {
		t.Fatal("repeat finalize changed document id")
	}
	var fileCount int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM documents.document_files WHERE document_id=$1`, result.DocumentID).Scan(&fileCount); err != nil {
		t.Fatal(err)
	}
	if fileCount != 1 {
		t.Fatalf("expected 1 file after idempotent finalize, got %d", fileCount)
	}
}

func TestPODUploadIntentIdempotency(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedFixtures(t, env.pool)
	ctx := context.Background()
	in := createInput(fix, fix.ShipmentA, fix.DriverA, "idem-pod")
	first, err := env.pod.CreateUploadIntent(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := env.pod.CreateUploadIntent(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if first.UploadID != second.UploadID || first.DocumentID != second.DocumentID {
		t.Fatal("idempotent create should return same upload/document ids")
	}
	var count int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM documents.document_upload_intent WHERE tenant_id=$1 AND driver_id=$2 AND idempotency_key=$3`,
		fix.TenantA, fix.DriverA, "idem-pod").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 upload intent row, got %d", count)
	}
}

func TestPODWrongDriverDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedFixtures(t, env.pool)
	ctx := context.Background()
	intent, err := env.pod.CreateUploadIntent(ctx, createInput(fix, fix.ShipmentA, fix.DriverA, ""))
	if err != nil {
		t.Fatal(err)
	}
	if err := env.pod.UploadContent(ctx, fix.TenantA, intent.UploadID, intent.UploadToken, bytes.NewReader([]byte("x"))); err != nil {
		t.Fatal(err)
	}
	_, err = env.pod.CompleteUpload(ctx, service.CompletePODUploadInput{
		TenantID: fix.TenantA, UploadID: intent.UploadID, DriverID: fix.DriverB,
	})
	if err == nil {
		t.Fatal("expected wrong driver finalize to fail")
	}
}

func TestPODCrossTenantShipmentRejected(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedFixtures(t, env.pool)
	ctx := context.Background()
	_, err := env.pod.CreateUploadIntent(ctx, createInput(fix, fix.ShipmentB, fix.DriverA, ""))
	if err == nil {
		t.Fatal("expected cross-tenant shipment linkage to fail")
	}
}

func TestPODInvalidMimeRejected(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedFixtures(t, env.pool)
	ctx := context.Background()
	in := createInput(fix, fix.ShipmentA, fix.DriverA, "")
	in.MimeType = "application/x-msdownload"
	_, err := env.pod.CreateUploadIntent(ctx, in)
	if err == nil {
		t.Fatal("expected invalid mime rejection")
	}
}

func TestPODInvalidUploadTokenRejected(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedFixtures(t, env.pool)
	ctx := context.Background()
	intent, err := env.pod.CreateUploadIntent(ctx, createInput(fix, fix.ShipmentA, fix.DriverA, ""))
	if err != nil {
		t.Fatal(err)
	}
	if err := env.pod.UploadContent(ctx, fix.TenantA, intent.UploadID, "bad-token", bytes.NewReader([]byte("x"))); err == nil {
		t.Fatal("expected invalid token rejection")
	}
}

func createInput(fix fixture, shipmentID, driverID uuid.UUID, idem string) service.CreatePODUploadInput {
	return service.CreatePODUploadInput{
		TenantID: fix.TenantA, ShipmentID: shipmentID, DriverID: driverID,
		OwnerCompanyID: fix.CompanyA, MimeType: "image/jpeg", FileName: "pod.jpg",
		IdempotencyKey: idem,
	}
}
