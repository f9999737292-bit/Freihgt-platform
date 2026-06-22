DROP TRIGGER IF EXISTS trg_lowcode_rules_updated_at ON lowcode.rules;
DROP TRIGGER IF EXISTS trg_lowcode_rule_sets_updated_at ON lowcode.rule_sets;
DROP TRIGGER IF EXISTS trg_lowcode_custom_field_values_updated_at ON lowcode.custom_field_values;
DROP TRIGGER IF EXISTS trg_lowcode_form_fields_updated_at ON lowcode.form_fields;
DROP TRIGGER IF EXISTS trg_lowcode_form_sections_updated_at ON lowcode.form_sections;
DROP TRIGGER IF EXISTS trg_lowcode_form_templates_updated_at ON lowcode.form_templates;
DROP TRIGGER IF EXISTS trg_lowcode_configurations_updated_at ON lowcode.low_code_configurations;

DROP TABLE IF EXISTS lowcode.configuration_approvals;
DROP TABLE IF EXISTS lowcode.configuration_audit_log;
DROP TABLE IF EXISTS lowcode.custom_field_values;
DROP TABLE IF EXISTS lowcode.rules;
DROP TABLE IF EXISTS lowcode.rule_sets;
DROP TABLE IF EXISTS lowcode.form_fields;
DROP TABLE IF EXISTS lowcode.form_sections;
DROP TABLE IF EXISTS lowcode.form_templates;
DROP TABLE IF EXISTS lowcode.low_code_configurations;
