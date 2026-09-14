# F-035 复核聚合与运行时故障修复任务书

编写日期：2026-09-14  
当前状态：`implementing`   
适用对象：负责 F-035 实施的 Agent  
工作目录：`/Volumes/oygsky/AIstudy/test-auto-pro-v2`

## 0. 任务结论

本任务书聚合上一轮 F-035 评审缺陷、本轮运行号 6/7「欧阳改测试1005 / 路径 12」的请求差异，以及运行详情页发现的两条控制台错误。

当前实现不能进入人工验收。原因不是单一的“目标平台偶发没有处理人”，而是工具侧在代理 ID、表单身份、请求字段形状、分节点数据和目标页面特殊业务链路上仍存在偏差；同时运行详情页存在异步响应覆盖路径和条件卸载组件的生命周期竞态。此前将特殊业务和 `vue_custom` 统一“命中即阻塞”只能作为临时安全措施，不能作为 F-035 的最终完成标准。用户已经明确要求：目标平台前端发起、审批以及无表单流程中的特殊逻辑，工具侧必须逐代码对齐实现；唯一豁免是流程实例名称继续使用工具侧命名规则。

本轮只实施本任务书列出的 F-035 整改，不修改目标平台源码、权限、组织架构、流程配置、数据库结构或流程名称规则之外的业务语义。所有请求参数、空值形状、业务数据处理顺序和特殊分支必须以目标 `GroupApproveManage` 前端及其直接引用公共组件、对应 Controller/Service/VO 为唯一依据，不得因为目标代码“看起来不合理”而自行简化。

## 1. 完成标准

以下条件全部满足，才能把 F-035 从 `implementing` 推进到 `ready_for_manual`；人工验收通过前不得标记 `accepted`：

- [ ] 运行号 6/7「欧阳改测试1005 / 路径 12」的新运行中，FormMaking `submit` 请求与手工成功请求在代理 ID、身份、业务关联、`batchCode`、空 `nextAuditorList`、`projectId`、请求头、表单容器和字段类型上逐项一致；允许的差异只有环境地址和流程名称。
- [ ] 当前计划配置的发起账号是唯一身份来源。账号更换后，用户、公司、部门、岗位、`global_user_basic_information`、`my*` 字段及伴生键全部随当前会话变化；历史样本、上一次运行账号和候选处理人不能混入本次请求。
- [ ] 每个节点的表单请求都在该节点、该动作、该实例时机读取并构造。发起不能提前写后续节点字段；后续节点必须保留上游已写数据；审批、暂存、重提使用目标页面要求的完整表单容器和当前实例基线。
- [ ] `auto_audit_info_*` 及其对象/列表伴生字段按目标衍生字段处理：发起时清空历史意见；审批后以目标当前值为权威；后续写入不得回写历史空值；写后核对不得把目标正常生成的意见判为工具改写。
- [ ] `form_person` 字段的收集和注入按动作分别对齐目标页面：新发起/重提使用目标发起页实际的遍历范围，审批只处理当前下一节点及其直接条件/并行入口，不把整棵树的字段一次性写入每个节点。
- [ ] 已知 FormMaking 特殊业务分支和 `vue_custom`/NoFormFlow 专用链路完成逐页面实现；不能以通用 `submit` 顶替，也不能以“命中即阻塞”宣称完成。无法从源码证明的新增分支必须登记为未实现并在写前阻塞。
- [ ] 当前节点处理人只来自目标实例最新 `currentAuditUserInfo` 或精确待办事实。节点到达但目标尚未生成处理人时有限等待，超时状态为“阻塞”，不得使用计划账号、发起人或配置候选人回退。
- [ ] 写后表单核对结果持久化到运行事实并能被下一节点读取；字段丢失、工具覆盖值未保存、工具删除目标已有非空值都必须阻塞，目标后端正常生成的衍生意见变化不阻塞。
- [ ] 请求协议矩阵真实表达 `required`、`optional`、`empty`、`forbidden`；不得用 `true` 伪造字段存在，也不得把 `omitempty` 当目标协议。未登记端点禁止发送。
- [ ] 运行详情快速切换路径、打开节点面板/Modal、轮询、放行、离开页面时不再出现 `emitsOptions` 崩溃或未处理 Promise；旧路径响应不能覆盖新路径。
- [ ] `Unchecked runtime.lastError` 已通过无扩展 A/B 验证归因。若来源确实为 `chrome-extension://`，产品代码零修改并在报告中留证；若来源为产品 URL，必须拿到完整调用栈后修发送端。
- [ ] 运行详情动作首屏只显示动作、节点、处理人、接口、真实接口耗时、返回摘要、不能继续的原因和下一步；内部阶段、完整参数和原始正文进入折叠排查区。
- [ ] 相关测试实际执行并保存于根目录 `test/` 分类目录；`git diff --check`、Go 定向测试、前端类型检查通过；每个原子成果用中文提交。

## 2. 已确认证据和边界

### 2.1 运行号 6/7、路径 12 的处理人缺失不是目标偶发问题

工具日志位于：

`logs/runs/运行_7__欧阳改测试1005__run-98/paths/路径%2012__path-run-163__path-4221/curl.log`

运行号 6 同样复现。人工在目标平台用当前计划发起人手工提交后，可以在“已发”看到当前处理人；工具运行则实例创建成功但没有生成工具能够读取到的当前处理人。当前代码和日志已经证明存在以下工具侧差异：

1. FormMaking 发起请求使用了 `data.flowProxyId`，而目标页面成功请求使用 `data.formProxyId`。
2. 工具请求缺少手工请求中实际携带的顶层 `batchCode`、空数组 `nextAuditorList` 和 `projectId` 形状。
3. 工具请求中的表单身份仍可能是历史账号张泽华，而计划当前账号是骆蒙恩；账号、部门、岗位和 `global_user_basic_information` 没有在构造表单前完成实时覆盖。
4. 工具与目标页面在日期、虚拟字段、空值和表单容器字段类型上存在差异。
5. 其他动作接口仍需逐接口对照，不能只修 `submit` 就认为协议闭环完成。

因此“目标平台没有产生处理人”必须先按请求差异、身份差异和处理人事实读取链排查。只有请求已与目标页面一致、实例仍无 `currentAuditUserInfo`/精确待办，才可以登记为目标侧真实异步或目标配置问题。

### 2.2 代码证据索引

| 问题 | 工具代码 | 目标代码/证据 |
| --- | --- | --- |
| `form_person` 全树收集、每步复用 | `internal/adapter/target/protocol_matrix.go:256-333`、`internal/engine/step/gate.go:153` | 新发起 `FlowDialog.vue:940-975`；审批 `EnterpriseExamineDialog.vue:645-732`、`:767-781` |
| 写后核对只写局部事实 | `internal/engine/step/executor.go:1154-1162`；`internal/model/run.go:359-388` | 下一步仅检查 `executor.go:231-236`，没有持久化字段 |
| 特殊业务当前以阻塞代替实现 | `internal/adapter/target/protocol_matrix.go:206-223`；`internal/engine/step/executor.go:251-266` | `FlowDialog.vue` 的业务分支；`EnterpriseExamineDialog.vue` 的审批分支 |
| 身份切换晚于表单构造 | `internal/engine/step/executor.go:348-378`、`:466-504` | `FlowDialog.vue:665-676` 无条件用登录态覆盖 |
| 身份覆盖被 `global_*` 存在性短路 | `internal/adapter/target/protocol_matrix.go:374-385` | 目标发起页无条件设置 `global_user_basic_information` |
| 矩阵伪造信封存在 | `internal/adapter/target/protocol_matrix.go:500-544` | 目标页面 axios 请求的真实字段形状 |
| 审批意见自动生成 | `internal/service/path_data_workspace.go:153-157`、`internal/service/history_replay.go:524` | `EnterpriseExamineDialog.vue:1266-1335`、`:1531-1538` |
| Vue 生命周期竞态 | `web/src/views/RunDetailView.vue:353-430`、`:735-738`、`:756-771`、`:1042-1051` | Vue 3.5.40 `runtime-core.esm-bundler.js:4794-4798` |

## 3. 修复铁律

1. **目标页面请求是唯一协议标准。** 同一个目标动作在同一个场景产生的请求必须具有同一字段层级和存在性。不能因为 Go 结构体有 `omitempty`、某字段通常为空或“后端可能忽略”而删掉目标页面实际发送的字段。
2. **工具身份是变量。** “骆蒙恩”只是本次证据中的计划发起人，不得写成常量；“刘志阳”只是目标自动生成审批意见中的实际审批人，不得写成固定测试值。发起账号、工具会话身份、表单身份和节点处理人是四个不同概念，必须分别取事实。
3. **业务数据处理顺序必须对齐目标。** 目标前端在发起、草稿、重提、审批前后的钩子、业务组件写入和金额/日期/意见改写都属于业务协议，不得以通用表单提交替代。
4. **流程名称是唯一豁免。** `data.name` 继续使用工具侧命名规则；除此之外的业务字段、代理 ID、批次号、业务关联、处理人、表单值和响应判定都按目标平台。
5. **写请求最多一次。** 网络异常或响应丢失先按目标实例/任务事实对账，不能因为 `batchCode`、超时或轮询再次发送同一业务写。
6. **不确定就阻塞，但不能把已知链路也阻塞掉。** 只有目标源码尚未勘定或实施证据不足的分支才阻塞；已知分支必须实现并测试。
7. **页面错误不得静默。** 不能在全局 `window.onerror`、Vue runtime 或 Promise 包装中吞掉错误来“让控制台干净”。必须修复产生错误的生命周期或消息发送端。

## 4. 工作包 A：修复运行详情 Vue 生命周期竞态

### A1. 根因判断

控制台报错位于 Vue 3.5.40：

```text
shouldUpdateComponent(prevVNode, nextVNode, optimized)
const emits = component.emitsOptions
TypeError: Cannot read properties of null (reading 'emitsOptions')
```

`prevVNode.component` 为 `null`，说明 Vue 正在更新一个被认为是组件、但实例尚未建立或正在卸载的旧 VNode。产品代码存在可以确认的竞态组合，不能把它归咎于 Vue runtime 本身：

- `#app-header-context` 在 `web/src/App.vue:33-38` 首屏就存在，`RunDetailView.vue:756-771` 仍使用 `Teleport v-if="detail" defer`；目标早已存在，不需要 `defer`。
- `/runs/:runId/paths/:pathRunId` 切换参数时复用同一个路由组件，`watch(route.params.pathRunId)` 会与旧 `loadDetail`、轮询和事件请求并发。
- `loadDetail`、`schedulePoll`、`pollEvents`、放行和路径切换没有统一请求代次或 AbortController；迟到响应可以把旧路径详情写回当前页面。
- 卸载只清 timer，不让已经在途的 Promise 失效。
- `RunNodePanel` 父级用 `v-if="selectedNodeKey"` 条件卸载，同时内部有三个 `NModal`（Teleport/Transition）；切路径或关闭面板时容易形成旧组件仍有异步更新、父级已进入新 VNode 的窗口。

完整浏览器堆栈尚未在本机自动浏览器中复现，因此实施报告必须保留“源码证据 + 浏览器实际堆栈”两部分，不能声称已经单点复现。禁止修改 Vue runtime 源码加 `component == null` 守卫。

### A2. 具体修改步骤

1. **移除无必要的 `defer`。** 将 `RunDetailView.vue` 的 `Teleport v-if="detail" defer` 改为不带 `defer`；保持 `#app-header-context` 作为 App 的稳定祖先节点。
2. **建立统一请求代次。** 在 `RunDetailView.vue` 增加 `requestVersion`、`disposed` 和当前 `AbortController` 集合。每次 `runId/pathRunId` 改变时递增代次、abort 旧请求、清空旧事件游标和事件数组，并关闭节点面板与 Modal。
3. **给 API 请求传 signal。** `fetchRunDetail` 已支持 `AbortSignal`；扩展 `fetchRunEvents`、`fetchFlowGraph` 或其调用层以支持 signal。AbortError 属于预期取消，不显示错误、不进入 `console.error`。
4. **响应写入前做三重判断。** 详情、事件和流程图响应只有在 `requestVersion` 未变化、`runId` 与 `pathRunId` 仍匹配、`disposed === false` 时才能赋值。旧响应不能覆盖新路径，即使 HTTP 200 也必须丢弃。
5. **避免并发轮询链。** `schedulePoll` 的回调开始时记录本次代次，使用 `await` 后只允许当前代次重新安排下一次 timer；禁止旧 timer 在新路径上继续调度。轮询失败不能吞掉 AbortError 以外的异常，至少落入页面已有的可读状态。
6. **重置事件游标。** `lastEventID` 必须按 `pathRunId` 绑定；切换路径时归零或从服务端返回的路径起点重新读取。旧路径事件不能追加到新路径时间线。
7. **稳定节点面板生命周期。** 给 `RunNodePanel` 传 `:key="\`${detail.pathRunId}-${selectedNodeKey}\`"`，或采用稳定实例并显式清理内部 Modal。不能让旧路径的 panel 实例被 Vue 当作新路径 panel 复用。
8. **切换路径前关闭面板。** `switchPathRun` 和路由 watch 先执行 `selectedNodeKey = ''`，清理当前节点详情弹窗状态，再开始加载新路径。
9. **处理所有 `void` Promise。** `void loadDetail()`、`void pollEvents()`、`void router.replace()` 等入口必须在函数内部捕获非取消错误；不得产生 `Uncaught (in promise)`。错误要么进入页面可读错误区，要么明确记录一次带路径上下文的错误。
10. **保持最后有效当前节点。** 详情刷新在新响应没有 `currentPreview` 时，不要立即把画布当前节点清空；保留同一代次上一次有效节点，直到新快照明确给出下一个节点或终态。这一条只解决短暂空白，不得用旧代次数据覆盖新路径。

### A3. 测试和验收

在 `test/unit/frontend/run_detail_lifecycle/` 保存测试，至少覆盖：

- 快速 A→B→A 路径切换，响应按 B、旧 A、当前 A 的乱序返回；最终只显示当前 A。
- 打开节点面板和三个 Modal 时切换路径，组件无 `emitsOptions` 崩溃。
- 首次 `detail=null` 变为有值同时发生 `router.replace`，Teleport 不报错。
- 离开页面时详情和事件请求仍在途，Abort 后不改状态、不产生未处理 Promise。
- 连续轮询与放行响应交错，旧响应不能回退当前节点。
- 事件游标切换路径后不串流。

人工浏览器验收时打开 DevTools Console，执行快速路径切换、打开/关闭节点详情、放行后立刻切换和返回任务列表；控制台不得出现 `emitsOptions`、`Uncaught (in promise)` 或由本页面抛出的异常。

## 5. 工作包 B：归因并处理 `Unchecked runtime.lastError`

### B1. 已确认边界

产品 `web/`、`form-runtime/` 和允许范围内的目标参考代码没有 `chrome.runtime`、`browser.runtime`、`runtime.sendMessage` 或 `runtime.lastError`。该错误是 Chromium 扩展消息 API 的典型错误：发送方给不存在、未注入或已卸载的 receiving end 发消息，且未消费扩展侧 `runtime.lastError`。

高概率来源是 Vue Devtools、密码管理器、自动化扩展或其他浏览器扩展内容脚本，不是本项目业务代码。不要在产品中加入全局错误吞掉补丁，也不要在 Vue runtime 中过滤。

### B2. 必须完成的 A/B 归因

1. 使用无扩展窗口或禁用全部扩展打开 `http://127.0.0.1:19000`，执行与正常窗口相同的路径详情操作。
2. 若无扩展窗口不再报错，打开原错误的 Console 源链接，记录 `chrome-extension://扩展 ID/...`、扩展名称和发生时机，在交付报告标记为外部噪声；产品代码不改。
3. 若无扩展窗口仍报错，点击源链接确认是否是产品 URL。只有在产品 URL 时，才沿完整调用栈定位发送端；将调用位置、消息名称、接收端注册时机写入报告，再做最小修改。
4. 只要来源仍不明，不得把该错误与 Vue `emitsOptions` 合并，也不得声称已修复。

## 6. 工作包 C：建立表单字段所有权，修复 `auto_audit_info_*`

### C1. 目标平台为什么会改写审批意见

目标审批页面在 `EnterpriseExamineDialog.vue:1266-1335` 读取实例数据时，会将：

- `auto_audit_info_<n>` 文本字段；
- `auto_audit_info_obj_<n>` 对象字段；
- `auto_audit_info_obj_list_<n>` 会签/多条意见列表；

按 `auditDesc`、`auditStatus`、`auditName`、`auditDate` 重新格式化展示。目标后端会依据真实审批记录机械生成这些值。用户样例中的：

```text
期望：空字符串
实际：刘志阳 同意 2026-09-14 15:19
```

是目标平台正常生成的审批事实，不是工具写错。当前工具在配置阶段通过 `clearAuditInfoValues` 清空历史意见，在 `BuildNodeFormData` 中又把所有字段纳入跨节点严格比较，导致合法目标衍生值被错误判为“上一节点写入的字段被改写”。

### C2. 字段分类（只允许一个登记处）

在 `internal/adapter/target` 建立字段所有权/来源登记，供配置、运行、写后核对共同使用：

| 分类 | 含义 | 核对规则 |
| --- | --- | --- |
| `tool_owned` | 当前节点配置明确覆盖的业务字段 | 目标保存后必须等于本次实际发送值 |
| `preserved_business` | 上游节点已经填写、当前节点无权限覆盖的业务字段 | 必须保留；丢失或被工具改写则阻塞 |
| `target_derived` | 目标引擎根据审批事实自动生成的字段 | 不与历史空值严格相等；禁止工具删除、清空或回写历史旧值 |
| `node_owned_future` | 后续节点才拥有的字段 | 当前请求不得提前发送 |

`target_derived` 至少匹配以下前缀，并同时识别伴生对象/列表：

- `auto_audit_info_`（排除 `auto_audit_info_obj_` 的前缀重叠时要先做精确判断）；
- `auto_audit_info_obj_`；
- `auto_audit_info_obj_list_`。

不要只写一个 `auto_audit_info_` 的特例判断；登记函数必须返回分类和伴生键关系，并用对象、列表、缺失对象三种 fixture 测试。

### C3. 正确的数据生命周期

1. **新发起/草稿：** 继续清空历史审批意见及对象/列表内容，保留目标页面需要的键和结构；不得将历史审批人、意见和时间带进新的实例。
2. **审批前：** 读取目标实例最新完整表单。对 `target_derived` 字段，以目标当前值作为基线，不用配置阶段被清空的值覆盖；当前审批动作如目标页面会产生意见，由目标页面/后端按真实处理人生成。
3. **审批后：** 重读目标实例，把新生成的 `target_derived` 值写入本次运行事实或最新实例基线；下一节点完整表单请求必须原样保留这些目标值。
4. **写后核对：**
   - `tool_owned`：缺失或值不同，记录核对失败；
   - `preserved_business`：缺失或变化，记录核对失败；
   - `target_derived`：新增、格式化变化、审批人和时间变化均正常；只有工具将目标已有非空值改为空、删除、回写旧历史值时失败；
   - `node_owned_future`：不应出现在当前载荷，出现即失败。
5. **错误文案：** 不再显示“上一节点写入的字段 auto_audit_info_* 被改写”。应显示类似：“目标平台已根据上一节点审批事实生成审批意见，已采用目标当前值并继续；若工具请求删除该值，则阻塞”。

### C4. 修复当前实现的两个核对缺陷

- `BuildNodeFormData` 跨节点核对（`internal/engine/step/formdata.go:148-193`）在比较上一节点覆盖字段前，先跳过 `target_derived`，改为比较“工具是否破坏目标现值”。
- `verifyFormDataAfterWrite`（`internal/engine/step/executor.go:549-602`）当前基线字段不存在时直接 `continue`。对于 `preserved_business` 和目标已有非空 `target_derived`，缺失必须报告；只允许目标从空到自动生成的衍生字段缺失判断走目标事实规则。

## 7. 工作包 D：修复 `form_person` 按动作、节点和入口的作用域

当前 `CollectFormPersonFields` 递归收集整棵树，运行时却在每次发起、重提、审批全部应用；`NodeID` 没参与当前下一节点过滤。这与目标页面的两套规则不同，不能继续用一个全局字段列表。

### D1. 目标规则

- 新发起页面 `FlowDialog.vue:678-682` 对 `staff_annual_performance` 有单独全树 `traverseFlowNode` 逻辑；这是业务类型特例，不得推广成所有审批动作规则。
- 审批页面 `EnterpriseExamineDialog.vue:645-732` 先找到 `nextNodeProxyId`，只收集该节点以及它的直接条件/并行入口；不是整棵树。
- 审批值提取 `EnterpriseExamineDialog.vue:767-781` 使用 `replace('__formPersonId','')`；新发起 `FlowDialog.vue:953-965` 使用 `fieldKey.split('__')[0]`。这是目标页面本身存在的动作差异，工具必须按动作选择对应规则，不得自行统一。

### D2. 实施步骤

1. 保留流程树递归能力，但输出 `NodeFormPersonField{NodeID, Field, Scope}`，其中 `Scope` 至少区分 `initiation_full_tree`、`approval_next_entry`、`resubmit_page_rule`。
2. 发起/草稿/重提分别绑定目标页面实际使用的 `traverseFlowNode` 变体；审批根据目标最新 `nextNodeProxyId` 只计算当前入口和直接条件/并行子入口。
3. 注入前必须确认字段对应当前动作和真实目标节点；同字段被多个节点声明时，按目标页面当前入口消歧，不能把所有节点值写入同一请求。
4. JSON 源值取 `id`，纯文本按目标页面原值规则；不得无依据 trim、改写大小写或把缺失值伪造成用户 ID。已有目标值不覆盖。
5. `nextAuditorList` 的 `bizId`、`nodeProxyId`、`name`、`auditDetailType` 等字段严格按目标接口矩阵生成；空数组、缺失和自动规则节点必须区分。

## 8. 工作包 E：身份与处理人链路

### E1. 先切真实会话，再构造表单

当前 `nodeFormData` 先构造 `BuildNodeFormData`，之后才在 `executor.go:466-504` 读取当前账号身份并覆盖。这会导致动态表单字段、选项和伴生键仍按旧账号生成。调整顺序：

1. 从计划读取当前发起账号（计划账号是变量，不固定为骆蒙恩）。
2. 建立/恢复该账号目标会话。
3. 通过 `CurrentUserIdentity` 读取当前用户、公司、部门、岗位完整事实。
4. 将身份注入表单运行时上下文和目标页面需要的表单值。
5. 再执行目标页面对应的 `getValues`/表单合并/`BuildNodeFormData`。
6. 生成请求预览和实际请求必须复用同一份最终载荷。

审批动作则在读取目标任务快照后切换到当前真实处理人会话，再构造审批表单和请求。不能使用计划账号会话构造审批表单后，发送前才替换 actor 名称。

### E2. `ApplyUserIdentity` 不能要求全局字段先存在

目标新发起页面无条件设置 `global_user_basic_information`（`FlowDialog.vue:665-676`）。工具当前在 `protocol_matrix.go:381-385` 发现全局字段不存在就整体返回，导致应写的身份字段未覆盖。修复为：

- 对目标页面实际会发送的字段逐项注入；全局字段不存在时，若目标动作要求该字段，则创建并注入，若该页面确实不要求则不创建；
- 所有登记的 `myUserName`、`myDepName`、`myCompanyName` 和 `__condition`、`__formPersonId` 伴生键按目标页面规则处理；
- `UserIdentity.Complete` 验证本次动作实际需要的 ID/名称集合，不能只验证 UserID、CompanyID 和岗位而忽略 DepartmentName、UserName 等目标必需值；
- 身份读取失败、部门/岗位缺失或用户与公司关系不一致，在写请求前阻塞并记录明确字段名。

### E3. 当前节点处理人不得与候选人混淆

“候选人/下一节点选人”只用于目标页面要求的 `nextAuditorList`；当前节点处理人必须来自目标实例的 `currentAuditUserInfo` 或精确任务事实。执行器应：

- 先按目标页面视角读取实例和任务；
- 验证实例 ID、目标节点 ID、`jobTaskId`、`batchNo`、`flowProxyId` 与处理人属于同一最新快照；
- 缺失时最多 3–5 次、总计不超过 10 秒只读轮询；
- 仍缺失时状态为 `blocked/assignment_missing`，文案为“目标节点未生成处理人”，不显示“已处理”，不登录计划账号代替处理人。

## 9. 工作包 F：写后核对事实必须持久化

当前 `after.FormDataVerifyIssue` 只赋给局部 `InstanceFacts`，而 `RunStepAttempt` 只有 `BeforeFacts`；下一步检查虽然存在，但重启、轮询或下一事务拿不到上一轮核对结论，门禁不成立。

### F1. 实施要求

1. 为运行事实增加与当前模型兼容的持久化字段，优先复用现有 JSON 列/事实快照，不新增数据库表；字段至少包含 `FormDataVerifyIssue`、验证时间、决策指纹和字段级结果。
2. 在写后核对事务中，与步骤尝试、`after_facts`、核对结论一起落账；不得只写内存对象。
3. 下一步读取最新持久化事实，以 `pathRunId + stepNo` 精确取得上一节点结论；不能从全局共享变量读取。
4. 服务重启后仍能恢复阻塞结论；旧运行事实只读，不做回填或迁移猜测。
5. 详情 API 返回用户可读摘要，日志保留字段级原始核对结果；页面显示“上一步表单核对未通过”，点击可到对应步骤和日志。

### F2. 必测场景

- 覆盖值保存成功、保留值未变：继续。
- 覆盖值未保存：阻塞。
- 上游业务字段被目标或工具改写：阻塞。
- 上游字段从实例中删除：阻塞，不再静默 `continue`。
- `auto_audit_info_*` 从空变为目标生成意见：继续。
- 目标已有非空 `auto_audit_info_*` 被工具写空：阻塞。
- 写后核对落账后重启服务，下一步仍阻塞。

## 10. 工作包 G：协议矩阵和所有写接口逐项对齐

### G1. 修复矩阵伪造字段存在

`ValidateBodyMatrix` 当前把 `sid`、`projectId`、`data.customerCode` 直接设为 `true`（`protocol_matrix.go:525-531`），这会让缺失/空值与真实存在混为一谈。改为：

- 统一信封构造器先按目标页面生成最终 body；
- 矩阵校验读取最终 body 的真实路径和值；
- `required` 检查字段是否真实存在，`empty` 检查存在且值形状为空字符串/空数组/空对象，`forbidden` 检查路径不存在，`optional` 不强制；
- `sid`、`projectId`、`customerCode` 只有在目标该端点矩阵登记 required/empty 时校验；不能全局默认必有；
- 任何未登记端点、未登记字段或形状不明端点禁止发出写请求。

### G2. 逐接口矩阵最低覆盖

| 场景 | 端点 | 必须单独核对 |
| --- | --- | --- |
| FormMaking 发起/草稿 | `/web/flowInstanceApi/submit` | `formProxyId`、`status=draft`、完整表单、业务关联、`nextAuditorList`、`batchCode`、`projectId`、公司 ID |
| 驳回/撤回后重提 | `/web/flowInstanceApi/reSubmit` | 实例 ID、实时代理 ID、完整表单、业务关联、入口、显式人员、页面同源字段；不能复用新发起协议 |
| 当前节点暂存 | `/web/flowInstanceApi/storageFormData` | 实例 ID、当前目标节点 ID、`auditRecord.executeDesc`、完整表单 |
| 同意/不同意 | `/flowInstanceApi/audit` | `jobTaskId`、`flowProxyId`、`auditRecord.auditStatus/executeDesc`、完整表单、业务关联、下一节点人员、`tracking` |
| 移交/加签 | `/web/flowInstanceApi/approverAppend`、`updateFlowProxy` | `batchNo`、任务 ID、节点 ID、用户列表或完整代理树 |
| 回退/取回/撤回 | 对应 `rollBackThePreviousLevel`、`retrieveProcess`、`revocation` | 实例/任务事实、说明字段、当前任务参数 |
| 转发/关注/催办 | `transpond`、`flowTracking`、`urgeHandleRecord/sendUrgeMessage` | 顶层与 `data` 层级、接收人、实例 ID、关注布尔值、催办参数 |
| 只读事实 | 实例、任务、审核记录、当前表单接口 | 顶层实例过滤、用户视角、节点/任务 ID、批次号、当前处理人和完整数据来源 |

`batchCode`、`nextAuditorList`、空字符串、空数组、空对象和省略不是全局规则，必须逐端点登记。删除旧的 `batchCode` 一律禁止规则及与矩阵冲突的测试；不能为了通过旧测试而修改目标协议。

### G3. 人工 curl 对照清单

对运行 6/7 路径 12，使用日志中的人工成功 curl 逐字段生成差异表：

- URL 路径和 query：`sid`、`platformCode`；
- 请求头：`Accept`、语言、`Content-Type`、`Origin`、`User-Agent`，环境地址允许不同；
- `data.name`：只允许工具命名规则差异；
- `data.formProxyId` 与 `flowProxyId` 互斥；
- `companyId`、`customerCode`、业务关联；
- `formDataMongoVo.data` 的每个字段值、类型、虚拟字段、人员字段、空值；
- 顶层 `nextAuditorList` 是否存在及其数组形状；
- 顶层 `batchCode`、`sid`、`projectId` 的存在性与位置；
- 响应 `isSuccess/code/message` 与目标实例事实。

差异必须标记为“协议差异”“允许业务值差异”或“工具缺陷”，不能只比较 JSON 字符串顺序。

## 11. 工作包 H：FormMaking 特殊业务与 NoFormFlow 必须实现

上一轮实现登记了 `specialBusinessFlowTypes` 并在命中时阻塞；这是安全临时措施，不满足本轮用户要求。现在必须逐分支迁移目标页面逻辑，并保留未知分支的明确阻塞。

### H1. 已知特殊业务分支

以 `rsh-cloud-invest-power-system` 的 `GroupApproveManage` 及直接引用公共组件为范围，逐项建立“目标源码符号 → 工具函数 → 请求端点 → 前置/后置顺序 → 测试 fixture”登记：

- 合同合规/合同盖章：自定义组件分支，涉及 `beforeSubmitAndDraft`、组件写入和 `afterSaveFlowInstance`；不能只发通用表单 submit。
- 资金往来/投资款：先保存业务数据，再发起/审批；业务数据保存成功判据和失败顺序按目标代码。
- 出版委托/专业提资：审批前后日期改写逻辑按目标 `EnterpriseExamineOpinion` 分支执行。
- 年度绩效/考核：审批写意见、目标页面全树表单人员遍历及其字段重置按目标代码执行。
- 费用报销：审批金额计算、归口/费用数据处理按目标顺序执行。

每一类必须回答：

1. 触发条件是目标返回的哪个流程类型/页面字段；
2. 使用哪个当前节点、当前真实处理人和表单值；
3. 业务写接口的确切 URL、query、body 和成功判据；
4. 失败时是否允许继续主流程；
5. 主流程前置、发起/审批、后置钩子的顺序；
6. 重试、响应丢失和服务重启如何对账且不重复写；
7. 日志如何关联到同一 `step/attempt/phase`。

目标源码无法确认的字段不能猜测。完成登记前，命中该分支继续阻塞并指向具体未实现的源码符号；完成后删除“已知分支统一阻塞”逻辑。

### H2. 无表单 `vue_custom`/NoFormFlow

`ResolveVueCustomPage` 目前标记 `partial` 后，执行器对发起/草稿/重提全部阻塞。这与已验收 F-012 的无表单运行时支持和用户要求不一致。必须复制目标无表单页面的实际链路：

- 页面初始化数据、`initiatorRange`、项目/业务关联；
- 自定义字段类型、金额/日期/附件和虚拟字段；
- `submitFinal` 或等效提交方法的请求 envelope；
- 并行/手动分支的选人字段和 `nextAuditorList`；
- 草稿、重提、审批暂存和同意的专用形状；
- 页面钩子和业务组件写入顺序。

复制逻辑必须保持目标字段名和空值形状，只在流程名称上使用工具命名。不能用 FormMaking 的 `formProxyId` 协议顶替 NoFormFlow 的 `flowProxyId`，不能通过“通用请求成功”掩盖业务数据未写入。

## 12. 工作包 I：分节点有效表单数据闭环

### I1. 节点时机

- 发起 `submit`：基线是发起态完整表单；只发送发起节点允许字段、目标页面自动生成系统字段和目标要求的伴生键；后续节点字段进入 `withheld`，不得提前出现。
- 当前节点 `storageFormData`：先读同一实例最新完整表单，再叠加当前节点可编辑字段；不得把审批提交协议当暂存协议。
- 当前节点 `audit`：按目标审批页面需要发送完整合并表单，当前节点只能覆盖自身权限字段，审批意见衍生字段保留目标现值。
- `reSubmit`：基线来自实例当前表单和目标重提页面字段；不得退回新建发起白名单。

### I2. 数据绑定

下拉、远程选项、人员关联和虚拟字段必须在 FormMaking runtime 完成绑定、联动、最终回读和保存后才进入 `effective_form_data`。页面配置看到的参考值不能代替运行时最终值。每个节点请求必须记录：

- `stepNo`、工具 `nodeKey`、目标真实节点 ID；
- 动作和表单基线来源/版本；
- 当前节点可编辑、覆盖、保留、扣留、目标衍生字段；
- 最终 JSON 指纹及实际发送内容来源；
- 写后核对结论和目标当前表单事实。

后续节点只能消费“上一节点决策 + 最新目标实例事实”，禁止直接读取全局 `EffectiveFormData` 作为最终载荷。

## 13. 工作包 J：运行详情与日志可读性

### J1. 节点详情首屏

首屏固定显示：

1. 做了什么：中文动作 + 节点名称；
2. 谁做的：真实处理人，未知时显示原因；
3. 请求哪个接口：目标端点；
4. 请求耗时：只使用 `internal/adapter/target` 传输层真实 `RoundTrip`，未知显示“未知/暂无真实接口耗时”，禁止用阶段差值或 `0ms` 猜测；
5. 目标返回什么：状态码、目标 `code/message` 摘要和实例/任务事实；
6. 为什么不能继续：业务拒绝、处理人缺失、表单核对失败、特殊链路未实现等具体原因；
7. 下一步：用户应检查哪个节点、哪个字段或目标页面。

内部阶段、参数键、完整请求/响应、curl 和日志路径放入“查看日志与原始请求”折叠区。日志仍按内网规则原样落盘，不经公开 DTO 返回完整 SID/正文。

### J2. 状态文案

- `blocked`：工具或目标已明确知道当前条件不满足，停止在当前步骤；
- `assignment_missing`：目标节点已到达，但目标未生成处理人/待办，有限等待后阻塞；
- `result_unconfirmed`：写出后响应丢失或无法核实，不能判断目标是否生效；
- `target_skipped`：目标事实明确证明节点已跳过；
- `failed`：目标明确拒绝或工具确定失败。

“准备运行”“准备放行”“处理中”“当前待办已经处理”等泛化或误导文案不得作为唯一结论。每条阶段说明至少带动作、节点、处理人或等待原因和下一步。

## 14. 测试任务目录

所有新增测试保存到根目录 `test/`，不放在临时目录或仅写文档：

- `test/unit/frontend/run_detail_lifecycle/`：请求代次、Abort、Teleport、节点面板、Modal、事件游标和无未处理 Promise；
- `test/unit/backend/form_data_by_node/`：分节点基线、字段所有权、目标衍生意见、保留/扣留/删除检测；
- `test/unit/backend/assignment/`：处理人出现、超时、节点越过、`isSkip`、禁止账号回退和不重复写；
- `test/contracts/f035/protocol/`：每个目标写接口的前端 fixture、工具 body、四态字段矩阵和归一化差异；
- `test/contracts/f035/identity/`：两个不同计划账号、身份字段切换、岗位缺失、历史账号污染和任务真实处理人；
- `test/contracts/f035/special_business/`：已知五类特殊分支的前置/主流程/后置顺序、请求和失败判定；
- `test/contracts/f035/no_form_flow/`：`vue_custom`/NoFormFlow 初始化、业务关联、发起、草稿、重提、审批和手动/并行选人；
- `test/unit/backend/form_data_derived_fields/`：`auto_audit_info_`、`obj_`、`obj_list_` 的生成、保留、清空和核对；
- `test/contracts/f035/drift/`：参考前端/Java Controller/Service 关键符号漂移检测；不能把实现文件和测试文件复制同一份列表后互相自证。

测试 fixture 必须来自参考源码真实请求或脱敏后的日志结构；不能用与实现相同的硬编码列表作为唯一测试依据。未知分支必须有“未登记即拒绝发送”的测试。

## 15. 建议实施顺序与门禁

1. 先建立并评审协议矩阵、特殊业务登记、字段所有权分类和目标衍生字段规则；同步修改 `docs/TARGET_SEMANTICS.md`、F-014 相关旧禁令和本功能文档。
2. 修复身份/代理 ID/信封构造与分节点表单基线；先用人工 curl fixture 验证预览和实际 body 同源。
3. 实现处理人事实读取、有限轮询和写后核对持久化；确保路径 12 第二节点不再用计划账号回退。
4. 实现已知 FormMaking 特殊业务和 NoFormFlow 专用链路；未知分支继续安全阻塞。
5. 修复 `auto_audit_info_*` 目标衍生字段核对，再补完整表单写入闭环。
6. 修复运行详情前端代次、取消和稳定组件生命周期；完成控制台错误 A/B 归因。
7. 补齐测试并实际运行；每个原子成果中文提交。所有验证通过后停在 `ready_for_manual`，不能自动接受或开始其他功能。

## 16. 禁止实现清单

- 不得把流程发起人骆蒙恩、历史账号张泽华、审批人刘志阳写成固定身份。
- 不得把候选人/下一节点处理人当成当前节点处理人。
- 不得用 `flowProxyId` 顶替 FormMaking 的 `formProxyId`，也不得无依据同时发送两个代理字段。
- 不得把 `batchCode` 当幂等键、重试键或跨接口公共字段；是否发送由端点矩阵决定。
- 不得继续使用矩阵中 `flat["sid"] = true`、`flat["projectId"] = true` 这类伪造存在性。
- 不得把目标平台自动生成的审批意见与历史空值做严格相等比较。
- 不得把特殊业务和 NoFormFlow 全部“命中即阻塞”当作最终完成；已知目标分支必须实现。
- 不得在 Vue runtime、全局错误处理器或 Promise 包装器中吞掉 `emitsOptions`/`runtime.lastError`。
- 不得为了修复本任务新增数据库表、第二套表单模型、目标接口或兼容旧协议的回退分支。
- 不得通过重复写请求掩盖响应丢失；先按实例/任务事实对账。

## 17. 验证命令

按当前改动范围执行并记录完整输出：

```bash
go test ./test/unit/... ./test/contracts/f035/...
bash test/contracts/f035/drift/protocol_symbols_drift.sh
pnpm --dir web run typecheck
git diff --check
```

如新增前端测试，使用项目现有 Node 测试入口，只运行 `test/unit/frontend/run_detail_lifecycle/` 及本次相关目录。不要把未运行的集成测试或真实目标写操作写成通过。真实目标写请求的人工验收仍由用户在页面执行。

## 18. 人工验收清单

1. 重新运行“欧阳改测试1005 / 路径 12”，在 `curl.log` 中对照手工成功请求：`formProxyId`、`batchCode`、空 `nextAuditorList`、`projectId`、公司关联、请求头和表单身份。
2. 更换计划发起账号再运行，确认所有身份字段和 `data.name` 随计划变化，旧账号 ID 不出现。
3. 确认目标第二节点的 `currentAuditUserInfo`/精确待办与工具读取的 `jobTaskId`、`batchNo`、节点 ID、处理人一致；没有处理人时页面显示“阻塞”。
4. 对至少两个可填写节点逐节点查看请求：节点 1 数据只在发起请求出现，节点 2 数据只在节点 2 的暂存/审批请求出现；节点 2 保留节点 1 值。
5. 验证审批后 `auto_audit_info_1` 等字段以目标生成的“审批人 + 状态 + 时间”为准，下一节点不会回写空历史值，也不会误报条件不满足。
6. 验证下拉/远程选项调整后，配置页、重新打开页面、`effective_form_data` 和实际请求的显示值/真实值/虚拟字段一致。
7. 验证合同盖章、资金往来/投资款、出版委托/专业提资、年度绩效/考核、费用报销等已登记特殊分支，确认业务写入顺序与目标页面一致；未知特殊分支明确阻塞。
8. 验证至少一条 NoFormFlow/`vue_custom` 流程的发起、草稿、重提和审批动作，确认使用 `flowProxyId` 和目标页面专用业务关联。
9. 快速切换运行详情路径、打开节点面板及三个 Modal、放行后立即切换、离开页面；Console 无 `emitsOptions`、未处理 Promise。
10. 在无扩展窗口和正常窗口分别复现 `runtime.lastError`，记录源 URL；产品源错误才进入代码修复，扩展源错误登记为外部噪声。

## 19. Agent 交付报告模板

```text
F-035 第四轮整改交付报告

1. 完成范围：按 Txx 列出实际修改文件和目标源码依据
2. 请求协议：运行 6/7 路径 12 与人工 curl 的逐字段差异结果
3. 身份与处理人：计划账号、当前处理人、任务 ID/批次号来源
4. 分节点表单：每节点基线、覆盖/保留/扣留、target_derived 处理
5. 特殊业务/NoFormFlow：已实现分支、未知分支阻塞清单
6. Vue 错误：浏览器堆栈、请求代次/取消验证结果
7. runtime.lastError：无扩展 A/B 结果和源 URL
8. 测试命令及原始结果：不得只写“通过”
9. 未完成/受阻项：明确外部权限或目标环境依赖
10. 中文提交哈希：每个原子成果一条
11. 当前状态：保持 implementing，全部自动验证通过后申请人工验收
```

