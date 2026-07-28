CREATE TABLE IF NOT EXISTS test_plans (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  create_key CHAR(36) NOT NULL,
  name VARCHAR(60) NOT NULL,
  account VARCHAR(100) NOT NULL,
  account_display_name VARCHAR(100) NOT NULL DEFAULT '',
  flow_source VARCHAR(16) NOT NULL,
  target_object_id VARCHAR(100) NOT NULL,
  target_object_name VARCHAR(255) NOT NULL,
  run_mode VARCHAR(16) NOT NULL,
  max_concurrency SMALLINT UNSIGNED NULL,
  scheduled_at DATETIME(3) NULL,
  status VARCHAR(32) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_test_plans_create_key (create_key),
  KEY idx_test_plans_status_updated (status, updated_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
