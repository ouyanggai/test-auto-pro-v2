# F-016 真实写闭环操作手册（手动执行，默认不随 test/run-f016.sh 自动运行）

这是本项目唯一会产生目标平台真实写数据的操作流程。只在用户指定的可污染真实测试账号
（`.env.local` 的 `TEST_AUTO_PRO_TARGET_ACCOUNT`）与该账号可见的流程上执行；
账号与口令只存在于被 Git 忽略的 `.env.local`，不入仓库。

## 与自动测试的边界

`test/run-f016.sh` 只执行只读用例（假目标、真实 MySQL 事实表），不会向目标平台发任何写请求。
本手册的每一步都写真实数据，必须人工逐步确认后执行。

## 前置条件

1. `.env.local` 配齐 `PLAN_DB_*`、`TARGET_*`、`TEST_AUTO_PRO_TARGET_ACCOUNT`。
2. 一条「新发起」来源、运行前检查判定为可启动的执行路径（记下 `planId` 与 `pathId`）。
3. 后端已启动：`go build -o /tmp/f016-server ./cmd/server && /tmp/f016-server`。

## 逐步操作（每步停在人面前）

1. 启动运行（模式强制单步）：
   `curl -s -X POST http://127.0.0.1:19099/api/plans/<planId>/runs -H 'Content-Type: application/json' -d '{"planId":<planId>,"executionPathId":<pathId>}'`
   确认响应里 `currentPreview` 的动作、演员、门禁结论与请求预览，**此时还没有任何写请求发出**。
2. 放行第一步（发出真实写请求）：
   `curl -s -X POST http://127.0.0.1:19099/api/runs/<runId>/approve`
   核对响应里步骤事实：三值判定、trace_id、`logPath:logLine`。
3. 打开日志目录 `logs/plans/<计划>/runs/<路径>/<运行号>/`：
   `step.log` 应有七阶段流水，`curl.log` 的命令可直接重放，`network.log` 的 `request_class=write`。
4. 每次放行后回目标平台核对实例状态与待办；写结果判不确定时路径运行停在待对账，
   **不提供任何重试入口**，等待 F-018 的对账。
5. 需要终止时用 `POST /api/runs/<runId>/stop`（已发生的事实全部保留）。

## 2026-09-05 首次真实写实测记录（T09）

- 计划 11（oyg00，账号骆蒙恩）路径 1121（路径 1，新发起）。
- 运行 8：提交被目标受理（isSuccess=true，实例 `caf2046d896f477c81c819153fc7d52f`，status=run，
  当前节点=所选分支「资源开发部」，分支选择经 `nextAuditorList[].nodeProxyId` 传递——语义清单第 15 条）。
- 核验重读即时未在已发列表见到实例 → 按判定矩阵「成功声明+明确未变=不确定」→ 路径运行进入待对账并停止，零重发。
- trace_id：`5ad3ab3454cce08d`；日志：`logs/plans/oyg00__plan-11/runs/路径 1__path-1121/8/`。
- 由此登记「提交」为已验证动作（`internal/service/run_readiness.go` 的 `verifiedRunnableActions`）；
  「同意」尚未在真实目标执行过，不登记。
