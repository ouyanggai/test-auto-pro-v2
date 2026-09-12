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

日志也存在独立的归档问题：当前运行目录是 `logs/plans/<计划>/runs/<路径>/<运行号>`，和页面的“运行记录 -> 路径运行 -> 运行详情”层级相反，计划重复运行时还要靠猜运行号找目录。F-031 改成与页面一一对应的稳定目录：从 `logs/runs` 进入，运行列表的 `runId/runNo` 对应运行目录，路径页的 `pathRunId/pathId/pathName` 对应路径目录；目标实例名称只作为该路径运行的业务信息写入 `meta.json`、日志字段和详情页，不再用它作为唯一目录键。这样实例名称变更、读取延迟或同名实例都不会造成日志搬迁和误归档。

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
6. 既有日志文件不改写、不猜测实例名称。旧运行没有实例名称时显示“实例名称不可用”；新运行的物理目录不等待目标实例名称，也不因名称读取延迟而搬迁。
7. 不增加数据库迁移。实例名称和实例 ID写入运行日志 `meta.json`、日志字段和详情 DTO；运行业务表继续只保存不透明的 `MainInstanceRef`。

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

### T05 页面身份与日志目录一一对应

#### 页面身份映射

- 运行列表页的唯一定位是 `runId`，用户看到的 `运行 #runNo` 是同一运行的可读标签；目录必须同时保留二者，不能只用计划内运行号。
- 运行路径页的唯一定位是 `pathRunId`，`pathId` 和 `pathName` 是可读辅助信息；同一运行内即使出现同名路径，也不能互相覆盖。
- 新发起或已有实例的目标名称仍通过发起人会话对 `/web/flowInstanceApi/list` 做 `ids: [instanceID]` 精确读取，优先 `name`，其次 `formName`；读取结果只更新该路径运行的元数据和页面摘要，不参与目录寻址。
- 目标名称为空或读取失败时记录 `instanceNameAvailable=false` 和原因；不能用计划名、路径名或候选处理人名称冒充实例名。

#### 目录与元数据

- 目录严格镜像页面导航层级，改为：

  `logs/runs/运行_<运行号>__<计划名>__run-<运行ID>/paths/<路径名>__path-run-<路径运行ID>__path-<路径ID>/`

- `logging.Scope` 增加 `InstanceID`、`InstanceName`、`InstanceNameAvailable`，并保留已有 `PlanID/PlanName`、`RunID`、`RunSeq`、`PathRunID`；运行 `BucketDir` 按“运行 -> 路径”生成目录，配置阶段仍使用原有 `logs/plans/<计划>/configuration`。目录的稳定键只使用本工具页面已有的 ID。
- `Merge`、`Fields`、`BucketDir`、`RunFolder`、新增 `PathRunFolder` 和所有控制/恢复/网络日志写入点必须使用同一作用域。任何日志路径都能由页面 URL `/runs/<runId>/paths/<pathRunId>` 反查。
- `meta.json` 增加 `runNo`、`pathRunId`、`pathId`、`pathName`、`instanceId`、`instanceName`、`instanceNameAvailable`，同时保留 `planId`、`planName`、`executionPathId`、`executionPathName`、`runId`、`startedAt`。目录与页面显示的运行号、计划名、路径名保持同一文字来源；ID 用于保证唯一。
- 运行开始即按 `runId/pathRunId` 建立最终目录；不使用 `pending-instance` 目录，不做提交成功后的目录改名，不需要路由器搬迁 writer，也不会因目标名称读取慢而延长运行。
- 服务重启或详情读取时按数据库中的 `RunID`、`PathRunID` 和已落账 `LogPath` 复用同一目录；禁止仅按“计划 + 运行号”猜路径，禁止为同一 `pathRunId` 创建第二个目录。运行详情的日志链接直接指向该路径目录，不要求用户先进入计划目录再人工筛选。
- 旧目录不迁移、不重命名；历史日志仍按旧路径可读。详情页对旧日志显示“实例名称不可用”或读取到的旧元数据，不补造名称。

#### 作用域覆盖点

- `internal/service/run_orchestration.go` 的 `runLogScope`、`withRunScope`、控制日志和恢复日志。
- 执行器提交成功落库 `MainInstanceRef` 后只更新当前作用域的实例字段；网络传输层继承 context 作用域，确保 `network.log`、`curl.log` 和 `step.log` 指向同一路径运行目录。
- 调度器、重试/重新核验、后台收尾和运行详情读取必须使用同一 `runId/pathRunId` 目录，不得重新按实例名称、计划名或路径名生成另一套目录；只有目录中的可读标签使用计划名、路径名。

### T06 运行详情与排障信息

- 运行详情返回实例名称、实例 ID、日志目录相对位置；动作详情中的“日志位置”直接指向新的实例目录。
- 请求明细仍只返回安全摘要，不返回 SID、密码、完整请求/响应正文；完整正文继续留在 `curl.log`。
- 错误文案明确区分：实例过滤未生效、当前处理人事实缺失、真实账号无法解析、任务不存在、实例名称暂不可读。禁止统一显示“请求发送成功”掩盖读取语义错误。

## 验证与测试

测试文件按类别放在根目录 `test/`，只验证本切片：

- **协议单元测试**：逐类断言 `flowInstanceIdList` 在顶层；断言不存在 `data.flowInstanceId`；pending 使用 `queryUserId`，done 使用 `data.executorId`；空实例 ID被拒绝。
- **目标响应解析测试**：currentAuditUserInfo 含多个节点、会签人员递减、bizId/姓名/手机号匹配、空处理人和目标返回其他实例时均有明确结果。
- **执行器行为测试**：配置了下一节点候选人但当前事实属于另一人时，只查询事实中的真实处理人；没有 currentAuditUserInfo 时停步；写后核验不命中写前缓存；每次尝试写次数不增加。
- **日志路由测试**：运行列表中的一行能直接映射到 `logs/runs/运行_<runNo>__<planName>__run-<runId>`；同一计划连续运行 1、2、3 次分别使用各自 `runId`，不会覆盖；路径页每行能直接映射到 `paths/<pathName>__path-run-<pathRunId>__path-<pathId>`；实例名含中文、空格、斜杠、超长文本时只清洗元数据字段；`meta.json` 字段完整；重启按 `runId/pathRunId` 复用同一目录。
- **结构契约测试**：全库不残留本端点的 `data.flowInstanceId`；`NextNodeAuditors` 仅出现在下一节点写载荷路径；所有运行日志 writer 都从带实例作用域的函数获取目录。
- **定向执行验证**：使用与用户反馈相同的流程和发起账号，比较任务列表请求次数、单次请求返回范围、当前处理人、目标已发记录和总耗时；不启动浏览器，由用户手工页面验证。

## 完成标准

- [ ] 所有精确任务查询的实例过滤都使用顶层 `flowInstanceIdList`；请求日志能证明没有再扫描无关实例任务。
- [ ] pending/done/waiting_send 的 `queryUserId`、`executorId`、`taskStatus` 位置和语义与参考前后端一致。
- [ ] 当前节点处理人只由发起人已发列表 `currentAuditUserInfo` 和精确任务事实确定；下一节点候选人不再被当作当前处理人。
- [ ] 同一实例、同一事实边界内不重复扫描全量候选；写前缓存不会进入写后核验；节点顺序、动作顺序、写请求次数和运行结论不变。
- [ ] 运行日志目录严格对应页面的“运行记录 -> 路径运行”层级：`logs/runs/<运行记录>/paths/<路径运行>`；同一计划重复运行不会覆盖，`/runs/<runId>/paths/<pathRunId>` 可以直接反查目录。
- [ ] `meta.json` 与每行日志字段能交叉确认计划、运行号/运行 ID、路径/路径运行 ID 和目标实例；实例名称读取失败只影响名称展示，不改变目录，不使用计划名/路径名伪装。
- [ ] 运行详情可以直接展示日志相对目录和目标实例名称；用户不需要遍历全盘或按时间猜目录。
- [ ] 协议、处理人、缓存失效、日志重绑定和敏感信息边界测试全部实际执行且无跳过。
- [ ] 运行详情和日志中的错误能区分“请求已发送”与“实例/任务事实已正确读取”；不把 HTTP 成功误报为业务动作生效。
- [ ] 实施完成后停在 `ready_for_manual`，等待用户在目标平台发起人已发列表和本地日志目录中人工核对；不得自动进入下一切片。

## 人工验收

1. 用同一个计划连续启动两次，打开“运行记录”列表，按每行的“运行 #”进入路径页；确认每一行都能在 `logs/runs` 下找到同名运行目录，`runId` 对应唯一目录，第二次运行不会进入第一次目录。
2. 打开某次运行的路径页，按页面显示的路径名称和路径运行 URL 进入详情；确认日志目录的 `pathRunId` 与当前 URL 一致，多个同名路径也不会覆盖。
3. 在详情页核对目标实例名称、实例 ID与 `meta.json`，确认名称只作为业务信息展示，日志目录仍以页面运行/路径运行身份定位。
4. 打开该实例的 `currentAuditUserInfo`，把目标显示的当前处理人和节点与动作详情中的实际处理人、待办账号、`jobTaskId` 对照；故意配置一个不是当前处理人的下一节点候选人，确认工具不会登录候选人账号找当前任务。
5. 检查 `network.log`，确认任务列表请求包含顶层 `flowInstanceIdList`，pending 使用 `queryUserId`，done 使用真实 `executorId`，响应只含目标实例。
6. 对比修复前后的同一样本：任务列表请求数和本地等待明显下降，接口真实耗时仍单独显示；不能以“请求发送成功”替代实例已发记录核对。
7. 服务重启后从同一个页面详情再次打开日志，确认不会丢日志、重复建目录或按实例名称生成另一套目录。

## 状态记录

- 2026-09-12：根据用户反馈再次调整日志方案。运行日志从 `logs/runs` 直接进入，严格镜像页面的“运行记录 -> 路径运行”层级；目录使用 `runId/pathRunId` 稳定定位，显示 `runNo/planName/pathName` 作为页面对应标签；实例名称只进详情和 `meta.json`，不触发目录重命名。源码和参考代码的协议、处理人结论不变。文档停在 `awaiting_approval`，未修改执行代码、数据库或目标平台。
