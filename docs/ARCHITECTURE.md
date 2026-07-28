# 技术架构

## 固定选型

- 前端：纯 Vue 3、Vite、Vue Router、Pinia、Naive UI。
- 后端：Go 单体，一个二进制进程；F-000 使用标准库 `net/http`。
- 包管理：pnpm。
- 本地端口：前端 `19000`，后端 `19080`；前端开发服务器将 `/api` 代理到后端。
- Go module：`test-auto-pro-v2`。

不采用 Vben Admin、Element Plus 或其他中后台应用基座。历史 PRD 中相反描述已被本文件取代。

## 本地开发方式

- 根目录 `package.json` 仅提供全栈开发与检查命令；Go 依赖仍只由 `go.mod` 管理。
- 根目录 `pnpm-workspace.yaml` 只包含 `web`，从根目录执行 `pnpm install` 只生成一个 `pnpm-lock.yaml`。
- 前端在一个前台终端执行 `pnpm dev:f`，Vite 监听 `127.0.0.1:19000` 并提供热更新。
- 后端在另一个前台终端执行 `pnpm dev:b`，实际命令为 `go tool air -c .air.toml`。Air 固定为 Go 1.25 tool dependency，构建 `cmd/server` 到被忽略的 `.runtime/`。
- 两个前台进程的日志留在各自终端，用户通过 `Ctrl+C` 停止；不使用后台守护、PID 文件、日志文件、`concurrently` 或后台重启命令。
- `.air.toml` 只监听项目 Go 源码，排除 `.git`、`.runtime`、根目录 `node_modules`、`web`、`参考代码`、`test`、`docs` 和历史资料。

## 系统边界

```text
浏览器 -> Vue 单页应用 -> Go HTTP API -> 目标平台适配层 -> 目标平台
                              |
                              -> 工具侧存储（后续功能）
```

- `cmd/server`：唯一服务入口。
- `internal/api`：HTTP 路由、参数校验和响应。
- `internal/engine`：后续执行状态机与调度。
- `internal/adapter/target`：后续唯一可直接调用目标平台的区域。
- `internal/analyzer`、`internal/builder`：后续表单与人员分析、数据生成。
- `internal/repository`：后续存储访问，不包含业务判断。
- `internal/model`：领域类型。
- `internal/config`：配置加载。
- `web`：单个 Vue 应用。

## 前端边界

- 应用壳只提供产品名、三个中文导航、主题切换和路由内容区；不在 F-000 实现计划列表、创建页或其他业务页面。
- 应用壳使用 Naive UI 文档站式的扁平布局：顶栏横跨全宽，侧栏从顶栏下方开始，主内容占用剩余空间；`NLayoutHeader bordered` 与 `NLayoutSider bordered` 负责肉眼可见的 1px 语义分隔线，不使用卡片套层、渐变、阴影或装饰图形。
- 顶栏、侧栏、工作区与主内容均使用未嵌入的 Naive `NLayout` 层级，浅色和深色下保持同一基础表面色；普通 DOM 不依赖 Naive 组件局部 `--n-*` 背景或边线变量。
- 侧栏使用 `NLayoutSider` 的 `collapse-mode="width"`、`show-trigger="arrow-circle"` 与 `collapsed-width="0"`；圆形触发器真实点击区至少为 `32px × 32px`，零宽收缩态通过定位覆盖完整留在 viewport 内，收缩时不展示无依据图标。
- 浏览器根节点固定为 `100dvh` 且不滚动；顶栏固定，侧栏可收缩，主内容区独立 `overflow-y: auto`，侧栏仅在菜单超出时自身滚动。Flex/Grid 子项使用最小尺寸约束，避免撑出 viewport。
- 通过 `NConfigProvider`、`NGlobalStyle` 与 `darkTheme` 实现深浅主题；默认浅色，主题值保存到 `localStorage`，所有表面、文字、边线和选中态使用 Naive UI 语义变量随主题切换。
- 保持中文、桌面优先；计划列表继续使用 F-001 的静态计划行，新建页从 F-002 起通过 `web/src/features/plans/api.ts` 读取真实账号与候选，不建立通用表格、表单或远程列表框架。
- F-003 将计划列表切换到 Go API 和 MySQL 真实数据；创建成功后进入独立 `/plans/:id/paths` 路由。该路由在 F-003 只读取计划摘要和展示无路径空状态，不提前创建流程画布或通用 CRUD 页面。
- Vue Flow 与 dagre 统一延后到 F-004；F-000 不安装也不预留画布抽象。
- 只有真实复杂流程证明 dagre 不可读时，才另行批准评估 ELK.js。

## F-002 目标平台只读接入

- `internal/config` 按“进程环境 > 项目根目录 `.env.local` > 非敏感代码默认值”的顺序读取 `TARGET_*`。`.env.local` 被 Git 忽略且只在当前机器保存；Air 从项目根启动时会自动读取，不修改进程环境，也不进入浏览器。
- `TARGET_API_GATEWAY`、`TARGET_LOGIN_PASSWORD`、`TARGET_LOGIN_AES_KEY`、`TARGET_LOGIN_CODE` 必需；本地文件不存在时仍允许纯环境变量运行。本地文件解析失败、关键配置缺失或 AES key 长度非法时，服务和 health 正常启动，三个 target API 稳定返回 `TARGET_CONFIG_MISSING`。
- `cmd/sync-v1-target-config` 只在维护时读取显式指定的 V1 YAML，将目标网关、平台/租户代码和 AES key 与既有本机登录配置合并，使用 `0600` 临时文件原子替换 `.env.local`，并在清空当前进程 `TARGET_*` 后做完整性检查。命令不含 V1 绝对路径或登录值，不回显配置值；正常启动不依赖 V1 仓库。
- `TARGET_PLATFORM_CODE` 默认 `200001`、`TARGET_TEMPLATE_PLATFORM_CODES` 默认 `200001,999999`、`TARGET_SESSION_TTL` 默认 `8h`、`TARGET_HTTP_TIMEOUT` 默认 `120s`，均来自用户批准沿用的 V1 非敏感约定；`TARGET_CUSTOMER_CODE` 可为空。敏感配置没有代码默认值。
- `internal/adapter/target` 是唯一拼装目标 URL、加密登录密码、传递 SID 和解析目标响应的区域。SID 按目标协议进入 body、query、header，但应用不得记录完整出站 URL、header、body 或目标原始报文。
- `internal/session` 使用单进程内存缓存，按去除首尾空白的账号键控，默认绝对 TTL 为 8 小时；每个账号独立锁定登录，不同账号不互相阻塞。会话失效后只允许删除缓存、重登和重放当前只读请求一次。
- 缓存条目只保存 SID、必要账号摘要和目标代码，不保存密码、AES key 或 code；进程退出自然清空。F-002 不引入 Redis，多实例共享需求出现后再单独评估。
- 对浏览器提供三个独立边界：`POST /api/target/accounts/verify`、`GET /api/target/flow-templates`、`GET /api/target/flow-instances`。公开响应只含验证摘要、候选 DTO、分页或稳定错误，不含 SID、凭证、customerCode、platformCode 或目标敏感原文。
- 模板、已发、待发分别映射 `/web/flowTemplateApi/list`、`/web/flowInstanceApi/list`、`/web/flowJobTaskLink/list`；三类 DTO 独立，不用同一个模糊目标类型覆盖字段差异。
- 模板 DTO 只补充实施平台列表已使用的 `formExist` 与 `formTemplateCount`；`groupName` 是模板既有分组字段，前端只在其非空时以内联标签高亮，不请求公司目录或公开额外公司字段。已发 DTO 保留原始 `status` 并提供集中派生的中文 `statusName`。候选虚拟列表固定行高 `96px`、视口 `480px`，常见桌面一次完整显示 5 行，加载、空、错误与来源切换共用 `574px` 稳定外壳。
- 前端搜索以 250ms 防抖触发真实分页请求，通过 `AbortController` 和 account/source/query/version 联合身份取消或忽略旧结果；追加按三类真实 ID 去重，错误不回退 mock。

## F-003 最小计划持久化

- 工具侧计划使用 MySQL；Go 通过 `database/sql` 和 MySQL 驱动访问，不引入 ORM。`internal/repository` 定义计划仓储边界，MySQL 实现只负责迁移、SQL 和行映射，业务校验与幂等语义位于 `internal/service`。
- 工具数据库连接只读取 `PLAN_DB_*`，配置优先级沿用“进程环境 > 根目录 `.env.local` > 非敏感默认值”。本机配置从 V1 `runnerDb` 安全同步，禁止使用目标平台 `target.mysql*` 作为工具库。
- F-003 建议在同一 MySQL 服务上创建独立数据库 `test_auto_pro_v2`；数据库名只允许字母、数字和下划线。同步和启动日志不得输出 DSN、用户名密码或配置值。
- 版本化 SQL 通过 `schema_migrations` 记录；后端在监听端口前完成数据库连接、建库和向前迁移，失败则直接退出。迁移不自动 down，不修改 V1 原数据库或目标平台数据库。
- F-003 只建立 `test_plans`。`scheduled_at IS NULL` 表示未开启定时，串行计划的 `max_concurrency` 为 `NULL`；时间统一以 UTC 保存并用 RFC 3339 传输。
- 创建使用 `Idempotency-Key` 和数据库唯一键保证同一次网络重试返回同一条计划。公开响应不返回幂等键、SID、目标凭证或数据库内部字段。
- 执行路径表延期到 F-005：F-004 先用保存的账号、来源和目标对象 ID 重新读取真实流程结构，F-005 再依据真实节点和分支建立路径数据，不在 F-003 保存无业务依据的占位路径。

## F-004 只读真实流程图

- `GET /api/plans/{id}/flow-graph` 是唯一公开图读取入口。服务端先读取计划，再使用计划内账号、来源和目标 ID 调用目标平台；浏览器不得覆盖这些字段。
- `new` 先通过模板列表精确核对可见性，再调用 `/web/flowTemplateApi/findById`；`started` 通过已发列表精确取得当前 `flowProxyId`；`pending` 通过待发列表精确取得当前 `flowProxyId`。后两类再调用 `/web/flowProxy/findById`，禁止把实例 ID 当代理 ID。
- 多个目标只读调用放在 F-002 `session.Manager.DoRead` 的一次操作中；任一调用发现会话失效时，整个核对与详情读取链只允许重新登录并重放一次。
- `internal/adapter/target` 解析最小目标树；`internal/analyzer` 将 `childFlowNodeTemplate`、`conditionNodes`、`parallelNodes` 规范化为独立图 DTO。API 不透传目标 envelope、代理 ID、审批配置、字段权限或凭证。
- 分支解析以真实节点 ID 去重，以真实策略 ID 标识分支边；各分支末端连接到分支节点的同一个主线后继来表达汇合，不创建虚假业务节点。循环、关键 ID 缺失、空分支入口或超过 500 节点/200 深度时拒绝半图。
- 前端使用 `@vue-flow/core`、`@vue-flow/controls` 与 `@dagrejs/dagre`，布局方向固定为 `TB`。条件、手动分支和并行节点作为不可见布局路由点，分支使用正交树干展开；同一路由点的分支按后端边顺序严格从左到右。只允许画布平移、缩放、适配视口和进入/退出全屏；关闭节点拖动、连接、删除、选择和位置持久化。
- 画布使用 Naive UI 主题变量并与内容区保持同一表面色。首次图数据稳定后只执行一次根节点定位；后续重绘和窗口尺寸变化不得反复重置用户视口。
- 首次适配只把根节点置于画布上方并保持可读缩放，不把整张大图压进当前视口；右上角入口通过页面内 `fixed` 覆盖层铺满浏览器内容视口，不调用 Fullscreen API，退出页面全屏后保留当前浏览位置。
- F-004 不增加数据库迁移、不缓存图、不生成拓扑指纹和路径。F-005 才依据真实节点与分支建立路径持久化。

## 数据与部署演进

- MySQL 在 F-003 用于最小计划持久化；执行路径在 F-005 才进入 MySQL。
- MongoDB 延后到真实运行阶段；Redis 只在多实例共享会话或登录频率约束形成真实需求后评估。
- 当前不引入 Docker、微服务、独立 worker 或部署编排。

## 参考源码边界

- `参考代码/` 完整克隆 13 个 canonical 仓库，但被当前仓库忽略。
- 同步只能首次 clone 或在分支正确、工作树干净、未分叉时执行 `pull --ff-only`。
- `rsh-cloud-invest-power-system` 的业务分析只允许查看 `GroupApproveManage` 及其直接引用公共组件。
- 不复制旧项目脏工作树，不修改同步源码。
