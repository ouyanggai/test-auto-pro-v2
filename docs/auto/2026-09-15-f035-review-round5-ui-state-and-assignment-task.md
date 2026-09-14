# F-035 第五轮复核与界面、有效数据、处理人修复任务书

编写日期：2026-09-15
当前状态：`implementing`
适用对象：负责 F-035 实施的 Agent
工作目录：`/Volumes/oygsky/AIstudy/test-auto-pro-v2`

## 0. 先给结论

本轮评审不能通过，F-035 不得进入 `ready_for_manual`。最近三个提交（`16a99c2`、`78753e0`、`e417a2c`）虽然补充了特殊页面、无表单页面和运行详情生命周期代码，但人工验收仍证明有真实业务请求没有按目标页面发送，界面配置模型也没有把系统动作和用户动作分开。

最直接的证据是路径 12 的运行日志：右侧提示已经记录“请假类别：事假 → 产假”，但是最终 `submit` 载荷仍是 `vacateType=事假`、`vacateType__virtualName=事假`；同一载荷中的 `myDutyName=行政专员`，而用户提供的目标平台成功 curl 使用的是 `myDutyName=工程经理`。这不是目标平台偶发没有处理人，而是工具发出的有效业务数据和身份字段与目标页面不一致。目标平台根据实际表单条件没有生成处理人，工具随后只轮询并卡住。

本任务只要求在工具侧修复和验证，不修改目标平台源码、目标数据、流程配置或权限。流程实例名称仍可使用工具侧命名规则，这是唯一豁免；其余业务字段、身份、代理 ID、批次号、空值形状、特殊业务顺序和处理人事实必须逐代码对齐目标 `GroupApproveManage` 及其直接引用公共组件。不能用“目标后端可能忽略”“请求已返回 200”或通用阻塞替代实现。

## 1. 完成标准（全部满足才可停在 ready_for_manual）

- [ ] 下拉框路径补丁会改变控件实际绑定值、虚拟显示值和目标要求的名称伴生字段；界面、`getValues`、最终请求和日志四处显示同一个修正后的值。静态下拉、远程下拉、级联、多选、子表单和 ID/Name 成对字段均有测试。
- [ ] 运行号 9、路径 12 重新运行时，工具 `submit` 与用户提供的成功 curl 逐字段、逐层级、逐存在性一致。允许的差异只有环境地址、SID、批次号随机值和流程名称；业务数据和身份字段不能差异。
- [ ] 计划账号是变量，不得写死骆蒙恩；表单身份、当前处理人会话、候选下一节点处理人分别取各自实时事实。表单构造发生在正确会话切换之后。
- [ ] “同意”“提交”“重新提交”等固定尾动作只出现一次并始终位于动作列表最后；用户动作不能保存这些固定尾动作的重复记录。保存前端和后端都阻止尾动作被插入、移动、删除或改成普通动作。
- [ ] 所有动作用户界面只显示中文白名单；不再出现 `approve`、`submit` 等英文稳定键，也不出现英文键与中文默认动作重复展示。
- [ ] 添加动作后有明确可见的“删除”按钮；新增动作可以在未保存前删除，保存后重新打开仍能正确删除；系统固定尾动作显示为只读，不提供删除入口。
- [ ] 当前节点缺少配置时，提示直接说明要操作的项目、人员名称、需要几人或需要哪一种动作；不得只显示“缺少 N 项，请在本面板完成”。提示能定位或滚动到对应区块。
- [ ] 详情、运行列表、路径列表使用同一个服务端状态投影。沿现行架构保留数据库技术状态，使用 `stopKind=blocked`/`assignment_missing` 的统一 DTO 投影；路径或运行阻塞后，三个位置同时显示“阻塞”，不再一个显示“阻塞”、两个显示“运行中”。处理人轮询超时、工作进程中断、租约过期都必须写入终态并停止前端轮询。
- [ ] 目标审批生成的 `auto_audit_info_*`、对象和列表字段按目标衍生字段处理：历史空值不能覆盖目标新意见，目标真实审批人和时间变化不阻塞；工具删除或清空目标已有非空值仍阻塞。
- [ ] `form_person`、特殊 FormMaking 业务、NoFormFlow、请求协议矩阵、写后核对和运行详情异步竞态的上一轮缺陷均完成代码、契约测试和日志证据；不能只登记为“已实现”或“安全阻塞”。
- [ ] `runtime.lastError` 完成无扩展 A/B 归因；Vue `emitsOptions` 完成真实浏览器操作验证。不得通过吞掉控制台错误或修改 Vue runtime 伪造通过。

## 2. 本轮评审发现（按严重程度）

### Critical 1：下拉框只改了提示/虚拟名称，没有改真实绑定值

证据：

- `form-runtime/src/runtime/formTemplate.js:364-385` 的 `triggeredNameSource` 只识别名称字段和 `__virtualName`，不识别补丁直接命中的模型字段（例如 `vacateType`）。
- `evaluateOptionPatches`（同文件 `:556-626`）只有发现名称字段触发时才解析选项并回填真实值，因此 `vacateType` 的路径补丁不会执行选项映射。
- `web/src/features/path-configuration/FormRuntimeFrame.vue:146` 把 `branchPatches.map(patch => patch.path)` 传给运行时，当前路径补丁正是直接模型路径。
- 运行日志 `logs/runs/运行_9__欧阳改测试1005__run-100/paths/路径 12__path-run-187__path-4221/step.log:3-4` 显示路径调整涉及 `vacateType`，但 `finalPayload` 仍为 `vacateType=事假` 和 `vacateType__virtualName=事假`。
- 同目录 `curl.log.1:62` 的真实提交也保留了“事假”，证明界面提示和实际请求已经分离。

这直接解释了“目标没有当前处理人”：目标分支依据真实模型值计算，工具没有把 `vacateType` 改成目标路径要求的值，目标没有走出与手工成功请求相同的后续节点。不能通过硬编码处理人解决。

#### 修复要求

1. 在运行时补丁模型中增加明确的触发来源：`model`、`name`、`virtual`、`both`，不要把所有补丁都当成显示名补丁。嵌套路径、子表单行路径必须保留原路径信息。
2. 对直接模型补丁：先读取组件描述符和真实选项；补丁值若能按 `value` 唯一匹配则使用该值，若补丁值是目标显示名则按 `label` 唯一匹配并取得对应 `value`。找不到或重名时阻塞，不能猜测。
3. 对每次解析同时同步目标实际需要的 `model`、`__virtualName`、`Id/Name` 伴生字段；`setData` 写入宿主 `editData` 和 FormMaking 实例，等待 `onChange`、远程选项和链式请求全部结束，再用 `getValues` 做最终确认。
4. `optionCoordinationIssues` 与 `coordinateOptionPatches` 必须使用同一解析结果；不能出现右侧提示已是新值但 iframe、保存数据或请求仍是旧值。
5. 在日志中分别记录“补丁意图”“控件最终模型值”“最终请求值”，出现三者不一致时在发送前阻塞并给出字段中文名。

必测：静态单选、远程单选、级联、多选、子表单、直接模型路径、`Id/Name` 成对字段、名称与虚拟名称矛盾、选项为空、同名选项两条、远程响应晚到覆盖新值。

### Critical 2：路径 12 仍发送了错误的业务数据和身份，处理人缺失是工具请求不一致的结果

运行号 9 / 路径 12 的 `step.log:4` 记录：

- `vacateType=事假`，而页面路径提示期望“产假”；
- `myDutyName` 是 `行政专员`；
- `global_user_basic_information.dutyName` 是 `工程经理`，同一请求内部身份已自相矛盾。

用户提供的目标平台手工成功 curl 中，计划发起人的表单数据使用 `myDutyName=工程经理`，且 `global_user_basic_information` 同样是工程经理。工具请求因此不能被称为“与目标请求一致”。`submit` 返回 HTTP 200/`isSuccess=true` 只代表请求被接收，不代表目标按相同业务条件生成了下一节点处理人。

#### 修复要求

1. 先修复上一项有效下拉值，再对运行号 9 / 路径 12 生成工具 curl 和手工 curl 的规范化 diff。比较请求方法、URL、请求头、顶层字段、`data`、`formDataMongoVo.data`、`nextAuditorList`、`batchCode`、`projectId`、代理 ID 和每个业务字段的存在性与类型。
2. 计划账号、发起人表单身份、审批当前处理人和下一节点候选人分别建事实对象；任何一个对象不得从历史表单或另一个对象复制。
3. 读取当前会话目录树和岗位后，再构造 FormMaking 表单和 `global_user_basic_information`。`myDutyName`、`myUserName`、`myDepName` 等字段若属于目标页面实际发送集合，必须使用该会话事实；字段不属于目标页面时也不得用旧值冒充当前身份。
4. 发送前增加“工具请求 vs 目标 fixture”契约检查；除名称、SID、环境地址、随机批次号外存在任何业务差异即阻塞并列出字段路径。不能先发送再解释为什么目标没有处理人。
5. 请求成功后必须读取同一实例的 `currentNodeProxyId`、`currentAuditUserInfo` 和精确待办。目标节点无处理人时最多按既有预算只读轮询；超时后落“阻塞/assignment_missing”，不能用计划账号或候选人代替。

### Critical 3：固定尾动作被重复建模，保存没有阻止“同意不在最后”

证据：

- `web/src/features/path-configuration/ActionOrchestrationEditor.vue:40` 把已保存节点动作和实例动作合并，`:121` 又把两种作用域压成一份 `actionDraft`。
- `:207-215` 先渲染保存动作，随后再单独渲染 `container.actionConfiguration.base`，所以保存记录中出现 `approve` 时会显示英文/中文重复的“同意”。`:209` 的 `|| item.action.kind` 还会直接把稳定英文键显示出来。
- `web/src/features/path-configuration/logic.ts:239-249` 的 `validPathConfigActions` 只检查目录和人员，没有检查固定尾动作唯一性、位置和作用域。
- `internal/service/path_action_configuration.go:624-688` 的 `mergeNodeActions` 只检查节点归属和顺序唯一，没有拒绝用户提交 `approve`、`submit`、`resubmit` 作为普通用户动作。

#### 修复要求

1. 数据模型中把固定尾动作作为独立只读字段（`base`），用户动作数组不得包含 `submit`、`approve`、`resubmit`。读取、保存和编译时都执行一次规范化；当前数据若包含尾动作，按稳定键确认其是否为旧自动占位记录，直接从当前模型移除并重新保存，不靠标签猜测。
2. 根据节点类型生成唯一固定尾动作：发起节点为“提交”，审批/协同节点为“同意”，驳回/撤回后的恢复链由编译器生成“重新提交”。固定尾动作永远排在最后，不能拖动、编辑、删除或重复添加。
3. 前端保存前显示具体错误，例如“同意必须是最后一步，请删除动作列表中的重复同意”；后端 `SaveActionConfiguration` 也必须拒绝同样的正文，不能只依赖浏览器校验。
4. 不要在 `openEditor` 和 `savedSummary` 中混合节点级、实例级动作。分别维护两个草稿和两个作用域的顺序，保存时只替换对应容器。
5. 引入统一 `actionLabel`（当前 `web/src/features/runs/presentation.ts:4-31` 已有白名单）供目录、编辑器、流程图、错误和运行详情调用；未知键显示“未识别动作（请查看日志）”，禁止原始英文回退。
6. 添加动作改为显式选择安全候选，不得静默取 `enabledCatalog[0]`。新增动作后在每一行显示文字按钮“删除”，固定尾动作只读显示。删除动作后同步清理重复次数和人员策略，保存/取消行为要明确。

必测：发起节点提交重复、审批节点同意重复、同意拖到中间、同意删除、实例/节点动作混排、添加后删除、重新打开后删除、未知动作键、所有动作界面中文。

### High 1：缺少配置提示仍是泛化计数

`web/src/features/path-configuration/NodeConfigurationPanel.vue:10` 只接收 `missingCount`，`:110` 固定显示“当前节点仍缺少 N 项配置，请在本面板处理人员或节点动作区完成”。父组件 `web/src/views/PlanPathConfigurationView.vue:238` 已经有 `currentNodeConfigurationComplete(...).missing`，但 `:1060` 只传数量，丢失了人员名称和动作规则。

#### 修复要求

将 `missing` 改成结构化项目并传入面板，例如：

```text
{ kind: "person", key: "departmentLeader", title: "部门领导", requiredCount: 1 }
{ kind: "action", title: "节点动作", rule: "至少添加 1 个可执行动作；同意必须放最后" }
```

前端分别渲染：

- “请在‘处理人员’中为‘部门领导’选择 1 名人员”；
- “请在‘节点动作’中添加 1 个可执行动作；固定的‘同意’必须是最后一步”；
- 候选目录为空时说明“目标平台没有返回可用候选，不能自动配置”，不要叫用户继续点击。

保存失败也复用同一结构化项目，并提供 `data-testid` 和滚动定位到人员选择器/动作区。测试不能只断言数量，必须断言最终中文指令。

### High 2：详情已阻塞但列表仍显示运行中，终态事实没有统一落账

现状证据：

- `internal/model/run.go:8-18` 的运行状态没有 `blocked`；`PathRunStatus`（`:41-60`）也没有阻塞状态。
- `internal/engine/step/executor.go:624-658` 的处理人轮询只有在完整执行链返回后才可能进入失败落账；最新日志 `step.log:66-69` 只到第 3/5 次轮询，没有第 5 次、阻塞记录或完成记录。
- `executor.go:937-962` 只把已经构造出的门禁阻塞写成失败；工作进程中断、租约失效或轮询协程消失时，数据库仍可能保留 `running`。
- `internal/service/run_records_view.go:52-101`、`:104-182` 分别直接读取 `runs.status` 和 `path_runs.status`，没有同一状态投影；`web/src/views/RunPathsView.vue:55-64` 按中文终态列表停止轮询，状态不落账会一直显示运行中。

#### 修复要求

1. 沿现行架构保留数据库 `failed` 技术状态，不新增数据库状态枚举或表；所有列表、详情 DTO 统一输出机器字段 `status=failed`、`statusName=阻塞`、`stopKind=assignment_missing`（或其他已登记阻塞分类），前端不得自行从中文文本推导。
2. 处理人轮询预算耗尽、写前门禁阻塞、特殊业务未实现、写后核对失败时，在同一事务中写 `RunStep`、`RunStepAttempt`、`path_runs` 终态并调用运行聚合；释放执行租约，写入明确原因和最后一次事实。
3. 增加启动恢复/租约清理任务：发现没有活动 worker 且超过租约期限的 `running` 路径，按“阻塞/执行器中断”落账，不能永久运行中。
4. `ListAllRuns`、`RunPaths`、详情接口共享一个状态投影函数，返回机器状态、中文名称、停止分类、原因和最后一步；计划列表、运行列表、路径列表、运行详情全部使用该 DTO。
5. 前端轮询按机器终态停止，阻塞页显示“第几步、哪个节点、为什么阻塞、下一步怎么处理”，不再使用“运行中”占位。

必测：处理人轮询超时、worker 在第 3 次轮询退出、租约过期恢复、多个路径一条阻塞一条完成、详情/运行列表/路径列表同时读取、前端轮询自动停止。

### High 3：写后 `auto_audit_info_*` 的核对语义必须保留目标新值

目标页面会按真实审批记录生成 `auto_audit_info_<n>`、`auto_audit_info_obj_<n>` 和 `auto_audit_info_obj_list_<n>`。用户给出的“期望空、实际刘志阳同意时间”是目标正常生成的审批事实，不能回写空字符串，也不能判定为工具改写。

当前代码在配置阶段通过 `internal/service/path_data_workspace.go:153-169` 和 `internal/service/history_replay.go:524-526` 清空历史审批意见，这是正确的发起态行为；但运行时 `internal/engine/step/formdata.go:149-189`、`executor.go:554-613` 必须确保审批后的目标值成为下一节点基线。所有对象/列表伴生字段也要覆盖，不能只对一个文本前缀做例外。

#### 修复要求

沿用单一字段所有权登记：

- `tool_owned`：本节点明确覆盖，目标保存后必须等于实际发送值；
- `preserved_business`：上游业务字段，丢失或变化阻塞；
- `target_derived`：目标根据审批事实自动生成，目标当前值权威；新增或格式化变化正常，工具删除、清空、回写历史值才阻塞；
- `node_owned_future`：后续节点字段，当前请求不得提前发送。

写后核对要把目标当前衍生值落入持久化事实，下一步直接读取该事实；不能拿配置阶段空值与目标审批新值做深度相等比较。基线字段缺失也必须按所有权报告，不能无条件 `continue`。

### High 4：上一轮 `form_person`、身份、协议矩阵和写后事实缺陷仍需逐项闭环

以下项目在 `docs/auto/2026-09-14-f035-review-and-runtime-repair-task.md` 中已经登记，本轮仍作为阻塞项，不接受只补测试名称或只返回阻塞：

1. `form_person` 必须区分新发起/草稿/重提的目标全树遍历和审批当前下一节点入口遍历；`NodeID` 必须参与过滤，值提取按目标页面的 `split('__')[0]` 或 `replace('__formPersonId','')` 规则分别实现。
2. 当前实际处理人只取目标实例最新 `currentAuditUserInfo`/精确待办。候选人是下一节点选人，不是当前处理人；禁止回退计划账号。
3. 真实身份必须在表单构造前切换会话并读取用户、公司、部门、岗位；不能要求历史表单先存在 `global_user_basic_information` 才注入，也不能保留旧岗位字段。
4. `ValidateBodyMatrix` 必须真实表达 required/optional/empty/forbidden；禁止用 `true` 伪造 `sid`、`projectId`、`customerCode` 的存在性，矩阵必须覆盖每一个实际写出口。
5. 写后核对结果要与步骤尝试、事实快照和路径状态同事务持久化，重启后下一步仍能读到阻塞原因。
6. 特殊业务和 NoFormFlow 的登记状态必须有目标源码请求 fixture 支撑。`internal/adapter/target/special_business.go:47-156` 把多种链路标成 `implemented`，而 `special_business_write.go:74-168` 仍有通用/猜测型 `{data: values}` 构造；没有完整的 Controller/Service/VO、参数顺序和响应 ID 证据时必须保持未实现并写前阻塞，不能以“专用接口已登记”宣称完成。
7. `test/contracts/f035/special_business/special_business_contract_test.go` 不能复制同一份登记列表作为期望；测试必须读取版本化目标源码符号/fixture，能发现新增、遗漏和顺序漂移。
8. 特殊业务静态阻塞必须在读取表单、轮询和其他只读请求之前判断，避免明知不能发送仍等待十秒。

### High 5：Vue 生命周期修复仍缺少浏览器证据，不能把两个控制台错误混为一谈

#### `Unchecked runtime.lastError`

全项目源码搜索未发现 `chrome.runtime`、`browser.runtime`、`runtime.sendMessage` 或 `runtime.lastError`。该错误符合 Chromium 扩展向不存在 receiving end 发消息且未读取 `lastError` 的行为，最可能来自 Vue Devtools、密码管理器或自动化扩展，不是业务接口错误。

Agent 必须做一次无扩展窗口 A/B：

1. 在无扩展窗口执行相同的路径切换、节点详情和放行操作。
2. 若错误消失，记录控制台源链接的 `chrome-extension://扩展 ID`、扩展名称和发生时机，产品代码零修改。
3. 若仍出现，点击源链接确认是否为项目 URL；只有拿到完整调用栈和消息发送端后才能改代码。
4. 禁止用 `window.onerror`、全局 console 过滤、Promise 吞错或 Vue runtime 补丁掩盖它。

#### `emitsOptions`

当前 Vue 版本为 3.5.40，错误行在 `runtime-core.esm-bundler.js:4794-4798`，说明旧 VNode 的 `component` 在更新时为空。产品中仍需验证的竞态包括：

- `RunDetailView.vue:756-771` 对始终存在的 `#app-header-context` 使用了条件 `Teleport defer`；
- `/runs/:runId/paths/:pathRunId` 路由复用时，详情、事件、流程图、放行和轮询响应可能乱序到达；
- `RunNodePanel` 在条件卸载时内部含多个 `NModal` Teleport/Transition；
- 卸载只能清 timer，必须同时使在途 Promise 失效。

上一轮 `78753e0` 已加入部分代次/Abort 逻辑，但本轮必须确认所有 API、事件游标和组件 key 都覆盖，且保存真实堆栈。

#### 修复要求

- 移除目标节点已在祖先稳定渲染的 Teleport `defer`。
- 详情、事件、流程图请求统一使用 `requestVersion + AbortController + disposed`；响应写入前核对 runId、pathRunId 和代次。
- 路径切换先关闭节点面板和 Modal，清空该路径事件游标；`RunNodePanel` 使用路径运行 ID + 节点键稳定 key。
- 所有 `void` Promise 在函数内部捕获非取消错误；AbortError 不进入错误提示，但其他错误必须进入可读区域和日志。
- 没有新快照时保留同一代次最后一个有效当前节点，不能用旧代次响应覆盖新路径。
- 不修改 Vue runtime 源码，不用 null guard 掩盖损坏的 VNode 生命周期。

## 3. 实施顺序（必须按顺序，不要绕过前置事实）

### 阶段 A：冻结证据和请求对照

1. 保存运行号 9 / 路径 12 的原始 `step.log`、`curl.log`、`network.log`，生成规范化工具请求 fixture。
2. 将用户手工成功 curl 保存为测试 fixture（SID、随机批次号和地址参数化），对 `data` 与 `formDataMongoVo.data` 做逐字段 diff。
3. 先确认下拉补丁最终模型值和身份字段，再允许重新发起目标写请求；对照前禁止反复污染目标流程。

### 阶段 B：修复 FormMaking 有效数据

1. 实现直接模型路径和显示名路径的统一补丁解析器。
2. 宿主 `editData`、iframe FormMaking 实例、配置保存接口和执行器 `BuildNodeFormData` 只接受同一份最终值。
3. 保存前做最终 `getValues` 与修正提示的逐字段一致性检查。
4. 补齐运行时单测和实际 iframe 集成测试，测试文件放在 `test/unit/frontend/form_runtime/` 或 `test/integration/f035/`。

### 阶段 C：修复动作模型和编辑器

1. 规范化固定尾动作与用户动作的数据边界，分离节点/实例作用域。
2. 统一中文动作标签、添加候选选择、可见删除按钮和保存校验。
3. 前后端同时拒绝尾动作重复、非尾位置和错误作用域。
4. 补齐 `test/unit/frontend/path_configuration/` 组件/逻辑测试，以及 `test/contracts/f035/actions/` 后端契约测试。

### 阶段 D：修复结构化配置提示和阻塞状态

1. 把 `currentNodeConfigurationComplete` 的返回值改为结构化缺口，父子组件传递完整项目。
2. 建立 `blocked/assignment_missing` 机器状态或统一阻塞投影，完成轮询超时、worker 中断和租约恢复落账。
3. 统一三个列表/详情 API DTO，前端按机器状态停止轮询。
4. 补齐 `test/unit/backend/run_status/`、`test/unit/frontend/run_status/` 和恢复场景集成测试。

### 阶段 E：完成 F-035 旧缺陷和真实路径复验

1. 按上一轮任务修复 `form_person`、身份切换、协议矩阵、写后核对、衍生字段和特殊业务/NoFormFlow。
2. 每个特殊分支以目标源码 request fixture、响应 fixture 和顺序断言为准；未勘定分支保持写前阻塞。
3. 用新的有效表单数据重新运行路径 12，确认目标实例的 `currentAuditUserInfo`、待办 `jobTaskId`、节点 ID 与工具事实一致。
4. 只有目标请求对照、处理人事实、分节点表单和状态列表全部通过，才可进入人工验收。

### 阶段 F：前端控制台验证和交付

1. 无扩展 A/B 验证 `runtime.lastError`。
2. 真实浏览器快速 A→B→A 切换、打开三个 Modal、放行后立即切换、离开页面，保存 Console 和 Network 证据。
3. 运行所有定向测试、类型检查和 `git diff --check`；测试必须保存在根目录 `test/` 分类目录。
4. 每个原子成果用中文提交；本功能在用户人工验收前始终保持 `implementing`。

## 4. 日志和可观测性要求

- 日志目录继续按页面层级使用 `logs/runs/运行_<运行号>__<计划名>__run-<runId>/paths/<路径名>__path-run-<pathRunId>__path-<pathId>/`，日志名称与页面一一对应。
- `step.log` 必须同时记录：补丁意图、表单最终值指纹、发送字段摘要、当前会话用户/部门/岗位、当前处理人事实、目标节点、动作中文名、请求耗时、响应 `code/message/isSuccess`、阻塞分类和下一步建议。
- `network.log`/`curl.log` 按用户要求原样保留完整请求和响应（含 SID、表单正文），不做脱敏；不得把日志内容作为业务逻辑的唯一事实来源。
- 处理人缺失时记录最后一次实例节点、`currentAuditUserInfo`、待办列表、查询视角和轮询次数；不能只写“找不到当前处理人”。
- 下拉值不一致时记录字段路径、补丁前值、补丁目标值、控件最终模型值和最终请求值；任何一项为空或不一致都在发送前阻塞。

## 5. 禁止实现清单

- 禁止把“产假”或任何用户值硬编码到 `vacateType`，也禁止把骆蒙恩、刘志阳、工程经理等测试事实写成常量。
- 禁止用 `__virtualName` 或右侧提示值代替控件真实绑定值。
- 禁止把候选人当成当前节点处理人，禁止处理人缺失时回退计划账号。
- 禁止在用户动作数组中再次保存固定尾动作，禁止用标签文本猜测旧动作来源。
- 禁止在动作编辑器中用英文稳定键作最终用户文案，禁止静默选择第一个候选动作。
- 禁止只传缺口数量、只显示“请在本面板处理”、只显示“运行中”或只依赖中文状态判断终态。
- 禁止用通用 `{data: values}` 顶替未逐代码确认的特殊业务请求；未确认的分支必须明确阻塞。
- 禁止将目标自动生成的审批意见清空、回写历史空值或作为普通工具覆盖字段核对。
- 禁止吞掉 `runtime.lastError`/Promise 异常、修改 Vue runtime 或增加全局 console 过滤。
- 禁止修改目标平台源码、目标数据库、流程名称规则之外的业务数据，也禁止提交凭证、日志、构建产物和 `package-lock.json` 等本任务外文件。

## 6. 验证命令和交付报告

至少执行并记录：

```bash
go test ./test/unit/... ./test/contracts/f035/...
pnpm --dir web run typecheck
pnpm --dir form-runtime run typecheck
node --experimental-strip-types --test test/unit/frontend/form_runtime/* test/unit/frontend/path_configuration/* test/unit/frontend/run_detail_lifecycle/*
bash test/contracts/f035/drift/protocol_symbols_drift.sh
git diff --check
```

若某条命令因本机环境失败，要记录完整命令、错误和与本次改动的关系，不能删掉测试或声称通过。交付报告必须包含：

1. 修改文件和每个缺陷对应的代码位置；
2. 工具请求与手工 curl 的规范化 diff；
3. 路径 12 目标实例处理人/待办事实；
4. 下拉框界面值、最终表单值和请求值截图或日志；
5. 动作编辑器添加、删除、尾动作校验和中文标签证据；
6. 阻塞状态在详情、运行列表、路径列表的一致性证据；
7. 自动测试输出和无扩展 A/B、Vue 控制台人工验证结果；
8. 明确声明 F-035 仍为 `implementing`，没有代替用户进行最终验收。
