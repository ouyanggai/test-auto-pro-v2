-- F-016 执行器最小真实闭环：运行记录六张表（纲领第 7.1、7.2 节）。
-- 纪律：
-- 1. 事实表（run_steps、run_step_attempts、run_events、run_controls）只 INSERT，永不 UPDATE、永不 DELETE；
--    一步、一次尝试的事实要么完整落库，要么完全不落库，不存在“先占位再补写”的中间态。
-- 2. 聚合表（runs、path_runs）的状态列只单向前进，每次状态更新必须在同一事务内追加 run_events 事件行。
-- 3. 编号取 026：024 建过断言表、025 已用于删除它，迁移编号一律不复用。
-- 4. 载荷类列一律 LONGTEXT 存原始 JSON 文本，不用原生 JSON 列（023 的教训：原生 JSON 会改写数字字面量）。
-- 5. 目标平台只存不透明键与业务摘要；目标会话 SID 绝不进任何表。

-- 一次启动：计划、运行号、运行模式、触发方式、并发上限、起止时间与公开状态。
-- 运行号在计划内单调递增，由 (plan_id, run_no) 唯一键保证；需要“再次运行”时新建行，不改旧记录。
CREATE TABLE IF NOT EXISTS runs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  plan_id BIGINT UNSIGNED NOT NULL,
  run_no BIGINT UNSIGNED NOT NULL,
  mode VARCHAR(32) NOT NULL,
  trigger_kind VARCHAR(32) NOT NULL,
  max_concurrency INT NULL,
  status VARCHAR(32) NOT NULL,
  result VARCHAR(32) NULL,
  failure_class VARCHAR(32) NULL,
  started_at DATETIME(3) NULL,
  finished_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_runs_plan_run_no (plan_id, run_no),
  KEY idx_runs_plan_status (plan_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- 一条执行路径的运行：一条路径运行独占一个真实主实例（main_instance_ref 只存不透明引用）。
-- 状态机：未开始 -> 等待运行 -> 运行中 -> 核验中 -> (已完成 | 失败 | 暂停 | 待对账 | 已停止 | 已取消)，只前进不回退。
-- result（路径结果）与 final_target_summary（最终目标事实摘要）是两个独立字段，不合成一个判定。
-- 租约三列（lease_owner、lease_expires_at、fencing_token）保证同一路径运行同时只有一个 Worker 推进，
-- 形状与 test_history_replay_jobs 一致；进程重启后处于运行中/核验中的行一律置为待对账。
CREATE TABLE IF NOT EXISTS path_runs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  execution_path_id BIGINT UNSIGNED NOT NULL,
  status VARCHAR(32) NOT NULL,
  result VARCHAR(32) NULL,
  failure_class VARCHAR(32) NULL,
  main_instance_ref VARCHAR(255) NULL,
  final_target_summary LONGTEXT NULL,
  lease_owner VARCHAR(128) NULL,
  lease_expires_at DATETIME(3) NULL,
  fencing_token BIGINT UNSIGNED NOT NULL DEFAULT 0,
  started_at DATETIME(3) NULL,
  finished_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  KEY idx_path_runs_run (run_id),
  KEY idx_path_runs_path (execution_path_id),
  KEY idx_path_runs_status (status),
  CONSTRAINT fk_path_runs_run FOREIGN KEY (run_id) REFERENCES runs (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- 每个编译步骤的执行事实（append-only）：步骤在落账时一次性写入完整事实，
-- 崩溃时未走完的步骤不落行，只以事件行记录“在第几步被打断”，保证表里只有已发生且已确认的事实。
CREATE TABLE IF NOT EXISTS run_steps (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  path_run_id BIGINT UNSIGNED NOT NULL,
  step_no INT UNSIGNED NOT NULL,
  source VARCHAR(32) NOT NULL,
  action VARCHAR(64) NOT NULL,
  node_key VARCHAR(128) NOT NULL,
  actor_summary VARCHAR(255) NULL,
  gate_snapshot LONGTEXT NULL,
  status VARCHAR(32) NOT NULL,
  started_at DATETIME(3) NOT NULL,
  finished_at DATETIME(3) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  KEY idx_run_steps_path_run (path_run_id),
  CONSTRAINT fk_run_steps_path_run FOREIGN KEY (path_run_id) REFERENCES path_runs (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- 每次尝试的判定事实（append-only）：三值判定、传输结论、响应侧初判、事实重读结论、
-- 失败分类，以及记录与日志双向可达所需的 trace_id、curl_trace_id、相对日志路径与行号。
-- 一次尝试最多一次写请求；任何重发都必须先经过对账并作为新的一次尝试记录（F-018）。
CREATE TABLE IF NOT EXISTS run_step_attempts (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  path_run_id BIGINT UNSIGNED NOT NULL,
  step_id BIGINT UNSIGNED NOT NULL,
  attempt_no INT UNSIGNED NOT NULL,
  verdict VARCHAR(32) NOT NULL,
  side_effect VARCHAR(16) NOT NULL,
  transport VARCHAR(32) NOT NULL,
  status_code INT NULL,
  initial VARCHAR(32) NOT NULL DEFAULT '',
  reread VARCHAR(32) NOT NULL DEFAULT '',
  failure_class VARCHAR(32) NULL,
  reason VARCHAR(512) NOT NULL,
  basis VARCHAR(512) NOT NULL,
  trace_id VARCHAR(64) NOT NULL,
  curl_trace_id VARCHAR(64) NOT NULL,
  log_path VARCHAR(512) NOT NULL,
  log_line BIGINT UNSIGNED NOT NULL,
  duration_ms BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  KEY idx_run_step_attempts_path_run (path_run_id),
  KEY idx_run_step_attempts_step (step_id),
  CONSTRAINT fk_run_step_attempts_path_run FOREIGN KEY (path_run_id) REFERENCES path_runs (id),
  CONSTRAINT fk_run_step_attempts_step FOREIGN KEY (step_id) REFERENCES run_steps (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- 运行事件流（append-only）：聚合表每次状态前进在同一事务内追加一行，label 存中文标签，
-- 供实时界面与事后回放“状态怎么变的”。
CREATE TABLE IF NOT EXISTS run_events (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  path_run_id BIGINT UNSIGNED NULL,
  kind VARCHAR(32) NOT NULL,
  label VARCHAR(255) NOT NULL,
  detail LONGTEXT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  KEY idx_run_events_run (run_id),
  KEY idx_run_events_path_run (path_run_id),
  CONSTRAINT fk_run_events_run FOREIGN KEY (run_id) REFERENCES runs (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- 人工控制事实（append-only）：本切片只承载放行（approve）与停止（stop）两类事实，可审计。
CREATE TABLE IF NOT EXISTS run_controls (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  path_run_id BIGINT UNSIGNED NOT NULL,
  action VARCHAR(32) NOT NULL,
  source VARCHAR(32) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  KEY idx_run_controls_path_run (path_run_id),
  CONSTRAINT fk_run_controls_run FOREIGN KEY (run_id) REFERENCES runs (id),
  CONSTRAINT fk_run_controls_path_run FOREIGN KEY (path_run_id) REFERENCES path_runs (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
