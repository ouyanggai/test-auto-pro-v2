# F-036 二次核对与修复施工方案（原生装载与七个动作闭环）

- 审查基线：`48f7ced`（第一次审查不通过，结论见 `docs/features/F-036-independent-executor-native-page.md`）
- 本次核对范围：`48f7ced..17ca7e3`，即 `bd8b1a3`、`6988dbf`、`e3b13d3`、`942a5a4`、`9806ab9`、`a5c280e`、`afc807d`、`d7c0989`、`1cccf63`、`ba8c194` 等"P1 问题 1-8 修复完成"提交
- 核对方式：不采信提交说明与功能文档结论，直接读当前 HEAD 源码 + 独立快照
  `executor-service/flow-source/`（`test`/`be355ae`）+ 执行服务实际装载的构建产物 `executor-service/flow-source/dist.zip`；
  未调用真实目标写接口，未修改任何代码或状态
- 结论：**八个 P1 没有一项真正闭环**。其中 P1-1、P1-4、P1-5、P1-6 未修；P1-2、P1-3、P1-7 属"改了一半、方向正确但落地错误"；P1-3、P1-7、P1-8 的改动引入了新的确定性故障或口径矛盾。
  当前 HEAD 仍不能达成"通过原生页面完成七个动作"。

## 一、逐条判定

| 编号 | 事项 | 判定 | 一句话依据 |
| --- | --- | --- | --- |
| P1-1 | 所有装载最终都会失败 | **未修** | 捕获脚本挂在 `window.Vue/window.app`，产物只暴露 `window.$vue`；`server.ts:297` 只是把 `null` 赋回去，`:358` 仍必然 500 |
| P1-2 | 精确选模板用了错误 ID、单模板被阻塞 | **未修（换了个错 ID）** | 选择改用 `flowProxyId`（`server.ts:320`），但 URL 仍写 `flowTemplateId=formProxyId`（`:111`）；0 模板被当成"单模板直接加载"，且没有任何对照列表的校验 |
| P1-3 | 重提走错模式、taskContext 不全 | **未修（新增硬故障）** | `runner.go:101-102` 已改 `edit`，但 `loader.go:123` 仍只允许 `initiate/approve/forward` → 重提在 Go 侧构造装载载荷时必失败；URL 仍只有 `mode/jobTaskId`（`server.ts:119`） |
| P1-4 | 动作调用落不到真正业务组件 | **未修** | 仍在容器直接子级按名字找 `FlowDialog`/`EnterpriseExamineOpinion`（`server.ts:443-444`）；审批侧组件名是 `ExamineOpinion` 且深两层；找不到即静默跳过（`:447-481`） |
| P1-5 | 配置数据可能完全未应用却继续执行 | **未修（语义反了）** | `optionsReady: false`（`server.ts:355`）会让 `applyNodePatch` 直接抛错；表单 API 宿主层级仍然错；找不到接口时 `if (api)` 静默跳过；回读值被丢弃 |
| P1-6 | 人员、意见、选择没有接通 | **未修（被显式推迟）** | WS 执行只读 `action`（`server.ts:411`）；`choicePending` 从未置 `true`；`choice_required` 事件从未发出（全仓 grep 为 0） |
| P1-7 | 完成判定仍会误报 | **未修（端点写错，更严重）** | `completion.ts:14/15` 写入了两个**不存在**的端点，且删掉了正确的 `/web/flowInstanceApi/reSubmit`；`__businessError` 是幻影标志，无人写入 |
| P1-8 | 自定义表单、NoFormFlow 绕过新服务 | **半修 + 口径矛盾** | 路由已放开（`executor.go:84`），但 `executor.go:116` 仍以 `FormProxyID == ""` 拦截无表单；服务端只支持 FormMaking 形状 |

补充（第一次审查未点名、但同属阻断级）：

- **主写识别会被"权限校验 POST"污染**：重提链路先调 `checkFlowPermission()`，它 POST 的正是 `/web/flowInstanceApi/submit`（`EnterpriseExamineOpinion.vue:1316`）。只按 URL+方法判定主写，会把权限校验当成"流程写已受理"，完成判定随之失真。
- **`applyNodePatch` 抛错会变成装载 500**：`optionsReady: false` 时该函数必然抛 `UnsupportedFormError`；当前组件探测层级写错（多数情况探不到 → 走 `if (api)` 静默跳过），但一旦探测通过，装载块没有对该错误做 `unsupported` 分类，就会直接返回 500。

## 二、必须先读的目标平台事实（照做前逐条核对）

统一路径前缀：`executor-service/flow-source/src/`（下称快照）。这些是原生页面自身的行为，不是推测。

### 2.1 全局句柄与薄桥接落点

- `main.js`：`window.$router` / `window.$store` / **`window.$vue = app`（根实例）**。产物中可验证：`app.js` 内存在 `prototype.$initFlow=ko`、`prototype.$flowDetail=Co`、`window.$vue=zo`；`window.Vue=`、`window.app=` 在产物中**零赋值**（只有 `if (window.Vue) window.Vue.use(...)` 这种只读判断）。
- `components/InitFlowDialog/index.js`：`Vue.prototype.$initFlow = createDialog`、`Vue.prototype.$flowDetail = flowDetailDialog`；
  `createDialog` 返回 `{ instance, close, setData }`，`flowDetailDialog` 返回 `{ instance, close }`。**要的是 `instance`**。
- 因此正确捕获方式：从 `window.$vue` 取其原型（`Object.getPrototypeOf(window.$vue)`），包装该原型上的 `$initFlow`/`$flowDetail`，把 `created.instance` 存进 `window.__executorNative`。当前 `server.ts:93` 的 `window.Vue || (window.app && window.app.constructor)` 永远取不到东西。
- `page-origin.ts:85` 也有一份 `injectIdentityScript`，**没有被 import**（`server.ts:12` 只引入 `extractPageDist/serveFlowPage`），是死代码；它用 `window.$vue.$children.find(name==='InitFlowDialog')` 也是错的：`$initFlow` 把弹窗挂在独立 div 上（`parent: this`），不在任何组件的 `$children` 里。两份必须合并成一份并删除死代码。

### 2.2 发起链

- `views/FlowInitiate.vue`：`mounted` 里 `this.$initFlow({ title, params: { flowTemplateId: params.flowTemplateId, companyId: params.companyId, ...params }, onSuccess, onClosed })`；**`params.auditWay` 才是查询条件**，`flowTemplateId` 只是透传。
- `components/InitFlowDialog/InitFlowDialog.vue`：
  - 只在 `params.auditWay` 存在时 `getFlowByTemplate(auditWay)`；
  - 列表走 `Api.schedule.getFlowTemplateList` → `/web/flowTemplateApi/list`；
  - `res.data.length == 0` → `this.$message.error('暂无流程权限，请联系管理员')`，**不发生任何加载**；
  - `== 1` → `getFlowFindById(res.data[0].id)`，**`flowList` 保持 `[]`**；
  - `> 1` → `this.flowList = res.data`，打开 `chooseFlowDialog`，确认后 `confirm(id)` → `getFlowFindById(find.id)`；
  - 详情走 `Api.schedule.flowTemplateFindById` → `/web/flowTemplateApi/findById`，成功后 `flowJson = res.data`、`approveDialogVisible = true`。
  - 组件名：本组件 `InitFlowDialog`；其模板内直接子组件 `FlowDialog`（`name: 'FlowDialog'`，`v-if="approveDialogVisible"`）。
- 发起动作落点：`views/GroupApproveManage/Submitted/components/FlowDialog.vue`
  - `sumbitFlow('draft')`（模板第 31/32 行按钮）、`sumbitFlow('submit')`（第 33 行按钮），方法定义在第 1052 行；
  - 表单数据接口：`this.$refs.OtherSteps2.$refs.generateForm`（第 391/459/666 行），`OtherSteps2` = `Submitted/components/OtherSteps2.vue`，通过 `ref="OtherSteps2"` 挂载。

### 2.3 详情链

- `views/FlowDetail.vue`（路由页，`name: 'FlowDetail'`）：读 `mode` 与 `taskContext`，`switch(mode)`：
  - `audit` → `isExamine=true`，`operaType='examine'`（费用类无表单在 `strictEditAuditWaySet` 且 `formExist` 不成立时走 `'edit'`），`taskStatus='pending'`；
  - `edit` → `isReInitiate=true`，`operaType='reEdit'`，`taskStatus='edit'`；
  - `view` → `operaType='check'`；
  - `taskContext` 解析：优先 `taskContext` query 的 JSON，否则从逐字段 query 里取（字段清单见源码 `fieldNames`，含 `auditWay/formExist/formProxyId/flowProxyId/flowNodeProxyId/nextNodeProxyId/nextNodeName/nextNodeType/auditPassLogicFlag/branchExecuteType/lastCountersignFlag/initiatorId/flowInstanceName/flowInstanceBizRelevanceList/currentPendingNodeName/tracking` 等）；
  - `mode==='edit'` 时 `data.row = taskContext`，并 `applyEditEntryContext`（写 `flowName/selectFlowName/flowStatus/btnVisible`）；
  - `data.isExamine/isReInitiate/operaType/actionType/taskStatus` 都来自上面的映射，**执行服务不需要自己拼 data**，但必须把 `mode` 与 `taskContext` 传到位。
- 弹窗实例来自 `$flowDetail(...)`，其 `instance` 是 `layout/components/globalDialog/components/flowDetail.vue`：`name: ""`（**空名字**，任何 `name === 'FlowDetail'` 的查找都不可能命中），模板直接子级是 `EnterpriseExamineDialog`（`v-if="examineDialogVisible"`）和 `ExamineDialog`（无表单，`v-if="ExpensesClaimFormVisible"`）。
- `views/GroupApproveManage/components/EnterpriseExamineDialog.vue`：表单接口 `ref="generateForm"`（`fm-generate-form`），数据 `editData: {}`；模板内含 `EnterpriseExamineOpinion v-if="visible"`（组件名 `ExamineOpinion`，见 `EnterpriseExamineOpinion.vue:271`）。原生自己就是用 `this.$parent.$parent.$refs.generateForm` / `this.$parent.$parent.editData`（`EnterpriseExamineOpinion.vue:589/598`）取表单与数据的，说明宿主是 **EnterpriseExamineDialog**。
  → 审批/重提侧的动作宿主是 `ExamineOpinion`，且它在 `flowDetail → EnterpriseExamineDialog → <component :is> 包装 → ExamineOpinion`的深处；**不能按层数取**（设计 §4 明令禁止），必须从捕获实例递归遍历 `$children` 按 `$options.name` 命中并断言唯一。
- 动作方法真名（`EnterpriseExamineOpinion.vue`，全部照原生按钮走）：

| 动作 | 原生按钮 | 应调方法 |
| --- | --- | --- |
| 同意 | `<el-button @click="submitCheckBefore('pass')">同意`（第 42 行） | `submitCheckBefore('pass')`，内部：`$confirm` → `$refs.approveForm.validate` → `formMakingFormBusiness(true,'pass')`（第 745 行） |
| 不同意 | `submitCheckBefore('no_pass')`（第 52 行） | `submitCheckBefore('no_pass')` |
| 审批暂存 | `@click="temporaryStorage"`（第 57 行） | `temporaryStorage()`（第 587 行，无参） |
| 重提 | `v-else-if="isReInitiate"` 分支的 `saveSubmit('submit')`（第 87 行） | `saveSubmit('submit')`（第 1135 行；`isReInitiate` 时转 `reSubmitFormMakingFormBusiness(true)` → `handleRePostSubmit`） |
| 重提保存草稿 | 同分支 `saveSubmit('save')` | `saveSubmit('save')` |
| 转发 | `confirmTranspond`（`flowTypeMixin.js:127`，混入 `ExamineOpinion`） | 必须先有 `batchSelectDatas[0]`、`currentFormData`、`transpondFlowInstance`、`approveForm.personId/personName/checkList`（`transpond(type)` 第 60 行负责准备），否则必抛错 |
| 选择继续 | 并行 `handleSaveParallelChooseNode`（第 655 行）、分支 `handleSaveBranchChooseNode`（第 695 行）、自选 `getSelectPerson(data)`（第 626 行） | 同名方法；`getSelectPerson` 需要 `{checkboxPersonGroup:[{id,name,nodeProxyId}]}` |

- 审批意见入口是 `approveForm.approveMessage`（`rules.approveMessage` 必填），最终进 `auditRecord.executeDesc`（第 598/982 行）；下一节点人员进 `nextAuditorList`（第 318 行），构造为 `{name, bizId, auditDetailType:'personnel', nodeProxyId}`。

### 2.4 端点真名（`api/index.js`，决定完成判定与主写识别）

| 语义 | 真实端点 | 备注 |
| --- | --- | --- |
| 提交/保存草稿/权限校验 | `/web/flowInstanceApi/submit` | 三个语义共用同一端点（`saveFlowInstance`），草稿靠 `data.status='draft'` 区分 |
| 审批（同意/不同意） | `/flowInstanceApi/audit` | `submitTask`，**无 `/web` 前缀** |
| 审批暂存 | `/web/flowInstanceApi/storageFormData` | 方法名叫 `temporaryStorage`，端点叫 `storageFormData` |
| 重提 | `/web/flowInstanceApi/reSubmit` | `saveFlowInstanceAgain` |
| 转发 | `/web/flowInstanceApi/transpond` | |
| 模板列表/详情 | `/web/flowTemplateApi/list`、`/web/flowTemplateApi/findById` | 计划绑定的"模板 ID"就是这里返回的 `id` |

**`/web/flowInstanceApi/reEdit` 与 `/web/flowInstanceApi/temporaryStorage` 在参考项目与快照中都不存在**（全仓 grep 为 0），当前 `completion.ts:14/15` 把它们当成主写端点是错的，且删掉了正确的 `reSubmit`。

## 三、修复要求（按顺序，一项一提交）

### 第 1 步：装载捕获（P1-1）

1. 删除 `page-origin.ts:85` 的死实现，唯一实现留在执行服务内。
2. 注入脚本改为：`const root = window.$vue`（带轮询等待，直到出现且其原型上有 `$initFlow/$flowDetail`），
   `const proto = Object.getPrototypeOf(root)`（找不到就用 `root.constructor.prototype` 兜底），
   包装 `proto.$initFlow`、`proto.$flowDetail`，把 `created.instance` 分别写入 `window.__executorNative.initFlow/flowDetail`。
   包装必须是幂等的（重复注入不得二次包装）。
3. 装载成功判据不能只看"实例非空"：发起侧要等到 `instance.approveDialogVisible === true && instance.flowJson && instance.flowJson.id`；详情侧要等到 `flowDetail` 的 `examineDialogVisible === true` 或 `ExpensesClaimFormVisible === true`。等不到就按超时阻塞，**不得**把 `loaded` 置真。
4. 保留 `server.ts:358` 的 nativeHost 门禁（它现在正确地挡住了假装载），修的是捕获，不是门禁。
5. 同时在该脚本里注册 `window.onerror`、`unhandledrejection` 监听，把页面异常写入 `window.__executorPageErrors`（第 7 步要用）。

验收：装载一次发起会话与一次审批会话，返回 200 且 `/status` 能读出 `identity`、`loaded=true`；对错误路由（如缺失 `auditWay`）必须 500/阻塞而不是假装成功。

### 第 2 步：模板精确选择（P1-2）

1. 装载载荷新增 `flowTemplateId`（专指计划绑定的**流程模板 ID**，即 `/web/flowTemplateApi/list` 返回的 `id`），
   Go 侧 `LoadPayload`/`LoadInput`/`RunAction` 同步；`server.ts:111` 的 URL 参数与选择逻辑都改用该字段。
   现有 `formProxyId`（表单代理）与 `flowProxyId`（发布流程代理）都**不是**模板 ID，不得互相顶替。
   - 说明：Go 侧目前只有 `runCtx.FormProxyID` 与 `runCtx.FlowProxyID`（`run_orchestration.go:542` 把 `plan.TargetObjectID` 当 `FlowProxyID`），
     需要按 `FlowSource` 把新发起路径的计划目标对象 ID 作为模板 ID 传下来（新发起来源配置就是从 `flowTemplateApi/findById` 读的，见 `internal/service/target_read.go:317`），并写测试锁定。
2. 选择判据改为**对照真实列表**，而不是看 `flowList` 是否为空（空列表既可能是单模板，也可能是无权限）：
   - 从 request trace 里取 `/web/flowTemplateApi/list` 的响应体（`isSuccess` + `data[].id`）；
   - 响应失败、`data` 为空 → 阻塞，消息与原生一致（无权限），**绝不继续调用 `getFlowFindById`**；
   - `boundTemplateId` 不在返回集合 → 阻塞，日志带上"计划绑定 ID + 实际集合"；
   - 集合只有 1 项 → 等原生自加载完成后核对 `instance.flowJson.id === boundTemplateId`；不一致则调 `instance.getFlowFindById(boundTemplateId)` 纠正（这正是精确保守的语义）；
   - 集合多项 → 调 `instance.confirm(boundTemplateId)`（原生会关选择框并加载详情）。
3. `native-initiate.ts:31-33` 的 `list.length === 0 → direct` 分支必须删除或改为"由列表证据决定"，不能让"无权限"走成"直接加载"。

验收：给两条同 `auditWay` 的模板，装载必须命中计划绑定的那一条（记录实际 ID）；给一条不存在的 ID 或空列表，必须阻塞。

### 第 3 步：重提模式与上下文（P1-3）

1. `loader.go:123` 允许 `LoadModeEdit`；同时给 edit 装载加校验：必须带实例 ID（`id/flowInstanceId`），
   并根据 `EnterpriseExamineOpinion.saveSubmit('submit')` 的真实载荷决定是否强制 `jobTaskId`（**先读代码再定**，不要照抄其它动作的门禁）。
2. 详情 URL 必须携带完整 `taskContext`（设计 §3.2）：按 `FlowDetail.getTaskContext` 的可解析形状序列化成 JSON 放 `taskContext` query。
   至少覆盖：`id/flowInstanceId`、`flowInstanceName`、`auditWay`、`jobTaskId`、`formProxyId`、`flowProxyId`、`flowNodeProxyId`、
   `formExist`、`nextNodeProxyId`、`nextNodeName`、`nextNodeType`、`nextAuditorList`、`auditPassLogicFlag`、
   `branchExecuteType`、`lastCountersignFlag`、`initiatorId`、`tracking`、`flowInstanceBizRelevanceList`、`currentPendingNodeName`。
   缺失字段按"原生打不开对应动作入口 → 阻塞"处理，不要用空对象糊过去。
3. `server.ts:118-121` 现在只写 `mode/jobTaskId`，视为未实现。

验收：`mode=edit` 的装载在 Go 侧不再报"装载模式只支持 initiate/approve/forward"；页面上 `operaType==='reEdit'`、`isReInitiate===true`、`btnVisible===true`（可从 `flowDetail` 实例读出来做断言）。

### 第 4 步：动作落到真实组件（P1-4）

1. 写一个"从捕获实例递归找唯一组件"的工具函数：按 `$options.name` 命中，返回实例或抛"找不到/多命中"。
   发起侧找 `FlowDialog`；审批侧找 `ExamineOpinion`（**不是** `EnterpriseExamineOpinion`）。
2. 分发必须按第 2.3 节的真实入口表，且**找不到方法要阻塞**（`unsupported`），不允许 `if (x && typeof x.f === 'function')` 静默跳过（`server.ts:457-478` 现在就是静默跳过）。
3. 同意/不同意：原生入口会弹 `$confirm`。二选一并在文档里写死：
   - A：调 `submitCheckBefore(type)`，同时在页面内自动确认 Element MessageBox 的主按钮（`page.evaluate` 内触发 DOM 事件，不使用 Playwright 定位器，符合设计 §2.1 对"禁止用定位器点业务控件"的约束）；
   - B：复刻原生的校验链（`$refs.approveForm.validate` → `formMakingFormBusiness(flag,type)`），并把"跳过 $confirm"记录为已知差异。
   推荐 A（保留原生校验与同意/不同意分支），但必须写清例外理由。
4. 转发：先准备 `approveForm.personId/personName`（来自 Go 的接收人事实）、`checkList`（`fuyan/yijian`）、
   以及 `batchSelectDatas[0]/currentFormData/transpondFlowInstance`（走原生的 `transpond('backlog')` 等价准备步骤或用有效载荷等价构造），
   再调 `confirmTranspond()`；任何前置缺失按阻塞处理，不允许"先调了再说"。

验收：每个动作都能在 trace 里看到**该动作应有的那一笔写请求**（端点见 2.4），且后置请求（附言等）按序出现。

### 第 5 步：配置数据应用与回读（P1-5）

1. 表单 API 宿主按侧分别解析：
   - 发起：`FlowDialog.$refs.OtherSteps2.$refs.generateForm`；
   - 审批/重提：`EnterpriseExamineDialog.$refs.generateForm`（原生自己就是这么取的，见 `EnterpriseExamineOpinion.vue:589`）。
   解析不到 → `unsupported` 阻塞，不得跳过补丁而继续动作。
2. `applyNodePatch` 的 `optionsReady` 语义是"选项已就绪"（`data-patch.ts` 在 `false` 时抛 `UnsupportedFormError`）。
   正确做法：**等待就绪**（轮询：补丁后再无在途目标请求且稳定窗满足），就绪后 `setData` → 等联动请求收敛 → `getValues` 回读。
   永远不要硬编码 `true`，也不要在装载时传 `false` 让它必然抛错。
3. `changedFields` 为空时同样要回读完整值（设计 §3.4 第 6 步），把回读结果放进装载结果/日志，不得丢弃。
4. 装载块里的 `UnsupportedFormError` 必须映射为 `unsupported`（阻塞且说明原因），不能落成 500 通用错误。

验收：带补丁的装载能读回被改字段与虚拟字段（`<key>__virtualName`）；不支持的组件返回 `unsupported` 而不是 500 或"成功"。

### 第 6 步：动作上下文接通（P1-6）

1. WS 执行分支必须消费 Go 已经发出的字段（`runner.go` 已发，`protocol.ts:82` 已声明）：`auditDesc`、`branchSelections`、`nextNodeAuditors`、`choice`。
   - 审批/重提前把 `auditDesc` 写入动作宿主的 `approveForm.approveMessage`；
   - `nextNodeAuditors` 写入 `nextAuditorList`（`{name,bizId,auditDetailType:'personnel',nodeProxyId}`），空人员一律不写、直接阻塞；
   - `branchSelections` 按原生 `manualChooseNodes`/`parallelNodeChooseList` 的形状落到实例上（先在原生代码里核对键名，不要自创）。
2. 选择要求事件：在 trace 发现流程写响应 `isSuccess === false` 且 `data.errorType ∈ {custom_choose, parallel_choose, run_node_choose}` 时：
   - 先置 `session.choicePending = true`（`server.ts:413/510` 依赖它，现在恒为 false，等于永不等待）；
   - 再向 Go 发 `event` 类型 `choice_required`，`data` 原样带目标响应 JSON（Go 的 `OnChoiceRequired` 只认这个事件）。
3. `choice_continue` 处理：宿主是**第 4 步解析出的业务组件**（不是 `nativeHost` 容器），并且必须把 `choice.nextAuditors` 作为载荷传给 `getSelectPerson`；
   `custom_choose/parallel_choose/run_node_choose` 分别落到 `handleSaveBranchChooseNode/handleSaveParallelChooseNode/getSelectPerson`。只允许继续一次（保留 `canContinueChoice`）。

验收：构造一条手动分支路径与一条自选人员路径，能看到 `choice_required` 事件、继续后只多一笔写请求、且没有硬超时。

### 第 7 步：完成判定（P1-7）

1. `completion.ts:10-15` 的主写端点改为：`/web/flowInstanceApi/submit`、`/flowInstanceApi/audit`、`/web/flowInstanceApi/storageFormData`、`/web/flowInstanceApi/reSubmit`、`/web/flowInstanceApi/transpond`。
   删除 `temporaryStorage`/`reEdit` 两个不存在的端点。
2. 主写识别不能只看 URL+方法：`/web/flowInstanceApi/submit` 同时承担草稿、正式提交与**权限校验**（`checkFlowPermission`）。
   至少要按载荷语义区分（提交/草稿带 `data.id|null`、`data.status`、`formDataMongoVo`；权限校验带 `data.checkPermissions`），
   否则权限校验的 200 会把"没有提交"判成"流程写已受理"。
3. 删除幻影 `__businessError`（`server.ts:504`，全仓无人写入）。改用第 1 步注册的 `__executorPageErrors` 与原生成功回调
   （`FlowDialog @success`、`flowDetail` 的 `callback(payload)`、`postMessage('RSH_FLOW_EVENT','flow-action-done')`）。
4. `businessDone` 与 `finalize` 的口径必须自洽（`completion.ts:160-164`）：要么把"业务完成信号"做成真信号，
   要么把判据改成"页无异常 + 后置请求收敛"，不能留一个永远为 `false` 的输入让所有动作都落到 `business_failed`。

验收：四个端点各跑一次能判 `success`；权限校验型 POST 不会让判定变成功；人为断网得到 `uncertain` 且后续写被锁。

### 第 8 步：无表单口径（P1-8）

当前状态是"路由放开、能力没做、门禁还在拦"，两处口径相反。二选一，写进功能文档，不要两说：

- 方案 A（本切片做）：实现无表单路径——`flowDetail → ExamineDialog`（`EnterpriseExamineOpinion` 的无表单版本在 `views/GroupApproveManage/components/examineOpinion.vue`，审批走 `submitTask` → `/flowInstanceApi/audit`），
  同时放开 `executor.go:116` 的 `FormProxyID == ""` 门禁（改为按 `RenderType` 选择宿主与数据接口）。
- 方案 B（本切片不做）：`shouldRunViaRuntime` 改回"仅 FormMaking"，功能文档明确"自定义表单/NoFormFlow 仍走既有路径，不在本切片"。
  方案 B 与第一次审查的整改要求冲突，采用它必须由用户明确批准并同步修订 F-036 的范围与完成标准。

### 第 9 步：清理与文档订正

1. `docs/features/F-036-independent-executor-native-page.md` 中"P1 问题 1-8 修复完成 ✅"整段必须替换为本文件的判定表（不得保留与代码不符的 ✅）。
2. 功能文档状态保持 `implementing`；未通过人工验收前不得写 `ready_for_manual`。
3. 提交说明不得再出现"修复完成"这类未经真实证据支撑的结论。

## 四、测试换血（否则同样的假修会重复发生）

现有 `test/integration/executor-service/` 下的 `detail-load.test.ts`、`data-patch-readback.test.ts`、`actions.test.ts` 等，
断言的是**另一套实现**：它们只调 `resolveDetailSemantics/applyNodePatch/dispatchNativeAction` 这类纯函数，而生产代码走的是
`server.ts` 里 `page.evaluate` 的 switch 分支。这类绿灯不能证明任何闭环。

必须新增（可离线、可复现）：

1. **真实装载用例**：启动执行服务（真实 dist），装载 `/flow/initiate` 与 `/flow/detail/:id`，
   断言捕获到实例、选中绑定模板、`taskContext` 生效（`operaType/isReInitiate`），并断言**不再 500**。
2. **动作分发用例**：在真实页面上下文里触发七个动作，断言调用到第 4 步解析出的同一个业务组件实例，
   目标写请求经 stub 转发端点出去，端点与载荷字段与 2.4/2.3 一致；找不到组件时必须是 `unsupported`。
3. **选择闭环用例**：stub 返回带 `errorType` 的响应 → 断言 `choice_required` 事件、`choicePending`、继续命令只发一次、继续后写请求数正确。
4. **完成判定用例**：覆盖权限校验 POST 不计主写、四个真端点、SID 失效、写响应丢失、超时锁。
5. 删除或改写与生产实现不一致的旧用例（保留"测了别的实现"的绿灯等于自欺）。

## 五、人工验收前置（重启服务后按此顺序）

1. 启动 `pnpm --dir executor-service start`，健康检查通过。
2. 发起一条计划绑定模板与同 `auditWay` 其它模板共存的路径：必须打开绑定的那一条（把实际模板 ID 打进日志）。
3. 保存草稿：目标"待发"里能看到草稿，流程名称是工具流程名。
4. 提交：进入审批链，实例可读。
5. 审批侧：暂存、同意、不同意各一次，目标审批记录里能看到配置的审批意见文本。
6. 重提：待发实例能被重新提交（`/web/flowInstanceApi/reSubmit` 出现一笔写）。
7. 转发：接收人来自配置；先 `transpond` 后附言，附言只在第一步确认成功后发出。
8. 手动分支 / 自选人员路径：`choice_required` 后能继续，且只继续一次。
9. 断开执行服务后再放行七个动作之一：写前阻塞，配置页仍可用。
10. 无表单/自定义表单：按第 8 步选定方案的结论验证（要么真能跑，要么明确 `unsupported` + 文档口径一致）。

## 附录：禁止的"假修"清单（本轮实际出现过）

- 把 `null` 赋回去当作"已修复"（`server.ts:297`）。
- 用错误的字段顶替（`formProxyId`/`flowProxyId` ↔ `flowTemplateId`）。
- 常量加了、校验没改（`LoadModeEdit` vs `loader.go:123`）。
- 用不存在的端点名凑清单（`reEdit`/`temporaryStorage`），并删掉正确的 `reSubmit`。
- 用幻影全局标志当判据（`__businessError`）。
- 参数语义写反（`optionsReady: false`）或写死（`true`）。
- `if (found) {...}` 静默跳过，把"没找到组件/接口"变成"动作成功"。
- 测试只覆盖纯函数，而生产分支是另一套实现。
