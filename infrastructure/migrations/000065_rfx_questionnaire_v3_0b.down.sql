ALTER TABLE rfx.rfx_events DROP COLUMN IF EXISTS draft_version_id;

DROP TABLE IF EXISTS rfx.rfx_question_rules;
DROP TABLE IF EXISTS rfx.rfx_question_options;
DROP TABLE IF EXISTS rfx.rfx_questions;
DROP TABLE IF EXISTS rfx.rfx_sections;
DROP TABLE IF EXISTS rfx.rfx_versions;
