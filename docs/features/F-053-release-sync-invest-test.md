# F-053 发布时自动同步全过程 test 业务源码

- 状态：ready_for_manual
- 提出时间：2026-10-26
- 产品依据：`docs/PRODUCT.md`「F-053 发布时同步全过程业务源码」。
- 架构依据：`docs/ARCHITECTURE.md`「F-053 发布时同步最新源码」。
- 关联范围：F-023 一键发布、F-036 独立执行器、F-052 执行页值保真。
- 计划确认时间：2026-10-27 用户明确“开始实施”，批准本文件范围。

## 目标

每次发布测试平台时，自动把全过程远端 `test` 分支当次最新的流程业务代码纳入执行器页面，并能证明发布包和构建产物使用的是本次读取的来源版本。每次后续发布重新读取远端，不把本次 SHA 沿用为未来固定输入。用户不再需要先手工更新流程组件仓库，才能得到全过程最新的合同盖章评审参数处理。

## 已核实的现状

2026-10-26 对本机代码、Git 对象和全过程远端分支作了只读核对：

| 对象 | 核实结果 | 对本次的影响 |
| --- | --- | --- |
| 用户指定全过程仓库 | `D:\work\newroom\rsh-cloud-invest-power-system`，当前检出 `oygdev2`，工作区干净 | 不能把当前工作区或 `HEAD` 当作 `test` 来源，也不要求用户切换分支 |
| 全过程远端 `test` | `git ls-remote origin refs/heads/test` 返回 `9984fad66f4f67f7b68038d062bde4aba13e2d38` | 这是本次调查时的远端事实；实施后的每次发布必须重新解析，不能把此 SHA 永久写死 |
| 合同参数修复 | `test` 包含 `72b38a31d`「修复合同评审金额字段映射」 | 修改合同业务组件、发起/审批页面，并增加直接依赖 `src/utils/contractAmount.js` |
| 执行器基座 | `executor-service/flow-source-manifest.json` 仍指向 `rsh-flow-components` 的 `test` / `e141a799408192bac1fe1f3dc7d821a2cee64986` | 更新全过程 `test` 不会自动改变执行器输入 |
| 一键发布准备 | `deploy/update.py:prepare_source` 在本地来源与清单一致时直接返回，否则同步清单固定版本 | 没有查询全过程 `test` 的步骤 |
| 发布打包 | `deploy/update.py:package` 从基座清单 SHA 的 `git archive` 生成执行器源码 | 只在本机目录复制最新业务文件仍无效，打包会重新取旧 Git 对象 |
| 页面构建 | `deploy/Dockerfile.executor` 调用 `build-native-page`，消费 `executor-service/flow-source` | 必须核对最终构建输入，不能只证明参考目录更新过 |
| 上游同步入口 | 基座 `sync-manifest.json` 仍带 macOS 绝对路径，范围还包含其他业务模块 | 不能直接运行它并把所有上游模块整体覆盖过来 |

合同修复的直接依据是本次核实的全过程业务源码：`ContractSealReviewBusiness.vue`、`LegalContractDocTable.vue` 调用 `getContractAmountFields`；`FlowDialog.vue`、`EnterpriseExamineOpinion.vue` 调用 `getContractRmbAmount`。相关组件位于 `GroupApproveManage` 或其直接引用的公共组件范围内。

## 与原身份报错的关系

“目标业务数据流程身份不一致”来自 `internal/service/history_data.go:validateHistorySnapshotSource`，比较的是选中实例的 `FlowCode` 与计划当前流程的 `FlowCode`，不是登录账号是否相同；比较发生在配置期保存历史数据来源时，尚未进入执行器。

本次只读读取线上计划列表，最新计划为计划 8「oyg测试007」，绑定「oyg-合同盖章评审（智数技术）」，账号「欧阳改」，已有一次成功运行；计划 7 绑定「合同盖章评审（智数技术）」。这些事实不能证明原选择失败时两侧的流程编码相同，也不能证明成功运行的合同金额请求正确。

源码同步解决发布时业务实现滞后；它不能作为放宽流程身份校验的理由。本切片不修改候选匹配、流程编码校验或已有计划绑定。原身份问题若仍出现，应按当次计划、所选实例及目标原始响应单独对照两侧编码，不把源码过期直接当作该错误的已证实根因。

## 范围

包含：

- 接入当前仓库实际存在的一键发布入口 `deploy/update.cmd` / `deploy/update.py`，包含 `--package-only`，统一在打包前准备源码。
- 固定全过程业务来源为用户指定仓库对应远端的 `refs/heads/test`，并在每次发布重新解析；执行器 `rsh-flow-components` 基座远端 `test` 也每次重新解析，不在来源清单固定 SHA。本机路径由受忽略的发布配置保存，不让开发机绝对路径成为执行器构建前提。
- 直接以全过程 `test` 完整应用源码为页面主体，只保留执行器 `/flow` 路由、身份、请求和组件注册适配文件；合同业务参数修复以实际合同组件和金额函数做回归。
- 使用 Git 对象生成隔离候选；核对来源、映射文件内容、依赖是否可解析及合成输入摘要；发布包从这个候选生成，构建也消费它。
- 在执行器源码元数据、发布来源事实和构建证明中记录基座版本、全过程业务版本、映射版本和最终内容摘要，能逐级对上。
- 来源或同步失败时返回非零，在上传与服务切换前停止；已生成的旧包不能被本次失败流程继续上传。
- 更新当前相关测试、发布使用说明和人工核对步骤。

不包含：

- 修改全过程源仓库，切换其分支、提交或推送到 `oygdev2` / `test`；本次只读取 `test`。未来如需改该仓库，仍先改 `oygdev2` 再 cherry-pick 到 `test`。
- 将执行器适配文件替换为完整全过程应用入口，或把执行器切换到全过程站点路由。
- 修改 FormMaking 库源码、重造合同参数映射、增加兼容旧数据的分支或失败后沿用旧业务快照。
- 自动升级配置侧 `form-runtime` 或覆盖其持久化 live 版本；它继续由已有受控维护入口管理。
- 更改身份校验、候选查询、动作顺序、一次写纪律、写后核验、数据库、计划与历史运行事实。
- 实际部署或自动重跑真实业务。完成实现与本地验证后停在 `ready_for_manual`，上线发布及目标业务由用户后续指令安排。

## 技术约束

本节列出需要验收的约束；实现决定统一以 `docs/ARCHITECTURE.md` 的对应章节为准。

1. “最新”指每次发布源码准备开始时分别从两个远端成功读取的 `test` HEAD。该次候选、打包、构建和版本记录使用本次读取的完整 SHA；它只是本次发布的追溯事实，不是下一次发布的固定输入。下次发布重新查询两个远端，解析或取得来源失败则停止。
2. 用户提供的本机仓库用于确认规范远端和定位来源，不能复制工作树。即使本机检出 `oygdev2` 或有未提交内容，发布也只能消费远端 `test` 的 Git 对象；不替用户清理、切换或覆盖该目录。
3. `参考代码/` 仍只通过 `make refs-sync` 更新。本切片的发布候选放在隔离临时目录；不直接修改现有参考检出，不借用旧上游脚本的绝对路径或让无关参考仓库成为发布依赖。
4. 全过程 `test` 的完整 `src`、`public`、`package.json` 和 `package-lock.json` 作为页面主体；仅保留执行器 `/flow` 路由、身份、请求和组件注册适配文件，已知 `FlowDetail.vue`、`FlowInitiate.vue` 路由适配页不得被业务主体覆盖。
5. 普通业务源码不再拆成逐目录/文件同步清单，新增或删除文件自然随完整主体变化；隔离和摘要验证通过后整体生成候选，不以旧候选文件补齐。
6. 发布打包不能继续只对旧基座 SHA 做 `git archive`。必须验证合成候选的全部文件及摘要后打包；容器内构建消费同一输入，缓存失效也要包含全过程业务 SHA 和内容变化。
7. 删除本次发布路径上“远端不可达便沿用清单旧版本”的行为。原本运行中的服务可以继续服务，但一次新发布不能声称已同步最新后使用旧业务源码。
8. 同步过程不调用目标业务接口。凭证、`.env*`、Git 元数据、日志、依赖缓存和本机产物不进入发布包；受控 FormMaking 产物沿用既有来源，不改其源码。

## 实施任务与完成标准

### T01 来源清单与隔离候选

- 扩展执行器来源清单，分别表示执行器适配来源及全过程业务来源；不把开发机目录写入构建契约。
- 每次从两个远端 `test` 解析最新 SHA，并仅在本次候选范围内固定这两个版本事实；直接以这些提交生成执行器页面候选。
- 核对本次合同业务组件、发起页、审批页和 `contractAmount.js` 已进入页面主体，保留执行器 `/flow` 路由入口、身份与请求适配。
- 候选构造失败不触碰当前执行器目录；来源目录和用户既存修改保持。

完成条件：远端 `test` 未变化时重复准备的候选摘要一致；任一远端 `test` 前移后下一次候选确实变化；`oygdev2` 或工作区文件不混入候选；全过程业务页面未覆盖执行器路由适配文件。

### T02 发布打包与版本记录

- 一键发布与仅打包共用源码准备路径，不允许仅打包绕过远端读取。
- 从核对过的合成候选打包，记录两套来源的 remote / branch / 完整 SHA，以及清单和最终输入摘要。
- 服务器从本次发布包构建，执行器启动不再改变当次发布来源；成功发布事实与执行器报告的来源一致。
- 更新 `deploy/README.md` 的本机配置、直接 Compose 构建前置步骤和失败处理。直接构建也须校验准备事实，缺少或不匹配时失败。

完成条件：解包后可以逐文件对上候选；业务 SHA 改变能导致执行页重建；远端不可读、依赖或摘要失败时未调用上传与切换。

### T03 定向验证

测试按类别放在根目录 `test/`，仅运行当前功能相关范围：

| 类别 | 必测场景与可判断结果 |
| --- | --- |
| 单元：来源选择 | 临时 Git 远端有不同的 `test` / `oygdev2`；执行器工作树有未提交修改，仍只发布远端 `test` 的已提交页面主体，源工作区保持；执行器适配文件优先保留 |
| 单元：每次取最新 | 两个远端的 `test` 前移后再次准备，基座和业务 SHA 都更新；相同来源重复准备摘要一致；下一次发布不读旧 SHA |
| 单元：失败边界 | 远端不可达、分支不存在、SHA 取不到、文件缺失、未知依赖、路径冲突及候选被改动时失败，无上传、无切换、无旧包继续使用 |
| 集成：真实打包路径 | 调用生产的准备和打包函数，解包核对业务文件与直接依赖；不能只检查清单文本或 mock 同步函数后宣称成功 |
| 集成：包边界 | Git 元数据、凭证、缓存、日志不入包；受控 FormMaking 产物可用；不将配置侧源码或产物作为执行页基座 |
| 回归：合同参数 | 执行本次目标源码的真实金额函数及相关调用，覆盖 CNY、GBP、对象形式币种和 0；核对原币、人民币及币种参数按目标最新代码处理 |
| 回归：原生页面 | 发起/详情/审批/转发入口仍可构建；完整身份、F-052 写入意图和一次写相关现有定向回归通过 |
| 构建 | 执行器类型检查、正式原生页面构建；记录构建输入摘要，证明与发布包中的全过程 SHA 和业务源码一致 |

本地模拟及构建不等于真实目标参数验收，不自动启动浏览器或发送真实流程写请求。模拟远端和临时仓库不用真实参考目录。

### T04 交接

- 完成相关验证后中文原子提交；不纳入用户既存 `docs/功能清单.md` 修改、参考源码或运行证据。
- 将状态更新为 `ready_for_manual`，记录实际通过的命令与边界；未执行的线上发布和真实请求明确写待核对。
- 不自动开始其他功能，不把已存在的 F-051 / F-052 待验收状态改为通过。

## 验收清单

- [x] 每次发布及 `--package-only` 都重新读取组件基座与全过程远端 `test` 最新 SHA；SHA 只作为本次发布事实，不写死在来源清单或沿用到下次发布。
- [x] 基座与全过程远端都不在清单固定 SHA；两个上游前移后再次准备均取最新 `test`。
- [x] 全过程 `oygdev2` 工作区内容不进入发布，原仓库分支和修改保持。
- [x] 完整业务页面源码、合同组件及直接依赖进入合成候选、发布包和执行器构建；执行器路由适配文件保留。
- [x] 同一次发布的业务 SHA、基座 SHA、输入摘要和构建证明能对上。
- [x] 远端或同步失败会停止发布，不静默使用旧业务版本。
- [x] 包边界、临时远端前移、固定 SHA 拒绝、失败门禁、合同金额函数和相关执行器回归通过。
- [x] 最新双来源候选的正式执行页构建实际运行成功，缓存命中时跳过构建。
- [x] 文档与人工检查点齐全；原实现提交 `e8e7647`，最新同步策略与候选保护修订已验证，功能状态为 `ready_for_manual`。

### 已执行验证

- `node --check executor-service/scripts/sync-flow-source.js`
- `node --check executor-service/scripts/source-integrity.js`
- `node --check executor-service/scripts/build-native-page.js`
- `node --experimental-strip-types --test test/unit/executor-service/flow-source-sync.test.ts test/unit/executor-service/baseline-source.test.ts test/unit/executor-service/contract-amount.test.ts`：15 项通过，覆盖两个远端前移、适配文件保护和远端不可达失败门禁。
- `python test/unit/deploy/test_update.py -v`：3 项通过。
- `pnpm --dir executor-service typecheck`
- `python -m py_compile deploy/update.py`
- `python deploy/update.py --package-only <临时包路径>`：真实远端准备与打包通过；包内确认保留 `vue-form-making/package.json`、`backendRequestRouting.js`、`portalFlowAuth.js`。本次读取基座 SHA `546ca9c87a5155428f4a904efbf26c7b814a6289`、全过程 SHA `7cebcaf335fb48de4ad174763804dfb12fd56ce3`，候选摘要 `2c61aadd3fa79282cca5c43aece29a383631fec983a8e8d70bf8cbd71010e748`。
- `node executor-service/scripts/build-native-page.js`：最新双来源候选构建成功；随后 `--package-only` 再次重新查远端并打包，页面构建确认摘要仍匹配而跳过重建。第一次真实构建暴露的 3 个基座适配依赖缺失已纳入保护边界并由最终候选构建验证通过。
- `node --experimental-strip-types --test test/unit/executor-service/flow-source-sync.test.ts test/unit/executor-service/baseline-source.test.ts test/unit/executor-service/contract-amount.test.ts`：16 项通过，额外覆盖来源清单误带固定 SHA 时拒绝准备。
- `node executor-service/scripts/sync-flow-source.js`：取得全过程 `test` `9984fad66f4f67f7b68038d062bde4aba13e2d38`，生成 1911 个业务文件的候选。
- `node executor-service/scripts/sync-flow-source.js --verify-prepared`
- `node --experimental-strip-types --test test/unit/executor-service/flow-source-sync.test.ts test/unit/executor-service/baseline-source.test.ts test/unit/executor-service/contract-amount.test.ts`：14 项通过。
- `python test/unit/deploy/test_update.py -v`：3 项通过。
- `pnpm --dir executor-service typecheck`
- `python -m py_compile deploy/update.py`
- `node executor-service/scripts/build-native-page.js`：全过程锁文件依赖安装后正式页面构建通过；第二次执行命中缓存。构建有 4 项原有路由导出/资源体积警告，无构建错误。
- 2026-10-27 修正核对：下一次发布必须重新查询两侧远端 `test`，上一轮记录的 SHA 仅是当时的追溯事实，不是未来发布固定输入。
- 上一轮来源元数据（仅作历史追溯，不作为当前或未来发布输入）：基座 SHA `e141a799408192bac1fe1f3dc7d821a2cee64986`；全过程业务 SHA `9984fad66f4f67f7b68038d062bde4aba13e2d38`；映射摘要 `70c73828e2c53540bb26ce74a87c91d1e97c70467dd7f61bd4f9e893696df353`；候选摘要 `5592126768a79db75269e52c8b0bd7025bd12fc8ef2e6d678ab26cbc3fd1f83c`；构建输入摘要 `dee2942bf305fc153efa83effc8b8c56e71af792280a666bf3c81b5de0dde8d8`。
- 未执行线上发布、未启动浏览器、未发送真实流程写请求；未核对真实合同页面参数。

## 人工核对

实施完成且另行安排发布后，由用户核对：

1. 运行一键更新，日志应显示当次全过程 `test` 的完整提交号及业务源码已纳入执行器；服务器发布记录和执行器版本信息应一致。全过程有新的 `test` 提交时，下次发布无需手工改业务 SHA。
2. 使用合同盖章评审真实页面，检查发起、重提或审批时的合同业务请求：币种、原币金额、人民币金额按该次 `test` 源码规则处理；流程名称金额取值也与目标页面一致。
3. 检查 F-052 已有的路径补丁保真与身份，页面实际值和目标业务请求一致；不能只用“运行成功”替代参数核对。
4. 在本地仅打包的隔离测试中使远端不可访问，入口应明确失败且不连接部署服务器；线上当前服务保持。
5. 配置侧 live 表单、已有计划与运行事实按发布前后的正常边界核对。原流程编码不一致若重现，保留当次所选实例与计划的编码证据另行排查，不绕过校验。

## 状态记录

- 2026-10-27 `ready_for_manual`：按用户最新收敛要求改为全过程 `test` 完整应用源码作为页面主体，仅保留执行器 `/flow` 适配层；修正同步依赖不匹配与路由适配页被删除的问题。真实远端候选、真实 `--package-only`、打包/失败边界、14 项 Node 回归、3 项 Python 打包测试、类型检查、Python 编译与正式页面构建通过。实现提交 `e8e7647`。下一步由用户安排发布，核对来源版本和真实合同参数；身份编码报错若重现仍独立采证，不放宽校验。
- 2026-10-27 修订：按用户要求移除 `rsh-flow-components` 基座的固定提交号；基座与全过程仓库每次发布/仅打包都重新查询各自 `test`，当次 SHA 只用于本次候选、构建与发布追溯。16 项执行器测试、3 项发布测试、类型检查、Python 编译、真实 `--package-only` 与最新源码正式构建通过，功能仍停在 `ready_for_manual`；实际发布和页面人工验收未进行。
- 2026-10-27 `implementing`：用户明确批准开始实施；修改前完成标准采用本文件验收清单，覆盖源码冻结、真实打包构建、失败门禁和合同参数回归。实际部署和真实业务仍按本文件边界另行安排。
- 2026-10-26 `preparing`：承接用户“每次发布自动同步全过程 test”的需求，只读核对本机发布链、执行器快照、全过程远端 HEAD 和合同映射修复。
- 2026-10-26 `awaiting_approval`：完成本计划及产品、架构、路线图、进度文档。尚未修改同步、打包或执行器代码，未发布、未重跑真实业务；下一步等待用户明确确认本文件范围。
