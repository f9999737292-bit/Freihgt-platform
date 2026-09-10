//go:build integration

package templatelibrary

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

// PROVENANCE_ARCHIVED_TEMPLATE_SEMANTICS=PASS
func TestE6PG01ArchivedTemplateProvenanceSemantics(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail, published, _ := setupPublishedE6HistoricalTemplate(t, env, fix, "e6-pg-01", &fix.CompanyA)

	cloneResult := cloneFromTemplate(t, env, fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E6-PG-01", fix.CompanyA), uuid.NewString())

	baselineProv, err := env.rfxSvc.GetEventProvenance(context.Background(), fix.BuyerA, cloneResult.Event.ID)
	if err != nil {
		t.Fatalf("baseline provenance: %v", err)
	}
	if baselineProv == nil {
		t.Fatal("expected baseline provenance before archive")
	}
	assertProvenanceExact(t, baselineProv, published, detail.Template.TemplateCode, false)

	archived, err := env.templateSvc.ArchiveTemplate(context.Background(), fix.BuyerA, detail.Template.ID)
	if err != nil {
		t.Fatalf("archive template: %v", err)
	}
	if archived.Status != domain.RfxTemplateStatusArchived {
		t.Fatalf("expected archived template status, got %s", archived.Status)
	}

	afterArchiveProv, err := env.rfxSvc.GetEventProvenance(context.Background(), fix.BuyerA, cloneResult.Event.ID)
	if err != nil {
		t.Fatalf("PG provenance after archive: %v", err)
	}
	if afterArchiveProv == nil {
		t.Fatal("authorized buyer must retain provenance after template archive")
	}
	if afterArchiveProv.SourceTemplateID != baselineProv.SourceTemplateID {
		t.Fatalf("source_template_id changed after archive: %s -> %s", baselineProv.SourceTemplateID, afterArchiveProv.SourceTemplateID)
	}
	if afterArchiveProv.SourceTemplateVersionID != baselineProv.SourceTemplateVersionID {
		t.Fatalf("source_template_version_id changed after archive: %s -> %s", baselineProv.SourceTemplateVersionID, afterArchiveProv.SourceTemplateVersionID)
	}
	if afterArchiveProv.SourceVersionNumber != baselineProv.SourceVersionNumber {
		t.Fatalf("source_version_number changed after archive: %d -> %d", baselineProv.SourceVersionNumber, afterArchiveProv.SourceVersionNumber)
	}

	buyerRec := getEventHTTP(t, env, e6EnabledConfig(), fix, cloneResult.Event.ID)
	assertHTTPEventProvenanceExact(t, buyerRec, published, detail.Template.TemplateCode, false)

	carrierRec := getEventHTTPWithActor(t, env, e6EnabledConfig(), fix.CarrierAct, cloneResult.Event.ID)
	expectHTTPErrorCode(t, carrierRec, http.StatusNotFound, apperrors.CodeNotFound)

	otherCompanyRec := getEventHTTPWithActor(t, env, e6EnabledConfig(), fix.BuyerB, cloneResult.Event.ID)
	expectHTTPErrorCode(t, otherCompanyRec, http.StatusNotFound, apperrors.CodeNotFound)
}

// PROVENANCE_UUID_IMMUTABLE_AFTER_EVENT_UPDATE=PASS
func TestE6PG02EventUpdateProvenanceImmutability(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail, published, _ := setupPublishedE6HistoricalTemplate(t, env, fix, "e6-pg-02", &fix.CompanyA)
	cloneResult := cloneFromTemplate(t, env, fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E6-PG-02", fix.CompanyA), uuid.NewString())

	baselineProv, err := env.rfxSvc.GetEventProvenance(context.Background(), fix.BuyerA, cloneResult.Event.ID)
	if err != nil {
		t.Fatalf("baseline provenance: %v", err)
	}
	if baselineProv == nil {
		t.Fatal("expected baseline provenance")
	}

	newTitle := "Updated title after clone"
	newDescription := "Updated description preserves provenance"
	updated, err := env.rfxSvc.UpdateEvent(context.Background(), fix.BuyerA, cloneResult.Event.ID, domain.UpdateRfxEventInput{
		Title:       &newTitle,
		Description: &newDescription,
	})
	if err != nil {
		t.Fatalf("update event metadata: %v", err)
	}
	if updated.Title != newTitle {
		t.Fatalf("title: got %q want %q", updated.Title, newTitle)
	}
	if updated.Description == nil || *updated.Description != newDescription {
		t.Fatalf("description: got %#v want %q", updated.Description, newDescription)
	}

	afterUpdateProv, err := env.rfxSvc.GetEventProvenance(context.Background(), fix.BuyerA, cloneResult.Event.ID)
	if err != nil {
		t.Fatalf("PG provenance after update: %v", err)
	}
	if afterUpdateProv == nil {
		t.Fatal("expected provenance after metadata update")
	}
	assertProvenanceExact(t, afterUpdateProv, published, detail.Template.TemplateCode, false)
	if afterUpdateProv.SourceTemplateID != baselineProv.SourceTemplateID ||
		afterUpdateProv.SourceTemplateVersionID != baselineProv.SourceTemplateVersionID ||
		afterUpdateProv.SourceVersionNumber != baselineProv.SourceVersionNumber {
		t.Fatalf("provenance changed after metadata update: before=%+v after=%+v", baselineProv, afterUpdateProv)
	}

	rec := getEventHTTP(t, env, e6EnabledConfig(), fix, cloneResult.Event.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP event GET: %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode event response: %v", err)
	}
	if got, _ := payload["title"].(string); got != newTitle {
		t.Fatalf("HTTP title: got %q want %q", got, newTitle)
	}
	if got, _ := payload["description"].(string); got != newDescription {
		t.Fatalf("HTTP description: got %q want %q", got, newDescription)
	}
	assertHTTPEventProvenanceExact(t, rec, published, detail.Template.TemplateCode, false)
}

// PROVENANCE_DB_GUARD=PASS PROVENANCE_NULLIFICATION_DENIED=PASS
func TestE6PG03DirectDBProvenanceGuardReplaceAndNullDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e6-pg-03", &fix.CompanyA)
	result := cloneFromTemplate(t, env, fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E6-PG-03", fix.CompanyA), uuid.NewString())

	originalID, err := env.rfxRepo.GetSourceTemplateVersionID(context.Background(), result.Event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("load original provenance pointer: %v", err)
	}
	if originalID == nil || *originalID != published.ID {
		t.Fatalf("expected original source_template_version_id=%s got %#v", published.ID, originalID)
	}

	const immutableMsg = "source_template_version_id is immutable"
	otherVersionID := uuid.New()

	t.Run("replace denied", func(t *testing.T) {
		ctx := context.Background()
		tx, err := env.pool.Begin(ctx)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		_, err = tx.Exec(ctx, `UPDATE rfx.rfx_events SET source_template_version_id=$3 WHERE id=$1 AND tenant_id=$2`,
			result.Event.ID, fix.TenantID, otherVersionID)
		if err == nil {
			t.Fatal("expected replace mutation to be rejected by provenance trigger")
		}
		if !strings.Contains(err.Error(), immutableMsg) {
			t.Fatalf("expected trigger error %q, got %v", immutableMsg, err)
		}
		if err := tx.Rollback(ctx); err != nil {
			t.Fatalf("rollback replace attempt: %v", err)
		}
	})

	t.Run("nullify denied", func(t *testing.T) {
		ctx := context.Background()
		tx, err := env.pool.Begin(ctx)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		_, err = tx.Exec(ctx, `UPDATE rfx.rfx_events SET source_template_version_id=NULL WHERE id=$1 AND tenant_id=$2`,
			result.Event.ID, fix.TenantID)
		if err == nil {
			t.Fatal("expected nullification to be rejected by provenance trigger")
		}
		if !strings.Contains(err.Error(), immutableMsg) {
			t.Fatalf("expected trigger error %q, got %v", immutableMsg, err)
		}
		if err := tx.Rollback(ctx); err != nil {
			t.Fatalf("rollback nullify attempt: %v", err)
		}
	})

	storedID, err := env.rfxRepo.GetSourceTemplateVersionID(context.Background(), result.Event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload provenance pointer after rollback: %v", err)
	}
	if storedID == nil || *storedID != *originalID {
		t.Fatalf("original UUID not preserved after rollback: got %#v want %s", storedID, *originalID)
	}
}
