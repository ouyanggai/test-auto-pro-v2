# F-014 目标错误语义与幂等勘定

- 状态：accepted
- 产品依据：`docs/PRODUCT.md` 的产品原则第 2 条（工具问题与目标平台问题必须分开说明，暂时无法判断时按工具问题处理）与第 6 条（写操作结果不明确时先核对真实状态，禁止盲目重发）
- 架构依据：`docs/ARCHITECTURE.md` 的「系统边界」（`internal/adapter/target` 是唯一可直接调用目标平台的区域）与「参考源码边界」
- 纲领依据：`docs/EXECUTION_PROGRAM.md` 第 4.3 节阶段 6、第 4.4 节、第 8.2 节「错误语义」与「幂等与重复提交」两行、第 8.3 节、第 8.4 节、第 9 节 F-014 行、第 10 节的写端点白名单纪律
- 计划确认时间：2026-09-04（用户明确要求开始执行 F-014 任务，视为本文件记录的门禁所要求的明确批准）
- 前置条件：F-012 人工验收未通过，已由另一线程返工，状态为 `implementing`；F-013 停在 `ready_for_manual`，用户暂未发现问题但不视为明确验收。本切片纯只读、不发写请求，代码上只依赖已进入 `main` 的 F-013 日志底座；按用户裁决，须等 F-012 返工并通过人工验收后由用户明确批准，才能进入 `implementing`。

## 目标

在工具第一次向目标平台发出真实写请求之前，把「一次写请求的结果到底是成功、失败，还是不知道」这件事变成一条可编码、有目标源码证据、有对照测试锁定的规则，并把重复写在目标平台上的后果按证据强度分级勘定清楚。

产出物是 `docs/TARGET_SEMANTICS.md` 的前两条语义（错误语义、幂等与重复提交）加一个纯判定包与对照测试。本切片不实现执行器、不实现对账、不发任何写请求、不执行任何重复写探针。

## 为什么现在必须做这一项

纲领第 9 节已经排定：错误语义与幂等是目标语义清单里最危险的两条，必须早于第一次真实写。当前工作区的实际状况让这条排序不是形式主义：

- `internal/adapter/target/client.go:1314` 的 `responseError` 把目标返回的**所有**业务失败收敛成 `ErrorUnavailable`，对外文案是「目标平台暂时不可用」。这个收敛对只读功能是够用的，但如果写请求沿用它，「目标明确拒绝了这次审批」和「目标可能已经生效但我没看清」会变成同一个结论，安全重试就只能靠猜。
- 目标平台的失败响应里 `code` 可以是 `RESP200`：`GlobalExceptionHandler.java:48` 与 `:63` 把空指针和未知异常都渲染成 `code=RESP200` 且 `isSuccess=false`。任何按 `code` 判成功的实现都会把程序异常读成成功。
- 审批动作走的是 `/flowInstanceApi/audit`，与其余动作的 `/web/flowInstanceApi/*` 不是同一条路由族，经过的异常处理器与前置门禁不同。两族的失败形状不同，判定规则必须带上端点，不能只看文案。
- 目标平台没有幂等键。重复写会不会被拦住，取决于乐观锁与各接口的前置状态校验；这些分支在源码里可证明存在，但**运行时的实际反应必须等 F-016 首次真实写才能确认**。本切片把两类结论分开写清楚，F-018 的「允许重放同一步」才不会建立在没有依据的推断上。

## 单一用户结果

用户打开 `docs/TARGET_SEMANTICS.md`，能看到两张表。

第一张是错误语义：目标平台在什么情况下返回什么形状的失败、每种形状按「端点 + 精确文案」对应「确定失败」还是「不确定」、依据是目标源码的哪个文件哪一行。

第二张是幂等与重复提交：目标平台有没有幂等键、哪几道防线可能拦住重复写、每条结论标注证据强度是「源码可证明」还是「源码推断、待 F-016 实测」。凡未实测的条目一律按后者表述，不写成目标平台的真实反应。

两张表的每一条都能对应到 `test/` 下一个可重跑的用例；参考仓库改动后跑一次漂移检测就能知道哪一条证据失效了。

## 范围

### 包含

- 新增 `docs/TARGET_SEMANTICS.md`：语义清单载体。本切片只填「错误语义」与「幂等与重复提交」两条完整内容，其余 12 条只保留标题与「未开始」占位，不写任何行为定义（避免产生第二份权威文档）。
- 每条语义按纲领第 8.3 节的三件套落地：证据位置（参考仓库文件与符号，可定位到行）、对照测试引用、中文说明（解释规则与不可破坏的约束，不翻译代码）。
- 每条结论标注证据强度：`源码可证明`（分支、常量、注解在源码中可直接读到）或 `源码推断、待 F-016 实测`（需要真实写请求才能确认运行时行为）。禁止把后者表述为目标平台的真实反应。
- 错误语义勘定覆盖动作目录当前 15 条动作、11 个不同写端点（`internal/engine/actioncatalog/catalog.go`），逐条落：所属路由族、是否经过 `@FlowSubmitVerify`、是否声明 `@Consistency`、异常到响应形状的映射、以及该形状是否足以单独判定成败。
- 新增 `internal/engine/verdict` 纯判定包：把「本次动作与端点 + 传输结果 + HTTP 状态 + 响应包 + 事实重读结论」映射为三值（确定成功 / 确定失败 / 不确定）并给出中文原因与依据。无 IO、无目标调用、无数据库、无 API 路由、无前端改动，本切片不接入任何执行流程，只被测试调用。
- 判定包必须覆盖 `AUTH_401`：把它与 `RESP401`、HTTP 401 一并识别为鉴权拒绝，并有用例锁定。
- 对照测试：判定表单元测试、真实目标环境的只读错误语义集成测试、写端点白名单为空的契约断言、纲领第 8.4 节要求的证据漂移检测脚本、`test/run-f014.sh` 汇总入口。
- 幂等勘定的结论分级与风险清单，包含明确写死的编码约束（例如写请求不得携带 `batchCode`，理由见下节）。
- 重复写实测探针清单：写清将来在 F-016 用什么最小步骤确认哪一条结论、期望观察什么。本切片**只产出并检查清单是否完整**，不执行任何探针。

### 不包含

- 任何目标写请求。本切片写端点白名单为空，测试断言实际发出的写请求集合为空集，沿用 F-013 的断言形式。
- 执行重复写探针，包括自动化、人工浏览器操作和 `curl.log` 重放。真实验证全部推迟到 F-016，F-016 仍是第一次真实写。
- 修改 `responseSessionExpired` 漏认 `AUTH_401` 的现状，以及任何其它现有只读路径行为与界面文案。该差异只作为已知差异写入语义文档；是否修改现有读路径另行立项。
- 修改 `responseError` 现有的只读路径错误收敛。本切片只记录「写判定不得复用它」。
- 对账实现、恢复动作、界面按钮、`recovery.log` 写入。属于 F-018。
- 执行器七阶段、运行记录表、`run_step_attempts`、`step.log`。属于 F-015 与 F-016。
- 语义清单其余 12 条的内容（条件求值、演员解析、回退取回、转发、附件富文本等）。
- 数据库结构、迁移、API 路由、前端页面的任何改动。

## 已读到的初步证据

以下为撰写本计划时已在参考仓库中直接读到的事实，属于 `源码可证明` 一类，实施阶段需逐条复核、补全行号并写入 `docs/TARGET_SEMANTICS.md`。列在这里是为了让批准范围时能看到结论的分量，不是最终交付内容。

### 两条路由族与异常映射

| 路由族 | 使用者 | 异常处理器 | 失败形状 |
| --- | --- | --- | --- |
| `/web/flowInstanceApi/*` | 提交、重新发起、暂存、加签移交、回退、取回、撤销、转发、催办、关注 | `GlobalExceptionHandler`（`WebApiApplication.java:29` 显式 `@Import`） | HTTP 200 + `isSuccess=false`；`BusinessException` 走 `:76` 得 `code=ERROR_99999`，空指针走 `:48`、未知异常走 `:63` 得 `code=RESP200` |
| `/flowInstanceApi/audit` | 同意、不同意 | 同一个 `GlobalExceptionHandler`（`RshCloudApiApplication.java:23` 组合注解导入，`WorkflowCenterApiApplication` 用该注解） | 同上；但**不经过** `@FlowSubmitVerify`，因为该注解只声明在 web 层控制器上 |

- 审批走无 `/web` 前缀的路径不是工具的臆造：真实前端 `参考代码/rsh-cloud-invest-power-system/src/api/index.js:498` 就是 `submitTask: '/flowInstanceApi/audit'`，与动作目录 `catalog.go:127`、`:143` 一致。
- 更下游的中心服务（`rsh-cloud-workflow-center`）用的是另一个处理器 `CenterExceptionHandler`：`BusinessException` 被渲染成 HTTP 500 + 响应头 `code=exception_500` + URL 编码的 `errorMsg`（`:86`-`:97`）。api 层通过 `InvokeCenterFeignErrorDecoder:16`-`:21` 把 `errorMsg` 头解码还原成 `BusinessException`，于是最终又变回 HTTP 200 的失败包。这条链决定了「中心侧业务拒绝」在工具眼里长什么样，T02 必须确认没有 500 直接穿透到工具。
- 会话失效有三种表现：HTTP 401、`code=RESP401`（`ProtocolCode.java:10`）、`code=AUTH_401`（`:199`）。真实前端 `utils/axios.js:186`-`:187` 三者并列处理，且 `:226`-`:231` 用 `res.isSuccess` 作为唯一成功判据。工具当前 `client.go:1309` 的 `responseSucceeded` 只看 `isSuccess`/`success`，方向正确；`responseSessionExpired` 只认 `RESP401` 与 `-1`，漏认 `AUTH_401`。**本切片不修改它**，只把该差异写入语义文档，并要求新判定包与用例覆盖 `AUTH_401`。

### 三值判定的关键陷阱

- `code=RESP200` 不代表成功，`isSuccess` 才是权威。判定实现必须禁止用 `code` 判成败。
- 业务拒绝不等于「暂时不可用」。`responseError` 的收敛不可用于写判定。
- `@FlowSubmitVerify` 的拒绝发生在 `joinPoint.proceed()` 之前（`FlowSubmitVerifyAspect.java:104` 早于 `:109`），所以该族拒绝可判「确定失败、无副作用」。但同一个切面在 `:115` 会把 `proceed()` 抛出的非业务异常也包成 `isSuccess=false` 的响应，这类响应发生在写之后，只能判「不确定」。两者形状相似、含义相反，是本切片最容易出错的地方，也是判定必须带上端点与精确文案的原因。
- 审批路径同时写关系库流程数据与 Mongo 表单数据（`FlowInstanceServiceImpl.audit` 调 `formDataService.saveFormData`），跨存储不原子，「部分生效」在真实运行中是可能的。这是「不确定必须靠事实重读、且重读只覆盖流程侧事实」的根本原因，也是不可解释失败即使重读显示流程未变也仍判「不确定」的原因。

### 幂等现状

按证据强度分开表述。

`源码可证明`：

- 目标平台没有幂等键。写接口参数里没有任何客户端可控的去重标识。
- `@Consistency` 不是幂等机制，是批次补偿：`ConsistencyInterceptor` 只在请求带 `batchCode` 时生效（`:104`），失败时会调用注解声明的 `deleteMethodName` 回滚同批次已登记数据（`:130`、`:143`）。**结论：工具的写请求一律不携带 `batchCode`，否则一次失败可能触发目标平台的额外删除写入。** 当前 Go 侧确认未使用该字段（动作目录里的 `batchNo` 是另一个业务字段）。
- 源码中存在两道可能拦住重复写的防线：`FlowInstance` 的 `@Version` 乐观锁（`entity/FlowInstance.java:31`）经 `saveAndFlushWithOptimisticLockMessage` 转成固定中文提示「流程状态已发生变化，请刷新后重试」（`FlowInstanceServiceImpl.java:70`、`:526`）；以及各写接口在写事务前抛出的状态校验异常（如「该待办记录不存在」、「流程已完结,不支持取回」、「起始节点,不支持取回」）。
- 浏览器侧另有 `utils/RequestQueue.js` 级别的重复请求抑制，属于前端行为，不构成服务端保证，工具不能依赖。

`源码推断、待 F-016 实测`：

- 重复提交同一动作时这两道防线是否真的命中、命中时返回哪一条精确文案、返回的是 `ERROR_99999` 还是 `code=RESP200` 的异常包。
- 乐观锁失败时是否已经有其它存储（Mongo 表单数据）完成写入，即该场景能否判「确定失败」。T02 必须先用源码确认写入顺序与事务边界；确认不成立时，判定矩阵中该行降级为「不确定」。
- 各动作重复写的实际后果分级（会被拦住 / 会写第二次）。

## 参考仓库同步影响（2026-09-04）

用户告知后台有更新，已用 `make refs-sync` 按清单分支同步 13/13。与本切片相关的三个仓库当前 HEAD：`rsh-cloud-workflow-center` `master` `37c01d04`、`rsh-cloud-workflow-center-api` `master` `088aed79`、`rsh-framework-all` `test` `84bb1973`。

用户随后裁决把 `rsh-cloud-workflow-center` 的锚定分支由 `test` 改为 `master`：清单已改，旧克隆经洁净核对后移除并按 `master` 干净克隆，未使用 reset 或 checkout。切换时 `master`（`37c01d04`）相对原 `test`（`0c6c7f0e`）只多一个把 test 合入 master 的合并提交，`git diff` 内容差异为空，因此本节结论与全部引用证据不受分支切换影响，已在新 HEAD 上逐条复核通过。`rsh-flow-components` 按裁决保持 `test`。

本次同步用事实证明了第 8.4 节漂移检测的必要性：一次同步就动了四处与本切片证据直接相关的符号。

### 与本切片直接相关

- **审批方式动态化（`feature/dynamic-audit-way`，三个仓库联动）**：`auditWay` 不再受 `AuditWayEnum` 约束。`FlowSubmitVerifyBaseController` 的路由表由 `Map<AuditWayEnum, …>` 改为 `Map<String, …>`，`doVerify` 不再调用 `AuditWayEnum.valueOf`；未注册的 `auditWay` 现在返回 `BaseResponseProtocol("未发现实例", false, null)`，旧版本则抛 `IllegalArgumentException`。同一场景在新旧部署上分别落「前置拒绝」和「不可解释失败」，所以每条证据必须同时记录源码版本与目标平台部署版本。
- **门禁会静默放行**：`FlowSubmitVerifyAspect` 改用 `RshRedisServer.hashGetNormalizedOwner(REDIS_HASH_KEY, auditWay)`，并新增两条放行分支——`auditWay` 未注册校验服务、已注册但无健康实例，都直接放行且只写一行日志。工具不能把「没有被拒绝」当成「通过了业务校验」。
- **`AuditWayEnum` 新增 8 项**：供应商入库、年度评估、清退、工商信息变更、黑名单重新入库、股转款登记、合同补充协议变更、合同终止。
- **目标库结构迁移**：`sql/20260828_flow_trigger_audit_way_ordinal_migration.sql` 把 `flow_trigger_config_relevance.audit_way` 由数字 ordinal 改为 `VARCHAR(100)` 编码名，附 `20260828_audit_way_dynamic_precheck.sql` 预检。脚本要求先停旧版服务，因此目标环境是否已执行必须实测确认，不能从源码推断。
- **写路径本身没有改动**：`FlowInstanceApiServiceImpl` 的 5 处改动全部落在 `list` 与 `getUserMap`（第 815 行之后），`submit`、`audit`、`reSubmit` 未被触及；`FlowInstanceServiceImpl` 只改了一行 `auditWay` 取值，乐观锁与 `CONCURRENT_UPDATE_MESSAGE` 证据仍在 `:70`、`:526`。
- **已逐条复核未变的证据**（最近一次复核在 `rsh-cloud-workflow-center` 切到 `master` `37c01d04` 之后）：`GlobalExceptionHandler:48/63/73/79`、`ProtocolCode:10/199`、`CenterExceptionHandler:86/90`、`InvokeCenterFeignErrorDecoder:16/21`、`RshCloudApiApplication:23`、`WebApiApplication:29`、`ConsistencyInterceptor:104/130/143`、`FlowSubmitVerifyAspect:104/109/115`、`FlowInstance.java:31`、`FlowInstanceServiceImpl:70/526`，以及跨存储写入点 `FlowInstanceServiceImpl:584`。只有 `api/index.js` 的 `submitTask` 由 `:495` 移到 `:498`，本文件已更正。
- **新增可用证据**：`GlobalExceptionHandler:79` 的 `FlowBusinessException` 与 `BusinessException` 同形状，而 `FlowProxyServiceImpl.findById` 缺代理 ID 时抛的正是它，属于工具已在调用的读路径。

### 记入其它切片，本切片只记录不改动

- 待办列表（`taskStatus=pending`）不再查询流程实例，返回记录不带实例运行态字段。工具当前只用 `waiting_send`，暂不受影响；F-015/F-016 定义事实重读时必须单独读实例，不能指望待办列表带状态。
- 已发/待发列表的当前处理人只对 `status=run` 的实例解析，终态实例不再返回 `currentAuditUserNames`。
- 流程图详情改走无 Redis 的快速查询链路（`FlowProxyServiceImpl`、`FlowTemplateServiceImpl`、`FlowTemplateFastTreeAssembler`），源码声明契约不变，需要一次只读回归确认。
- `auditWay` 迁移未在目标环境执行前，工具读到的可能仍是数字，而 `internal/service/history_target.go:419` 用它作 `vue_custom` 页面键。

以上四条属于 F-012 返工线程或 F-015 的检查范围。

## 三值判定规则

三步求值，首个命中即为结论。判定输入只有五项，全部是可观测事实，不含工具推断：

1. 本次动作与目标端点：动作目录中的动作标识与 `targetOperation`。判定必须带上它，禁止只按全局文案匹配。
2. 传输结果：请求是否建立连接、是否收到完整响应、是否超时或被中断、进程是否在此期间崩溃。
3. HTTP 状态码。
4. 响应包：`isSuccess`、`code`、`message`。
5. 事实重读结论：`verify` 阶段重读实例状态、当前节点、当前待办、演员与已办后，与配置期望比较得到的四值之一——`已前进`、`明确未变`、`无法读取`、`自相矛盾`。

### 第一步：传输层

| 条件 | 结论 | 依据 |
| --- | --- | --- |
| 连接建立阶段即失败，且可明确识别（连接被拒、DNS 解析失败） | 确定失败，无副作用 | 请求未到达目标进程 |
| 超时、连接中断、`context` 取消、进程崩溃 | 不确定 | 写可能已在目标侧发生，响应丢失不等于写没发生 |
| 收到完整响应 | 进入第二步 | —— |

识别不出属于哪一类时按「不确定」处理，不做乐观归类。

### 第二步：响应侧初判

| 初判 | 条件 |
| --- | --- |
| 成功声明 | HTTP 200 且 `isSuccess=true` |
| 鉴权拒绝 | HTTP 401，或 `code` 为 `RESP401`、`AUTH_401` |
| 前置拒绝 | `isSuccess=false` 且「端点 + 精确文案」命中前置拒绝清单 |
| 乐观锁冲突 | `isSuccess=false` 且「端点 + 精确文案」命中乐观锁提示 |
| 不可解释失败 | 其余 `isSuccess=false`（含 `code=RESP200` 的异常包、清单外文案、中心侧异常还原包）、HTTP 5xx、响应体不可解析或超长 |

清单匹配规则：按「端点 + 文案全等」匹配，禁止模糊匹配、关键字包含或跨端点复用文案。清单必须来自目标源码枚举，清单外的失败一律落「不可解释失败」。

### 第三步：与事实重读结论组合

| 响应侧初判 | 重读=已前进 | 重读=明确未变 | 重读=无法读取 | 重读=自相矛盾 |
| --- | --- | --- | --- | --- |
| 成功声明 | 确定成功 | 不确定 | 不确定 | 不确定 |
| 鉴权拒绝 | 不确定 | 确定失败，无副作用 | 不确定 | 不确定 |
| 前置拒绝 | 不确定 | 确定失败，无副作用 | 不确定 | 不确定 |
| 乐观锁冲突 | 不确定 | **不确定**（T02 源码确认后的结论，见下） | 不确定 | 不确定 |
| 不可解释失败 | 不确定 | 不确定 | 不确定 | 不确定 |

矩阵之外的任何组合、任何两项输入互相矛盾、任何新出现的响应形状，一律判「不确定」。这是兜底规则，不允许被后续切片放宽。

几条必须写进说明的理由：

- 「成功声明 + 明确未变」是响应与事实冲突，不是成功，也不能算失败。
- 「鉴权拒绝 / 前置拒绝 + 已前进」说明这次拒绝与观察到的推进不是同一件事，可能是上一次不确定写已生效，只能判「不确定」。
- 「不可解释失败 + 明确未变」仍判「不确定」，因为重读只覆盖流程侧事实，跨存储的部分生效证明不了不存在。
- 「乐观锁冲突 + 明确未变」原计划判「确定失败」，前提是乐观锁失败前没有其它已提交存储写入。T02 已用源码确认该前提**不成立**：审批端点最终进入中心 `FlowAuditServiceImpl.audit`，该方法虽带 `@Transactional`，但方法体内先写 Mongo 表单数据（`:225`）后更新关系库并触发乐观锁检查，Mongo 写入不受该事务约束。因此该格按计划的降级条款改判「不确定」，语义清单第 1.6 节、`internal/engine/verdict` 与状态记录三处一致。

必须写进代码的三条硬约束：

- 禁止用 `code` 判成功，只认 `isSuccess`。
- 禁止把业务拒绝并入「暂时不可用」；写判定不复用 `responseError`。
- 写请求禁止携带 `batchCode`。

## 详细执行任务

每个任务独立验证并单独提交，提交说明用中文。

### T01：语义清单载体

新增 `docs/TARGET_SEMANTICS.md`：说明用途、与其他文档的关系、条目状态取值、证据格式、证据强度取值（`源码可证明` / `源码推断、待 F-016 实测`），以及供漂移检测解析的机器可读证据块格式。14 条语义只建立标题，本切片只推进两条，其余标注「未开始」。

### T02：错误语义证据勘定

按动作目录 15 条动作、11 个不同写端点逐条落表：路由族、异常处理器、是否经过 `@FlowSubmitVerify`、是否声明 `@Consistency`、失败形状、可判定性。补齐 `ProtocolCode`、`GlobalExceptionHandler`、`CenterExceptionHandler`、`InvokeCenterFeignErrorDecoder`、`FlowSubmitVerifyAspect` 的行级证据与中文说明。同时确认审批路径中流程侧与表单侧的写入顺序与事务边界，据此确定判定矩阵「乐观锁冲突 + 明确未变」一格是「确定失败」还是降级为「不确定」。把 `responseSessionExpired` 漏认 `AUTH_401` 记为已知差异，写明本切片不修改、影响范围与后续另行立项。每条证据必须同时记录三个仓库的源码 HEAD 与该结论对应的目标平台部署版本；源码可证明与部署已验证是两件事，不得混写。补记本次同步暴露的两条新事实：`FlowSubmitVerifyAspect` 在 `auditWay` 未注册或无健康实例时静默放行（工具不得把未被拒绝当成通过校验），以及 `FlowBusinessException` 与 `BusinessException` 同形状且出现在工具已调用的读路径上。

### T03：前置拒绝清单

从目标源码枚举各写接口在写事务开始前抛出的 `BusinessException` 文案，按「端点 + 精确文案」成对登记，逐条标注所属接口与代码位置。无法确认发生在写之前的文案不进清单，落「不可解释失败」并在文档中说明原因。清单不做跨端点合并，同一文案出现在不同端点时分别登记。审批方式动态化后新增的 `未发现实例`（提交校验服务未注册该 `auditWay` 时由 `FlowSubmitVerifyBaseController.doVerify` 返回）必须登记，并标注它依赖动态注册状态、旧部署上同场景会变成不可解释失败。

### T04：幂等与重复提交勘定

写清「没有幂等键」的证据、`@Consistency` 的真实语义与 `batchCode` 禁令、两道防线的源码位置，以及跨存储非原子导致的部分生效风险。每条结论标注证据强度；凡属运行时行为的结论一律标 `源码推断、待 F-016 实测`，不表述为目标平台的真实反应。产出重复写实测探针清单：每条待实测结论对应一条探针，写明前置条件、最小步骤、期望观察点和将由 F-016 哪一步覆盖。本切片不执行探针。探针清单必须包含一条确认目标环境的动态审批方式注册状态与 `flow_trigger_config_relevance.audit_way` 迁移是否已执行，因为前置状态校验这道防线是否触发取决于它。

### T05：三值判定包

新增 `internal/engine/verdict`：三值枚举、五项判定输入结构（含动作与端点）、四值重读结论、纯判定函数、兜底分支、中文原因与依据输出。前置拒绝清单与乐观锁文案都以「端点 + 精确文案」为键，全等匹配。乐观锁端点清单只登记能证明会走到中心 `FlowInstanceServiceImpl.update` 或 `.save` 的端点（`/web/flowInstanceApi/submit`、`/flowInstanceApi/audit`）；未登记端点返回同一文案时落不可解释失败，两者结论都是不确定，登记与否只影响原因说明，不会让结论变乐观。鉴权拒绝识别覆盖 HTTP 401、`RESP401`、`AUTH_401`。所有具名函数与方法写中文注释，说明职责与不可破坏的约束；单文件不超过 800 行；不引入 IO、不引入目标调用、不接入任何现有流程。

### T06：判定表对照测试

新增 `test/unit/backend/target_semantics/`：覆盖第一步三条、第二步五类初判、第三步矩阵全部 20 格，以及兜底分支（未知响应形状、输入互相矛盾）。样本分两类，来源必须在用例与文档中标明：

- 只读路径样本：由 T07 从真实目标抓取，存 `test/fixtures/f014/readonly/`。
- 写路径样本（前置拒绝文案、乐观锁提示、跨存储异常包、`AUTH_401`）：本切片不发写请求，无法真实抓取，因此按 T02、T03 的源码证据构造，存 `test/fixtures/f014/from-source/`，每个文件标注对应源码位置与「待 F-016 首次真实写复核或替换」。

### T07：真实环境只读集成测试

新增 `test/integration/f014_error_semantics_readonly_test.go`：连真实目标，用只读接口验证成功包形状、无效参数触发的业务失败包形状、缺失或失效会话触发的会话失效形状、极短超时触发的超时分类，并确认判定包对这些真实响应的分类与文档一致。抓到的响应写入只读 fixture 目录。另加两条只读回归：确认 `/web/flowTemplateApi/findById` 与 `/web/flowProxy/findById` 在目标改走无 Redis 快速查询链路后响应契约未变；确认目标返回的 `auditWay` 是编码名还是数字 ordinal，据此判定目标环境是否已执行 `20260828` 迁移，并把结论写进语义文档的部署版本一栏。用例不得静默跳过，缺配置直接失败。

### T08：写端点白名单与漂移检测

新增 `test/contracts/f014/target_write_whitelist.sh`（沿用 F-013 形式，断言本切片写端点集合为空）与 `test/contracts/f014/semantics_evidence_drift.sh`（解析 `docs/TARGET_SEMANTICS.md` 的证据块，断言引用的类、方法、常量与关键分支数量在参考仓库中仍存在）；新增 `test/run-f014.sh` 汇总编译、静态检查、单测、只读集成测试与两个契约脚本，并检查每条 `源码推断、待 F-016 实测` 结论都配有探针条目、每个源码构造样本都带来源标注。漂移检测除了断言引用符号存在，还必须固定断言本次同步已证明会变动的几处：`FlowSubmitVerifyBaseController.REDIS_HASH_KEY`、`RshRedisServer.hashGetNormalizedOwner`、`GlobalExceptionHandler` 的异常处理器数量、`AuditWayEnum` 的枚举项数，并把三个仓库的 HEAD 记入检测输出，便于对照证据失效范围。

## 完成标准

- [x] `docs/TARGET_SEMANTICS.md` 的错误语义与幂等两条同时具备证据位置、对照测试引用与中文说明，每条结论标注证据强度，其余 12 条无内容占位。
- [x] 幂等一节没有把未实测结论表述为目标平台的真实反应；每条 `源码推断、待 F-016 实测` 结论都有对应探针条目，探针清单完整性由脚本检查。
- [x] 三值判定规则已编码为 `internal/engine/verdict` 的纯函数：五项输入含动作与端点，前置拒绝与乐观锁文案按「端点 + 精确文案」全等匹配，鉴权拒绝覆盖 `AUTH_401`，矩阵 20 格与兜底分支全部有用例。
- [x] 未覆盖组合与矛盾输入一律判「不确定」，并有用例证明。
- [x] `test/run-f014.sh` 在本机实际跑通：编译、`go vet`、单测、真实目标只读集成测试全部通过且无跳过用例。
- [x] 写端点白名单为空的契约断言通过；本切片未发出任何写请求，未执行任何重复写探针；证据漂移检测脚本在当前参考仓库 HEAD 下通过。
- [x] `responseSessionExpired` 漏认 `AUTH_401` 已作为已知差异写入语义文档，现有读路径未被修改。
- [x] 每条证据同时记录源码 HEAD 与目标平台部署版本；凡依赖部署状态的结论（动态审批方式注册、`audit_way` 迁移）已实测确认或明确标注未确认。
- [x] 漂移检测固定覆盖本次同步已证明会变动的符号，并输出三个仓库的 HEAD。
- [x] 已检查同一范围内的相似问题：动作目录路由族与真实前端是否一致、是否还有其它按 `code` 判成败的位置，结论写入文档，不在本切片扩张改动。
- [x] 文档状态更新为 `ready_for_manual`，`docs/PROGRESS.md` 与 `docs/ROADMAP.md` 同步。
- [x] 已列出用户手工核对步骤。

## 状态记录

- 2026-09-04 `preparing` -> `awaiting_approval`：用户明确要求开始 F-014。已读 `CONTEXT.md`、`AGENTS.md`、`docs/EXECUTION_PROGRAM.md` 第 4、8、9、10 节与 F-013 功能文档，并在参考仓库中完成初步证据勘定，产出本范围等待用户批准。
- 2026-09-04 计划修订（状态保持 `awaiting_approval`）：用户裁决两处边界并要求修正三点，已全部落入本文件——
  1. F-014 保持纯只读，重复写探针只产出清单、不自动也不人工执行，真实验证推迟到 F-016；未实测结论一律标 `源码推断、待 F-016 实测`，不表述为真实反应。
  2. 不修改 `responseSessionExpired`，漏认 `AUTH_401` 作为已知差异记录，新判定包与用例必须覆盖 `AUTH_401`；现有读路径是否修改另行立项。
  3. 判定输入增加动作与端点，前置拒绝清单改为「端点 + 精确文案」全等匹配。
  4. 判定矩阵补齐重读「无法读取」「自相矛盾」与响应冲突组合，未覆盖或矛盾一律「不确定」，并写为不可放宽的兜底规则。
  5. 写路径样本改为按源码证据构造并标注待 F-016 复核或替换，只读样本才要求真实抓取。
- 2026-09-04 门禁：用户明确 F-012 人工验收未通过（已由另一线程返工，状态 `implementing`），F-013 暂不视为明确验收；F-014 继续停在 `awaiting_approval`，等 F-012 返工通过人工验收后由用户明确批准才进入 `implementing`。
- 2026-09-04 锚定分支调整（状态保持 `awaiting_approval`）：按用户裁决把 `rsh-cloud-workflow-center` 由 `test` 改为 `master`，`make refs-sync` 与 `make refs-status` 均通过（13/13 正常）；切换时两分支内容完全一致，证据已在 `master` `37c01d04` 上复核，本切片范围、任务与判定规则不变。`rsh-flow-components` 保持 `test`。
- 2026-09-04 参考仓库同步（状态保持 `awaiting_approval`）：按用户要求执行 `make refs-sync`（13/13 快进），核对 `rsh-cloud-workflow-center`、`rsh-cloud-workflow-center-api`、`rsh-framework-all` 的新提交。结论：写路径未变，本切片方向不变；审批方式动态化改变了前置门禁的失败形状与放行条件，已按此调整 T02、T03、T04、T07、T08 与完成标准，并新增「参考仓库同步影响」一节。四条属于 F-012 返工或 F-015 的连带影响只记录、不在本切片改动。

- 2026-09-04 `awaiting_approval` -> `implementing`：用户明确要求开始执行 F-014 任务，按本文件门禁条目视为明确批准。
  需要说明的偏差：门禁原文要求「等 F-012 返工通过人工验收后」再进入 `implementing`，而 F-012 与 F-013 目前都停在
  `ready_for_manual`，尚未被标记为 `accepted`。本切片纯只读、不发写请求、不接入任何现有执行流程，
  代码上只依赖已进入 `main` 的 F-013 日志底座，因此按用户指令开始实施，并把这条偏差如实记录在此。
  开工时参考仓库 HEAD 与计划记录一致：`rsh-cloud-workflow-center` `master` `37c01d04`、
  `rsh-cloud-workflow-center-api` `master` `088aed79`、`rsh-framework-all` `test` `84bb1973`（`make refs-status` 13/13 正常）。

- 2026-09-04 `implementing` -> `ready_for_manual`：T01 至 T08 全部实施完成，`./test/run-f014.sh` 本机实跑通过。
  交付与实测要点：
  - `docs/TARGET_SEMANTICS.md` 建立 14 条标题，只填错误语义与幂等两条，20 条机器可读证据块在当前 HEAD 下全部校验通过。
  - 按计划要求做的源码确认改写了一处待定结论：审批端点最终进入中心 `FlowAuditServiceImpl.audit`，
    该方法虽带 `@Transactional`，但先写 Mongo 表单数据（`:225`）后写关系库并触发乐观锁检查，
    Mongo 写入不受该事务约束，因此判定矩阵「乐观锁冲突 + 明确未变」由「确定失败」降级为「不确定」。
  - `internal/engine/verdict` 是纯判定，未接入任何现有流程；20 格矩阵、五类初判、三条传输分支、
    八种兜底分支、`AUTH_401`、端点绑定的文案全等匹配、`batchCode` 禁令都有用例。
  - 真实目标只读实测取得两条部署事实：`auditWay` 返回字符串编码名，说明目标环境已执行 20260828 迁移；
    失效会话真实响应是 HTTP 200 + `code=RESP401`，说明会话失效不一定伴随 HTTP 401。两条已写进语义清单。
  - 六条待实测结论全部标注「源码推断、待 F-016 实测」并配同编号探针，本切片未执行任何探针，
    未发出任何写请求，写端点白名单为空且 `batchCode` 禁令在位。
  - 现有读路径未被改动：`responseSessionExpired` 漏认 `AUTH_401` 与 `responseError` 的收敛只作为已知差异记录。
  只读集成测试的不稳定已定位：失败信息是 `dial tcp 192.168.1.220:38081: connect: operation timed out`，
  即目标平台在那几分钟内 TCP 连接超时；同一时段 `curl` 直连也复现，恢复后连接耗时回到 11 毫秒，
  随后连续两次全量通过。这是共享内网目标的可用性抖动，不是用例缺陷。已做两处减少放大的调整：
  客户端超时改为沿用配置值（默认 120 秒，原先写死 30 秒），并明确每个用例各自登录而不共用会话
  （测试账号是共享账号，目标平台在同账号别处登录时会让旧会话失效，共用长会话反而会让后面的用例
  偶发「会话已失效」）。按计划要求这类用例不得静默跳过，因此没有加重试掩盖目标抖动。
  待人工验收；F-015 未开始。

- 2026-09-04 复审修复（状态保持 `ready_for_manual`）：一个 P1、四个 P2 全部修复。
  1. **P1 判定器会把矛盾包与非预期状态码判成确定失败**。原实现先认鉴权码再校验形状，
     于是 `isSuccess=true + code=AUTH_401` 会被识别为鉴权拒绝，HTTP 3xx/4xx 的失败包也能命中前置拒绝清单，
     两者配合「明确未变」都会得出「确定失败、无副作用」，违背兜底规则。
     修复：`classifyResponse` 改为形状优先——先认 HTTP 401（它本身就是完整信号），
     再要求响应可解析且 `isSuccess` **显式存在**（新增 `Response.IsSuccessPresent`，缺字段落不可解释失败），
     再要求除 401 外只接受 HTTP 200，然后才进鉴权码与文案清单；声明成功却带鉴权码、乐观锁提示或
     该端点前置拒绝文案的包一律判自相矛盾。新增矛盾包专项用例与 3xx/4xx 失败包用例，
     两处修复都做了变异验证——把鉴权识别提前或去掉状态码门，用例分别报出 `confirmed_failure`。
  2. **P2 本文件判定矩阵与实现口径不一致**：矩阵仍写「乐观锁冲突 + 明确未变 = 确定失败」，
     而语义清单、实现与状态记录已改为「不确定」。已修正矩阵与其前提说明，四处口径统一。
  3. **P2 证据未逐条绑定源码 HEAD 与部署版本**：每条证据块新增 `head=<仓库键>@<HEAD>` 与 `deployment=`，
     漂移脚本改为逐条比对 `head` 与该仓库当前 HEAD（不一致即判需重新勘定，已变异验证），
     并要求 `deployment` 显式登记；部署提交号未取得的条目写明「未取得（目标平台不提供版本接口）」，
     依赖部署状态的三条写入已实测结论。全局基线不再充当逐条绑定。
  4. **P2 只读辅助把 `isSuccess` 与旧字段 `success` 做 OR**，与「只认 isSuccess」的硬约束冲突。
     已改为只解析 `isSuccess`（指针解析以区分缺字段），并在注释里说明别名兼容不属于 F-014 范围。
  5. **P2 乐观锁未按「端点 + 精确文案」匹配**：已登记端点清单，只收录能证明会走到中心
     `FlowInstanceServiceImpl.update`/`.save` 的两个端点（`/web/flowInstanceApi/submit`、`/flowInstanceApi/audit`），
     未登记端点返回同一文案落不可解释失败。清单刻意保守，两种结论都是不确定，不会让结论变乐观。
     语义清单第 1.8 节补上可达性证据与不登记的理由。

- 2026-09-04 `ready_for_manual` -> `accepted`：用户明确「F-14 也可以标记完成了」，本切片验收通过。
  交付以 `docs/TARGET_SEMANTICS.md` 的错误语义与幂等两条、纯判定包 `internal/engine/verdict`、
  真实目标只读对照测试与两个契约脚本为准；写路径样本仍按源码构造，待 F-016 首次真实写复核或替换。

## 人工验收

1. 打开 `docs/TARGET_SEMANTICS.md`，确认错误语义与幂等两条能读懂：每条都说清了规则、为什么这样处理、哪一行代码支持它，以及证据强度是「源码可证明」还是「源码推断、待 F-016 实测」；确认其余 12 条确实是空占位，没有第二份行为定义。
2. 随机挑三条 `源码可证明` 的证据，按文档给出的路径与符号回到 `参考代码/` 自行核对是否存在且含义一致。
3. 检查幂等一节的措辞：凡未实测的结论都标了待实测，没有任何一句把推断写成目标平台的真实反应。
4. 检查重复写实测探针清单是否完整：每条待实测结论都有对应探针，每条探针写明前置条件、最小步骤、期望观察点和由 F-016 哪一步覆盖。**本切片不要求执行任何探针**，只看清单本身。
5. 在项目根执行 `./test/run-f014.sh`，确认全部通过、没有跳过用例，并确认输出包含「写端点白名单为空」与探针清单完整性检查两项。
6. 打开 `logs/plans/` 下本次集成测试产生的目录，确认只读请求的 `network.log` 与 `curl.log` 有真实记录，且没有任何写端点出现。
7. 确认 `internal/adapter/target` 的现有读路径没有被改动（含 `responseSessionExpired` 与 `responseError`），语义文档里 `AUTH_401` 已记为已知差异。
