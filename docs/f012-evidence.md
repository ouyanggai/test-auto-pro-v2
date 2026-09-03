# F-012 证据台账

本台账只记录当前切片可消费的目标平台事实；目标写操作保持关闭。

| 领域 | 事实/入口 | 当前项目依据 |
| --- | --- | --- |
| 历史实例 | `/web/flowInstanceApi/list` 返回 `id`、`flowCode`、`formName`、`name`、`status`、摘要、发起人和时间；列表支持 `flowCode` 精确查询 | `internal/adapter/target/client.go` 的 `ListSubmitted`，由 `internal/service/target_read.go` 负责服务端过滤 |
| 待发实例 | `/web/flowJobTaskLink/list` 返回实例/代理/节点入口和 `formName`；只读分页上限 20 页 | `internal/adapter/target/client.go` 的 `ListDue`、`FindDueFlow` |
| 原始表单 | `/web/flowInstanceApi/getCurrentFromData` 返回 `data` 原始 JSON；不经字段映射 | `internal/adapter/target/client.go` 的 `ReadInstanceCurrentData` |
| 模板/页面 | `/web/flowTemplateApi/findById`、`/web/flowProxy/findById` 和现有 form-runtime 协议负责模板、页面与数据回显 | `internal/adapter/target/client.go`、`form-runtime/src/runtime` |
| 条件 | `lt`、`gt`、`lte`、`gte`、`eq`、`neq`、`contains`、`is_update`、`is_not_null`、`boolean_value`；保持目标保存顺序、首命中和末分支兜底 | `docs/ARCHITECTURE.md` F-012 章节；目标流程树条件字段 |
| 动作 | 发起生命周期、当前待办、已办恢复、实例管理动作只读投影；系统节点只读说明 | `docs/features/F-012-history-form-replay-action-orchestration.md`；GroupApproveManage 直接引用源码 |
| 写边界 | F-012 不创建运行记录、不调用目标平台写接口；所有候选/快照读取在目标会话 `DoRead` 内完成 | `internal/session/manager.go`、F-012 范围约束 |

## 数据库清单

- 保留：`schema_migrations`、`test_form_runtime_sync_jobs` 表及维护能力。
- 重建：`test_plans` 的路径配置关联、`test_execution_path_configs`。
- 新增：历史快照、计划默认来源、历史回放任务、回放明细，以及按新领域字段拆分的路径配置。
- 删除：旧路径准备、模板规则目录/分析任务、旧动作循环和仅服务旧生成链路的表；迁移不读取或解释旧 JSON。

迁移只在通过 `PLAN_DB_*` 连接的工具数据库中运行；目标平台数据库、Mongo 和实例数据不参与迁移。
