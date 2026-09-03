-- F-012 只清理测试工具数据库的业务数据与旧领域表；schema_migrations 和运行时维护表保留。
SET FOREIGN_KEY_CHECKS = 0;

DELETE FROM test_form_runtime_sync_jobs;
DROP TABLE IF EXISTS test_path_preparation_items;
DROP TABLE IF EXISTS test_path_preparation_jobs;
DROP TABLE IF EXISTS test_template_rule_analysis_jobs;
DROP TABLE IF EXISTS test_template_rule_catalog;
DROP TABLE IF EXISTS test_execution_path_batch_items;
DROP TABLE IF EXISTS test_execution_path_batches;
DROP TABLE IF EXISTS test_execution_path_configs;
DROP TABLE IF EXISTS test_execution_path_choices;
DROP TABLE IF EXISTS test_execution_paths;
DROP TABLE IF EXISTS test_plans;

CREATE TABLE test_plans (
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
  next_path_sequence_no INT UNSIGNED NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_test_plans_create_key (create_key),
  KEY idx_test_plans_status_updated (status, updated_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE test_execution_paths (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  plan_id BIGINT UNSIGNED NOT NULL,
  sequence_no INT UNSIGNED NOT NULL,
  create_key CHAR(36) NOT NULL,
  name VARCHAR(50) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_test_execution_paths_plan_sequence (plan_id, sequence_no),
  UNIQUE KEY uk_test_execution_paths_create_key (create_key),
  CONSTRAINT fk_test_execution_paths_plan FOREIGN KEY (plan_id) REFERENCES test_plans (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE test_execution_path_choices (
  path_id BIGINT UNSIGNED NOT NULL,
  route_node_id VARCHAR(100) NOT NULL,
  branch_id VARCHAR(100) NOT NULL,
  PRIMARY KEY (path_id, route_node_id),
  CONSTRAINT fk_test_execution_path_choices_path FOREIGN KEY (path_id) REFERENCES test_execution_paths (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- F-005 的批量路径结果仍是路径拓扑的一部分；清空旧业务数据后重建表结构，不能因删除旧路径准备任务而一并删除。
CREATE TABLE test_execution_path_batches (
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE test_execution_path_batch_items (
  batch_id BIGINT UNSIGNED NOT NULL,
  item_order INT UNSIGNED NOT NULL,
  path_id BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (batch_id, item_order),
  UNIQUE KEY uk_execution_path_batch_path (batch_id, path_id),
  CONSTRAINT fk_execution_path_batch_items_batch FOREIGN KEY (batch_id) REFERENCES test_execution_path_batches (id) ON DELETE CASCADE,
  CONSTRAINT fk_execution_path_batch_items_path FOREIGN KEY (path_id) REFERENCES test_execution_paths (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE test_history_data_snapshots (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  plan_id BIGINT UNSIGNED NOT NULL,
  source_account VARCHAR(100) NOT NULL,
  candidate_key CHAR(64) NOT NULL,
  flow_code VARCHAR(255) NOT NULL DEFAULT '',
  form_name VARCHAR(255) NOT NULL DEFAULT '',
  flow_name VARCHAR(255) NOT NULL DEFAULT '',
  render_type VARCHAR(32) NOT NULL,
  instance_status VARCHAR(64) NOT NULL DEFAULT '',
  instance_summary JSON NOT NULL,
  template_summary JSON NOT NULL,
  raw_form_data JSON NOT NULL,
  source_digest CHAR(64) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_history_snapshot_plan_candidate (plan_id, candidate_key),
  KEY idx_history_snapshot_plan_flow (plan_id, flow_code),
  CONSTRAINT fk_history_snapshot_plan FOREIGN KEY (plan_id) REFERENCES test_plans (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE test_plan_history_data_defaults (
  plan_id BIGINT UNSIGNED NOT NULL,
  snapshot_id BIGINT UNSIGNED NOT NULL,
  revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
  idempotency_key CHAR(36) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (plan_id),
  CONSTRAINT fk_history_default_plan FOREIGN KEY (plan_id) REFERENCES test_plans (id) ON DELETE CASCADE,
  CONSTRAINT fk_history_default_snapshot FOREIGN KEY (snapshot_id) REFERENCES test_history_data_snapshots (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE test_history_replay_jobs (
  id CHAR(36) NOT NULL,
  plan_id BIGINT UNSIGNED NOT NULL,
  idempotency_key CHAR(36) NOT NULL,
  status VARCHAR(24) NOT NULL,
  total_count INT UNSIGNED NOT NULL DEFAULT 0,
  pending_count INT UNSIGNED NOT NULL DEFAULT 0,
  running_count INT UNSIGNED NOT NULL DEFAULT 0,
  ready_count INT UNSIGNED NOT NULL DEFAULT 0,
  needs_input_count INT UNSIGNED NOT NULL DEFAULT 0,
  affected_count INT UNSIGNED NOT NULL DEFAULT 0,
  failed_count INT UNSIGNED NOT NULL DEFAULT 0,
  cancelled_count INT UNSIGNED NOT NULL DEFAULT 0,
  lease_owner VARCHAR(128) NULL,
  lease_expires_at DATETIME(3) NULL,
  fencing_token BIGINT UNSIGNED NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  completed_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_history_replay_job_idempotency (plan_id, idempotency_key),
  KEY idx_history_replay_job_plan_status (plan_id, status, updated_at),
  CONSTRAINT fk_history_replay_job_plan FOREIGN KEY (plan_id) REFERENCES test_plans (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE test_history_replay_items (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  job_id CHAR(36) NOT NULL,
  path_id BIGINT UNSIGNED NOT NULL,
  path_revision BIGINT UNSIGNED NOT NULL,
  snapshot_id BIGINT UNSIGNED NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'pending',
  data_status VARCHAR(24) NOT NULL DEFAULT 'empty',
  issue JSON NOT NULL,
  branch_patches JSON NOT NULL,
  effective_form_data JSON NOT NULL,
  lease_owner VARCHAR(128) NULL,
  lease_expires_at DATETIME(3) NULL,
  updated_at DATETIME(3) NOT NULL,
  completed_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_history_replay_item_job_path (job_id, path_id),
  KEY idx_history_replay_item_job_status_id (job_id, status, id),
  CONSTRAINT fk_history_replay_item_job FOREIGN KEY (job_id) REFERENCES test_history_replay_jobs (id) ON DELETE CASCADE,
  CONSTRAINT fk_history_replay_item_path FOREIGN KEY (path_id) REFERENCES test_execution_paths (id) ON DELETE CASCADE,
  CONSTRAINT fk_history_replay_item_snapshot FOREIGN KEY (snapshot_id) REFERENCES test_history_data_snapshots (id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE test_execution_path_configs (
  path_id BIGINT UNSIGNED NOT NULL,
  revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
  node_revision BIGINT UNSIGNED NOT NULL DEFAULT 0,
  data_revision BIGINT UNSIGNED NOT NULL DEFAULT 0,
  action_revision BIGINT UNSIGNED NOT NULL DEFAULT 0,
  idempotency_key CHAR(36) NOT NULL DEFAULT '',
  config_status VARCHAR(24) NOT NULL DEFAULT 'pending',
  node_status VARCHAR(24) NOT NULL DEFAULT 'pending',
  data_status VARCHAR(24) NOT NULL DEFAULT 'empty',
  source_mode VARCHAR(16) NOT NULL DEFAULT 'none',
  snapshot_id BIGINT UNSIGNED NULL,
  runtime_type VARCHAR(32) NOT NULL DEFAULT 'unknown',
  person_strategies JSON NOT NULL,
  user_actions JSON NOT NULL,
  compiled_steps JSON NOT NULL,
  confirmed_node_keys JSON NOT NULL,
  effective_form_data JSON NOT NULL,
  branch_patches JSON NOT NULL,
  runtime_validation JSON NOT NULL,
  issues JSON NOT NULL,
  latest_idempotency_result JSON NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (path_id),
  CONSTRAINT fk_f012_path_config_path FOREIGN KEY (path_id) REFERENCES test_execution_paths (id) ON DELETE CASCADE,
  CONSTRAINT fk_f012_path_config_snapshot FOREIGN KEY (snapshot_id) REFERENCES test_history_data_snapshots (id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

SET FOREIGN_KEY_CHECKS = 1;
