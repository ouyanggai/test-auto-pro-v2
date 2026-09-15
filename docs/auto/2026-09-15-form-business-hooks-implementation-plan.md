# 表单业务前后置操作同步施工方案

## 一、交付目标

执行 Agent 必须把目标平台在表单提交前后做的业务操作补到工具中。完成后，工具每次操作的顺序必须与目标平台一致：

1. 读目标当前实例、当前任务、当前节点和当前表单。
2. 读当前节点字段权限。
3. 绑定远程下拉/人员/组织选项，得到最终表单值。
4. 执行表单校验。
5. 执行目标表单配置里的提交前业务脚本。
6. 发送一次流程动作请求。
7. 重新读取目标结果。
8. 执行附件绑定和提交成功后的业务脚本。
9. 再读一次表单和流程状态，把最终结果显示在运行详情。

流程名称完全使用工具平台保存的名称，禁止从目标接口读取、拼装或覆盖流程名称。

## 二、必须先读的文件

实施 Agent 开始前必须依次阅读：

1. `CONTEXT.md`
2. `docs/PRODUCT.md`
3. `docs/ARCHITECTURE.md`
4. `docs/TARGET_SEMANTICS.md`
5. `docs/features/F-012-history-form-replay-action-orchestration.md`
6. `docs/features/F-019-full-action-and-recovery-execution.md`
7. `form-runtime/runtime-source/src/views/GroupApproveManage/components/EnterpriseExamineOpinion.vue`
8. `form-runtime/runtime-source/src/views/GroupApproveManage/components/ExamineDialog.vue`
9. `form-runtime/runtime-source/src/views/GroupApproveManage/Submitted/components/FlowDialog.vue`
10. `form-runtime/runtime-source/src/views/GroupApproveManage/components/flowTypeMixin.js`
11. `form-runtime/runtime-source/src/views/GroupApproveManage/components/mixin.js`
12. `form-runtime/runtime-source/src/api/index.js`

只查看 `GroupApproveManage` 和它直接调用的公共组件；不要分析其他业务页面来扩大范围。

## 三、目标平台已有逻辑，逐项照做

### 1. 读表单

文件：`EnterpriseExamineDialog.vue`、`ExamineDialog.vue`、`components/mixin.js`。

必须实现：

- 用目标接口读取实例表单原始数据。
- 保留原始数字、数组、对象、附件字段，不把数字转成字符串。
- 对转发实例保留 `dynamicParam`，不能丢掉历史转发信息。
- 读取失败时停止当前动作，不能拿旧数据继续发送。

### 2. 读字段权限

目标接口：`/web/flowProxy/getFlowNodePower`。

必须实现：

- 请求参数使用真实 `flowNodeProxyId` 和当前任务信息。
- `fieldPower != hide` 的字段才允许修改。
- 当前节点隐藏字段与之前已审批节点隐藏字段合并。
- 重新发起时重新读取权限，不能沿用上次运行的权限。
- 权限接口失败时显示“无法确认当前节点可编辑字段”，停止发送。
- 任何脚本或前端配置都不能修改隐藏字段。

### 3. 绑定远程选项

表单控件配置中的 `remoteFunc`、`remoteOption` 是目标平台的取数入口。执行 Agent 必须：

- 找出运行时真正调用这两个字段的代码和接口。
- 按目标平台相同参数调用接口，记录完整请求和响应。
- 将选项的真实值、显示文字、虚拟字段、名称字段一起写入最终表单。
- 下拉值找不到对应显示文字，或同一个值对应多个选项时停止发送并指出字段名。
- 远程选项接口失败时保留原表单，不用空数组覆盖。
- 远程人员、公司、部门、岗位选项必须重新读取，不能只使用历史快照。

### 4. 执行表单校验

目标代码入口：`handleBeforeSubmit`、`generateForm.getData()`。

必须按以下顺序执行：

1. 先执行审批意见表单校验。
2. 再执行表单控件校验。
3. 校验失败时记录失败字段和中文原因，流程写请求次数必须为 0。
4. 校验成功后调用 `getValues()` 取得包含虚拟字段的完整数据。

### 5. 执行提交前业务

目标事件名只能使用：

- `beforeSubmitAndDraft`
- `beforeSubmitAndDraftNoBiz`

目标代码位置：`EnterpriseExamineOpinion.vue`、`Submitted/components/FlowDialog.vue`。

必须实现：

- 按目标顺序先执行 `beforeSubmitAndDraft`，再执行 `beforeSubmitAndDraftNoBiz`（目标代码没有调用的事件不得强行调用）。
- 传入完整表单、动作类型、当前实例 ID、当前任务 ID、当前节点真实 ID、业务 ID 和工具流程名称。
- 允许事件返回修改后的表单数据；返回结果必须再次检查字段权限。
- 事件返回失败、超时或格式错误时停止，流程写请求次数为 0。
- 事件不能修改流程名称、实例 ID、任务 ID、节点 ID、公司 ID 和接收人 ID。
- 每个事件单独记录开始时间、结束时间、完整请求、完整响应、返回状态和错误信息。

### 6. 发送流程动作

继续使用现有动作目录和 12 个白名单写接口。业务事件不得直接调用流程写接口。

每次动作只允许发送一次流程写请求。写请求必须包含：

- 目标实例当前完整表单数据。
- 当前节点声明允许修改且已经配置的字段。
- 当前真实任务 ID、节点 ID、流程代理 ID。
- 审批意见和下一节点人员（目标动作要求时才带）。

禁止发送：工具内部 `nodeKey`、空的任务 ID、`batchCode`、未配置人员、工具流程名称覆盖值。

### 7. 写入成功后的处理

只有目标写入“确定成功”并且重新读取到目标实例后，才执行后置处理。

必须按以下顺序：

1. 重新读取实例状态、当前节点、当前任务和当前表单。
2. 确认状态变化符合动作预期；不符合时标记“目标结果与预期不一致”，不执行后置脚本。
3. 发起动作执行 `afterSaveFlowInstance`，传入新实例 ID 和完整结果。
4. 审批动作重新读取 `auto_audit_info_*`、`auto_audit_info_obj_*`、`auto_audit_info_obj_list_*`，以目标返回值为准。
5. 绑定附件：发起使用实例 ID，审批使用任务 ID；关系 ID 不得混用。
6. 后置脚本失败时保留目标已改变事实，运行状态为“业务处理失败”，禁止再次发送流程动作。

### 8. 转发

目标代码：`flowTypeMixin.js`、`FlowDialog.vue`、`TranspondLog.vue`。

必须实现：

- 将已有 `dynamicParam` 与本次转发填写内容按目标顺序合并。
- 日志、附言和业务关联列表按目标字段写入。
- 从目标响应取得辅助实例 ID。
- 用目标列表接口确认辅助实例确实存在。
- 主实例和辅助实例分别记录，辅助实例失败不能伪造主实例前进。
- 转发接收人必须来自发送前实时读取的候选列表。

## 四、代码落点和执行顺序

### 第一步：建立目录和数据结构

新增：

- `internal/engine/hooks/catalog.go`：保存事件名称、适用动作、输入字段、输出字段和目标接口。
- `internal/engine/hooks/context.go`：保存一次动作的实例、任务、节点、表单、人员和附件数据。
- `internal/engine/hooks/runner.go`：按顺序执行前置和后置处理。
- `internal/engine/hooks/result.go`：保存每个处理步骤的开始、结束、请求、响应和结果。

不在动作适配器中复制第二套事件判断。

### 第二步：接入有效表单

修改 `internal/engine/step` 中构造表单数据的入口：

1. 读历史表单。
2. 应用当前路径字段补丁。
3. 读取远程选项并更新值/显示文字/虚拟字段。
4. 应用字段权限。
5. 输出一份完整有效表单。

所有动作共用这一入口。

### 第三步：接入前置处理

在现有动作写入前调用 `hooks.RunBefore`。调用失败直接返回，不进入写请求。把结果写入现有 step、attempt、network 和运行日志。

### 第四步：接入后置处理

在现有写后核验成功分支调用 `hooks.RunAfter`。核验不确定时直接结束为“结果待确认”，不得调用任何后置业务接口。

### 第五步：接入页面

沿用现有运行画布，增加三段显示：

- 提交前检查和业务处理
- 目标流程动作
- 提交后业务处理

显示中文动作名、实际操作人、接口结果、失败原因和下一步。不得显示 SID、密码、内部节点键或脚本源码。

## 五、日志要求

这是内网项目，日志不做脱敏。每个请求和响应必须原样写入：

- `step.log`
- `network.log`
- `curl.log`
- `recovery.log`（发生恢复时）

日志必须包含 SID、表单正文、请求头、响应正文、事件名、动作名、尝试序号和时间。凭证本身不得提交到 Git；本机运行日志不得提交。

## 六、失败处理（逐条执行）

- 读取失败：停止，不写目标。
- 权限失败：停止，不写目标。
- 远程选项失败或无法唯一匹配：停止，不写目标。
- 表单校验失败：停止，不写目标。
- 提交前事件失败：停止，不写目标。
- 目标明确拒绝：记录目标原始 code/message，停止，不执行后置事件。
- 目标响应丢失：状态为“结果待确认”，不执行后置事件，不重发。
- 写入成功但后置事件失败：状态为“业务处理失败”，不重发流程动作。
- 附件绑定失败：记录实例/任务关系和完整错误，不能假装绑定成功。

## 七、测试文件和验收

测试只能放在根目录 `test/`：

- `test/unit/backend/form_hooks/`：顺序、短路、字段权限、选项匹配、JSON 保真。
- `test/integration/form_hooks/`：目标只读接口、实例回读、附件绑定、转发辅助实例。
- `test/contracts/form_hooks/`：事件名、写接口白名单、禁止字段、流程名称隔离。
- `test/run-form-hooks.sh`：只运行以上测试。

执行 Agent 必须实际运行测试，并在结果中写明通过数量。真实写动作按现有人工门禁逐项执行，未真实验证的动作不得写入已验证清单。

人工验收必须完成：

1. 含远程下拉和附件的发起表单。
2. 字段无权限时提交被阻止。
3. 提交前事件失败时没有流程写请求。
4. 发起成功后检查实例回读、`afterSaveFlowInstance` 和附件绑定。
5. 审批暂存、同意、不同意、重新提交分别检查前后处理顺序。
6. 转发检查辅助实例独立记录。
7. 模拟写响应丢失，确认后置处理没有执行且没有重试入口。

## 八、禁止事项

- 不处理流程名称逻辑。
- 不修改目标平台源码、权限、数据库和基础设施。
- 不复制旧项目另建兼容接口。
- 不猜测目标接口参数；没有源码或真实请求证据就阻塞并记录。
- 不自动重发、不自动补偿、不替换实际操作人。
- 不把目标后置业务失败说成流程失败，也不把流程成功说成业务全部成功。
