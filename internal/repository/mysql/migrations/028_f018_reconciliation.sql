-- F-018 对账与安全重试：run_step_attempts 增补对账三列，新增人工核对结论事实表。
-- 纪律：事实表只 INSERT；对账三列是纲领第 7.2 节明确归属 run_step_attempts 的字段，
-- 更新仅限这三列（对账结论/恢复动作/是否重放），不触碰尝试的其他事实。

ALTER TABLE run_step_attempts
  ADD COLUMN reconcile_verdict VARCHAR(32) NULL,
  ADD COLUMN recovery_action VARCHAR(32) NULL,
  ADD COLUMN is_replay TINYINT(1) NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS run_manual_conclusions (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  path_run_id BIGINT UNSIGNED NOT NULL,
  step_no INT UNSIGNED NOT NULL,
  instance_status VARCHAR(64) NOT NULL,
  current_node VARCHAR(255) NOT NULL,
  note VARCHAR(512) NULL,
  reporter VARCHAR(128) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  KEY idx_run_manual_conclusions_path_run (path_run_id),
  CONSTRAINT fk_run_manual_conclusions_path_run FOREIGN KEY (path_run_id) REFERENCES path_runs (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
