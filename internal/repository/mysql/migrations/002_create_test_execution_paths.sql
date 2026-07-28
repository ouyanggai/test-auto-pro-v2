CREATE TABLE IF NOT EXISTS test_execution_paths (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  plan_id BIGINT UNSIGNED NOT NULL,
  sequence_no INT UNSIGNED NOT NULL,
  create_key CHAR(36) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_test_execution_paths_plan_sequence (plan_id, sequence_no),
  UNIQUE KEY uk_test_execution_paths_create_key (create_key),
  CONSTRAINT fk_test_execution_paths_plan FOREIGN KEY (plan_id) REFERENCES test_plans (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
