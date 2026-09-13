# F-035 目标处理人异步核验与有效表单数据闭环

- 状态：awaiting_approval
- 产品依据：`docs/PRODUCT.md` 的目标平台事实、运行状态、表单数据和安全写入原则
- 架构依据：`docs/ARCHITECTURE.md` 的目标适配层、执行器门禁、回放存储和 form-runtime 边界
- 本切片性质：修复现有执行与数据准备链路；不修改目标平台源码、权限、流程配置，不新增目标写接口，不新增数据库表
- 排查样本：计划“欧阳改测试1005”（plan 29），运行号 6（run 97），路径 12（path 4221）

## 完成标准

- [ ] 目标节点已到达但处理人尚未异步生成时，系统进入“正在生成处理人”状态并执行有界轮询。
- [ ] 等待窗口内生成真实处理人后，使用目标返回的处理人继续执行，不使用计划账号、候选人或发起人账号代替。
- [ ] 等待窗口结束仍无处理人和待办时，路径明确标记为“阻塞”，不显示“已处理”、不无限等待、不重复发送写请求。
- [ ] `isSkip=true` 只有在目标事实已越过节点时才记录“目标已跳过”；`isSkip=false/未声明` 和 `run_node_choose` 无处理人均阻塞。
- [ ] `currentAuditUserInfo`、精确待办、实例当前节点和处理人匹配结果全部写入可回放诊断。
- [ ] 回放后的表单值经过运行时下拉选项绑定、最终回读和保存后，才进入最终 `ready`。
- [ ] 页面、路径配置和运行发起请求使用同一份最终有效表单数据；实际值、显示值、`__virtualName`、`__condition`、`__formPersonId` 一致。
- [ ] 回放失败、待补充、目标读取失败、选项不存在或路径版本冲突不会覆盖已有有效表单数据。
- [ ] 路径 12 的请求载荷、目标节点、处理人事实和表单数据来源可以在日志中逐项对照。
- [ ] 不改变一次写、写后重读、账号串行、运行快照、动作顺序、F-034 动作矩阵和 F-033 节点过渡语义。

## 根因和证据

### A. 无处理人是目标异步分配与工具等待策略共同造成

运行 6 / 路径 12 的 `submit` 返回 `HTTP 200`、`isSuccess=true`，实例状态为 `run`，当前节点为“行政综合部-考勤管理”，但返回的 `currentAuditUserInfo` 为空，紧接着的精确待办查询为 0。见 [step.log](/Volumes/oygsky/AIstudy/test-auto-pro-v2/logs/runs/运行_6__欧阳改测试1005__run-97/paths/路径%2012__path-run-151__path-4221/step.log:6) 和 [curl.log](/Volumes/oygsky/AIstudy/test-auto-pro-v2/logs/runs/运行_6__欧阳改测试1005__run-97/paths/路径%2012__path-run-151__path-4221/curl.log:4)。

同一运行的路径 6 也先进入相同节点，稍后目标才生成待办并由刘慧玲执行同意，证明目标平台的扩展属性处理人计算是异步的。路径 12 的目标节点配置为 `auditType=extendedAttribute`、处理规则“集团考勤管理员”、`isSkip=true`，不是要求工具传入固定人员的 `run_node_choose` 节点。

工具侧根因有三点：

1. 提交成功后只读取一次处理人事实，没有给目标异步建任务留出有界等待窗口。
2. `resolveTaskSnapshotForStep` 在事实为空时正确拒绝冒用账号，但上层把“节点仍在、无处理人、无待办”错误归类为“当前待办已经处理”。
3. `run_orchestration.go` 对固定规则节点不要求 `nextAuditorList` 是正确的，但也没有记录“等待目标自动分配”的预期，更没有提交后验证该预期是否兑现。

因此，目标平台负责异步生成延迟；工具负责过早结束等待和错误状态呈现。不能仅修改登录账号或放宽处理人回退。

### B. 表单值问题是回放没有完成运行时绑定，不是路径配置写入失败

当前 `CompleteReplayItem` 已在同一事务中更新回放明细和 `test_execution_path_configs.effective_form_data`，因此不能把问题定性为“回放结果未落账”。见 [history_replay_repository.go](/Volumes/oygsky/AIstudy/test-auto-pro-v2/internal/repository/mysql/history_replay_repository.go:1107)。

路径 12 的数据库事实显示回放明细与路径配置均为 `ready`，`data_revision=2`，但 `branch_patches=[]`，`effective_form_data` 中的 `vacateType=事假` 与原始快照相同。回放服务在没有浏览器 runtime 时走 `HISTORY_RUNTIME_VALIDATION_DEFERRED`，仍将数据标成最终 `ready`，没有执行下拉选项请求、选项 ID 绑定和最终值回读。见 [history_replay.go](/Volumes/oygsky/AIstudy/test-auto-pro-v2/internal/service/history_replay.go:501)。

因此责任划分为：

- 原始快照读取和路径补丁落盘：当前链路正常。
- 下拉框远程选项绑定、显示值/实际值同步和最终快照确认：工具缺失闭环。
- 页面重新打开显示旧值：页面读取的是已经落盘、但尚未完成运行时绑定的快照。
- 目标平台没有参与这次配置数据生成，不能把该问题归因于目标平台。

## 实施任务

### T01 记录目标节点分配预期

- 在目标图节点事实中保留 `auditType`、`isSkip`、节点名称、自动分配方式和是否要求显式人员。
- 构造 `AssignmentExpectation`：`explicit_person`、`target_auto_assign`、`target_skip_allowed`、`manual_branch_entry`。
- 对路径 12 这类扩展属性节点记录“目标自动分配，提交后等待目标事实”，不构造虚假 `nextAuditorList`。
- 对 `run_node_choose` 缺少已保存人员、手动分支缺少入口、动作专属人员缺失，保存或启动前直接阻塞。

### T02 实现提交后的有界处理人核验

- 提交成功后按实例 ID和目标节点读取实例事实与精确待办。
- 轮询 3 至 5 次，总等待不超过 10 秒；每次等待时间固定、可配置但有上限。
- 成功条件只有：目标返回本节点 `currentAuditUserInfo` 并匹配真实用户，或精确待办返回真实处理人。
- 目标节点已越过且 `isSkip=true` 时记录 `target_skipped`，不发送写请求。
- 仍在当前节点、没有处理人和待办时返回 `assignment_pending`；超时后返回 `assignment_missing` 并停止路径。
- 轮询期间不得重复 submit/approve；写请求只允许原有一次写路径。

### T03 统一真实处理人发现

- `currentAuditUserInfo` 是已发实例视角的首选事实，待办任务是辅助核对事实。
- 处理人匹配顺序保持目标 ID、手机号、姓名；匹配后再取得目标登录会话。
- 删除或禁止配置候选人、计划账号、发起人账号作为当前处理人的隐式回退。
- 记录节点、用户 ID、姓名、账号、任务 ID、来源、匹配方式和查询时间；不记录凭证。

### T04 修正运行状态和用户文案

- 增加结构化 `stopKind`：`assignment_pending`、`assignment_missing`、`target_skipped`、`target_rejected`、`write_uncertain`。
- “无处理人”不能进入“已处理”或“结果待确认”；只有写出后响应无法确认才是 `write_uncertain`。
- 页面文案必须包含节点名称、目标当前状态、等待/阻塞原因和下一步建议。
- 运行详情展示目标事实证据，不展示内部阶段名，不让 `currentAuditUserInfo` 缺失退化成泛化提示。

### T05 建立最终有效表单状态

- 将回放状态分成“基础数据已生成”和“最终表单已确认”；`HISTORY_RUNTIME_VALIDATION_DEFERRED` 不得伪装成最终 `ready`。
- 有 FormMaking 下拉、远程选项或人员关联字段的路径，必须经过 runtime 最终绑定和回读。
- 无需远程绑定的 `vue_custom`/NoFormFlow 数据可以沿用现有后台校验，但仍需保存最终快照摘要。

### T06 完成下拉框值闭环

- 注入基础值后等待目标选项请求完成。
- 按目标实际值或显示值唯一匹配选项，写回实际值和显示值。
- 同步 `__virtualName`、`__condition`、`__formPersonId` 及同前缀关联字段。
- 触发必要的联动事件后再次读取运行时当前值。
- 选项不存在、重复或值被异步刷新覆盖时，返回具体字段的阻塞问题，不保存为 `ready`。
- `FormRuntimeFrame` 需要返回“最终值已应用”的确认，而不是只返回 iframe 加载完成。

### T07 将最终值保存为路径唯一数据源

- 复用现有 `SaveData` 和 `test_execution_path_configs.effective_form_data`，不新增表。
- 保存时校验路径修订、数据修订和回放来源修订；发生冲突时拒绝覆盖并标记受影响。
- 运行上下文只读取 `effective_form_data`；当状态为最终 `ready` 时禁止回退原始快照。
- 日志记录数据来源、`data_revision`、运行时确认状态和摘要指纹，不记录完整表单正文。

### T08 补齐页面刷新和回放任务反馈

- 回放完成后刷新路径状态和表单数据版本，不仅刷新回放任务计数。
- 路径列表明确显示“基础数据已生成/最终表单已确认/需打开表单确认”。
- 打开表单后若发现旧快照，页面必须显示未完成原因并触发一次最终值确认，不允许静默展示为完成。

## 测试任务

测试按根目录 `test/` 分类保存，只运行本切片相关测试。

- `test/unit/backend/assignment/`：异步处理人出现、超时、节点越过、`isSkip`、`run_node_choose`、手动分支入口、禁止账号回退、有限轮询和不重复写。
- `test/unit/backend/history_replay/`：回放状态拆分、最终快照保存、失败不覆盖、路径修订冲突、重复回放幂等。
- `test/unit/frontend/form_runtime/`：下拉实际值/显示值、虚拟字段、人员关联、异步覆盖、选项不存在阻塞、最终值确认。
- `test/integration/f035_target_facts_mysql_test.go`：回放结果、路径配置、`data_revision` 和运行上下文读取的一致性。
- `test/contracts/f035/`：阻塞 DTO、处理人来源、数据版本和敏感字段边界。

## 人工验收

1. 重新运行“欧阳改测试1005 / 路径 12”，观察目标处理人异步生成期间的状态；生成后应继续，超时后应阻塞。
2. 确认路径 12 不使用骆蒙恩或计划候选人冒充“行政综合部-考勤管理”的处理人。
3. 对比路径 6，确认正常延迟生成待办的流程仍能继续。
4. 在表单配置中修改下拉框值，保存、关闭、重新打开，确认实际值和显示值均保持修改结果。
5. 发起流程前后对比页面有效值与 submit 请求中的值，确认数据来源和 `data_revision` 一致。
6. 模拟回放失败、选项不存在和路径版本变化，确认不会覆盖旧的有效表单数据。
7. 检查日志能区分“目标正在生成处理人”“目标未生成处理人”“目标明确拒绝”和“写结果待确认”。

## 不在范围内

- 不修改目标平台的扩展属性、组织架构、权限或待办生成代码。
- 不把目标平台的异步延迟简单改成固定 sleep 或无限重试。
- 不使用计划账号绕过真实处理人事实。
- 不新增目标接口、数据库表、全局缓存或第二套表单模型。
- 不修改已经完成的历史运行事实；旧运行只读展示，F35 从新运行和新回放开始生效。

## 实施顺序和门禁

1. T01-T04 先完成处理人事实、轮询和阻塞语义，停在自动验证。
2. T05-T08 再完成表单运行时确认、最终快照和页面刷新。
3. 测试和静态检查通过后停在 `ready_for_manual`。
4. 只有用户确认人工验收通过后，F35 才进入 `accepted`，不得自动开始下一功能。
