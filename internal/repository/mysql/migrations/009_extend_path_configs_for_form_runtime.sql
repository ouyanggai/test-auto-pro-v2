ALTER TABLE test_execution_path_configs
  ADD COLUMN config_version INT UNSIGNED NOT NULL DEFAULT 1 AFTER config_status,
  ADD COLUMN node_revision BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER revision,
  ADD COLUMN form_revision BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER node_revision,
  ADD COLUMN confirmed_node_keys JSON NOT NULL DEFAULT (JSON_ARRAY()) AFTER action_values,
  ADD COLUMN form_values JSON NOT NULL DEFAULT (JSON_OBJECT()) AFTER confirmed_node_keys,
  ADD COLUMN form_status VARCHAR(32) NOT NULL DEFAULT 'empty' AFTER form_values,
  ADD COLUMN form_validated TINYINT(1) NOT NULL DEFAULT 0 AFTER form_status,
  ADD COLUMN form_seed BIGINT NOT NULL DEFAULT 0 AFTER form_validated,
  ADD COLUMN generated_field_paths JSON NOT NULL DEFAULT (JSON_ARRAY()) AFTER form_seed,
  ADD COLUMN manual_override_paths JSON NOT NULL DEFAULT (JSON_ARRAY()) AFTER generated_field_paths,
  ADD COLUMN sample_summary JSON NOT NULL DEFAULT (JSON_OBJECT()) AFTER manual_override_paths,
  ADD COLUMN form_template_version VARCHAR(128) NOT NULL DEFAULT '' AFTER sample_summary;
