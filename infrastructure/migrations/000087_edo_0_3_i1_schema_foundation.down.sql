-- Down is allowed only while I1 objects hold no business or evidence rows.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM documents.signatures
        WHERE document_version_id IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'EDO-0.3 I1 down refused: signature revision bindings exist';
    END IF;

    IF EXISTS (SELECT 1 FROM documents.certificate_evidence) THEN
        RAISE EXCEPTION 'EDO-0.3 I1 down refused: certificate_evidence is not empty';
    END IF;

    IF EXISTS (SELECT 1 FROM documents.signature_verification_records) THEN
        RAISE EXCEPTION 'EDO-0.3 I1 down refused: signature_verification_records is not empty';
    END IF;

    IF EXISTS (SELECT 1 FROM documents.document_package_members) THEN
        RAISE EXCEPTION 'EDO-0.3 I1 down refused: document_package_members is not empty';
    END IF;

    IF EXISTS (SELECT 1 FROM documents.document_relationships) THEN
        RAISE EXCEPTION 'EDO-0.3 I1 down refused: document_relationships is not empty';
    END IF;

    IF EXISTS (SELECT 1 FROM documents.document_packages) THEN
        RAISE EXCEPTION 'EDO-0.3 I1 down refused: document_packages is not empty';
    END IF;
END $$;

DROP TRIGGER IF EXISTS trg_edo_i1_verification_insert ON documents.signature_verification_records;
DROP TRIGGER IF EXISTS trg_edo_i1_verification_append ON documents.signature_verification_records;
DROP TRIGGER IF EXISTS trg_edo_i1_certificate_append ON documents.certificate_evidence;
DROP TRIGGER IF EXISTS trg_edo_i1_relationships_append ON documents.document_relationships;
DROP TRIGGER IF EXISTS trg_edo_i1_members_seal ON documents.document_package_members;
DROP TRIGGER IF EXISTS trg_edo_i1_signatures_binding ON documents.signatures;
DROP TRIGGER IF EXISTS trg_edo_i1_signatures_delete ON documents.signatures;
DROP TRIGGER IF EXISTS trg_edo_i1_sessions_delete ON documents.signing_sessions;
DROP TRIGGER IF EXISTS trg_edo_i1_files_delete ON documents.document_files;
DROP TRIGGER IF EXISTS trg_edo_i1_versions_payload ON documents.document_versions;
DROP TRIGGER IF EXISTS trg_edo_i1_versions_delete ON documents.document_versions;
DROP TRIGGER IF EXISTS trg_edo_i1_documents_soft_delete ON documents.documents;
DROP TRIGGER IF EXISTS trg_edo_i1_documents_delete ON documents.documents;

DROP TABLE IF EXISTS documents.signature_verification_records;
DROP TABLE IF EXISTS documents.certificate_evidence;
DROP TABLE IF EXISTS documents.document_relationships;
DROP TABLE IF EXISTS documents.document_package_members;
DROP TABLE IF EXISTS documents.document_packages;

DROP FUNCTION IF EXISTS documents.edo_i1_verification_matches_binding();
DROP FUNCTION IF EXISTS documents.edo_i1_reject_verification_mutation();
DROP FUNCTION IF EXISTS documents.edo_i1_reject_certificate_mutation();
DROP FUNCTION IF EXISTS documents.edo_i1_reject_relationship_mutation();
DROP FUNCTION IF EXISTS documents.edo_i1_reject_sealed_membership();
DROP FUNCTION IF EXISTS documents.edo_i1_reject_bound_signature_rewrite();
DROP FUNCTION IF EXISTS documents.edo_i1_reject_signed_signature_delete();
DROP FUNCTION IF EXISTS documents.edo_i1_reject_signed_session_delete();
DROP FUNCTION IF EXISTS documents.edo_i1_reject_signed_file_delete();
DROP FUNCTION IF EXISTS documents.edo_i1_reject_signed_payload_update();
DROP FUNCTION IF EXISTS documents.edo_i1_reject_signed_version_delete();
DROP FUNCTION IF EXISTS documents.edo_i1_reject_signed_document_soft_delete();
DROP FUNCTION IF EXISTS documents.edo_i1_reject_signed_document_delete();
DROP FUNCTION IF EXISTS documents.edo_i1_document_is_signed_class(UUID);

ALTER TABLE documents.signing_sessions
    DROP CONSTRAINT IF EXISTS fk_signing_sessions_document_tenant;

ALTER TABLE documents.signatures
    DROP CONSTRAINT IF EXISTS fk_signatures_document_version,
    DROP CONSTRAINT IF EXISTS fk_signatures_session_tenant,
    DROP CONSTRAINT IF EXISTS fk_signatures_document_tenant,
    DROP CONSTRAINT IF EXISTS uq_signatures_id_version,
    DROP CONSTRAINT IF EXISTS chk_signatures_digest_value,
    DROP CONSTRAINT IF EXISTS chk_signatures_digest_algorithm,
    DROP CONSTRAINT IF EXISTS chk_signatures_revision_binding;

ALTER TABLE documents.signatures
    DROP COLUMN IF EXISTS content_digest_value,
    DROP COLUMN IF EXISTS content_digest_algorithm,
    DROP COLUMN IF EXISTS document_version_id;

ALTER TABLE documents.document_versions
    DROP CONSTRAINT IF EXISTS uq_document_versions_document_id_id;

ALTER TABLE documents.signatures
    DROP CONSTRAINT IF EXISTS uq_signatures_id_tenant;

ALTER TABLE documents.signing_sessions
    DROP CONSTRAINT IF EXISTS uq_signing_sessions_id_tenant;

ALTER TABLE documents.documents
    DROP CONSTRAINT IF EXISTS uq_documents_id_tenant;
