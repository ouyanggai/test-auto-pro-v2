CREATE TABLE IF NOT EXISTS test_execution_path_configs (
  path_id BIGINT UNSIGNED NOT NULL,
  revision BIGINT UNSIGNED NOT NULL DEFAULT 0,
  idempotency_key CHAR(36) NOT NULL DEFAULT '',
  config_status VARCHAR(32) NOT NULL DEFAULT 'configured',
  field_values JSON NOT NULL,
  action_values JSON NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (path_id),
  CONSTRAINT fk_test_execution_path_configs_path FOREIGN KEY (path_id) REFERENCES test_execution_paths (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
