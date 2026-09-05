# 目标平台语义清单

本文件是本工具对目标平台行为的唯一权威结论集。行为定义只写在这里，代码与测试引用这里，不在别处产生第二份定义。

- 载体建立时间：2026-09-04（F-014）
- 本切片只填第 1 条「错误语义」与第 2 条「幂等与重复提交」，其余 12 条只建立标题并标注「未开始」。

## 用途与边界

- 这里写的是**目标平台的行为**，不是本工具的实现。工具实现要服从这里的结论。
- 每条结论必须给出证据位置、对照测试引用与中文说明。说明解释规则和不可破坏的约束，不翻译代码。
- 目标平台没有对外契约文档，所有结论只能来自参考仓库源码与真实环境实测。两者是不同强度的证据，必须分开表述。

## 与其他文档的关系

| 文档 | 分工 |
| --- | --- |
| `docs/PRODUCT.md` | 产品原则。第 2 条要求工具问题与目标平台问题分开说明，第 6 条要求写结果不明确时先核对真实状态。本文件是这两条的技术落地依据。 |
| `docs/ARCHITECTURE.md` | 系统边界。`internal/adapter/target` 是唯一可直接调用目标平台的区域。 |
| `docs/EXECUTION_PROGRAM.md` | 第 8 节列出 14 条语义清单条目与三件套要求，第 8.4 节要求证据漂移检测。 |
| 本文件 | 语义结论本体。 |

## 条目状态与证据格式

条目状态取值：`未开始`、`勘定中`、`已勘定`。

证据强度取值，只有两个：

| 取值 | 含义 |
| --- | --- |
| `源码可证明` | 分支、常量、注解在参考仓库源码中可直接读到，结论不依赖运行时观察。 |
| `源码推断、待 F-016 实测` | 结论涉及运行时实际反应，源码只能给出可能性。**禁止把这类结论表述为目标平台的真实反应。** |

机器可读证据块供 `test/contracts/f014/semantics_evidence_drift.sh` 解析，格式固定为：

```text
（格式示例，用 text 围栏以免被漂移检测当成真实证据）
file=参考代码/<仓库>/<文件路径>
line=<行号>
contains=<该文件中必须存在的字符串>
strength=源码可证明
head=<仓库键>@<该结论勘定时的仓库 HEAD>
deployment=<该结论对应的目标平台部署版本，未取得就写「未取得」并说明原因>
```

真实证据块用 `evidence` 围栏，字段与上面一致。

`line` 只作人工定位用，漂移检测按 `contains` 判断证据是否还在；`file` 不存在或 `contains` 消失即判漂移。

`head` 与 `deployment` 是逐条绑定的，不靠本文件的全局基线代替：`head` 由漂移检测与该仓库当前 HEAD 逐条比对，对不上即说明这条证据是在另一个版本上勘定的，必须重新核对；`deployment` 记录该结论对应的目标平台部署版本，目标平台不提供版本接口，因此绝大多数条目写「未取得」，只有被只读探测确认过的条目写实测结论。

## 证据基线

源码可证明与部署已验证是两件事，任何结论都必须同时说明两者。

| 项 | 值 | 取得方式 |
| --- | --- | --- |
| `rsh-cloud-workflow-center` | `master` `37c01d04eb10` | `make refs-status` 2026-09-04 |
| `rsh-cloud-workflow-center-api` | `master` `088aed79ad0b` | 同上 |
| `rsh-cloud-web-api` | `master` `16410b5e7315` | 同上 |
| `rsh-framework-all` | `test` `84bb19736a8a` | 同上 |
| `rsh-cloud-invest-power-system` | `test` `8a00cb9995df` | 同上 |
| 目标平台部署版本 | **提交号未取得**（目标平台不提供版本接口），但已由只读探测取得两条部署事实，见下 | `test/integration/f014_error_semantics_readonly_test.go` 2026-09-04 实跑 |

部署版本未取得的后果，必须在读本文件时始终记住：

- 凡结论依赖「某段源码已经上线」，都只能标注部署未确认。
- 已知两处结论强依赖部署状态：动态审批方式的注册情况、`flow_trigger_config_relevance.audit_way` 迁移是否执行。

只读探测已取得的部署事实（2026-09-04 实跑，样本存 `test/fixtures/f014/readonly/`）：

| 事实 | 观测结果 | 含义 |
| --- | --- | --- |
| 流程模板详情返回的 `auditWay` | 字符串编码名 | 目标环境**已执行** `20260828_flow_trigger_audit_way_ordinal_migration.sql`。因此审批方式动态化那一版源码已上线，第 1.4 节的静默放行分支与第 1.7 节的 `未发现实例` 对当前环境有效。 |
| 失效会话的真实响应 | HTTP **200** + `code=RESP401` + `message=SID已失效!` | 会话失效不一定伴随 HTTP 401，判定必须按 `code` 识别。三种形状中 `RESP401` 已实测确认。 |
| 只读业务失败的真实响应 | HTTP 200 + `code=ERROR_99999` + 业务文案 | 与第 1.2 节一致；该文案不在前置拒绝清单内，判定落不可解释失败，符合预期。 |

动态审批方式的**注册情况**仍未确认：只读接口无法观察 Redis 注册表，需要 F-016 的探针 P5 覆盖。

## 语义清单总览

| 序号 | 条目 | 状态 |
| --- | --- | --- |
| 1 | 错误语义 | 已勘定（F-014） |
| 2 | 幂等与重复提交 | 已勘定（F-014） |
| 3 | 条件求值 | 未开始 |
| 4 | 演员解析 | 未开始 |
| 5 | 回退语义 | 未开始 |
| 6 | 取回语义 | 未开始 |
| 7 | 转发语义 | 未开始 |
| 8 | 加签与移交语义 | 未开始 |
| 9 | 会签与并行语义 | 未开始 |
| 10 | 附件与富文本 | 未开始 |
| 11 | 表单权限与可见性 | 未开始 |
| 12 | 子流程语义 | 未开始 |
| 13 | 超时与跳过语义 | 未开始 |
| 14 | 通知与催办语义 | 未开始 |

## 1. 错误语义

状态：已勘定（F-014）。对照测试：`test/unit/backend/target_semantics/`、`test/integration/f014_error_semantics_readonly_test.go`。

### 1.1 两条路由族与门禁

本工具动作目录共 15 条动作、11 个不同写端点（`internal/engine/actioncatalog/catalog.go`）。它们分属两条路由族，经过的门禁不同，因此失败形状不同。

| 动作 | 动作标识 | 目标端点 | 路由族 | `@FlowSubmitVerify` | `@Consistency` |
| --- | --- | --- | --- | --- | --- |
| 保存草稿 | `save_draft` | `/web/flowInstanceApi/submit` | web | 无 | **有** |
| 提交 | `submit` | `/web/flowInstanceApi/submit` | web | 无 | **有** |
| 重新提交 | `resubmit` | `/web/flowInstanceApi/reSubmit` | web | 无 | 无 |
| 暂存当前表单 | `storage_form_data` | `/web/flowInstanceApi/storageFormData` | web | 无 | 无 |
| 加签 | `add_sign` | `/web/flowInstanceApi/approverAppend` | web | 无 | 无 |
| 移交 | `transfer` | `/web/flowInstanceApi/approverAppend` | web | 无 | 无 |
| 同意 | `approve` | `/flowInstanceApi/audit` | 中心 api | 无 | 无 |
| 不同意 | `reject` | `/flowInstanceApi/audit` | 中心 api | 无 | 无 |
| 回退上一节点 | `rollback_previous` | `/web/flowInstanceApi/rollBackThePreviousLevel` | web | **有** | 无 |
| 取回 | `retrieve` | `/web/flowInstanceApi/retrieveProcess` | web | **有** | 无 |
| 撤回 | `withdraw` | `/web/flowInstanceApi/revocation` | web | **有** | 无 |
| 催办 | `urge` | `/web/urgeHandleRecord/sendUrgeMessage` | web | 无 | 无 |
| 转发 | `forward` | `/web/flowInstanceApi/transpond` | web | 无 | 无 |
| 关注 | `follow` | `/web/flowInstanceApi/flowTracking` | web | 无 | 无 |
| 取消关注 | `unfollow` | `/web/flowInstanceApi/flowTracking` | web | 无 | 无 |

两条族的归属证据：

```evidence
file=参考代码/java-serve/rsh-cloud-web-api/src/main/java/com/rsh/cloud/web/api/controller/ops/flow/FlowInstanceWebController.java
line=38
contains=@RequestMapping("/web/flowInstanceApi")
strength=源码可证明
head=java-serve/rsh-cloud-web-api@16410b5e7315
deployment=未取得（目标平台不提供版本接口）
```

```evidence
file=参考代码/java-serve/rsh-cloud-workflow-center-api/src/main/java/com/rsh/cloud/workflow/center/api/controller/FlowInstanceApiController.java
line=70
contains=@PostMapping("/audit")
strength=源码可证明
head=java-serve/rsh-cloud-workflow-center-api@088aed79ad0b
deployment=未取得（目标平台不提供版本接口）
```

必须写清的三件事：

1. **同意与不同意不经过 `@FlowSubmitVerify`。** 工具用的是无 `/web` 前缀的 `/flowInstanceApi/audit`，该控制器方法只有 `@ParaCheck({})`。`/web/flowInstanceApi/audit` 确实带 `@FlowSubmitVerify(submitType = audit)`，但那是另一个端点，工具不调用它。真实前端也走无前缀路径（`api/index.js:498` `submitTask: '/flowInstanceApi/audit'`）。
2. **`/web/flowInstanceApi/v1` 是另一个控制器**（`FlowInstanceWebControllerV1`，`@RequestMapping("/web/flowInstanceApi/v1")`），带自己的 `@Consistency`。工具不调用 v1 路径，勘定时不要把两个控制器的注解混读。
3. **只有 submit 带 `@Consistency`**，且 `deleteMethodName = "delete"`。这是第 2 章 `batchCode` 禁令的直接来源。

```evidence
file=参考代码/java-serve/rsh-cloud-web-api/src/main/java/com/rsh/cloud/web/api/controller/ops/flow/FlowInstanceWebController.java
line=71
contains=deleteMethodName = "delete"
strength=源码可证明
head=java-serve/rsh-cloud-web-api@16410b5e7315
deployment=未取得（目标平台不提供版本接口）
```

### 1.2 异常到失败形状的映射

两条族最终都由同一个 `GlobalExceptionHandler` 渲染失败响应（web 层 `WebApiApplication` 与中心 api 层 `RshCloudApiApplication` 都导入它），所以失败形状一致：**HTTP 200 + `isSuccess=false`**。区别只在 `code` 与 `message`。

| 目标侧异常 | 渲染结果 | 行号 |
| --- | --- | --- |
| `BusinessException` | `code` 取异常自带值，为空时 `ERROR_99999`；`message` 为异常文案 | `:76` |
| `FlowBusinessException` | 与 `BusinessException` **完全同形状** | `:82` |
| `NullPointerException` | `code=RESP200`，`message=发生空指针异常` | `:48` |
| 其它任何 `Exception` | `code=RESP200`，`message` 取 `e.getMessage()`，为空时才是 `程序异常` | `:63` |
| `EntityNullException` / `NotAuthException` | `code` 与 `message` 均取异常自带的 `ProtocolCode` | `:86` `:92` |

```evidence
file=参考代码/rsh-framework-all/rsh-framework-cloud-server/src/com/rsh/framework/cloud/server/GlobalExceptionHandler.java
line=48
contains=return new ExceptionResponseProtocol("发生空指针异常", false, null, ProtocolCode.RESP200);
strength=源码可证明
head=rsh-framework-all@84bb19736a8a
deployment=未取得（目标平台不提供版本接口）
```

```evidence
file=参考代码/rsh-framework-all/rsh-framework-cloud-server/src/com/rsh/framework/cloud/server/GlobalExceptionHandler.java
line=63
contains=!StringUtils.isEmpty(e.getMessage()) ? e.getMessage() : "程序异常"
strength=源码可证明
head=rsh-framework-all@84bb19736a8a
deployment=未取得（目标平台不提供版本接口）
```

这张表带出四条不可绕过的判定约束：

- **`code=RESP200` 不代表成功。** 它是程序异常的渲染结果。判定实现禁止用 `code` 判成败，只认 `isSuccess`。
- **`code=RESP200` 的 `message` 可以是任意 Java 异常文案。**（`:63` 直接取 `e.getMessage()`）所以「文案看起来像业务提示」不能作为业务拒绝的证据。
- **`ERROR_99999` 也不证明是业务拒绝。** 中心侧的空指针与未知异常经过 `CenterExceptionHandler` 写入 `errorMsg` 响应头，再由 `InvokeCenterFeignErrorDecoder` 还原成 `BusinessException`，最终同样渲染为 `ERROR_99999`。因此 `ERROR_99999` 同时覆盖「中心业务拒绝」和「中心程序崩溃」两种含义。
- **`FlowBusinessException` 与 `BusinessException` 同形状**，且它出现在工具已经在调用的读路径上（`FlowProxyServiceImpl.findById` 缺代理 ID 时抛它）。

中心侧到 api 层的还原链：

```evidence
file=参考代码/rsh-framework-all/rsh-framework-cloud-server/src/com/rsh/framework/cloud/server/CenterExceptionHandler.java
line=90
contains=response.addHeader("code", "exception_500");
strength=源码可证明
head=rsh-framework-all@84bb19736a8a
deployment=未取得（目标平台不提供版本接口）
```

```evidence
file=参考代码/rsh-framework-all/rsh-framework-cloud-server/src/com/rsh/framework/cloud/server/InvokeCenterFeignErrorDecoder.java
line=21
contains=return new BusinessException(msg);
strength=源码可证明
head=rsh-framework-all@84bb19736a8a
deployment=未取得（目标平台不提供版本接口）
```

结论：中心侧的 HTTP 500 不会穿透到工具，工具看到的仍是 HTTP 200 的失败包。**但这条链把中心程序异常伪装成了业务异常**，这是「不可解释失败」这一类必须存在的根本原因。

### 1.3 会话失效的三种形状

| 形状 | 证据 |
| --- | --- |
| HTTP 401 | 真实前端 `utils/axios.js:186` 起三者并列处理 |
| `code=RESP401`（`SID已失效!`） | `ProtocolCode.java:10` |
| `code=AUTH_401`（`当前登录用户会话过期或在其他设备登录，请重新登录`） | `ProtocolCode.java:199` |

```evidence
file=参考代码/rsh-framework-all/rsh-framework-core/src/com/rsh/framework/core/protocol/ProtocolCode.java
line=199
contains=AUTH_401("当前登录用户会话过期或在其他设备登录，请重新登录")
strength=源码可证明
head=rsh-framework-all@84bb19736a8a
deployment=未取得（目标平台不提供版本接口）
```

### 1.4 `@FlowSubmitVerify` 的两种相反含义

同一个切面会产出两种形状相似、含义相反的失败响应。这是本条语义最容易判错的地方。

| 分支 | 位置 | 发生时机 | 判定含义 |
| --- | --- | --- | --- |
| 校验服务返回不通过，直接返回该响应 | `:105`，早于 `:109` 的 `joinPoint.proceed()` | **写之前** | 确定失败，无副作用 |
| `proceed()` 抛异常，被包成 `isSuccess=false` | `:115` | **写之后** | 不确定 |

```evidence
file=参考代码/rsh-framework-all/rsh-framework-cloud-server/src/com/rsh/framework/cloud/flow/FlowSubmitVerifyAspect.java
line=109
contains=return joinPoint.proceed();
strength=源码可证明
head=rsh-framework-all@84bb19736a8a
deployment=未取得（目标平台不提供版本接口）
```

```evidence
file=参考代码/rsh-framework-all/rsh-framework-cloud-server/src/com/rsh/framework/cloud/flow/FlowSubmitVerifyAspect.java
line=115
contains=return new ExceptionResponseProtocol(throwable.getMessage(), false, null);
strength=源码可证明
head=rsh-framework-all@84bb19736a8a
deployment=未取得（目标平台不提供版本接口）
```

**门禁会静默放行。** 切面改用 Redis 查注册表后新增两条放行分支：`auditWay` 未注册校验服务（`:142` 只写一行 info 日志后放行）、已注册但无健康实例（`:153` 只写一行 warn 日志后放行）。

```evidence
file=参考代码/rsh-framework-all/rsh-framework-cloud-server/src/com/rsh/framework/cloud/flow/FlowSubmitVerifyAspect.java
line=139
contains=redisServer.hashGetNormalizedOwner(FlowSubmitVerifyBaseController.REDIS_HASH_KEY
strength=源码可证明
head=rsh-framework-all@84bb19736a8a
deployment=已实测：auditWay 返回字符串编码名，说明 20260828 迁移已执行；注册状态仍未取得
```

**工具不得把「没有被拒绝」当成「通过了业务校验」。** 这条与判定无关（放行不产生失败响应），但直接影响 F-016 之后对「提交成功」的解读：成功可能只是因为校验服务不在线。

未注册时校验服务侧的返回：

```evidence
file=参考代码/rsh-framework-all/rsh-framework-cloud-server/src/com/rsh/framework/cloud/flow/FlowSubmitVerifyBaseController.java
line=52
contains=return new BaseResponseProtocol("未发现实例",false,null);
strength=源码可证明
head=rsh-framework-all@84bb19736a8a
deployment=已实测：auditWay 返回字符串编码名，说明 20260828 迁移已执行；注册状态仍未取得
```

### 1.5 已知差异与部署依赖

| 项 | 结论 | 强度 |
| --- | --- | --- |
| 工具 `responseSessionExpired` 漏认 `AUTH_401` | `internal/adapter/target/client.go` 的会话失效识别只认 `RESP401` 与 `-1`，不认 `AUTH_401`。**本切片不修改**，只记录。影响范围：目标以 `AUTH_401` 拒绝时，现有只读路径会把它归入一般失败而不是会话失效，因此不会触发自动重登。是否修改现有读路径另行立项。新判定包已覆盖 `AUTH_401`。 | `源码可证明` |
| 工具 `responseError` 把所有业务失败收敛为「目标平台暂时不可用」 | 对只读功能够用，**写判定不得复用它**。`internal/engine/verdict` 不引用 `internal/adapter/target` 的任何错误收敛。 | `源码可证明` |
| 动态审批方式注册状态 | 决定 `@FlowSubmitVerify` 是拒绝、放行还是返回「未发现实例」。**部署状态未确认**，由只读集成测试间接探测。 | `源码推断、待 F-016 实测` |
| `flow_trigger_config_relevance.audit_way` 迁移 | 迁移把该列由数字 ordinal 改为编码名。目标返回编码名说明已执行，返回数字说明未执行。只读集成测试按目标实际返回值给出结论。 | `源码推断、待 F-016 实测` |

同范围内的相似问题检查结论（不在本切片改动）：

- 动作目录的路由族与真实前端一致，逐条核对了 11 个端点，只有审批走无 `/web` 前缀，与 `api/index.js` 相符。
- 工具当前没有其它按 `code` 判成败的位置：`responseSucceeded` 只看 `isSuccess`/`success`，方向正确。

### 1.6 三值判定规则

规则已编码为 `internal/engine/verdict`，对照测试 `test/unit/backend/target_semantics/verdict_matrix_test.go`。三步求值，首个命中即为结论。判定输入只有五项，全部是可观测事实：动作与端点、传输结果、HTTP 状态码、响应包（`isSuccess`/`code`/`message`）、事实重读结论。

第一步，传输层：

| 条件 | 结论 |
| --- | --- |
| 连接建立阶段即失败且可明确识别（连接被拒、DNS 解析失败） | 确定失败，无副作用 |
| 超时、连接中断、`context` 取消、进程崩溃 | 不确定 |
| 收到完整响应 | 进入第二步 |

识别不出属于哪一类时按「不确定」处理，不做乐观归类。

第二步，响应侧初判：

| 初判 | 条件 |
| --- | --- |
| 成功声明 | HTTP 200 且 `isSuccess=true` |
| 鉴权拒绝 | HTTP 401，或 `code` 为 `RESP401`、`AUTH_401` |
| 前置拒绝 | `isSuccess=false` 且「端点 + 精确文案」命中第 1.7 节清单 |
| 乐观锁冲突 | `isSuccess=false` 且「端点 + 精确文案」命中乐观锁提示 |
| 不可解释失败 | 其余 `isSuccess=false`（含 `code=RESP200` 异常包、清单外文案、中心侧异常还原包）、HTTP 5xx、响应体不可解析或超长 |

清单匹配按「端点 + 文案全等」，禁止模糊匹配、关键字包含或跨端点复用文案。

**校验顺序不可调换**，`internal/engine/verdict/classify.go` 按此实现：

1. HTTP 401 先认掉——它本身就是完整的鉴权拒绝信号，这类响应通常没有业务包络。
2. 响应包必须可解析，且 `isSuccess` 字段必须**显式存在**。缺字段与 `isSuccess=false` 含义不同，
   缺字段说明成功判据不存在，只能落不可解释失败，不允许用 `code` 或文案补判。
3. 除 401 外只接受 HTTP 200 的业务包络。3xx、4xx、5xx 一律按新出现的形状处理，**不进文案清单**，
   否则「4xx + 清单文案 + 重读明确未变」会得出「确定失败、无副作用」。
4. 声明成功的包若同时带着鉴权错误码、乐观锁提示或该端点登记过的前置拒绝文案，
   属响应包自相矛盾，落不可解释失败，绝不能识别成拒绝——那三类配合「明确未变」会得出确定失败。
5. 以上都通过后，才按鉴权码与文案清单收敛。

第三步，与事实重读结论组合：

| 响应侧初判 | 重读=已前进 | 重读=明确未变 | 重读=无法读取 | 重读=自相矛盾 |
| --- | --- | --- | --- | --- |
| 成功声明 | 确定成功 | 不确定 | 不确定 | 不确定 |
| 鉴权拒绝 | 不确定 | 确定失败，无副作用 | 不确定 | 不确定 |
| 前置拒绝 | 不确定 | 确定失败，无副作用 | 不确定 | 不确定 |
| 乐观锁冲突 | 不确定 | **不确定**（见下） | 不确定 | 不确定 |
| 不可解释失败 | 不确定 | 不确定 | 不确定 | 不确定 |

矩阵之外的任何组合、任何两项输入互相矛盾、任何新出现的响应形状，一律判「不确定」。这是兜底规则，不允许被后续切片放宽。

**「乐观锁冲突 + 明确未变」按计划要求做了源码确认，结论是降级为「不确定」，不是「确定失败」。** 依据：审批端点 `/flowInstanceApi/audit` 最终进入中心 `FlowAuditServiceImpl.audit`，该方法带 `@Transactional`，但方法体内先写 Mongo 表单数据（`:225` `flowOperate.saveFormData`），之后才走到关系库实例更新并触发乐观锁检查。Mongo 写入不在 JPA 事务内，乐观锁失败回滚关系库时不会回滚它。因此「流程侧明确未变」证明不了「什么都没写」。

```evidence
file=参考代码/java-serve/rsh-cloud-workflow-center/src/main/java/com/rsh/cloud/workflow/center/service/impl/FlowAuditServiceImpl.java
line=112
contains=@Transactional(rollbackFor = Exception.class, isolation = Isolation.READ_COMMITTED)
strength=源码可证明
head=java-serve/rsh-cloud-workflow-center@37c01d04eb10
deployment=未取得（目标平台不提供版本接口）
```

```evidence
file=参考代码/java-serve/rsh-cloud-workflow-center/src/main/java/com/rsh/cloud/workflow/center/service/impl/FlowAuditServiceImpl.java
line=225
contains=flowOperate.saveFormData(requestProtocol, flowInstanceVo, flowProxyVo)
strength=源码可证明
head=java-serve/rsh-cloud-workflow-center@37c01d04eb10
deployment=未取得（目标平台不提供版本接口）
```

其余几格的理由：

- 「成功声明 + 明确未变」是响应与事实冲突，既不是成功也不能算失败。
- 「鉴权拒绝 / 前置拒绝 + 已前进」说明这次拒绝与观察到的推进不是同一件事，可能是上一次不确定写已生效。
- 「不可解释失败 + 明确未变」仍判「不确定」，因为重读只覆盖流程侧事实，跨存储的部分生效证明不了不存在。

写进代码的三条硬约束：禁止用 `code` 判成功；禁止把业务拒绝并入「暂时不可用」；写请求禁止携带 `batchCode`。

### 1.7 前置拒绝清单

只登记能证明发生在任何写操作之前的失败文案，按「端点 + 精确文案」成对登记，不做跨端点合并。无法确认发生在写之前的文案不进清单，一律落「不可解释失败」。

| 端点 | 精确文案 | 位置 | 说明 |
| --- | --- | --- | --- |
| `/web/flowInstanceApi/revocation` | `当前实例不存在` | `FlowInstanceApiServiceImpl:398` | 守卫子句，早于任何 feign 写调用 |
| `/web/flowInstanceApi/revocation` | `当前流程不在运行中,无法撤销` | `:401` | 同上 |
| `/web/flowInstanceApi/revocation` | `非流程发起人不能撤销` | `:404` | 同上 |
| `/web/flowInstanceApi/approverAppend` | `该待办记录不存在` | `:1184` | 守卫子句，早于 `updateFlowJobTaskLinkStatusDone` |
| `/web/flowInstanceApi/approverAppend` | `当前任务已处理` | `:1187` | 同上 |
| `/web/flowInstanceApi/retrieveProcess` | `该待办记录不存在` | 中心 `FlowInstanceServiceImpl:730` | 在 `saveRetrieveProcess` 事务内、任何写之前抛出 |
| `/web/flowInstanceApi/retrieveProcess` | `流程已完结,不支持取回` | `:768` | 同上 |
| `/web/flowInstanceApi/retrieveProcess` | `起始节点,不支持取回` | `:772` | 同上 |
| `/web/flowInstanceApi/retrieveProcess` | `当前已办任务, 不支持取回` | `:776` | 同上，注意文案中逗号后有一个空格 |
| `/web/flowInstanceApi/retrieveProcess` | `当前环节不支持取回` | `:782` | 同上 |
| `/flowInstanceApi/audit` | `该待办记录不存在` | 中心 `FlowAuditServiceImpl:122` | 在 `audit` 方法首段、写 Mongo 之前返回 |
| 全部带 `@FlowSubmitVerify` 的端点 | `未发现实例` | `FlowSubmitVerifyBaseController:52` | 依赖动态注册状态；旧部署上同场景抛 `IllegalArgumentException`，会变成不可解释失败 |

```evidence
file=参考代码/java-serve/rsh-cloud-workflow-center/src/main/java/com/rsh/cloud/workflow/center/service/impl/FlowInstanceServiceImpl.java
line=768
contains=throw new BusinessException("流程已完结,不支持取回");
strength=源码可证明
head=java-serve/rsh-cloud-workflow-center@37c01d04eb10
deployment=未取得（目标平台不提供版本接口）
```

```evidence
file=参考代码/java-serve/rsh-cloud-workflow-center-api/src/main/java/com/rsh/cloud/workflow/center/api/service/impl/FlowInstanceApiServiceImpl.java
line=1184
contains=return BaseResponseProtocol.error("该待办记录不存在");
strength=源码可证明
head=java-serve/rsh-cloud-workflow-center-api@088aed79ad0b
deployment=未取得（目标平台不提供版本接口）
```

明确**不进**清单的两条，理由是它们发生在写之后：

- `/web/flowInstanceApi/retrieveProcess` 的 `失败`（`FlowInstanceApiServiceImpl:1143`）：feign 返回 false 时的兜底文案，写已经尝试过。
- `/web/flowInstanceApi/storageFormData` 的 `暂存失败`（`:1149`）：同上。

### 1.8 乐观锁提示

乐观锁提示与前置拒绝清单一样按「端点 + 精确文案」全等匹配。端点清单只登记能证明会走到
中心 `FlowInstanceServiceImpl.update`（`:426`）或 `.save`（`:541`）的端点，两者内部都调
`saveAndFlushWithOptimisticLockMessage`。

| 端点 | 精确文案 | 可达性证据 |
| --- | --- | --- |
| `/web/flowInstanceApi/submit` | `流程状态已发生变化，请刷新后重试` | 中心 `FlowSubmitServiceImpl:320` 调 `flowInstanceService.save` |
| `/flowInstanceApi/audit` | 同上 | 中心 `FlowAuditServiceImpl` 经 `updateFLowInstance:348` 调 `flowInstanceService.update` |

清单刻意保守：`reSubmit`、`approverAppend`、`retrieveProcess` 等端点没有在源码里读到直达
中心实例保存的调用（`retrieveProcess` 用的是 `findByIdForUpdate` 悲观锁，不是乐观锁那条路径），
因此不登记。未登记端点返回同一文案时落不可解释失败——两者结论都是「不确定」，
登记与否只影响原因说明，不会让结论变乐观。

```evidence
file=参考代码/java-serve/rsh-cloud-workflow-center/src/main/java/com/rsh/cloud/workflow/center/service/impl/FlowInstanceServiceImpl.java
line=526
contains=throw new BusinessException(CONCURRENT_UPDATE_MESSAGE);
strength=源码可证明
head=java-serve/rsh-cloud-workflow-center@37c01d04eb10
deployment=未取得（目标平台不提供版本接口）
```

命中该文案说明目标检测到并发更新，本次关系库写入被回滚。但按第 1.6 节的源码确认，它**不足以判定「什么都没写」**，因此仍需事实重读，且与「明确未变」组合后结论是「不确定」。

## 2. 幂等与重复提交

状态：已勘定（F-014）。对照测试：`test/unit/backend/target_semantics/idempotency_constraints_test.go`。

本章的结论按强度严格分开。凡涉及「重复写时目标平台实际怎么反应」的结论，一律标 `源码推断、待 F-016 实测`，**不表述为目标平台的真实反应**。

### 2.1 目标平台没有幂等键

| 结论 | 强度 |
| --- | --- |
| 目标平台的写接口参数里没有任何客户端可控的去重标识。工具不能靠传一个键让目标自己去重。 | `源码可证明` |

依据：动作目录 15 条动作的参数集合（`internal/engine/actioncatalog/catalog.go`）逐条对照目标 `FlowInstanceProtocol` 与各 `Vo`，没有幂等键字段。因此**重复写的防护责任全部在工具侧**，这是 F-016 必须先有运行记录与检查点的原因。

### 2.2 `@Consistency` 不是幂等机制，`batchCode` 是禁令

| 结论 | 强度 |
| --- | --- |
| `@Consistency` 是批次补偿机制，只在请求带 `batchCode` 时生效；生效后一旦失败会调用注解声明的 `deleteMethodName` 回滚同批次已登记数据。 | `源码可证明` |
| **工具的写请求一律不得携带 `batchCode`。** 否则一次失败可能触发目标平台的额外删除写入，把「一次失败的提交」放大成「删掉了别的数据」。 | `源码可证明` |

```evidence
file=参考代码/rsh-framework-all/rsh-framework-cloud-server/src/com/rsh/framework/cloud/server/ConsistencyInterceptor.java
line=104
contains=result instanceof BaseResponseProtocol && !StringUtils.isEmpty(batchCode)
strength=源码可证明
head=rsh-framework-all@84bb19736a8a
deployment=未取得（目标平台不提供版本接口）
```

```evidence
file=参考代码/rsh-framework-all/rsh-framework-cloud-server/src/com/rsh/framework/cloud/server/ConsistencyInterceptor.java
line=130
contains=this.invokeRollBackMethod(batchCode, req);
strength=源码可证明
head=rsh-framework-all@84bb19736a8a
deployment=未取得（目标平台不提供版本接口）
```

`/web/flowInstanceApi/submit` 是 11 个端点中唯一带 `@Consistency` 的，其 `deleteMethodName = "delete"`，即回滚动作是删除流程实例。禁令由 `test/unit/backend/target_semantics/idempotency_constraints_test.go` 与 `test/contracts/f014/target_write_whitelist.sh` 双向锁定。

当前 Go 侧确认未使用该字段。动作目录里的 `batchNo` 是另一个业务字段，与 `batchCode` 无关。

### 2.3 两道可能拦住重复写的防线

| 防线 | 源码位置 | 结论 | 强度 |
| --- | --- | --- | --- |
| 流程实例乐观锁 | `FlowInstance.java:31` 的 `@Version`；失败经 `saveAndFlushWithOptimisticLockMessage` 转成固定中文提示 | 存在这道防线，且失败提示文案固定 | `源码可证明` |
| 各写接口的前置状态校验 | 第 1.7 节清单，例如「当前任务已处理」、「流程已完结,不支持取回」 | 存在这道防线，且文案可枚举 | `源码可证明` |

```evidence
file=参考代码/java-serve/rsh-cloud-workflow-center/src/main/java/com/rsh/cloud/workflow/center/entity/FlowInstance.java
line=31
contains=@Version
strength=源码可证明
head=java-serve/rsh-cloud-workflow-center@37c01d04eb10
deployment=未取得（目标平台不提供版本接口）
```

浏览器侧另有 `utils/RequestQueue.js` 级别的重复请求抑制，属于前端行为，不构成服务端保证，**工具不能依赖它**。

### 2.4 跨存储非原子导致的部分生效

| 结论 | 强度 |
| --- | --- |
| 审批路径在同一个 `@Transactional` 方法里先写 Mongo 表单数据、后写关系库流程数据。Mongo 写入不受该事务约束，关系库回滚不会撤销它。因此「关系库明确未变」不等于「目标什么都没写」。 | `源码可证明` |
| 这条直接决定了第 1.6 节矩阵里「乐观锁冲突 + 明确未变」为「不确定」，也决定了「不可解释失败 + 明确未变」不能上升为「确定失败」。 | `源码可证明` |

证据见第 1.6 节的两个 `FlowAuditServiceImpl` 证据块。

### 2.5 待实测结论与重复写探针清单

以下结论只能由第一次真实写确认。**本切片不执行任何探针，只产出清单。**

| 编号 | 待实测结论 | 强度 |
| --- | --- | --- |
| P1 | 重复提交同一动作时，乐观锁这道防线是否真的命中 | `源码推断、待 F-016 实测` |
| P2 | 命中时返回的精确文案是否就是 `流程状态已发生变化，请刷新后重试`，`code` 是 `ERROR_99999` 还是 `RESP200` | `源码推断、待 F-016 实测` |
| P3 | 前置状态校验这道防线在重复写时是否触发，返回哪一条清单文案 | `源码推断、待 F-016 实测` |
| P4 | 各动作重复写的实际后果分级：会被拦住，还是会写第二次 | `源码推断、待 F-016 实测` |
| P5 | 目标环境的动态审批方式注册状态，以及 `flow_trigger_config_relevance.audit_way` 迁移是否已执行 | `源码推断、待 F-016 实测` |
| P6 | 乐观锁失败时 Mongo 表单数据是否已经写入，即该场景能否收窄为「确定失败」 | `源码推断、待 F-016 实测` |

探针清单：

| 编号 | 前置条件 | 最小步骤 | 期望观察点 | 由 F-016 哪一步覆盖 |
| --- | --- | --- | --- | --- |
| P1 | 一条 `status=run` 的实例，当前节点有待办，账号是当前处理人 | 对同一待办连续发两次同意，第二次不重新读实例 | 第二次响应的 `isSuccess`、`code`、`message`；重读实例 `version` 是否只加一次 | F-016 首次真实写后的重复写观察步 |
| P2 | 同 P1 | 记录 P1 第二次响应的完整包体 | 文案是否与第 1.8 节全等；`code` 取值 | 同上 |
| P3 | 一条已被他人处理完的待办 | 对该待办发一次同意 | 响应文案是否命中第 1.7 节清单；是否在写之前被拦 | F-016 前置拒绝观察步 |
| P4 | 每个动作各准备一条可执行实例 | 逐动作发两次同一请求，中间不读 | 重读后实例状态、待办数量、审批记录条数是否只变化一次 | F-016 逐动作首次真实写 |
| P5 | 只需只读权限 | 读一次流程模板详情，取 `auditWay` 字段 | 值是编码名（已迁移）还是数字 ordinal（未迁移） | 已由 `test/integration/f014_error_semantics_readonly_test.go` 只读探测给出间接结论，F-016 复核 |
| P6 | 能构造乐观锁冲突（两个会话并发审批同一实例） | 并发发两次同意，取失败那次 | 失败后重读表单数据，确认 Mongo 侧是否已被改写 | F-016 并发冲突观察步 |

清单完整性由 `test/run-f014.sh` 检查：每条 `源码推断、待 F-016 实测` 结论都必须有同编号探针条目。

## 3. 条件求值

状态：未开始。

## 4. 演员解析

状态：未开始。

## 5. 回退语义

状态：未开始。

## 6. 取回语义

状态：未开始。

## 7. 转发语义

状态：未开始。

## 8. 加签与移交语义

状态：未开始。

## 9. 会签与并行语义

状态：未开始。

## 10. 附件与富文本

状态：未开始。

## 11. 表单权限与可见性

状态：已勘定（源码可证明 + 目标库数据分布，2026-09-05 F-024 勘定）。

问题：节点级字段可见与可编辑如何声明，以及一个节点上「用户真实能填哪些字段」如何判定。

结论：

1. **声明位置**：`t_flow_node_field_power_template` 与 `_proxy`，按「节点 × 表单字段」存 `field_power`。
   枚举是 `FieldPower{only_read, edit, hide}`；目标库实测只出现 `edit`（94454 行）与 `hide`（714 行）。
   节点树读取接口以 `flowNodeFieldPowerTemplateList` 返回，元素带 `formFieldTemplateEnglishName` 与 `fieldPower`。
2. **可编辑集合**：目标审批页按 `nodeProxyId` 取本节点权限，`fieldPower != 'hide'` 的字段构成 `enableData`；
   整张表单先全禁用，再 `generateForm.disabled(enableData, false)` 只放开这批。**未声明的字段保持禁用**，
   所以「能填什么」由声明决定，不是由字段本身决定。
3. **必填按权限清理**：不在 `enableData` 里的字段一律去掉 `required` 与 `rules`；本身没设必填的字段
   即使有权限也不会变必填。
4. **两条路径约定**：权限里的嵌套字段用 `_$$_` 作分隔符，消费时替换为 `.`；子表单每列权限用
   `<model>_col` 这个额外字段承载。
5. **隐藏是累积的**：本节点 `hide` 加上此前已审批节点设过的 `hide`，只影响显示，不影响该节点能编辑什么。
6. **版本取用**：实例绑定的是代理版本，执行期必须取代理版（按节点），模板改动不影响在跑实例；
   配置期没有实例，用模板版。
7. **覆盖面**：目标库 2732 个流程模板中 1718 个存在「某审批节点可编辑、发起节点不可编辑」的字段，
   因此「配置阶段一次性回填全部数据」在多数流程上都不符合真实用户能做到的事。

```evidence
file=参考代码/rsh-framework-all/rsh-framework-cloud-commons/src/com/rsh/framework/cloud/commons/workflow/web/model/enums/FieldPower.java
line=7
contains=only_read,
strength=源码可证明
head=rsh-framework-all@84bb19736a8a
deployment=2026-09-05 已核对目标库取值分布（仅 edit 与 hide 出现）
```

```evidence
file=参考代码/rsh-cloud-invest-power-system/src/views/GroupApproveManage/components/EnterpriseExamineDialog.vue
line=1723
contains=filter(x=> x.fieldPower != 'hide')
strength=源码可证明
head=rsh-cloud-invest-power-system@8a00cb9995df
deployment=2026-09-05 未取得（前端部署版本未核对）
```

```evidence
file=参考代码/rsh-cloud-invest-power-system/src/views/GroupApproveManage/components/EnterpriseExamineDialog.vue
line=1620
contains=this.enableData?.includes(y.model + '_col')
strength=源码可证明
head=rsh-cloud-invest-power-system@8a00cb9995df
deployment=2026-09-05 未取得（前端部署版本未核对）
```

```evidence
file=参考代码/rsh-cloud-vue-form-making/src/components/GenerateForm.vue
line=1575
contains=disabled (fields, disabled)
strength=源码可证明
head=rsh-cloud-vue-form-making@695b2783e226
deployment=2026-09-05 未取得（前端部署版本未核对）
```

## 12. 子流程语义

状态：未开始。

## 13. 超时与跳过语义

状态：未开始。

## 14. 通知与催办语义

状态：未开始。

## 15. 手动分支的提交传参

状态：勘定中（F-016 实测，接受侧已证实的部分见下；其余仍标注待实测）。

实测问题：提交（`/web/flowInstanceApi/submit`）遇到手动条件分支节点时，所选分支如何随请求传递。

实测结论（2026-09-05，计划 11 路径 1121 真实提交）：

1. 不携带分支选择时，提交被目标以 `isSuccess=false`、`errorType=custom_choose`、`message=手动条件分支,请选择` 拒绝，
   拒绝包 `data.branchNodes` 携带全部候选分支节点记录（含 id 与 nodeName）。
   该拒绝发生在节点分派校验阶段，**没有任何写入发生**。
2. 源码证据：`FlowOperateServiceImpl.validateHandBranchAndReturnExecuteNode` 要求
   `flowInstanceProtocol.getNextAuditorList()` 非空，且某项的 `nodeProxyId` 等于候选分支节点之一的 `id`，
   匹配成功即以该分支节点为执行节点。`fixedExecuteNodeId` 是并行条件分支的另一机制，与手动分支无关。
3. 按此传参（`nextAuditorList[].nodeProxyId = 所选分支目标节点ID`）后，同一流程的提交被目标受理
   （`isSuccess=true`，返回实例 id 与 `status=run`，实例移动到所选分支的首个审批节点）。
4. 工具实现：`internal/engine/step` 的提交载荷把路径已保存的分支选择解析为
   `nextAuditorList[].nodeProxyId`（`internal/service` 从真实结构与路径选择解析）。

```evidence
file=参考代码/java-serve/rsh-cloud-workflow-center/src/main/java/com/rsh/cloud/workflow/center/service/impl/FlowOperateServiceImpl.java
line=1069
contains=validateHandBranchAndReturnExecuteNode
strength=源码可证明
head=java-serve/rsh-cloud-workflow-center@37c01d04eb10
deployment=2026-09-05 真实提交实测（计划 11 路径 1121，运行 6 拒绝、运行 8 受理，trace 84f9669d11f6a3b4）
```

待实测残余：

- `fixedExecuteNodeId` 的并行条件分支行为（本切片未触发并行分支）。
- 受理后实例在已发列表的可见性延迟：2026-09-05 实测受理成功后即时重读已发列表为空
  （成功声明 + 明确未变 → 按矩阵判不确定 → 待对账），实例可见性与异步落库时序待人工在目标平台核对。

## 16. 表单数据的保存语义（整份覆盖）

状态：已勘定（源码可证明，2026-09-05 F-024 勘定）。

问题：一次写请求携带的 `formDataMongoVo.data` 与实例已存表单数据是合并还是覆盖。

结论：

1. **整份覆盖，不合并。** `FormDataServiceImpl.saveFormData` 在 `dataId` 非空时读出原文档后直接
   `query.setData(formDataVo.getData())` 再 save；`dataId` 为空时新建一份文档。两条路径都不会把
   请求缺失的字段从旧数据里补回来。
2. **审批必定触发保存。** `FlowAuditServiceImpl` 无条件调用 `flowOperate.saveFormData`，随后把返回的
   `dataId` 写进审核记录与实例。因此审批请求带什么，实例之后的表单数据就是什么。
3. **目标前端每次提交整份表单模型。** 审批弹窗取 `generateForm.getValues()`（注释说明改用 getValues
   是为了带上虚拟字段）整份放进 `formDataMongoVo.data`，且不携带 `dataId`。
4. **对工具的硬约束**：任何携带表单数据的写请求都必须提交「实例当前完整表单数据 + 本节点应改字段」。
   只带部分字段等于把其余字段清空，这是不可逆的真实数据损坏。

```evidence
file=参考代码/java-serve/rsh-cloud-workflow-center/src/main/java/com/rsh/cloud/workflow/center/service/impl/FormDataServiceImpl.java
line=404
contains=query.setData(formDataVo.getData());
strength=源码可证明
head=java-serve/rsh-cloud-workflow-center@37c01d04eb10
deployment=2026-09-05 未在真实环境触发过（工具的同意动作此前从未真实执行）
```

```evidence
file=参考代码/java-serve/rsh-cloud-workflow-center/src/main/java/com/rsh/cloud/workflow/center/service/impl/FlowAuditServiceImpl.java
line=225
contains=FormDataMongoVo formDataMongoVo = flowOperate.saveFormData(requestProtocol, flowInstanceVo, flowProxyVo);
strength=源码可证明
head=java-serve/rsh-cloud-workflow-center@37c01d04eb10
deployment=2026-09-05 未在真实环境触发过
```

```evidence
file=参考代码/rsh-cloud-invest-power-system/src/views/GroupApproveManage/components/EnterpriseExamineDialog.vue
line=1804
contains=let value = this.$refs.generateForm.getValues();
strength=源码可证明
head=rsh-cloud-invest-power-system@8a00cb9995df
deployment=2026-09-05 未取得（前端部署版本未核对）
```

## 17. 条件求值读取的表单数据来源

状态：已勘定（源码可证明，2026-09-05 F-024 勘定）。

问题：分支条件求值用的是库里已存的表单数据，还是本次写请求带上来的表单数据。

结论：用**本次请求带上来的**。`FlowOperateServiceImpl` 把 `flowInstanceProtocol.getFormDataMongoVo()`
原样塞进节点分派协议，`FlowNodeProxyServiceImpl.getDataValue` 只对这份数据做一层 `map.get(fieldaName)`
（不解析嵌套路径，见第 3 节）。请求没带表单数据时目标会构造一个空的 `FormDataMongoVo`，
于是所有条件字段都取不到值。

对工具的直接含义：发起人无权编辑的条件字段，可以由路线上后续有编辑权限的节点在自己的写请求里带上，
分支走向会按那次请求的值重新计算。这就是 F-024「分段填写」成立的依据。

```evidence
file=参考代码/java-serve/rsh-cloud-workflow-center/src/main/java/com/rsh/cloud/workflow/center/service/impl/FlowOperateServiceImpl.java
line=515
contains=flowNodeProxyProtocol.setFormDataMongoVo(flowInstanceProtocol.getFormDataMongoVo());
strength=源码可证明
head=java-serve/rsh-cloud-workflow-center@37c01d04eb10
deployment=2026-09-05 已在真实环境间接证实（运行 8 提交携带条件字段后实例移动到所选分支）
```

```evidence
file=参考代码/java-serve/rsh-cloud-workflow-center/src/main/java/com/rsh/cloud/workflow/center/service/impl/FlowNodeProxyServiceImpl.java
line=351
contains=public Object getDataValue(FormDataMongoVo formDataMongoVo, String key)
strength=源码可证明
head=java-serve/rsh-cloud-workflow-center@37c01d04eb10
deployment=2026-09-05 已在真实环境间接证实
```

## 18. 对账两个只读维度：已办记录与动作痕迹

状态：已勘定（2026-09-05 用真实账号实测，全程只读）。

问题：写结果不确定时，「已办记录」与「动作痕迹」两个对账维度从哪读、字段名是什么、按节点怎么过滤。
这两维决定「未生效 → 允许重放」是否成立，而重放会再写一次，因此必须实测而不是源码推断。

实测结论（2026-09-05，用户指定测试账号，只调读端点）：

1. **已办记录**与待办同表同端点：`/web/flowJobTaskLink/list`，`data.taskStatus` 取 `done`
   （`TaskStatusEnum.done`=已办）。实测 HTTP 200、`isSuccess=true`，返回记录带 `flowInstanceId`
   与 `flowNodeProxyId`；同一端点的 `waiting_send`（待发）与 `has_been_sent`（已发）也都返回同形状记录。
   从该账号自己的已办列表取一条记录，用它的实例与节点回读必然命中（实例 `03b9a7a6…` 节点 `4b3b3591…`）。
2. **动作痕迹**读 `/web/flowAuditRecord/list`，请求只带 `data.flowInstanceId`（与目标自己的流程日志同源）。
   实测返回数组，元素带 `flowInstanceId`、`flowNodeProxyId`、`auditStatus`；
   取响应里真实出现过的 `flowNodeProxyId` 回读必然命中（实例 `312c3f31…` 节点 `993daf28…`，该实例共 1 条记录）。
3. **节点过滤真的生效**：两个维度传一个不存在的节点标识都返回未命中而不是报错。
4. 空实例标识按「没有事实」处理，不发请求也不报错；对账收集器据此标缺失并降级。

对工具的约束：读不到（端点失败、字段名不符、响应形状变化）一律按证据缺失降级为「仍无法判定」，
绝不把「没读到」当成「没有痕迹」——「未生效」是唯一会导致重放的结论，必须五维全部真的读到且全部未变。

```evidence
file=参考代码/rsh-framework-all/rsh-framework-cloud-commons/src/com/rsh/framework/cloud/commons/workflow/web/model/enums/TaskStatusEnum.java
line=15
contains=done("done","已办")
strength=源码可证明
head=rsh-framework-all@84bb19736a8a
deployment=2026-09-05 真实账号实测：taskStatus=done 返回 10 条记录且带 flowNodeProxyId
```

```evidence
file=参考代码/rsh-cloud-invest-power-system/src/api/index.js
line=742
contains=findRecord: '/web/flowAuditRecord/list'
strength=源码可证明
head=rsh-cloud-invest-power-system@8a00cb9995df
deployment=2026-09-05 真实账号实测：按 flowInstanceId 读到审核记录并按 flowNodeProxyId 过滤命中
```

## 19. 已发列表的业务关联过滤：本工具发起的实例会被整条排除

状态：已勘定（2026-09-05 用真实账号与本工具自己发起的实例实测，全程只读）。

问题：按实例 ID 精确复查事实（`/web/flowInstanceApi/list` + `ids`）时，
请求里的 `data.flowInstanceBizRelevanceList` 会不会影响能不能查到这条实例。

实测结论（2026-09-05，实例 `7bcf0c29f1054ba0bed5cf367ba2f2d2`，由本工具发起）：

1. 请求带 `flowInstanceBizRelevanceList=[{"otherBiz":"company","otherBizId":""}]` 时，
   目标返回 `isSuccess=true` 且 `data=[]`、`total=0`——**不是报错，是静默查不到**。
2. 去掉这一个字段（其余条件不变）立刻命中该实例，返回状态 `run`、`currentNodeProxyId`、
   `currentAuditUserInfo` 等完整事实。逐项试验证明只有这一个字段决定命中与否：
   去掉 `useScope`、去掉状态过滤、只按 `ids`、按 `data.id` 都能命中。
3. 原因侧的事实：本工具的发起载荷不带任何业务关联（`BuildSubmitBody` 只发
   name/flowProxyId/formProxyId/companyId 与表单数据），因此这条实例在目标侧没有公司业务关联行，
   而带 `otherBizId=""` 的过滤被目标当成「必须存在公司业务关联」，把它整条排除。
4. 同一实例在其他入口是可见的：`/web/flowAuditRecord/list` 能读到它的「流程发起」审核记录
   （执行人骆蒙恩），证明写确实已生效——所以这是**读取口径问题，不是目标没写成功**。

对工具的约束（属产品原则第 2 条：工具问题必须与目标平台问题分开说明）：

- 按实例 ID 精确复查事实的读取一律不得附加业务关联过滤。问的是「这条实例现在什么状态」，
  与它挂在哪个公司的列表里无关。反向断言见 `test/contracts/f018/reconcile_readonly.sh`。
- 浏览用的已发列表（用户挑选实例的入口）与本条无关，保持与目标页面一致的过滤，不在本条范围内。
- 这条过滤是 F-016 首次真实写（实例 `caf2046d…`）与 F-018 收口前运行 12（实例 `6bd617f3…`）
  被判「不确定」的唯一原因：核验重读读不到实例 → 「成功声明 + 明确未变」→ 不确定 → 待对账，
  同时对账五维里三个由实例事实派生的维度全部读不到，结论只能是仍无法判定。
  修掉后同一条路径的发起写实测判为「确定成功」（运行 13，trace `0715f0cdf7cc2700`）。

```evidence
file=internal/adapter/target/client_fact_reads.go
line=53
contains=按实例 ID 精确复查事实时绝不附加业务关联过滤
strength=源码可证明
head=rsh-cloud-invest-power-system@8a00cb9995df
deployment=2026-09-05 真实账号实测：带公司业务关联过滤返回空集，去掉后命中同一实例
```

```evidence
file=test/integration/f018_instance_visibility_test.go
line=44
contains=TestF018CompanyRelevanceFilterHidesToolCreatedInstance
strength=源码可证明
head=rsh-cloud-invest-power-system@8a00cb9995df
deployment=2026-09-05 真实账号实测：同一实例两种查询形状的命中差异已固化为用例
```
