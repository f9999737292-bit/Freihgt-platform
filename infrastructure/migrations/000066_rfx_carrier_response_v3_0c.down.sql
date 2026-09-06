DROP TABLE IF EXISTS rfx.rfx_answers;

ALTER TABLE rfx.rfx_responses
    DROP COLUMN IF EXISTS completion_percent,
    DROP COLUMN IF EXISTS last_saved_by,
    DROP COLUMN IF EXISTS last_saved_at,
    DROP COLUMN IF EXISTS save_version,
    DROP COLUMN IF EXISTS rfx_version_id;
