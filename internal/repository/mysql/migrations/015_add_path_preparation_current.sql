ALTER TABLE test_path_preparation_jobs
  ADD COLUMN current_path_id BIGINT UNSIGNED NULL AFTER preserved_manual_count,
  ADD COLUMN current_sequence_no INT UNSIGNED NULL AFTER current_path_id,
  ADD COLUMN current_path_name VARCHAR(50) NOT NULL DEFAULT '' AFTER current_sequence_no,
  ADD COLUMN current_item_status VARCHAR(24) NOT NULL DEFAULT '' AFTER current_path_name;
