ALTER TABLE rfx.rfx_template_question_rules
    DROP CONSTRAINT IF EXISTS fk_rfx_template_question_rules_target_question;

ALTER TABLE rfx.rfx_template_question_rules
    DROP CONSTRAINT IF EXISTS fk_rfx_template_question_rules_version_composite;

DROP INDEX IF EXISTS rfx.uq_rfx_template_question_rules_version_code;
DROP TABLE IF EXISTS rfx.rfx_template_question_rules;

ALTER TABLE rfx.rfx_template_question_options
    DROP CONSTRAINT IF EXISTS fk_rfx_template_question_options_question;

DROP INDEX IF EXISTS rfx.uq_rfx_template_question_options_question_code;
DROP TABLE IF EXISTS rfx.rfx_template_question_options;

ALTER TABLE rfx.rfx_template_questions
    DROP CONSTRAINT IF EXISTS fk_rfx_template_questions_section;

DROP INDEX IF EXISTS rfx.uq_rfx_template_questions_section_code;
DROP TABLE IF EXISTS rfx.rfx_template_questions;

ALTER TABLE rfx.rfx_template_sections
    DROP CONSTRAINT IF EXISTS fk_rfx_template_sections_version_composite;

DROP INDEX IF EXISTS rfx.uq_rfx_template_sections_version_code;
DROP INDEX IF EXISTS rfx.idx_rfx_template_sections_tenant_version;
DROP TABLE IF EXISTS rfx.rfx_template_sections;

ALTER TABLE rfx.rfx_template_versions
    DROP CONSTRAINT IF EXISTS fk_rfx_template_versions_template_composite;

DROP INDEX IF EXISTS rfx.uq_rfx_template_versions_one_published;
DROP INDEX IF EXISTS rfx.uq_rfx_template_versions_one_draft;
DROP INDEX IF EXISTS rfx.uq_rfx_template_versions_tenant_template_id;
DROP INDEX IF EXISTS rfx.idx_rfx_template_versions_tenant_template;
DROP TABLE IF EXISTS rfx.rfx_template_versions;

DROP INDEX IF EXISTS rfx.uq_rfx_templates_tenant_id;
DROP INDEX IF EXISTS rfx.uq_rfx_templates_tenant_code;
DROP INDEX IF EXISTS rfx.idx_rfx_templates_tenant_status;
DROP TABLE IF EXISTS rfx.rfx_templates;
