CREATE TABLE test_path_preparation_items (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  job_id CHAR(36) NOT NULL,
  path_id BIGINT UNSIGNED NOT NULL,
  sequence_no INT UNSIGNED NOT NULL,
  path_name VARCHAR(50) NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'pending',
  reason VARCHAR(500) NOT NULL DEFAULT '',
  node_configured TINYINT(1) NOT NULL DEFAULT 0,
  data_generated TINYINT(1) NOT NULL DEFAULT 0,
  needs_attention TINYINT(1) NOT NULL DEFAULT 0,
  preserved_manual TINYINT(1) NOT NULL DEFAULT 0,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_test_path_preparation_job_path (job_id, path_id),
  KEY idx_test_path_preparation_job_status_id (job_id, status, id),
  CONSTRAINT fk_test_path_preparation_item_job FOREIGN KEY (job_id) REFERENCES test_path_preparation_jobs (id) ON DELETE CASCADE,
  CONSTRAINT fk_test_path_preparation_item_path FOREIGN KEY (path_id) REFERENCES test_execution_paths (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
