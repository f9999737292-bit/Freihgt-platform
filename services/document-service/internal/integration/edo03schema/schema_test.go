//go:build integration

package edo03schema

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestI1MigrationRoundTrip(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	allow(t, applyI1(ctx, pool, false))
	allow(t, applyI1(ctx, pool, true))
	allow(t, applyI1(ctx, pool, false))
	var exists bool
	allow(t, pool.QueryRow(ctx, `SELECT to_regclass('documents.document_packages') IS NOT NULL`).Scan(&exists))
	if !exists {
		t.Fatal("package table missing after up-down-up")
	}
}

func TestI1DownNonEmptyDenied(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	allow(t, applyI1(ctx, pool, false))
	tenant := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO documents.document_packages (tenant_id, assembling_company_id)
		VALUES ($1,$2)`, tenant, uuid.New())
	allow(t, err)
	deny(t, applyI1(ctx, pool, true))
	var n int
	allow(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM documents.document_packages`).Scan(&n))
	if n != 1 {
		t.Fatalf("package rows=%d, want 1", n)
	}
}

func TestI1CrossTenantPrecheckFailsClosed(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.New(), uuid.New()
	doc := mustInsertDocument(t, pool, tenantA, "PRECHECK", "DRAFT")
	session := mustInsertSession(t, pool, tenantA, doc)
	sig := mustInsertLegacySignature(t, pool, tenantA, session, doc)
	_, err := pool.Exec(ctx, `UPDATE documents.signatures SET tenant_id = $2 WHERE id = $1`, sig, tenantB)
	allow(t, err)
	deny(t, applyI1(ctx, pool, false))
	var n int
	allow(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = 'documents' AND table_name = 'signatures' AND column_name = 'document_version_id'
	`).Scan(&n))
	if n != 0 {
		t.Fatal("precheck failure still added document_version_id")
	}
}

func TestI1LegacySignaturesStayUnbound(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	dir, err := migrationsDir()
	allow(t, err)
	source, err := os.ReadFile(filepath.Join(dir, i1UpFile))
	allow(t, err)
	upper := strings.ToUpper(string(source))
	if strings.Contains(upper, "UPDATE DOCUMENTS.SIGNATURES") {
		t.Fatal("migration updates documents.signatures")
	}
	tenant := uuid.New()
	doc := mustInsertDocument(t, pool, tenant, "LEGACY", "SIGNED")
	_ = mustInsertVersion(t, pool, doc, 1)
	session := mustInsertSession(t, pool, tenant, doc)
	sig := mustInsertLegacySignature(t, pool, tenant, session, doc)
	allow(t, applyI1(ctx, pool, false))
	var versionID, algorithm, value *string
	allow(t, pool.QueryRow(ctx, `
		SELECT document_version_id::text, content_digest_algorithm, content_digest_value
		FROM documents.signatures WHERE id = $1`, sig).Scan(&versionID, &algorithm, &value))
	if versionID != nil || algorithm != nil || value != nil {
		t.Fatalf("legacy binding inferred: version=%v algorithm=%v value=%v", versionID, algorithm, value)
	}
}

func TestI1PackageAndRelationshipInvariants(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	allow(t, applyI1(ctx, pool, false))
	tenantA, tenantB := uuid.New(), uuid.New()
	docA := mustInsertDocument(t, pool, tenantA, "A", "DRAFT")
	docA2 := mustInsertDocument(t, pool, tenantA, "A2", "DRAFT")
	docB := mustInsertDocument(t, pool, tenantB, "B", "DRAFT")
	var packageID uuid.UUID
	allow(t, pool.QueryRow(ctx, `
		INSERT INTO documents.document_packages (tenant_id, assembling_company_id)
		VALUES ($1,$2) RETURNING id`, tenantA, uuid.New()).Scan(&packageID))
	_, err := pool.Exec(ctx, `
		INSERT INTO documents.document_package_members (package_id, document_id, tenant_id)
		VALUES ($1,$2,$3)`, packageID, docA, tenantA)
	allow(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO documents.document_package_members (package_id, document_id, tenant_id)
		VALUES ($1,$2,$3)`, packageID, docA, tenantA)
	deny(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO documents.document_package_members (package_id, document_id, tenant_id)
		VALUES ($1,$2,$3)`, packageID, docB, tenantA)
	deny(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO documents.document_package_members (package_id, document_id, tenant_id)
		VALUES ($1,$2,$3)`, packageID, docB, tenantB)
	deny(t, err)

	_, err = pool.Exec(ctx, `
		UPDATE documents.document_packages
		SET status = 'SEALED', sealed_at = now(), updated_at = now()
		WHERE id = $1`, packageID)
	allow(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO documents.document_package_members (package_id, document_id, tenant_id)
		VALUES ($1,$2,$3)`, packageID, docA2, tenantA)
	deny(t, err)
	_, err = pool.Exec(ctx, `
		UPDATE documents.document_package_members SET created_by = $2 WHERE package_id = $1`, packageID, uuid.New())
	deny(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM documents.document_package_members WHERE package_id = $1`, packageID)
	deny(t, err)

	var openPackageID uuid.UUID
	allow(t, pool.QueryRow(ctx, `
		INSERT INTO documents.document_packages (tenant_id, assembling_company_id)
		VALUES ($1,$2) RETURNING id`, tenantA, uuid.New()).Scan(&openPackageID))
	_, err = pool.Exec(ctx, `
		INSERT INTO documents.document_package_members (package_id, document_id, tenant_id)
		VALUES ($1,$2,$3)`, openPackageID, docA2, tenantA)
	allow(t, err)
	_, err = pool.Exec(ctx, `
		UPDATE documents.document_package_members
		SET package_id = $2
		WHERE package_id = $1 AND document_id = $3`, openPackageID, packageID, docA2)
	deny(t, err)
	_, err = pool.Exec(ctx, `
		UPDATE documents.document_package_members
		SET package_id = $2
		WHERE package_id = $1 AND document_id = $3`, packageID, openPackageID, docA)
	deny(t, err)

	for _, relType := range []string{"CORRECTS", "REPLACES", "RELATED_TO"} {
		_, err = pool.Exec(ctx, `
			INSERT INTO documents.document_relationships (
				tenant_id, source_document_id, target_document_id, relationship_type
			) VALUES ($1,$2,$3,$4)`, tenantA, docA, docA2, relType)
		allow(t, err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO documents.document_relationships (
			tenant_id, source_document_id, target_document_id, relationship_type
		) VALUES ($1,$2,$3,'PACKAGE_CONTAINS_DOCUMENT')`, tenantA, docA, docA2)
	deny(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO documents.document_relationships (
			tenant_id, source_document_id, target_document_id, relationship_type
		) VALUES ($1,$2,$2,'RELATED_TO')`, tenantA, docA)
	deny(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO documents.document_relationships (
			tenant_id, source_document_id, target_document_id, relationship_type
		) VALUES ($1,$2,$3,'RELATED_TO')`, tenantA, docA, docB)
	deny(t, err)
	_, err = pool.Exec(ctx, `
		UPDATE documents.document_relationships SET relationship_type = 'CORRECTS'
		WHERE tenant_id = $1`, tenantA)
	deny(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM documents.document_relationships WHERE tenant_id = $1`, tenantA)
	deny(t, err)
}

func TestI1SignatureBindingAndEvidence(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	allow(t, applyI1(ctx, pool, false))
	tenantA, tenantB := uuid.New(), uuid.New()
	docA := mustInsertDocument(t, pool, tenantA, "SIG-A", "DRAFT")
	docB := mustInsertDocument(t, pool, tenantB, "SIG-B", "DRAFT")
	versionA := mustInsertVersion(t, pool, docA, 1)
	versionA2 := mustInsertVersion(t, pool, docA, 2)
	versionB := mustInsertVersion(t, pool, docB, 1)
	sessionA := mustInsertSession(t, pool, tenantA, docA)
	sessionB := mustInsertSession(t, pool, tenantB, docB)
	legacy := mustInsertSignature(t, pool, tenantA, sessionA, docA, nil)
	bound := mustInsertSignature(t, pool, tenantA, sessionA, docA, &versionA)
	other := mustInsertSignature(t, pool, tenantB, sessionB, docB, nil)

	_, err := pool.Exec(ctx, `
		UPDATE documents.signatures SET document_version_id = $2 WHERE id = $1`, legacy, versionA)
	deny(t, err)
	_, err = pool.Exec(ctx, `
		UPDATE documents.signatures
		SET document_version_id = $2, content_digest_algorithm = 'SHA-512', content_digest_value = $3
		WHERE id = $1`, other, versionB, digestHex)
	deny(t, err)
	_, err = pool.Exec(ctx, `
		UPDATE documents.signatures
		SET document_version_id = $2, content_digest_algorithm = 'SHA-256', content_digest_value = 'ABC'
		WHERE id = $1`, other, versionB)
	deny(t, err)
	_, err = pool.Exec(ctx, `
		UPDATE documents.signatures
		SET document_version_id = $2, content_digest_algorithm = 'SHA-256', content_digest_value = $3
		WHERE id = $1`, other, versionA, strings.ToUpper(digestHex))
	deny(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO documents.signatures (
			tenant_id, signing_session_id, document_id, signature_type,
			document_version_id, content_digest_algorithm, content_digest_value
		) VALUES ($1,$2,$3,'SIMPLE_ELECTRONIC',$4,'SHA-256',$5)`,
		tenantA, sessionA, docA, versionB, digestHex)
	deny(t, err)

	_, err = pool.Exec(ctx, `
		UPDATE documents.signatures
		SET content_digest_value = $2 WHERE id = $1`, bound, strings.Repeat("a", 64))
	deny(t, err)

	var copied int
	allow(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM documents.signatures WHERE document_version_id = $1`, versionA2).Scan(&copied))
	if copied != 0 {
		t.Fatal("new revision inherited a signature")
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO documents.certificate_evidence (
			tenant_id, signature_id, document_version_id, certificate_fingerprint
		) VALUES ($1,$2,$3,'fingerprint')`, tenantA, bound, versionA)
	allow(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO documents.certificate_evidence (
			tenant_id, signature_id, document_version_id, certificate_fingerprint
		) VALUES ($1,$2,$3,'fingerprint')`, tenantA, legacy, versionA)
	deny(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO documents.certificate_evidence (
			tenant_id, signature_id, document_version_id, certificate_fingerprint
		) VALUES ($1,$2,$3,'fingerprint')`, tenantB, bound, versionA)
	deny(t, err)
	_, err = pool.Exec(ctx, `UPDATE documents.certificate_evidence SET certificate_fingerprint = 'changed'`)
	deny(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM documents.certificate_evidence`)
	deny(t, err)

	var secretCols int
	allow(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = 'documents'
		  AND table_name IN (
		    'document_packages','document_package_members','document_relationships',
		    'certificate_evidence','signature_verification_records','signatures'
		  )
		  AND column_name ~* 'private_key|secret|credential|password|pem'`).Scan(&secretCols))
	if secretCols != 0 {
		t.Fatalf("private-key-like columns=%d", secretCols)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO documents.signature_verification_records (
			tenant_id, signature_id, document_version_id, verification_status
		) VALUES ($1,$2,$3,'PENDING')`, tenantA, bound, versionA)
	allow(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO documents.signature_verification_records (
			tenant_id, signature_id, document_version_id, verification_status
		) VALUES ($1,$2,$3,'VALID')`, tenantA, bound, versionA)
	allow(t, err)
	_, err = pool.Exec(ctx, `UPDATE documents.signature_verification_records SET verification_status = 'FAILED'`)
	deny(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM documents.signature_verification_records`)
	deny(t, err)
	var digest string
	var fingerprint *string
	allow(t, pool.QueryRow(ctx, `
		SELECT content_digest_value, certificate_fingerprint FROM documents.signatures WHERE id = $1`, bound).
		Scan(&digest, &fingerprint))
	if digest != digestHex || fingerprint == nil || *fingerprint != "fp-test" {
		t.Fatalf("bound evidence rewritten digest=%s fingerprint=%v", digest, fingerprint)
	}
}

func TestI1SignedArtifactProtection(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	allow(t, applyI1(ctx, pool, false))

	draft := mustInsertDocument(t, pool, uuid.New(), "DRAFT-DEL", "DRAFT")
	draftVersion := mustInsertVersion(t, pool, draft, 1)
	_, err := pool.Exec(ctx, `
		INSERT INTO documents.document_files (document_id, document_version_id, file_type, object_key)
		VALUES ($1,$2,'PDF','draft-key')`, draft, draftVersion)
	allow(t, err)
	draftSession := mustInsertSession(t, pool, mustTenant(t, pool, draft), draft)
	_ = mustInsertSignature(t, pool, mustTenant(t, pool, draft), draftSession, draft, nil)
	_, err = pool.Exec(ctx, `DELETE FROM documents.documents WHERE id = $1`, draft)
	allow(t, err)

	soft := mustInsertDocument(t, pool, uuid.New(), "SOFT", "DRAFT")
	_, err = pool.Exec(ctx, `UPDATE documents.documents SET deleted_at = now() WHERE id = $1`, soft)
	allow(t, err)

	for _, status := range []string{"SIGNED", "SENT_TO_OPERATOR", "ACCEPTED", "ARCHIVED"} {
		protectSignedStatus(t, pool, status)
	}
}

func protectSignedStatus(t *testing.T, pool *pgxpool.Pool, status string) {
	t.Helper()
	ctx := context.Background()
	tenant := uuid.New()
	doc := mustInsertDocument(t, pool, tenant, "SIGNED-"+status, status)
	version := mustInsertVersion(t, pool, doc, 1)
	_, err := pool.Exec(ctx, `
		INSERT INTO documents.document_files (document_id, document_version_id, file_type, object_key)
		VALUES ($1,$2,'PDF',$3)`, doc, version, "key-"+status)
	allow(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO documents.document_files (document_id, document_version_id, file_type, object_key)
		VALUES ($1,$2,'XML',$3)`, doc, version, "attach-"+status)
	allow(t, err)
	session := mustInsertSession(t, pool, tenant, doc)
	_ = mustInsertSignature(t, pool, tenant, session, doc, &version)
	_, err = pool.Exec(ctx, `UPDATE documents.document_versions SET payload_json = '{"changed":true}' WHERE id = $1`, version)
	deny(t, err)
	_, err = pool.Exec(ctx, `UPDATE documents.document_versions SET payload_xml_path = 'changed.xml' WHERE id = $1`, version)
	deny(t, err)
	_, err = pool.Exec(ctx, `UPDATE documents.document_versions SET pdf_file_path = 'changed.pdf' WHERE id = $1`, version)
	deny(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM documents.document_files WHERE document_id = $1`, doc)
	deny(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM documents.document_versions WHERE id = $1`, version)
	deny(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM documents.signing_sessions WHERE id = $1`, session)
	deny(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM documents.signatures WHERE document_id = $1`, doc)
	deny(t, err)
	_, err = pool.Exec(ctx, `UPDATE documents.documents SET deleted_at = now() WHERE id = $1`, doc)
	deny(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM documents.documents WHERE id = $1`, doc)
	deny(t, err)
}

func mustTenant(t *testing.T, pool *pgxpool.Pool, documentID uuid.UUID) uuid.UUID {
	t.Helper()
	var tenant uuid.UUID
	if err := pool.QueryRow(context.Background(), `SELECT tenant_id FROM documents.documents WHERE id = $1`, documentID).Scan(&tenant); err != nil {
		t.Fatal(err)
	}
	return tenant
}
