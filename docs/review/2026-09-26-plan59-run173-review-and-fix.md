# 计划 59「欧阳改测试 09」运行 #4：根因复核与工具侧修复

- **报告日期**：2026-09-26
- **运行**：Run ID `173`，Run No `4`
- **计划**：Plan ID `59`
- **路径范围**：12 条真实路径
- **本轮状态**：工具侧修复已实施并通过自动验证，切片停在 `ready_for_manual`

## 一、结论

本次运行 12 条路径中，路径 2、3、4、5、7、8、11、12 完成；路径 1、6 因新引入的循环 Bug 持续运行近 40 分钟后被停止；路径 9、10 停止。

| 路径 | 停止位置 | 根因归属 | 处理结论 |
|------|----------|----------|---------|
| 1 | 步骤 8 取回被目标跳过后触发无限循环（8,610 次） | 工具侧 Bug（本轮新引入） | 已修：跳过态取回不触发代理树重绑定 |
| 6 | 步骤 7 取回被目标跳过后触发无限循环（20,907 次） | 工具侧 Bug（本轮新引入） | 已修：同路径 1 |
| 9 | 步骤 20 取回成功后处理人 20 次轮询超时 | 目标平台异步（已知，上轮同结论） | 工具行为正确，等待目标 pending 生成后人工复验 |
| 10 | 步骤 19 取回成功后处理人轮询超时 | 目标平台异步（已知） | 同路径 9 |

## 二、路径 1、6 无限循环根因

### 触发链路

1. 步骤 8（路径 1）/ 步骤 7（路径 6）为取回动作，其前置节点 `38404fe8…` 被目标 `isSkip=true` 自动跳过。
2. 运行 172 的修复正确识别了这一情况，将取回落账为 `target_skipped`（零写入），`StepOutcome.Verdict = confirmed_success`。
3. 控制层 `approveOneStep`（`control.go:1336`）原先无条件对所有取回步骤设置 `remapAfterRetrieve = true`。被跳过的取回没有发出写请求，`outcome.CurrentNodeProxyIDs` 为空。
4. `ValidateCurrentNodesInRemap` 以「写后目标未返回当前节点」为由失败，控制层随即设置 `session.deviationStalled = true`，写入 `proxy_remap_failed` 事实。
5. 但 `remapStopped` 判定（`control.go:1442`）需要字符串包含「写后目标代理树」，且只在 `remapStopped=true` 时 `return result`——**这一 early return 发生在 `outcome.Verdict` 写回 `result` 之前**，导致 `result.Outcome.Verdict` 保持 `confirmed_success`。
6. 自动循环 `runLoop` 检查 `result.Outcome.Verdict != "confirmed_success"` 来决定是否继续，条件不满足，循环继续，下一轮再次 approve 同一步骤，永无止境。

路径 1 在 15:17-15:57 运行了约 40 分钟、路径 6 循环 20,907 次，均由用户停止才终结。

### Java 对照说明

目标侧没有参与：取回步骤整个处于零写入状态，目标代理树未变化，本不需要重绑定。Bug 完全在工具内部的条件判定逻辑。

## 三、路径 9、10 的目标异步事实

取回 API 返回 `{"data":null}`，无 `currentNodeProxyId`。工具从写后重读的 `flowInstanceApi/list` 获得实例当前节点 `6f91b42e…`，通过代理树重绑定把 NodeKey `f5e7e958…` 映射到该运行时 ID。

步骤 20 在节点 `6f91b42e…` 上等待处理人生成，轮询 20 次（约 80 秒）后超时，按 `assignment_missing` 阻断。

这与 Java `FlowInstanceServiceImpl.saveRetrieveProcess` 的 `createPendingJob` 语义相符——目标应产生 pending，但在实测的 80 秒内始终未出现。**工具行为正确**：等够再判，不把「目标还没生成」当成成功。

人工复验建议：在取回成功后等待更长时间（如 3-5 分钟）再复跑，排除目标异步延迟；若延迟后仍无处理人，则记录为目标平台侧问题。

## 四、本轮代码修复

### Fix 1：`internal/engine/control/control.go`

`remapAfterRetrieve` 增加 `!session.preview.TargetSkipped` 条件：

- **改前**：所有取回步骤都触发重绑定，包括零写入的跳过态取回。
- **改后**：只有取回步骤实际发出了写请求（`TargetSkipped=false`）时才触发重绑定。零写入跳过时目标代理树未变化，不需要也不能重绑定。

### Fix 2：`internal/engine/control/runner.go`

`runLoop` 循环入口增加 `deviationStalled` 安全阀：

- **改前**：`deviationStalled=true` 但 `result.Outcome.Verdict="confirmed_success"` 时，循环不退出。
- **改后**：每轮开始时检查 `deviationStalled`，为 `true` 立即 `return`，防御任何可能产生此状态的未来路径。

## 五、自动验证

- `go test ./test/unit/backend/executor -count=1`：通过
- `go test ./test/unit/backend/control -count=1`：通过
- `go build ./internal/engine/control/... ./internal/engine/step/...`：通过
- `go vet ./internal/engine/control/... ./internal/engine/step/...`：通过
- `git diff --check`：通过

本轮未连接目标平台、未发真实写请求、未启动浏览器。

## 六、人工验收交接

1. 重新运行路径 1、6：确认取回被目标跳过后步骤正常落账完成，路径正常结束，不再出现 8,000+ 次循环，网络日志无取回写请求。
2. 重新运行路径 9、10：在取回成功后等待 3-5 分钟后观察目标是否生成 pending 和处理人；若仍无，保留为目标平台异步问题记录。
3. 确认路径 2、3、4、5、7、8、11、12 行为无回归（原已完成路径）。

完成自动验证后本切片停在 `ready_for_manual`，不替代用户验收，也不自动开始下一项功能。
