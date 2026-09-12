# F-031 任务事实参数与实例日志归属修复

- 状态：awaiting_approval
- 计划确认时间：2026-09-12
- 产品依据：`docs/PRODUCT.md` 的计划与运行主线、真实目标状态与可观测要求
- 架构依据：`docs/ARCHITECTURE.md` 的目标适配边界、F-029 会话边界、F-030 性能观测边界
- 参考依据：`参考代码/rsh-flow-components/src/views/GroupApproveManage/Backlog/index.vue`、`Finished/index.vue`、`Submitted/index.vue`，以及目标 Java 的 `FlowJobTaskLinkProtocol`、`FlowJobTaskLinkServiceImpl`、`FlowJobTaskLinkRepository`、`FlowInstanceApiServiceImpl`

## 背景与结论

这不是目标接口本身慢，而是工具在目标接口上做了两类错误工作：

1. 对 `/web/flowJobTaskLink/list` 的实例筛选把 `flowInstanceId` 放进了 `data`。参考前端的精确查询把 `flowInstanceIdList` 放在请求顶层；参考 Java 仓储的 SQL 也只绑定 `f.flowInstanceIdList`。因此请求能返回 200，但实例条件被忽略，工具随后分页扫描全量任务表。
2. 运行时用 `NextNodeAuditors`（配置的下一节点选人）优先寻找当前任务。它代表将来提交/审批时要传的 `nextAuditorList`，不是当前节点已经产生的待办处理人。当前处理人的事实已经在发起人“已发”列表的 `currentAuditUserInfo` 中，目标后端会在这里完成角色、指定人员、岗位、移交、会签等计算并返回 `userList`。

日志也存在独立的归档问题：当前运行目录是 `logs/plans/<计划>/runs/<路径>/<运行号>`，无法从目录名直接认出目标平台中的发起实例。F-031 把目标实例名称和实例 ID 纳入运行日志目录、`meta.json` 和每行关联字段；计划名、路径名仍保留在元数据中，不丢失原有定位能力。

## 证据与现状定位

### 参数层级错误

| 位置 | 当前行为 | 目标行为 | 结论 |
| --- | --- | --- | --- |
| `internal/adapter/target/client.go:ListTaskSnapshotsForUser` | 已使用顶层 `flowInstanceIdList` | 保持 | 该处修复不能被后续代码覆盖 |
| `internal/adapter/target/client_fact_reads.go:FindDueFlow` | `data.flowInstanceId`，最多扫描 20 页 | 顶层 `flowInstanceIdList: [instanceID]`，只读 waiting_send | 必须修复 |
| `internal/adapter/target/client_fact_reads.go:FindDoneTaskOnNode` | `data.flowInstanceId`，只读第 1 页 | 复用统一任务快照查询，done 使用明确的 executor 视角 | 必须修复 |
| 参考前端 Backlog 精确查询 | 顶层 `flowInstanceIdList` | 同左 | 前端实际协议依据 |
| 参考 Java Repository | SQL `b1.id IN (:#{#f.flowInstanceIdList})` | 同左 | 后端实际过滤依据 |

“请求发送成功”只证明 HTTP/业务包络被接受，不证明过滤字段被目标使用；这正是本次慢和错的根因。

### 当前处理人与候选人的区别

| 名称 | 代码来源 | 含义 | F-031 用途 |
| --- | --- | --- | --- |
| `currentAuditUserInfo` / `InstanceFacts.CurrentHandlers` | 发起人会话调用 `/web/flowInstanceApi/list` | 目标已经计算出的当前节点、当前待办人员；含 `bizIds`、姓名、手机号 | 当前节点任务发现、实际登录账号解析 |
| `NextNodeAuditors` | `buildRunContext` 从路径配置读取的 `run_node_choose` | 下一次 submit/resubmit/approve 要发送给目标的选人参数 | 只构造 `nextAuditorList`，禁止作为当前待办发现首选 |
| `candidate` | 当前 `findCandidateTaskSnapshot` 遍历配置人员 | 工具侧配置候选，可能尚未成为目标待办 | F-031 不再用于证明当前处理人 |
| `jobTaskId` | 目标任务列表返回 | 当前操作必须携带的任务链接 ID | 找到真实当前处理人后再读取并发送 |

参考 `Submitted/index.vue` 直接展示 `currentAuditUserInfo.*.userList[].name`；参考后端 `queryCurrentPendingByFlowInstanceIds` 与 `queryCurrentProcessor` 先计算处理人，再由 `FlowNodeAuditConfigProxyApiServiceImpl` 批量补齐姓名、手机号。因此“候选人就是当前节点处理人”这个假设不成立。

## 不可破坏的边界

1. 不修改目标平台代码、数据库、权限、会话协议或业务状态。
2. 不改变节点顺序、动作顺序、分支选择、会签/竞签语义、写端点、写载荷和每次尝试最多一次真实写请求的约束。
3. 写后核验必须重新读取目标事实；任何任务列表缓存只在同一只读事实边界内有效，写请求发出后立即失效。
4. 同一账号仍串行，不通过并发登录或跳过账号锁换取速度；不同账号的现有并发能力不变。
5. 当前处理人只能由目标返回的 `currentAuditUserInfo` 或精确任务事实证明；没有事实时明确停步，不回退到配置候选人冒充当前处理人。
6. 既有日志文件不改写、不猜测实例名称。旧运行没有实例名称时显示“实例名称不可用”，新运行在实例事实确定后才使用实例名称目录。
7. 不增加数据库迁移。实例名称和实例 ID 写入运行日志 `meta.json` 与日志字段；运行业务表继续只保存不透明的 `MainInstanceRef`。

## 目标协议契约

所有 `/web/flowJobTaskLink/list` 调用先归入以下三类，再由一个共享构造函数生成载荷，禁止调用方自行拼接同名字段：

| 查询 | 顶层字段 | `data` 字段 | 处理人语义 |
| --- | --- | --- | --- |
| 当前实例待发 `waiting_send` | `flowInstanceIdList: [id]` | `taskStatus: waiting_send`、`useScope`、`auditWayList`、业务关联空值 | 发起人会话，不传 `queryUserId` |
| 当前实例待办 `pending` | `flowInstanceIdList: [id]`、必要时 `queryUserId` | `taskStatus: pending`、`useScope`、`auditWayList` | `queryUserId` 是目标任务查询视角，不是候选列表 |
| 当前实例已办 `done` | `flowInstanceIdList: [id]` | `taskStatus: done`、`data.executorId: actualUserID` | `executorId` 必须是要核对的真实处理账号 |

禁止出现 `data.flowInstanceId` 作为实例筛选条件。普通列表页仍可不带实例筛选，但精确实例读取必须使用顶层 `flowInstanceIdList`。`queryUserId`、`executorId` 的位置不能互换：参考 Java 服务对 pending 使用 `queryUserId`，对 done 使用 `data.executorId`。

## 实施任务

### T01 建立协议单一出口

- 在 `internal/adapter/target` 增加内部载荷构造/校验函数，参数包含实例 ID、任务状态、查询视角和已办执行人。
- 迁移 `ListTaskSnapshotsForUser`、`FindDueFlow`、`FindDoneTaskOnNode` 到该出口；保留现有分页上限、结果去重和目标错误分类。
- 全库搜索 `/web/flowJobTaskLink/list` 和 `data.flowInstanceId`。除目标确实要求实例字段位于 `data` 的其他端点外，本端点不得残留该字段。
- 在代码注释中写清“顶层 `flowInstanceIdList` 是目标协议字段，`data.flowInstanceId` 会被忽略”，防止再次按语义猜字段位置。

### T02 修复 waiting_send 与 done 精确读取

- `FindDueFlow` 改为顶层实例列表过滤，保留并行任务、多代理入口和表单代理收集；过滤后仍检查返回行的实例 ID，防止目标响应异常串入其他实例。
- `FindDoneTaskOnNode` 复用统一快照读取，不再固定只看第一页；按实际核对账号传入 `executorId`，再按节点过滤。
- 对账调用点必须传入“当前真实操作账号”，不能默认使用计划账号或配置候选人；若接口只需判断实例上是否有已办，则明确记录视角为空的产品语义。
- 记录每次精确查询的请求次数和命中行数，便于确认从全量扫描降为实例范围查询。

### T03 以 currentAuditUserInfo 作为当前处理人唯一首选事实

- 在 `resolveTaskSnapshotForStep` 中先读取/复用 `InstanceFacts.CurrentHandlers`，按目标真实 `NodeID` 选当前节点处理人；只有事实中无处理人时才返回可解释阻塞。
- 移除 `findCandidateTaskSnapshot` 作为当前任务发现路径，不再为 `NextNodeAuditors` 中的每个配置人员逐一登录、逐一查任务。
- `MatchHandlerAccounts` 继续按 `bizId` 优先、姓名/手机号兜底解析真实目标用户；解析结果按用户 ID 去重后再做任务查询。
- 任务查询顺序固定为：当前处理人事实 -> 真实用户 ID/会话 -> 精确实例 pending 任务 -> 精确节点和唯一 `jobTaskId`。禁止“候选人命中任务”覆盖当前处理人事实。
- `NextNodeAuditors` 只留在 `BuildSubmitBody`、`BuildAuditBody` 和对应动作请求构造中，用于下一节点选人；不得写入 `CurrentHandlers`，不得用来决定当前审批人。
- 会签仍按目标返回的当前处理人集合逐人推进；目标返回人员减少时，以新事实为准，不能重放已完成人员。

### T04 收敛读取边界与缓存

- 预览/门禁/写前准备在同一事实版本内共享“实例事实、当前处理人、精确任务快照”；缓存键至少包含会话账号、实例 ID、任务状态、查询视角和事实版本。
- 人工放行、会话代次变化、目标写请求发出、重试重新进入写前准备时全部失效；写后核验不读取写前缓存。
- 取消任何按配置候选人全量扫描的隐式回退。若目标响应缺少 `currentAuditUserInfo`，日志记录缺失字段与实例 ID，流程停在当前步骤。
- 性能验收以请求次数、实际目标传输耗时和本地等待分别统计，不把“HTTP 200”当成过滤正确的证据。

### T05 实例名称绑定与日志目录

#### 数据来源

- 新发起请求的 `SubmitFlowInstanceRequest.Name` 是工具发送的实例显示名；提交成功得到实例 ID 后，再用发起人会话对 `/web/flowInstanceApi/list` 做一次 `ids: [instanceID]` 精确读取，优先取目标返回的 `name`，其次取 `formName`，用于校正最终目录名。
- 已有已发/待发实例在 `runLogScope` 或首次运行事实读取时用同一精确列表响应取 `name`/`formName`；不得从 URL、计划名或路径名猜作实例名。
- 目标名称为空时使用固定的 `实例名称不可用__instance-<id>`，并在 `meta.json` 记录 `instanceNameAvailable=false`；不能再退回“路径 1”冒充实例名称。

#### 目录与元数据

- 保留现有计划和路径层级，运行目录改为：

  `logs/plans/<计划名>__plan-<计划ID>/runs/<路径名>__path-<路径ID>/<实例名>__instance-<实例ID>__run-<运行号>/`

- `logging.Scope` 增加 `InstanceID`、`InstanceName`、`InstanceNameAvailable`；`Merge`、`Fields`、`BucketDir`、`RunFolder` 和所有控制/恢复/网络日志写入点必须使用同一作用域。
- `meta.json` 增加 `instanceId`、`instanceName`、`instanceNameAvailable`，同时保留 `planId`、`planName`、`executionPathId`、`executionPathName`、`runId`、`startedAt`。
- 实例 ID/name 尚未可知时，先写入 `pending-instance__run-<运行号>`，提交成功并完成精确读取后，在路由器锁内把整个目录原子改名为最终目录并更新 writer 映射；日志文件内容和行号不重写。若实例名称读取失败，不改名为猜测值，使用不可用占位目录并记录原因。
- 服务重启后以 `MainInstanceRef` 在路径运行目录中定位已有 `__instance-<id>__run-<runNo>` 目录；找不到时再创建不可用占位目录，不能创建第二个同运行号目录。
- 旧目录不迁移、不重命名；详情页打开旧运行时从旧 `meta.json` 和原路径展示“实例名称不可用”，不补造历史名称。

#### 作用域覆盖点

- `internal/service/run_orchestration.go` 的 `runLogScope`、`withRunScope`、控制日志和恢复日志。
- 执行器提交成功落库 `MainInstanceRef` 后的日志重绑定点；网络传输层继承 context 作用域，确保 `network.log`、`curl.log` 和 `step.log` 指向同一实例目录。
- 调度器、重试/重新核验、后台收尾和运行详情读取不得重新按计划/路径生成旧目录名。

### T06 运行详情与排障信息

- 运行详情返回实例名称、实例 ID、日志目录相对位置；动作详情中的“日志位置”直接指向新的实例目录。
- 请求明细仍只返回安全摘要，不返回 SID、密码、完整请求/响应正文；完整正文继续留在 `curl.log`。
- 错误文案明确区分：实例过滤未生效、当前处理人事实缺失、真实账号无法解析、任务不存在、实例名称暂不可读。禁止统一显示“请求发送成功”掩盖读取语义错误。

## 验证与测试

测试文件按类别放在根目录 `test/`，只验证本切片：

- **协议单元测试**：逐类断言 `flowInstanceIdList` 在顶层；断言不存在 `data.flowInstanceId`；pending 使用 `queryUserId`，done 使用 `data.executorId`；空实例 ID被拒绝。
- **目标响应解析测试**：currentAuditUserInfo 含多个节点、会签人员递减、bizId/姓名/手机号匹配、空处理人和目标返回其他实例时均有明确结果。
- **执行器行为测试**：配置了下一节点候选人但当前事实属于另一人时，只查询事实中的真实处理人；没有 currentAuditUserInfo 时停步；写后核验不命中写前缓存；每次尝试写次数不增加。
- **日志路由测试**：实例名含中文、空格、斜杠、超长文本时安全清洗；提交前 pending 目录能原子绑定到实例目录；`meta.json` 字段完整；旧目录保持不变；重启按实例 ID复用同一目录。
- **结构契约测试**：全库不残留本端点的 `data.flowInstanceId`；`NextNodeAuditors` 仅出现在下一节点写载荷路径；所有运行日志 writer 都从带实例作用域的函数获取目录。
- **定向执行验证**：使用与用户反馈相同的流程和发起账号，比较任务列表请求次数、单次请求返回范围、当前处理人、目标已发记录和总耗时；不启动浏览器，由用户手工页面验证。

## 完成标准

- [ ] 所有精确任务查询的实例过滤都使用顶层 `flowInstanceIdList`；请求日志能证明没有再扫描无关实例任务。
- [ ] pending/done/waiting_send 的 `queryUserId`、`executorId`、`taskStatus` 位置和语义与参考前后端一致。
- [ ] 当前节点处理人只由发起人已发列表 `currentAuditUserInfo` 和精确任务事实确定；下一节点候选人不再被当作当前处理人。
- [ ] 同一实例、同一事实边界内不重复扫描全量候选；写前缓存不会进入写后核验；节点顺序、动作顺序、写请求次数和运行结论不变。
- [ ] 运行日志目录能直接看到实例名称和实例 ID，`meta.json` 与每行日志字段能交叉确认计划、路径、运行和实例归属。
- [ ] 新实例名称读取失败时使用明确不可用占位，不使用计划名/路径名伪装；旧日志不迁移、不改写。
- [ ] 协议、处理人、缓存失效、日志重绑定和敏感信息边界测试全部实际执行且无跳过。
- [ ] 运行详情和日志中的错误能区分“请求已发送”与“实例/任务事实已正确读取”；不把 HTTP 成功误报为业务动作生效。
- [ ] 实施完成后停在 `ready_for_manual`，等待用户在目标平台发起人已发列表和本地日志目录中人工核对；不得自动进入下一切片。

## 人工验收

1. 用发起人骆蒙恩启动一条新流程，登录目标平台的骆蒙恩账号，确认“已发”列表出现同名实例，并确认日志运行目录使用该实例名称而不是“路径 1/运行号”。
2. 打开该实例的 `currentAuditUserInfo`，把目标显示的当前处理人和节点与动作详情中的实际处理人、待办账号、`jobTaskId` 对照；故意配置一个不是当前处理人的下一节点候选人，确认工具不会登录候选人账号找当前任务。
3. 检查 `network.log`，确认任务列表请求包含顶层 `flowInstanceIdList`，pending 使用 `queryUserId`，done 使用真实 `executorId`，响应只含目标实例。
4. 对比修复前后的同一样本：任务列表请求数和本地等待明显下降，接口真实耗时仍单独显示；不能以“请求发送成功”替代实例已发记录核对。
5. 在提交前、提交成功、实例名称读取失败、服务重启后分别查看日志目录，确认不会丢日志、重复建目录或用错误名称归档。

## 状态记录

- 2026-09-12：根据用户要求建立 F-031 方案。源码和参考代码已完成勘定：确认 `flowInstanceIdList` 必须在协议顶层；确认当前处理人来自发起人已发列表的 `currentAuditUserInfo`，`NextNodeAuditors` 只是下一节点选人；确认当前日志目录只有计划/路径/运行号，没有实例名称。文档停在 `awaiting_approval`，未修改执行代码、数据库或目标平台。
