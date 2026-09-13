# F-035 目标请求协议一致性、真实处理人和有效表单数据闭环

- 状态：awaiting_approval
- 产品依据：`docs/PRODUCT.md` 的目标平台事实、运行状态、表单数据和安全写入原则
- 架构依据：`docs/ARCHITECTURE.md` 的目标适配层、执行器门禁、回放存储和 form-runtime 边界
- 本切片性质：修复现有目标请求、身份数据、处理人核验和表单数据链路；不修改目标平台源码、权限、流程配置，不新增目标写接口，不新增数据库表
- 排查样本：计划“欧阳改测试1005”（plan 29），运行号 7（run 98），路径 12（path 4221）；运行号 6 同样复现

## 单一用户结果

工具发出的每一个目标请求都必须按目标平台 GroupApproveManage 前端和对应 Java 服务的实际构造方式生成。表单流程使用目标返回的 `formProxyId`，无表单流程使用 `flowProxyId`；当前登录人的完整身份、显式选人、`nextAuditorList`、`batchCode`、`projectId`、业务关联和空值形状都按具体目标接口逐项对齐。目标成功创建实例后，工具只使用目标返回的真实节点处理人继续执行；目标没有产生处理人时明确阻塞，不用计划账号或候选人代替。

## 完成标准

- [ ] 建立本切片的目标接口协议矩阵，覆盖运行实际使用的全部读写接口；每个接口记录 HTTP 方法、路径、查询参数、请求头、请求体层级、字段名、字段类型、空值/空数组/省略规则、响应成功判据和证据位置。
- [ ] 工具请求的 `platformCode`、`sid`、`Accept`、`Content-Type`、`Origin`、`Referer`、请求体 `sid` 与目标页面请求保持同一语义；只允许环境地址差异，不得出现路径族或参数位置差异。
- [ ] FormMaking 发起/重提请求按目标页面发送唯一 `formProxyId`；NoFormFlow 发起请求按目标页面发送 `flowProxyId`；禁止把发布流程代理 ID 当作表单代理 ID，也禁止无依据同时发送两个代理字段。
- [ ] 目标页面实际携带的 `batchCode` 必须按接口和动作原样携带，生成时机、作用范围和字段位置与页面一致；不得把 `batchCode` 当作工具幂等键、重试依据或跨接口通用字段。若与 F-014 当前“统一禁止”规则冲突，实施前必须按目标源码重新确认并同步语义判定器，不能静默保留旧禁令或把它加到所有请求。
- [ ] `nextAuditorList` 的存在性和数组形状按目标页面保持一致；每个显式选人项完整携带目标要求的 `bizId`、`name`、`auditDetailType`、`nodeProxyId`。自动扩展属性节点不伪造处理人，空数组也不得被解释为已解析出人员。
- [ ] 发起人身份与表单身份一致：`global_user_basic_information`、`myCompanyName`、`myDepName`、`myDutyName`、`myUserName` 及全部 `__condition`、`__formPersonId` 从当前计划账号运行时会话构造，不能使用历史账号值；岗位信息缺失时发起前阻塞。
- [ ] `data.name`、`companyId`、`projectId`、`flowInstanceBizRelevanceList`、`formDataMongoVo.data` 的来源和格式与目标页面一致；日期、数字、空字符串、虚拟字段和人员字段不被工具自行改写。
- [ ] 审批、暂存、重提、回退、取回、撤回、移交、加签、转发、关注/取消关注、催办等实际使用的读写接口全部通过同一份协议矩阵和请求构造器；不得只修复 submit 而留下其他接口的字段位置、端点或身份差异。
- [ ] 当前待办任务的 `jobTaskId`、`batchNo`、`flowNodeProxyId`、`flowProxyId` 和真实处理人只能来自目标最新任务快照；缺失时阻塞，不能回退计划账号、发起人或候选人。
- [ ] 目标节点已到达但处理人尚未生成时进入“正在生成处理人”并执行有界轮询；等待结束仍无处理人和待办时标记“阻塞”，不显示“已处理”、不重复发送写请求。
- [ ] 回放后的表单值经过运行时下拉选项绑定、最终回读和保存后，才进入最终 `ready`；页面、路径配置和运行发起请求始终使用同一份最终有效表单数据。
- [ ] 请求日志能按 trace、接口、动作和尝试列出协议摘要、身份摘要、代理 ID、处理人来源、批次号是否携带、表单数据版本和目标事实，不记录 SID、密码或完整敏感表单正文。
- [ ] 不改变一次写、写后重读、账号串行、运行快照、动作顺序、F-034 动作矩阵和 F-033 节点过渡语义。

## 根因和证据

### A. 当前实例“成功创建但无处理人”首先是请求载荷不一致

运行 7 / 路径 12 的实际请求见 [curl.log](/Volumes/oygsky/AIstudy/test-auto-pro-v2/logs/runs/运行_7__欧阳改测试1005__run-98/paths/路径%2012__path-run-163__path-4221/curl.log:2)：

- 工具只发送 `data.flowProxyId=73eaa9...`，没有按 FormMaking 页面发送 `data.formProxyId=5222ef...`。
- 工具没有发送页面实际存在的 `batchCode`、空的 `nextAuditorList` 和 `projectId` 字段。
- 工具发送的 `data.name`、日期格式和表单值格式也不是目标页面本次手工发起的最终值。
- 工具表单身份是张泽华（用户 ID `0aff...`、综合管理部、行政专员），但计划账号和手工成功请求是骆蒙恩（用户 ID `08e604...`、建设运营部、工程经理）。

目标页面在 [FlowDialog.vue](/Volumes/oygsky/AIstudy/test-auto-pro-v2/参考代码/rsh-flow-components/src/views/GroupApproveManage/Submitted/components/FlowDialog.vue:757) 按“有表单使用 `formProxyId`、无表单使用 `flowProxyId`”构造请求，并在 [FlowDialog.vue](/Volumes/oygsky/AIstudy/test-auto-pro-v2/参考代码/rsh-flow-components/src/views/GroupApproveManage/Submitted/components/FlowDialog.vue:667) 用当前登录会话填充完整 `global_user_basic_information`。目标 Java 提交入口随后按该协议校验代理、权限和起始节点，见 [FlowSubmitServiceImpl.java](/Volumes/oygsky/AIstudy/test-auto-pro-v2/参考代码/java-serve/rsh-cloud-workflow-center/src/main/java/com/rsh/cloud/workflow/center/service/impl/FlowSubmitServiceImpl.java:86)。

目标返回 HTTP 200、`isSuccess=true` 只证明实例被创建。实际实例仍停在“行政综合部-考勤管理”，`currentAuditUserInfo=null`，待办为 0，见 [step.log](/Volumes/oygsky/AIstudy/test-auto-pro-v2/logs/runs/运行_7__欧阳改测试1005__run-98/paths/路径%2012__path-run-163__path-4221/step.log:6) 和 [step.log](/Volumes/oygsky/AIstudy/test-auto-pro-v2/logs/runs/运行_7__欧阳改测试1005__run-98/paths/路径%2012__path-run-163__path-4221/step.log:13)。因此“接口请求成功”不能等同于“目标处理人已成功生成”。

### B. 表单身份错位会直接影响扩展属性处理人解析

路径 12 的第二节点是目标扩展属性规则“集团考勤管理员”。工具把张泽华的部门、岗位和人员字段发给骆蒙恩会话，目标据此计算不到预期处理人，最终出现实例有当前节点但没有 `currentAuditUserInfo` 和待办。运行 6 与运行 7 均发送同一组张泽华身份并复现，说明不是目标平台偶发延迟。

当前代码虽然有身份替换，但读取和保存时忽略了 `currentUserIdentity` 错误，见 [path_data_workspace.go](/Volumes/oygsky/AIstudy/test-auto-pro-v2/internal/service/path_data_workspace.go:113) 和 [path_data_workspace.go](/Volumes/oygsky/AIstudy/test-auto-pro-v2/internal/service/path_data_workspace.go:414)；运行上下文提交时又没有最后一道身份一致性校验。必须在真正写请求前重新构造并校验身份，不能继续使用历史 `effective_form_data`。

### C. `formProxyId` 在读链路中已经存在，但被运行上下文丢弃

目标模板读取会解析 `formTemplateList` 并生成 `PathConfigurationSnapshot.Forms`，但 `buildRunContext` 只保存计划的 `FlowProxyID`，见 [run_orchestration.go](/Volumes/oygsky/AIstudy/test-auto-pro-v2/internal/service/run_orchestration.go:521)。提交门禁也只填充 `FlowProxyID`，见 [gate.go](/Volumes/oygsky/AIstudy/test-auto-pro-v2/internal/engine/step/gate.go:141)。因此这是工具内部数据传递丢失，不是目标平台没有返回表单代理 ID。

### D. 其他接口也存在协议漂移风险

当前 `BuildActionBody`、`BuildSubmitBody` 和任务查询虽然已经集中构造，但仍有“非空才发送”与目标页面“固定发送空数组/字段”的差异风险；动作端点同时存在 `/web/flowInstanceApi/*` 和 `/flowInstanceApi/audit` 两个路由族，审批字段、表单容器、业务关联、`tracking`、`batchNo` 和 `jobTaskId` 的位置不能凭通用模型猜测。任务查询还必须保持实例过滤在顶层 `flowInstanceIdList`、待办视角使用顶层 `queryUserId`、已办执行人使用 `data.executorId`，参考 [task_query.go](/Volumes/oygsky/AIstudy/test-auto-pro-v2/internal/adapter/target/task_query.go:11)。

## 目标协议基线

协议基线不是“所有请求都添加同样字段”，而是“同一个目标页面动作在同一个场景下产生同样的请求”。实施时必须为每个接口保留一份可读矩阵，至少覆盖以下集合：

| 场景 | 目标接口 | 必须核对的重点 |
| --- | --- | --- |
| 新建表单、保存草稿 | `/web/flowInstanceApi/submit` | `formProxyId/flowProxyId` 二选一、`status=draft`、完整表单、业务关联、`nextAuditorList`、`batchCode`、`projectId`、公司 ID |
| 驳回/撤回后重新发起 | `/web/flowInstanceApi/reSubmit` | 实例 ID、实时 `formProxyId`、完整表单、业务关联、分支入口、显式人员、批次号和页面同源字段 |
| 当前节点暂存 | `/web/flowInstanceApi/storageFormData` | 实例 ID、当前目标节点 ID、`auditRecord.executeDesc`、完整表单；不能误用实例提交协议 |
| 同意/不同意 | `/flowInstanceApi/audit` | `jobTaskId`、`flowProxyId`、`auditRecord.auditStatus/executeDesc`、完整表单、业务关联、下一节点人员、`tracking` |
| 移交/加签 | `/web/flowInstanceApi/approverAppend`、`/web/flowInstanceApi/updateFlowProxy` | `batchNo`、任务 ID、当前节点 ID、用户 ID 列表或完整代理树；不能发送简化节点模型 |
| 回退/取回/撤回 | `/web/flowInstanceApi/rollBackThePreviousLevel`、`retrieveProcess`、`revocation` | 目标要求的实例 ID、任务 ID、撤回说明和当前任务事实；不能从旧运行快照拼装 |
| 转发、关注/取消关注、催办 | `/web/flowInstanceApi/transpond`、`flowTracking`、`/web/urgeHandleRecord/sendUrgeMessage` | 顶层字段与 `data` 层级、接收人、表单容器、业务关联、关注布尔值、催办实例 ID |
| 实例、任务和表单读取 | `/web/flowInstanceApi/list`、`/web/flowJobTaskLink/list`、`/web/flowAuditRecord/list`、`getCurrentFromData`、`queryStorageFormData` | 实例过滤位置、用户视角、节点/任务 ID、批次号、当前处理人和完整表单数据来源 |

`batchCode` 和 `nextAuditorList` 的规则必须以具体目标页面是否实际发送为准；不能因为某个接口需要就扩散到全部接口，也不能因为某个旧语义清单曾禁止就忽略手工请求中的真实字段。F-035 实施时必须同步更新 `docs/TARGET_SEMANTICS.md`、F-014 的判定约束和相关契约测试，明确哪些接口允许、要求或禁止这些字段。

## 实施任务

### T01：建立目标请求协议矩阵和差异基线

- 仅查看 `参考代码/rsh-cloud-invest-power-system` 的 `GroupApproveManage` 及其直接引用公共组件；同时读取对应 Java Controller、Service、Protocol/VO 和已存在的 `docs/TARGET_SEMANTICS.md`。
- 为每个实际使用接口登记方法、路径、平台码、SID 位置、请求头、顶层字段、`data` 字段、响应字段、成功判据和证据强度。
- 从目标前端真实构造逻辑抽取“字段存在/省略/空数组/空对象/null”的规则，不把 Go 结构体的 `omitempty` 当成目标协议规则。
- 以本次人工 curl 和运行 7 curl 做逐字段差异报告，至少覆盖：代理 ID、身份、表单数据、名称、日期、`batchCode`、`nextAuditorList`、`projectId`、业务关联和请求头。
- 差异必须分类为“目标要求的协议差异”“场景允许的业务值差异”“工具缺陷”，不能只比较 JSON 字符串顺序。

### T02：统一目标请求信封和字段存在性策略

- 保持 `internal/adapter/target` 是唯一目标请求出口；所有读写请求通过同一套请求元数据生成 query、SID、头部和日志 trace。
- 增加按接口/动作声明字段存在性的协议构造规则：必须发送、条件发送、必须为空数组、必须为空对象、禁止发送分别表达，禁止用“非空才发送”一刀切。
- `Origin`、`Referer`、路径前缀和 `platformCode` 只按目标运行环境配置生成；不得把人工 curl 的地址硬编码进代码。
- `batchCode` 只在目标页面该动作实际发送时生成并发送，且与页面的生命周期一致；它不能作为工具重试键，写请求仍只能一次，响应丢失仍先对账。
- 将 `docs/TARGET_SEMANTICS.md` 和 F-014 中的统一 `batchCode` 禁令改为按端点、动作和目标部署证据的明确矩阵；未完成该同步前禁止实现“看起来一致”的半套修复。

### T03：修复表单/无表单代理 ID和发起动作

- 在运行上下文保留 `FormProxyID`，从新流程模板的唯一 `Forms` 项取得；FormMaking 发起/草稿只发送 `formProxyId`，NoFormFlow 只发送 `flowProxyId`。
- 重提使用实例事实中的实时代理 ID和表单代理 ID，严格按目标 `reSubmit` 前端形状构造，不把新发起协议复用到重提。
- `submit`、草稿和 `reSubmit` 固定生成目标页面会发送的 `nextAuditorList`、`batchCode`、`projectId` 等字段；自动解析节点只发送空数组或目标页面要求的形状，不伪造人员。
- `data.name` 由与目标页面相同的最终表单值和命名规则生成；日志目录名称仍沿用工具自己的页面映射，不用实例名反向影响协议。

### T04：修复当前账号、表单身份和显式处理人

- 扩展运行时账号事实，至少包含用户 ID/姓名、公司 ID/名称、部门 ID/名称、岗位 ID/名称；岗位无法从当前目标会话可靠取得时，提交前阻塞。
- 提交和重提前覆盖所有目标登录人字段及关联键，检查用户、部门、岗位之间相互一致；身份读取错误不得被忽略。
- 显式 `run_node_choose`、并行和分支选人严格按目标页面的 `nextAuditorList` 字段逐项发送；固定扩展属性节点不使用候选人、计划账号或发起人替代目标自动解析。
- 任务动作必须以目标任务快照的真实处理人账号发出，任务 ID、批次号和节点 ID全部来自同一次新鲜读取；发现人员缺失、多个任务或人员不匹配时阻塞。

### T05：逐接口修复写请求构造

- 对 `BuildSubmitBody`、`BuildActionBody` 和审批请求分别实现目标页面对应的协议形状，不以一个通用 `data` 模型强行合并所有动作。
- 逐项核对 `storageFormData`、`audit`、`reSubmit`、`approverAppend`、`updateFlowProxy`、回退、取回、撤回、转发、关注和催办的端点、路由族、字段层级、表单容器和业务关联。
- `audit` 的 `jobTaskId`、`flowProxyId`、`auditRecord`、`formDataMongoVo`、`nextAuditorList` 和 `tracking` 必须与目标审批页一致；同意与不同意只在目标允许的字段上有差异。
- 移交的 `batchNo` 必须来自当前待办快照；加签必须提交目标读取的完整代理树；不能从路径配置或上一次动作复制临时标识。
- 空值、空数组和可选字段的存在性按矩阵构造，并把最终“即将发送”的摘要和实际请求共用同一出口。

### T06：逐接口修复只读事实和处理人核验

- 保留实例查询、任务查询、审核记录、当前表单、暂存数据、催办记录和关注状态的目标原始字段，不用简化 DTO 丢失事实。
- 任务查询保持目标协议：实例筛选在顶层 `flowInstanceIdList`，`pending` 用顶层 `queryUserId`，`done` 用 `data.executorId`；响应行再次按实例 ID和节点 ID复核。
- 提交成功后按实例 ID读取实例当前节点、`currentAuditUserInfo`、精确任务和审核记录；轮询 3 至 5 次，总等待不超过 10 秒，期间不得重发写请求。
- 成功条件只有目标返回真实处理人或精确待办；`isSkip=true` 且目标事实已越过节点才记录跳过。仍在节点、无处理人、无待办在超时后记录 `assignment_missing` 阻塞。

### T07：完成有效表单数据闭环

- 回放状态拆成“基础数据已生成”和“最终表单已确认”；`HISTORY_RUNTIME_VALIDATION_DEFERRED` 不得伪装成最终 `ready`。
- FormMaking 下拉、远程选项、人员关联字段必须等待选项加载，按目标实际值/显示值唯一匹配，保持 `__virtualName`、`__condition`、`__formPersonId` 和关联字段一致。
- 触发必要联动后重新读取运行时当前值；选项不存在、重复、异步覆盖或字段版本冲突时阻塞且不覆盖旧有效数据。
- 保存和运行上下文只使用最终 `effective_form_data`；发起、审批、重提和暂存按目标页面要求提交完整表单数据，不回退历史参考值。

### T08：日志、状态和页面事实一致

- 增加协议摘要：接口、动作、代理 ID、是否有 `batchCode`、`nextAuditorList` 条目数、处理人来源、表单数据版本和响应事实；敏感值只记录脱敏摘要。
- 将“currentAuditUserInfo 缺失”显示为“目标节点未生成处理人”或“处理人查询阻塞”，禁止显示“当前待办已经处理”。
- 记录目标自动分配等待、目标明确跳过、处理人缺失、目标拒绝和写结果待确认的不同 `stopKind`。
- 页面首屏只展示业务结论、接口耗时和目标事实；协议差异放入可展开的排查区，确保用户能把页面请求与 `curl.log` 一一对应。

## 测试任务

测试按根目录 `test/` 分类保存，只运行本切片相关测试；真实写测试默认关闭。

- `test/contracts/f035/protocol/`：每个接口的目标前端 fixture、工具构造结果和归一化差异报告；覆盖字段层级、空值、代理 ID、`batchCode`、`nextAuditorList`、项目 ID、业务关联、请求头和路由族。
- `test/contracts/f035/identity/`：当前计划账号覆盖、岗位缺失阻塞、历史账号不能进入请求、显式选人字段完整性和任务真实处理人来源。
- `test/unit/backend/assignment/`：处理人出现、超时、节点越过、`isSkip`、`run_node_choose`、手动分支、禁止账号回退、有限轮询和不重复写。
- `test/unit/backend/target_write/`：submit、draft、reSubmit、audit、storage、transfer、add-sign、rollback、retrieve、withdraw、forward、tracking、urge 的请求构造和端点白名单。
- `test/unit/backend/history_replay/`：回放状态拆分、最终快照保存、失败不覆盖、路径修订冲突和重复回放幂等。
- `test/unit/frontend/form_runtime/`：下拉实际值/显示值、虚拟字段、人员关联、异步覆盖、选项不存在阻塞和最终值确认。
- `test/integration/f035_target_facts_mysql_test.go`：回放结果、路径配置、`data_revision`、代理 ID、身份摘要和运行上下文的一致性。
- `test/contracts/f035/drift/`：参考前端/Java Controller/Service 关键符号和字段形状漂移检测；发生漂移时阻止静默通过。

## 人工验收

1. 重新运行“欧阳改测试1005 / 路径 12”，在 `curl.log` 中逐字段对照手工成功 curl：`formProxyId`、`batchCode`、空 `nextAuditorList`、`projectId`、公司关联、请求头和表单身份必须符合目标页面规则。
2. 确认请求中的 `global_user_basic_information`、`myUserName`、`myDepName`、`myDutyName` 都是骆蒙恩，不能出现张泽华的 ID、部门或岗位。
3. 确认目标实例当前节点生成真实 `currentAuditUserInfo` 或精确待办 `jobTaskId`，第二节点使用目标返回的处理人继续执行；无处理人时显示“阻塞”，不能显示“已处理”。
4. 对比 NoFormFlow、FormMaking、重提、审批暂存和同意动作的实际请求，确认每种接口只带目标页面真实需要的代理字段和字段层级。
5. 在表单配置中调整下拉值，保存、关闭、重新打开并发起，确认页面最终值、`effective_form_data` 和 `formDataMongoVo.data` 一致。
6. 对每个受控写动作核对 `batchCode` 是否按矩阵要求存在或省略，确认网络异常不会依据 `batchCode` 重发请求。
7. 模拟回放失败、选项不存在、身份读取失败、处理人超时和路径版本变化，确认均阻塞且不覆盖旧的有效表单数据。

## 不在范围内

- 不修改目标平台源码、部署、权限、组织架构、流程配置或数据库。
- 不把“请求字段一致”理解为所有接口强行携带同一批字段；一致性必须按目标页面的具体动作和场景逐接口实现。
- 不使用计划账号、候选人或发起人账号冒充目标当前节点处理人。
- 不把 `batchCode` 当作工具幂等键、重试凭证或跨接口公共字段；是否携带只能由目标接口矩阵决定。
- 不新增目标接口、数据库表、全局缓存或第二套表单模型，不为旧请求保留兼容别名或回退协议。
- 不修改已经完成的历史运行事实；F-035 从新请求、新回放和新运行开始生效。

## 实施顺序和门禁

1. T01 先完成目标前后端协议矩阵和 F-014 冲突项裁决；未形成逐接口基线前不得修改写请求。
2. T02-T06 先完成请求构造、身份/处理人事实和有界核验，确保路径 12 的新发起请求与目标页面一致。
3. T07-T08 再完成表单最终确认、日志和页面事实投影。
4. 测试和静态检查通过后停在 `ready_for_manual`；真实目标人工验收前不得标记 `accepted`。
5. 只有用户明确批准后才能进入 `implementing`；本次更新仍停在 `awaiting_approval`，不得自动开始实现。
