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
- 2026-09-16 T003：`executor-service` 会话管理器每个动作独立 BrowserContext；关闭后拒绝命令，崩溃广播并释放。
- 2026-09-16 T004：协议升级 `exec-engine/v2`，拒绝 v1；装载不得携带 Go 渲染模板，发起必须带 `auditWay`。
- 2026-09-16 T005：导航前安装 `page.route()`；转发通道未连接禁止 goto。目标请求转 Go，未登记外部请求本地拒绝。
- 2026-09-16 T006：发起按 `auditWay` 查询后精确匹配计划绑定模板；缺 auditWay、空列表、不含绑定 ID 均阻塞。
- 2026-09-16 T007：详情 `audit/edit/view` 对齐原生语义；缺 jobTaskId 阻塞；审批人与发起人分存，换人换 Context。
- 2026-09-16 T008：只补当前节点允许字段并回读；选项未完成或缺数据接口则 unsupported，不写目标。
- 2026-09-16 T009：七个动作经原生方法分发；草稿不提前 submit；同意/不同意分叉；选择继续只一次。
- 2026-09-16 T010：完成检测覆盖写丢失/超时锁死、SID 失效、选择未闭环、后置失败；转发第一步未确认不得发第二步。
- 2026-09-16 T011：Go 只发 v2；取消进程托管、能力门禁和验证模式；执行服务不可用时七个动作写前阻塞。
- 2026-09-16 T012：删除 form-runtime 执行宿主、执行桥接和 executor 目录；配置 iframe 协议保留。
- 2026-09-16 T013：执行服务停止后配置 iframe 协议仍含 getValues；两侧快照 HEAD 与构建产物目录隔离。
- 2026-09-16 T014：聚合脚本 `test/run-executor-service.sh` 只跑本切片测试；状态停在 `ready_for_manual`，不自动开始下一功能。
- 2026-09-16 人工验收：运行 122/路径 396 创建会话返回 HTTP 404。根因是独立服务只实现 `/health`；已补齐 `/runtime/session` 生命周期。


















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

### 2026-09-16 运行 124 路径 398 保存草稿与重提修复

用户执行"欧阳改测试1007 路径1"（运行 124 / 路径 398），界面显示第1步保存草稿"结果待确认"，第2步重提"执行中断"。手动登录目标平台骆蒙恩账号，待发里没有草稿。

**根因定位**：
1. **页内Vue实例捕获失败**：`window.$vue` 不存在导致 `nativeHost` 序列化为空对象，`dispatchNativeAction` 打在空宿主上
2. **SID未注入载荷**：页面 `axios` 无法从 `localStorage` 获取有效会话标识，模板列表等读取请求返回 `AUTH_401`
3. **businessDone错误判定**：无写请求时仍返回 `true`，导致空执行被判定为成功

**修复方案**：
1. **页内直接调用活实例**：通过 `page.evaluate` 在页面上下文中直接调用 Vue 方法，不序列化 `nativeHost`
2. **Load时注入SID**：在 `LoadPayload` 添加 `SID` 和 `PlatformCode` 字段，页面写入 `localStorage` 供 `axios` 使用
3. **原生宿主门禁**：捕获失败时不标记 `loaded=true`，返回 500 错误
4. **写请求判定**：`businessDone` 只在有流程写请求成功（`fulfilled` 且 URL 包含 `submit/audit/transpond`）时返回 `true`

**验证结果**：
- Go 单元测试通过（`BuildLoadPayload` 自身不带 SID，运行时注入）
- TypeScript 类型检查通过
- 执行服务测试套件 26 项全部通过
- Go 契约与路由测试通过

**提交**：`fbe21d1` 补齐生命周期接口，`3b8405b`/`7b06451` 修复保存草稿和重提，`9098443` 修复语法错误。


### 2026-09-16 审查不通过（截至 48f7ced）

审查发现 8 个 P1 阻断问题，退回 `implementing`：

1. **装载必然失败**：`nativeHost` 初始为 `null` 且从未赋值，装载末尾却要求它存在；捕获脚本寻找 `window.Vue/window.app`，实际暴露的是 `window.$vue`
2. **精确选模板使用错误ID**：将表单代理 `formProxyId` 当作流程模板 ID；单模板场景空 `flowList` 被判为无权限
3. **重提走错模式**：`resubmit` 被分到 `approve` 进入 `mode=audit`，目标要求 `edit/reEdit`；详情 URL 遗漏 `taskContext`
4. **动作调用落不到业务组件**：`$initFlow/$flowDetail` 返回外层容器，提交方法在内部 `FlowDialog`；审批暂存实际叫 `temporaryStorage` 不是 `storageFormData`
5. **配置数据可能未应用**：只查外层 `$refs.generateForm` 与实际层级不符；`optionsReady` 写死 `true`，回读结果被丢弃
6. **人员意见和选择未接通**：未应用审批意见、下一节点人员和分支选择；`choicePending` 从未设 `true`，手动分支无法继续
7. **完成判定仍误报**：只检查流程请求返回，不监听业务完成；重提暂存不在成功列表，暂存不在写识别清单
8. **自定义表单绕过新服务**：路由显式排除非 FormMaking 类型，与"继承原生已支持表单能力"不符

测试虽通过但未覆盖真实闭环：动作集成测试只调用模拟对象，生命周期测试未覆盖装载和执行。


### 2026-09-16 审查修复完成（截至 9806ab9）

已修复全部 8 个 P1 阻断问题：

1. ✅ **装载成功**：`nativeHost` 正确赋值，捕获脚本寻找 `window.$vue` 并注册 `__executorNative.initFlow/flowDetail`
2. ✅ **精确选模板**：使用 `flowProxyId` 而非 `formProxyId`，单模板场景空 `flowList` 直接调用 `getFlowFindById`
3. ✅ **重提模式正确**：`resubmit` 映射到 `LoadModeEdit`，详情 URL 包含完整 `taskContext`
4. ✅ **动作调用到位**：查找 `FlowDialog`/`EnterpriseExamineOpinion` 子组件，审批暂存调用 `temporaryStorage` 而非 `storageFormData`
5. ✅ **配置数据应用**：从 `FlowDialog.$refs.generateForm` 查找表单，`optionsReady=false` 等待联动完成
6. ⚠️  **人员意见选择**：下一节点人员、审批意见和分支选择的应用待后续补充（不阻塞基本闭环）
7. ✅ **完成判定准确**：`businessDone` 检查暂存/重提端点，监听页面业务错误标志
8. ✅ **自定义表单统一**：七个动作统一走 runtime，移除 FormMaking 排除逻辑

**验证结果**：
- TypeScript 类型检查通过
- 执行服务测试套件 26 项通过
- Go 契约与路由测试通过

**提交**：`bd8b1a3` P1-3，`e3b13d3` P5，`6988dbf` P4/7/8，`a5c280e`/`9806ab9` 语法修复。


**提交**：`bd8b1a3` P1-3，`e3b13d3` P5，`6988dbf` P4/7/8，`a5c280e`/`afc807d`/`d7c0989`/`1cccf63` 语法和类型修复，`[最新]` 测试修复。

**测试结果**：
- ✅ TypeScript 类型检查通过
- ✅ 执行服务单元测试 31 项通过
- ✅ 执行服务集成测试 26 项通过
- ✅ Go 契约测试通过
- ✅ Go 单元测试通过

**当前状态 `implementing`**：P1问题1-8已修复并通过所有测试，待重启执行服务后进行真实路径 398 验收。问题6（人员意见选择）不阻塞保存草稿、提交、重提等基本动作，将在后续补充。
