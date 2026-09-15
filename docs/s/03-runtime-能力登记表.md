# runtime 能力登记表（T014）

本表是「哪些表单/动作组合可以由 runtime 执行」的唯一事实来源。执行器路由（T012）以
`internal/runtimeexec.CapabilityRegistry` 的登记为准：**未登记即阻塞，绝不回退 Go 写出口**。

## 登记纪律

1. 只有真实执行成功（协议判定 `success`）且 Go 目标事实核验为 `confirmed_success` 的组合才允许登记
   （`CapabilityRegistry.UnlockFromVerifiedRun` 强制只认 `confirmed_success`）。
2. 登记粒度为「流程类型 + 动作 + 表单摘要」三维：
   - 同流程提交成功**不**自动解锁同流程的同意/不同意/重新提交/转发；
   - 同端点一个动作成功**不**自动解锁另一个动作（同意与不同意是两个动作键）；
   - 其他流程模板不因本流程成功而解锁。
3. 每条登记必须携带：runtime 版本（协议版本 `exec-engine/v1`）、目标前端版本（快照 HEAD）、
   目标事实证据（实例 ID / trace ID）与日志引用（`logs/runs/...` 路径）。缺一不得登记。

## 当前登记（截至 2026-09-15）

| 流程类型 | 动作 | 表单摘要 | runtime 版本 | 目标版本 | 证据 | 日志引用 | 登记时间 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| （空） | （空） | （空） | — | — | — | — | — |

**当前登记为空**：T015 真实目标全动作闭环验证（15 个用户动作 + 系统自动动作）尚未执行，
在用户完成人工验收并逐动作登记之前，配置了 runtime 能力的执行器会阻塞全部七个表单动作
（提示「runtime 能力未登记」），保持既有 Go 写路径动作不受影响。

## 明确记录的阻塞原因（不可登记，直接阻塞）

| 原因常量 | 说明 |
| --- | --- |
| `BlockedUnsupportedComponent` | 表单包含未登记的目标自定义组件（无头环境下无法确认其业务保存结果） |
| `BlockedOnlineEditor` | 依赖在线 Office 编辑器且初始化流程未验证 |
| `BlockedUnconfirmableResult` | 动作结果无法由请求轨迹与目标回读确认 |
| `BlockedDragOrRealSize` | 依赖真实尺寸或拖拽交互（无头环境不模拟） |
| `BlockedUnregisteredExternalWS` | 存在未登记的外部网络请求（不能绕过 Go 转发） |

以上常量定义于 `internal/runtimeexec/capability.go`；登记流程遇到时如实记录原因并阻塞当前动作。

## 登记流程（人工验收通过后执行）

1. T015 真实闭环中某组合全链路成功：请求轨迹完整、目标事实核验 `confirmed_success`。
2. 从该运行的 `step.log` / `network.log` 提取证据（实例 ID、trace ID）与日志路径。
3. 调用 `CapabilityRegistry.UnlockFromVerifiedRun` 写入登记（持久化由装配层在启动时从本表载入）。
4. 更新本表：每个动作一行，一个组合一行，不合并。
