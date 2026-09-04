# 当前进度

- 当前功能：F-014 目标错误语义与幂等勘定（`ready_for_manual`）；F-013 分层日志与追踪底座同样停在 `ready_for_manual`。
- F-014 已完成：新增 `docs/TARGET_SEMANTICS.md` 作为目标平台行为结论的唯一载体，14 条标题建立，本切片只填错误语义与幂等两条；新增纯判定包 `internal/engine/verdict`（三值结论、五项输入含动作与端点、20 格矩阵、兜底一律不确定、会话失效覆盖 `AUTH_401`、`batchCode` 禁令），未接入任何现有流程；新增判定表对照测试、真实目标只读集成测试、写端点白名单与证据漂移检测脚本、`test/run-f014.sh` 汇总入口。
- F-014 关键结论修正：审批端点在乐观锁检查之前已写入 Mongo 表单数据且不受该事务约束，因此「乐观锁冲突 + 明确未变」由确定失败降级为不确定。
- F-014 实测部署事实：目标返回的 `auditWay` 是字符串编码名，说明已执行 20260828 迁移；失效会话真实响应是 HTTP 200 + `code=RESP401`，会话失效不一定伴随 HTTP 401。
- F-014 边界：纯只读，未发出任何写请求，未执行任何重复写探针，六条待实测结论均标注待 F-016 实测并配同编号探针。
- 当前状态：`ready_for_manual`（日志查看方式与日志归档方式两轮人工验收反馈均已修复并实测通过，等待重新验收）。用户 2026-09-04 明确：暂未发现问题，但不视为明确验收。
- F-013 人工验收反馈（已修复）：“日志没有按计划和执行路径归档，配置日志集中在日期目录，无法从业务对象定位。”
  现在顶层只有 `logs/application/<日期>/` 与 `logs/plans/<计划显示名>__plan-<ID>/`：
  配置阶段落 `configuration/<执行路径显示名>__path-<ID>/<日期>/`（`meta.json`、`operation.log`、`operation-error.log`
  与三个网络日志文件），执行阶段落 `runs/<执行路径显示名>__path-<ID>/<运行号>/`（`execution.log`、`execution-error.log`
  与同样三个网络日志文件），只知道计划时落 `configuration/_plan/<日期>/`。
  `logging.Scope` 增加计划与执行路径的 ID 与显示名并进日志字段，`WithScope` 改为合并语义；
  中间件只从路由取不可变 ID，显示名由 `service.LogScopeService` 从真实业务记录读取并完成归属校验，每请求解析一次；
  作用域随 context 传给目标站点客户端，`network.log`、`curl.log` 与业务错误日志因此都落进同一个计划目录；
  `application` 只保留启动停止与无法归属业务对象的系统级事件，业务日志不再重复写入根目录。
- F-013 归档实测（真实数据）：计划 2（`oyg测试002`）路径 13（`路径 1`）的配置接口日志落在
  `logs/plans/oyg测试002__plan-2/configuration/路径 1__path-13/2026-09-04/`，每行带 `plan_id`/`plan_name`/
  `execution_path_id`/`execution_path_name`；路径 13 与路径 14 互不串目录；只带计划 ID 的接口进 `_plan`；
  `application.log` 只有服务监听与停止等系统事件；code-server 可逐层点到含中文与空格的嵌套目录。
  改动前的 `logs/app-<日期>.log` 与 `logs/config/<日期>/` 已由用户永久删除，当前日志根下只有 `application/` 与 `plans/` 两棵目录树。
- F-013 代码审查两个 P1 问题已修复：后台全路径解析与一键配置 worker 原来用 `context.Background()` 起协程只传 planID，
  日志会掉进 `application` 目录，现在按任务的计划 ID 兜底并继承请求作用域，明细处理时再补上执行路径归属；
  `WriteBlock` 原来逐行加锁导致并发 `curl.log` 块互相穿插，现在整块在同一把锁内一次写入。
  顺带修掉 `StartGeneration` 解锁后复制任务结构体的数据竞争。三处修复均有用例并做过变异验证。
- F-013 复审两个 P2 已修复：后台 worker 与后台路径解析原来在持锁时解析日志作用域（计划名缺失会查库），
  数据库慢会连带阻塞取消、恢复与任务查询，现已把解析移到锁外，锁内只做任务状态判断；
  文档中"旧日志按要求未删除"的记录已更正为已由用户永久删除。
- F-013 已完成：新增 `internal/logging`（日志根、作用域注入、统一单行格式、有界写入器与行号、配置桶与运行目录路由、保留期清理）；
  目标请求日志在传输层单点接入，成功与失败分流到 `network.log` / `network-error.log`，可重放命令与完整响应写入 `curl.log`；
  API 中间件记录请求、失败响应的稳定错误码与界面同源中文提示，并恢复 panic 返回稳定中文 500；
  全局日志按天分文件（`app-<日期>.log`）并按保留期滚动删除，默认保留 7 天；
  日志查看沿用已批准的 code-server：`make logs-viewer` 起固定版本 `codercom/code-server:4.96.4`，
  19002 映射 8080，挂载本项目 `logs/` 到 `/home/coder/logs` 并直接打开该目录，无登录、可读写、
  以当前本机用户 UID/GID 运行；`make logs-viewer-stop` 只停止并删除该容器；`.gitignore` 已忽略 `/logs/`。
  本切片只给单容器启动方式，整套 Docker Compose 编排属于 F-023。
- F-013 实测：本机新构建接真实目标与真实数据库，`network.log` 出现真实请求行，`curl.log` 的命令直接重放成功，
  请求不存在的计划后 `program-error.log` 的 `user_message` 与接口响应体 `error.message` 完全一致。
- F-013 容器实测：`make logs-viewer` 起容器后，容器内实际列出并读到当天 `app-<日期>.log` 与 `config/<日期>/network.log`；容器内写入的探针文件宿主 `logs/` 立刻可见、删除后同步消失；19002 重定向到 `./?folder=/home/coder/logs`，code-server 读到的文件字节数与宿主完全一致；`make logs-viewer-stop` 后容器删除、端口释放。
  外接盘项目在 Colima 下需先把 `logs/` 挂进虚拟机（`~/.colima/default/colima.yaml` 的 `mounts` 加 `writable: true` 后 `colima restart`），否则容器只会看到一个空的可写目录；`make logs-viewer` 已内置挂载双向自检，挂载没生效时直接报错并停容器，不把 `docker inspect` 的 `RW=true` 当成挂载成功。
- F-012（上一切片）状态：`ready_for_manual`。复杂 FormMaking 基础数据初始化整改已完成，等待用户重新验收。
- 已完成：T01 至 T08 已按批准范围实施并完成中文原子提交；已验证 FormMaking 与 `vue_custom`/NoFormFlow 原始数据链路、分支窄补丁、真实动作目录、同一主实例场景编译、数据库重建、旧体系删除和前后端构建。
- 已完成复核修复轮：修复迁移执行器（服务可启动）、条件求值数字保真、动作门禁目录接入产品与实例动作编排、编译场景预览与拖拽排序、集成门禁真实执行；并修复复核中实测发现的 MySQL `JSON` 列改写目标数字和幂等重试按字节比较两个新缺陷。
- 已完成人工验收反馈修复：路径配置接口中的随机人员空选择由 `null` 收敛为按当前候选恢复的数组，前端保留旧响应防护；计划 2 的路径 13 已实测恢复节点配置画布。
- 已完成人工验收反馈修复：配置工作区读取与保存均清理目标 `auto_audit_info_*` 历史审批意见；路径关键字段从目标 FormMaking 报表布局提取真实可见中文标签，并兼容 `__virtualName` 虚拟字段。计划 2 的路径 13 已实测不再返回非空审批意见，“请假天数/请假类别”标签正确。
- 已完成人工验收反馈修复：恢复目标模板 `eventScript` 初始化与字段联动，只隔离提交/草稿钩子；FormMaking 模板中语义明确的查询 POST 进入只读清单，写请求与未知 POST 仍阻断；内存认证恢复 `invest-power-system-*` 键前缀和可遍历 Storage 行为。计划 4 路径 37 接口实测返回合同分类、用户、公司和字典查询 POST，F-012 全量验证与构建通过。
- 已完成人工验收反馈修复：直接挂载 FormMaking 时为业务自定义组件补齐与目标包装层一致的发起态 `extendProps`，空业务 ID 保证合同分类走可用分类查询且不读取目标业务详情覆盖历史数据；F-012 再次全量验证 19 个集成用例无跳过、前端 60 个用例通过。
- 阻塞：无。F-013 先后遇到的两个本机阻塞已解除——Docker 数据盘由用户自行恢复（现 59G 已用 19G、可用 37G，全程未执行 `prune`，镜像、构建缓存与卷均保留）；Colima 未挂载外接盘项目目录的问题已通过在 `mounts` 中新增本项目 `logs/`（`writable: true`）并 `colima restart` 解决，仓库内配套增加挂载自检。MySQL 真实迁移需配置独立测试库（`PLAN_DB_*`），目标平台和浏览器行为保留人工验收边界。
- 待用户判断：迁移 `023` 把 15 个载荷列由 `JSON` 改为 `LONGTEXT`，属于授权范围内的模型变更；另有两处待用户裁决的差异见 F-012 功能文档状态记录。
- 下一步：F-014 目标错误语义与幂等勘定已按用户要求产出计划文档 `docs/features/F-014-target-error-semantics-idempotency.md`，
  状态 `awaiting_approval`，只读、不发写请求，写端点白名单为空；等用户明确批准范围后才进入 `implementing`。
  F-012 与 F-013 均停在 `ready_for_manual`；两者未获明确验收前不进入 `accepted`。
  用户 2026-09-04 已按裁决要求修订 F-014 计划（纯只读、探针只列不跑、未实测结论标待实测、端点加精确文案联合判定、
  未覆盖组合默认不确定、写路径样本按源码构造），并明确等 F-012 返工通过人工验收后再批准 F-014。
- 2026-09-04 参考仓库锚定分支调整：按用户裁决把 `rsh-cloud-workflow-center` 由 `test` 改为 `master`。
  `scripts/reference-repositories.tsv` 已改；旧克隆经洁净核对（无改动、无未跟踪文件、无 stash、未领先远端）后移除，
  由 `make refs-sync` 按 `master` 干净克隆，未使用 reset 或 checkout。
  切换时 `master` `37c01d04` 相对原 `test` `0c6c7f0e` 只多一个把 test 合入 master 的合并提交、内容差异为空，
  既有语义证据不受影响，已在新 HEAD 上逐条复核通过。
  `make refs-sync` 13/13、`make refs-status` 13/13 全部「正常」；`rsh-flow-components` 按裁决保持 `test` `24f3a280`。
- 2026-09-04 用户裁决：`GroupApproveManage/CaseManage/index.vue`「我的案件」操作列不渲染是明确需求，不是缺陷。
  不恢复 `operation` 列，不作为 F-012 阻断项，也不为此修改表单渲染或补测试。
- 2026-09-04 参考仓库同步：按用户要求执行 `make refs-sync`（13/13 快进）。`rsh-cloud-workflow-center` `test` 到 `0c6c7f0e`、
  `rsh-cloud-workflow-center-api` `master` 到 `088aed79`、`rsh-framework-all` `test` 到 `84bb1973`。
  `rsh-flow-components` 规范分支维持 `test`（`24f3a280`）；不改用另一条语义基线 `master`。旧 `master` 工作树已移入
  `参考代码/.branch-backups/rsh-flow-components-master-3ffb41ba`，当前运行快照仍保留历史 `master` 基线，后续维护任务才按清单生成候选同步。
  目标平台上了「审批方式动态化」：`auditWay` 不再受 `AuditWayEnum` 约束，提交校验路由改为字符串键并支持动态注册，
  未注册或无健康实例时前置门禁静默放行，未注册 `auditWay` 由抛异常改为返回 `未发现实例` 失败包；
  另有 `flow_trigger_config_relevance.audit_way` 由数字 ordinal 迁移为 `VARCHAR(100)` 的脚本（目标环境是否已执行需实测）。
  写路径（`submit`/`audit`/`reSubmit`）未改动，F-014 方向不变，已按此调整其任务与完成标准。
  连带影响记入其它切片、本次不改动：待办列表不再带实例运行态字段、终态实例不再返回当前处理人、
  流程图详情改走无 Redis 快速查询链路需只读回归、`auditWay` 迁移前工具读到的可能仍是数字而它被用作 `vue_custom` 页面键。
- 已产出后续工作纲领 `docs/EXECUTION_PROGRAM.md`（执行器、调试器、运行记录与分层日志，F-013 至 F-023），只界定边界与顺序，不构成实施授权。用户已裁决：内网日志原样记录、code-server 随 Docker 交付、发布用 Docker Compose、真实写使用已指定测试账号（账号值不入仓库）；`docs/ARCHITECTURE.md` 的日志条文已同步。

## 当前决策摘要

- 数据统一支持真实 FormMaking 和 `vue_custom`/NoFormFlow 自定义表单；历史候选只按目标接口原字段匹配 `flowCode` 与表单/流程名称，所有状态可选，已完成优先，无候选只提示先在目标平台发起一次并填写业务数据。
- 目标原始表单数据深复制后直接交给复制的 `form-runtime` 完整回显；只对当前路径条件字段做可证明的最小补丁，条件字段可编辑，人工改值导致换路时必须确认；数据读取、runtime 接收或路径复验失败进入 `needs_input`，无生成器和兜底。
- 条件语义严格复刻目标 Java：`lt/gt/lte/gte/eq/neq/contains/is_update/is_not_null/boolean_value`、BigDecimal、B 包含 A、顺序 `and/or`、首个命中、最后分支兜底；目标布尔矛盾不自行修正。
- 动作覆盖目标真实用户操作；保存草稿是实例生命周期，审批暂存是任务检查点；加签只在当前活动人工待办且代理可编辑时可用，系统自动节点不提供动作。
- 用户动作按独立记录排序并编译为同一主实例的用户步骤、恢复步骤和导航步骤；不可恢复顺序保存时阻止，不用快照恢复或新建主实例。
- 清空本工具数据库全部业务数据，不保留备份；保留 `schema_migrations` 和运行时维护能力；绝不触碰目标平台数据。

详细范围、任务顺序、接口、表结构、验证和人工验收见 `docs/features/F-012-history-form-replay-action-orchestration.md`。
