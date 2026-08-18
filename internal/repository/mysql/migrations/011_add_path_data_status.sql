ALTER TABLE test_execution_path_configs
  ADD COLUMN data_status VARCHAR(32) NOT NULL DEFAULT 'not_generated' AFTER form_status;
