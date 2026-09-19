-- 一键配置动作模式（F-039 用户裁决）：区分「每一步都同意（快速跑通）」与「每节点随机一个动作」。
-- 模式是任务级选择，必须随任务持久化：worker 异步执行、服务重启后按 DB 恢复，内存传参不够。
-- 编号取 034：033 是 runtime 详情迁移，迁移编号一律不复用。默认 random 保持历史行为不变。
ALTER TABLE test_history_replay_jobs
  ADD COLUMN action_mode VARCHAR(24) NOT NULL DEFAULT 'random';
