# T015 真实目标全动作闭环验证清单

状态：**ready_for_manual**（等待用户人工验收）。本清单是执行引擎改造收口的唯一人工验收材料。
自动验证（离线全栈、假目标对端）已由 `bash test/run-runtime-exec.sh` 覆盖并通过；
真实目标写验证按本清单逐动作人工执行，**每项独立记录请求轨迹与目标事实证据，失败动作绝不重复提交**。

## 通用前置（每个动作都适用）

1. 使用指定的可污染测试账号（环境变量注入，账号值不入仓库）；同一账号串行、不同账号并行。
2. 每个动作一次尝试：写请求至多一次；响应丢失 → 结果待确认，不重发。
3. 每项验收证据固定四件套：**运行编号 / 实例 ID / trace ID / 日志路径**（`logs/runs/...`）。
4. 目标事实核验：动作后重读实例状态、当前节点、待办、表单数据，与人工操作对照。
5. 环境变量：`RUNTIME_TARGET_BASE_URL`、`RUNTIME_PLAN_ACCOUNT/PASSWORD`（发起人）、
   `RUNTIME_AUDITOR_ACCOUNT/PASSWORD`（审批人，需持有真实待办）。

## 执行环境启动命令（runtime 路由接入后的唯一真实验证入口）

```bash
# ① 构建 form-runtime 产物（执行服务的静态现场）：
pnpm --dir form-runtime build
# ② 以 runtime 路由 + 验证模式启动后端（端口 19080）——托管模式会自动拉起 19011 执行服务：
RUNTIME_EXECUTOR_URL=http://127.0.0.1:19011 RUNTIME_EXEC_VERIFICATION=1 go run ./cmd/server
#    （不想让后端托管执行服务时，置 RUNTIME_EXECUTOR_MANAGED=0 并手动执行：
#      cd form-runtime/executor && EXECUTOR_PORT=19011 node --experimental-strip-types src/server.ts）
# ③ 启动前端（端口 19000，代理 /api 到 19080）：
pnpm --dir web dev
# ④ 在浏览器打开 http://127.0.0.1:19000 → 测试计划 → 目标计划「运行」，
#    单步放行逐动作执行；证据回填到下方清单。
```

纪律说明：`RUNTIME_EXEC_VERIFICATION=1` 是 T015 真实验证专用——未登记组合放行执行一次以取得证据，
动作经目标事实核验判定 `confirmed_success` 后自动按「流程+动作+表单摘要」登记能力（step.log settle
阶段可见登记行），登记同时落盘 `.runtime/runtime-capabilities.json`，后端重启不丢失；
验证结束后关闭该变量重启后端，未登记组合恢复「阻塞不回退」的正常口径。

## 表单矩阵

| 编号 | 表单形态 | 说明 |
| --- | --- | --- |
| F-A | 普通 FormMaking 表单 | 员工请假单同构：输入 + 静态/远程下拉 |
| F-B | 含 JSON 业务脚本表单 | `beforeSubmitAndDraft`/`afterSaveFlowInstance` 有真实业务副作用 |
| F-C | 含附件表单 | 附件上传 + `relationFile/saveBatch` 绑定 + 状态回写 |
| F-D | 含自定义组件表单 | 目标自定义组件（合同盖章等）；未登记组件按阻塞口径处理 |

## 动作清单（15 个用户动作 + 系统自动）

| # | 动作 | 表单 | 执行位置 | 验收要点（与人工操作对比） | 证据回填 | 状态 |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | 保存草稿 | F-A | runtime | 草稿实例存在 status=draft；无待办生成；节点不变；`beforeSubmitAndDraft` 已执行 | 运行__/实例__/trace__/日志__ | ☐ 待人工 |
| 2 | 提交 | F-A → F-B → F-C | runtime | 实例 run；当前节点/处理人/待办与目标一致；工具流程名称不被覆盖；选择闭环仅继续一次 | 同上 | ☐ 待人工 |
| 3 | 审批暂存 | F-A | runtime | 暂存记录存在且实例/节点/处理人一致；实例状态、任务、待办**不变** | 同上 | ☐ 待人工 |
| 4 | 同意 | F-B | runtime | 待办完成；下一节点/批次/处理人一致；`auto_audit_info_*` 以目标为准；同意专属分支未误走不同意 | 同上 | ☐ 待人工 |
| 5 | 不同意 | F-A | runtime | 实例进入 rejected/退回状态；发起人重提入口可见；未调用同意专属业务 | 同上 | ☐ 待人工 |
| 6 | 重新提交 | F-A | runtime | 实例离开 draft/rejected/withdraw 进入 run；**不创建第二个主实例** | 同上 | ☐ 待人工 |
| 7 | 转发 | F-A | runtime | 辅助实例单独记录（ID/接收人/dynamicParam/业务关联）；附言记录关联辅助实例；**主实例状态/节点/待办不变** | 同上 | ☐ 待人工 |
| 8 | 加签 | F-A | go_target | 实例私有代理树更新（整树）；代理任务映射一致 | 同上 | ☐ 待人工 |
| 9 | 移交 | F-A | go_target | 当前待办处理人变更、节点不变；batchNo/flowNodeProxyId/userIds 与页面同源 | 同上 | ☐ 待人工 |
| 10 | 回退上一节点 | F-A | go_target | 回到直接前驱；前驱待办/处理人重建；批次一致 | 同上 | ☐ 待人工 |
| 11 | 取回 | F-A | go_target | 新批次建立、原审批节点恢复待办；后继任务未被处理过 | 同上 | ☐ 待人工 |
| 12 | 撤回 | F-A | go_target | 创建人撤回运行中实例；状态一致 | 同上 | ☐ 待人工 |
| 13 | 催办 | F-A | go_target | 催办记录 +1；实例状态/待办不变 | 同上 | ☐ 待人工 |
| 14 | 关注 / 15. 取消关注 | F-A | go_target | 仅关注状态变化；实例状态/节点/待办不变 | 同上 | ☐ 待人工 |
| 16 | 系统自动节点 | — | read_only | 条件/并行/汇聚/空/结束节点只读导航事实；无写请求 | 只读核验 | ☐ 待人工 |

## 明确不纳入本切片

- **待发删除**（`/web/flowInstanceApi/delete`，`DueOut/index.vue`）：破坏性管理操作，不加入动作目录，
  不由 runtime 或 Go 执行器调用。计划遇到该入口显示「不纳入自动化动作」。
- 未支持能力（如实记录阻塞原因，不修改代码伪造通过）：未登记自定义组件、在线 Office、
  依赖真实尺寸/拖拽、未登记外部网络请求、无法由请求/回读确认结果的组件。

## 失败处理纪律

- 任何动作失败/待确认：停止本步，保留现场与日志，**不重复提交**；按对照文档失败规则处理。
- 响应丢失（转发失败/httpStatus=0）→ 判定 `uncertain` → 结果待确认，等待目标事实核对。
- 全部 15 项人工回填证据并通过后，本改造才进入 `accepted`；在此之前保持 `ready_for_manual`。
