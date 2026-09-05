-- F-018 根治「对账现场只在内存里」：把每次尝试「写之前的目标事实」落库。
--
-- 为什么必须落库：对账判定的一半输入是"写之前实例什么样"（BeforeHadInstance/BeforeStatus）。
-- 它此前只存在于 control.Service 的内存现场里，进程一重启，停在待对账的路径运行就再也拿不到基准，
-- 对账与三个恢复动作全部失去入口（历史上已有 10 条这样的运行）。
--
-- 归属：这是"这一次尝试"的可观测事实，与 transport/initial/reread 同类，因此落在 run_step_attempts。
-- 纪律不变：本列在尝试行 INSERT 时一次写入，永不 UPDATE；列为空只表示那次尝试发生在本迁移之前
-- 或写请求根本没发出，对账据此按证据缺失降级，绝不猜。
-- 编号取 029：028 是 F-018 的对账三列与人工结论表，迁移编号一律不复用。

ALTER TABLE run_step_attempts
  ADD COLUMN before_facts LONGTEXT NULL;
