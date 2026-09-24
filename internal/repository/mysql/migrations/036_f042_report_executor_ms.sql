-- F-042 报告中心真实度量：执行器（独立执行服务/Playwright）动作墙钟真实耗时落库。
--
-- 为什么必须落库：runtime 动作的墙钟处理耗时此前只在执行服务内存里流转，Go 侧只有
-- duration_ms（引擎步骤计时）与 network.log（目标传输层计时），报告中心的「执行器耗时」
-- 分解拿不到执行器自身的真实处理耗时。执行服务在动作从装载到收敛用真实计时器测得，
-- 随 ActionRunResult 上报，Go 落账时一次写入本列。
-- 纪律：与 before_facts / runtime_detail 同类，只在尝试行 INSERT 时一次写入，永不 UPDATE；
-- 列为 NULL 表示非 runtime（Go 直发）步骤或本迁移之前的历史尝试，报告按缺失降级显示
-- 「无执行器环节」或「未知」，绝不用 0 冒充真实耗时。
-- 编号取 036：035 是加签排除名单，迁移编号一律不复用。

ALTER TABLE run_step_attempts
  ADD COLUMN executor_ms BIGINT UNSIGNED NULL;
