# F-018 真实对账与恢复动作操作手册（手动执行，默认不随 test/run-f018.sh 自动运行）

本手册记录 2026-09-05 用户授权的真实不确定写与对账收口过程，供复现与人工核对。
真实写只在用户指定的可污染账号（`.env.local` 的 `TEST_AUTO_PRO_TARGET_ACCOUNT`）上执行；
账号与口令只存在于被 Git 忽略的 `.env.local`，不入仓库。

## 与自动测试的边界

`test/run-f018.sh` 只跑只读用例与真实 MySQL 集成用例（目标读写是假件），不会向目标平台发写请求。
本手册的第 2 步会发出真实写请求，必须人工确认后执行。

## 前置条件

1. `.env.local` 配齐 `PLAN_DB_*`、`TARGET_*`、`TEST_AUTO_PRO_TARGET_ACCOUNT`。
2. 一条运行前检查判定为可启动的执行路径（本次用计划 11 路径 1121）。
3. 后端已启动（开发用 `go tool air`，或 `go build -o /tmp/f018-server ./cmd/server && /tmp/f018-server`）。
   注意：对账现场在服务进程内存里，重启即丢失，所以从启动到执行恢复动作要在同一个进程生命周期内完成。

## 逐步操作

1. 启动运行（单步模式，此时还没有任何写请求）：
   `curl -s -X POST http://127.0.0.1:19080/api/plans/11/runs -H 'Content-Type: application/json' -d '{"planId":11,"executionPathId":1121,"mode":"single_step"}'`
   核对响应里 `currentPreview` 的动作、演员、门禁结论与请求预览。
2. 放行第一步（**发出真实写请求**）：
   `curl -s -X POST http://127.0.0.1:19080/api/runs/<runId>/approve -H 'Content-Type: application/json' -d '{"command":"step","cursor":1,"controlVersion":1}'`
   核对步骤事实：三值判定、trace_id、`logPath:logLine`。
3. 若判定为不确定（路径运行进入待对账），执行只读对账：
   `curl -s -X POST http://127.0.0.1:19080/api/runs/<runId>/reconcile`
   核对：一句中文结论、五维逐条依据、`action` 只有一个、`replaysUsed/replaysMax`。
4. 试探性地提交一个与结论不匹配的动作，应被中文原因拒绝且不发任何写请求：
   `curl -s -X POST http://127.0.0.1:19080/api/runs/<runId>/recovery -H 'Content-Type: application/json' -d '{"action":"replay"}'`
5. 执行对账给出的唯一动作。仍无法判定时是登记人工核对结论（登记的是**你在目标平台上亲眼看到的事实**）：
   `curl -s -X POST http://127.0.0.1:19080/api/runs/<runId>/recovery -H 'Content-Type: application/json' -d '{"action":"manual_end","instanceStatus":"run","currentNode":"资源开发部","reporter":"<你的名字>","note":"<只读复查看到的事实>"}'`
6. 核对落库与日志：`run_step_attempts` 的对账三列、`run_manual_conclusions` 一行、
   `run_events` 的 `reconciled` 与 `manual_conclusion`、运行目录下的 `recovery.log`/`step.log`/`curl.log`。
7. 需要中止时：`curl -s -X POST http://127.0.0.1:19080/api/runs/<runId>/stop`（已发生的事实全部保留）。

## 2026-09-05 实测记录

### 运行 12：真实不确定写 + 首次真实对账

- 计划 11（oyg00，账号骆蒙恩）路径 1121，单步模式，放行第 1 步「发起」。
- 目标受理（isSuccess=true），实例 `6bd617f3069d462d8bfe63ba12b35739`，写请求耗时 464ms，
  trace `c6ea7555b51afaee`；核验重读读不到实例 → 「成功声明 + 明确未变」→ 不确定 → 待对账。
- 只读对账（真实目标）结论「仍无法判定」，唯一动作「登记人工核对结论并结束」。五维真实读数：
  实例状态/当前节点/当前待办三维「按实例精确复查，实例在已发列表不可见」，已办与动作痕迹按真实读取给出。
- 用 `replay` 试探被拒：「当前唯一合法动作是 manual_end，不接受 replay」，零写请求。
- 登记人工结论后：对账三列 = `indeterminate` / `manual_end` / `is_replay=0`；
  `run_manual_conclusions` 一行；`recovery.log` 首次有真实内容。
- 日志目录：`logs/plans/oyg00__plan-11/runs/路径 1__path-1121/12/`。

### 只读复查：这次「不确定」是工具读取口径问题

- `/web/flowAuditRecord/list` 读到该实例的「流程发起」审核记录（执行人骆蒙恩）→ 写确实已生效。
- 逐项试验定位：`/web/flowInstanceApi/list` 带 `flowInstanceBizRelevanceList=[{otherBiz:company,otherBizId:""}]`
  返回空集，去掉即命中。结论落语义清单第 19 条，修复见 `FindSubmittedFlow`。

### 运行 13：修复后的验证运行

- 同一条路径再跑一次真实发起：实例 `7bcf0c29f1054ba0bed5cf367ba2f2d2`，trace `0715f0cdf7cc2700`，
  判定**确定成功**（重读结论 advanced），路径前进到第 2 步。
- 第 2 步被路径偏离规则挡住（实际命中分支「资源开发部」与已配置路径不一致），按设计不提供放行，
  已用 `stop` 收尾（路径运行与运行聚合都是已停止）。

### 本次没能取得的真实证据

「未生效 → 重放」的真实目标端到端证据仍未取得，原因是结构性的（三条实测原因见
`docs/features/F-018-reconciliation-safe-retry.md`），不是少跑一次实验：发起类写永远得不到
「未生效」；对已有实例动手的执行接线属 F-019；且该账号手上没有任何审批待办
（实测 169 条全是草稿 2 / 驳回 83 / 撤回 84，审批待办 0）。
该链路的正确性由 `test/unit/backend/executor/f018_recovery_reachable_test.go`
（真实 MySQL + 真实状态机 + 真实仓储，只有目标读写是假件）保证。
