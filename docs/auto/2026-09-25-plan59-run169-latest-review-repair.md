# Plan 59 / Run 169 最新提交复审与修复方案

- 编写日期：2026-09-25
- 审查范围：`ff75a305`（表单核对与运行详情）、`de015374`（上一轮审查报告）以及其直接依赖的当前执行引擎代码
- 相关审查记录：`docs/review/2026-09-25-plan59-run169-executor-issues-review.md`
- 本文性质：复审结论与下一次修复方案；本次只写文档，没有修改业务代码，也没有代替真实目标平台验收
- 完成标准：明确区分已经生效的修复、仍可复现的代码缺陷和目标平台自身问题；为每个代码缺陷给出最小修复边界、回归用例和人工验收步骤

## 1. 结论

`ff75a305` 已经解决了路径 11 中 `flowInstanceId` 被写后核对误判为“字段丢失”的一条阻塞链，也把 runtime 尝试详情和非终态落账事件接到了页面。`de015374` 对 Run 169 的路径现象记录完整，但其中“取回准备同意被目标跳过判定误伤”的代码缺口在当前分支已经由 `8372beba` 的 `RetrievePrepareConsent` 保护和现有回归测试覆盖，不应再次按未修复问题实施。

复审仍发现以下问题，建议按顺序修复：

1. **P1：runtime 写后正文保护仍把系统结构字段当业务字段统计。** 新的 `IsSystemStructuralField` 只接入了普通执行器的字段核对，`protectRuntimeFormBody` 仍会把 `flowInstanceId` 计入写前字段数。runtime 的写后回读缺少它时，仍可能被判为正文减少并改成 `business_failed`。
2. **P1：会签/并行节点仍可能因“当前节点和后续节点同时存在”被判未推进。** `advancedByNodePosition` 发现本步节点仍在 `CurrentNodes` 时立即返回 `false`，还没有检查同一快照中是否已经出现合法后续节点。Run 169 路径 8 的待对账现象仍可由这条逻辑解释。
3. **P1：runtime 身份读取失败被静默回退到会话缓存。** `sessionFactsForRuntime` 忽略 `CurrentUserIdentity` 错误，而协议实现明确要求读取失败时阻塞；这会让 runtime 继续使用不完整或过期的公司、部门、岗位事实。
4. **P2：终态事件没有携带本步接口明细。** `BackToRunning` 才写入 `Detail`，最后一步进入 `Finish` 时仍只写普通标签，事件流看不到终态节点的接口和结果。动作详情仍可查尝试记录，但事件流的“按节点核对”不完整。
5. **P2：页面“目标平台原始返回”区只消费 `runtimeDetail`。** 普通 Go 目标写动作没有这个投影，页面只能看到已有的请求摘要和加工后的 `Reason`，不能保证显示目标原始 `message`。如果产品要求所有动作统一显示原文，需要补齐非 runtime 尝试的结构化返回摘要。

## 2. 已确认有效的改动

### 2.1 `flowInstanceId` 的普通写后核对误报已消除

`internal/adapter/target/field_ownership.go:97-108` 登记了系统结构字段，`internal/engine/step/executor.go:1266-1340` 在覆盖字段和基线保留字段两处核对循环中跳过它。这样能消除 Run 169 路径 11 的已知误报，且没有把业务字段 `instanceId` 一并排除；新增的 `system_structural_field_test.go` 锁定了这条边界。

### 2.2 取回准备同意的跳过保护已经存在

`internal/engine/step/executor.go:815-827` 对恢复型步骤退出通用目标跳过判定，`isRecoveryEffectStep` 在 `internal/engine/step/executor.go:1391-1402` 将 `RetrievePrepareConsent` 归为恢复前置步骤。现有 `retrieve_prepare_consent_test.go` 和 `recovery_skip_guard_test.go` 同时覆盖“已有正确已办时可跳过”和“不能被通用自动跳过误判”两种语义。后续修复不得删除这层区分。

### 2.3 runtime 详情和运行事件接线已通过静态与编译检查

`internal/runtimeexec/detail.go` 会保存状态、消息和逐请求方法/路径/HTTP 状态；`RunNodePanel.vue` 能渲染 runtime 尝试；`RunEvent.Detail` 已由服务层透出。当前实现仍需真实浏览器重跑确认页面数据是否来自目标实际返回。

## 3. 仍需修复的问题与最小方案

### 3.1 P1：系统结构字段只从普通核对中排除，没有从 runtime 正文保护和实际载荷中统一排除

证据链：

- `internal/engine/step/formdata.go:120-273` 在构造 `merged` 时仍会把配置快照中的 `flowInstanceId` 放入 `OverlaidFields` 和最终 `Payload`；当前提交只改变了核对，不改变实际写载荷。
- `internal/engine/step/executor.go:456-529` 的 `protectRuntimeFormBody` 只用 `cleanFormKeys` 清理空键，再由 `countMissingBusinessFields` 对所有键计数，没有调用 `target.IsSystemStructuralField`。
- 因此同一份带历史 `flowInstanceId` 的表单快照，在普通 Go 写路径上不再误报，但在 runtime 的正文保护路径上仍可能因为目标回读不含该键而判定 `business_failed`。

修复边界：

1. 在 target 侧保留唯一的系统结构字段判定入口，普通构造、普通写后核对和 runtime 正文保护都复用它。
2. 构造业务表单载荷时直接剔除系统结构字段；实例 ID 继续由请求协议的顶层 `instanceId`/实例上下文承担，不能从历史 formData 透传。
3. 正文保护在统计和“缺失业务字段”比较前过滤系统结构字段，保持空键清洗和业务字段缺失报警逻辑不变。
4. 不要用模糊前缀一次排除所有 `flow*` 键；新增结构字段必须有目标证据、字段归属说明和单独回归测试。

必须补的回归场景：

- 历史配置含 `flowInstanceId`，实例当前数据不含该键：最终 payload 不含该键，普通核对通过。
- runtime `formBefore` 含 `flowInstanceId`、`formAfter` 不含该键、业务字段完整：正文保护保持 `success`。
- 业务字段真实缺失：仍然判 `business_failed`，证明排除清单没有吞掉业务数据问题。

### 3.2 P1：会签/并行节点的推进判定需要“本步写入证据”，不能只看节点集合

证据链：

- `internal/engine/step/verify.go:953-997` 的 `advancedByNodePosition` 先检查本步节点；只要本步节点仍在 `after.CurrentNodes`，立即返回 `false`，后续合法节点即使同时存在也不会被检查。
- `ClassifyReread` 在 `internal/engine/step/verify.go:820-930` 先调用该函数，再根据当前待办和已办分型；因此“当前节点仍有其他会签人待办 + 实例已出现后续节点”的快照会落到 `RereadUnchanged` 或 `uncertain`。
- 现有 `step_test.go` 只覆盖“当前节点独存”和“后续节点独存”，没有覆盖“当前节点与后续节点并存”的安全边界。

修复边界：

1. 不要简单删除“当前节点存在即返回 false”的保护；单凭并行分支的后续节点不能证明本步这个处理人的写已生效。
2. 为本步动作补充同一执行人的正面完成事实。目标把已办行的节点 ID 改写时，应使用稳定的任务关联 ID/审核记录关联 ID 继续绑定本次处理，而不是只做节点等值比较。
3. 只有“本步执行人的完成事实已确认”且当前节点集合命中合法后续节点时，才允许把并存快照判为 `RereadAdvanced`；只有其他并行分支推进而本步没有完成事实时，继续保持未确认或未变。
4. 记录判定依据到 `step.log` 和 `BeforeFacts`，让页面能区分“本步已完成但还有会签人”和“只是别的并行分支推进”。

必须补的回归场景：

- `CurrentNodes=[本步]`、本人待办仍在、没有完成证据：`RereadUnchanged`。
- `CurrentNodes=[本步, 后续合法节点]`、本执行人已办/关联行已确认：`RereadAdvanced`。
- `CurrentNodes=[本步, 后续合法节点]`、只有其他并行分支完成：不能判本步成功。
- `CurrentNodes=[后续合法节点]`：继续保留现有跨节点成功判断。

### 3.3 P1：runtime 身份读取错误必须在写前阻塞

证据链：

- `internal/engine/step/executor.go:385-426` 的 `sessionFactsForRuntime` 在 `CurrentUserIdentity` 返回错误时静默忽略，继续返回会话缓存中的身份。
- `internal/engine/step/executor.go:209` 无条件把该结果装入 runtime 请求，调用方拿不到错误，无法区分“实时事实完整”和“实时读取失败”。
- `internal/adapter/target/protocol_matrix.go:665-674` 的 `CurrentUserIdentity` 合约写明任何身份读取失败都应由调用方阻塞；`nodeFormData` 已有同类阻塞口径，runtime 不能例外。

修复边界：

1. 把 `sessionFactsForRuntime` 改为返回 `(runtimeexec.SessionFacts, error)`；读取能力不存在、读取失败或必需身份字段缺失都返回可见的门禁错误。
2. `runViaRuntime` 在创建 runtime 会话和任何浏览器写请求前落 `GateBlocked`，不发送目标写请求，不登记成功能力。
3. 只允许继续使用已经明确核实的当前会话字段；不能用历史 plan 账号资料填补实时读取失败。

必须补的回归场景：

- 实时身份读取返回错误：`writeSent=false`、路径阻塞、目标写接口调用数为 0。
- 实时身份完整：runtime 收到的 `company/department/duty/user` 与当前读取结果一致。
- 测试假件不实现可选接口：按现行约定明确阻塞，不静默使用缓存冒充实时事实。

### 3.4 P2：终态事件需要带本步落账明细

证据链：

- `internal/engine/run/service.go:108-124` 的 `BackToRunning` 接收并写入 `Detail`。
- `internal/engine/run/service.go:135-138` 的 `Finish` 仍只接收 `label`，没有 `Detail`。
- `internal/engine/step/executor.go:2100` 附近的最后一步、确定失败和结果待确认分支都调用 `Finish`，所以事件流的终态事件没有节点接口明细。

修复边界：

1. 为终态事件增加独立 detail 参数或新增带 detail 的 Finish 方法，避免把已有用户可见 label 改成带内部信息的长文本。
2. 成功、确定失败、结果待确认都使用同一份 `settleEventDetail`；没有目标端点或没有可公开摘要时保持空值。
3. 仍只展示方法、路径、状态和目标返回摘要，不把 SID、密码和完整正文搬进事件流。

### 3.5 P2：普通 Go 写动作的“目标原始返回”显示范围需要补齐或明确

`RunNodePanel.vue:263-282` 只从 `attempt.runtimeDetail` 生成“目标平台原始返回”区，而 `RuntimeDetail` 的服务注释也明确非 runtime 步骤为空。普通写动作虽然已有 `Requests`、HTTP 状态和 `Reason`，但没有同形的原始 `message` 字段。

修复边界：

- 若产品要求所有动作统一显示原文：从已有 `WriteResponse`/错误链生成安全摘要并随尝试落库，前端统一消费；不增加重复目标请求。
- 若只要求 runtime 动作：把标题和说明改成“runtime 目标平台原始返回”，并在普通动作区明确沿用 `Reason` 与请求摘要，避免把空区误解为没有目标结果。

## 4. 不属于本次代码修复的 Run 169 问题

路径 9、10 的 `assignment_missing` 仍应保留为目标平台处理人未重新生成的事实阻断。工具没有可靠证据时不能用计划账号或候选人补发写请求；该问题需要目标平台或流程配置侧修复，不能在执行器里放宽门禁。

## 5. 验证记录

本次复审在干净分支 `执行引擎改造` 上执行：

- 通过：`go test ./internal/engine/step ./internal/engine/control ./internal/runtimeexec ./internal/adapter/target`
- 通过：`go build ./...`
- 通过：`go vet ./...`
- 通过：`web` 目录 `npm run typecheck`
- 通过：`go test` 已输出的全部已完成包，包括 `test/contracts`、`test/contracts/executor-service`、`test/contracts/runtimeexec` 等
- 未完成：`go test ./...` 在约 70 秒无新增输出后停止，不能把它记为全量测试通过；后续应定位具体长时间集成测试后再单独运行
- 未执行：真实浏览器和目标平台重跑；`executor-service` 没有 `npm test` 脚本，不能用该命令替代真实 runtime 验收

文档完成后应执行 `git diff --check`，并确认工作区只有本修复文档的改动。

## 6. 修复完成后的人工验收

1. 用 Run 169 同一计划重新跑路径 11：历史表单带 `flowInstanceId` 时，转发写后核对不再报字段丢失，且写载荷日志能证明该结构字段没有作为业务表单字段透传。
2. 用会签/并行真实流程复现“当前节点仍有其他待办、实例已出现后续节点”的窗口：本执行人已完成时继续推进；只有其他分支推进时保持阻塞或待确认。
3. 让当前会话身份读取接口失败：运行应在浏览器写请求前阻塞，并在详情中显示读取失败原因。
4. 查看最后一步成功、确定失败、结果待确认三种终态的事件流：均能看到节点、目标写接口和安全的结果摘要。
5. 分别打开 runtime 动作和普通 Go 动作详情，确认页面标题和原文显示范围与实际数据一致，不把空的 runtime 区域当成目标没有返回。

## 7. 当前状态

当前仍停在代码复审和修复方案阶段，未进入实现，也未进入人工验收。下一步只实施本文件列出的最小修复切片，完成对应自动验证后再次停在 `ready_for_manual`，不自动开始其他功能。
