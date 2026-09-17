# F-036 第二轮修复任务：统一原生执行（用户决策版）

> 读者＝执行 agent。上一轮（2026-09-17 夜）已把八个 P1 中的四项做实，但仍留下三处必须解决的问题。
> 本轮由用户在 2026-09-17 明确拍板了七项决策（见 §一），目标是：**把执行器修到"有表单/无表单一致、
> 七动作齐备、判据只看接口"的可用状态，并从清空后的新数据状态重新验证**。
> 本文件是施工令，不是报告；结论以你自己跑出的证据为准，不得采信旧文档的结论性表述。

- 基线：`27fec27`（工作树干净）
- 上一轮晨报：`docs/auto/2026-09-17-f036-autonomous-run-report.md`
- 上一轮施工令：`docs/auto/2026-09-16-f036-second-review-repair-task.md`
- 本轮证据目录：`docs/auto/evidence-round2/`
- 本轮晨报：`docs/auto/2026-09-18-f036-round2-run-report.md`

---

## 一、本轮用户决策（照此执行，不得自行改口径）

| 编号 | 决策 | 落地含义 |
| --- | --- | --- |
| D1 | **保存草稿支持多次执行**：第一次走发起保存草稿，后续"在待发里点开数据再次保存" | `save_draft` 第二次起改为装载 `mode=edit`（`isReInitiate`）并调原生 `ExamineOpinion.saveSubmit('save')`；写后必须断言实例 ID 不变（不得新建实例） |
| D2a | **通道全程复用一条** | 消除"Go 通道被新连接替换/已断开"造成的中止；预热连接与执行命令共用同一连接 |
| D2b | **完成判定直接以接口判定** | 终局由该动作期望写端点的响应（HTTP + `isSuccess` + 错误类型）推导；页面异常与业务信号只作证据，不再参与判定 |
| D3 | 测试门禁由施工方（你）把控 | 按 §六 的 G1–G8 执行，不得放宽 |
| D4 | **转发直接修复完** | 按原生列表页的转发链路实现（`flowTypeMixin.transpond` → `confirmTranspond`），含两笔写顺序 |
| D5 | **无表单与有表单完全一致，去除特殊处理** | 删除按渲染类型收窄/回退；装载、分发、判据对两类一致；无表单的暂存入口是 `temporarySaveBizData` |
| D6 | **清空所有计划与运行记录**，从空数据继续 | §三 第 0 步；保留 `schema_migrations`，其余数据表清空 |
| D7 | 无用的直接删除，避免误导 | §四 T7 与 §七：旧结论段落、死代码、过期证据一并处理 |

---

## 二、事实底稿（已核实，直接用）

### 2.1 上一轮做成与没做成的（据实核对）

**做成的**：装载捕获改 `window.$vue` 原型包装；模板选择以 trace 里 `/web/flowTemplateApi/list` 真实响应为准；
动作分发落到真实组件（递归唯一命中）；完成判定端点表改为五个真实端点；选择闭环 `choice_required → 继续一次`；
保存草稿/提交/暂存/同意/不同意/重提六个动作有真实写证据。

**没做成的（本轮要修）**：

| 编号 | 问题 | 证据 |
| --- | --- | --- |
| P0-A | **审批族写入携带空表单正文，把实例表单体清空**：所有 approve/reject/choice 证据的请求体都是 `"formDataMongoVo":{"data":{}}`（原生应发 `Object.assign(editData,value)`，`EnterpriseExamineOpinion.vue:986`）。实测：`bc847fa1…`、`cd9c8b01…`、`d71ee968…` 当前表单字段数 = **0**；对照实例 `7bcf0c29…`=84、`bbfbbac5…`=28、`c3aab923…`=2（读取口径正常）。根因：`waitForDetailReady` 只等 `examineDialogVisible`，不等实例表单数据加载完成 |
| P0-B | **run 129/130 停在"结果待确认"，且第 2 步真的写出了重复实例**：`logs/runs/运行_2__oyg测试1003__run-130/.../curl.log:192` 的 submit 体带 `"status":"draft"` 却**没有 `data.id`**，响应 `:194` 返回新实例 `d71ee968…`；引擎跟踪的是 `bc847fa1…`。上一轮晨报把它写成"第 2 步被正确拦截停止（属计划配置问题，非引擎缺陷）"——**该结论错误，本轮不采用** |
| P1-C | **通道替换中止在途请求**：`forward-channel.ts` 的 `attach()` 先 `detachAll('Go 通道被新连接替换')`；上一轮执行服务日志出现 6 次"被新连接替换" + 6 次"已断开"，页面因此报 `Network Error`，是 P0-A 的诱因之一 |
| P1-D | **门禁过软**：真实闭环用例在服务未启动时一律 SKIP，`ready_for_manual` 可在没有任何真实执行的情况下达成 |
| P1-E | **判据被页面信号污染**：`countRelevantPageErrors` 过滤掉所有带 URL 的错误；`businessDone` 用"页面无异常"参与终局，导致坏写看起来像成功（D2b 直接改判据） |
| P1-F | **转发未实现**：`nativeDispatch` 的 switch 没有 `forward` 分支，直接落到 `default: throw`；且 `confirmTranspond` 根本不在 `ExamineOpinion` 上（见 2.3） |

### 2.2 目标平台真名（改动前逐条对照）

- 全局：`window.$vue = app`；`Vue.prototype.$initFlow/$flowDetail`。
- 发起链：`InitFlowDialog` → 子 `FlowDialog`（`sumbitFlow('draft'|'submit')`；表单 `FlowDialog.$refs.OtherSteps2.$refs.generateForm`）。
- 详情链（有表单）：`flowDetail`(`name:''`) → `EnterpriseExamineDialog`(`ref="generateForm"`、`editData`) → `ExamineOpinion`（`name:'ExamineOpinion'`）。
  方法：同意/不同意 `submitCheckBefore('pass'|'no_pass')`；暂存 `temporaryStorage()`；重提/待发 `saveSubmit('submit')`/`saveSubmit('save')`；选择 `handleSaveBranchChooseNode`/`handleSaveParallelChooseNode`/`getSelectPerson`。
- 详情链（无表单）：`flowDetail` → `ExamineDialog`（`v-if=ExpensesClaimFormVisible`，内部按 `searchFlowType` 映射 `ExpensesClaimForm/NewBudget/detailBudget/workTarget/MonthlyPerformence/NewEvent` 等）→ `examineOpinion.vue`（组件名同样是 `ExamineOpinion`）。
  方法：同意/不同意 `submitCheckBefore('pass'|'no_pass')`；**暂存 `temporarySaveBizData()`**；重提/待发 `saveSubmit('submit')`/`saveSubmit('save')`；选择三方法同名。
- **转发不在详情弹窗里**：`transpond()` / `confirmTranspond()` 来自 `flowTypeMixin.js`，只混入列表页
  `Backlog/index.vue`（待办）、`Finished/index.vue`（已办）、`Submitted/index.vue`（待发）。
  列表页路由：`/flow/approve/:tab`（`backlog|finished|dueout|submitted|timedout`），`GroupApproveManage/index.vue` 按 `activeName` 渲染子组件（`ref='backlog'` 等）。
  列表模板：`<dy-table ... ref="dytable" showCheckBox :list='tableData'>`；转发前置：`this.$refs.dytable.selectDatas`（`Backlog/index.vue:579`），
  且 `transpond(type)` 内部会拉表单数据、重置 `approveForm`、置 `transpondVisible=true`；`confirmTranspond()` 发送
  `{data:{name:finalFlowName}, receiverId: approveForm.personId, formDataMongoVo:{data:Object.assign(currentFormData,paramData)}}` + `flowInstanceBizRelevanceList`（`transpond_flowInstanceId`/`<auditWay>_transpondFlow`/`transpond_formExist`/`transpond_auditWay`/`transpond_originalName`/`company`）。
- 端点真名：提交/草稿/权限校验 `/web/flowInstanceApi/submit`；审批 `/flowInstanceApi/audit`；暂存 `/web/flowInstanceApi/storageFormData`；
  重提 `/web/flowInstanceApi/reSubmit`；转发 `/web/flowInstanceApi/transpond`；附言 `/web/flowInstancePostscript/save`；模板 `/web/flowTemplateApi/list`、`/web/flowTemplateApi/findById`。
- 详情 query 契约：`mode`（`audit|edit|view`）+ `taskContext`（JSON；字段至少含 id/flowInstanceId、auditWay、jobTaskId、formProxyId、flowProxyId、flowNodeProxyId、nextNodeProxyId、nextNodeName、nextNodeType、initiatorId、formExist、flowInstanceName、flowInstanceBizRelevanceList、auditPassLogicFlag、branchExecuteType、lastCountersignFlag、tracking）。

### 2.3 必须删除的"特殊处理"（D5）

1. `internal/engine/step/executor.go` 的 `shouldRunViaRuntime` 里 `if runCtx.RenderType != string(target.FormRenderTypeFormMaking) { return false }` —— 删除。
2. 同文件 `runViaRuntime` 里的 `if strings.TrimSpace(runCtx.FormProxyID) == "" { ...阻塞... }` —— 删除，改为按装载模式的必需事实校验：
   initiate 需 `flowTemplateId` + `auditWay`；实例族（approve/edit/view/forward）需 `instanceId`；审批族（approve/reject/storage）需 `jobTaskId`；forward 另需 `receiverId`。
3. `executor-service` 中任何以渲染类型分流表单接口的代码 —— 改为"宿主是否提供数据接口"，与渲染类型无关。
4. 文档口径：`docs/features/F-036…` 与 `docs/s2/01-执行服务设计.md` 中"runtime 只接 FormMaking / 无表单回退 Go 直连"的表述 —— 删除，恢复为"原生页面支持的渲染类型全部走执行服务"。

---

## 三、第 0 步：清空数据，从空状态开始（D6）

顺序不可颠倒（先停写方，再清数据，再重启）：

```bash
# 0.1 停后端（避免 Worker 回写）。air 托管时停 air 进程，确认 19080 端口释放。
# 0.2 复核表清单（禁止凭记忆清表）
set -a && . ./.env.local && set +a
mysql -h"$PLAN_DB_HOST" -P"$PLAN_DB_PORT" -u"$PLAN_DB_USER" -p"$PLAN_DB_PASSWORD" "$PLAN_DB_NAME" -N -e "SHOW TABLES"
```

```sql
-- 0.3 清空（保留 schema_migrations；清单以 SHOW TABLES 实测为准）
SET FOREIGN_KEY_CHECKS=0;
TRUNCATE TABLE path_runs;
TRUNCATE TABLE plan_run_queue;
TRUNCATE TABLE run_controls;
TRUNCATE TABLE run_events;
TRUNCATE TABLE run_manual_conclusions;
TRUNCATE TABLE run_step_attempts;
TRUNCATE TABLE run_steps;
TRUNCATE TABLE runs;
TRUNCATE TABLE test_execution_path_batch_items;
TRUNCATE TABLE test_execution_path_batches;
TRUNCATE TABLE test_execution_path_choices;
TRUNCATE TABLE test_execution_path_configs;
TRUNCATE TABLE test_execution_paths;
TRUNCATE TABLE test_form_runtime_sync_jobs;
TRUNCATE TABLE test_history_data_snapshots;
TRUNCATE TABLE test_history_replay_items;
TRUNCATE TABLE test_history_replay_jobs;
TRUNCATE TABLE test_plan_history_data_defaults;
TRUNCATE TABLE test_plans;
SET FOREIGN_KEY_CHECKS=1;
```

```bash
# 0.4 验证：除 schema_migrations 外表行数全为 0
# 0.5 清本机日志：logs/runs/*、logs/plans/*（保留目录本身）
# 0.6 旧证据归档（D7）：mv docs/auto/evidence docs/auto/archive/2026-09-17-f036-round1-evidence
# 0.7 重启后端；curl /api/health 通过，curl /api/plans 返回空列表
```

目标平台侧：当夜产生的测试实例由用户自行清理，工具侧**不得**直接写目标库删数据。需要写进晨报的清单：
`c3aab9235d6c4b75b54a55c5c1bfe6e1`（草稿）、`06c4a0d8…`（run）、`bbfbbac5d74b4a9ab0acf123b16b98b1`（run）、
`bc847fa12c5f4307b9d33e00d45361b0`、`cd9c8b01fdc3474485de18da37bc68fd`、`d71ee9684cd446df8f5e9452d250a9b9`（表单体已为空）。

---

## 四、代码任务（一项一提交；每项都要有对应测试）

### T1 通道全程复用（D2a）

**改哪里**：`executor-service/src/forward-channel.ts`、`executor-service/src/server.ts`（upgrade/attach）、`internal/runtimeexec/client.go`（Prewarm/Execute）。

**怎么改**：
1. `ForwardChannel.attach()` 不得在接管新连接时中止在途请求；同一会话只保留一条活动连接——若已有连接且仍有在途请求，
   拒绝新连接（记日志、关掉新 socket），或在旧连接自然空闲后再替换。禁止 `detachAll('被新连接替换')`。
2. Go 侧改为**单连接持有**：`Prewarm` 建立的连接写入 `Client` 的会话表（`map[sessionID]*sessionConn`），
   `Execute` 复用同一条连接（复用其 requestId 多路复用与结果等待），不再新拨一条；`Destroy` 时统一关闭。
3. 连接意外断开时，按既有纪律（已有写出→`uncertain`；无写出→`failed`），并允许下一次动作重建连接。

**验收**：同一会话连续执行 ≥10 次动作，执行服务日志中不得出现 `Go 通道被新连接替换` 与 `Go 通道已断开`；
页面错误里不得再出现由本地 abort 引起的 `Network Error`（见 G5）。

### T2 完成判定改为接口判据（D2b）

**改哪里**：`executor-service/src/completion.ts`（主判据）、`executor-service/src/server.ts`（装配）、Go 侧结果映射（`runViaRuntime`）、
`docs/s2/01-执行服务设计.md` §7 + F-036 完成标准。

**怎么改**（判据表，按顺序命中）：

| 情形 | 判定 |
| --- | --- |
| 该动作**期望写端点**出现且 `state=fulfilled`、HTTP 200、响应体 `isSuccess !== false` | `success` |
| 响应 HTTP 401 / `code=RESP401`/`SID已失效` | `target_rejected`（SID 失效） |
| 响应 `isSuccess=false` 且 `data.errorType ∈ {custom_choose, parallel_choose, run_node_choose}` | 触发选择闭环；继续后重试成功→`success`，未闭环→`target_rejected` |
| 响应 `isSuccess=false` 其他 | `target_rejected`（带目标 message） |
| **没有期望写端点**但窗口内有其他写端点 | `failed`（工具侧端点不符，属缺陷，必须可读原因） |
| 期望写 `aborted`/连接断开且已写出 | `uncertain`（锁 WriteGate） |
| 硬超时 | `timeout`（锁 WriteGate） |
| 转发第一笔成功、第二笔（附言）失败/未完成 | `business_failed` |

- 页面异常（`__executorPageErrors`）与业务完成信号：**只写入 `businessSummary` 作为证据**，不得参与终局判定；
  删除 `businessDone` 参与 `finalize` 的分支（保留 `__executorPageErrors` 采集）。
- 期望写端点表（与动作一一对应）：save_draft/submit→`/web/flowInstanceApi/submit`（排除权限校验型）；
  storage_form_data→`/web/flowInstanceApi/storageFormData`；approve/reject→`/flowInstanceApi/audit`；
  resubmit→`/web/flowInstanceApi/reSubmit`；forward→`/web/flowInstanceApi/transpond`。
- Go 侧 `preview.writeResponse` 继续由 `result.Status` 映射；**写后事实核验（Go 重读目标）保持为最终裁决**，本项只改 runtime 的终局判定来源。
- 同步改设计文档：§7 完成判断改为"以接口判据为准，页面信号仅作证据"，并写明七值语义不变。

**验收**：G4 契约测试八个判定值（success / target_rejected-SID / target_rejected-业务 / 选择闭环 / failed-无写 / failed-端点不符 / uncertain / timeout / business_failed-后置）逐项通过。

### T3 保存草稿支持多次执行（D1）

**改哪里**：`internal/engine/step/executor.go`（装载模式决策）、`internal/runtimeexec/runner.go`（`loadModeFor` 改为上下文感知）、
`executor-service/src/server.ts`（按装载模式分发）、`executor-service/src/native-dispatch.ts`（分发分支）。

**怎么改**：
1. 装载模式：`save_draft` 时若 `runCtx.PathRun.MainInstanceRef` 为空 → `LoadModeInitiate`（首次，走发起）；
   非空 → `LoadModeEdit`（后续，走待发编辑）。在 `runViaRuntime` 里直接给出 `LoadMode`（它拿得到 runCtx），
   `loadModeFor` 只作兜底。
2. 载荷：`LoadModeEdit` 必须带 `instanceId`（= `MainInstanceRef`）与 `taskContext`（含 jobTaskId 若目标有）；
   详见 §2.2 的 query 契约。
3. 分发：`save_draft` + `initiate` → `FlowDialog.sumbitFlow('draft')`；`save_draft` + `edit` → `ExamineOpinion.saveSubmit('save')`。
   Go 在 execute 载荷里显式下发 `loadMode`（服务端以它分发，不靠猜）。
4. **防重复实例**：save_draft/submit 的写响应若返回的实例 ID 与 `MainInstanceRef` 不一致（非空且不同）→ 判 `failed`（工具缺陷），
   并在日志/证据中给出两个 ID。这条断言是 P0-B 的回归防线。

**验收**：G6——同一实例连续两次保存草稿：第二次走 `mode=edit`，写端点仍为 `/web/flowInstanceApi/submit`（`status=draft`），
实例 ID 不变，目标上不产生第二个草稿；两次都有证据。

### T4 转发完整实现（D4）

**改哪里**：`executor-service/src/native-dispatch.ts`（新增转发链路）、`executor-service/src/server.ts`（列表页装载与就绪）、
`internal/engine/step/executor.go`（forward 的任务快照刷新与所需事实）。

**怎么改**：
1. 装载：forward 不再用 `/flow/detail/:id`，改为装载列表页 `#/flow/approve/<tab>`
   （待办来源→`backlog`；待发来源→`dueout`；由 Go 下发的 `flowSource/任务状态` 决定，写进 `loadMode` 或新字段）。
2. 就绪：等列表页组件（`$refs.backlog` / `$refs.submitted`）装载完成且 `tableData` 非空。
3. 选行（页面自身数据模型，不改 DOM）：在该组件 `tableData` 中按**实例 ID + jobTaskId**定位唯一行；
   写入 `$refs.dytable.selectDatas`（若 dy-table 暴露选择 API 则优先调它）；两处都不可用 → `unsupported` 阻塞。
4. 准备：调原生 `transpond(type)`（`type` 同 tab），等其内部 `getFormDataById`/`getFormData` 收敛；
   然后写 `approveForm.personId/personName`（Go 下发的接收人）、`approveForm.checkList`（`fuyan`/`yijian`，来自 `forwardCheckList`）、
   `approveForm.approveMessage`（附言文本，若有）。
5. 执行：调 `confirmTranspond()`；两笔写必须按序：`/web/flowInstanceApi/transpond`（必须成功）→ 若勾选则 `/web/flowInstancePostscript/save`。
   保留既有门禁：第一步未确认成功绝不发第二步（`transpondConfirmed`）。
6. Go 侧：`runViaRuntime` 的写前任务快照刷新分支要覆盖 `forward`（现在只覆盖 approve/reject/storage），
   确保 `jobTaskId`/`nodeProxyId` 是现场事实；`receiverId` 缺失时写前阻塞（已有）。

**验收**：真实转发一条待办实例：目标上出现转发/辅助实例，附言按序写入；两笔端点、顺序、接收人可从证据中读出；
原实例任务状态变化；无第二实例的误判（若目标本就设计为产生辅助实例，证据里要写明这是预期的辅助实例，与重复实例区分）。

### T5 有表单/无表单统一，删除特殊处理（D5）

**改哪里**：`internal/engine/step/executor.go`、`executor-service/src/native-dispatch.ts`、`executor-service/src/server.ts`、文档。

**怎么改**：
1. 删除 §2.3 列出的三处特殊处理（RenderType 收窄、FormProxyID 门禁、按渲染类型分流表单接口）。
2. 分发按**方法存在性**（同一个 `ExamineOpinion` 名称对两类都成立，靠 v-if 分支只会出现一个）：
   - 暂存：`temporaryStorage` 或 `temporarySaveBizData`（存在哪个用哪个；都不存在 → 抛错，禁止静默）
   - 同意/不同意：`submitCheckBefore('pass'|'no_pass')`
   - 重提/待发保存：`saveSubmit('submit')` / `saveSubmit('save')`
   - 发起：`sumbitFlow('draft'|'submit')`
   - 转发：T4 的列表页链路（两类同入口）
   - 选择继续：三方法同名
3. 数据补丁：不按渲染类型分支。规则统一为——**当 `changedFields` 非空时**，在已解析宿主上发现原生数据接口
   （有表单：`EnterpriseExamineDialog.$refs.generateForm` / `FlowDialog.$refs.OtherSteps2.$refs.generateForm`；
   无表单：自定义页组件自身的数据方法）→ 应用并回读；找不到接口 → `unsupported`（原因里写明页面/组件名）。
   当 `changedFields` 为空时，不做补丁，直接进入动作。
4. 文档同步：删除"仅 FormMaking"口径，写明"原生页面支持的渲染类型全部走执行服务"。

**验收**：一条 FormMaking 路径与一条无表单（vue_custom/NoFormFlow）路径分别跑：装载 + 审批族动作（暂存与同意各一次）+ 回读；
两条路径使用同一装载模式集合、同一分发实现、同一判据；G3 静态断言通过。

### T6 表单正文保护（P0-A 回归防线）

**改哪里**：`executor-service/src/server.ts`（就绪判据）、`test/integration/runtimeexec/real_action_loop_test.go`（读回）、
证据结构（`businessSummary`）。

**怎么改**：
1. 详情装载就绪增加"实例数据已加载"判据：有表单=`EnterpriseExamineDialog.editData` 非空或 `getValues()` 字段数 > 0；
   无表单=宿主表单数据对象非空。超时 → `unsupported` 阻塞（**绝不让空表单进入写动作**）。
2. 写前快照：装载完成后在页面内读一次当前表单值，存入 `businessSummary.formBefore`（字段数与关键字段摘要）。
3. 写后回读：由 Go 侧 `ReadInstanceCurrentData` 读实例，字段数与 `formBefore` 比对；**明显减少 → `business_failed`** 并写入证据
   （这就是"审批不得清空表单正文"的硬约束）。
4. L2/L3 证据模板增加 `formBefore`/`formAfter`/`fieldCountBefore`/`fieldCountAfter` 四个字段，缺一即视为证据不合格。
5. 清理：若 `countRelevantPageErrors` 不再参与判定，降级为证据函数并改名（如 `collectPageErrors`），注释写明"仅作证据"。

**验收**：G7——任一审 approval 族动作后，实例表单字段数不少于写前；三个被清空的旧实例由用户清理，不再新增同类。

### T7 删除无用与误导（D7）

1. 代码：`optionsReady` 参数若已无使用方 → 删除；`businessDone` 参与判定的分支 → 删除（T2）；
   任何"FormMaking 专属"注释/分支 → 删除；临时探针、一次性脚本、调试开关 → 删除。
2. 文档：`docs/features/F-036…` 中"八个 P1 全部 ✅"与"第 2 步被正确拦截（属计划配置问题）"两处与证据不符的表述删除；
   晨报里被本轮证伪的结论加订正说明（不删历史，但必须标注"已被 2026-09-17 复核证伪，见第二轮任务 §2.1"）。
3. 证据：旧一轮证据移入 `docs/auto/archive/2026-09-17-f036-round1-evidence/`，避免与本轮结论混用。

---

## 五、从空数据构建最小验证计划集（禁止手写 DB）

清空后，用工具自身接口/界面创建（示例，按实际可见模板调整；每个计划只配达到目的的最小节点动作）：

| 计划 | 来源 | 路径 | 目的 | 覆盖 |
| --- | --- | --- | --- | --- |
| A | 新发起（有表单） | 路径 1 = 提交 | 产生待办 | D1 首次、提交、后续审批前置 |
| B | 待办（有表单） | 路径 1 = 同意；路径 2 = 不同意；路径 3 = 审批暂存 | 审批族 | 三个审批动作各自独立实例 |
| C | 待发（有表单） | 路径 1 = 保存草稿 → 保存草稿 | 多次草稿 | D1（同一实例两次保存） |
| D | 待办（有表单） | 路径 1 = 转发（接收人=可用账号，勾选附言） | 转发 | D4、两笔写顺序 |
| E | 待发（有表单） | 路径 1 = 重提 | 重提 | 重提链路 |
| F | 无表单（vue_custom/NoFormFlow） | 路径 1 = 审批暂存 + 同意 | 无表单一致性 | D5 |
| G | 含手动分支/自选人员 | 路径 1 = 同意 | 选择闭环 | `choice_required` |

规则：不使用历史数据回放、不复制旧配置；每个计划账号用可用账号；流程名带 `T-AUTO-F036R2-<编号>` 便于识别与清理；
真实写预算 ≤12 个实例，达到上限即停并如实写明。

---

## 六、测试与门禁（由施工方把控，不得放宽）

| 编号 | 门禁 | 判定方式 |
| --- | --- | --- |
| G1 | 聚合脚本分层 | `bash test/run-executor-service.sh` 允许"未配置环境"跳过（必须打印原因）；新增 `TEST_AUTO_PRO_F036_REQUIRE_REAL=1` 时把跳过当失败 |
| G2 | 证据完整 | `docs/auto/evidence-round2/` 必须含七个动作各一份四件套（状态 + 写端点 + 目标响应原文 + 写后回读；审计族含 `formBefore/formAfter`） |
| G3 | 无特殊处理 | 静态断言：`executor-service/src`、`internal/engine/step` 中不得以 `FormRenderTypeFormMaking` 作为 runtime 路由/分支条件 |
| G4 | 判据契约 | T2 判据表八个取值各有夹具测试 |
| G5 | 通道复用 | 同一会话 ≥10 次动作，服务日志无"通道被新连接替换/已断开"；动作窗口内无本地 abort 引起的 `Network Error` |
| G6 | 多次草稿 | 同实例两次保存草稿：实例 ID 不变、无第二个草稿；两次均有证据 |
| G7 | 表单正文完整 | 审批族动作写后字段数 ≥ 写前（对照 `formBefore`） |
| G8 | 从空开始 | 清空校验通过后才允许建计划；新计划全部由工具接口创建（禁止 INSERT） |

---

## 七、完成定义（DoD）

1. 七个动作（含**转发**）各有第二轮真实证据：状态=success、期望写端点、目标响应原文、写后回读；审计族含表单正文比对。
2. 有表单与无表单两条链路都跑通（装载 + 至少两个审批族动作），走同一套装载/分发/判据。
3. 完成判定完全由接口判据得出，页面信号只作证据（T2 落地 + 设计文档同步）。
4. 通道全程复用：G5 通过。
5. 保存草稿多次执行：G6 通过，同一实例不产生副本。
6. 测试门禁 G1–G8 全部满足；`bash test/run-executor-service.sh` 在本机服务运行时**真实用例不跳过**。
7. 文档与证据一致：F-036 状态回 `ready_for_manual`，逐项结论按证据重写，人工核对步骤更新；晨报按模板写。

**不要求**：其他审批动作（加签/移交/回退/取回/撤回/催办/关注）仍在 Go 直连路径，不改。

---

## 八、停止条件（满足任一，收尾写晨报）

1. 目标平台不可达、账号/SID 失效且重登失败（连续 3 次不同动作失败）。
2. 计划库不可连（后端起不来）。
3. 真实写预算用尽（≤12 实例）且仍有动作未验证。
4. 出现影响真实业务数据且无法解释的异常（例如审批再次清空表单正文、同一动作被写两次）。
5. 时间到 07:30。

**不许**：为求通过而放宽 G1–G8、删除有效断言、把跳过写成通过、把未验证写成 `accepted`。

---

## 附录 A：一页速查

```
装载模式：initiate（首发起）/ edit（待发编辑、重提）/ audit（审批）/ forward（转发，走列表页）/ view（只读）
动作→模式：save_draft 首次=initiate，再次=edit；submit=initiate；storage/approve/reject=audit；resubmit=edit；forward=forward
动作→端点：submit 族=/web/flowInstanceApi/submit；audit=/flowInstanceApi/audit；storage=/web/flowInstanceApi/storageFormData；
          resubmit=/web/flowInstanceApi/reSubmit；forward=/web/flowInstanceApi/transpond（+ /web/flowInstancePostscript/save）
动作→原生方法：sumbitFlow('draft'|'submit') / temporaryStorage() 或 temporarySaveBizData() /
          submitCheckBefore('pass'|'no_pass') / saveSubmit('save'|'submit') / transpond(tab)+confirmTranspond()
选择继续：handleSaveBranchChooseNode / handleSaveParallelChooseNode / getSelectPerson
```

## 附录 B：本轮必须闭环的缺陷清单

| 编号 | 缺陷 | 闭环方式 |
| --- | --- | --- |
| P0-A | 审批族写空表单正文、实例表单体被清空 | T6（就绪判据 + 写前快照 + 写后回读 + G7） |
| P0-B | 第二次保存草稿新建实例、run 停在待确认且被误归档 | T3（edit 模式 + 实例 ID 不变断言 + G6） |
| P1-C | 通道替换中止在途请求 | T1（单连接 + 禁止 attach 中止） |
| P1-D | 跳过即通过 | G1（`REQUIRE_REAL` 门禁） |
| P1-E | 判据被页面信号污染 | T2（接口判据 + 设计文档同步） |
| P1-F | 转发未实现 | T4（原生列表页链路 + 两笔写顺序） |
| — | 有表单/无表单口径不符 | T5（删除特殊处理 + G3） |
| — | 旧数据与旧证据干扰判断 | 第 0 步清空 + T7 归档与订正 |
