DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM documents.document_attachments) THEN
        RAISE EXCEPTION 'document_attachments is not empty';
    END IF;
    IF EXISTS (SELECT 1 FROM documents.attachment_signatures) THEN
        RAISE EXCEPTION 'attachment_signatures is not empty';
    END IF;
    IF EXISTS (SELECT 1 FROM documents.signature_verification_history) THEN
        RAISE EXCEPTION 'signature_verification_history is not empty';
    END IF;
    IF EXISTS (SELECT 1 FROM documents.attachment_audit_events) THEN
        RAISE EXCEPTION 'attachment_audit_events is not empty';
    END IF;
END $$;

DROP TRIGGER IF EXISTS trg_edo_i2_signature_immutable ON documents.attachment_signatures;
DROP TRIGGER IF EXISTS trg_edo_i2_attachment_immutable ON documents.document_attachments;
DROP TRIGGER IF EXISTS trg_edo_i2_history_append ON documents.signature_verification_history;
DROP TRIGGER IF EXISTS trg_edo_i2_history_unverified ON documents.signature_verification_history;
DROP TRIGGER IF EXISTS trg_edo_i2_signatures_unverified ON documents.attachment_signatures;

DROP TABLE IF EXISTS documents.attachment_audit_events;
DROP TABLE IF EXISTS documents.signature_verification_history;
DROP TABLE IF EXISTS documents.attachment_signatures;
DROP TABLE IF EXISTS documents.document_attachments;

DROP FUNCTION IF EXISTS documents.edo_i2_reject_signature_rewrite();
DROP FUNCTION IF EXISTS documents.edo_i2_reject_finalized_attachment_change();
DROP FUNCTION IF EXISTS documents.edo_i2_reject_history_rewrite();
DROP FUNCTION IF EXISTS documents.edo_i2_reject_unverified_claim();
