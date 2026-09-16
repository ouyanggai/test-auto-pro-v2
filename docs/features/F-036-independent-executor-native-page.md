# F-036 独立执行服务与原生页面装载

- 状态：implementing
- 产品依据：`docs/PRODUCT.md` 的「F-036 独立执行服务与原生页面装载」
- 架构依据：`docs/ARCHITECTURE.md` 的「F-036 独立执行服务」
- 计划确认时间：2026-09-16（用户批准开始实现）

- 详细设计：`docs/s2/01-执行服务设计.md`
- 任务清单：`docs/s2/02-执行任务清单.md`
- 入口对照：`docs/s2/03-原生入口与装载对照.md`

## 目标

把页面执行能力迁到根目录 `executor-service/`，使用独立的 `rsh-flow-components` 副本，按参考项目原生路由、身份和流程上下文装载页面。表单配置继续负责数据显示、编辑和数据取回；七个表单动作通过新服务执行。两者独立构建、更新和启动；执行服务停止时配置页面仍可使用。

## 范围

### 包含

- 迁出现有 Node/Chromium 执行服务，建立独立页面构建包和组件源码快照。
- 初始快照：`make refs-sync` 后复制已核实的 `test` 分支 `be355ae`；不复用 `form-runtime` 副本。
- 必需 FormMaking 库产物在执行服务内独立构建；参考源码、依赖和产物不提交。
- 实例经 `/flow/detail/:id` 的 `audit/edit/view` 装载；发起经 `/flow/initiate` 装载。
- 发起按 `auditWay` 查询并精确选中计划绑定模板。
- 薄桥接捕获 `$initFlow`、`$flowDetail` 实例，调用页面业务方法。
- 原生页面自行读模板、实例数据和字段权限；完成后只补当前节点允许的配置值，等待选项及联动再回读。
- 继承原生已支持的 FormMaking、自定义组件和 NoFormFlow；缺入口或数据接口时阻塞。
- 删除 form-runtime 内执行路由、执行宿主和执行桥接；保留配置侧协议及数据功能。
- 协议升级 `exec-engine/v2`；生命周期接口保持。
- 七个动作接入新服务；其余动作继续 Go 执行。
- 页面请求导航前拦截，经 Go 转发；转发通道装载前必须连通。
- 取消能力登记门禁和验证模式。
- 服务单独启动，默认端口 `19011`，沿用 `RUNTIME_EXECUTOR_URL`；Go 不再托管进程。
- 返回执行前后数据、请求轨迹和组件版本；不新增数据库迁移。
- 测试按单元、契约、集成保存到 `test/`，只跑本切片检查。

### 不包含

- 新增目标业务能力或未支持的自定义组件实现。
- 扩展加签、移交、回退、取回、撤回、催办、关注等其余审批动作。
- 改造整套 Go 执行引擎、调度、核验或运行记录模型。
- 修改目标平台源码、权限或数据库。
- 配置版本管理、用户侧对账入口、新的数据库表。
- 把配置 iframe 改成原生 `/flow` 页面。
- 自动开始 F-027 或其他后续切片。

## 完成标准

- [ ] `executor-service/` 可独立构建、启动和健康检查；默认 `127.0.0.1:19011`。
- [ ] 组件快照 HEAD 为 `be355ae`，与 `form-runtime/runtime-source` 目录和构建产物隔离。
- [ ] 发起装载走 `/flow/initiate`，按 `auditWay` 精确选中计划绑定模板。
- [ ] 实例装载走 `/flow/detail/:id` 的 `audit/edit/view`，身份与 `taskContext` 对齐参考项目。
- [ ] 装载完成后只补当前节点允许字段，选项和联动结束后回读成功。
- [ ] 七个动作通过原生组件方法执行；选择继续只允许一次；无重复写请求。
- [ ] `exec-engine/v2` 拒绝 v1；装载载荷不含 Go 渲染模板。
- [ ] 转发通道未连接时不导航；写响应丢失进入结果待确认，不重发、不回退 Go 写路径。
- [ ] 能力登记不再作为执行门禁；验证模式删除。
- [ ] Go 不再托管执行进程；服务不可用时只阻塞七个 runtime 动作。
- [ ] `form-runtime` 删除执行宿主和执行路由后，配置表单仍能显示、编辑和取回数据。
- [ ] 停止执行服务后配置功能可用；独立更新一侧快照不改变另一侧。
- [ ] `bash test/run-executor-service.sh` 通过；文档状态为 `ready_for_manual`。
- [ ] 已列出用户手工核对步骤。

## 实施任务

按 `docs/s2/02-执行任务清单.md` T001–T014 顺序实施。每项验证通过后中文原子提交。

规划时本机 `rsh-flow-components` `test` 为 `24f3a28`，未包含 `be355ae`。T002 必须先安全同步再核对提交；缺失则阻塞整个切片，不得改用其他 HEAD。

## 测试任务

- `test/unit/executor-service/`：会话、协议 v2、拦截、完成检测。
- `test/unit/backend/runtimeexec/`：v2 客户端、装载载荷、路由、无托管、服务不可用阻塞。
- `test/contracts/executor-service/`：协议夹具、动作入口、无重复写、无执行宿主残留。
- `test/integration/executor-service/`：无头 Chromium 原生装载、精确选模板、身份切换、数据回读、七个动作闭环、失败路径、服务隔离。
- `bash test/run-executor-service.sh` 只聚合本切片检查。
- 真实目标可见页面和写动作由用户人工验收，不自动启动可见浏览器。

## 状态记录

正常状态按 `preparing -> awaiting_approval -> implementing -> ready_for_manual -> accepted` 推进。

- 2026-09-16：用户要求在 `docs/s2` 按 `docs/s` 结构规划，将页面执行迁到 `executor-service/`，使用独立 `rsh-flow-components` 副本和原生路由装载。本文件从 preparing 进入 `awaiting_approval`。未获明确批准不得写实现代码。
- 2026-09-16：用户批准开始实现，状态进入 `implementing`。规划文档已提交 `ee519c3`。
- 2026-09-16 T001：根目录 `executor-service/` 骨架可启动无头 Chromium 并响应 `/health`；workspace 改为独立包；Go 托管路径改为 `executor-service`。
- 2026-09-16 T002：独立快照来自原项目 `/Volumes/oygsky/bigsys/rsh-flow-components` 的 `test`/`be355ae`（含 FormMaking dist，不自行编译）；不复用 `form-runtime/runtime-source`。后续一键升级与原项目同一套 `sync-manifest`。





上一轮 `docs/s` 执行引擎改造仍停在 `ready_for_manual`；本切片批准后替换其执行宿主方案，不兼容 v1 协议。

## 人工验收

1. 只启动后端和 `pnpm dev:form`，不启动执行服务：打开计划路径数据页，表单能显示、编辑并取回数据。
2. 启动执行服务健康检查通过后，再启动一次运行：发起页按计划绑定模板打开，不是同 `auditWay` 的其他模板。
3. 保存草稿、提交、审批暂存、同意、不同意、重提、转发在原生允许场景下各走一遍；目标实例/任务/表单与人工操作一致。
4. 切换处理人账号后，页面身份和请求 SID 属于当前操作人，不残留上一账号。
5. 人为断开执行服务后再放行七个动作之一：运行被阻止，配置页仍可用，目标无新的写请求。
6. 装载失败、校验失败、选择继续缺配置、SID 失效、写响应丢失、超时、后置失败：分类正确，network 日志无重复主写。
7. 独立改执行快照一个无关文件并重建执行服务：配置 iframe 行为不变；反向改配置快照，执行服务 HEAD 元数据不变。
8. 系统设置「表单运行时维护」只更新配置服务，不改 `executor-service` 快照。
9. 可见页面和真实目标动作由你在浏览器核对；没有你的明确确认，不得标记 `accepted`。
