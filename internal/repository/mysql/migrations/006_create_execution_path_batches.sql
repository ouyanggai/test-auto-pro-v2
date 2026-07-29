CREATE TABLE IF NOT EXISTS test_execution_path_batches (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  plan_id BIGINT UNSIGNED NOT NULL,
  create_key CHAR(36) NOT NULL,
  total_count INT UNSIGNED NOT NULL,
  existing_count INT UNSIGNED NOT NULL,
  created_count INT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_execution_path_batch_create_key (create_key),
  KEY idx_execution_path_batches_plan (plan_id),
  CONSTRAINT fk_execution_path_batches_plan FOREIGN KEY (plan_id) REFERENCES test_plans (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
