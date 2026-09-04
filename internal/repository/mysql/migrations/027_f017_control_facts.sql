-- F-017 调试器：run_controls 增补控制事实列（纲领第 5.2、6.2、7.2 节）。
-- 纪律：
-- 1. run_controls 保持 append-only（只 INSERT，永不 UPDATE/DELETE），本迁移只加列不改行。
-- 2. F-016 的 action 列（approve/stop）保留原义；本切片的控制事实九类以 kind 列区分，
--    断点类事实携带 breakpoint_type/object_kind/object_key，命令类事实携带 command，停止/暂停携带 reason。
-- 3. runs.mode 为 VARCHAR（无数据库枚举），扩展 auto/manual_control 取值不需要迁移，编号 027 仅此一处。

ALTER TABLE run_controls
  ADD COLUMN kind VARCHAR(32) NOT NULL DEFAULT '' AFTER path_run_id,
  ADD COLUMN breakpoint_type VARCHAR(32) NULL AFTER action,
  ADD COLUMN object_kind VARCHAR(32) NULL AFTER breakpoint_type,
  ADD COLUMN object_key VARCHAR(255) NULL AFTER object_kind,
  ADD COLUMN command VARCHAR(32) NULL AFTER object_key,
  ADD COLUMN reason VARCHAR(512) NULL AFTER command;

-- 存量行（F-016 的放行与停止）按 action 回填 kind，保持事实可按统一类别检索。
UPDATE run_controls SET kind = 'approved' WHERE action = 'approve' AND kind = '';
UPDATE run_controls SET kind = 'stopped' WHERE action = 'stop' AND kind = '';

CREATE INDEX idx_run_controls_kind ON run_controls (path_run_id, kind);
