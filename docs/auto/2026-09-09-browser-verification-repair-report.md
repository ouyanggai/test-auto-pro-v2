# 浏览器验证与返工执行报告

任务书：docs/auto/2026-09-09-browser-verification-repair-task.md
开始时间：2026-09-09
状态：进行中（总体未完成，不写收口）

## 0. 现场记录

- 基线提交：1fe7efa 文档：新增真实网页验证与功能返工任务书
- 未提交改动（任务书 2.3 列出的 8 个文件 + 1 个删除）：
  - form-runtime/src/App.vue、runtime/formTemplate.js、runtime/protocol.js、runtime/requestPolicy.js
  - internal/adapter/target/write_actions.go（其中 `approve` 同意动作载荷改动与表单遮罩无关，单独审查验证提交）
  - test/unit/frontend/form_runtime_test.mjs
  - web/src/features/path-configuration/FormRuntimeFrame.vue
  - web/src/views/PlanPathConfigurationView.vue
  - docs/CLAUDE_CODE_FORM_LOADING_FIX.md 已删除（保持删除，不恢复文件）
- 用户反馈：未提交改动导致“表单数据显示一直在加载中，都显示不了了”。
- 服务现场：19000（web）、19001（form-runtime dev）、19080（backend via air）均已在运行，health 正常；沿用，不重复起服务。
- 本机被忽略配置 .env.local 存在，沿用，不打印配置值。

## 1. 样本清单（按任务书 4.2 登记，实际名称/编号从网页读取）

| 样本号 | 计划及路径 | 真实分支组合 | 历史来源 | 关键字段前后值 | 动作链 | 节点处理人 | 模式 | 运行定位 | 结果/证据 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| H01—H10 | 合同盖章评审 10 条，逐条展开 | 待读取 | 待选择 | 待核对 | 待配置 | 待核对 | 待安排 | 待运行 | 未开始 |
| O01 | oyg流程测试 至少 1 条 | 待读取 | 待选择 | 待核对 | 待配置 | 待核对 | 待安排 | 待运行 | 未开始 |
| L01 | 请假单 至少 1 条 | 待读取 | 待选择 | 待核对 | 待配置 | 待核对 | 待安排 | 待运行 | 未开始 |

## 2. 问题记录（按任务书 12.2 模板逐个登记）

### 问题 1：表单数据工作区一直显示"正在加载表单运行时"，最终表单被销毁（工作包 A）

- 关联工作包/功能：F-024 表单装载生命周期；任务书工作包 A。
- 网页复现入口和点击步骤：测试计划 → oyg测试102 → 编辑 → 路径 1「配置节点」→「表单数据」。
- 预期：遮罩在总预算内关闭，表单可见、可保存。
- 实际：遮罩停留 60 秒后出现「表单 iframe 初始化响应超时」，iframe 会话被销毁只剩占位文案；此前表单内容实际已渲染完成（约 1.7 秒内网络请求全部结束）。
- 根因与证据（确认事实）：
  1. 父↔iframe 消息时序实测：boot 后 60.076 秒才收到 load 的 result，恰好晚于父页面 60 秒预算；父页面按预算销毁会话（postMessage 记录 + 页面快照）。
  2. iframe 内资源时序实测：73 个目标请求全部在约 1.7 秒内完成，最慢 430ms，"分钟级慢请求"假设不成立（performance.getEntriesByType）。
  3. 分段探针定位：卡点在 `refreshOptionFields` → FormMaking `refreshFieldOptionData` 的 `Promise.all` 内部，且挂起期间请求屏障 pending=0（无在途请求）。
  4. 目标源码核对：FormMaking `refreshOptionData` 只有 `fx` 与"带数据源键的 datasource"两个落定分支；合同盖章评审表的身份字段 `currentDepartment` 声明 `remote=true, remoteType=datasource` 但 `remoteDataSource=null`，走不到任何分支，返回永不落定的 Promise，拖死整次装载。
  5. 配置数据接口读取模板证实该控件形状（GET /api/plans/24/execution-paths/3165/configuration/data）。
  - 待验证假设：无（根因链完整）。
- 修改文件和行为：`form-runtime/src/runtime/formTemplate.js` 的 `refreshOptionFields` 现在只刷新 FormMaking 能落定的控件（datasource 型必须带数据源键、fx 型、模板未声明类型的简单 remote 控件），并给整次刷新调用加 10 秒有界兜底；`optionFieldDescriptors` 透传 `remoteType/remoteDataSource/remoteFx`。此类身份控件的选项来自宿主回显与联动，不依赖该刷新。
- 相似场景检查：既有 53 项表单运行时用例全部通过；新增回归用例锁定"缺数据源键的 datasource 控件不得被刷新、不得拖死装载"。
- 运行的测试/构建及结果：`node --experimental-strip-types --test test/unit/frontend/form_runtime_test.mjs` 53/53 通过；web 与 form-runtime typecheck、生产构建通过；`git diff --check` 干净。
- 浏览器复验结果：原失败路径装载 8.3 秒内完成，遮罩关闭、保存/恢复按钮可用；关闭重开、浏览器整页刷新后重开均复现一致；iframe 内不再出现占位或超时错误。
- 计划/路径/运行/步骤定位：计划 24（oyg测试102）路径 3165（路径 1）。
- 截图或日志的本地证据位置：本报告第 0、2 节记录（postMessage 时序与探针数据已在修复前采集）。
- 中文提交：见第 3 节提交列表第 1 项。
- 当前状态：已验证。
- 附带发现并已修复：19001 form-runtime dev server 文件监视失效、长期供给陈旧产物（验证浏览器消费的构建产物时发现），已重启并确认新产物被浏览器消费；重编译暴露 `10_000` 数字分隔符语法不被本仓库 Babel 支持，已改为普通数字字面量。

### 问题 2：合同分类显示值与路径条件不一致（控件显示历史分类"行政综合类"，右侧补丁为"施工类"）（工作包 B）

- 关联工作包/功能：F-024 关键字段协调；任务书工作包 B。
- 网页复现入口和点击步骤：同问题 1（路径 1 的分支组合即施工类>50万、盖章地=深圳）。
- 预期：控件选中值、显示名称、右侧路径关键信息三者一致为"施工类"。
- 实际（修复前）：右侧显示施工类，控件停留行政综合类；协调循环反复回填-被覆盖，装载时间被拖长并触发问题 1 的超时链。
- 根因与证据：问题 1 的装载挂死是本问题的主要放大器——协调完成前会话已被销毁，任何已取得的修正都看不到。挂死移除后协调自然收敛（确认事实：修复后控件显示施工类）。
- 修改文件和行为：本问题未新增独立修改；验证随问题 1 的修复完成。
- 相似场景检查：关闭重开、浏览器刷新后重开，控件均为施工类，无自触发反复改值。
- 运行的测试/构建及结果：同问题 1。
- 浏览器复验结果：控件"施工类" = 右侧路径关键信息"施工类" = 保存后重读一致。
- 六处事实核对进展：历史快照（行政综合类，来自历史实例）、路径补丁（施工类）、运行时控件（施工类）、运行时取值（保存通过）已核对；服务端重读与真实目标实例的提交值待运行阶段（H01）核对。
- 当前状态：已验证（配置态）；目标写入一致性归入 H01 运行证据。

## 3. 验证与提交记录

### 提交列表

1. （本次）修复表单装载挂死：跳过 FormMaking 永不落定的选项刷新控件并加有界兜底；接手表单遮罩生命周期未提交改动（含取消命令、会话代次、总预算与 iframe 屏障）；移除已被取代的旧修复文档。
2. （本次）补齐同意动作载荷：approve 分支与发送前校验（auditStatus=pass、实例/待办必填），含回归测试；与表单修复分开提交。

### 验证命令与结果

- `node --experimental-strip-types --test test/unit/frontend/form_runtime_test.mjs`：53/53 通过（0 跳过）
- `npm run typecheck`（web、form-runtime）：通过
- `npm run build`（web、form-runtime）：通过
- `go test -count=1 ./test/unit/backend/executor ./test/unit/backend/target ./test/unit/backend/action_orchestration`：通过
- `git diff --check`：干净
