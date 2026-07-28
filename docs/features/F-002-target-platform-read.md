# F-002 真实账号与目标平台只读列表闭环

- 状态：implementing
- 产品依据：`docs/PRODUCT.md` 的“计划与运行主线”和“测试计划页面行为”
- 架构依据：`docs/ARCHITECTURE.md` 的“系统边界”“数据与部署演进”和“参考源码边界”
- 计划形成时间：2026-07-27
- 计划确认时间：2026-07-27

## 单一用户结果

用户在新建计划页输入目标平台真实账号后，可以由 Go 后端真实登录并复用安全缓存的会话，再按“新发起”“已发”“待发”搜索、分页读取并顺畅选择该账号的真实流程模板或实例。

本功能只建立“真实账号验证 + 三类真实候选读取”的只读闭环。账号验证与 SID 缓存本来就属于 F-002，不回填或改写已经验收的 F-001 静态原型范围。

## 范围

包含：

- Go 后端读取服务端目标平台配置，按用户输入账号执行真实登录。
- SID 仅在 Go 进程内缓存和使用；浏览器只得到验证结果与必要账号摘要。
- “新发起”读取当前已验证账号可见的流程模板。
- “已发”读取当前已验证账号发起过的流程实例。
- “待发”读取当前已验证账号的待发流程实例。
- 将 F-001 已验收的候选区从本地 mock 数据源切换为真实分页数据，保留稳定外壳、虚拟滚动、选中态和渐进引导。
- 搜索防抖、分页增量加载、旧请求取消或忽略，以及加载、空、错误、到底状态。
- 目标平台会话失效后，后端只允许一次安全重登并重放当前只读请求。
- 使用可控假目标服务完成自动测试，再由用户使用真实目标环境人工验收。

不包含：

- 保存或创建测试计划、数据库、路径选择、Vue Flow、流程解析和真实执行。
- 修改、删除或提交目标平台业务数据；除登录外，只调用列表型只读接口。
- 将 SID、密码、AES key 或其他凭证返回浏览器、写入浏览器存储或记录到日志。
- Redis、Nacos、Docker、MySQL、MongoDB、多进程共享会话或持久化会话。
- 目标平台写操作、FormMaking、V1/V2 业务代码复制，以及 F-003 之后的工作。
- 真实环境不可用时静默回退到 mock；生产交互必须明确显示读取失败，不能用假数据冒充真实结果。

## 已核实证据

### V1 会话与目标平台适配

| 证据 | 已核实行为 | F-002 采用或调整 |
| --- | --- | --- |
| V1 `internal/session/session.go` | 会话按账号索引；本地 map 可兜底；每个账号有独立互斥锁；默认 TTL 为 8 小时；业务请求遇到会话失效后再失效缓存并重登 | 采用单进程内存、按账号键控、每账号并发去重、8 小时可配置 TTL 和一次重登；不引入 Redis |
| V1 `internal/session/session.go` | `SessionInfo` 含 SID、账号、用户、公司、部门、customerCode、platformCode | 缓存保留 SID 与服务端所需摘要；公开响应只返回账号、用户显示名和公司名，不返回 SID 或内部代码 |
| V1 `internal/adapter/target/http_client.go` | 登录入口为 `POST /web/user/api/login/user/login`；请求字段为 `loginType: ACCOUNT`、`account`、加密 `password`、`platformCode`、`customerCode`、`code` | 保留已核实字段；密码、AES key、code 只从服务端配置读取，不从浏览器接收 |
| V1 `internal/adapter/target/http_client.go` | 目标业务请求会在 body、query 和 header 传递 SID，并在 query 追加 `platformCode` | SID 传递封装在 `internal/adapter/target`；日志禁止输出完整 URL、query、header、body 或 SID |
| V1 `internal/adapter/target/http_client.go` | 会话失效既可能是 HTTP 认证失败，也可能是业务码 `RESP401`、`-1` 或带特定会话失效消息的 `ERROR_99999` | 适配层统一识别为会话失效；清缓存、重登、重放当前只读调用一次，禁止循环重试 |
| V1 `internal/handler/workbench.go` | `doSessionAPICall` 已验证“取缓存或登录—调用—会话失效—失效缓存—重登—重放一次”的顺序 | F-002 保留该顺序，但使用更小的独立模块，不复制 V1 工作台的大型 handler |
| V1 `frontend/src/api/index.ts`、`WorkbenchView.vue` | 前端通过独立 API 查询账号可见模板，使用名称搜索、分页、加载/空/错误状态和整行选择 | 只复用交互经验；F-002 继续使用当前项目已验收候选区，不复制 V1 卡片页面或状态管理 |

V1 中有日志把 SID 当字段输出的做法，不符合本功能安全要求，明确不复制。用户已批准环境协议沿用 V1，但密码、AES key 和 code 的真实值仍必须由 Go 后端运行时环境提供，代码和文档不固化任何值。

### 流程模板

V1 `internal/adapter/target/flow_reader.go` 与 `internal/handler/workbench.go` 共同确认：

- 目标入口：`POST /web/flowTemplateApi/list`。
- 分页：顶层 `pagination: true`、`pages`、`size`，分页结果来自响应顶层 `total`、`pages`、`current`、`size`。
- 搜索：`data.flowName`。
- 当前业务范围：`data.useScope: invest`、`showMe: true`，并携带 `customerCode`、`platformCode: 200001,999999` 及已核实的排除参数。
- 实际字段：`id`、`flowName`、`code`、`groupName`、`flowStatus`、`typeName`、`updateDate`、`createDate`、`remark`、`flowCreateType`；V1 兼容解析还包含 `flowProxyId`、`formProxyId` 等，但 F-002 列表不需要的字段不公开。

### 实施平台模板配置证据

本轮只读核对 `参考代码/rsh-cloud-saas-implementation-web`，未修改或复制其实现：

| 证据文件 | 实施平台实际行为 | F-002 采用边界 |
| --- | --- | --- |
| `src/api/index.js` | `templateLibrary.flowList` 明确指向 `POST /web/flowTemplateApi/list` | 与 V1 和当前适配入口一致，不新增接口 |
| `src/views/flowLibrary/index.vue` | 列表实际展示 `flowName`、由 `formExist` 派生的流程类型、`typeName`、`formTemplateList.length`、`remark`、表单模型关联和 `updateDate`；请求使用 `ignoreTemplateData: true` | 确认 `formExist`、表单模板数量、备注和更新时间是列表真实数据；不请求大体积模板 JSON |
| `src/views/flowLibrary/FormMulBranch/components/Steps1.vue` | 配置流程时实际填写流程名称、类型、流程分组、用途说明和可选表单模板编号；用途说明 `remark` 为必填 | 列表显示分类/分组与备注，帮助用户区分同名或近似模板 |
| `src/views/flowLibrary/NoFormMulBranch/components/Steps1.vue` | 无表单流程同样维护类型、分组和用途说明，并以 `formExist` 区分有/无表单 | 将有表单/无表单作为辅助信息，不臆造新的状态 |
| `src/views/formTemplates/index.vue` | “表单模板配置”页使用另一个配置列表，仅展示配置范围、名称与公司 | 该页不是 `/web/flowTemplateApi/list` 的流程候选，不把它的字段错误混入流程模板 DTO |
| `src/views/flowLibrary/index.vue`、`FormMulBranch/index.vue`、`NoFormMulBranch/index.vue` | 流程库按 `formTemplateBizRelevanceList[{otherBiz: "company", otherBizId}]` 筛选；流程详情从响应 `formTemplateBizRelevanceVoList` 中取 `otherBiz: "company"` 的 `otherBizId` | 已核实流程模板公司关联的 JSON 结构是 `formTemplateBizRelevanceVoList[].otherBiz`、`otherBizId`；该值是内部 ID，不能直接公开 |
| `src/api/index.js`、`src/views/flowLibrary/index.vue` | `frameworkInfo.getParentCompanyList` 指向 `POST /web/user/api/company/getParentCompanyList`，请求 `data.id`，页面以返回项 `id`、`name` 填充公司选择项 | 适配层仅在模板带公司关联且登录响应有 `companyVo.id` 时服务端读取该目录，并按关联 ID 映射为名称；未匹配或空名称不显示标签 |

字段分层结论：

- **接口确实返回**：V1 已解析名称、编码、分组、状态、分类、更新时间、备注、创建方式和 `formExist`；实施平台的同一列表还直接读取 `formTemplateList`，当前后端只公开其数量，不公开表单内容。流程模板的公司归属以 `formTemplateBizRelevanceVoList` 中 `otherBiz: "company"` 的 `otherBizId` 表示，不是可公开名称。
- **实施平台页面实际使用**：名称、分类、表单存在性与数量、备注、表单模型关联、更新时间；流程配置页进一步证明类型、分组和用途说明是业务配置，而非展示臆测。
- **当前列表值得公开**：名称为标题；仅在服务端能从已核实公司目录匹配到名称时显示公司标签，分类/分组、表单关联、更新时间为辅助信息；备注独立显示。编码只保留在后端兼容 DTO，不在候选行展示，也不参与前端本地搜索。`flowStatus/statusText` 继续保留在底层兼容 DTO，但不影响当前可选性，因此不再占主标签。
- **明确不采用**：`formTemplateBizRelevanceVoList` 中的表单模型编号虽被实施平台展示，但当前请求主动忽略关联详情，F-002 不为填满列表而扩大响应；表单 JSON、节点配置和创建方式也不在候选行展示。

### 已发流程

仅核对 `参考代码/rsh-cloud-invest-power-system/src/views/GroupApproveManage/Submitted/index.vue` 及其直接引用组件，并用 V1 适配层交叉确认：

- 参考代码接口入口符号：`Api.schedule.getFlowInstanceList`。
- V1 已核实目标入口：`POST /web/flowInstanceApi/list`。
- 请求：顶层 `pagination: true`、`pages`、`size`；`data` 含 `useScope: invest`、`auditWayList: []`、状态集合 `await_sent/run/withdraw/termination/abandon/rejected/end`、公司关联筛选和名称搜索。
- 参考页面实际展示字段：标题取 `name || formName`，另有 `id`、`status`、`createDate`、`currentNodeName`、`currentAuditUserInfo`；当前处理人名称从 `currentAuditUserInfo.*.userList[].name` 汇总。

### 待发流程

仅核对 `参考代码/rsh-cloud-invest-power-system/src/views/GroupApproveManage/DueOut/index.vue` 及其直接引用组件，并用 V1 适配层交叉确认：

- 参考代码接口入口符号：`Api.approveManage.getTaskList`。
- V1 已核实目标入口：`POST /web/flowJobTaskLink/list`。
- 请求：顶层 `pagination: true`、`pages`、`size`；`data` 含 `taskStatus: waiting_send`、`auditWayList: []`、`useScope: invest`、业务关联筛选和实例名称搜索。
- 参考页面实际展示字段：标题取 `flowInstanceName || formName`，另有 `flowInstanceId`、`flowStatus`、`initiator`、`initiatorDate`；页面将 `rejected/withdraw/draft` 分别显示为“驳回/撤销/草稿”。
- V1 `TodoItem` 还确认目标返回可包含 `jobTaskId`、`nodeName`、`taskStatus`、`createTime` 等；F-002 只公开选择区实际需要的字段。

参考仓库的 `Api.*` 具体 URL 定义不在允许查看的 `GroupApproveManage` 边界内，因此没有越界读取；具体 URL 由 V1 已验证适配层交叉确认。实施时不得修改参考源码。

## 已批准技术方案

### 服务端配置

F-002 不接入 V1 的 Nacos 或配置中心。当前工作区采用“进程环境优先、项目根目录 `.env.local` 次之、非敏感默认值最后”的加载顺序：本机忽略文件已从 V1 当前 YAML 安全生成，并合入用户已确认的统一登录约定；`pnpm dev:b` 从项目根启动时自动读取，用户日常无需填写或导出任何 `TARGET_*`。真实值不得写入 Git、文档、测试或前端。

`cmd/sync-v1-target-config` 是可选维护工具：显式接收 V1 YAML 路径，读取已核实的 `target.apiGateway`、`target.platformCode`、`target.customerCode`、`target.loginAesKey`，保留既有本机登录配置并以 `0600` 原子更新 `.env.local`。命令本身不含 V1 绝对路径或敏感默认值，执行时不回显配置值；正常启动不依赖 V1 仓库持续存在。

| 配置项 | 用途 | 当前依据与状态 |
| --- | --- | --- |
| `TARGET_API_GATEWAY` | 目标平台 base URL | 必需；当前本机值由 V1 `target.apiGateway` 同步，进程环境可覆盖 |
| `TARGET_LOGIN_PASSWORD` | 目标测试环境服务端登录密码 | 必需且敏感；当前统一值仅保存于本机忽略配置，页面始终只传账号 |
| `TARGET_LOGIN_AES_KEY` | 登录密码加密 key | 必需且敏感；当前本机值由 V1 `target.loginAesKey` 同步，不硬编码、不记录日志 |
| `TARGET_LOGIN_CODE` | 目标环境登录 `code` | 必需且敏感；当前统一值仅保存于本机忽略配置，不进入浏览器 |
| `TARGET_PLATFORM_CODE` | 登录与普通目标请求的平台代码 | 可选；未设置时沿用已批准的 V1 默认 `200001` |
| `TARGET_TEMPLATE_PLATFORM_CODES` | 模板列表的顶层平台过滤 | 可选；未设置时沿用已批准的 V1 默认 `200001,999999` |
| `TARGET_CUSTOMER_CODE` | 首次登录所需租户代码 | 可选；允许首次为空，登录后优先采用用户或公司摘要中的租户代码 |
| `TARGET_SESSION_TTL` | 进程内会话绝对过期时间 | 可选；默认 `8h`，目标更早失效时由一次安全重登处理 |
| `TARGET_HTTP_TIMEOUT` | 单次目标 HTTP 请求超时 | 可选；默认 `120s`，沿用 V1 请求超时约定 |

V1 的 `LoginRequest` 虽有 `sign` 字段，但已核实的登录请求体没有发送 `sign`。F-002 不臆造该参数；若当前目标环境要求 sign、动态验证码或其他挑战，则必须先由用户说明服务端来源和刷新规则，再调整计划。

### SID 缓存与安全

- 先采用 Go 单进程内存缓存，key 为规范化后的账号，value 只含 SID、账号摘要、必要目标代码和 `expiresAt`。
- 使用每账号互斥锁实现并发去重；同一账号并发验证或列表请求只允许一个真实登录，其他请求复用结果。不同账号不互相阻塞。
- 默认绝对 TTL 为 8 小时，可由 `TARGET_SESSION_TTL` 覆盖；不主动为每次列表请求增加 `checkLoginStatus` 往返。
- 密码、AES key 和验证码不进入缓存条目，登录时从服务端配置读取；进程重启会自然清空所有 SID。
- 目标返回 HTTP 401 或已核实的会话失效业务码时，立即删除该账号缓存，只安全重登一次，并只重放原只读请求一次；第二次仍失败则返回明确错误，禁止循环。
- SID 绝不进入浏览器响应、URL、localStorage/sessionStorage、前端状态、错误消息、日志、指标标签或追踪字段。由于目标适配可能把 SID 放入 query/header/body，应用日志不得记录完整出站请求。
- Redis 继续延期：当前架构只有一个 Go 进程，没有跨进程共享或会话持久化需求；进程重启后重新登录是可接受行为。只有后续明确采用多实例部署或目标平台对登录频率有严格限制时，再单独评估 Redis。

### 后端 API

登录和三类列表不硬耦合成一个接口。所有响应都使用项目统一 JSON 包装；错误只返回稳定错误码、中文消息和是否可重试，不透传 SID、凭证或可能含敏感信息的目标原文。

#### 1. 验证账号

`POST /api/target/accounts/verify`

请求：

```json
{"account":"tester01"}
```

成功数据：

```json
{
  "verified": true,
  "account": {
    "account": "tester01",
    "displayName": "测试专员",
    "companyName": "示例公司"
  }
}
```

`displayName` 来自目标登录响应 `user.name`，`companyName` 来自 `companyVo.name`；不存在时允许为空。响应不得含 SID、customerCode、platformCode、内部用户 ID 或任何凭证。

#### 2. 流程模板列表

`GET /api/target/flow-templates?account=...&query=...&page=1&pageSize=20`

成功数据字段：

- `account`：当前账号，便于前端核对迟到响应。
- `items[]`：`id`、`flowName`、`code`、`groupName`、`flowStatus`、`statusText`、`typeName`、`updateDate`、`createDate`、`remark`、`flowCreateType`、`formExist`、`formTemplateCount`、`companyName`。`formTemplateCount` 只由响应 `formTemplateList` 长度派生，不公开表单内容；`companyName` 仅由已核实的模板公司关联 ID 与服务端公司目录名称匹配后产生，绝不返回 `otherBizId`、`companyId`。
- `page`、`pageSize`、`total`、`hasMore`：由目标顶层分页字段规范化；`statusText` 与 `hasMore` 为有明确来源的展示派生值。

#### 3. 流程实例列表

`GET /api/target/flow-instances?account=...&source=submitted|due&query=...&page=1&pageSize=20`

`source=submitted` 的 `items[]`：

- `id`、`name`、`formName`、`title`、`status`、`createDate`、`currentNodeName`、`currentAuditUserNames`。
- `title = name || formName`；`currentAuditUserNames` 从 `currentAuditUserInfo.*.userList[].name` 汇总。

`source=due` 的 `items[]`：

- `flowInstanceId`、`flowInstanceName`、`formName`、`title`、`flowStatus`、`statusName`、`initiator`、`initiatorDate`。
- `title = flowInstanceName || formName`；`statusName` 按参考页面已核实映射生成。

公共分页字段同模板列表。后端只接受 `submitted`、`due`，避免用一个模糊 source 值映射三类不同目标契约。页码小于 1、pageSize 不在 1～100 时直接返回参数错误，不请求目标平台。

### 错误契约与失败归属

| 场景 | API 行为 | 页面行为 | 归属 |
| --- | --- | --- | --- |
| 账号为空或分页参数非法 | `400 INVALID_ARGUMENT` | 字段就地提示或保持当前列表，不发目标请求 | 工具输入 |
| 必需服务端配置缺失 | `503 TARGET_CONFIG_MISSING` | 提示“服务配置不完整，请联系维护人员”，不进入已验证态 | 工具配置 |
| 目标拒绝登录 | `401 TARGET_LOGIN_REJECTED` | 账号保持未验证，提示“账号验证失败，请核对账号” | 目标平台拒绝 |
| SID 失效 | 后端失效缓存并重登、重放一次；再次失败为 `401 TARGET_SESSION_EXPIRED` | 首次透明恢复；最终失败时使账号验证失效并要求重试 | 目标平台会话 |
| 目标返回空列表 | `200`、空 `items`、正确分页 | 稳定空状态，不当成错误 | 正常结果 |
| 目标分页字段或响应结构异常 | `502 TARGET_RESPONSE_INVALID` | 保留当前已选兼容状态，提示“流程数据异常，请重试” | 工具适配，待核实目标变化 |
| 目标网络失败或 5xx | `502 TARGET_UNAVAILABLE` | 提示“暂时无法读取流程，请重试”，不回退 mock | 目标平台或链路，暂按工具问题处理 |
| 目标请求超时 | `504 TARGET_TIMEOUT` | 结束 loading，提示“读取流程超时，请重试” | 工具链路，待核实目标状态 |
| 浏览器取消或快速切换账号/来源 | Go 透传 `request.Context()` 取消；连接已断开时不再写响应 | 旧请求结果不得覆盖新账号/来源 | 正常交互 |

目标原始错误只允许写入受控的后端诊断上下文，并在输出前脱敏；公开错误不包含请求体、URL query、SID、密码或 AES key。工具无法确定是网络还是目标平台故障时，遵循产品原则先按工具问题说明。

### 前端替换边界

- 保留 F-001 的账号输入、三类来源、稳定候选区、虚拟列表、分隔线、吸顶返回入口、主题与内容滚动边界。
- 将账号验证的本地 mock 状态替换为 `accounts/verify`；账号编辑立即取消旧请求、失效验证、清空选择，并将不合法来源恢复为“新发起”。
- 保留当前 `FlowCandidateList` 的展示职责，新增最小数据源 props/emits：items、loading、error、hasMore、query-change、load-more、retry；不创建通用远程列表框架。
- 搜索输入使用约 250ms 防抖；空搜索也走真实分页。每次账号、来源或查询变化都增加 request version，并用 `AbortController` 取消旧请求；即使取消未及时生效，也以 account/source/query/version 四元组忽略迟到结果。
- 增量加载从第 1 页开始，每次追加前核对 request version，按模板 `id`、已发 `id`、待发 `flowInstanceId` 去重；达到 `total`、目标返回空页或当前累计数不再增长时显示到底。
- 首屏、追加、空、错误、到底都在同一稳定外壳内呈现。首屏错误提供重试；追加失败保留已有项并允许重试当前页。
- 面向用户的账号、验证和读取提示采用简洁业务语言，不展示“真实目标平台”“未登录真实平台”等技术证明式文案；稳定后端错误码由前端映射为简短提示。
- 三类来源共用 `96px` 固定虚拟行与 `480px` 列表视口，常见桌面一次完整显示 5 行；工具栏、列表和页脚组成 `574px` 稳定外壳。模板备注最多显示两行并通过 `title` 保留完整内容，空值显示“暂无备注”。
- 模板未选中时不展示“正常/不正常”状态标签；选中时只显示“已选择”。已发、待发仍保留各自已核实状态语义。
- 生产代码不再导入 F-001 候选 mock；mock 仅保留为测试 fixture。提交按钮仍只执行当前表单校验和静态边界提示，不保存计划、不导航到路径页。

## 有序内部里程碑

以下里程碑共同交付一个用户结果，不单独进入人工验收，也不扩张为 F-003：

1. **配置、目标客户端与会话**：完成环境配置校验、登录适配、内存 SID 缓存、并发去重、一次安全重登、脱敏日志及账号验证 API。
2. **真实模板读取**：完成模板请求/响应映射、分页与模板 API，接入“新发起”候选区。
3. **真实实例读取**：完成已发和待发的独立目标请求映射、统一实例 API、搜索分页与三类来源切换。
4. **交互收口与验证**：完成取消/迟到响应保护、增量状态、当前范围自动测试、文档和真实环境人工验收交接。

## 完成标准

- [x] 用户输入非空真实账号并点击验证后，后端真实登录；成功摘要准确，失败不伪装为已验证。
- [x] SID 只存在于 Go 后端进程，浏览器响应、前端状态/存储和应用日志均无法取得 SID。
- [x] 同账号并发登录被去重；缓存按账号隔离并遵守 TTL；会话失效最多重登和重放一次。
- [x] “新发起”读取并分页展示账号可见真实模板，字段与请求参数符合已核实证据。
- [x] “已发”“待发”分别读取真实实例，字段、状态和请求参数符合各自参考来源，不复用错误契约。
- [x] 搜索防抖、取消/迟到响应保护、分页追加、去重以及加载/空/错误/到底状态实际可操作。
- [x] 编辑账号或切换来源会清空不兼容选择，不会被旧请求回写；候选区几何和自动引导不回退。
- [x] 目标请求全程只读；不保存计划、不创建数据库记录、不修改目标平台、不进入路径选择。
- [x] F-002 当前范围测试、Go build、前端类型检查和生产构建通过。
- [x] 当前机器无需设置 `TARGET_*`，`pnpm dev:b` 自动读取已准备的本机忽略配置，页面只需输入用户名。
- [ ] 页面文案已去除证明式技术表达；模板候选展示名称、分类与分组（仅 `groupName` 有值时以内联标签高亮）、表单关联、备注和更新时间，不再展示编码或以启停状态作为主信息。
- [x] 候选列表使用 `96px` 固定行和 `480px` 视口，常见桌面一次显示 5 行，并保持虚拟滚动、稳定外壳和渐进引导。
- [x] 文档状态更新为 `ready_for_manual`，列出真实环境人工步骤并停止，不开始 F-003。

## 当前范围自动验证

所有新增测试必须位于根目录 `test/`，只运行 F-002 相关测试：

- `test/unit/backend/`：配置必填与脱敏、本机文件加载、环境优先级、非法文件与测试隔离、TTL、账号隔离、同账号并发去重、一次重登上限、登录/模板/实例 DTO 映射。
- `test/contracts/`：三个 API 的请求、成功字段、分页、模板公司名称与稳定错误码，以及任何响应均不含 SID。
- `test/integration/`：使用 `httptest.Server` 模拟目标平台，覆盖登录、模板公司关联 ID 到目录名称的服务端匹配、无名称空值、三类列表、空列表、HTTP/业务会话失效、401 后一次重登、超时、取消、坏分页和目标 5xx；另以临时 V1 YAML 验证安全同步、`0600` 权限与自动加载，不读取真实本机配置。
- `test/unit/frontend/`：搜索防抖、request version、账号/来源切换忽略旧响应、分页追加去重、到底和错误恢复；同时验证模板 DTO 公司名称映射、无名称、编码不展示与不参与本地搜索、备注空值/长值、分类/分组/表单关联、无主状态标签，以及 `96px` 行高与 `480px` 视口常量。
- `test/integration/`：结构契约确认生产候选区不再导入 mock，并保留稳定外壳、虚拟列表、吸顶返回入口和独立滚动边界。
- `test/manual/F-002.md`：真实账号和三类真实候选的人工核对清单，不记录账号、密码或 SID。
- `test/run-f002.sh`：聚合当前范围测试、Go build、`vue-tsc` 与 Vite 生产构建；不跑无关全量测试，不启动浏览器。

## 人工验收前提

当前机器的本机忽略配置已经准备完成。用户无需提供或填写连接、密码、AES key、code，只需准备可人工使用的真实用户名；任何实际账号与流程值都不写入仓库、文档或测试 fixture。

最小验收数据：

- 至少一个可成功登录并能看到一个简单流程模板的账号。
- 最好同一账号还各有一条已发和待发实例；若没有，可分别提供账号，但必须明确三类数据预期。
- 一个便于人工识别的最简单流程名称，以及各列表预期至少出现的标题或状态；只用于现场核对，不提交。

## 人工验收步骤

1. 在两个终端直接运行 `pnpm dev:b` 和 `pnpm dev:f`，无需先执行 `export`、复制配置或同步 V1。
2. 打开 `/plans/new`，输入空账号确认就地校验；输入错误账号确认不会进入已验证态。
3. 输入可用真实账号并验证，核对显示名、公司名和目标平台实际身份一致，页面不声称保存凭证。
4. 在浏览器网络响应和存储中核对没有 SID、密码、AES key、customerCode 或 platformCode。
5. 选择“新发起”，核对模板名称、分类/分组、表单关联、备注和更新时间，不显示编码；有已核实公司名称时显示小标签，无名称时不显示占位，选中后公司标签与“已选择”并存；确认未选择项不再显示“正常/不正常”，一次完整显示约 5 行。
6. 切换“已发”和“待发”，分别核对标题、状态、时间、当前节点/处理人或发起人字段与目标平台页面一致。
7. 快速修改搜索词、切换来源和切换账号，确认没有旧列表闪回、混入或覆盖当前选择。
8. 在读取中断开目标环境或使用可重现失败条件，确认 loading 会结束、错误可重试且不会展示 mock 数据。
9. 刷新页面或重启后端，确认浏览器没有保存 SID；后端重启后会安全重新登录。
10. 填完整表单并点击“创建并选择路径”，确认仍只显示 F-001 的静态边界提示，不保存、不跳转、不修改目标平台。

## 回退与失败边界

- F-002 没有目标平台业务写入和工具数据迁移；回退应用版本即可恢复到 F-001 已验收状态。
- 后端重启即可清除全部内存 SID，不需要清理 Redis、数据库或浏览器存储。
- 真实环境暂不可用时，页面保留错误与重试，不允许自动切换 F-001 mock 冒充真实数据。
- 若目标登录新增动态验证码、单点登录或每账号独立密码，停止实施并回到本计划补充凭证来源，不在当前切片内猜测绕过方式。
- 若已发或待发实际接口、字段与核实证据不一致，保存脱敏响应结构作为诊断证据并回到计划评审；禁止临时扩大到目标平台写操作或越界修改参考源码。

## 人工验收现场待提供

环境协议与非敏感默认约定已经用户批准沿用 V1。当前机器的连接与登录配置只保存在 Go 后端读取的 Git 忽略文件中，不写入仓库；进程环境仍可在未来安全覆盖。

用户现场只需提供可登录的最小账号和最简单流程；最好该账号至少各有一条可见模板、已发实例和待发实例。若需多个账号覆盖三类数据，现场分别说明预期即可，任何值都不记录到仓库。

## 状态记录

- 2026-07-27：用户明确回复“开始下一个任务”，批准开始 F-002 规划；状态进入 `preparing`。
- 2026-07-27：V1 与限定目标参考证据、范围、接口、安全边界、失败处理、测试和人工门禁已形成；状态进入 `awaiting_approval`，等待用户明确批准，不写 F-002 业务代码。
- 2026-07-27：用户明确回复“批准实施，环境配置沿用 V1，验收账号和流程我稍后现场提供。”；状态进入 `implementing`。本轮按“配置与会话、三类真实读取、前端接入、定向验证与文档门禁”完成全部勾选项，真实账号与流程延后到人工验收现场提供。
- 2026-07-27：三个 API、目标协议适配、后端 SID 隔离与缓存、三类真实分页前端和当前范围自动验证完成；状态进入 `ready_for_manual`，等待用户现场账号与流程验收，不开始 F-003。
- 2026-07-27：用户人工启动准备未通过，并明确要求沿用 V1 当前配置、统一登录密码与验证码由后端本机配置提供，日常启动不再手工填写 `TARGET_*`；状态从 `ready_for_manual` 返回 `implementing`，仅返工本机零手填启动准备。
- 2026-07-27：V1 YAML 映射、安全同步工具、环境优先的本机配置加载、当前机器忽略配置、定向测试和无 `TARGET_*` 的 `pnpm dev:b` 健康检查完成；状态重新进入 `ready_for_manual`，等待用户只输入真实用户名验收。
- 2026-07-28：用户人工验收反馈页面存在证明式技术文案、模板候选信息价值不足且一次可见条目过少；状态从 `ready_for_manual` 返回 `implementing`，仅返工 F-002 用户文案、模板信息层级与候选列表容量。
- 2026-07-28：实施平台模板列表与配置组件证据核实完成；模板 DTO 补充表单存在性与关联数量，页面文案、模板信息层级和候选容量完成返工，定向测试与构建通过；状态重新进入 `ready_for_manual`，等待用户视觉与现场数据复验。
- 2026-07-28：用户人工验收反馈“不要显示编码，属于哪个公司可以高亮”；状态从 `ready_for_manual` 返回 `implementing`，仅核实流程模板响应中的公司归属字段并返工候选行展示，不开始 F-003。
- 2026-07-28：已核实模板关联只返回 `formTemplateBizRelevanceVoList[].otherBizId`，公司目录返回 `id`、`name`；后端仅在服务端完成匹配并公开 `companyName`，前端移除编码展示和本地编码搜索、在有名称时以小标签高亮。无名称不显示占位；定向测试和构建通过，状态重新进入 `ready_for_manual`，等待用户现场复验。
- 2026-07-28：用户截图明确纠正：模板高亮应使用现有分类分组 `groupName`，不是额外公司目录名称；已发状态不得显示 `end`、`run` 等原始英文值。状态从 `ready_for_manual` 返回 `implementing`，仅移除错误公司目录路径、集中映射已发中文状态并调整候选行展示，不开始 F-003。

## 自动验证结果

- `./test/run-f002.sh`：通过。
- Go 定向测试：后端单元、API 契约、假目标集成全部通过；新增配置测试覆盖本机文件、环境优先级、缺失/非法配置和测试路径隔离，集成同步验证覆盖 V1 字段映射、`0600` 权限与清空 `TARGET_*` 后的完整加载。
- 前端单元：8 项全部通过；结构契约脚本通过，覆盖模板公司标签、无名称、编码移除、本地搜索边界、备注、无主状态标签、固定虚拟尺寸和过期文案清理。
- `go build ./cmd/server`、`pnpm --filter test-auto-pro-v2-web typecheck`、`pnpm --filter test-auto-pro-v2-web build`：全部通过。
- 自动验证只使用运行期生成的假目标值，未连接真实平台、未启动浏览器、未写入目标数据。
- 当前工作区已在不显示配置值的前提下从 V1 配置生成 `.env.local`；清空 `TARGET_*` 后完整性复检通过，`pnpm dev:b` 启动与 health 精确响应通过，验证结束后后端进程已停止。
