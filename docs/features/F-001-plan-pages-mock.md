# F-001 应用壳与测试计划页静态 mock

- 状态：implementing
- 产品依据：`docs/PRODUCT.md` 的“测试计划页面行为”
- 架构依据：`docs/ARCHITECTURE.md` 的“前端边界”
- 计划确认时间：2026-07-27

## 单一用户结果

用户可以在已验收的 Naive UI 应用壳内查看、筛选静态测试计划，并进入独立新建页完成一轮带联动和校验的静态表单体验。

## 范围

包含：

- `/plans` 测试计划列表，使用本地 mock 数据和 `NDataTable`。
- 按计划名称与计划状态进行真实本地过滤，并可清空筛选条件。
- 待配置、可运行、运行中、已完成四种状态，以及每种状态唯一对应的主要操作。
- `/plans/new` 独立新建计划页，包含返回、说明、账号静态验证、按来源选择流程对象、联动重置、条件字段和前端校验。
- 通过提示说明静态操作的边界，不保存、不伪造跳转或后端结果。
- 深浅主题、可收缩侧栏、统一表面、1px 分隔线和内容区独立滚动。

不包含：

- 后端 API、数据库、本地持久化、真实账号凭证或目标平台请求。
- 计划详情、编辑、删除、路径选择、启动执行和运行结果页面。
- Vue Flow、FormMaking、V1/V2 页面复制，以及超出 `GroupApproveManage/Submitted`、`DueOut` 及其直接引用组件的业务分析。
- 通用表格框架、通用表单框架或额外顶级导航。

## 设计依据

- [Naive UI 标签上置示例](https://github.com/tusen-ai/naive-ui/blob/main/src/form/demos/zhCN/top.demo.vue)：采用 `label-placement="top"`、24 列 `NGrid`、半行 `NFormItemGi` 与 `x-gap=24`。
- [Naive UI 标签左置示例](https://github.com/tusen-ai/naive-ui/blob/main/src/form/demos/zhCN/left.demo.vue)：作为已排除方案的对照；本页字段较多，不继续使用左置标签。
- [Naive UI Form API 与规则](https://github.com/tusen-ai/naive-ui/blob/main/src/form/demos/zhCN/index.demo-entry.md)：使用 `model`、`rules`、`path`、`trigger`、`first` 与 `show-feedback`，必填标记由规则推导。
- [Naive UI 自定义校验示例](https://github.com/tusen-ai/naive-ui/blob/main/src/form/demos/zhCN/custom-validation.demo.vue)：特殊反馈仍归属对应表项，不建立页面级错误汇总。
- [Naive UI warning 示例](https://github.com/tusen-ai/naive-ui/blob/main/src/form/demos/zhCN/abnormal-warning.demo.vue)：`warning` 仅用于非阻断提醒；必填缺失使用 `error`。
- V1 `frontend/src/views/WorkbenchView.vue`：只参考流程模板的名称搜索、整行选择和清晰选中态；不复制旧业务实现。

## 目标平台字段依据

本轮仅据此组织本地 mock 字段，不调用接口、不写入参考代码：

- 已发流程依据 `参考代码/rsh-cloud-invest-power-system/src/views/GroupApproveManage/Submitted/index.vue`：列表入口为 `Api.schedule.getFlowInstanceList`，使用分页参数 `pages`、`size`，筛选语义包含标题/流程名、发起人、开始与结束日期、状态；可确认的实例字段包括 `id`、`name`/`formName`、`status`、`createDate`、`currentNodeName`、`currentAuditUserInfo` 和 `flowProxyId`。
- 待发流程依据 `参考代码/rsh-cloud-invest-power-system/src/views/GroupApproveManage/DueOut/index.vue`：列表入口为 `Api.approveManage.getTaskList`，查询条件包含 `taskStatus: waiting_send`、分页参数 `pages`、`size`；可确认的实例字段包括 `flowInstanceId`、`flowInstanceName`/`formName`、`flowStatus`/`statusName`、`initiator` 和 `initiatorDate`。
- 上述接口名称只是 F-002 后续接入的核查依据；F-001 不提供地址、SID、请求适配或真实响应。

## 页面行为

### 测试计划列表

- 标题为“测试计划”，右侧唯一主要操作为“新建计划”，进入 `/plans/new`。
- 筛选项只有计划名称搜索和状态筛选；筛选条件实时作用于本地 mock 行，清空后恢复全部行。
- 表格字段为计划名称、流程名称、发起账号、路径数量、运行方式、定时时间、计划状态、最近运行结果、操作。
- 四种计划状态分别只显示一个主要入口：待配置为“继续配置”、可运行为“开始运行”、运行中为“查看运行”、已完成为“查看结果”。
- 行操作只显示静态原型提示；空结果使用 `NDataTable` 正常空状态。

### 新建计划

- 独立整页，顶部提供“返回测试计划”、标题“新建计划”和必要说明。
- 表单使用标签上置的 24 列栅格，常规字段各占 12 列，宽度不超过 960px；不使用卡片、背景块或阴影。
- 字段为计划名称、可输入的真实账号、流程来源、来源对应的流程模板/已发流程/待发流程、运行方式、并行最大并发数和定时启动。
- “验证账号”仅形成明确标注的本地静态已验证状态，不登录目标平台、不生成 SID；账号编辑后验证立即失效，并清空已选模板或实例。
- 未验证账号时“已发”“待发”不可选；“新发起”保留可选，但模板区提示先验证账号。
- 验证后按来源分别显示流程模板、已发流程和待发流程选择区；三类选择区均支持本地搜索、固定高度虚拟滚动、分批追加、空结果、加载中和到底状态。
- 切换账号或来源时清空不兼容选择，并使旧批次结果失效；页面不再出现统一“目标流程”标签或输入框。
- 并行最大并发数只在并行模式显示并校验 2 至 20；切换运行方式会清理已隐藏字段的旧校验；定时时间保持可选。
- 定时时间以“定时启动”开关控制，关闭后不渲染时间选择器并清空时间值。
- 表单在内容区居中，返回入口粘附在内容区左上方；页面和列表滚动时保持容易找到。
- 唯一主要操作为“创建并选择路径”。提交失败时错误仅显示在对应字段下，并短暂提示一次“请检查标红的必填项”；通过后只短暂提示“静态原型已完成校验，真实创建将在后续功能接入”。
- 不保存、不加入列表、不进入路径页；返回列表直接可用，不添加离开确认。

## 完成标准

- [x] `/plans` 与 `/plans/new` 路由可用，列表和新建页均处于已验收应用壳内。
- [x] 名称、状态筛选及清空真实作用于本地 mock 数据。
- [x] 四种状态各映射且只映射一个主要操作，点击后只显示静态边界提示。
- [ ] 账号静态验证、来源禁用与失效联动可操作，且没有真实登录或请求。
- [ ] 三类来源分别显示可搜索虚拟增量列表，切换时没有不兼容选择或旧结果。
- [ ] 定时启动、并行条件字段、居中布局、粘附返回入口和提交校验符合本轮范围。
- [x] 1366×768 桌面视口下表格可在内容区水平处理，不撑出 body。
- [x] 当前范围测试、前端类型检查和生产构建全部通过。
- [x] 状态更新为 `ready_for_manual` 并停止，不开始 F-002。

## 自动验证

- `test/unit/frontend/plan_filters_test.mjs`：名称/状态筛选和清空。
- `test/unit/frontend/plan_actions_test.mjs`：四种状态到唯一主要动作的映射。
- `test/unit/frontend/plan_form_test.mjs`：目标流程候选对账号和流程来源的依赖边界。
- `test/integration/plan_pages_structure.sh`：路由、两列上置标签、Naive UI `FormRules`、字段反馈、消息容器和静态边界契约。
- `test/run-f001.sh`：聚合当前范围测试、`vue-tsc` 与生产构建。

## 人工验收

1. 在 `/plans` 核对表格字段、四种状态、唯一行操作和无套盒的扁平布局。
2. 使用计划名称和状态筛选，核对组合过滤、清空和无结果状态。
3. 点击四种行操作，确认只出现清晰静态提示，不发生伪造业务跳转。
4. 进入 `/plans/new`，核对两列上置标签、返回入口、全部字段、账号/来源联动重置和并行条件字段。
5. 空表提交时确认错误只显示在对应字段下，页面底部没有常驻错误块，并且只短暂出现一次错误提示。
6. 核对目标流程在前置条件不足时禁用且不报错，定时时间无必填星号；切换串并模式后隐藏字段不保留旧错误。
7. 填写有效值后提交，确认只短暂显示静态成功提示，不保存、不进入路径页。
8. 在浅色与深色下核对表格、表单、分隔线、主题工具栏和侧栏收缩；确认 body 不滚动，宽表格仅在内容区水平处理。

## 状态记录

- 2026-07-27：`preparing`。
- 2026-07-27：范围和完成标准已形成，进入 `awaiting_approval`。
- 2026-07-27：用户明确回复“可以了 开始下一项任务吧”，批准路线图既定 F-001，进入 `implementing`。
- 2026-07-27：静态列表、新建表单及当前范围自动验证完成，进入 `ready_for_manual`；等待用户人工核对，不开始 F-002。
- 2026-07-27：用户反馈新建表单人工验收未通过，要求依据 Naive UI 官方示例改为上置标签两列栅格，并统一使用原生字段反馈与短暂 message；从 `ready_for_manual` 返回 `implementing`。
- 2026-07-27：表单返工与当前范围自动验证完成，重新进入 `ready_for_manual`；等待用户人工核对，不开始 F-002。
- 2026-07-27：用户再次反馈人工验收未通过，确认账号静态验证、来源映射、三类选择列表与定时启动交互；从 `ready_for_manual` 返回 `implementing`。
