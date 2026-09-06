# 目标动作真值表（交付返工阶段 1 产物）

- 任务书：`docs/auto/2026-09-07-delivery-repair-task.md` 执行顺序第 1 步。
- 覆盖 15 个动作：保存草稿、提交、重新提交、暂存表单、加签、移交、同意、不同意、回退、取回、撤回、催办、转发、关注、取消关注。
- 依据：`参考代码/rsh-cloud-invest-power-system/src/views/GroupApproveManage/` 及直接公共组件、`src/api/index.js`、`参考代码/java-serve/`（rsh-cloud-web-api 网关 → workflow-center-api → workflow-center 三跳均已追到服务实现）；工具侧现状取自 `internal/engine/actioncatalog/catalog.go`、`internal/adapter/target/write_actions.go`。
- 规则：每条事实附 `文件:行号` 证据；没有源码依据的语义一律不写。
- 状态标记：✅=源码链路闭合且与工具实现一致；⚠️=部分闭合（缺项写明）；⛔=阻塞/实现与目标语义不符。

## 缩写（参考代码内路径）

- **FD** = `GroupApproveManage/Submitted/components/FlowDialog.vue`；**EEO** = `GroupApproveManage/components/EnterpriseExamineOpinion.vue`；**EO** = `components/examineOpinion.vue`；**Mixin** = `components/mixin.js`；**Backlog/Submitted/Finished/DueOut** = `GroupApproveManage/<页>/index.vue`；**API** = `src/api/index.js`；**Axios** = `src/utils/axios.js`
- **WebCtrl** = `rsh-cloud-web-api/.../ops/flow/FlowInstanceWebController.java`；**ApiCtrl/ApiSvc** = `workflow-center-api/.../controller/FlowInstanceApiController.java`、`.../service/impl/FlowInstanceApiServiceImpl.java`；**WFCtrl** = `workflow-center/.../controller/FlowInstanceController.java`；**AuditSvc** = `workflow-center/.../service/impl/FlowAuditServiceImpl.java`；**RollSvc** = `.../FlowRollBackServiceImpl.java`；**InstSvc** = `.../FlowInstanceServiceImpl.java`；**SubmitSvc** = `.../FlowSubmitServiceImpl.java`；**ReSubmitSvc** = `.../FlowReSubmitServiceImpl.java`；**OperateSvc** = `.../FlowOperateServiceImpl.java`；**Enum** = `rsh-framework-all/.../ExecuteResultEnum.java`；**TaskVo** = `.../FlowJobTaskLinkVo.java`

## 0. 通用事实（多动作共享）

- **统一响应**：`BaseResponseProtocol`（`res.isSuccess`/`res.message`/`res.data`；Axios:218-231 按 isSuccess resolve/reject）；鉴权失败 body `code===AUTH_401|RESP401` 清会话跳登录（Axios:186-212,246-269）；`data.errorType ∈ {run_node_choose, parallel_choose, custom_choose}` 表示需补选人、抑制全局报错（Axios:221-224）。
- **任务事实来源**：`POST /web/flowJobTaskLink/list`（API:496,743）。行字段即实时任务事实：`jobTaskId`(TaskVo:39)、`flowInstanceId`(:29)、`flowNodeProxyId`(:34)、`batchNo`(:51)、`auditWay`(:55)、`flowNextNodeAuditType`(:132)、`formProxyId`(:210)、`flowProxyId`(:215)。待办查 `taskStatus:'pending'`（Backlog:653-655）；已办查 `taskStatus:'done'`+`executorId`（Finished:459-465）。**返工任务书第 3 步「发送前读取最新待办/已办补齐 jobTaskId、batchNo」的接口与字段依据即此。**
- **执行账号**：前端 sid（localStorage.token）→ 网关参数 → 后端 `getUser(sid)` 从 Redis 取当前用户（AuditSvc:93-95、RollSvc:310-312、SubmitSvc:90）；工具侧对应「计划账号会话」，写前复用经探活确认的会话（提交 `d58cfb8`）。
- **网关幂等**：`/web/flowInstanceApi/submit` 带 `@Consistency(submitCount=1)`（WebCtrl:71-74），前端提交实际携带随机 `batchCode`（FD:261,875）。工具侧语义清单 2.2 的 batchCode 禁令是对工具自身的纪律约束，与目标行为不冲突，但须知道目标存在这道去重。
- **权限旁路实测线索**：用户名 `欧阳改` 在审批权限校验中直接放行（AuditSvc:53,97-99,159-160）——测试账号行为差异需纳入真实运行预期。

## 1. 保存草稿 save_draft —— ✅

- **入口/条件**：FD:31 草稿按钮（`stepsActive != 1 && isForm && !isLending`）；待发编辑态 EEO:1617-1716 `handleSaveDraft`。首次无实例 ID，再次保存带 `data.id`（更新原草稿，SubmitSvc:347-361）。
- **前置钩子顺序**：金额等业务校验（FD:1063-1127）→ `checkFlowPermission` 预检（同端点带 `checkPermissions:'first'`，FD:364-386；SubmitSvc:123-126 直接返回成功）→ `getData(false)` 不做表单必填（FD:663）→ 注入 `global_user_basic_information`（FD:667-676）→ 组 `flowInstanceBizRelevanceList`（FD:686-736）→ `param.data.status='draft'`（FD:752-755）→ **表单事件脚本 `beforeSubmitAndDraft`（FD:876，可中止）**。
- **端点**：`POST /web/flowInstanceApi/submit`（FD:898-899；API:400）——与提交同端点，`status='draft'` 区分。
- **请求字段**：`data{name, status:'draft', flowInstanceBizRelevanceList[], flowProxyId|formProxyId, companyId, id?}` + `formDataMongoVo.data` + 顶层 `nextAuditorList[]`（草稿为空）、`batchCode`。
- **后端**：SubmitSvc:86-253；草稿跳过审批权限（:112-120）与下一节点选人校验（OperateSvc:894-896）；`initFLowInstanceSaveVO` 无 id 生成 `instanceId+batchNo`（:347-361）；draft 分支**不生成待办/审核记录/回调**（:219-242）。
- **写后重读**：目标无重读接口；父页 `@success="fetchData"` 刷新。工具判据：实例状态=draft + 原始表单数据一致，不生成待办——与 `catalog.go:39-52` 一致。

## 2. 提交 submit —— ✅

- **入口**：FD:33 提交按钮 → `sumbitFlow('submit')`（FD:1051）。
- **前置钩子**：同草稿链，但 `getData(true)` 强制表单校验（FD:663,665）、**不设 `status`（后端默认 run，SubmitSvc:317-319）**、`beforeSubmitAndDraft` 可中止（FD:876,881）。
- **端点**：`POST /web/flowInstanceApi/submit`。
- **nextAuditorList 规则（已按 `e4a1841` 实现工具侧）**：`validateNextNodeRunChoose`（OperateSvc:893-923）——下一节点/并行节点 `auditType==run_node_choose` 必须携带匹配 `nodeProxyId` 的条目，缺失抛 `errorType:'run_node_choose'`（OperateSvc:1105-1123）；`level` 查职级 0 人/多人同样要求（ApiSvc 层 :237-267）；`department_supervisor`/`branched_passage_manager` 后端按发起人上级自动定人（:231-235）；`form_person` 从表单字段取人（FD:845-872,940-983）；`initiator/assign` 服务端解析。条目形状 `{name, bizId, auditDetailType:'personnel', nodeProxyId}`（EEO:628-635）。
- **后端写入**：首节点待办（SubmitSvc:221）、下一节点审核人（:225）、审核记录（:229）、回调与流程线（:236-240）。
- **写后重读**：实例状态=run、首个待办、实际路径。工具已有真实运行证据（运行 13/31 到达业务层）。

## 3. 重新提交 resubmit —— ⚠️（载荷已实现，执行链不可达=返工缺口①）

- **入口**：待发列表 DueOut:87-103「重新发起」（来源 `POST /web/flowJobTaskLink/list` + `taskStatus:'waiting_send'`，DueOut:289-305）→ EEO:84-88（isReInitiate）。
- **前置钩子**：`handleRePostSubmit`（EEO:1335）：权限预检（EEO:1307-1333）→ `getData(true)` → 关联列表沿用原值去 commonFlow（EEO:1423-1454）→ `param{data:{name, id:flowInstanceId, formProxyId, flowInstanceBizRelevanceList}}`（EEO:1455-1466）→ **nextAuditorList 无条件映射**（EEO:1473-1477）→ `beforeSubmitAndDraft({reInit:true})`（EEO:1504-1507）→ `beforeSubmitAndDraftNoBiz`（EEO:1528）。
- **端点**：`POST /web/flowInstanceApi/reSubmit`（EEO:1542-1543；API:402）——**与 submit 不同端点**。
- **请求字段**：同上 + `formDataMongoVo{editData+value}` + `companyId`。
- **后端**：ReSubmitSvc:45-148：**状态必须 ∈ {rejected, withdraw, draft}，否则「当前实例状态不正确」**（:54-56）；新 batchNo（:166-167）、status=run（:173）、表单重存（:185-220）、新待办（:124）、审核记录（:127）、删旧流程线写新线（:139-141）。
- **注意**：待发列表的「保存草稿」仍走 `/submit` + `data.id`（EEO:1660,1702-1703），与 reSubmit 是两条链。
- **写后重读**：实例离开 rejected/withdraw/draft 进入 run、新批次、新待办。

## 4. 暂存表单 storage_form_data —— ⚠️（不变类判读=缺口⑤）

- **入口**：EEO:54-58「暂存」（审批人视角，非转发流程）→ `temporaryStorage()`（EEO:586）。
- **前置钩子**：`getValues()` 取当前表单值 → **`beforeSubmitAndDraft({temporary:true})`（EEO:605，与提交同名事件不同参数）**。
- **端点**：`POST /web/flowInstanceApi/storageFormData`（EEO:610-611；API:387）。
- **请求字段**：`data{id, currentNodeProxyId, auditRecord{executeDesc}}` + `formDataMongoVo.data = editData(实例原值)+value(当前值)`（EEO:592-603）。
- **后端**：InstSvc:997-1031：实例存在校验；**无权限/状态校验（仅会话）**；按 `(flowInstanceId, batchNo, createrId, nodeId)` upsert `FlowFormDataStorage`（:1011-1026）；回写 `currentDataId`（:1027-1029）；**不改实例状态、不生成待办；审批成功后暂存记录被删除**（AuditSvc:267）。
- **写后重读**：`POST /web/flowInstanceApi/queryStorageFormData`（EEO:477-481,549-569；API:388）可回读 `auditDesc`——工具的写后重读可直接用它判「检查点已更新」，比「实例不变」更强。

## 5. 加签 add_sign —— ⛔（工具实现与目标语义不符，需返工）

- **入口**：审批意见区「加签」→ 全屏流程设计器（EEO:62-64,544-547 → `AddCounterSign` 组件，有表单用 FormMulBranch、无表单用 NoFormMulBranch），先拉 `POST /web/flowProxy/findById` 作节点树底稿（AddSign:96-111）。
- **端点**：`POST /web/flowInstanceApi/updateFlowProxy`（API:750；WebCtrl:294 → Feign → WFCtrl:222-225 → InstSvc:1175-1221）。**不是 approverAppend。**
- **请求字段**：`{data:{id:flowInstanceId}, flowProxyProtocol:{data:<完整流程代理节点树>}}`（FMB:1047-1057）。**人员在节点树 `flowNodeAuditConfig.flowNodeDetailConfigList`，没有 `userIds` 字段。**
- **后端**：InstSvc:1175-1221：代理存在且未删（:1204-1207）；**所有权隔离——非本实例私有的代理不原地改，而是复制出新代理并把实例指向新 `flowProxyId`、置 `separationFlowProxyFlag=true`**（:1210-1218）。加签本身不产生 auditStatus。
- **响应**：`success(flowInstance)`，前端回填 `currentNodeProxyId`/`flowProxyId`（FMB:1060-1066）。
- **工具现状**：`catalog.go:94-108` 与 `write_actions.go:128-144` 把加签映射到 `approverAppend`+`approverAppendVo.userIds`——**与目标语义不符**（那是移交的端点）。返工任务书第 4 步实施加签时必须改为 updateFlowProxy + 整树载荷 + 所有权复制语义；当前映射不得登记为可运行。
- **写后重读**：实例私有流程代理、当前节点代理、待办任务映射（与 catalog 预期一致）。

## 6. 移交 transfer —— ⚠️（端点正确；batchNo/userIds 实时补齐=缺口②④）

- **入口**：待办行「移交」（Backlog:300-307，关联流程组件不显示）→ `clickHandOver`（Backlog:915-918）→ `PersonSelectDialog` 多选（Backlog:994-1000，空选提示「至少添加一位审批人」）。
- **端点**：`POST /web/flowInstanceApi/approverAppend`（API:404；ApiCtrl:132-135 → ApiSvc:1178-1205）。
- **请求字段**（Backlog:1011-1027，全部取自待办列表行）：`data{id, jobTaskId, batchNo, auditRecord{auditStatus:'transfer', executeDesc:'移交'}}` + `approverAppendVo{flowNodeProxyId, userIds[人员id数组]}`。
- **后端**：ApiSvc:1178-1205：待办不存在/已 done 拒绝（:1183-1188）；**用实例 batchNo 覆盖请求 batchNo**（:1199）；原任务置 done + 同节点新建 pending（新 jobTaskId=UUID，pid=原任务）（createRelevancePendingFlowDataReturn :591-609）；按 userIds 写移交记录、仅使「自己移交过」的旧记录失效（FlowInstanceApproverAppendServiceImpl:46-105）；写 `transfer` 审核记录（Enum:13）。
- **写后重读**：**原 jobTaskId 失效**，同节点出现新 pending 待办（新 jobTaskId）——工具必须在移交后重读任务映射，不能复用旧 jobTaskId 执行后续动作。
- **工具缺口**：`write_actions.go:142` 恒发空 `userIds`、`batchNo` 未承接（缺口②④），移交后任务重读未实现。

## 7. 同意 approve —— ⚠️（jobTaskId 现场补齐=缺口②；判读基线已对齐）

- **入口**：EEO:40-43 / EO:27「同意」；批量 Backlog:96→`auditBatchFlow`（Backlog:474-498）。
- **前置钩子顺序**：确认框（EEO:747-751）→ 意见表单校验（EEO:752）→ `getData(true)` 表单必填（EEO:896-897）→ 业务金额校验（EEO:925-956）→ 选人/分支 `nextAuditorList`（EEO:625-651,997-1004）→ **`beforeSubmitAndDraft`（仅 pass 触发，EEO:1034-1043）** → **`beforeSubmitAndDraftNoBiz`（EEO:1049-1050）** → 关联列表回填（EEO:1057-1076）。
- **端点**：`POST /flowInstanceApi/audit`（API:498，**无 /web 前缀**；网关另有 `/web/flowInstanceApi/audit`：WebCtrl:111-115 → Feign → WFCtrl:112-115 → AuditSvc.audit:112-328，`@Transactional`）。
- **请求字段**：`data{name, jobTaskId, id:flowInstanceId, auditRecord{auditStatus:'pass', executeDesc}, flowInstanceBizRelevanceList?}` + `formDataMongoVo.data`（无表单时 `{}`，EO:717-719）+ `nextAuditorList?` + `tracking`（EEO:978-992）。
- **后端校验链**：待办存在（:120-123）→ `taskStatus==done` 报「当前任务已处理」（:124-126）→ 非 pending 报「请刷新待办列表」（:127-129）→ Redis 锁 `userId_jobTaskId` 3 秒（:131-139）→ 行锁 `findByIdForUpdate`（:150-153）→ **处理人权限校验**（:159-163 → OperateSvc:475-492 按节点配置+batchNo+职务）→ 会签末人 `lastCountersignFlag`（:178-183）。
- **pass 写入**：递归下一节点/并行/条件（:390-460）、下节点待办（:717-723）、无下节点置 `status=end`（:690-694,726-729）、表单 mongo（:225）、当前待办 done（:230-232）、审核记录（:236）、实例 currentDataId/name（:244）、回调（:262）、推送（:318-320）、tracking（:310-316）。
- **写后重读**：待办列表、实例状态、当前节点、实际路径。

## 8. 不同意 reject —— ✅（判读已按预期效果修正）

- **入口**：EEO:51-53 / EO:29；批量 Backlog:98。
- **前置钩子**：确认框明示「终止，并驳回给发起人」（EEO:745）；意见必填（EEO:752-768）；`getData(false)` 不校验表单必填（EEO:896）；**跳过 `beforeSubmitAndDraft`**（仅 pass 触发，EEO:1036-1043）。
- **端点/请求**：同同意，仅 `auditRecord.auditStatus:'no_pass'`（Enum:11）；一般不带 nextAuditorList（EEO:765,997）。
- **后端**：AuditSvc:201-216：实例 `status=rejected`、`currentNodeProxyId=起始节点`（:203）；**其余并行待办全部置 done+isDelete**（:206-213）。
- **写后重读**：实例状态=rejected、待办清空、发起人重提状态。

## 9. 回退 rollback_previous —— ⚠️（缺口②）

- **入口**：待办行「回退上一节点」（Backlog:292-299,966-993）/ 审批弹窗内按钮（Mixin:51-88）/ 批量（Backlog:530-581）。
- **前置钩子**：仅确认框+意见；无表单钩子。
- **端点**：`POST /web/flowInstanceApi/rollBackThePreviousLevel`（API:403；链路 WebCtrl:155-159 → ApiSvc:1129-1132 → WFCtrl:239-243 → RollSvc:97-307）。
- **请求字段**：`data{id, jobTaskId, withdrawDesc?}`（Mixin:59-67）。**回退目标不在请求里**——后端按当前待办 `pid`（上一待办）推导（RollSvc:135-137）。
- **后端校验**：实例 `status==run`（:324-338）；待办未处理；`pid==null` 报「当前节点不支持回退」（:113-115）；上一节点是 start 报「请直接驳回给发起人」（:121-129）。写入：`auditStatus=roll_back_the_previous_level`（:143-147；Enum:12）、**换新 batchNo**（:150-152）、为上一节点重建待办（:246,259）、`currentNodeProxyId=上一节点`（:193,237）、当前待办 done、其余并行待办删除（:279-289）。
- **写后重读**：实例当前节点=上一节点、**新批次**、新待办及演员。

## 10. 取回 retrieve —— ⚠️（缺口②）

- **入口**：仅**已办列表**行内，条件 `flowStatus=='run'`（Finished:260-277,404-424）。
- **端点**：`POST /web/flowInstanceApi/retrieveProcess`（API:747；链路 WebCtrl:224-228 → InstSvc.saveRetrieveProcess:721-995）。
- **请求字段**：`data{jobTaskId, id}`（取自已办行）。
- **后端硬校验**（违例抛异常）：未完结非起始（:767-773）；该节点最近记录已是 `retrieve` 拒绝（:775-777）；**该节点最新批次办结记录必须含当前用户**（:784-786）；任务已办结（:788-790）；**下一节点待办仍为 pending**（:823-832，已处理不支持取回）；会签已有人 pass 拒绝（:793-800）。写入：新批次（:881）、给取回人建新 pending（:884）、`auditStatus=retrieve`（:890；Enum:15）、`currentNodeProxyId=取回节点`（:909）、删除后续待办（:904,910）。
- **写后重读**：批次、恢复节点、当前待办；流程重新出现在取回人待办。

## 11. 撤回 withdraw —— ✅

- **入口**：已发行内「撤销」，仅 `status=='run'`（Submitted:277-290,674-696）。
- **端点**：`POST /web/flowInstanceApi/revocation`（API:645；网关 `@FlowSubmitVerify(revocation)` WebCtrl:123-126 → ApiSvc:395-445）。
- **请求字段**：`data{id, withdrawDesc}`（Submitted:647-652，实际发空说明）。
- **后端校验**：`status` 必须 run（:400-402「当前流程不在运行中」）；**会话用户必须等于实例 createrId**（:403-405「非流程发起人不能撤销」）。写入：`status=withdraw`、`currentNodeProxyId=起始节点`（:409-413）；所有 pending 待办置 `withdraw` + 写 withdraw 审核记录（:415-428）；回调（:437）。
- **写后重读**：实例状态=withdraw、待办清空、创建人已发列表。

## 12. 催办 urge —— ⚠️（不变类判读=缺口⑤）

- **入口**：已发列表「催办」，仅 `status=='run'`（Submitted:262-275,717-733）。
- **端点**：`POST /web/urgeHandleRecord/sendUrgeMessage`，body `{flowInstanceId: row.id, data:{}}`（Submitted:719-722；链路 UrgeHandleRecordWebController:28-30 → ApiCtrl:32-35 → UrgeHandleRecordServiceImpl:140-175）。
- **后端**：查该实例全部 pending 任务（:142），无 pending 直接成功（:143-145）；对每个待办人写 `UrgeHandleRecord`（:157-170）并推送消息（:171）。**不改任务/实例状态。**
- **响应**：boolean 包装 success；前端 `res.isSuccess` → 本地 `row.urgeFlag=true`（Submitted:723-727），**不刷新列表**。`urgeFlag` 后端无生产者（java-serve 全量 grep 无命中）——「查看催办记录」基本只在本次会话可见。
- **写后重读**：实例/待办不变（不变类动作）；`POST /web/urgeHandleRecord/list`（Submitted:756-763）可查催办记录佐证。

## 13. 转发 forward —— ⚠️（辅助实例独立登记未实现）

- **入口**：待发/已发列表勾选一条 → `transpond(type)`（flowTypeMixin.js:59-125，强制单选；取 `row.batchNo`）→ 选人（IndicatorHeaderDialog，单人）→ `confirmTranspond`（:127-283）。
- **语义**：**不改原实例**——用「系统默认转发流程」模板 submit 一个全新实例，首节点审批人=被转发人，名称加「(由XX原发)」（InstSvc:1045-1090）。
- **端点**：`POST /web/flowInstanceApi/transpond`（API:386；InstSvc:1045-1090 内部转 `flowSubmit.submit`）。
- **请求字段**（flowTypeMixin.js:206-267）：`data{name=原名(由当前用户原发), flowInstanceBizRelevanceList[transpond_flowInstanceId, transpondFlow, transpond_formExist, transpond_auditWay, transpond_originalName, company]}` + 顶层 `receiverId`（单人 id）+ `formDataMongoVo.data`（原表单+转发附言/意见 dynamicParam）；附言另存 `savePostScript`（:285-300，用新实例 id）。
- **后端**：模板不存在抛「转发流程未配置」（:1046-1049）；构造 `nextAuditorList{personnel, bizId=receiverId, nodeProxyId=模板首节点}`（:1069-1081）+ `validatePermissionsFlag`（:1082）；**新实例生成新 instanceId+新 batchNo**（SubmitSvc:353-362）。
- **写后重读**：主实例不变；辅助实例存在且首待办=被转发人。工具必须把辅助实例与主实例分开登记（返工任务书第 4 步，未实现）；`receiverId` 当前只读动作参数、未接候选校验（缺口④）。

## 14. 关注 follow / 15. 取消关注 unfollow —— ⚠️（不变类判读=缺口⑤）

- **入口**：已办行「设为跟踪/取消跟踪」（Finished:278-301,381-402，仅 `flowStatus=='run'`）；查看弹窗底部按钮（EO:61-67 / EEO:96-102）；审批时勾选「跟踪此流程」随审核提交（EO:14-17,720）。
- **端点**：`POST /web/flowInstanceApi/flowTracking`（API:430；WebCtrl:306-309 → InstSvc:1224-1237）。**取消关注无独立端点**：同一端点 `tracking:false`。
- **请求字段**：`{data:{id:flowInstanceId}, tracking: true|false}`（Mixin:11-17）。
- **后端**：tracking 为 null 或 id 空报错（:1225-1230）；按 `(flowInstanceId,userId)` upsert `FlowTracking`，`"0"`=跟踪、`"1"`=取消（OperateSvc:1787-1802）；审批提交带 tracking 时在 audit 成功后同样保存（AuditSvc:310-316）；流程完结/驳回/回退向跟踪人推送消息（FlowTrackingMessageUtil:65-96）。
- **写后重读**：待办/已办列表行的 `tracking` 标志（FlowJobTaskLinkServiceImpl:240-246,302-309；发起人无记录默认 true :308）。

## 汇总：工具现状 × 目标真值 对照

| 动作 | 端点 | 载荷形状 | 执行链 | 人员/任务实时补齐 | 判读 |
| --- | --- | --- | --- | --- | --- |
| save_draft | /submit（status=draft） | ✅ | ✅ 已修（`06c744a`/`a5f325c`） | 不适用 | ✅ |
| submit | /submit | ✅ | ✅ | nextAuditorList 按审批方式（`e4a1841`） | ✅ |
| resubmit | /reSubmit | ✅ | ⛔ 缺口① | 同 submit 规则 | ⚠️ |
| storage_form_data | /storageFormData | ✅ | ⚠️ | 不需要 jobTaskId | ⚠️ 缺口⑤（可用 queryStorageFormData 加强） |
| add_sign | **updateFlowProxy（非 approverAppend）** | ⛔ 端点/载荷均不符 | ⛔ | 节点树人员配置 | ⛔ 需按第 5 节返工 |
| transfer | /approverAppend | ⚠️ 缺 batchNo | ⚠️ | ⛔ 空 userIds（缺口④） | ⛔ |
| approve | /flowInstanceApi/audit | ✅ | ⚠️ 缺口② | jobTaskId 待现场读取 | ⚠️ |
| reject | /flowInstanceApi/audit | ✅ | ⚠️ 缺口② | 同上 | ✅ 判读已修 |
| rollback | /rollBackThePreviousLevel | ✅ | ⚠️ 缺口② | jobTaskId；回退后新批次 | ⚠️ |
| retrieve | /retrieveProcess | ✅ | ⚠️ 缺口② | jobTaskId=已办键 | ⚠️ |
| withdraw | /revocation | ✅ | ✅ | 不适用 | ✅ |
| urge | /urgeHandleRecord/sendUrgeMessage | ✅ | ⚠️ | 不适用 | ⚠️ 缺口⑤ |
| forward | /transpond | ✅ | ⚠️ | receiverId 未接候选（缺口④） | ⛔ 辅助实例独立登记未实现 |
| follow/unfollow | /flowTracking | ✅ | ⚠️ | 不适用 | ⚠️ 缺口⑤ |

## 对返工执行顺序的三点修正建议

1. **加签不能在 approverAppend 上修**：目标语义是 updateFlowProxy+整树+所有权复制（InstSvc:1210-1218），与移交完全不同端点；第 4 步实施加签时按本表第 5 节重做适配层与目录端点声明。
2. **暂存表单的写后重读有真接口**：`/web/flowInstanceApi/queryStorageFormData` 回读 `auditDesc`，缺口⑤的「不变类」判读可用它升级为「检查点已更新」的确定判据。
3. **移交/回退/取回都换 batchNo、任务身份都变**：写后必须重读 `/web/flowJobTaskLink/list` 重建「实例×节点×处理人→jobTaskId/batchNo」映射，再执行后续动作；这正是返工第 3 步「实时任务读取」要落的位置。
