-- F-020 多路径调度与单次定时启动：运行级调度所需的字段补充（编号从 030 起，不复用）。
-- 纪律：
-- 1. runs/path_runs 是事实聚合表，本迁移只加列不改行；既有单路径运行行为不变。
-- 2. 载荷类列一律 LONGTEXT 存原始 JSON 文本，不用原生 JSON 列（023 的教训）。
-- 3. 目标平台只存不透明键与业务摘要；目标会话 SID 绝不进任何表。

-- 启动请求幂等：同一幂等键的重试返回同一次运行，绝不创建第二个运行或第二份路径实例。
-- idempotency_key 为空表示未携带（人工单次启动允许为空；前端会对每次启动生成键）。
ALTER TABLE runs
  ADD COLUMN idempotency_key VARCHAR(64) NULL,
  ADD COLUMN preset_breakpoints LONGTEXT NULL,
  ADD UNIQUE INDEX uk_runs_plan_idempotency (plan_id, idempotency_key);

-- preset_breakpoints 是启动请求里预置的断点集合（JSON 数组）：
-- 串行/并行调度会在每条路径运行开始时重放同一预置（首次写断点对每条路径都是安全阀），
-- 服务重启后等待路径恢复启动时也从这里取，不依赖进程内存。

-- 单次定时启动的一次性消费标记：到点只消费一次，服务重启或并发扫描不得重复触发。
-- 计划的 scheduled_at 继续用 F-003 既有列，消费事实落在计划行上。
ALTER TABLE test_plans
  ADD COLUMN scheduled_consumed_at DATETIME(3) NULL;
