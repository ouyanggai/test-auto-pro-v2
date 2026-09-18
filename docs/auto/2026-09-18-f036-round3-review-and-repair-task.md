# F-036 第三轮修复任务：从「跑完」到「跑对」（评审 + 施工令）

> 历史评审/施工令（2026-09-18 起废止）。用户已决定删除本文件要求的目标账号、固定模板/实例和真实动作台；本轮不继续实施 T1–T9 的特殊真实测试，改由用户页面人工验证后再规划问题。

> 读者＝执行 agent。本文件由第三方对你（上一轮执行方）2026-09-18 的收口结果做只读复核后写成。
> 结论以代码与真实运行日志为准，**不采信任何文档里的结论性表述，包括上一轮晨报和功能文档的完成点**。
> 本文件是施工令；你按 T1–T9 施工并自己跑出证据，判定口径以 §五 的门禁为准。

- 基线：`d9ec98a`（工作树干净）
- 上一轮施工令：`docs/auto/2026-09-17-f036-round2-unified-native-execution-repair-task.md`
- 上一轮晨报：`docs/auto/2026-09-18-f036-round2-run-report.md`（**其中 §一/§四/§五 的结论性表述已被本轮复核部分证伪，见 §一**）
- 本轮证据目录：`docs/auto/evidence-round3/`（新建；`evidence-round2/` 冻结归档）
- 本轮晨报：`docs/auto/2026-09-18-f036-round3-run-report.md`（施工结束后由你写）

---

## 一、复核结论（已核实的证据，直接用）

### 1.1 上一轮真实做成的（予以确认，不要重做）

| 项 | 证据 |
| --- | --- |
| 通道单连接复用 | `executor-service/src/forward-channel.ts:29` `attach()` 不再中止在途请求，改为关闭新连接；`internal/runtimeexec/client.go` 每会话一条 `sessionConn`。run 2 日志零「被新连接替换」。 |
| 接口终局判据（D2b） | `executor-service/src/completion.ts` 只按 `ACTION_WRITE_PATH` 期望端点响应判决；`server.ts:1039-1057` 的 debugWrites 已彻底排除页面信号；`test/unit/executor-service/completion.test.ts` 覆盖七值与选择闭环。 |
| 多次保存草稿走 edit（D1） | `internal/engine/step/executor.go:216` 主实例已存在时 `LoadModeEdit`；`native-dispatch.ts` `save_draft && loadMode==='edit'` → `saveSubmit("save")`；run 2 步 1–2 同实例无副本。 |
| 转发真实实现（D4） | `server.ts:161 prepareForwardList` 走原生 `transpond(tab)` + `confirmTranspond()`；完成检测含两笔写顺序门禁；本轮已补写后实例存在断言（见 T6）。 |
| 处理人＝计划账号本人时回填 accountKey | 提交 `616dbf2`，回归用例 `TestApproveAsSelfAssigneeKeepsRuntimeAccountKey`；run 2 步 5 日志实证：gate 用视角 `08e604bf…`（本人）查到待办，submit 阶段本人会话 `query_view_user=-` 但读到 1 行。 |

### 1.2 复核不成立、本轮必须修的问题

严重度：P0＝会导致错误判定或功能不可用；P1＝门禁/证据不诚实或跨切片无关架构残留；P2＝可读性与文档。

| 编号 | 问题 | 证据（文件:行 / 日志） | 影响 |
| --- | --- | --- | --- |
| **P0-1** | **run 2「20/20 步全部成功」不成立**：第 20 步（结束节点）根本没有执行。`step_id=20` 没有任何 `phase=submit`/`phase=prepare` 行，只有 `phase=plan`→`phase=gate`→`phase=verify`，实例已 `status=end`，节点与前一步相同（`6bf2a645…`）。 | `logs/runs/运行_2__…__run-2/paths/路径 1__path-run-2__path-1/step.log`（step 20 各 phase 行）；晨报 §二 与之矛盾 | 用户看到的「全流程通过」含一个未执行的成功步；晨报结论错误 |
| **P0-2** | **无表单（vue_custom）「统一」名不副实，且从未真实执行**：`executor.go:651-658` 仍按 `runCtx.RenderType == FormRenderTypeVueCustom` 对 submit/save_draft/resubmit 三个动作走 `target.NoFormFlowBlockReason` 分流；`native-dispatch.ts` 无表单暂存走 `temporarySaveBizData` 单独分支（有表单走 `temporaryStorage`）。这三条分支从未有真实运行证据。 | `executor.go:651`；`native-dispatch.ts` temporarySaveBizData 分支；`docs/auto/evidence-round2/` 无 noForm 类证据 | G3「无特殊处理」与 D5 均未闭环；晨报「非工具代码缺陷」的归因不成立（见 §一.3） |
| **P0-3** | **任务查询丢失「视角用户」**：引擎内建事实发现路径不复用 `ListTaskSnapshotsForUser`，submit 阶段 pending 查询一律 `query_view_user=-`（即执行者自身视角）；日志实证：step 5 gate 用视角 `08e604bf…` 查到 1 行待办，submit 阶段同实例同节点空视角查询恰好也 1 行（处理人＝执行者）；但 step 8 submit 空视角 1 行、verify 空视角 **0 行**，而 step 8 gate 用的是 `39323f15…` 视角。 | run 2 `execution.log`：`phase=submit` 段所有 `query_task_status=pending` 行的 `query_view_user=-`（11 处）；`phase=gate` 段每步都带不同视角用户 ID | 一旦任务不在执行者名下就必然查空。**这正是历史「目标节点未生成处理人」误判的直接成因**，当前运行恰好没触发 |
| **P0-4** | **「当前处理人已办=true/false」不是分型事实**：`Executor.approveAsSelfAssignee`（提交 616dbf2）把处理人视作自己、不切换会话，却仍把「本人在本节点的 done 任务」当成核验依据；该布尔在 run 2 的每一步都是 `true/false`（第一个是真事实，第二个是占位）。 | run 2 `step.log` 全部 14 行 `核验事实` | 核验依据语义未定义即落进可读事实；会签/委托/代批场景会误判成功 |
| **P0-5** | **门禁可被伪造**：G1 要求的 `TEST_AUTO_PRO_F036_REQUIRE_REAL=1` 只在 `real_load_loop_test.go:121` 读取，`test/run-executor-service.sh` 未设置；L2 动作台由 `TEST_AUTO_PRO_F036_ACTION` 空值静默跳过（`real_action_loop_test.go:93`）。7 个动作的真实写在一次「全绿」里被执行了 0 次。 | `test/run-executor-service.sh:53-55`；`real_load_loop_test.go:121`；`real_action_loop_test.go:93` | P1-D 未闭环；「全绿」不能证明任何真实写 |
| **P0-6** | **逐请求轨迹的 trace 证据与实际发出的请求不等价**：`request-trace.ts` 的 `TraceEntry.body/headers` 在 `begin()` 时写入，而 `target-route.ts` 的工具流程名保护与后续改写发生在 `begin()` **之后**，轨迹里记的是改写前 URL 查表得到的旧 body。 | `target-route.ts`（toolFlowName 改写段）与 `request-trace.ts:36`；对比同一 run 的 `curl.log` 与运行详情请求轨迹 | 运行详情与证据里的请求体不是真正发出的那一份；审计不可信 |
| **P1-1** | **G3 无真实断言**：`run-executor-service.sh:27-35` 只做 `rg` 字面量检查（拒绝 `FormRenderTypeFormMaking`、`businessDone`、`countRelevantPageErrors`）。P0-2 的分流用 `FormRenderTypeVueCustom` 就绕过了；把 `businessDone` 改名同样绕过。本轮施工令 §六明确要求「无表单与有表单同一套代码路径」，未落实。 | `test/run-executor-service.sh:27-35` | 门禁与要证明的事实不匹配 |
| **P1-2** | **G5 未包含「在途请求零本地 abort」**：门禁只有执行服务控制台日志的人工检索，仓库内无断言。 | 晨报 §四 G5 行；`test/run-executor-service.sh` 无对应检查 | 通道问题回归无人拦 |
| **P1-3** | **G7 写后证据来源可疑**：`executor-service/src/server.ts:1079` 把**页面内**快照（`readNativeFormSnapshot`）作为 `formAfter` 与 `fieldCountAfter` 写进 businessSummary，L2 证据的 `fieldCountAfter` 也来自它；而 Go 侧 `protectRuntimeFormBody`（`executor.go:363-400`）用目标重读覆盖 `formAfter`。两套来源同名落盘，字段数用页面值、正文用目标值，证据自相矛盾。 | `server.ts:1079-1089`；`executor.go:389-390`；`evidence-round2/20260918-043248-L2-approve.json` 的 `formAfter` 是页面形状的扁平 map | 表单正文保护的实际防线与文档声明不一致 |
| **P1-4** | **P1-D 的另一半未处理**：`L2-approve-target_rejected.json` 是**设计上就是失败**的一次用例（用于制造选择闭环前置），却直接落在正式证据目录 `evidence-round2/`，没有任何状态标记。施工令只要求排除旧归档证据，未要求区分同轮预期失败。 | `docs/auto/evidence-round2/20260918-043034-L2-approve-target_rejected.json` | 证据目录里正例与预期失败例混放，人工核对会误判 |
| **P2-1** | 两处工具 UI 缺陷仍未修（晨报已登记）：配置页「选择基础表单数据」弹窗离开面板后残留不可见遮罩拦截点击；表单数据面板数字输入框键入值不进模型（真实用户手工填数必踩）。 | 晨报 §六；`PlanPathConfigurationView` 弹窗未随路由卸载 | 页面手工配置体验缺陷，本轮用户裁决纳入 |
| **P2-2** | 文档口径未对齐实现：`docs/ARCHITECTURE.md:5` 仍写「状态 `ready_for_manual`」且第 16 行仍写「完成判定仍要求业务完成信号、请求收敛、无页面异常」；`docs/PRODUCT.md:103` 同样保留页面信号口径；`executor.go:74-76` 在注释里把 `shouldRunViaRuntime` 描述成「runner 为 nil 时全部动作保持既有 Go 写路径」。 | `docs/ARCHITECTURE.md:5,16`；`docs/PRODUCT.md:103`；`executor.go:74-76` | 架构/产品文档与 D2b 决策相反，会误导下一次评审 |

### 1.3 对晨报 §五「无表单遗留」的复核（用户已裁决：仍要做，根因在工具侧）

晨报把两个阻塞都归为「目标页面自身行为，非工具代码缺陷」，复核后**不成立**：

1. **「必填附件，自动化浏览器无法上传文件」不成立**：装载后页面本身是 Chromium 里的真实页面，`flow-source/src/api/index.js` 有文件上传接口 `fileupload`；附件字段（`fileupload_common_file`）是目标表单模型的普通字段，可由页面等价接口写入。正确做法不是模拟文件选择框，而是查清原生附件组件把已上传文件写进表单模型的确切形状后经现有数据通道写入。
2. **「导入最近一次数据空指针」不成立为「无解」**：`Cannot read properties of null (reading 'kpiList')` 是页面在某账号无历史数据时的自身缺陷，但**工具侧可以通过先提供可用数据/选择有历史的账号**让该入口不再触发。当前是「试了一次就登记」，不是「无解」。
3. **更重要的是**：无表单的三个动作（暂存/提交/重提）分发本就走与有表单不同的分支（§一.2 P0-2），「无表单与有表单走同一套装载/分发/判据代码路径」这句话在分发这一层就是错的。G3 通过静态 `rg` 不能证明这一点。

---

## 二、本轮用户决策（照此执行，不得自行改口径）

| 编号 | 决策 | 落地含义 |
| --- | --- | --- |
| R1 | **无表单仍然做，并认定根因在工具侧** | 无表单真实写运行列为本轮必做项。T5 的「统一化」判据改为「按真实执行证明」：同一动作键在两类表单上走同一分发函数、同一完成判据、同一写后核验，且各有真实证据。不再以「接受边界」收口。 |
| R2 | **全部收口** | 本轮覆盖 §一.2 的全部 P0/P1 项、两处工具 UI 缺陷（P2-1）与文档订正（P2-2），不留「已登记待裁决」。 |
| R3 | **工具 UI 缺陷纳入本轮** | 作为独立原子提交项（T8），与执行链路修复分开提交。 |
| R4 | **判定口径以本文件 §五 为准** | 上一轮的 G1–G8 表作废为「自评」；本轮门禁全部改为可执行断言或带明确「未覆盖」声明的报告项，不允许用文字「✅」代替证据。 |

---

## 三、事实底稿（改动前逐条核对）

1. 引擎执行链：`internal/engine/step/executor.go`
   - `shouldRunViaRuntime`（:84）只按动作键判定；`runViaRuntime`（:100）→ `preview` 写事实。
   - `protectRuntimeFormBody`（:363）用 Go 重读覆盖 `formAfter`/`fieldCountAfter`。
   - 任务快照：`reloadFacts`（verify.go:25 起）→ `FindDueFlow`（无视角）→ `CurrentHandlers = snapshot.Handlers` → `HandlerMissing`（verify.go:76）→ `readActionTaskFacts`（verify.go:200 起）→ `refreshTaskSnapshot`。
   - 跳过判定：`targetSkippedStepReason`（executor.go:665）在 task 作用域、门禁之前执行。
2. runtime 执行链：`internal/runtimeexec/runner.go`（`RunAction`）→ `client.go`（`CreateSession`/`Prewarm`/`Load`/`Execute`）→ `executor-service/src/server.ts`（装载 + execute 处理）→ `native-dispatch.ts`（动作分发）→ `completion.ts`（终局判定）。
3. 目标真名（不得改动）：发起 `FlowDialog.sumbitFlow('draft'|'submit')`；审批 `ExamineOpinion.submitCheckBefore('pass'|'no_pass')`、`temporaryStorage()`、`saveSubmit('save'|'submit')`；无表单 `examineOpinion.vue` 的 `temporarySaveBizData()`；转发 `flowTypeMixin.transpond/confirmTranspond`（只在列表页 `Backlog/Finished/Submitted`）；选择继续 `handleSaveBranchChooseNode`/`handleSaveParallelChooseNode`/`getSelectPerson`。
4. 写端点：submit=`/web/flowInstanceApi/submit`、audit=`/flowInstanceApi/audit`、storageFormData=`/web/flowInstanceApi/storageFormData`、reSubmit=`/web/flowInstanceApi/reSubmit`、transpond=`/web/flowInstanceApi/transpond`（+ `/web/flowInstancePostscript/save`）。
5. 真实运行日志（只读，作为本轮的对照基线，不得改动、不得用作证据）：
   `logs/runs/运行_2__T-AUTO-F036R2-请假单-单路径__run-2/paths/路径 1__path-run-2__path-1/{step.log,execution.log,curl.log,network.log}`。

---

## 四、任务清单（T1–T9，按序施工；每个原子成果一次中文提交）

### T1：运行编排区分「已执行」与「已跳过」，并禁止把跳过计入成功步数

- 位置：`internal/engine/step/executor.go`（跳过分支 :668-680）、`summary.go`、`steplog.go`、运行详情投影。
- 要求：
  1. `TargetSkipped` 的步骤在聚合与界面文案里必须与「真实执行成功」分开计数（例如「成功 N 步（其中 M 步为目标自动跳过）」），不得合并成「N/N 步成功」。
  2. 新增断言用例：一个含跳过步的运行，其聚合结果必须显式暴露跳过步编号。
- 完成标准：run 2 那种形态（step 20 无 submit 行）在新代码下不可能被表述为「20/20 步全部成功」。

### T2：任务查询统一带「视角用户」，消灭引擎侧无视角 pending 查询

- 位置：`internal/engine/step/executor.go`（`refreshTaskSnapshot` 及其调用点）、`internal/adapter/target/task_query.go` / `client`（`ListTaskSnapshotsForUser`）。
- 要求：
  1. 引擎在执行一个任务级动作时，凡已解析出**目标处理人用户 ID**（`CurrentTaskAssigneeID` / `snapshot.PendingUserID`），提交阶段的 pending 快照查询必须用该用户视角，不得退回执行者自身视角。
  2. `FindTaskSnapshot` 的调用点必须显式传视角；新增或改造的函数名与注释说清「视角」语义。
  3. 回归用例（真 MySQL 或可控假件，二者选一但必须真实覆盖）：构造「待办在处理人 B 名下、执行者 A 名下无待办」的实例，断言提交阶段仍能拿到任务号；并断言**不会**因此把 `HandlerMissing` 判成“未生成处理人”。
- 完成标准：`execution.log` 里任何 `phase=submit` 的 pending 查询都带非空 `query_view_user`，或带可解释的「执行者即处理人」原因。

### T3：核验分型事实取代 `true/false`

- 位置：`internal/engine/step/verify.go`、`types.go`、运行详情展示层。
- 要求：
  1. `Executor.approveAsSelfAssignee` 场景（处理人＝执行者）必须有明确的分型事实名，例如「本人待办已办（执行者=目标处理人）」，而不是把「处理人是自己」与「本节点当前账号有已完成任务」写成同一个布尔。
  2. run 2 里出现的 `true/false` 形态必须在新版日志里消失；`step.log` 的「核验事实」行改为逐项具名。
  3. 用例覆盖：会签（部分人已完成）、委托/代批（执行者≠处理人）、本人审批三种情形各自的核验依据。
- 完成标准：单步日志里不再存在未命名的布尔位；三种情形各有至少一个用例。

### T4：门禁真实化（G1 接线 + 门禁可执行化）

- 位置：`test/run-executor-service.sh`、`test/integration/runtimeexec/*`。
- 要求：
  1. 聚合脚本必须设置 `TEST_AUTO_PRO_F036_REQUIRE_REAL=1`，并在脚本内新增一层「真实动作台」：由 `TEST_AUTO_PRO_F036_ACTIONS`（逗号分隔的七个动作键）驱动，逐个真实执行；该变量未配置时**打印明确的「未覆盖：本轮未验证任何真实写」并计入失败**，而不是静默跳过。
  2. 删除或改造 `real_action_loop_test.go` 里「未配置即跳过」的宽松路径：普通本地跑允许跳过，但必须返回可机读的「已跳过/已覆盖」清单，并由脚本核对「清单里的动作数 ≥ 要求的动作数」。
  3. 新增 G3 的真实断言（替代 `rg` 字面量）：用一个跨两类表单的动作键驱动同一条分发路径，断言分发事实（component/method）与完成判据（期望写端点）完全一致；不允许出现按渲染类型分支的**动作分发**差异。
  4. 新增 G5 断言：在 executor-service 侧把「本地 abort 在途转发」计数暴露为可读事实（事件或 `/health` 扩展），测试断言运行期间该计数为 0。
- 完成标准：故意把一个真实动作台变量置空时，聚合脚本必须失败并打印未覆盖清单。

### T5：无表单与有表单的真实统一（含真实写闭环）

- 位置：`internal/engine/step/executor.go:651-658`、`executor-service/src/native-dispatch.ts`、`server.ts` 装载分支。
- 要求：
  1. 删除按 `RenderType` 对 submit/save_draft/resubmit 的 `NoFormFlowBlockReason` 分流；改为**与有表单完全相同的可执行能力判据**（同一函数、同一代码路径）。若某无表单业务确实缺入口，判据必须来自页面实际探测结果，不得来自渲染类型。
  2. `temporarySaveBizData` 与 `temporaryStorage` 的选择改为**组件能力探测**（组件上存在哪个方法就用哪个，两个都存在时按目标真实行为），不得按渲染类型硬编码分支。
  3. 完成**无表单真实写闭环**：至少覆盖 `save_draft`（或 `submit`）、`approve`、`storage_form_data`（或 `reject`）三个动作，各产出「状态 + 写端点 + 目标响应原文 + 写后回读」四件套证据到 `evidence-round3/`。
  4. 处置两个已知阻塞，且**必须留下可判定的结论**（成功运行 / 或「已排除的处置路径 + 剩余阻塞点的原始报错」）：
     - 附件：用页面 `fileupload` 接口或原生附件组件的数据写入路径，查清 `fileupload_common_file` 的确切形状并写入；
     - `kpiList` 空指针：先提供可用历史数据（可选用有历史的账号/先造一份历史实例），使「导入最近一次数据」不触发；若仍触发，必须给出触发点与工具侧可绕开的具体做法，不得只写「目标页面缺陷」。
  5. 无表单运行若确实无法闭环，**不得**以「接受边界」收口；应写明：已排除的路径、原始报错、下一步需要用户提供什么（例如某个账号或某个模板配置），并停下等用户裁决。
- 完成标准：G3 有真实断言；无表单至少三个动作有真实证据；两个阻塞点各有可判定的结论。

### T6：判据与证据格式统一（P0-6、P1-3、P1-4）

- 位置：`executor-service/src/request-trace.ts`、`target-route.ts`、`server.ts`、`internal/engine/step/executor.go`。
- 要求：
  1. 轨迹里落盘的 `body/headers` 必须是**真正发出**的那一份：把改写（工具流程名保护、SID 信封等）移到 `begin()` 之前，或在发出前更新条目并留下「改写前/改写后」两个字段。补用例：改写发生时，轨迹 body == 实际发出 body。
  2. 表单写后事实**只有一个来源**：`formAfter`/`fieldCountAfter` 一律以 Go 目标重读为准；页面快照改名（如 `pageFormAfter`/`pageFieldCountAfter`）并明确标注为「页面自述，仅作诊断」。
  3. L2 证据目录只放正向证据；预期失败用例的落盘名必须带 `expected-failure` 且在报告里单独成节。
  4. 补断言：写后回读失败时证据里必须只有目标值（无页面值冒充）。
- 完成标准：任取一份 `evidence-round3/` 的审批族证据，`formAfter` 与目标只读 API 重读结果逐字段一致。

### T7：子流程节点的真实执行（P0-1 根因）

- 位置：`internal/engine/step/executor.go`（`targetSkippedStepReason` 与门禁顺序）、运行编排的节点/实例映射。
- 要求：
  1. 查清 run 2 步 20（结束节点）为何没有 submit 行：是「结束节点无动作」还是「跳过判定放行」；结论必须用日志 + 目标实例事实说清。
  2. 若属「跳过判定放行」，修正为：**路径上配置了动作的节点不允许被跳过判定吞掉**；跳过只允许用于目标确实自动越过的节点，且必须与「已执行」分开计数（配合 T1）。
  3. 用例覆盖：路径最后一个节点已配置动作、目标实例已到 end 的两种组合。
- 完成标准：重跑单路径计划，报告中每一步都能对应到 `phase=submit` 或明确的「目标自动跳过（含目标事实）」。

### T8：两处工具 UI 缺陷（独立提交）

- 位置：配置页（`PlanPathConfigurationView` 等）。
- 要求：
  1. 基础数据弹窗必须随路由/面板卸载而销毁，离开面板后不再存在不可见遮罩；补前端用例或明确的复现步骤 + 修复前后对照。
  2. 表单数据面板数字输入框键入值必须进入表单模型；用页面实际手工输入验证（不启动浏览器自动化）。
- 完成标准：两项各有复现步骤、修复说明、人工验证步骤；提交与执行链路修复分开。

### T9：文档订正与证据归档

- 要求：
  1. `docs/ARCHITECTURE.md:5` 状态改为当前真实状态；第 16 行「完成判定仍要求业务完成信号、请求收敛、无页面异常」按 D2b 改为接口判据口径；`docs/PRODUCT.md:103` 同步。
  2. `executor.go:74-76` 的 `shouldRunViaRuntime` 注释与实际行为对齐（删除已失效的「runner 为 nil 时全部动作保持既有 Go 写路径」表述，或在代码中恢复该语义——二选一，必须一致）。
  3. `evidence-round2/` 冻结：在 `evidence-round2/README.md` 写明该目录是第二轮归档、已被第三轮复核部分证伪、引用时以 `evidence-round3/` 与功能文档为准；把 `saveF036Evidence` 的输出目录切到 `evidence-round3/`。
  4. 功能文档 `docs/features/F-036-independent-executor-native-page.md` 的「第二轮完成点」段落逐条订正（特别是 20/20 步、无表单归因、G3/G5 结论）；`docs/PROGRESS.md`、`docs/ROADMAP.md` 同步。
  5. 更新 `docs/s2/02-执行任务清单.md` 与 `docs/s2/01-执行服务设计.md`：补 T015–T0xx 的第三轮任务与判据口径。
- 完成标准：文档中没有与代码相反的结论性表述；每个「已完成」都能指到 `evidence-round3/` 的具体文件。

---

## 五、本轮门禁（替代上一轮 G1–G8；每项必须是可执行断言或明确「未覆盖」）

| 门禁 | 判据 | 未覆盖时 |
| --- | --- | --- |
| G1 真实写齐备 | 聚合脚本设置 `REQUIRE_REAL=1`；七个动作各自真实跑过一次并落 `evidence-round3/` 四件套 | 脚本失败，打印未覆盖动作清单 |
| G2 无表单统一 | 同一动作键在两类表单上分发事实（component/method）与期望写端点一致；无表单真实写 ≥3 个动作有证据 | 失败 |
| G3 无渲染类型分流 | 静态扫描 + **行为断言**：不存在按渲染类型决定动作分发或放行的代码路径 | 失败 |
| G4 判据契约 | `completion.test.ts` 七值与选择闭环用例全绿；请求轨迹 body == 实际发出 body | 失败 |
| G5 通道复用 | 运行期间「通道被替换」计数 = 0、「在途转发本地 abort」计数 = 0（可机读事实） | 失败 |
| G6 多次草稿 | 同实例两次保存草稿，实例 ID 不变；证据含两次写端点 | 失败 |
| G7 表单正文完整 | 审批族写后字段数不减少；`formAfter` 唯一来源为目标重读，且与只读 API 逐字段一致 | 失败 |
| G8 跳过与执行分离 | 报告中「已执行成功」与「目标自动跳过」分别计数，跳过步给出目标事实 | 失败 |
| G9 文档一致 | ARCHITECTURE/PRODUCT/features/PROGRESS 无与代码相反的结论 | 失败 |

---

## 六、明确禁止

1. 不得把「已登记」「已裁决」「接受边界」当作缺陷闭环。
2. 不得用文字「✅」代替可执行证据；不得在报告里复用上一轮的结论段落。
3. 不得改动 `logs/runs/运行_2__…` 的历史日志作为证据（只读对照）。
4. 不得为通过门禁而放宽判据、删用例或把断言改成人工核对。
5. 不得在未修完 §一.2 的 P0 项之前进入 `ready_for_manual`。
6. 不得改动目标平台源码、权限或业务规则；不得新增按渲染类型的兼容层、别名或回退实现。
7. 不得把 `evidence-round2/` 里的旧证据当作本轮完成依据。

---

## 七、完成后

1. 跑 `bash test/run-executor-service.sh`（含真实动作台），把完整输出留档。
2. 写 `docs/auto/2026-09-18-f036-round3-run-report.md`：逐条对应 §一.2 的问题表，给出「已修 / 未修 + 原因」，附证据文件路径；无表单部分如实写结论与剩余阻塞。
3. 停在 `ready_for_manual` 等用户人工验收，**不自动开始下一功能，不代替用户验收**。
