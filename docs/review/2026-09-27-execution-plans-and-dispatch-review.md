# 运行计划调度、派生状态与执行异常全面复盘报告

- **报告日期**：2026-09-27
- **覆盖运行**：Run ID `192` (Plan 70), `193` (Plan 69), `194` (Plan 68), `195` (Plan 67)
- **诊断范围**：计划列表状态派生、串行队列调度、自动放行时间呈现、流程图结构异常报错、多计划执行失败根因
- **诊断性质**：证据第一的只读根因诊断（依据技能 `diagnose-one-issue` 与项目规范 `AGENTS.md`，未获用户授权不修改代码）

---

## 一、诊断结论与责任归属矩阵

| 问题现象 | 核心表现 | 根因归属 | 处理建议与修复方向 |
|---|---|---|---|
| **1. 计划列表一直显示“运行中”且无法删除** | 串行计划在排队时列表显示为“运行中”，前端删除按钮被锁定 | **工具侧缺陷** | 1. 拆分 `internal/repository/mysql/plan_repository.go` 中的 `planDerivedStatusExpr`，将 `status IN ('pending', 'running')` 拆开，让 `pending` 运行正确派生为 `'pending'`（排队中）；<br>2. 修复 `internal/repository/mysql/run_repository_maintenance.go` 的 `DeleteRun`，补充 `DELETE FROM plan_run_queue WHERE run_id = ?` 级联删除；<br>3. 前端 `PlansView.vue` 放开对 `'pending'` 状态的删除权限。 |
| **2. 串行计划一直在排队未自动运行** | 感觉卡住不自动运行，手动停止后仍为运行中 | **非调度死锁，属认知差与状态感知错误** | 1. 调度器逻辑完全正常，今日 Run 193 包含 12 条路径依次串行执行，全程耗时 21 分 29 秒，期间后续 Run 194/195 必须严格等待其完成；<br>2. 修复问题 1 后，排队中的计划明确显示为“排队中”，消除“卡住不跑”的感知误解。 |
| **3. 自动放行步骤界面未显示开始时间** | 处于运行中/处理中的步骤没有显示开始时间 | **前端组件设计表现** | `RunNodePanel.vue` 第 522–524 行当前设计为：仅在步骤落账完成后（`item.status === 'executed'`）才展示 `开始于 XX:XX:XX`，处理中（`running`）展示说明文案。可优化为实时展示本步开始时间。 |
| **4. 两个计划查看详情报“结构异常”流程打不开** | 画布出现“流程暂时无法显示 · 目标流程结构异常” | **前端布局器拦截机制** | 文案来自 `web/src/features/flow-graph/layout.ts` 的 `safeLayoutFlowGraph`。当前库内 4 个计划的静态和当前动态图经核验均能正常布局。若目标流程存在孤立断边、条件分支缺少显式汇合节点或动态加签节点拓扑断开，布局器会触发单根树校验保护。 |
| **5. Plan 70 (Run 192) 全部路径在第 1 步失败** | 动作没有发出期望写请求 `/web/flowInstanceApi/submit` | **工具侧装载超时** | `executor-service` 在加载原生表单页面时超时（`page.waitForFunction: Timeout 60000ms exceeded at waitForDetailReady`），触发执行器零写入保护退出。 |
| **6. Plan 67 (Run 195) 提交报“选择要求未闭环：手动条件分支,请选择”** | 停在第 1 步提交 | **工具侧分发缺陷** | 目标平台返回 `custom_choose`（手动条件分支）。执行器 `nativeChoiceContinue` 当前硬编码查找 `ExamineOpinion`，而发起态弹窗组件是 `FlowDialog`，组件未对齐导致二次提交未触发。 |
| **7. Plan 67 (Run 195) 转发后审批报“未发出期望写请求 /flowInstanceApi/audit”** | 路径 4 转发成功后，审批步骤未发出请求 | **特殊业务组件未适配** | 路径 4 执行 `transpond` 成功生成辅助流程后，下一步审批针对的是合同盖章评审表单（`contract_seal_review`），原生组件委托给 `custom_contractSealField.contractBusiness`，在无头浏览器环境下缺少数据未触发主流程审核。 |

---

## 二、问题详细技术证据与链路复盘

### 1. 计划派生状态与删除拦截缺陷

- **代码调用链**：
  `GET /api/plans` -> `PlanRepository.List` -> 构造查询字段 `planDerivedStatusExpr`:
  ```sql
  CASE
    WHEN EXISTS(SELECT 1 FROM runs ar WHERE ar.plan_id = test_plans.id AND ar.status IN ('pending','running')) THEN 'running'
    WHEN EXISTS(SELECT 1 FROM plan_run_queue WHERE plan_id = test_plans.id AND status = 'pending') THEN 'pending'
    WHEN EXISTS(SELECT 1 FROM runs ar WHERE ar.plan_id = test_plans.id) THEN 'completed'
    ELSE test_plans.status
  END
  ```
- **证据事实**：
  - 串行计划入队时，`runs` 表记录的状态为 `pending`。
  - 第一条 `WHEN` 将 `pending` 与 `running` 一并收敛为 `'running'`，使得第二条 `plan_run_queue ... status = 'pending'` 永远无法命中。
  - 前端 `web/src/views/PlansView.vue` 第 253 行：`row.status !== 'running' ? h('span', ...)`，只要状态为 `running` 就直接隐藏删除按钮。
- **孤儿排队记录核验**：
  - `run_repository_maintenance.go` 的 `DeleteRun` 事务中未执行对 `plan_run_queue` 的清理。
  - MySQL 实测：`plan_run_queue` 表中存在 `id=13, run_id=191, plan_id=70, status='dispatched'`，而 `runs` 表中根本没有 `191`，形成了孤儿行。

### 2. 串行调度器运行轨迹

- **数据库时间戳复原**：
  - `Run 192 (Plan 70)`：10:29:17 启动 -> 10:30:16 终态（失败）
  - `Run 193 (Plan 69)`：10:30:16 自动出队 -> 10:51:46 终态（全部 12 条路径成功，耗时 21 分 30 秒）
  - `Run 194 (Plan 68)`：10:51:46 自动出队 -> 10:53:38 终态（路径 2 成功，路径 1 用户手动停止）
  - `Run 195 (Plan 67)`：10:53:38 自动出队 -> 10:54:48 终态
- **结论**：调度器严格按序自动放行，未发生阻塞；排队之所以显得长，是因为 Plan 69 包含 12 条完整审批路径，耗时较长。

### 3. Plan 70 (Run 192) 原生页面超时

- **日志证据**：`.runtime/executor-stdout.log`：
  ```text
  [executor] 装载失败 mode= edit error= page.waitForFunction: Timeout 60000ms exceeded.
      at waitForDetailReady (executor-service/src/server.ts:103:14)
  ```
- 目标平台表单在无头 Chromium 中未能于 60 秒内渲染就绪，触发了执行器的零写入保护，未向目标平台发出错误请求。

### 4. Plan 67 (Run 195) 发起态手动条件分支未闭环

- **日志证据**：`logs/runs/欧阳改测试2__运行1/paths/路径 1__2026-09-27 10-53-38/step.log`：
  ```text
  submit message=runtime 判定失败：选择要求未闭环：手动条件分支,请选择
  ```
- **代码根因**：
  - 在 `executor-service/src/native-dispatch.ts` 第 169 行：
    ```ts
    const opinion = find(container, 'ExamineOpinion')
    ```
  - 当动作是 `submit` 时，原生宿主是发起弹窗 `FlowDialog`（`native.initFlow`），内部根本没有 `ExamineOpinion` 组件。
  - `nativeChoiceContinue` 强行查找 `ExamineOpinion` 抛出组件未找到异常，导致第二次带有分支选择的提交未能发出。

---

## 三、待用户批准的修复方案

在用户明确批准进入 `implementing` 阶段前，当前工作区保持只读，不修改生产代码与配置。后续实施计划建议包含：

1. **修正计划派生状态**（`internal/repository/mysql/plan_repository.go`）：
   - 拆分 `pending` 与 `running`，使排队中的计划派生为 `'pending'`；
   - 在 `web/src/views/PlansView.vue` 中放开对 `'pending'` 计划的删除操作。
2. **完善运行级联删除**（`internal/repository/mysql/run_repository_maintenance.go`）：
   - `DeleteRun` 补充清理 `plan_run_queue`，并清理库中残留的 `run_id = 191` 记录。
3. **修复发起态 `custom_choose` 闭环**（`executor-service/src/native-dispatch.ts`）：
   - `nativeChoiceContinue` 支持识别 `container` 为 `FlowDialog`，并将分支选择注入到 `FlowDialog` 组件（调用原生方法 `handleSaveBranchChooseNode`）。
