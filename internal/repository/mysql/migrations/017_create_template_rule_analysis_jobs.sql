CREATE TABLE IF NOT EXISTS test_template_rule_analysis_jobs (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  mode VARCHAR(32) NOT NULL,
  account VARCHAR(255) NOT NULL,
  status VARCHAR(32) NOT NULL,
  total_count INT NOT NULL DEFAULT 0,
  processed_count INT NOT NULL DEFAULT 0,
  completed_count INT NOT NULL DEFAULT 0,
  needs_attention_count INT NOT NULL DEFAULT 0,
  failed_count INT NOT NULL DEFAULT 0,
  message VARCHAR(500) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  finished_at DATETIME(3) NULL,
  KEY idx_template_rule_jobs_account (account, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
