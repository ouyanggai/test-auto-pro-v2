-- 两级调度（切片 1）：计划运行排队列 + 启动时路径并发可覆盖。
-- 纪律：聚合状态列只单向前进；队列放行由调度器原子领取，同一次运行绝不被两个 Tick 同时启动。
--
-- 1) runs.path_dispatch：计划内路径调度方式（serial / parallel），来自启动弹窗的本次选择；
--    runs 已有 max_concurrency 列，语义收窄为「本运行的路径并发上限」，与 path_dispatch 配套。
-- 2) plan_run_queue 是计划间串行的等待队列：计划层串行模式下，已有计划运行未到终态时，
--    新启动/定时触发的运行先进队列 pending，调度器按入队顺序放行；并行模式不入队直接跑。
--    queue_no 用计划内自增不行（跨计划），改用全局自增主键 + 入队时间排序，放行即删行。

ALTER TABLE runs
  ADD COLUMN path_dispatch VARCHAR(16) NULL AFTER max_concurrency;

CREATE TABLE IF NOT EXISTS plan_run_queue (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  plan_id BIGINT UNSIGNED NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  enqueued_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_plan_run_queue_run (run_id),
  KEY idx_plan_run_queue_order (status, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
