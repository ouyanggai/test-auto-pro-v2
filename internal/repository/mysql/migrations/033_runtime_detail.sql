-- 评审 P1-7 修复：runtime 动作的执行详情（三阶段投影 + 逐请求摘要 + 七值状态）落库。
--
-- 为什么必须落库：此前 runtime 返回的请求轨迹（ActionRunResult.RequestTrace）与判定状态
-- 只在内存里流转，BuildDetailProjection 没有任何生产调用方，运行详情看不到 runtime 三阶段、
-- 请求轨迹与业务失败状态。投影在执行器构造后随尝试行 INSERT 一次写入，与 before_facts 同纪律：
-- 只写一次、永不 UPDATE；列为空表示非 runtime 步骤或本迁移之前的尝试，前端按缺失降级。
-- 编号取 033：032 是二级派发迁移，迁移编号一律不复用。

ALTER TABLE run_step_attempts
  ADD COLUMN runtime_detail LONGTEXT NULL;
