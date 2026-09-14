# F-035 目标请求协议一致性、真实处理人和有效表单数据闭环

- 状态：implementing（2026-09-14 第四轮整改：最新复核发现目标特殊业务/NoFormFlow 仍未逐页面实现、写后核对未持久化、目标衍生审批意见误判及运行详情生命周期竞态；完成前不得进入 ready_for_manual）

## 本切片实施现状（2026-09-14，第四轮整改登记，停在 implementing）

最新用户复核否定了“第三轮全部处理、等待复验”的结论。本轮必须以目标页面实际行为重新校准：特殊业务分支和无表单专用链路需要逐页面实现，不能继续把命中流程统一阻塞；`auto_audit_info_*` 属于目标根据审批事实生成的衍生字段，不能与配置阶段清空的历史值做严格相等比较；写后核对必须持久化并传递到下一节点；运行详情页必须修复路径切换与异步请求生命周期竞态。聚合执行任务、证据、测试和人工验收见 `docs/auto/2026-09-14-f035-review-and-runtime-repair-task.md`。

以下内容是第三轮实施时的登记记录，不代表第四轮整改已经通过。第四轮复核确认其中若干项只是“安全阻塞”或“内存内记录”，
尚未达到目标页面逐代码一致和可恢复运行的完成标准；实施 Agent 必须以本文件顶部的第四轮说明及聚合任务书为准。

评审（第二轮）提出的 2 Critical + 6 High，第三轮曾登记为已处理：

- **Critical 1（FormMaking 特殊业务链路）**：新增目标业务生命周期登记处 `specialBusinessFlowTypes`
  （`internal/adapter/target/protocol_matrix.go`），逐项对应参考页面分支：合同合规/合同盖章（自定义组件，FlowDialog.vue:367-374）、
  资金往来/投资款（业务数据保存，:375、:380-425）、出版委托/专业提资（审批改写日期，EnterpriseExamineOpinion:939-975）、
  年度绩效/考核（审批写意见）、费用报销（审批金额计算）。命中类型的全部写动作在门禁阻塞（含阻断说明与手工处理指引），
  绝不发通用请求顶替；登记外的普通类型才走通用提交路径。
- **Critical 2（无表单专用链路）**：`ResolveVueCustomPage` 不再返回 complete——vue_custom 页面如实标记 partial
  并携带业务链路阻断说明；执行器对 vue_custom 的发起/草稿/重提全部阻塞（项目/业务关联、initiatorRange、并行/手动分支选人
  未逐页面实现前不发通用请求）。
- **Critical 3（formPersonFields）**：新增 `CollectFormPersonFields` 递归目标流程树（含条件/并行分支）收集
  `auditType=form_person` 节点声明的选择器字段（逐节点携带目标节点 ID）；`FlowLifecycleMeta` 随快照进入运行上下文；
  发起/重提/审批前按目标 `traverseFlowNode` 规则生成字段（JSON 取 id、纯文本取原值、已有值不覆盖、源缺失不产出不伪造），
  契约测试锁定全部规则。固定字段名表从 service 层收敛进 adapter 单一登记处，配置期与运行期共用。
- **High 1（运行时旧身份）**：执行器在发起/草稿/重提/审批的表单构造后，用当前会话实时读取身份
  （`Client.CurrentUserIdentity`：目录树 + 岗位）覆盖 `global_user_basic_information` 与全部登记登录人字段及伴生键；
  身份读取失败或岗位缺失阻塞；配置期快照只作为初始值。
- **High 2（customerCode）**：`CallWrite` 增加 customerCode 参数，三个写出口传 `session.CustomerCode`；
  信封注入优先当前会话客户码，缺失时才回落全局配置（契约测试锁定两个分支）。
- **High 3（跨节点值改写）**：决策记录新增 `OverlaidValues`（实际写入值）与 `BaseValues`（完整基线快照）；
  跨节点核对升级为 JSON 规范化深度比较（覆盖嵌套对象/数组/表格字段），丢失与被改写分别给出中文阻塞结论；
  新增 `TestCrossNodeValueRewriteBlocks`/`TestCrossNodeNestedValueRewriteBlocks`。
- **High 4（写后表单核对）**：新增 `verifyFormDataAfterWrite`——写后重读实例当前表单，
  逐字段核对覆盖字段已保存成目标值、基线保留字段未被改写（深度比较）；核对不一致落
  `InstanceFacts.FormDataVerifyIssue`，下一步门禁直接阻塞；通过时落核对通过日志（含指纹）。
- **High 5（矩阵与构造器一致）**：新增 `ValidateBodyMatrix` 在三个写出口发送前强制校验（required 缺失、
  forbidden 出现、未登记端点均拒绝）；审批空业务关联不再伪造空数组（矩阵改 optional）；
  修正 `write_whitelist_dynamic_test` 为按矩阵断言（submit 必带 batchCode、audit 禁带）；
  契约测试锁定矩阵强制行为。

测试：`go test ./test/unit/... ./test/contracts/f035/...` 全部通过（含新增 business_lifecycle_contract、
矩阵强制、会话客户码、特殊业务阻塞、vue_custom 阻塞、实时身份覆盖、跨节点值改写用例）；
`go vet`/`gofmt` 通过；`test/contracts/f035/drift/protocol_symbols_drift.sh` 通过。

第四轮复核已将上述“命中即阻塞”重新定性为临时安全措施，而非完成结果。`beforeSubmitAndDraft`、
`afterSaveFlowInstance`、`contractBusiness`、`saveCostFundsBusiness` 等必须继续逐类型读取目标前端、Controller、Service 和 VO，
实现已确认的业务写入顺序；仅对尚未勘定的分支保留写前阻塞。具体执行、测试和人工验收见
`docs/auto/2026-09-14-f035-review-and-runtime-repair-task.md`。

## 历史轮次（第二轮）

本切片经用户批准实施完成；用户反馈 T05/T06/T08 未完成后退回 `implementing`，第二轮已补齐，重新停在人工验收。已落地的核心修复：

- T01/T02：新增逐接口协议矩阵登记处 `internal/adapter/target/protocol_matrix.go`（四态 required/optional/empty/forbidden，
  覆盖 submit/reSubmit/audit/暂存/移交/加签/回退/取回/撤回/转发/关注/催办全部实际使用的写端点）；删除
  `verdict.ForbiddenWriteField` 无条件 `batchCode` 拦截；`docs/TARGET_SEMANTICS.md` 第 2.2 节与 F-014 同步改写为逐接口矩阵；
  统一写出口按目标 axios 拦截器同语义注入顶层 `sid`/`projectId`（空字符串保留）与 `data.customerCode`。
- T03：`submit/draft` 固定发送顶层 `batchCode`（构造请求时生成一次、预览与实发严格同源、不作为幂等键）与
  空/非空 `nextAuditorList` 数组；`formProxyId` 与 `flowProxyId` 在 submit 和 reSubmit 内强制互斥；重提按页面形状
  固定发送 `nextAuditorList` 数组且不带批次号；审批 `tracking` 顶层布尔无条件携带。
- T04：运行上下文新增 `FormProxyID`，由 `PathConfigService.TemplateFormProxyID` 从模板唯一 Forms 项取得（多表单阻塞）；
  `FormRuntimeSession`/身份替换扩展岗位事实（目标人员目录 `dutyId/dutyName`），身份读取失败与岗位缺失分别落
  `IDENTITY_READ_FAILED`/`IDENTITY_DUTY_MISSING` 阻断问题，不再被忽略。
- T05：`BuildNodeFormData` 每次构造产出完整 `NodeFormDataDecision`（步号、目标节点、动作、基线来源、可编辑/覆盖/
  保留/扣留字段、最终载荷 SHA-256 指纹、校验结论）并完整落 step.log；决策经控制现场进入
  `RunContext.LastFormDataDecision`，下一节点必须以实例最新数据为基线并核对上一节点覆盖字段仍在，丢失即阻塞；
  扣留字段不进 Overlaid（结构保证），空实例基线绝不回退发起态。
- T06：全部实际使用写端点的形状重新与参考页面逐项核对（暂存/移交/回退/取回/撤回/催办/转发/关注与现有实现一致，
  差异端点已在矩阵中登记并修正），新增 `test/contracts/f035/protocol/action_bodies_contract_test.go` 把每个动作的
  实际构造器输出与矩阵逐字段对照（required 必须存在、forbidden 不得出现）。
- T07：实例事实新增“目标节点已到达但未生成 currentAuditUserInfo/待办”分型；进入“正在生成处理人”有界轮询
  （≤5 次、≤10 秒、只读复查、绝不重发写请求）；超时后门禁按 `assignment_missing` 阻塞并显示“目标节点未生成处理人”。
- T08：回放状态拆分——新增 `HistoryDataStatusBaseReady`（基础数据已生成、最终表单未确认），批量回放无浏览器
  运行时校验时不再伪装成最终 `ready`；运行前检查对 `base_ready` 给出“请打开表单数据页完成确认”的明确阻塞；
  前端路径列表与类型定义同步展示新状态。最终 `ready` 仍只由“运行时选项绑定+回读+保存确认”产生（既有链路）。
- T09：step.log 新增协议摘要行（接口、动作、代理来源、批次号、nextAuditorList 条目数、表单基线、处理人）；
  内网系统按用户裁决日志原样记录完整请求/响应，不做脱敏（已写入 AGENTS.md 与本文件）。

测试：`test/contracts/f035/protocol/`（submit/draft/reSubmit/audit/全部动作载荷契约 + 信封注入 + 矩阵登记 + 批次号形状）、
`test/contracts/f035/identity/`、`test/unit/backend/form_data_by_node/`（分节点基线/扣留/保留/决策记录/跨节点丢失阻塞）、
执行器 `assignment_poll_test.go`、`test/contracts/f035/drift/protocol_symbols_drift.sh`。
`go test ./test/unit/... ./test/contracts/f035/...` 全部通过；`test/integration` 与真实目标相关的既有失败
（环境配置缺失与 F-014/F-018/F-016 既有记录）经未修改工作树复现确认与本切片无关。

已如实登记的剩余边界：`test/integration/f035_target_facts_mysql_test.go`（回放结果、data_revision、代理 ID 与身份摘要的
真实 MySQL 一致性用例）未在本轮落地，相关事实已由既有 f012/f015 集成用例与上述契约用例覆盖；如人工验收认为必须补充，
按验收反馈退回 `implementing` 处理。

## 原始状态记录

- 本切片登记时状态：awaiting_approval（2026-09-14）
- 产品依据：`docs/PRODUCT.md` 的目标平台事实、运行状态、表单数据和安全写入原则
- 架构依据：`docs/ARCHITECTURE.md` 的目标适配层、执行器门禁、回放存储和 form-runtime 边界
- 本切片性质：修复现有目标请求、身份数据、处理人核验和表单数据链路；不修改目标平台源码、权限、流程配置，不新增目标写接口，不新增数据库表
- 排查样本：计划“欧阳改测试1005”（plan 29），运行号 7（run 98），路径 12（path 4221）；运行号 6 同样复现

## 单一用户结果

工具发出的每一个目标请求都必须按目标平台 GroupApproveManage 前端和对应 Java 服务的实际构造方式生成。表单流程使用目标返回的 `formProxyId`，无表单流程使用 `flowProxyId`；发起人的身份全部取当前计划配置的账号，不能把排查样例中的骆蒙恩、张泽华或任何其他账号写成固定规则。显式选人、`nextAuditorList`、`batchCode`、`projectId`、业务关联和空值形状都按具体目标接口逐项对齐。目标成功创建实例后，工具只使用目标返回的真实节点处理人继续执行；目标没有产生处理人时明确阻塞，不用计划账号或候选人代替。

## 硬性执行规则

- 目标 GroupApproveManage 前端、Controller、Service 和请求/响应 VO 是唯一行为依据。不得为了“合理”而修改目标行为；项目中阻挠目标请求形状的代码、判定器、文档和测试必须在本切片内改写。
- 同一计划快照、同一节点、同一动作必须生成同一个规范化请求。禁止随机候选人、隐式默认账号、字段重排之外的分支差异和用重试掩盖不确定结果。
- 字段存在性按协议矩阵固定：目标发送的字段必须发送，目标省略的字段必须省略；空数组、空对象、`null` 和缺失是四种不同形状，不能用 Go 的 `omitempty` 或“非空才发送”替代。
- `batchCode` 不再接受项目级“一律禁止”规则。FormMaking `submit` 以目标 `FlowDialog` 和人工成功 curl 的形状发送顶层 `batchCode`；其他动作逐行按其目标页面构造。实施时必须删除 `verdict.ForbiddenWriteField` 对 `batchCode` 的无条件拦截，并同步改写 F-014、`docs/TARGET_SEMANTICS.md` 和契约测试；不得保留旧禁令作为运行时兜底。
- 每次写请求只发送一次；响应缺失先按目标实例/任务事实对账，不因 `batchCode`、超时或网络错误自动再次写入。

## 完成标准

- [ ] 建立本切片的目标接口协议矩阵，覆盖运行实际使用的全部读写接口；每个接口记录 HTTP 方法、路径、查询参数、请求头、请求体层级、字段名、字段类型、空值/空数组/省略规则、响应成功判据和证据位置。
- [ ] 工具请求的 `platformCode`、`sid`、`Accept`、`Content-Type`、`Origin`、`Referer`、请求体 `sid` 与目标页面请求保持同一语义；只允许环境地址差异，不得出现路径族或参数位置差异。
- [ ] FormMaking 发起/重提请求按目标页面发送唯一 `formProxyId`；NoFormFlow 发起请求按目标页面发送 `flowProxyId`；禁止把发布流程代理 ID 当作表单代理 ID，也禁止无依据同时发送两个代理字段。
- [ ] 目标页面实际携带的 `batchCode` 必须按接口和动作原样携带，生成时机、作用范围和字段位置与页面一致；不得把 `batchCode` 当作工具幂等键、重试依据或跨接口通用字段。FormMaking `submit` 必须复现目标页面的顶层字段形状；其他接口必须按各自行矩阵决定存在或省略。`verdict`、F-014 和语义清单中的旧“一律禁止”必须被同步删除或改写，不能与本切片并存。
- [ ] `nextAuditorList` 的存在性和数组形状按目标页面保持一致；每个显式选人项完整携带目标要求的 `bizId`、`name`、`auditDetailType`、`nodeProxyId`。自动扩展属性节点不伪造处理人，空数组也不得被解释为已解析出人员。
- [ ] 发起人身份与表单身份一致：每次运行先读取计划配置的账号并建立 `InitiatorIdentity`，再由该事实构造 `global_user_basic_information`、`myCompanyName`、`myDepName`、`myDutyName`、`myUserName` 及全部 `__condition`、`__formPersonId`；禁止使用历史账号值，岗位信息缺失时发起前阻塞。计划账号变化后必须得到新身份，不能沿用旧账号快照。
- [ ] `data.name`、`companyId`、`projectId`、`flowInstanceBizRelevanceList`、`formDataMongoVo.data` 的来源和格式与目标页面一致；日期、数字、空字符串、虚拟字段和人员字段不被工具自行改写。
- [ ] 分节点填写的表单按“节点 + 动作”构造：每次写请求先读取目标实例当前完整表单，再只覆盖当前目标节点声明可编辑且已配置的字段；新建发起没有实例时只发送发起节点允许的字段，后续节点字段不得提前写入；当前节点未声明可编辑字段时不得用历史配置覆盖目标现值。
- [ ] 每个节点实际发出的表单数据都能追溯到该节点的目标字段权限、配置值、实例基线和最终 JSON；节点 1 写入的值必须在节点 2/3 的请求中被保留，节点 2 只能追加/修改其自身字段，不能回滚或覆盖其他节点已经填写的值。
- [ ] 审批、暂存、重提、回退、取回、撤回、移交、加签、转发、关注/取消关注、催办等实际使用的读写接口全部通过同一份协议矩阵和请求构造器；不得只修复 submit 而留下其他接口的字段位置、端点或身份差异。
- [ ] 当前待办任务的 `jobTaskId`、`batchNo`、`flowNodeProxyId`、`flowProxyId` 和真实处理人只能来自目标最新任务快照；缺失时阻塞，不能回退计划账号、发起人或候选人。
- [ ] 目标节点已到达但处理人尚未生成时进入“正在生成处理人”并执行有界轮询；等待结束仍无处理人和待办时标记“阻塞”，不显示“已处理”、不重复发送写请求。
- [ ] 回放后的表单值经过运行时下拉选项绑定、最终回读和保存后，才进入最终 `ready`；页面、路径配置和运行发起请求始终使用同一份最终有效表单数据。
- [ ] 请求日志能按 trace、接口、动作和尝试列出协议摘要、身份摘要、代理 ID、处理人来源、批次号是否携带、表单数据版本和目标事实；本项目为内网系统，日志按原样记录完整请求与响应（含 SID 与表单正文），不做脱敏。
- [ ] 不改变一次写、写后重读、账号串行、运行快照、动作顺序、F-034 动作矩阵和 F-033 节点过渡语义。

## 根因和证据

### A. 当前实例“成功创建但无处理人”首先是请求载荷不一致

运行 7 / 路径 12 的实际请求见 [curl.log](/Volumes/oygsky/AIstudy/test-auto-pro-v2/logs/runs/运行_7__欧阳改测试1005__run-98/paths/路径%2012__path-run-163__path-4221/curl.log:2)：

- 工具只发送 `data.flowProxyId=73eaa9...`，没有按 FormMaking 页面发送 `data.formProxyId=5222ef...`。
- 工具没有发送人工请求中出现的 `batchCode`、空的 `nextAuditorList` 和 `projectId` 字段；对本次 FormMaking `submit`，人工成功 curl 已确认这些字段必须存在且位于示例中的层级。该结论不能扩散到其他动作，其他动作只能使用各自行矩阵的已登记形状。
- 工具发送的 `data.name`、日期格式和表单值格式与人工请求不同；业务值可以不同，但字段类型、层级、虚拟字段和空值规则不能不同。
- 本次工具表单身份是张泽华（用户 ID `0aff...`、综合管理部、行政专员），而该计划这次运行的配置账号和人工请求是骆蒙恩（用户 ID `08e604...`、建设运营部、工程经理）。这不是“发起人固定为骆蒙恩”的规则，而是本次运行的身份错配证据；其他计划必须使用各自配置账号的身份。

目标页面在 [FlowDialog.vue](/Volumes/oygsky/AIstudy/test-auto-pro-v2/参考代码/rsh-flow-components/src/views/GroupApproveManage/Submitted/components/FlowDialog.vue:757) 按“有表单使用 `formProxyId`、无表单使用 `flowProxyId`”构造请求，并在 [FlowDialog.vue](/Volumes/oygsky/AIstudy/test-auto-pro-v2/参考代码/rsh-flow-components/src/views/GroupApproveManage/Submitted/components/FlowDialog.vue:667) 用当前登录会话填充完整 `global_user_basic_information`。目标 Java 提交入口随后按该协议校验代理、权限和起始节点，见 [FlowSubmitServiceImpl.java](/Volumes/oygsky/AIstudy/test-auto-pro-v2/参考代码/java-serve/rsh-cloud-workflow-center/src/main/java/com/rsh/cloud/workflow/center/service/impl/FlowSubmitServiceImpl.java:86)。

目标返回 HTTP 200、`isSuccess=true` 只证明实例被创建。实际实例仍停在“行政综合部-考勤管理”，`currentAuditUserInfo=null`，待办为 0，见 [step.log](/Volumes/oygsky/AIstudy/test-auto-pro-v2/logs/runs/运行_7__欧阳改测试1005__run-98/paths/路径%2012__path-run-163__path-4221/step.log:6) 和 [step.log](/Volumes/oygsky/AIstudy/test-auto-pro-v2/logs/runs/运行_7__欧阳改测试1005__run-98/paths/路径%2012__path-run-163__path-4221/step.log:13)。因此“接口请求成功”不能等同于“目标处理人已成功生成”。

### B. 表单身份错位会直接影响扩展属性处理人解析

路径 12 的第二节点是目标扩展属性规则“集团考勤管理员”。工具把历史表单中的张泽华部门、岗位和人员字段发给当前计划账号会话，目标据此计算不到该表单应有的处理人，最终出现实例有当前节点但没有 `currentAuditUserInfo` 和待办。运行 6 与运行 7 均发送同一组张泽华身份并复现，说明本次是工具身份数据污染，不是目标平台偶发延迟。修复规则必须按每个计划的配置账号动态计算，不能把骆蒙恩写死。

当前代码虽然有身份替换，但读取和保存时忽略了 `currentUserIdentity` 错误，见 [path_data_workspace.go](/Volumes/oygsky/AIstudy/test-auto-pro-v2/internal/service/path_data_workspace.go:113) 和 [path_data_workspace.go](/Volumes/oygsky/AIstudy/test-auto-pro-v2/internal/service/path_data_workspace.go:414)；运行上下文提交时又没有最后一道身份一致性校验。必须在真正写请求前重新构造并校验身份，不能继续使用历史 `effective_form_data`。

### C. `formProxyId` 在读链路中已经存在，但被运行上下文丢弃

目标模板读取会解析 `formTemplateList` 并生成 `PathConfigurationSnapshot.Forms`，但 `buildRunContext` 只保存计划的 `FlowProxyID`，见 [run_orchestration.go](/Volumes/oygsky/AIstudy/test-auto-pro-v2/internal/service/run_orchestration.go:521)。提交门禁也只填充 `FlowProxyID`，见 [gate.go](/Volumes/oygsky/AIstudy/test-auto-pro-v2/internal/engine/step/gate.go:141)。因此这是工具内部数据传递丢失，不是目标平台没有返回表单代理 ID。

### D. 其他接口已经存在可复现的协议漂移

当前 `BuildActionBody`、`BuildSubmitBody` 和任务查询已经集中构造，但存在可确认的“非空才发送”与目标页面固定发送空数组/字段的差异；动作端点同时存在 `/web/flowInstanceApi/*` 和 `/flowInstanceApi/audit` 两个路由族，审批字段、表单容器、业务关联、`tracking`、`batchNo` 和 `jobTaskId` 的位置不能凭通用模型猜测。F-035 必须用逐接口矩阵替换这些分支，不允许留下运行时猜测。任务查询还必须保持实例过滤在顶层 `flowInstanceIdList`、待办视角使用顶层 `queryUserId`、已办执行人使用 `data.executorId`，参考 [task_query.go](/Volumes/oygsky/AIstudy/test-auto-pro-v2/internal/adapter/target/task_query.go:11)。

### E. 分节点表单数据必须按节点时机发送

当前 `BuildNodeFormData` 已尝试按字段权限合并，但 F-035 必须把“节点填写时机”变成写请求门禁，而不是只作为日志说明：

- 发起 `submit`：基线是发起态完整表单；只保留发起节点可填写字段和目标页面会自动生成的系统字段，后续节点专属字段不得从历史样本提前写入。
- 当前节点 `storageFormData`：基线是目标实例当前完整表单；只提交当前节点声明可编辑字段和目标页面要求的当前节点检查点字段。
- 当前节点 `audit`：先读同一实例最新完整表单，再叠加当前节点允许修改的配置值；如果目标页面审批表单会同时提交整份表单，工具必须提交这份合并后的完整数据，不能只发当前节点字段。
- `reSubmit`：基线必须来自该实例当前表单和目标页面重提字段；不得使用新建发起的字段白名单，也不得把历史快照覆盖当前实例已经填写的字段。
- 节点没有可编辑字段、字段值没有经过目标选项绑定或当前实例读取失败时，在写请求前阻塞；不得使用“配置里有值”作为发送依据。

每次请求必须落一份 `NodeFormDataDecision`：`stepNo`、目标节点 ID、动作、基线来源、当前节点可编辑字段、覆盖字段、保留字段、禁止提前写入字段、最终值版本和校验结果。后续节点请求只能消费该节点决策和最新实例事实，禁止直接读取一份全局 `EffectiveFormData` 作为最终载荷。

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

矩阵的字段存在性使用四个固定值：`required`（目标必发）、`optional`（仅目标页面条件成立时发送）、`empty`（目标固定发送空值）和 `forbidden`（目标明确不发送）。`batchCode` 和 `nextAuditorList` 不能因为某个接口需要就扩散到全部接口，也不能因为旧语义清单曾禁止就忽略手工请求中的真实字段。F-035 实施时必须同步更新 `docs/TARGET_SEMANTICS.md`、F-014 的判定约束和相关契约测试，使每个接口只落入上述四种状态；未登记状态的接口禁止进入实现。

### 已确认的 FormMaking `submit` 载荷形状

以下是本次人工成功请求已经确认的字段契约，实施者必须先用同一目标页面源码和 Java 入参再次登记证据，再按原样实现；字段值来自当前计划和当前会话，不能照抄示例值：

| 位置 | 字段 | 形状和规则 |
| --- | --- | --- |
| query | `sid`、`platformCode` | `sid` 为当前会话，`platformCode=200001`；两者均在请求日志中脱敏记录 |
| headers | `Accept`、`Content-Type`、`Origin`、`sid` | 与目标页面同语义；`sid` 与 query/body 使用同一会话值 |
| `data` | `name` | 目标页面命名规则生成的字符串，使用最终表单业务值和计划发起人 |
| `data` | `formProxyId` | FormMaking 表单代理 ID；本请求不得出现 `flowProxyId` |
| `data` | `companyId`、`customerCode` | 当前计划发起人的目标公司和客户编码 |
| `data` | `flowInstanceBizRelevanceList` | 数组；每项按目标页面携带 `otherBiz`、`otherBizId` |
| `formDataMongoVo.data` | 表单字段对象 | 完整表单对象；节点白名单、当前身份伴生字段和目标选项值按 T05 生成 |
| 顶层 | `nextAuditorList` | 本场景固定发送数组；无显式选人时发送 `[]`，有显式选人时逐项发送目标要求字段 |
| 顶层 | `batchCode` | 本场景固定发送字符串，位置在顶层；不用于幂等或重试 |
| 顶层 | `sid`、`projectId` | `sid` 为同一会话；`projectId` 按目标页面发送，空字符串也必须保留，不得因空值省略 |

这张表只约束已确认的 FormMaking `submit` 场景；草稿、重提和其他动作必须有自己的矩阵行，不能复用或删减本表字段。

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
- 将 `docs/TARGET_SEMANTICS.md` 和 F-014 中的统一 `batchCode` 禁令改为按端点、动作和目标部署证据的明确矩阵，并删除 `internal/engine/verdict` 的无条件字段拦截；矩阵同步、判定器和契约测试必须在同一原子提交完成，不能留下“新构造器 + 旧禁令”的半套修复。

### T03：修复表单/无表单代理 ID和发起动作

- 在运行上下文保留 `FormProxyID`，从新流程模板的唯一 `Forms` 项取得；FormMaking 发起/草稿只发送 `formProxyId`，NoFormFlow 只发送 `flowProxyId`。
- 重提使用实例事实中的实时代理 ID和表单代理 ID，严格按目标 `reSubmit` 前端形状构造，不把新发起协议复用到重提。
- `submit`、草稿和 `reSubmit` 按各自目标页面矩阵生成 `nextAuditorList`、`batchCode`、`projectId` 等字段；本次人工 `submit` 的 `batchCode`、空 `nextAuditorList` 和 `projectId` 为 `required/empty/required` 的具体证据，不能套用到草稿或重提。自动解析节点只发送矩阵规定的空数组或其他形状，不伪造人员。
- `data.name` 由与目标页面相同的最终表单值和命名规则生成；日志目录名称仍沿用工具自己的页面映射，不用实例名反向影响协议。

### T04：修复当前账号、表单身份和显式处理人

- 扩展运行时账号事实，至少包含用户 ID/姓名、公司 ID/名称、部门 ID/名称、岗位 ID/名称；这些值必须由计划配置账号的当前目标会话读取，不能从示例、浏览器上一次登录或路径历史数据取得；岗位无法从当前目标会话可靠取得时，提交前阻塞。
- 提交和重提前覆盖所有目标登录人字段及关联键，检查用户、部门、岗位之间相互一致；身份读取错误不得被忽略。计划账号是变量，验收时用当前计划实际配置的账号替换示例账号。
- 显式 `run_node_choose`、并行和分支选人严格按目标页面的 `nextAuditorList` 字段逐项发送；固定扩展属性节点不使用候选人、计划账号或发起人替代目标自动解析。
- 任务动作必须以目标任务快照的真实处理人账号发出，任务 ID、批次号和节点 ID全部来自同一次新鲜读取；发现人员缺失、多个任务或人员不匹配时阻塞。

### T05：逐节点构造表单数据

- 给每个编译步骤绑定目标真实节点 ID、动作、表单数据来源和字段权限快照；不能只传工具 `nodeKey`。
- 发起、暂存、审批、重提分别调用明确的基线选择规则：无实例用发起态数据，有实例必须读取目标实例当前完整数据；读取失败或返回空时按协议判断，不能静默切换另一种基线。
- 当前节点只能覆盖其 `fieldPower=edit` 的字段及目标页面明确要求的伴生字段；后续节点字段列入 `withheld`，并在写前阻塞检查中确认没有进入载荷。
- 每个节点保存实际发送值的摘要指纹和字段决策；下一节点必须重新读取目标实例并验证上一节点值仍在，发现丢失或被覆盖立即阻塞。
- 下拉、人员、远程选项和虚拟字段全部完成绑定、回读后才允许进入该节点写请求；配置页面看到的值不能直接替代运行时最终值。
- 编译时建立 `NodeFieldOwnership`：按目标节点 ID、路由顺序和 `fieldPower` 为每个配置字段确定唯一节点所有者；无法唯一归属的字段直接阻塞，不得由执行时猜测。
- 每次表单写入按固定算法执行：绑定当前节点和动作；读取一次同实例最新完整表单作为基线；深拷贝基线；仅覆盖当前节点所有且已绑定的字段及目标要求的伴生字段；将其他字段保留；将后续节点字段列入 `withheld` 并断言未提前进入载荷；写后读取并逐字段核对覆盖值和保留值。
- `EffectiveFormData` 只能作为配置来源，不能直接作为任一节点最终载荷。每个 `NodeFormDataDecision` 必须记录基线版本、字段所有权、覆盖/保留/禁止字段和规范化 JSON 指纹。

### T06：逐接口修复写请求构造

- 对 `BuildSubmitBody`、`BuildActionBody` 和审批请求分别实现目标页面对应的协议形状，不以一个通用 `data` 模型强行合并所有动作。
- 逐项核对 `storageFormData`、`audit`、`reSubmit`、`approverAppend`、`updateFlowProxy`、回退、取回、撤回、转发、关注和催办的端点、路由族、字段层级、表单容器和业务关联。
- `audit` 的 `jobTaskId`、`flowProxyId`、`auditRecord`、`formDataMongoVo`、`nextAuditorList` 和 `tracking` 必须与目标审批页一致；同意与不同意只在目标允许的字段上有差异。
- 移交的 `batchNo` 必须来自当前待办快照；加签必须提交目标读取的完整代理树；不能从路径配置或上一次动作复制临时标识。
- 空值、空数组和可选字段的存在性按矩阵构造，并把最终“即将发送”的摘要和实际请求共用同一出口。

### T07：逐接口修复只读事实和处理人核验

- 保留实例查询、任务查询、审核记录、当前表单、暂存数据、催办记录和关注状态的目标原始字段，不用简化 DTO 丢失事实。
- 任务查询保持目标协议：实例筛选在顶层 `flowInstanceIdList`，`pending` 用顶层 `queryUserId`，`done` 用 `data.executorId`；响应行再次按实例 ID和节点 ID复核。
- 提交成功后按实例 ID读取实例当前节点、`currentAuditUserInfo`、精确任务和审核记录；轮询 3 至 5 次，总等待不超过 10 秒，期间不得重发写请求。
- 成功条件只有目标返回真实处理人或精确待办；`isSkip=true` 且目标事实已越过节点才记录跳过。仍在节点、无处理人、无待办在超时后记录 `assignment_missing` 阻塞。

### T08：完成有效表单数据闭环

- 回放状态拆成“基础数据已生成”和“最终表单已确认”；`HISTORY_RUNTIME_VALIDATION_DEFERRED` 不得伪装成最终 `ready`。
- FormMaking 下拉、远程选项、人员关联字段必须等待选项加载，按目标实际值/显示值唯一匹配，保持 `__virtualName`、`__condition`、`__formPersonId` 和关联字段一致。
- 触发必要联动后重新读取运行时当前值；选项不存在、重复、异步覆盖或字段版本冲突时阻塞且不覆盖旧有效数据。
- 保存和运行上下文只使用最终 `effective_form_data`；发起、审批、重提和暂存按目标页面要求提交完整表单数据，不回退历史参考值。

### T09：日志、状态和页面事实一致

- 增加协议摘要：接口、动作、代理 ID、是否有 `batchCode`、`nextAuditorList` 条目数、处理人来源、表单数据版本和响应事实；敏感值只记录脱敏摘要。
- 将“currentAuditUserInfo 缺失”显示为“目标节点未生成处理人”或“处理人查询阻塞”，禁止显示“当前待办已经处理”。
- 记录目标自动分配等待、目标明确跳过、处理人缺失、目标拒绝和写结果待确认的不同 `stopKind`。
- 页面首屏只展示业务结论、接口耗时和目标事实；协议差异放入可展开的排查区，确保用户能把页面请求与 `curl.log` 一一对应。

## 测试任务

测试按根目录 `test/` 分类保存，只运行本切片相关测试；真实写测试默认关闭。

- `test/contracts/f035/protocol/`：每个接口的目标前端 fixture、工具构造结果和归一化差异报告；覆盖字段层级、空值、代理 ID、`batchCode`、`nextAuditorList`、项目 ID、业务关联、请求头和路由族。
- `test/contracts/f035/identity/`：当前计划账号覆盖、至少两个不同计划账号的用户/公司/部门/岗位/条件字段切换、岗位缺失阻塞、历史账号不能进入请求、显式选人字段完整性和任务真实处理人来源；禁止任何样例账号常量进入请求。
- `test/unit/backend/assignment/`：处理人出现、超时、节点越过、`isSkip`、`run_node_choose`、手动分支、禁止账号回退、有限轮询和不重复写。
- `test/unit/backend/form_data_by_node/`：发起节点、后续节点、暂存、审批、重提的基线选择、字段权限、上一节点值保留、后续字段禁止提前写入和最终 JSON 对照。
- `test/unit/backend/target_write/`：submit、draft、reSubmit、audit、storage、transfer、add-sign、rollback、retrieve、withdraw、forward、tracking、urge 的请求构造和端点白名单。
- `test/unit/backend/history_replay/`：回放状态拆分、最终快照保存、失败不覆盖、路径修订冲突和重复回放幂等。
- `test/unit/frontend/form_runtime/`：下拉实际值/显示值、虚拟字段、人员关联、异步覆盖、选项不存在阻塞和最终值确认。
- `test/integration/f035_target_facts_mysql_test.go`：回放结果、路径配置、`data_revision`、代理 ID、身份摘要和运行上下文的一致性。
- `test/contracts/f035/drift/`：参考前端/Java Controller/Service 关键符号和字段形状漂移检测；发生漂移时阻止静默通过。

## 人工验收

1. 重新运行“欧阳改测试1005 / 路径 12”，在 `curl.log` 中逐字段对照手工成功 curl：`formProxyId`、`batchCode`、空 `nextAuditorList`、`projectId`、公司关联、请求头和表单身份必须符合目标页面规则；身份值取该计划当前配置账号，不能照抄样例。
2. 确认请求中的 `global_user_basic_information`、`myUserName`、`myDepName`、`myDutyName` 都与该计划当前配置账号的目标会话一致；样例中的骆蒙恩只是本次证据，不是固定发起人，不能出现其他历史账号的 ID、部门或岗位。
3. 确认目标实例当前节点生成真实 `currentAuditUserInfo` 或精确待办 `jobTaskId`，第二节点使用目标返回的处理人继续执行；无处理人时显示“阻塞”，不能显示“已处理”。
4. 对比 NoFormFlow、FormMaking、重提、审批暂存和同意动作的实际请求，确认每种接口只带目标页面真实需要的代理字段和字段层级。
5. 对一个至少包含两个可填写节点的表单逐节点验收：节点 1 的数据进入 submit；节点 2 的数据只在节点 2 的 storage/audit 请求中出现；节点 2 请求保留节点 1 已填写值；任何后续节点字段不得在 submit 中提前出现。
6. 将同一流程计划切换到另一个发起账号重新运行，确认所有身份字段、公司/部门/岗位关联和 `data.name` 随计划账号变化，且不出现上一次账号的任何 ID。
7. 在表单配置中调整下拉值，保存、关闭、重新打开并发起，确认页面最终值、`effective_form_data` 和每个节点实际 `formDataMongoVo.data` 一致。
8. 对每个受控写动作核对 `batchCode` 是否按矩阵要求存在或省略，确认网络异常不会依据 `batchCode` 重发请求。
9. 模拟回放失败、选项不存在、身份读取失败、处理人超时和路径版本变化，确认均阻塞且不覆盖旧的有效表单数据。

## 不在范围内

- 不修改目标平台源码、部署、权限、组织架构、流程配置或数据库。
- 不把“请求字段一致”理解为所有接口强行携带同一批字段；一致性必须按目标页面的具体动作和场景逐接口实现。
- 不使用计划账号、候选人或发起人账号冒充目标当前节点处理人。
- 不把 `batchCode` 当作工具幂等键、重试凭证或跨接口公共字段；是否携带只能由目标接口矩阵决定。
- 不新增目标接口、数据库表、全局缓存或第二套表单模型，不为旧请求保留兼容别名或回退协议。
- 不修改已经完成的历史运行事实；F-035 从新请求、新回放和新运行开始生效。

## 实施顺序和门禁

1. T01 先完成目标前后端协议矩阵，并同步改写 F-014、`docs/TARGET_SEMANTICS.md` 和 `verdict` 的旧 `batchCode` 禁令；未形成逐接口基线前不得修改写请求。
2. T02-T07 先完成请求构造、计划账号身份、逐节点表单数据、处理人事实和有界核验，确保路径 12 的新发起请求与目标页面一致。
3. T08-T09 再完成表单最终确认、日志和页面事实投影。
4. 测试和静态检查通过后停在 `ready_for_manual`；真实目标人工验收前不得标记 `accepted`。
5. 2026-09-14 用户明确批准实施，状态流转 `awaiting_approval -> implementing -> ready_for_manual`；
   真实目标人工验收通过前不得标记 `accepted`。
