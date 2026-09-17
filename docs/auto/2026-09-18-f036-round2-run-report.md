# F-036 第二轮修复暂停报告

- 日期：2026-09-17
- 依据：`docs/auto/2026-09-17-f036-round2-unified-native-execution-repair-task.md`
- 当前状态：`implementing`
- 停止原因：用户明确要求标识当前完成进度后停止

## 已完成

1. T1 通道复用：Go 的 `Prewarm` 与 `Execute` 共用会话唯一 WebSocket；执行服务拒绝第二条活动连接，旧连接关闭事件不误伤新连接；十次连续命令复用一条连接的单测通过。
2. T2 接口判据：终局只由动作期望写端点的 HTTP 状态与响应包络裁决；页面异常只保留证据；SID、普通业务拒绝、选择闭环、无写、错端点、不确定、超时和转发附言失败均有契约测试。
3. T3 多次草稿：首次走 `initiate`，已有主实例走 `edit` 与 `saveSubmit("save")`；响应实例 ID 不一致按重复实例缺陷失败。
4. T4 转发代码：装载 `#/flow/approve/<tab>`，按实例 ID + 任务 ID 唯一选中列表行，调用 `transpond(tab)` 准备后再调 `confirmTranspond()`；主写与附言写在实际请求路由层按 1、2 序号串行并防重复。
5. T5 统一渲染类型：删除 runtime 的 FormMaking 路由限制和 FormProxyID 门禁；暂存按方法存在性选择 `temporaryStorage` 或 `temporarySaveBizData`，数据接口按读写能力发现。
6. T6 正文保护：详情装载等待非空表单正文；保存 `formBefore/formAfter/fieldCountBefore/fieldCountAfter`；审批族写后由 Go 重读目标，字段减少或回读失败均改判 `business_failed`；业务摘要进入正式 runtime 详情。
7. T7 部分完成：旧 `docs/auto/evidence/` 的 12 份文件已移入 `docs/auto/archive/2026-09-17-f036-round1-evidence/`；代码中的旧完成判据和特殊路由已删除。
8. 第 0 步完成：停止后端并确认 19080 释放；真实 `SHOW TABLES` 得到 20 张表；清空除 `schema_migrations` 外的 19 张表并逐表确认 0 行；`logs/runs`、`logs/plans` 均为空；重启后端确认 `/api/health` 正常、`/api/plans` 为空。

## 已验证

- `executor-service` TypeScript：`tsc --noEmit` 通过。
- 执行服务 Node 单元/集成：73 通过、0 失败。
- Go：`./test/contracts/runtimeexec` 与 `./test/unit/backend/runtimeexec` 通过。
- 静态检查：生产代码没有 `FormRenderTypeFormMaking` runtime 分支、`businessDone` 或旧页面错误终局函数。
- `git diff --check` 通过。

## 未完成，恢复时从这里继续

1. 尚未执行 `TEST_AUTO_PRO_F036_REQUIRE_REAL=1` 下的真实 L1/L2；本轮没有新增目标平台写入，也没有生成 `docs/auto/evidence-round2/`。
2. 尚未通过工具 API/界面创建计划 A–G；G8 只完成“从空库开始”，未完成新计划构建。
3. G2、G5–G7 尚缺真实证据：七动作四件套、通道真实日志、同实例两次草稿、审批正文写后完整、真实转发两笔写均未验证。
4. 无表单 `vue_custom/NoFormFlow` 的真实暂存与同意尚未执行。
5. `bash test/run-executor-service.sh` 尚未作为最终聚合门禁执行；设计文档、产品文档和旧晨报订正也尚未完成最终同步。
6. 功能不得标记 `ready_for_manual` 或 `accepted`，不得开始下一功能。

## 目标平台清理仍由用户执行

工具没有删除目标平台实例。仍需用户自行核对并清理任务书列出的旧测试实例：
`c3aab9235d6c4b75b54a55c5c1bfe6e1`、`06c4a0d8…`、`bbfbbac5d74b4a9ab0acf123b16b98b1`、
`bc847fa12c5f4307b9d33e00d45361b0`、`cd9c8b01fdc3474485de18da37bc68fd`、`d71ee9684cd446df8f5e9452d250a9b9`。
