DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM documents.attachment_signature_blobs) THEN
        RAISE EXCEPTION 'attachment_signature_blobs is not empty';
    END IF;
    IF EXISTS (SELECT 1 FROM documents.signature_verification_evidence) THEN
        RAISE EXCEPTION 'signature_verification_evidence is not empty';
    END IF;
END $$;

DROP TRIGGER IF EXISTS trg_edo_i4b_evidence_append ON documents.signature_verification_evidence;
DROP TRIGGER IF EXISTS trg_edo_i4b_blob_immutable ON documents.attachment_signature_blobs;

DROP TABLE IF EXISTS documents.signature_verification_evidence;
DROP TABLE IF EXISTS documents.attachment_signature_blobs;

DROP FUNCTION IF EXISTS documents.edo_i4b_reject_evidence_rewrite();
DROP FUNCTION IF EXISTS documents.edo_i4b_reject_blob_rewrite();
