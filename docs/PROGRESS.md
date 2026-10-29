# 当前进度

- 2026-10-29 F-052 按用户“快点干好发布”进入 implementing：删除工具自加字段不变性拦截，保留目标原生处理、技术错误、真实结果核验与一次写保护；修正自动停止文案。仅本项目本地提交，验证后发布，用户人工验收。

- 2026-10-29 关联流程选择器误隐藏已修复发布（F-051，ready_for_manual）：本地提交 `e054e69` 删除 `_flow` 后缀强制隐藏，77 项表单回归、10 项同步回归、3 项打包测试、语法及正式构建通过。发布 `20261029-142314-5f361bb1` 实际使用全过程远端 test `e5cea7795d67`、基座 test `19b01898f838`；六服务与执行器验活通过。配置侧用当前受控 `candidate-job-3` 重建，仅应用适配修复，来源与维护指针保持；live 53 文件、HTTP 46 资源和两入口一致，新 buildHash `f246c300f48a08b70432`，实际入口已移除后缀隐藏规则。1113 个发布文件、环境/挂载、原计划配置及运行稳定字段核对通过；外部新增计划 17 导致计划/路径分母变化，运行 11/12 可选节点名投影重启后缺失已单独记录，不冒称全部响应不变。仅本项目本地提交，无推送、无目标业务写；用户强刷后验收关联选择器、附件和原联动，详细证据见 F-051。

- 2026-10-28 重提与审批端点全量回退已适配并发布，停在 **ready_for_manual**：全过程 `test` 新提交 `27f82a62b`「回退流程重新发起与审批接口为老端点（抄送相关新接口保留）」把重提从 `/newWorkflowCenter/flowInstance/reSubmit` 回退为 `saveFlowInstanceAgain=/web/flowInstanceApi/reSubmit`、审批从 `/newWorkflowCenter/flowInstance/flowAudit` 回退为 `qualityManage.submitTask=/flowInstanceApi/audit`（无 /web 前缀），并从 api 定义删除全部新工作流写端点（仅剩 `flowProxy/findById` 抄送只读接口，`/newWorkflowCenter/` 路由前缀因此保留）。重提链权限预检也随回退打到老 reSubmit 端点（带 `checkPermissions='first'`）。工具侧（提交 `0e80fc8`）把 resubmit/approve/reject 的完成检测期望端点、写白名单、协议矩阵、动作目录、runtimeexec 登记、详情投影、耗时归类全部回退为老端点，删除 `RuntimeReSubmitEndpoint`/`RuntimeAuditEndpoint` 登记；`isPermissionCheckSubmit` 预检排除扩展到 reSubmit 端点，与 submit 同口径不算正式写。Go 构建、6 个受影响包、协议矩阵定向用例、执行器类型检查与 completion/target-route Node 38 项通过（f035 包全量仍有既有 `TestSpecialBusinessLifecycleRegistry` 失败，与前两轮相同、已记录未纳入）。发布 `20261028-115816-0d734cab` 重新读取基座 `test` `546ca9c8`、全过程 `test` `27f82a62`，映射摘要 `c6839074...`、候选摘要 `f4646c51...`、覆盖数量 1；六服务健康、HTTP、Chromium 与日志挂载验活通过；容器内核实 api 定义已无新端点、审批页用 `submitTask`、重提页用 `saveFlowInstanceAgain`、合同初始化修复块保留，服务器 revision `0e80fc8` 与摘要一致。未重试旧运行、未发送新的目标业务写；请强刷后重新发起路径，验收提交/审批/重提全部走老端点及节点流转。

- 2026-10-28 线上审批失败已诊断并修复再次发布，停在 **ready_for_manual**：用户运行 `oyg测试11` 运行 1/运行 2 在第 2 步审批报「动作没有发出期望写请求 /newWorkflowCenter/flowInstance/flowAudit」，第 1 步提交成功（提交端点回退适配生效）。服务器取证（root SSH 读运行日志与执行器容器日志）还原根因：合同初始化收尾向表单回写引起 `ContractSealReviewBusiness` 组件重建，`initFormInstance` 接线只在 `jsonData` 监听器发送一次，重建实例收不到接线，分发时 `currentContractInfo` 为空被合同门禁按设计阻断「合同当前状态未读取成功，已阻止提交」，写请求从未发出——属 6cf2e25 门禁自身的边界缺陷，首次真实合同审批即暴露，与端点无关。修复（提交 `4b1c742`）：`contractBusiness` 门禁前，若组件从未初始化（Promise 空、无错误、无合同 ID）就用分发参数上的表单实例就地补一次有界初始化再等待结果；覆盖真实候选断言补齐。10 项同步测试与类型检查通过。发布 `20261028-113200-f9c22865` 重新读取基座 `test` `546ca9c8`、全过程 `test` `8aaaf2a0`，映射摘要 `c6839074...`、候选摘要 `ff46c4f3...`、覆盖数量 1；六服务健康、HTTP、Chromium 与日志挂载验活通过，执行器容器内已确认含修复块，服务器 `last-release.json` revision `4b1c7421...` 与摘要一致。未重试旧运行、未发送新的目标业务写；请强刷后重新发起合同盖章评审路径，验收第 2 步审批与后续节点流转。

- 2026-10-28 发起提交端点回退已随再次发布全过程同步上线，停在 **ready_for_manual**：全过程 `test` 新关键回退 `2e10a724b`「发起流程submit接口回退为老端点flowInstanceApi/submit」把新发起正式提交从 `/api/newWorkflowCenter/flowInstance/submit` 回退为 `/web/flowInstanceApi/submit`（正式提交前删除 `checkPermissions`，与草稿同端点；重提仍走新 `reSubmit`、审批仍走新 `flowAudit`）。工具侧把 submit 完成检测期望端点、写白名单、协议矩阵、动作目录、runtimeexec 动作登记、详情投影、耗时归类、路由流程名保护与新实例解析同步回退，删除已无页面调用方的新提交端点登记；同端点 `data.checkPermissions` 权限预检仍不算正式写。Go 构建、5 个受影响测试包、协议矩阵定向用例、执行器类型检查与 completion/target-route Node 37 项通过。本地中文提交 `9df2087f758f2bd41b045bdfc4644b2eeeca3d61`（另收口此前漏提交的终态明细测试对齐 `354d272`）；发布 `20261028-110626-8711dcfb` 重新读取基座 `test` `546ca9c87a5155428f4a904efbf26c7b814a6289`、全过程 `test` `8aaaf2a0165cd676357fd89010fb000351a700dc`，候选摘要 `ae234271754c56c8add877983b06fdd21e2706af49f249998cccbfff809d4fae`，映射摘要 `adb4aac09d1f9b59c3aa9c8b448281e80f50876485d10ff538e2e7cb67e1d83d`，项目内覆盖数量 1；服务器 `last-release.json` 与 `deploy/release-source.json` 完全一致，六服务健康、HTTP、Chromium 启动与日志挂载验活通过，执行器 `/health` 报告同一候选摘要。未推送共享远端、未重试旧运行、未发送新的目标业务写；请强刷后用新发起路径验收提交走老端点、表单显示与节点流转。

- 2026-10-28 手动条件分支自动续办修复已发布到 `http://192.168.2.200:19000`，停在 **ready_for_manual**：线上运行 7 / 路径运行 18 第 11 步返回 HTTP 200、`data.errorType=custom_choose` 且省略 `isSuccess`，执行器旧事件入口只识别 `isSuccess=false`，未触发已有 `choice_required → choice_continue`，所以未代入路径中已保存的入口，目标弹出“手动条件分支，请选择”。修复后事件识别与完成结算共同识别三类明确选择错误；Go 只按已保存入口与目标候选精确匹配、只继续一次，自选分支须有已配置人员，不匹配即阻断；写响应必须有明确 `isSuccess=true` 才算成功，结果不明仍禁止重发。执行器 Node 全量 144 项、runtimeexec/分支 Go 定向用例、TypeScript 类型检查及差异检查通过。本地中文提交 `249bfcb9f7bd2f8255f79eb8d0cb9a7439bce33d`；发布每次重新同步的基座 `546ca9c87a5155428f4a904efbf26c7b814a6289` 与全过程 `f9133e67cc8ecf66d4c12a3f9c5b98d64414fe02`。六服务健康、F-023 HTTP/Chromium/日志挂载验活通过，执行器版本与候选摘要核对一致，主表单服务及 19001 直连健康；线上路径状态数量仍为结果待确认 1、已完成 1、失败 1，活动路径 0，未改数据库记录。未推送共享远端、未重放旧待确认步骤、没有发出新的目标业务写。请强刷后用新运行验收执行器自动代入手动分支，以及表单显示与节点流转。

- 2026-10-27 F-053 新工作流提交端点修复已发布，停在 **ready_for_manual**：运行 6 证实旧 `/web/flowInstanceApi/submit` 只是 `checkPermissions=first` 权限预检，正式业务写已改为 `/api/newWorkflowCenter/flowInstance/submit` 且目标返回成功并创建实例 `3b98cc7fb0694730832b71d07de5eb30`；误报根因是执行器完成检测仍等待旧正式端点。已把 runtime 正式 submit/reSubmit/flowAudit、路径转发、写白名单、协议矩阵、动作目录、运行详情与新实例解析统一切到当前页面契约，保存草稿仍走旧 submit 且权限预检不算写。29 项执行器回归、类型检查、相关 Go 契约、Go 构建与原生页面构建通过。发布 `20261027-184106-ff85238f` 使用本地提交 `4c2d369e52cb33af2170c1f4501b7bce66630529`，当次读取基座 `546ca9c87a5155428f4a904efbf26c7b814a6289`、全过程 `f9133e67cc8ecf66d4c12a3f9c5b98d64414fe02`；六服务 healthy，executor `exec-engine/v2`、Chromium 启动、主入口与 19001 表单 runtime 健康均通过。未重试运行 6、未发送新的业务写请求；等待用户强刷后重新发起新路径验收执行器与表单显示。

- 2026-10-27 F-053 已发布到 `http://192.168.2.200:19000`，停在 **ready_for_manual**：第三次发布 `20261027-180641-4b252ac3` 使用本项目提交 `71d41f58cddce7985c8c169ca69a93ab63b6c79f`，当次重新读取基座 `test` `546ca9c87a5155428f4a904efbf26c7b814a6289` 与全过程 `test` `60d1c53858739d369199bcf3f8d7c1b4ebe0aae2`，执行器候选摘要 `aa928febcef1770572c84eb9f028102088951c5b0f4bbcfe29b2d7bc129986b2`。六服务 healthy，F-023 验活通过。表单维护任务 3 成功完成，线上表单主入口与 19001 直连均报告 `test/546ca9c8/e89625f5...`，维护来源干净；前两次任务分别定位并修复 live 卷挂载点与镜像层源码目录不能 rename 的边界。仅更新本项目服务器运行目录，未推送或修改共享远端；未启动浏览器、未发送真实业务写请求，等待用户强刷验收执行器与配置表单。

- 2026-10-27 F-053 发布同步行为修订，自动验证完成，停在 **ready_for_manual**：组件基座和全过程仓库 `test` 均于每次发布/`--package-only` 重新解析最新 HEAD；来源清单无固定基座 SHA，当次 SHA 只作该次发布追溯。修正新基座依赖的请求路由、门户身份和 FormMaking 入口文件保护。15 项 Node 回归、3 项 Python 打包测试、执行器类型检查、Python 编译、真实远端仅打包和最新双来源正式页面构建通过。验证当次 SHA：基座 `546ca9c87a5155428f4a904efbf26c7b814a6289`、全过程 `7cebcaf335fb48de4ad174763804dfb12fd56ce3`；候选摘要 `2c61aadd3fa79282cca5c43aece29a383631fec983a8e8d70bf8cbd71010e748`。未连接部署服务器、未启动浏览器、未发送真实业务写请求。

- 2026-10-27 机房迁移来源切换完成，待人工验收：`rsh-flow-components` 受控参考仓库已从旧远端安全迁移至新机房，迁移当时 `test` 快进到 `546ca9c87a5155428f4a904efbf26c7b814a6289`；表单运行时和执行器基座均配置为该新远端。执行器现于每次发布重新读取该 `test` 最新提交，全过程业务仓库仍为独立来源。表单运行时隔离同步与正式构建通过（4 项既有 Sass/Vue CLI 警告）；执行器新基座与全过程独立合成并通过摘要校验，15 项执行器 Node、3 项打包测试、61 项表单运行时测试、类型检查及远端迁移/脏目录回归通过。正式构建发现新上游依赖 `vue-json-viewer`，已将其加入表单运行时依赖及锁文件。未发布、未启动浏览器、未更改全过程仓库或其分支；下一步用户安排发布并核对页面。

- 2026-10-27 F-053 实施与自动验证完成，停在 **ready_for_manual**：按用户最新要求直接以全过程远端 `test` 的完整应用源码作为页面主体，只保留执行器 `/flow` 路由、身份、请求和组件注册适配层；发布与 `--package-only` 均先同步当时远端源码，失败停止且不复用旧包。真实候选、合同金额回归、打包边界、14 项执行器 Node 测试、3 项发布测试、类型检查与正式页面构建通过；二次构建命中缓存。该轮记录的来源 `test` SHA `9984fad66f4f67f7b68038d062bde4aba13e2d38`、基座 SHA `e141a799408192bac1fe1f3dc7d821a2cee64986` 与候选摘要仅为上一轮历史事实，不作为后续发布固定输入。全过程 `oygdev2` 工作树未改变。未执行线上发布、未启动浏览器或发送真实业务写请求；下一步用户发布后核对执行器来源版本及合同币种、原币金额和人民币金额。

- 2026-10-26 F-053「发布时自动同步全过程 test 业务源码」计划完成，状态 **awaiting_approval**：已核实当时发布只归档旧 `rsh-flow-components` / `e141a79`，全过程远端 `test` 当时为 `9984fad66` 且包含合同金额修复 `72b38a31d`。计划覆盖每次查询最新来源、受控同步、真实构建/打包、版本证明和失败终止；完整范围见 [功能文档](features/F-053-release-sync-invest-test.md)。尚未改实现、发布或重跑业务；下一步等待用户确认范围。原“流程身份不一致”比较 `FlowCode`，不能由源码滞后直接推定根因；线上最新计划 8 已有一次成功运行，真实参数仍需核对。

- 2026-10-26 F-052 审批反馈修复已发布 **2.200 / 8923504**，停在 **ready_for_manual**：空关联正常转换不再误拦，长错误完整保存在详情且不再阻断失败收尾，自动跳过/只读确认不再冒充成功审批或 0 毫秒。99 项 Node、Go 编排/契约、真实 MySQL 边界、实际审批模板隔离复验及类型/构建通过；线上六服务、1091 源文件、91 主前端资源、46 配置表单资源和配置/数据保持核对通过。运行 3 / 路径运行 14 已按无写失败事实补齐收尾并显示可重试，原七步与七次尝试保留。下一步由用户刷新后从第 7 步重试，核对跳过/耗时展示；未自动审批或代替验收。

- 2026-10-26 F-052 已发布 **2.200 / 94cdb55**，停在 **ready_for_manual**：执行页统一按路径数据初始化，完整身份避免被原生入口清空，装载后和分发前均核对。80 项 Node、三个 Go 契约包、类型与原生产物检查通过；实际模板 87 字段隔离复验保持 5000001/600/直签且原生校验通过。线上 1090 源文件、91 主前端资源与运行容器新模块匹配；六服务健康，配置表单 46 项资源、路径配置、原失败运行与报告统计保持。下一步由用户刷新后重试原路径，不自动提交目标业务或代替验收。

- 2026-10-26 F-052 实施与自动验证完成，停在 ready_for_manual：共用原生初始化保留路径写入意图，完整 URL 身份避免被清空，装载后及分发前均核对。80 项 Node 回归、三个 Go 契约包、类型与原生产物检查通过；实际输入 87 字段隔离复验保持 5000001/600/直签，原生校验通过。当前发布核对，之后由用户重试原路径，不自动提交目标业务。

- 2026-10-26 F-052 经用户授权进入 implementing：修复独立执行页回填后值偏离、原生 URL 身份被清空及提交前缺少核对的问题。保留路径修正与目标校验，范围和修改前完成标准见 [功能文档](features/F-052-executor-form-values.md)；发布后由用户核对，不自动重试失败运行。

- 2026-10-26 F-051 已发布 **2.200 / e38e2e3**，停在 **ready_for_manual**：统一修复整份数据回填后的初始化联动与六类容器失效引用，路径修正逻辑和实际两笔金额保持。69 项检查、语法检查、正式构建及截图数据隔离复验通过；1088 源文件、91 主前端资源和 46 项表单 HTTP 文件匹配，两个表单入口均已更新，目标快照未升级。六服务健康，配置/挂载、计划路径配置和报告事实保持；下一步用户强制刷新后核对外币双金额、重复刷新及其他同类联动，不自动验收。

- 2026-10-26 F-051 实施与自动验证完成，停在 ready_for_manual：统一修复原生初始化取旧值和六类容器失效引用；69 项回归、截图数据隔离复验、语法检查与正式构建通过，路径回写及两笔金额保持。当前执行发布和 live 表单资源核对，之后由用户在页面验收。

- 2026-10-26 F-051 按用户“修复并发布，我来核对”进入 implementing：修复配置表单刷新采用旧值与 report 注册残留，保留路径回写、金额与原生手动联动。范围和修改前完成标准见 [功能文档](features/F-051-form-refresh-layout.md)。实施后发布到 2.200 并停在 ready_for_manual。

- 2026-10-26 F-050 已发布 **2.200 / 3e9a3d0** 并独立核对：1087 源文件、91 个前端资源一致，六服务健康，配置/挂载、计划路径配置、运行事实、报告统计和 live 表单版本保持。后端与 web 更新；表单容器因后端依赖重建但镜像/live 不变，数据库、执行器和日志容器保持。状态仍为 **ready_for_manual**；下一步用户刷新页面并重新准备原失败路径后检查运行，不自动验收或继续其他功能。

- 2026-10-26 F-050 实施与定向自动验证完成，停在 **ready_for_manual**：零额外动作会编译默认提交/同意，人员确认、落盘状态、列表与运行前检查对齐；快速方案显式替换额外动作并保留主人员与表单，准备/运行交互及失败反馈已收口。五个 Go 包和执行器相关定向回归、真实 MySQL 重连预检、71 项前端检查、构建/类型/vet 通过。执行器整包另有一项原有终态日志用例失败，已用修复前源码复现，记录在 F-050，未扩入本轮。下一步发布核对后由用户按功能文档在页面准备并运行，不自动验收或开始下一项。

- 2026-10-26 F-050 按用户明确要求进入 **implementing**：修复快速同意方案的零额外动作判待配置、节点漏确认和准备反馈问题；范围与完成标准见 [功能文档](features/F-050-quick-approve-readiness.md)。不修改执行写语义或数据库结构，不开始其他功能。

- 2026-10-15 F-049 皮肤副作用复查完成，重新停在 **ready_for_manual**：修复浅色按钮交互变淡及浅底按钮叠色导致的文字对比不足，六类真实按钮在四种背景/四种状态的 2880 组检查最低 4.52:1。实际应用壳经换肤入口往返 16 次，路由页和未保存探针状态保持；弹窗、图表、导航、动效和现有业务前端回归共 84 项、类型检查、正式构建通过，未确认额外功能故障。只改前端，已发布 2.200（0964502）：1086 源文件与 91 个前端资源一致，六服务健康、其他五服务/配置/数据与报告统计保持；待用户浏览器人工检查，不自动 accepted 或启动下一项。

- 2026-10-15 F-049 按用户要求复查皮肤副作用，返回 **implementing**；确认浅色悬停/聚焦状态存在按钮文字对比不足，按功能文档中的本轮完成标准修复并补真实组件验证。范围仍为前端，验证发布后重新停在 ready_for_manual。
- 2026-10-15 F-049 报告图表高度修复已发布到 `http://192.168.2.200:19000`，源码 `f433406`，停在 **ready_for_manual**：ReportChart 以自身 220px 实体外框承载懒加载与图表，渲染器绝对定位填满，删除四处经 Suspense 丢失的无效高度类。真实组件检查在旧代码复现失败、修复后八种外观通过；82 项检查、类型检查与构建通过。六服务健康，1086 个发布源文件匹配；报告页懒加载 CSS 与本地逐字节一致，线上已有修正高度规则；只更新 web 容器，报告 counts/metrics、已有计划与运行、配置及表单版本发布前后保持。下一步用户 Ctrl+F5 刷新报告页，观察高度是否持续稳定、能否滚到下方表格，再缩窗和换肤核对；未代替人工视觉验收。
- 2026-10-15 F-049 极光/钛金前端重做已发布到 `http://192.168.2.200:19000`，线上源码 `0309343`，停在 **ready_for_manual**：极光是窄轨悬浮工作空间，钛金是石墨/暖铂金横向工作台；连续弹簧选中、页面/弹窗分层入场和单表面反光已接入，配置和表单按真实工作区高度适配。默认/深空的令牌与原样式对照保持，业务组件、后端、执行器与表单源码未改。81 项相关检查、类型检查和构建通过；六服务验活与 1086 个发布源文件、首页 23 个 JS/CSS 对照通过，实际只更新 web 容器。配置、数据卷、表单版本及已有 1 个计划/1 条运行均保留。下一步用户强制刷新，在极光/钛金深浅模式下核对观感、长弹窗、配置表单、导航和未保存内容，不自动验收。
- 2026-10-15 F-049 长列表弹窗溢出反馈已修复并发布到 `http://192.168.2.200:19000`，线上源码版本 `b49353f`，重新停在 **ready_for_manual**：统一正文规则改为当前 Naive 实际的 `n-card-content`，长列表可收缩并在正文内滚动，标题和操作区保留空间；本轮未改业务模板、选择、分页和确认关闭逻辑。新增真实 NCard/NDialog 服务端渲染检查覆盖八种外观，在旧规则上准确失败、修复后通过；移除错误类名的文本断言并纠正规格。76 项检查、类型检查、正式构建与差异检查通过。一键发布、六服务与执行器验活通过；1080 个源文件及首页 23 个 JS/CSS 与本地一致，线上已提供修正的滚动规则；配置、数据卷、表单版本及已有 1 个计划保留。下一步用户强制刷新、重新打开长列表，在较矮窗口复核滚动与按钮可达性，不自动验收。
- 2026-10-15 按用户“发布到2.200”要求，F-049 已通过现有一键发布入口部署到 `http://192.168.2.200:19000`，线上源码版本为 `128a8e07c3f348b5b704a8e30d1537dc18ea14ae`。六服务健康，HTTP、执行器协议、Chromium 启动和日志双向挂载验活通过；另行核对 1079 个发布文件与本次源文件一致、首页引用的 23 个 JS/CSS 与本地正式构建逐字节一致，三套新皮肤资源已上线。配置文件摘要、全部挂载、MySQL 容器及配置表单维护版本保持；计划与运行数量发布前后均为 0。F-049 仍为 **ready_for_manual**，下一步用户刷新页面，在顶栏“外观”核对八种组合与弹窗；发布检查不代替视觉和业务人工验收。
- 2026-10-15 F-049「全界面打磨与三套未来感皮肤」完成并停在 **ready_for_manual**：顶栏“外观”提供默认、深空指挥舱、极光玻璃、钛金精密及独立深浅选择，共八种组合并保持偏好。全局颜色与阅读面、页面留白、11 处弹窗、画布和报告图表已统一；三套背景氛围、刻度、折光和低频掠光完成，指针监听与合成层/玻璃数量受控。统一入口 test/run-f049.ps1 实际通过：外观 15 项 + 相关回归 60 项、类型检查、正式构建与差异检查。26 个现有业务组件模板 AST 核对保持内容、绑定与回调；真实 Pinia/ECharts 内存测试与当前图表更新规划器确认偏好、图例和数据保持。原生表单、执行器、后端、业务 API 与布局算法未改。扩展检查中两条 F-012 旧载荷断言在隔离基线同样失败，已独立记录，未夹带业务修改。下一步按 [F-049 人工清单](features/F-049-futuristic-skins-and-ui-polish.md) 核对八种外观、弹窗、未保存内容、长错误、滚动及空闲 CPU；本轮未启动浏览器，不标 accepted，不自动开始下一功能。
- 2026-10-15 用户明确要求“开始实施改造”，F-049 进入 **implementing**。按已保存的功能文档与视觉规格实施主题基础、页面与 11 处弹窗打磨、三套氛围动效；验证通过后停在 ready_for_manual，原生表单、业务行为与其他功能验收状态保持当前边界。
- 2026-10-15 F-049「全界面打磨与三套未来感皮肤」计划已保存，状态为 **awaiting_approval**：默认保留当前配色并打磨排版、留白和弹窗；新增深空指挥舱、极光玻璃、钛金精密，各有深浅版，共八种组合。计划已写清强氛围动效、原生表单保持原样、换肤不丢草稿/画布/图例/弹窗状态及不影响业务请求的边界，并列出实施顺序、完成标准和人工核对清单。本轮仅修改文档，未实施界面或进行应用验收；下一步待用户明确批准后实施同一 F-049。计划见 [功能文档](features/F-049-futuristic-skins-and-ui-polish.md)；同日完成前端美学评审，新增可直接照做的[视觉与实现规格](features/F-049-visual-spec.md)（色值、变量、文件清单、氛围 CSS、弹窗落点、测试），三套皮肤保留但修订了招牌与材质规则，仍待用户批准实施。F-048 仍为 ready_for_manual，其他功能状态不变。
- 2026-10-15 F-048「事件总览与动作接口详情」完成，停在 **ready_for_manual**：事件流记录节点流转、实际执行人和错误原话；动作弹窗只显示当前动作的有序接口，不串同节点其他动作、不重复列轨迹。修正运行/步骤身份错配，完整保留目标错误，耗时覆盖响应正文，保留实际重试并区分日志缺失与未发送；轮转记录和只读节点事件也已补齐。统一本地验证及后续定向复验通过：Go 相关回归、Node 44 个用例、执行服务类型检查、前端类型检查与构建、Go 构建和差异检查。未进行真实目标平台与浏览器视觉验收。下一步按 docs/features/F-048-run-log-and-action-detail.md 在浏览器核对总览/局部分工、接口顺序、错误原话与长内容布局；不自动验收或开始下一功能。
- 2026-10-14 F-047「新建计划只开放新发起」完成，状态为 **ready_for_manual**：新建页把“已发/待发”置灰，页面与服务端都只允许新建“新发起”；旧的已发/待发计划仍可编辑名称等内容，保存后保留原来源与流程对象。前端来源测试、后端创建与编辑测试、执行器类型检查、Go 构建和差异检查通过。下一步请在浏览器核对新建选项，并对已有旧来源计划做一次名称编辑保存；不自动标记 accepted。
- 2026-10-14 F-046「转发完整执行与附言保存」实施完成，状态进入 **ready_for_manual**：转发统一走原生页面 `confirmTranspond`，主请求成功并返回新实例 ID 后才允许保存当前附言；历史附言勾选只控制历史内容复制，空或非空当前附言都照目标页面保存。两笔请求按 `transpond → flowInstancePostscript/save` 顺序各发一次，实例 ID、接收人和请求证据独立保留；主请求不明确成功、附言失败/丢响应/错实例时停步并锁住后续写入，不重发主请求；执行服务不可用时转发写前阻断，不回退到单笔直连。自动验证通过：后端定向 Go 测试、执行器 Node 61 项、TypeScript 类型检查、`go build ./cmd/server` 和差异检查；真实受控运行 217/路径运行 810 实际得到一笔转发主请求和一笔附言保存，均 HTTP 200，附言保存与新实例 ID 一致，只读回查返回 1 条记录。下一步用户在浏览器核对转发详情与页面结果，不自动标记 accepted、不开始下一切片。
- 2026-10-10 F-042「报告中心真实度量」按用户人工验收反馈完成整改，重新停在 **ready_for_manual**：
  ① 加载非常慢——只读定位到根因并实测取证：每次报告读取按路径运行串行扫 `network.log`（工作区 83 个文件 / 66.9 MB / 118,919 行，按现有逐行建 map 口径解析耗时 1.13 s，纯 IO 仅 31 ms）与 `step.log`（66 个文件 / 131.4 MB），并对每条路径运行各发一次 `ListRunAttempts`（N+1）；
  ② 参数和层级不合理——`ReportMetricsPanel` 在 L0/L1/L2 原样复用，5 张 KPI + 4 张图重复三遍；
  ③ 缺名词解释；④ 排版需优化；⑤ 数据要细致权威（无分子分母、无样本数、无统计截止时间）。
  用户已裁决（2026-10-10）：记为 F-042 人工验收反馈并返回实施；层级按「总—分—总各自只放自己的一等信息」收敛；名词解释「行内 info 图标 + 右上角指标口径说明总入口」两者都要；数据只做细不做多（分子/分母、样本数、口径、数据截至时间上卡片，不新增指标种类）。
  本轮计划与实施记录见 `docs/features/F-042-report-center-real-metrics.md`（提交 `9b24c86`/`aa8bf2a`/`6946a6f`）：① 后端改为一条 SQL 取回路径运行日志目录（去掉逐条 `ListRunAttempts` 的 N+1）+ 只扫 `network.log` 的快速聚合（不建 DTO、不排序、不读 `step.log`）+ 固定 8 并发 + 终态路径运行按文件签名缓存；接口耗时口径修正为按路径运行全量请求行统计（原按尝试归属漏计 `step_id=-` 的登录/结构读取行，工作区 38,414 行、约占 12%）；工作区真实日志实测冷启动聚合 14.6 ms（旧实现仅解析 `network.log` 就 1.13 s，且还要再扫 131.4 MB `step.log`），二次读取命中缓存 0.55 ms；② 层级收敛——整套指标面板只在 L0，L1/L2 改为「核心指标摘要 + 本级主体表」；③ 名词解释落地为行内 info 图标 + 三个页面右上角「指标口径说明」总入口（`glossary.ts` 29 条术语，含算式、分母、来源与易误解点）；④ 数据做细——比率卡带分子分母、分位带样本数、结果/失败分类图下补数值明细表、三页显示「数据截至 …」；⑤ 排版统一（结果状态标签、表格横向滚动 + 粘性表头、数字右对齐）并把 ECharts 改懒加载（569 KB 独立 chunk）。验证：`go build ./...`、`go vet`、后端新单测 + `run_request_timings` 回归、真实 MySQL 集成（含新增 `TestF042ReportInterfaceStatsFromNetworkLog`）、3 个前端 node 用例（16 例）、前端类型检查与构建全部通过，汇总入口 `test/run-f042.sh`。不新增表与迁移、不新增指标种类、不改 L3 运行详情。下一步由用户在浏览器核对速度、层级、名词解释与数字口径，不自动验收或开始下一功能。
- 2026-10-09 F-023 一键更新发布补齐完成，停在 `ready_for_manual`：本机双击 `deploy/update.cmd` 自动读被忽略的服务器凭证，无需再次输入；打包、上传、构建、六服务切换与验活已经在 `192.168.2.200` 实际通过。包边界/脏源码 3 项测试、Linux 故障与并发/代理恢复 4 项验证通过；配置摘要、三个数据卷、迁移和计划数量保持。下一步用户使用入口并核对页面和真实流程，不自动验收或开始下一项；使用说明见 `deploy/README.md`。
- 2026-10-08 F-023 按用户指令完成 `192.168.2.200` 部署，停在 `ready_for_manual`：`/opt/test-auto-pro-v2` 的 MySQL、后端、前端、配置表单、独立执行服务、日志查看器六服务全部健康；主应用 `19000`、日志查看器 `19002`。23 个现有迁移、HTTP/API、Chromium 启动、执行器连接、日志双向挂载与数据库重启保持验证通过；撤除临时代理并重启 Docker 后再次验活通过。使用独立新工具库，未导入开发机计划/运行记录，既有 V1 保留。未发送目标业务写请求或自动打开浏览器；下一步用户核对页面、表单编辑及真实流程，不标记 `accepted`，不开始下一功能。部署命令见 `deploy/README.md`，服务器实测证据见 F-023。
- 2026-09-28 F-044 运行 16 路径 5 人工复验未通过，返回 `implementing`；本轮已完成修复并重新停在 `ready_for_manual`：step 50 trace `8d2e8f9396adbfb8` 的 runtime 页面装载前快照有 `accountantUserName__formPersonId`，提交后页面快照与实际 `/flowInstanceApi/audit` 请求缺失该伴生键；目标以 HTTP 200 业务失败拒绝“财资部-会计”表单人员解析。根因确认在工具侧 runtime 回填/回读链，不是目标平台随机失败。新增独立 `readonlyFields` 装载依赖，审批页回填 form_person 源字段及伴生键，但不进入节点编辑权限或本步变更字段；定向 Go 测试、vet、`go build ./...`、执行服务 typecheck 和差异检查通过。后端 `19080`、执行服务 `19011` 已重启并健康，未发送新的目标写请求。下一步用户再次重跑路径 5。
- 2026-09-28 F-044 运行 12 路径 5 加签人工反馈未通过，返回 `implementing` 后修复并重新停在 `ready_for_manual`：`updateFlowProxy` 写入已成功，写后实例当前节点属于新代理树，但接口响应单一 `currentNodeProxyId` 与实例事实不同，旧控制层误判结构不一致并停机。现改为以写后实例 `CurrentNodes` 完整集合验证新代理树，响应游标只作诊断；非加签代理重建仍保留严格校验。执行器/控制层单测、vet、Go 构建通过，后端已热重载，未发送新的真实写请求。下一步用户重跑路径 5，确认加签后继续插入审批节点和后续节点，不标 `accepted`。
- 2026-09-28 F-044 运行 11 人工反馈未通过，返回 `implementing` 后修复并重新停在 `ready_for_manual`：旧动作广度序在拓扑调整后造成调度启动前结构错误；编译器按真实图归一旧节点序号，手工节点保存、一键配置与运行前检查共用编译规则。不同意/撤回重提与退回分叉前恢复均重审此前完成的兄弟支线；仅兄弟待办活动不足以证明本支已跳过。运行详情同时高亮目标全部活动节点，写请求仍逐步核验并保持同一主实例。只读预检实际计划 67 路径 1-6、9、10 均可运行，7、8 仍因历史数据路由与嵌套分支选择阻塞。定向 Go 单测、前端类型检查、构建与流程图测试通过；无目标写实测、无浏览器自动验收。下一步由用户重跑可运行路径，核对各分支待办/汇合/恢复事实；不标 accepted。
- 2026-09-28 F-044 运行 9 人工反馈修复后重新停在 `ready_for_manual`：路径 1/3/4/6/9/10 的多活动待办不再由最小步序掩盖；路径 2 的裴国英所在合规支线必须在公司顾问主线前完成；路径 5 的加签插入点以下重新对齐并行分支，重绑失败后自动落 `stopped` 并退出循环。相关 Go 测试、定向 vet、构建和差异检查通过；计划库集成测试因连接配置缺失而跳过，未连接目标平台、未发真实写请求、未启动浏览器。运行 9 既有路径 5 状态未回写（本机后端未监听且缺计划库配置），待服务恢复后通过停止入口处理；用户重跑同路径人工验收，不标记 accepted。
- 2026-09-28 F-044 运行 8 人工验收反馈已修复并重新停在 `ready_for_manual`：路径 1/4/6/10 的实例私有代理换 ID 后，执行器只允许按唯一同名节点重绑定，并把实时节点 ID 贯穿 Go 写前任务刷新、runtime `nodeProxyId` 与写后核验；路径 2 的未激活并行兄弟分支按真实图关系零写入跳过，不再误套模板 `isSkip`；路径 3 在人员选择器主字段仍存在时允许目标清洗 `__formPersonId/__condition/__virtualName` 伴生字段；路径 9 将同一恢复触发组的后续恢复同意纳入合法前进锚点。场景编译、执行器、服务、动作编排与目标适配层测试、定向 `go vet`、`go build ./...` 和差异检查通过；未连接目标平台、未发送真实写请求、未启动浏览器。下一步由用户重跑“欧阳改测试2__运行8”的同一路径，F-044 不标记 `accepted`。
- 2026-09-28 工作区清理与 F-035 单项修复完成：先将上一轮未提交的并行/场景草稿保存为可恢复 stash `清理前保存：上一轮并行与表单修复草稿`，再在洁净基线上实施本轮修复。运行 7 路径 2 的 `currentDepName` 部门变化不再被跨节点表单核对误判为业务值改写；目标登录上下文统一归类为 `target_context`，写后正文保护排除这些会话字段但继续阻塞真正丢失的业务字段；阻塞消息新增具体缺失字段名。新增回归测试，`go test ./test/unit/backend/form_data_by_node ./test/unit/backend/executor ./test/unit/backend/target` 通过。F-035 仍保持 `implementing`，并行分支独立执行/汇合、加签后路径校验及真实目标请求对照尚未在本原子切片内完成；未连接目标平台、未启动浏览器。
- 2026-09-27 计划状态阻塞、串行调度卡死与发起态自选分支崩溃根因复核与修复完成，切片停在 **ready_for_manual**：
  ① 计划列表运行中与删除阻断：根因为 planDerivedStatusExpr 过度判定 pending 为 running，且 Delete/BatchDelete 事务内将 pending 排队误算为运行占用阻断删除，加上 CreateRun 过早刷写入 running；修复为仅以存在活跃在跑路径的真实 running 为准，排队 pending 允许正常删除并级联清理 plan_run_queue，CreateRun 延后至真实出队或启动才同步计划运行态。
  ② 串行队列卡死与开始时间丢失：根因为 DequeueRun 从 plan_run_queue 取出运行记录推进时遗漏了设置 started_at，导致界面开始时间为空；且历史删除未级联清理等待队列产生孤儿排队行；修复为 DequeueRun 补齐 started_at 赋值并在 DeleteRun/Delete 事务中级联清理 plan_run_queue。
  ③ 发起态自选分支崩溃与后续审批自定义组件容错：Plan 67（合同盖章评审）首步发起报 opinion.handleSaveChooseBranch is not a function；根因为 native-dispatch.ts 中针对自选分支（custom_choose / run_node_choose）硬编码调用审批态 ExamineOpinion 组件，未适配发起态 InitFlowDialog/FlowDialog；且 FlowDialog 真实原生保存方法名为 handleSaveBranchChooseNode（此前误调 handleSaveChooseBranch 报方法缺失）；修复为在 FlowDialog 与 ExamineOpinion 均统一设置 chooseBranchNode 与 chooseBranchList，发起态补充 nextNodeProxyId，自适应调用 handleSaveBranchChooseNode，彻底闭环发起态自选分支继续；同时排查到后续审批节点因表单未挂载自定义组件抛出 TypeError 阻断审批请求发出（报动作没有发出期望写请求 /flowInstanceApi/audit），在 nativeDispatch 的 dealFormBusiness 增加容错平滑回退标准流程审批；在 session-manager.ts 挂载 page.on('console'/'pageerror') 补齐原生页面异常诊断。
  ④ 验证与部署：新增 TestF032PlanPendingDoesNotBlockDelete、actions.test.ts（发起态自选分支闭环与选人测试、缺失自定义业务组件时平滑回退标准审核测试共 17 项）、plan_last_run_text_test 回归用例；go test（单测/集成）、actions.test.ts、tsc --noEmit、go vet、git diff --check 全数通过；重新编译构建 .runtime/server.exe 并热重启后端服务（19080）与执行器服务（19011），已核验 203 库历史阻塞计划 67、68、69、70 全部回归终态可正常删除。
  ⑤ 评审与修复文档：`docs/review/2026-09-27-execution-plans-and-dispatch-review.md`。
- 2026-09-26 Plan 59 / Run 172 根因复核与修复完成，切片停在 **ready_for_manual**：路径 1、6 的停止根因为取回准备同意被目标 `isSkip=true` 自动跳过后，工具仍把取回当作有已办记录的可执行动作；路径 4 的停止根因为动态加签审批步骤未继承被目标跳过的恢复触发键。工具现按前一步已落账的“目标自动跳过”事实将取回零写入记为 `target_skipped` 并继续，动态加签步骤继承恢复来源和 `SourceActionKey`，触发动作跳过时一并记为 `recovery_not_applicable`。路径 9、10 的取回接口返回成功且有取回审核记录，但目标后续响应仍缺少 Java `saveRetrieveProcess/createPendingJob` 预期的 pending/当前处理人，暂记目标平台待确认，不在工具侧加兜底。新增取回与动态加签回归测试；`go test` 受影响包、`go vet`、`git diff --check` 通过。未连接目标平台、未发真实写、未启动浏览器；待人工复跑路径 1、4、6、9、10。
- 评审与修复文档：`docs/review/2026-09-26-plan59-run172-review-and-fix.md`。
- 2026-09-26 Plan 59 / Run 171 Java 对照复核与工具侧修复完成，切片停在 **ready_for_manual**：确认 6 条停止路径均为工具侧问题。按 Java `FlowOperateServiceImpl` 的 `isSkip=true` 递归跳过语义，`RetrievePrepareConsent` 现在会在目标已越过无人节点时零写入记录 `target_skipped`；不同意/撤回/回退被跳过后，恢复链读取内存或最近落账尝试事实并标记 `recovery_not_applicable`，不再发送无效 `reSubmit`；取回写后读取最新实例代理和当前节点集合，使用现有 `ProxyRemap` 结构映射，当前节点缺失或不在新树时停止后续放行。新增目标跳过、重启恢复和代理节点安全校验回归。受影响 Go 测试、`go build ./...`、受影响包 `go vet`、差异检查通过；完整 `go test ./...` 已跑到全部 Go 包及契约测试通过，但命令在后续长耗时阶段超时，未将其宣称为全量完成。未连接目标平台、未发真实写、未启动浏览器；待用户人工核对路径 1、3、4、6、9、10 后再决定是否接受。
- 2026-09-26 执行器服务启动修复完成（分支 `执行引擎改造`，3 处代码提交）：
  现象：`pnpm --dir executor-service start` 报 `缺少 flow-source/dist` 退出；启动日志先出「自动同步 rsh-flow-components 告警 … node.exe ETIMEDOUT」。
  根因链：`server.ts` `ensureLatestFlowSource` 把远端同步与本地构建放在同一个 try——同步一旦抛错，构建被整段跳过，dist 从不产出，`extractPageDist` 抛错退出；`sync-flow-source.js` `cloneAt` 先 `rmSync(flow-source)` 再就地 fetch，同步中断会把 babel.config.js/dist/.git 连同工作副本清空；babel.config.js 被 gitignore、只随克隆落地，缺失后 vue-cli 构建再无法进行。
  修复：① `server.ts` 拆分同步（尽力而为、失败仅告警）与构建（始终执行），`EXECUTOR_DISABLE_AUTO_SYNC=1` 只关远端拉取、不再连带跳过构建（提交 `b5bda7d`）；② `build-native-page.js` 新增 `ensureBabelConfig`，babel.config.js 缺失时按上游标准预设兜底恢复（提交 `b5bda7d`）；③ `sync-flow-source.js` `cloneAt` 改为临时目录克隆成功后原子替换，失败时旧 flow-source 原样保留（本提交）。
  纠正上一条（9-25）运维注意的错误判断：`git@192.168.1.155` 实际可达——本机 ssh key 直连成功（`ssh -T` 返回 GitLab 欢迎、`git ls-remote`/`fetch --depth 1` 均约 2s、`refs/heads/test`=`e141a79`），babel.config.js 上游有跟踪、克隆会带出；此前 ETIMEDOUT 属瞬时/首连叠加破坏性 cloneAt 造成的连锁，并非远端不可达。因此本机运行 executor-service 不再必须带 `EXECUTOR_DISABLE_AUTO_SYNC=1`。
  验证：`npm run typecheck` 通过；两脚本 `node --check` 通过；本地 `build-native-page` 成功产出 dist；起 executor-service 后 `/health` 返回 `{ok:true,...}`、`/flow` 页面正常装载；`cloneAt` 临时替换/失败回滚语义以隔离脚本实测（失败保留旧目录、成功原子替换）。真实浏览器流程仍由用户人工核对。
  更新收口（用户「更新」指令）：`--pull-latest` 同步 flow-source 到 test 最新 `e141a79`（`flow-source-manifest.json` 锁定 head 与 `flow-source.lock.json` digest 同步更新），重建 dist；查明启动同步 60s 超时线正是原始 ETIMEDOUT 真凶——一次全量 checkout+FormMaking 补齐+全树摘要实测约 60s，已把 `ensureLatestFlowSource` 同步子进程超时 60s→180s（提交 `a144c8d`）。auto-sync 启用下起 executor-service 端到端验证：日志「远端 e141a79 已是最新（未变更）」「dist 已最新跳过构建」、监听 19011、`/health` ok、`/flow` 装载正常；冒烟遗留监听进程已清理，端口无残留。
- 2026-09-25 Plan 59 / Run 169 最新复审（`docs/auto/2026-09-25-plan59-run169-latest-review-repair.md`）五项修复实施完成，停在 **ready_for_manual**（分支 `执行引擎改造`）：
  ① §3.1 系统结构字段（`flowInstanceId`）统一从业务载荷构造（`formdata.go`）与 runtime 写后正文保护（`executor.go` `protectRuntimeFormBody`→`businessFormKeys`）剔除，复用 `target.IsSystemStructuralField` 唯一入口，实例 ID 由顶层协议承担（提交 `354a81a`）；
  ② §3.3 `sessionFactsForRuntime` 改为返回 `(SessionFacts, error)`，runtime 身份读取能力缺失/失败/字段缺失一律写前 `GateBlocked`，与 `nodeFormData` 同口径，不回退会话缓存（提交 `8f25c51`）；
  ③ §3.2 `advancedByNodePosition` 新增并存分支：本步节点与后续合法节点并存（会签/并行未完全推进）时需本步执行人完成证据（优先按稳定任务关联行 ID `CurrentTaskLinkID`↔`CompletedTaskLinkIDs` 绑定，回退 CheckedFound+本节点匹配）才判已推进；只有别分支推进不成功（提交 `f60b531`）；
  ④ §3.4 `run.Service.Finish`/`RunStateControl.Finish` 增独立 `detail` 参数，成功/确定失败/结果待确认终态事件统一携带 `settleEventDetail`（节点·动作·目标写端点·结果），`StepOutcome.SettleDetail` 带回最后一步明细供 `finishCompleted` 写入（提交 `59579d6`）；
  ⑤ §3.5 采「仅要求 runtime 动作」方案（不新增数据库列/迁移）：`RunNodePanel.vue`「目标平台原始返回」改名「runtime 目标平台原始返回」，说明普通 Go 动作沿用「请求的接口」结果摘要与「为什么不能继续」（提交 `3dd3501`）。
  验证：`go build ./...`、`go vet ./...`、`go test ./test/unit/backend/...`、`go test ./test/contracts/...`、`web` 目录 `npm run typecheck` 全通过；`git diff --check` 无异常；新增定向回归覆盖五项。未跑：真实浏览器与目标平台重跑，按报告第 6 节由用户人工验收后再决定是否 `accepted`。
- 2026-09-25 运行 #2「欧阳改测试09」（合同纠纷/非诉讼登记）第 2 步 resubmit 装载 60s 超时只读定位并修复（非新功能，属运行引擎双平台码适配缺陷）：
  根因：目标为 200001/999999 双平台，同一账号在不同平台发起的实例分属不同平台码——机房进入实例=200001、合同纠纷实例（`0d7f17a2ebb24fbdbbe4579cf0b4182a`）=999999（`flowInstanceApi/list` 实证）。运行时页面 axios 默认 `platformCode||200001`，实例重建类装载（重提/编辑/审批/转发）对 999999 实例一律用 200001 查询，详情弹窗 `examineDialogVisible` 永不置真，`waitForDetailReady` 的 `page.waitForFunction` 60s 超时、装载失败；200001 实例恰好匹配才正常。该次运行所有转发请求 platformCode 均为 200001，与实例 999999 不符。
  修复（全部落在主仓 Go 唯一目标出口，避免依赖易被 auto-sync 覆盖的 flow-source 副本）：`findSubmittedFlowFacts` 捕获实例 `platformCode` → `InstanceFacts.PlatformCode`（`verify.go` 映射）→ `RuntimeRunRequest.PlatformCode`（`executor.go` 取 `preview.Facts.PlatformCode`，发起类实例不存在留空）；`runtimeexec.Client` 新增按会话登记的 `sessionPlatform`，`RunAction` 装载前 `SetSessionPlatformCode`、销毁时清除；读泵转发 `handleForward`/`forwardOnce` 携带该平台码，`ForwardRuntimeRequest` 以「运行事实为准」覆盖查询 `platformCode`（与 sid 同一纪律），非空才覆盖、发起类保留页面默认。
  验证：新增 `test/unit/backend/target/instance_platform_code_test.go`（实例事实捕获平台码）、`runtime_forward_path_test.go` 增 `TestForwardRuntimeRequestOverridesPlatformCode`（非空覆盖/空保留）、`loader_test.go` 增平台码透传用例；`go build`/`go vet`/相关包 `go test` 全通过；重建并热重启 `.runtime/server.exe`（后端 `/api/health` ok）；重建 flow-source/dist 并以 `EXECUTOR_DISABLE_AUTO_SYNC=1` 起 executor-service（`/health` ok）。待人工浏览器核对：对 999999 实例执行 resubmit 应不再 60s 超时、详情弹窗正常打开。
  运维注意：本机无法连到 flow-source 远端（`git@192.168.1.155` ETIMEDOUT），executor-service 启动自动同步会删旧目录再 clone、clone 超时后把 flow-source/dist、babel.config.js 连同 .git 一并清空（本次已两次触发并恢复）；在本机运行 executor-service 必须带 `EXECUTOR_DISABLE_AUTO_SYNC=1`。修复不依赖 flow-source 自定义改动，远端可达环境正常 auto-sync 亦不受影响。
- 2026-09-24 运行 #1「欧阳改测试08」第 2 步（重新提交 resubmit）表单写后核验误报「字段 myCompanyName 发送的值未被目标保存」只读定位并修复（非新功能，属运行引擎表单写后核验口径缺陷）：
  根因：工具侧按目标选择器规范发送标准 JSON 字符串 `{"id":"6e1235cf416e4143947311c6cc93d78e","name":"广东斯能投资有限责任公司"}`。目标平台后端在存储表单数据（`/web/flowInstanceApi/submit` 及 `getCurrentFromData`）时，对部分公司选择器只保留核心标识 `{"id":"6e1235cf416e4143947311c6cc93d78e"}`，省略冗余的 `name`（其伴生字段 `myCompanyName__condition` 则单独保留了中文名）。写后核验工具函数 `deepEqualNormalized` 原先仅做严格的 JSON 序列化字符串比对，把目标的瘦 ID 对象与工具发送的完整对象判定为不相等，产生误报并阻塞了后续步骤。
  修复：在 `internal/engine/step/formdata.go` 的 `deepEqualNormalized`（及导出的 `DeepEqualNormalized`）中增加选择器 JSON 等价容差：当双方均为合法选择器 JSON 且 `id` 相同非空时，若一方省略 `name` 或双方 `name` 相同，即判定等价，不再误报丢失。
  验证：新增定向单元测试 `test/unit/backend/executor/form_data_equal_test.go`（覆盖 id 相等省略 name、反向等价、不同 id 拦截、不同 name 拦截、普通字符串）；后端 26 个测试包全部通过，go vet/go build 通过；重新编译并热重启 `.runtime/server.exe`。


- 2026-09-24 运行 #2「欧阳改测试07」路径 1 失败与路径 12 反复跳过第 25 步 45 次只读根因定位与修复完成：
  ① 路径 12「跳过 25 的什么鬼」（45 步跳过与最终已停止）：根因为第 25 步系恢复链插入的「取回准备同意」步骤（RetrievePrepareConsent），首次真实同意成功后因审核人 10 为 2 人会签节点实例仍停在原地，被错误送入会签重跑循环（control.go:1438）；在重跑构建预览时 RetrievePrepareConsent 判定本节点已办结返回 TargetSkipped=true，但循环内部未检查该标志，继续把跳过当成功落账，且跳过不改变实例节点导致死循环空转 44 次直到场景步数上限（45 次第 25 步）。紧接着第 26 步取回命中目标会签未完结取回（FlowInstanceServiceImpl.java:929），目标仅将记录设为失效未恢复待办，致第 27 步查无待办卡住轮询后停止。修复：control.go 明确隔离 RetrievePrepareConsent 严禁进入会签循环；会签循环内部一旦遇到 extraPreview.TargetSkipped=true 立即 break 退出循环，彻底消除空转。
  ② 路径 1「审核人5 失败/处理人身份未确认」：根因为审核人 5 为抢签节点（type=scramble，16 名候选人），其中「赵飞-中润科技」在目标配置中的 biz_id=13e0b5da46d6449baa620d1620cb1df0 系用户中心 t_user 已不存在的历史旧 ID，静态排除无法命中；switchToCurrentHandler 通过视角查到待办后尝试登录赵飞会话失败（无权限!），但代码误认定并直接返回赵飞为 found=true，导致执行阶段 prepare 登录失败整条路径停机。修复：executor.go 升级 switchToCurrentHandler 容错，当候选处理人账号登录失败时记录诊断并 break 尝试 matches 中下一位候选人，发挥抢签节点任一可用人推进的设计契约，绝不被单人凭据不可用阻断流程。
  ③ 验证：新增定向单元测试（TestRetrievePrepareConsentDoesNotEnterCountersignLoop、TestSwitchToCurrentHandlerSkipsUnloginableCandidateAndPicksNext）通过；后端 26 个测试包全部通过，go vet/go build 通过；编译更新并热重启 .runtime/server.exe。
- 2026-09-24 多动作写后核验审计：移交/取回/不同意与会签末签同型的「待办快照晚一拍」漏判已只读定位并修复（非新功能，属运行引擎写后核验口径缺陷）。根因：三类动作目标已在同一事务改写实例，但本人待办快照晚一拍返回旧值，只看待办会把成功写判成「明确未变→暂时无法确认」。修复按动作取目标同事务落库、不被改写的原子正面事实：不同意认实例 `status=rejected`（回退起始节点，`FlowAuditServiceImpl.audit`）优先于滞后待办；移交认原任务关联行 `auditStatus=transfer` 审核记录（`approverAppend`）；取回认原办结关联行 `auditStatus=retrieve` 审核记录（`retrieveProcess`，门禁已确保写前无该记录）。审核记录读不到时一律不补强。改动 `internal/engine/step/verify.go`（`ClassifyReread` 不同意状态优先）与 `executor.go`（新增 `hasAuditRecordForTaskLink` + `classifyTransferReread`/`classifyRetrieveReread` 审核记录兜底）；新增定向单测（不同意状态滞后待办 2 例于 `step_test.go`、移交/取回审核记录兜底 2 例于 `task_action_facts_test.go`），`go build`/`go vet ./internal/engine/...`/后端执行器单测全绿。语义见 `docs/TARGET_SEMANTICS.md` 第 9 节。
- 2026-09-24 用户报告运行「欧阳改测试07 路径 1」第 18 步（审核人3，会签满足 3 人）又出现「目标接口已返回成功，但执行结果暂时无法确认」并要求修复为明确结论。已只读定位真实根因并修复（非新功能，属运行引擎写后核验口径缺陷）：会签末签处理人（张宏伟）同意后目标立即把实例从本节点 c82e 推进到后续节点 42aa，此刻本人待办快照晚一拍消失（`CurrentTaskFound` 仍为真）、本人已办行 `flowNodeProxyId` 被改写成推进后的新节点（本节点等值过滤命中不到→`CheckedAbsent`）；而写后核验的「已推进」判据 `advancedByNodePosition` 只认紧邻下一跳这一单一锚点，真实落点是被目标穿透空/无处理人节点后的更靠后合法节点，于是漏判、落进「待办仍在+已办 CheckedAbsent」审批分支得「成功声明+明确未变→不确定」并让整条运行终局停止。修复：把单一锚点 `expectedNextTargetNodeID` 换成「本步之后所有合法路径节点集合」`forwardPathTargetNodeIDs`（运行期分支选择锚点 + 全部后续用户步骤节点，恢复链/系统导航排除），实例当前节点命中集合内任一节点即判「已前进」；越到配置之外的节点仍不认成推进，交由独立路径偏离判定处理（与既有偏离判定 `hasValidNextNode` 对齐，消除两者不对称）。改动 `internal/engine/step/verify.go`（`ClassifyReread`/`advancedByNodePosition` 改收锚点集合）与 `executor.go`（新增 `forwardPathTargetNodeIDs`、替换两处调用、删除旧 `expectedNextTargetNodeID` 及其测试导出）。更新 `docs/TARGET_SEMANTICS.md` 第 9 节新增会签末签核验口径。验证：`go build ./...`、`go vet ./internal/engine/...` 通过；`test/unit/backend/executor` 全绿，新增/改写定向用例（会签末签跨节点前进判已推进、配置外分支仍不判成功、`forwardPathTargetNodeIDs` 覆盖后续全部节点/去重/排除恢复导航）。下一步：用户在浏览器重跑该会签路径核对第 18 步末签判为确定成功、运行继续到后续节点，不再停在「暂时无法确认」；未获明确验收不自动开始下一功能。
- 2026-09-24 用户报告「一键配置历史数据」两处缺陷并要求修复，已只读定位根因并收敛为切片 F-043，状态 **awaiting_approval**（未获批准不进入 implementing）：
  缺陷①（当前必填被放行）：配置期就绪判定唯一出口 `internal/service/path_data_workspace.go` `workspaceDataStatus`（L758-768）只在有 `Blocking` 问题时不就绪，且明确不依赖浏览器校验；`GetData`/`SaveData` 汇总的 issues 只覆盖分支/静态下拉/公司同步/身份/决定性分支字段，**没有按当前模板「必填+可编辑」校验有效值是否为空**，故历史为空而现在必填的普通字段被判 ready 放行。事实齐备：字段 `Required` 在 `client_fact_reads.go:706`/`types.go:288,346`，可编辑时机在 `path_runtime_projection.go`。
  缺陷②（残留历史发起人）：配置期会话与 `load` 载荷已用计划账号身份（`path_config.go:150` RuntimeSession、`FormRuntimeFrame.vue` load L141-158），但历史正文的申请人/部门/公司字段未被覆盖——`replaceUserIdentityValues`→`target.ApplyUserIdentityForAction`（`protocol_matrix.go:525-562`）只覆盖固定键集（global_user_basic_information/expenseUserId 等），未含本表单当前用户字段。参考门户 `rsh-unified-vue3-web-test` 的 `buildFlowUrl`（useFlowApproval.ts:239-296）传入 sid/userId/companyId/customerCode 及 userName/companyName/userDepartmentName 供表单解析当前用户。
  计划见 `docs/features/F-043-config-form-data-required-and-identity.md`。2026-09-24 用户选定「有限安全补齐」方案（否决全量随机生成器）：①对当前模板必填但历史为空且可编辑的字段按类型做确定性安全补齐（模板默认/options 首项/日期/数字/布尔/无约束文本占位，不随机），文件上传用项目占位文件一次性真实上传目标并缓存返回引用复用，自定义组件/业务敏感/不可解析远程等无法确定有效值者标 needs_input 带字段名并支持一次填写跨路径传播；②当前环境人员字段注入计划账号身份、身份键对齐目标契约。不逐个反向工程覆盖全部自定义组件、不随机造业务数据、不用静态文件冒充目标上传、不改 form-runtime fork 源码、不新增表或迁移。下一步：等待用户明确批准进入 implementing，先读 fork 当前用户解析、目标文件上传接口与真实模板确认补齐/身份判据。
  2026-09-24 已获批准进入 **implementing**，完成两项原子成果并提交（798dea4、4a5b74d）：缺陷①安全补齐引擎 `internal/service/path_data_fill.go`（按类型确定性补齐：文本占位/数字 json.Number 保真/单多选取有效选项/日期今天/开关 true/颜色；人员组织/级联/未知自定义/未上传附件落 needs_input 带字段中文名）接入 GetData（只读预览、附件非阻断待上传）、SaveData、一键回放 `replayItem` 三入口，发起态权限统一取 `initiatorFieldPowers`（不依赖 reachable）；文件上传 `internal/adapter/target/client_file_upload.go` 的 `UploadFile`（multipart `/web/file/api/file/uploadFile`，等待上限由 ctx 决定=180s 受限超时、只发一次不自动重发，非业务写不进白名单）+ `path_data_upload.go` 账号级进程内一次性上传缓存与目标各组件可识别的超集回显值。`go build`/`go vet` 通过，新增定向测试（确定性补齐、只读/隐藏/非必填跳过、无法确定落阻断、附件待上传/上传回填、multipart 契约、超集形状）+ 后端单测全绿。缺陷②身份注入**受阻**：现有身份替换是固定键映射且刻意不动 `contractUserName` 这类真实选定他人（`path_data_company_sync_test.go` 已锁定），模板无「默认取当前登录用户」标记（fork `valueOrigin` 仅 `fromUser`），必须先拿到机房进入申请表当前用户字段的 englishName（或历史 raw 表单样本）才能安全扩展映射，已请用户提供。文件上传值为超集数组，异形上传组件待浏览器人工核对。下一步：F-043 仍在 implementing——收到字段键后补缺陷②与跨路径传播、补定向测试，再收口 ready_for_manual。
  2026-09-24 缺陷②身份注入按用户裁决换方案收口，F-043 停在 **ready_for_manual**：放弃“扩展固定键映射/补当前用户字段 englishName”的老路（该路需机房进入申请表字段键、且会误伤真实选定他人），改为“注入正确身份 → 表单初始化事件按计划账号填好当前用户字段 → 回放历史业务数据时剔除这些字段、不覆盖”。两侧对称落地：运行执行侧 `executor-service/src/data-patch.ts` 的 `applyNodePatch` 先读初始化值、`dropCurrentUserFields` 按“值内嵌当前账号 userId/companyId”判定当前用户字段并从补丁剔除，`server.ts` 的 `applyNodePatchViaPage`/`restoreIntendedPatchViaPage` 透传 identity（提交 965e821）；配置预览侧 `form-runtime/src/App.vue` load(formmaking) 在 setData 历史值前调用新纯函数 `runtime/identityFields.js:stripCurrentUserFields`，从待写入值与恢复基线剔除当前用户字段及人员选择器伴生键（`__virtualName`/`__formPersonId`/`__condition`），判据只看值不看名故不误伤业务里真实选定的他人（提交 2f853ed）。验证：executor typecheck 通过、data-patch 单测 5/5；`form_runtime_test.mjs` 61/61（新增剔除/回落/不误伤 3 例）；`go build ./...` 与后端 history_replay/executor 单测通过。范围决定：完成标准 61“无法自动补齐的 needs_input 字段手填一次跨共享路径自动传播”（效率项，非正确性门禁）经用户明确选择“先收口”暂不实现，另开后续切片；缺陷① needs_input 正确性门禁与运行前检查阻塞已在 798dea4/4a5b74d 落地。下一步：等待用户在浏览器与真实目标核对缺陷①补齐与缺陷②身份呈现，未获明确验收不标 accepted、不自动开始下一功能。
  2026-09-24 缺陷②人工核对反馈未通过并重做,重新停在 **ready_for_manual**:用户反馈一键配置后当前登录人字段仍显示历史发起人,并提供机房进入申请表初始化事件代码。真实根因:目标表单 `eventScript` 里 `key='refresh'`（名「初始化当前登录人信息」）从 `window.localStorage` 的 `invest-power-system-*`（userName/userDepartmentId/userDepartmentName/companyId/companyName 等）读当前登录人填申请人/部门/公司,但用「仅空值填充」守卫（`if(!this.getValue('applicantName'))...`）;历史回放时这些字段已被历史发起人值填满,守卫跳过,故残留历史值——此前「注入身份→初始化事件覆盖→回放剔除」的前提（初始化会覆盖）不成立,推倒重做。新方案（不硬编码字段名/englishName）:初始化完成后用表单自身初始化脚本在「空值探针」下（getValue 归空、setData 捕获、triggerEvent/display/validate 屏蔽）重跑所有读 `invest-power-system-*` 的脚本,再用 `pickIdentityDerived` 只挑出「取值内嵌计划账号身份串」的字段 setData 覆盖,业务默认值与历史真实选定的他人不动。配置预览侧 `form-runtime/src/App.vue` `applyPlanAccountIdentity` + `runtime/identityFields.js:pickIdentityDerived`（删除旧 `stripCurrentUserFields`）;运行执行侧 `executor-service/src/server.ts` `injectIdentityScript` 补 `userDepartmentId`/`departmentId`、新增 `applyPlanIdentityViaPage`（仅发起态权威覆盖,审批等模式不动真实发起人）。提交 478e7f8。验证:`form_runtime_test.mjs` 61/61（pickIdentityDerived 3 例）、executor typecheck、`go build ./...` 与后端 history_replay/executor 单测通过。下一步:用户在浏览器一键配置后核对申请人/部门/公司显示计划账号本人;未获明确验收不标 accepted、不自动开始下一功能。
- 2026-09-24 F-042「报告中心真实度量（总分总）」实施与自动验证完成，停在 **ready_for_manual**：
  ① 只读聚合后端：新增 `internal/repository/report.go`/`internal/repository/mysql/report_repository.go`（计数、计划汇总、路径运行行、尝试行的分组聚合 SQL，样本按范围有界拉取，无 N+1/无全表）、`internal/service/report_center.go`（成功率、一次通过率 FPY、路径覆盖度、结果/失败分类分布、步骤与目标接口时延分位、接口读写与重试统计、运行实例耗时三分解）、`internal/api/report.go`（`GET /api/reports/overview`、`/plans/{id}`、`/plans/{id}/paths/{pathId}`），并在 `cmd/server/main.go` 接线；分位一律用 `github.com/montanaflynn/stats` 计算，接口耗时读取复用运行编排服务的 `network.log` 口径。
  ② 前端三级看板：`ReportsView.vue`(L0)/`ReportDetailView.vue`(L1) 改为真实接口驱动并删除全部硬编码假数据常量，新增 `PathReportView.vue`(L2) 与 `/reports/:planId/paths/:pathId` 路由；图表用 ECharts + `vue-echarts`（`web/src/features/reports/`）渲染，L3 复用现有运行详情页；空/错/暂无样本按产品原则用文字状态呈现（无 `network.log` 显示「暂无真实请求耗时」，缺执行器/编排分量显示「未知」，不用 0ms 冒充）。
  ③ 验证：`go build ./...` 通过；新增 `test/integration/f042_report_aggregation_test.go` 在真实 MySQL 上断言聚合口径（成功率 0.5、FPY 0、覆盖度、结果分布、执行器耗时分解与未知降级），与既有 `TestF042ExecutorMsRoundTrip` 一并通过；前端 `pnpm run typecheck` 与 `pnpm run build` 通过。
  下一步：等待用户在浏览器与真实目标环境人工核对（首页统计与任务中心一致、三级下钻数据自上而下对得上、运行实例耗时分解真实、图表清晰无假数据残留）；未获用户明确验收不标记 accepted，不自动开始下一功能。
- 2026-09-24 催办接收人判定、跟踪命名纠正与幂等跳过、取回准备同意防误跳过修复完成，停在 **ready_for_manual**：
  ① 催办待办接收人门禁基于实时实例运行事实判定：`gate.go` 修正 `ctx.HasPendingRecipient` 计算逻辑，当实例处于运行态（`run`）且已发事实返回了待办人（`CurrentHandlers`）或当前节点（`CurrentNodes`）时直接判定为存在可催办接收人，彻底消除发起人因自身名下无待办导致查空拦截的缺陷；催办写请求固定由发起人（计划账号）发出；
  ② 纠正目标原生跟踪命名并实现幂等放行跳过：彻底废除“关注/取消关注”自造名词，前后端统一恢复为目标平台原生术语**“设为跟踪（flowTracking）”**与**“取消跟踪”**；`executor.go` 实现幂等跳过逻辑，当实例已处于跟踪状态执行“设为跟踪”或已未跟踪执行“取消跟踪”时，直接判定放行并标记 `TargetSkipped=true`（记录跳过原因），跳过重复写请求并继续推进下一步，绝不中断执行器；
  ③ 修复取回准备同意步骤被误跳过导致查无审核记录缺陷：`isRecoveryEffectStep` 纳入 `step.RetrievePrepareConsent`，禁止将取回动作前置的“准备同意”步骤送入 `targetSkippedStepReason` 误当成“已被目标自动跳过”；同时在 `retrieveDoneViewUser` 与 `retrieveNodeHasAuditPass` 中引入加签重建代理树后的节点名称辅助匹配，确保取回准备同意真实执行产生审核记录后，取回能够精准定位该已办并顺利执行；
  ④ 自动化验证与服务热重启：后端 26 个测试包全部通过，新增 3 个定向单元测试（取回防误跳过、跟踪幂等跳过、催办接收人判定），`go vet` 通过，前端类型检查通过；重新编译并热重启后台 `.runtime/server.exe`。
- 2026-09-24 规划并经用户批准开始实施 F-042「报告中心真实度量（总分总）」，状态 **implementing**：报告中心当前整页为硬编码假数据（`ReportsView.vue`/`ReportDetailView.vue` 写死常量、`/reports` 不调后端）。
  规划改为只读三级聚合看板——L0 首页上半统计第一级计划情况、下半给专业测试指标（路径成功率/一次通过率 FPY/路径覆盖度/步骤与接口时延分位/停机分类占比/接口读写统计/重试统计）；
  L1 计划详情看各路径指标；L2 路径详情看运行实例指标（执行了哪些接口、接口耗时、工具端编排耗时、执行器耗时分解）；L3 复用现有运行详情。
  只读既有运行记录六表与 `network.log`，严格沿用 F-030 耗时口径（未知显示「未知」，不用 0ms 或阶段差值冒充），不新增运行状态机或运行事实表、不调用目标写接口；
  用户已裁决（2026-09-24）：执行器精确墙钟耗时采用选项 B——独立执行服务真实计时器采集、随动作结果上报，仅补一条最小向前迁移 036 为 `run_step_attempts.executor_ms` 落库；
  统计分位与图表一律使用成熟热门库（Go `github.com/montanaflynn/stats` + 前端 ECharts/`vue-echarts`），不手写分位与图表。计划见 `docs/features/F-042-report-center-real-metrics.md`，未获批准不进入 `implementing`。
  下一步：按 F-042 完成标准实施（迁移 036 executor_ms、执行器计时上报、只读聚合服务与三接口、前端三级看板）；内网项目不脱敏、日志按原样保留，不新增剥离层。完成自动验证后停在 `ready_for_manual`。
- 2026-09-23 参照目标平台代码将移交动作完整纳入动作配置与执行体系，停在 **ready_for_manual**：
  ① 移交人员候选与门禁严格对齐目标平台架构树：参照目标平台 `views/GroupApproveManage/Backlog/index.vue` 的 `clickHandOver` 与 `PersonSelectDialog.vue` 实现，移交弹出的选人窗口实际选用全公司组织架构树人员；`path_config_plan.go` 改造 `transferNodePersonConfig` 与 `nodeActorSwitchable`，优先复用 `AddSignCandidates` 公司人员目录池，彻底解除固定审批人或未预先计算候选节点导致移交被误禁用的缺陷；
  ② 修复一键配置移交候选策略与发起人排除：`path_action_configuration.go` 新增 `autoTransferPersonStrategyExcludePlanAccount`，明确排除流程发起人并确定性选出被移交人；自动配置候选时同步补齐 `candidate.action.ActorPolicy`，彻底消除一键配置试编译时因 `ACTOR_POLICY_REQUIRED` 导致的移交候选全数淘汰；
  ③ 修正场景编译器跨节点移交连续性校验：`scenario.go` 修正 `validateNodeOrder` 与 `transferred` 标志位维护，限制 `ACTOR_CONTINUITY_UNKNOWN` 仅对同一节点内移交后的紧邻动作生效；当流程流转到后续节点时自动重置 `transferred`，由执行引擎的 `switchToCurrentHandler` 动态发现并登录真实处理人，彻底消除跨节点普通动作因缺少 `ActorPolicy` 被无辜阻断的缺陷；
  ④ 写入动作、核验与后续同意执行体系闭环：移交写端点 `/web/flowInstanceApi/approverAppend` 保持严格协议信封（`data{id, jobTaskId, batchNo, auditRecord{transfer}}` + `approverAppendVo{flowNodeProxyId, userIds}`）；写后核验 `classifyTransferReread` 确认原处理人待办移除；同节点默认尾动作 `approve` 自动通过 `switchToCurrentHandler` 切换至被移交人会话完成节点审批流转；
  ⑤ 自动化验证与服务编译：后端 26 个测试包全部通过，新增 `TestAutoConfigureTransferActionWithExcludePlanAccount` 定向单测通过，`go vet` 通过，前端类型检查通过。
- 2026-09-23 历史候选全量查询与智能亲和排序改造完成，停在 **ready_for_manual**：
  ① 放宽直连库历史实例硬过滤：`target_history_repository.go` 彻底移除 `WHERE p.flow_name = ...` 强制同名与同表单过滤，解除因流程重命名、模板小版本改动或新流程无同名实例导致的候选弹窗全空硬阻断；
  ② 智能多维亲和度权重排序：查询结果全面按 1) 流程名完全一致优先；2) 流程名模糊包含次之；3) 流程编码一致再次之；4) 表单名一致再次之；5) 完结态(`status='end'`)优先；6) 发起时间倒序排列；保证当前流程的实例必然顶置在前排，其他可用表单实例顺延展示；
  ③ 统一「一键配置」与「更换历史数据」弹窗底层来源：保留租户隔离（`customer_code`）与搜索分词下推，用户点开弹窗（`BaseFormDataPicker.vue`）即刻展现最相关历史实例并支持翻页浏览全量实例，彻底闭环人工选择；
  ④ 自动化验证与服务热重启：后端 17 个测试包全部单测通过，`go vet` 通过，重新编译并热重启 `.runtime/server.exe`；实测计划候选接口返回 44892 条全量实例，首屏精准置顶当前流程已办结实例。
- 2026-09-23 自选审批人配置锁定与画布卡片处理人呈现修复完成，停在 **ready_for_manual**：
  ① 运行时处理人锁定配置阶段选定人选：`path_config_plan.go` 与 `path_action_configuration.go` 改造为优先返回并锁定已保存的 `selected` 名单（无论用户手选还是配置界面已选定人选），只要人员有效且未被排除，启动运行时直接作为最终处理人下发给目标平台（`nextAuditorList`），严禁运行期二次洗牌替换人员；
  ② 统一前后端随机与预览算法：`projectedPersonSelection`、`deterministicPathConfigPeople` 与前端 `resolvedPersonStrategySelection` 对齐为一致的确定性选择算法，彻底消除“配置界面预览与运行执行算法不一致”的脱节；
  ③ 运行详情画布节点处理人呈现精细化：`run_orchestration.go` 新增 `applyConfiguredAuditorsToGraph`，在生成运行详情图时将配置期确定的处理人名字（如资金副主任【左荣升】、张政【丁康】）直接覆盖到图节点的 `AuditPeople`，彻底消除自选节点在等待运行期间把 621 人候选集全量展示在卡片上的缺陷；`FlowGraphNode.vue` 在已有落账步骤事实时专职由下方步骤记录展示真实执行人与耗时；
  ④ 验证与服务热重启：前端类型检查与 Vite 生产构建通过，后端 17 个测试包单测全绿，`go vet` 通过，更新 `.runtime/server.exe` 并完成后台服务热重启；实测运行 151 动态图节点处理人已精确呈现为【左荣升】、【丁康】。
- 2026-09-23 全动作节点绑定归属与面板分栏透出改造完成，停在 **ready_for_manual**：
  ① 前端彻底废除全局实例容器分流与跨节点弹窗污染：`ActionOrchestrationEditor.vue` 与 `logic.ts` 改造为无论何种动作（撤回、催办、转发、关注等），只要在当前节点配置，动作全部直接绑定当前节点键（`nodeKey`），不再剥离到全局实例容器；每个节点的配置弹窗只打开和编辑当前节点动作，消除“点开任何节点弹窗都有撤回”的交叉污染；
  ② 节点面板详情清晰分栏呈现：`ActionOrchestrationEditor.vue` 在节点 Panel 详情看板中将【节点动作】（如回退、加签、暂存等）与【实例动作】（如撤回、催办等）分开两栏清晰展示（实例动作使用醒目黄色标签），并在末尾保留默认尾动作（如同意默认），所见即所得；
  ③ 场景编译器按节点原位编排并保持撤回恢复链：`scenario.go` 允许所有动作绑定语义节点键，在到达该节点时原位触发撤回等动作；撤回后自动生成发起人重新提交并沿途节点再次同意恢复至该节点的完备恢复链；
  ④ 清理历史残留全局实例动作与一键配置末节点取回保护：`path_action_configuration.go` 在一键配置时恢复 `dropRetrieveCandidateIfNoNextWindow` 调用，严禁末节点配置取回；路径 1 历史残留的孤儿撤回动作已清理；
  ⑤ 验证与热重启完成：新增 `TestNodeBoundWithdrawCompilesInPlaceAndRecovers` 定向单测，后端 17 个测试包全部通过，`go vet` 通过，前端构建通过；重新编译 `.runtime/server.exe` 并热重启服务。
- 2026-09-23 历史表单数据通用自动对齐与回填意图保护修复完成，停在 **ready_for_manual**：
  ① 后端子表单控制列通用自动对齐：`formdata.go` 增加 `alignSubformColumnControlFields`，严格遵循目标平台规范（`_col` 结尾的占位字段承载子表单列权限），若占位字段在快照中为空，自动从子表单明细行对应列提取非空值补齐，消除因历史空值导致目标原生校验拦截；
  ② 执行服务回填后联动意图值自动恢复与保护：`server.ts` 增加 `restoreIntendedPatchViaPage`，在数据注入并等待异步选项/联动稳定后，通用比对意图数据与页面实时数据，若项目/合同/账号等非空意图值被目标页面原生联动脚本误置空，自动进行二次补偿赋值，确保原生提交校验 100% 通过；
  ③ 数据工作区自动就绪闭环：`path_data_workspace.go` 在具备有效表单数据且无阻断问题时自动收敛数据状态为 `ready`，彻底消除人工必须每次点开表单弹窗点保存的依赖；
  ④ 真实运行端到端验证通过：`欧阳改并行测试加签__运行5` 第 1 步提交 `submit`（创建实例 `d61be8e63531493cbddc9759f1755f0c`）直接自动成功，并连续自动执行完成系统导航、加签（白珣发出 `updateFlowProxy`）与资金副主任审批，第 5 步加签插入节点审批顺利就绪。
  后端全量单元测试与契约测试全绿，`go vet` 通过，前端类型检查通过，等待用户人工核验。
- 2026-09-23 F-041「完整分支与选人组合适配」实施完成，停在 **ready_for_manual**：
  ① 目录层补齐主管副总接口：`person_directory.go` 增加 `FindSuperiorLeader`（`/web/user/api/user/findSuperiorLeader`）与 `FindGrandpaSuperiorLeader`（`/findGrandpaSuperiorLeader`）；
  ② 解析层自选策略智能兜底：`path_action_configuration.go` 对并行分支或未单独手动配置的 `run_node_choose` 节点，自动以目标默认候选人（`target_default`）或范围随机（`random`）兜底，消除少配分支即中断；
  ③ 运行上下文全流程自选解析：`run_orchestration.go` 增加对流程图全量 `nodes` 中所有自选节点的全局解析，并存入 `NextNodeAuditors`，确保并行同级各分支人员全部就绪；
  ④ 执行服务并行闭环与节点人员注入：`runner.go` 增强对 `parallelChooseNodes`、`parallelNodes` 和 `branchNodes` 的兼容解析；`native-dispatch.ts` 在 `parallel_choose` 时向原生组件的 `parallelNodeChooseList` 与 `parallelChooseNodes` 自动注入对应分支人员的 `nodeAuditList`，彻底解决原生组件报“当前并行分支下有节点审批人未指定”的断裂。
  Go 构建、vet、后端 17 个测试包全部通过，前端类型检查通过，等待用户人工核验。

- 2026-09-23 F-040「表单数据自主生成、子表单保护与校验错误明确透出」实施完成，停在 **ready_for_manual**：
  ① 子表单与动态表格回填保护：`formTemplate.js` 的 `VALUE_GROUP_TYPES` 补充 `table` 类型，递归解析 `tableColumns` 列描述符；新增 `restoreIntendedValues` 机制，在表单初始化联动与异步数据源加载收敛后，自动对比并恢复被目标模板脚本（如合同 onChange）误清空的明细行非空意图值（如收款明细【收款账户】），彻底消除子表单下拉框留空问题；
  ② 执行服务表单必填校验错误明确透出：`server.ts` 在动作分发后捕获页面的表单校验错误（`.is-error` / `.el-form-item__error`）；`completion.ts` 在未捕获到期望写请求时，若存在表单校验错误，由原本含糊的“未发出期望写请求 /web/flowInstanceApi/reSubmit”转为直接明确提示“表单数据缺失无法发起请求，请检查表单数据（明细第N行【字段名】...）”，前端用户一目了然；
  ③ 合同组件回显修复与组件补齐：`GeneralListSelectShow/index.vue` 增强 `displayName` 计算属性，同时兼容 `contractName`、`name`、`title` 以及纯 ID 场景；补齐引入并全局注册 `contract-change-list-select` 及其专用弹窗 `ContractChangeListSelectList.vue`；`form-runtime` 同步脚本增加启动拉取 Git 最新代码逻辑；
  ④ 已发/待发流程自动继承实例数据：`path_data_workspace.go` 在未保存路径覆盖时，若实例已有当前表单数据（`InstanceValues`），自动将其作为初始基线快照，无需用户弹窗二次选择；
  ⑤ 放开新发起流程表单保存硬门禁：`SaveData` 允许在未选基础表单数据时，只要用户在目标表单组件内填写了有效数据（`input.Values` 非空），即可直接作为完整表单数据保存；
  ⑥ 历史实例查询全面解绑：`client_list_reads.go` 彻底移除 `flowInstanceBizRelevanceList`（公司关联）与 `useScope: "invest"`（投资范围）硬编码过滤；`history_target.go` 优化 `historyCandidateIdentityMatch` 支持表单名与流程名的包含与模糊匹配，解决机房进入申请表等非投资或非公司流程无法被检索到的问题；
  ⑦ 已发/待发流程自动推导后续路径：`execution_path.go` 移除 `plan.FlowSource != "new"` 拦截，`PlanPathsView.vue` 支持对已发/待发计划自动触发后台路径解析，从实例当前节点沿代理树自动推导后续确定分支路径。
  Go 构建、vet、后端 17 个包全部单元测试全绿，前端 58 个测试用例全绿，类型检查通过，等待用户人工核验。

- 2026-09-22 「一键配置全是关注」根因修复并按用户裁决调整为随机起点轮询：①旧实现把实例动作候选池按动作键字母序排列，follow 恰好排第一且轮询偏移从 0 开始，路径 1 永远配置成关注（画布每张节点卡又都附带实例动作摘要，看起来"全部节点都是关注"）；②一键配置只对勾选路径生效且只补空不覆盖——问题计划当时的批量任务 total=1，路径 2/3/4 根本没被配置，已配好的路径 1 即使修复后重跑也保留旧动作；③发起人"交替"此前用哈希种子奇偶决定，无法保证两种发起方式都出现。现改为：实例动作按固定池 转发→撤回→催办→关注「随机起点严格轮询」——起点由计划级种子随机决定（不同计划组合不同），同计划内按路径序号依次轮转必然全覆盖，第五条路径循环回第一条；发起节点交替起点同样由计划级种子随机，同计划奇偶路径严格互补 50/50；取消关注不参与自动轮询（仍可手工配置）。新增随机轮询覆盖与发起互补定向单测，Go 构建、vet、全部单测与契约测试通过；用真实计划 26 对路径 2/3/4 重新一键配置实测：催办/关注/转发全出现、发起方式逐条互补。停在 **ready_for_manual**，等待用户人工核验（注意：路径 1 的关注是修复前旧配置，一键配置只补空不覆盖，需手工清除或删除该路径重跑才能看到新轮询结果）。
- 2026-09-22 动作编排升级为按路径序号严格轮询的最大化全动作覆盖算法并重启后端服务：①修复转发候选在目标公司人员超过 1000 人时因附带上限提示 issue 被误禁用的缺陷，放行转发接收人解析；②实例动作改用基于路径序号 `path.SequenceNo` 严格轮转（Round-Robin）：路径 1、2、3、4 严格按顺序分配转发（forward）、撤回（withdraw）、催办（urge）、关注（follow），多路径循环轮流穷尽覆盖；③发起节点按路径序号交替分配“直接提交”与“先存草稿再重提”；④审批节点按路径序号与节点位移轮流分配暂存、加签、移交、驳回、回退、取回等全部审批动作，消除伪随机分布不均导致动作遗漏；⑤重新编译 `.runtime/server.exe` 并完成后台服务热重启，确保浏览器请求立即命中最新逻辑。定向测试全部通过，Go 构建、vet、全量单元测试全绿，停在 **ready_for_manual**，等待用户人工核验。
- 2026-09-22 多次加签导致运行详情状态整体丢失（运行 124 复现）修复完成：①同一链路连续多次加签时，加签会重建代理树并使锚点后子链结构路径按锚点累加位移（2 段、3 段），旧「恰好一段位移」容错全部失配，且后续重建会整树更换节点 ID 使已落账插入节点合成键失效；②详情投影改为锚点感知的结构对齐——以已落账加签步骤的锚点令牌推导每个静态节点的期望动态路径直连绑定，未命中再按位移段数从小到大逐层唯一容错，彻底避免同类型链上「浅一层节点恰好同路径」的精确命中错绑；③插入节点合成键按「锚点期望路径 + 第 N 跳后继」确定性对齐到当前事实图节点（与投影成败无关）；④投影成功后画布回到计划 ID 空间，插入审批人保留目标真实 ID。真实运行 124 验证：45/45 步骤 nodeId 全部恢复、12 个漂浮状态键归零、三次加签审批人（含 ID 已重建的两位）全部点亮。新增多次加签位移定向单测，Go 构建、vet、全部单测与契约测试通过，停在 **ready_for_manual**，等待用户人工核验。
- 2026-09-22 运行完成态画布状态丢失与首节点转发配置限制修复完成：①已完结实例待办为空时，`GetRuntime` 允许无入边根节点作为入口，避免 `validateEntryNodeIDs` 报错阻断已结束流程代理树读取；②加签等动态树导致投影回退或控制层现场释放时，后端详情利用结构路径补齐静态 Token 与事实图节点映射，杜绝刷新后节点状态全退化为“未开始”及步骤脱节；③前端运行画布在退回静态图或刷新时常驻虚拟结束节点；④发起人节点（首节点）界面动作目录排除实例级动作（转发、催办等），场景编译器增加“流程未发起前禁止执行转发”的硬性结构阻断，杜绝未提交实例执行转发。定向 Go 测试、vet、全量构建、前端类型检查全绿，停在 **ready_for_manual**，等待用户人工核验。
- 2026-09-21 动作编排策略修复（`docs/auto/20260921-action-orchestration-repair.md`）已获批准，进入 **implementing**：
  1. 一键配置移除取回只能在末节点的人为限制，移除回退/取回等级压制，实例动作（转发等）纳入自动配置；
  2. 场景编译分离取回来源节点与执行窗口，回退前驱解析与恢复链时序完善；
  3. 手动保存支持非阻断提醒，区分结构错误与可执行性提醒，运行时无副作用不满足跳过并继续；
  4. 动作次数展开逐条消费，恢复链不重复计费；
  5. 15 个用户动作及系统语义覆盖审计。

- 2026-09-21 执行引擎与运行画布修复已完成：①处理人姓名显示为 32 位 UUID 缺陷已彻底修复（全员目录解析、姓名回退、反序列化补齐）；②动作详情弹窗清理（移除术语、尝试转次、表单门禁转储，呈现业务人名与动作，API 按写/读归组，标明耗时与日志路径）；③结束节点常驻画布末尾，完成态亮绿，仅展示“结束”文本，移除多余状态标签；④加签动态节点在轮询偶发超时/抖动时复用缓存并在前端粘性保留，避免后续审批步骤中途闪烁或消失；⑤修复加签后节点状态和数据流显示不一致问题：动态图成功投影时以画布实际持有的静态 ID 空间为准，仅在投影失败时才使用动态目标 ID 兜底，杜绝键空间被动态 UUID 误污染导致已完成节点与当前节点全部退化为“未开始”的严重缺陷。定向 Go 单测、全量契约测试、受影响包 vet、前端类型检查通过，停在 **ready_for_manual**，等待用户人工核验。

- 2026-09-21 路径运行界面处理人显示为 32 位 UUID 缺陷已修复完成：①`UserAccountsByID` 改造为返回包含 `Account` 和 `Name` 的结构体，保留全员目录中的真实中文姓名；②`assigneeAccount` 改为优先返回真实姓名，姓名为空时回退为账号，彻底删除将 32 位 `assigneeUserID` 赋给 `name` 覆盖执行人摘要的错误逻辑；③`ListAuditRecords` 补齐对目标平台 `executorName` 的反序列化与透传，使审核记录中的审批人姓名被真实捕获；④阶段 2 门禁预览在名字缺失时从目录按用户 ID 解析中文姓名，避免在放行前就把 UUID 渲染到界面；⑤详情服务层增加 `isOpaqueIdentifier` 保护，历史步骤误存 32 位 UUID 时优先从写前事实恢复真实姓名。新增定向单元测试与回归测试，Go 构建、vet、定向测试和前端类型检查全绿；未启动浏览器、未连接目标平台，等待用户人工核验。

- 2026-09-21 用户运行反馈五项收口已完成，F-037/F-039 保持 **ready_for_manual**：①取回按最近一次通过审批记录使用当前节点实际审批人账号；②写请求到核验完成期间继续显示当前节点并标记“确认结果中”，全部落账后才切换；③加签节点历史摘要为空时从目标事实恢复真实执行人姓名或 ID；④最新结构暂时不可读时静默保留计划画布，不再反复显示内部结构提示；⑤完成态补出固定 ID 的虚拟“结束”节点及终端连线，刷新投影幂等。定向 Go 测试、构建、受影响包 vet、前端类型检查和差异检查通过。完整 `test/contracts/...` 仍受既有 `contractPlanRepository` 缺少 `BatchDelete` 接口阻塞，未扩大范围；未启动浏览器、未连接目标平台、未发送真实写请求，等待用户人工验收。

- 2026-09-21 重发起审核意见语义已按用户重新裁决：`resubmit` 不沿用旧文档“保留目标衍生意见”的建议，而是必须清空实例当前已返回的 `auto_audit_info_*` 文本及已存在的对象/列表伴生键。`BuildNodeFormData` 现把这些键登记为显式覆盖并传给 Go/runtime 两条写路径；写后核对对重发起要求目标确实为空，普通审批/暂存仍保留目标生成意见。新增衍生字段单测，定向 Go 测试与 vet 通过；待真实目标页面人工验收，不自动改变 F-035 状态。

- 2026-09-21 夜测 F-P03 401 风暴根因已定位并完成止损：目标平台同账号单会话（浏览器/其他进程会互踢），工具原先虽有进程内账号锁和 SID 缓存，但连续新 SID RESP401 会让每个排队读取再次登录；`Refresh` 还可绕过使用锁。`session.Manager` 现对“新会话重放仍失效”建立 30 秒账号级冲突熔断，熔断期 `Current`/`Verify`/`Refresh`/`DoRead` 不再登录，成功请求自动解除；新增会话管理器回归测试，定向 Go 测试通过。F-P03 的“0 步结果待确认/401 风暴”工具侧放大器已修复，仍需用户在真实浏览器/目标平台确认外部同账号会话不再持续互踢后再验收。

- 2026-09-21 夜测报告 F-P05 高优先级缺陷已修复：独立执行服务会话从“只依赖 Go 主动销毁”改为 90 秒租约，Go runtime 动作每 10 秒续租；执行服务每 10 秒回收只回收租约已过期的孤儿会话，并在回收时卸载路由、释放 BrowserContext 和同账号门禁。正常结束先停心跳再主动销毁，不改变同账号唯一会话、一次写和结果待确认边界。新增会话租约与 Go 心跳定向测试；执行服务类型检查、会话管理器 5/5、runtimeexec Go 单测通过。F-036 保持 **ready_for_manual**，待用户实际中止/重启 Go 后验证无需重启执行服务即可恢复。

- 2026-09-20 F-034 真实运行反馈修复完成：目标已证明待办在后续节点时，`isSkip=true` 的节点整体记为“目标跳过”，节点上的用户配置动作不执行且不发写请求；`isSkip=false`/未声明或 `run_node_choose` 缺少可用处理人时在发送前阻塞并显示原因。错误详情优先绑定已落库步骤，路径停止行按 `stopStepNo` 回溯，失败后不依赖已清除的 `currentPreview`。执行器定向测试、前端类型检查、构建和生命周期静态测试通过；未连接目标平台、未启动浏览器，F-034 保持 **ready_for_manual**，等待用户手工验收。

- 2026-09-19 F-039 按用户「你来实施」进入实施并停在 **ready_for_manual**（代码+仿真，未连目标、未发真实写、未启动浏览器）。
  六个原子提交：`核实目标登录协议并修正错误分类`(T01)、`统一动作规格与候选阻塞语义`(T02)、`条件化固定尾同意编排`(T03)、
  `实现取回特殊动作组`(T04)、`实现加签动态节点审批编排`(T05)、`收口动作写后核验与恢复边界`(T06)。要点：
  T01 修正登录会话失效与登录/密码拒绝分类不混淆（envelope RESP401/AUTH_401/SID 保留会话错误，HTTP 401 保留登录拒绝），
  「服务端不校验验证码文字」由通用断言改为 profile 事实；T02 新增 `StructureEffect` 与候选三态 + 节点级原因（既有排序/配置不变）；
  T03 固定尾同意补条件化门禁事实与独立放行组断言；T04 取回准备同意改运行时按目标已办事实决定（已有已办跳过不多发同意，
  无已办不可同意则阻塞），仿真覆盖两条路径；T05 加签重绑定确定性暴露插入的独立审批节点并把静默偏离改为显式命名停机，
  **插入节点自动审批执行**按前置门禁留待 F-037 验收后人工阶段；T06 补跨动作统一安全边界断言。
  本地验证全绿：`go build`、`go vet`、`test/unit/backend/*`、`test/contracts/...`、前端 `typecheck`、`git diff --check`。
  未闭环（用户手工阶段）：真实浏览器登录矩阵/加签/两条取回/普通与恢复与并发回归/画布刷新四方对照、各真实写动作；
  F-037/F-038 页面验收仍为前置。T07 画布回归属前端真实页面项随 F-037 人工核对；状态不自动置 `accepted`。

- 2026-09-19 F-039 夜间审查全量修复实施方案已完成，状态为 **awaiting_approval**：方案将登录 flag/sign 与 profile 差异、统一动作规格、固定尾同意条件化、取回特殊动作组、加签动态节点审批、其它动作安全边界、运行事实/日志/画布收口及自动与真实浏览器验证拆为 T01-T09。F-037/F-038 作为前置成果仍待用户人工验收；本方案未修改业务代码、未连接目标平台、未进入 `implementing`。

- 2026-09-19 F-038 夜间审查动作候选恢复已完成：删除一键配置对加签、取回的永久静态排除，恢复候选级门禁、人员校验和完整场景编译；目录不完整或场景不安全时保留具体节点原因。相关单测、`go build ./...`、`go vet ./...` 和 `git diff --check` 通过。未启动浏览器、未连接目标平台、未发送真实写请求。状态为 **ready_for_manual**，等待用户人工验收；加签动态节点和取回特殊动作组留在后续切片，不自动开始。

- 2026-09-19 F-037 人工验收反馈（运行 8）两问题已修复：①驳回后重新发起失败——加签触发目标后端 updateFlowProxy 所有权复制后，实例私有代理 ID 不在目标发起映射表，重新发起弹窗权限探测被「流程模板已被更新」拒绝，页面永不发出 reSubmit；修复为重新发起装载行事实改用计划绑定的发布流程代理 ID（新发起来源），待发/已发来源保持实例事实代理并记录边界。②加签节点在目标平台「查看流程」不可见——工具此前只把人员并入既有节点明细，现按目标手动加签真实语义在写前当前节点之后插入独立加签审批节点（styleType/currentDelete、assign+personnel 明细、字段权限复制、无 ID 由后端生成），写后核验改为全树恰好一个节点包含全部加签人员，代理树重绑定按插入锚点确定性平移，控制层强制加签重建必须携带锚点。回归测试、构建、全部单测与契约测试通过；F-037 保持 **ready_for_manual**，等待用户重新验收页面。

- 2026-09-18 F-037 已结合目标后台 `saveFlowProxy/updateFlowProxy` 和目标前端 `flowNodeAuditConfig.flowNodeDetailConfigList` 完成加签链路修复：分析器保存目标树真实结构路径，运行详情按路径投影重建后的代理树，合法嵌套条件不再误报结构无法绑定；审批/加签人员按目标明细名称显示在原审批节点；第 9 步失败事实与时间线保留，日志目录继续使用“计划名__运行号/路径__YYYY-MM-DD HH-mm-ss”。新增嵌套条件投影和人员展示回归测试，Go 定向测试、构建、前端类型检查及差异检查通过。F-037 保持 **ready_for_manual**，等待用户页面验收。

- 2026-09-18 F-037 人工验收反馈已修复：真实日志证明加签写入成功后，执行器仍在代理树重绑定前用旧节点 ID 做路径比较，错误设置“路径偏离”并停止后续放行。现在加签步骤等待控制层按最新代理树完成整体重绑定后再决定后续动作；重绑定失败仍只保留查看/停止，普通审批真实分支偏离继续强制停止。新增两条执行器回归测试，定向测试通过；F-037 保持 **ready_for_manual**，等待用户重新验收页面。
- 2026-09-18 F-037 加签后刷新与日志命名修复完成：加签写后强制重新读取实例当前待办/已发事实并按最新 `flowProxyId` 读取整棵代理树；结构映射优先精确路径，插入节点仅接受最近且唯一的 child 候选。运行日志目录改为“计划名__运行号/路径__YYYY-MM-DD HH-mm-ss”，完整 ID 保留在 meta.json 与日志字段中。Go 定向测试、执行器测试、日志测试、`go build ./...`、前端类型检查和 `git diff --check` 全部通过。F-037 保持 **ready_for_manual**，等待用户页面验收。

- 2026-09-18 F-037 人工验收反馈已修复：加签成功后读取最新代理树，按唯一结构位置整体重绑定节点、图边、分支选择和下一节点处理人；允许链路中安全识别插入的新节点，无法唯一证明时清空放行命令并只保留查看/停止。详情服务与前端改为统一动态投影，避免新旧 ID 并集造成双根画布。Go 测试、构建、前端类型检查、契约检查和差异检查通过；Bash 结构脚本因当前 Windows 无 bash/WSL 无法运行。F-037 状态为 **ready_for_manual**，等待用户真实页面验收，不自动开始下一功能。

- 2026-09-18 F-037 运行画布与动作配置可用性修复：动作行增加稳定可见的“删除动作”文字+图标并保留确认/只读/保存语义；点击运行节点不再自动居中；运行详情按当前实例最新代理树投影动态节点与可证明连线，路径偏离展示预期/实际节点且继续由控制硬门禁禁止放行；节点面板统一展示已执行、正在执行、等待执行步骤并区分颜色。已通过受影响 Go 包测试、Go build、前端类型检查、结构/契约检查和 `git diff --check`，未启动浏览器、未连接目标平台、未使用固定账号/模板/实例/端口测试。状态为 **ready_for_manual**，等待用户真实页面验收，不标记 accepted，不开始下一功能。
- 2026-09-18 F-036 页面路径配置兼容修复：后端 `/configuration` 成功返回后，节点绑定阶段不再直接依赖
  非安全上下文不可用的 `crypto.subtle`；`pathConfigNodeKey` 在 HTTPS 原生 Web Crypto 不可用或调用失败时，
  使用同输入、同 SHA-256 结果的浏览器本地实现，保持与 Go 服务端节点 key 一致。未新增固定账号、模板、实例或
  浏览器自动测试，状态保持 **ready_for_manual**，等待用户通过页面重新打开路径配置确认。
- 2026-09-18 F-036 按用户要求进入 **ready_for_manual**：删除目标账号、固定模板/实例 ID、动作变量和
  真实写动作台；删除 `real_load_loop_test.go`、`real_action_loop_test.go` 与 `f036-real-action-bench.mjs`。
  `test/run-executor-service.sh` 现在只运行本地类型检查、构建、执行服务单元/集成、Go 契约和路由检查，
  不加载 `.env.local`，不连接目标平台，不自动启动浏览器。`RUNTIME_EXECUTOR_URL` 仍是正式 Go 服务连接
  独立执行服务的运行时配置，不是页面测试参数。后续由用户从页面创建/选择真实计划并手工运行，记录问题后再规划修复。
- 2026-09-18 F-036 页面阻塞修复：局域网 HTTP 下统一使用兼容 UUID 工具，`PlanPathsView` 补齐 `NSpace` 注册，
  基础数据弹窗仅在打开时挂载并在关闭/离开后销毁。用户此前反馈的 `crypto.randomUUID is not a function`、
  `Failed to resolve component: n-space`、Vue `emitsOptions` 连带错误对应的本地代码问题已处理；
  `injected.js` 的 unload/WebSocket 日志属于浏览器注入层，待人工验证确认是否仍影响页面。下一步只等待用户手工点击
  「配置节点」和表单数据面板，反馈实际页面问题后再规划后续功能切片。
- 2026-09-17 F-036 第二轮修复按用户要求暂停，状态保持 **implementing**。已完成代码主体：通道单连接复用、
  接口终局判据、多次草稿 edit 模式、列表页原生转发与两笔写顺序门禁、所有原生渲染类型统一 runtime 路由、
  表单正文写前快照/写后回读保护；已通过 TypeScript 类型检查、执行服务测试 73/73、runtime Go 契约与单测。
  第 0 步已从真实 `SHOW TABLES` 核对 20 张表并清空 19 张业务表，保留 `schema_migrations`；
  `logs/runs`、`logs/plans` 为空，12 份第一轮证据已归档。尚未跑强制真实 L1/L2、计划 A–G、七动作第二轮证据、
  无表单真实路径、真实多次草稿/转发和最终聚合门禁，故不得进入 `ready_for_manual`。
  暂停报告：`docs/auto/2026-09-18-f036-round2-run-report.md`。
- 2026-09-17 迁移修复（macOS → Windows）：清掉三处只在 Mac 上成立的路径依赖，并把执行器跑起来。
  ① `flow-source` 不再复制本机静态目录，改为按 `executor-service/flow-source-manifest.json`
  直接从固定远端 git 同步（`test`/`bff4ef8`），脚本内零本机绝对路径；幂等（一致时只报告「已是最新」，
  本地被改脏则重新检出覆盖）。FormMaking 产物经核实不在上游 Git（上游只跟踪
  `src/lib/vue-form-making/package.json`），改由同步脚本从 `form-runtime/runtime-source` 受控副本
  补齐并逐字节校验。② 原生 `/flow` 页面产物不再依赖 Mac 上手工产出的 `dist.zip` 与系统 `unzip`，
  改由新增的 `build-native-page` 在本机真实构建到 `flow-source/dist`（按源码摘要判定是否过期）。
  ③ `E:\bigsys` 下失效的 worktree 绝对路径已修复：`rsh-cloud-invest-power-system-oygdev2`、
  `rsh-cloud-invest-power-system-test`、`rsh-cloud-invest-power-system-test`（test-auto-pro 内副本）、
  `rsh-unified-vue3-web-test`、`rsh-unified-vue3-web-hot-fix` 共 5 个可正常用 git；
  修复脚本为 `scripts/repair-migrated-worktrees.ps1`（幂等、改写前备份、支持 `-DryRun`）。
  参考仓库清单远端统一为 SSH（HTTP 远端需凭据，本机未配置会挂住），`make refs-status` 13 个仓库全部可达。
- 2026-09-17 F-036 第一轮曾标为 **ready_for_manual** 的结论已被第二轮任务复核证伪；旧证据已归档，
  不得再作为当前完成依据。当前状态与剩余项以上方第二轮暂停记录为准。
- 2026-09-16 F-036 **ready_for_manual**。T001–T014 自动验证完成。可见页面和真实目标动作等待人工验收；不得自动开始下一功能。
















- 2026-09-16 执行引擎改造浏览器真实代验收首轮完成（部分受阻，状态保持 **ready_for_manual**）。
  已验证：验证模式放行与能力门禁行为正确、runtime 全链路机械贯通（会话→装载→页面请求转发→
  完成检测→收口）、失败路径零写请求落目标、停止按「可能已写出」保守收口。排障修复四缺陷
  （转发双 /api 前缀、空 flowProxyId 空指针、装载 60s 预算、装载先于命令通道），并有
  页面异常/请求失败/中止原因可观测化，全部提交（a2d270b 等）。当前阻塞：`flowGroup/list`
  与 `type/list` 对测试账号返回 NotAuthException（同 SID 其余端点正常），页面静默停装，
  保存草稿未达提交请求——属目标侧权限行为，需目标侧核实后继续逐动作验收。
  详见当时运行日志与功能文档状态记录。


- 2026-09-16 执行引擎改造评审退回修复完成（评审判定 HEAD 3baef75 不能按「已完成接入」验收，
  指出 7 个 P1 + 2 个 P2）。全部问题已在独立分支 **执行引擎改造评审修复**（基于 3baef75）修复，
  等待用户裁决何时合并，不自动合并；逐项中文原子提交：
  ① `ErrUncertain`（连接断开且已有目标请求写出）改按「写已发出、结果未知」处理进入结果待确认，
  不再误判零写入失败诱发重复执行；`ErrFailedCleanly` 与会话/装载失败保持零写入失败并如实分类（9d091b6）。
  ② 能力登记落盘持久化：装配层从 `.runtime/runtime-capabilities.json` 加载、登记成功即原子重写，
  重启不再清空；`Register` 强制校验流程类型/动作/表单摘要/runtime 版本/目标版本/证据/日志引用，
  自动登记补目标版本（runtime-source HEAD）与转发写请求传输层 trace 证据，废除无摘要粒度回落（be02cd3）。
  ③ 装载载荷补齐目标代理与节点事实：formProxyId/flowProxyId/selectFlowType/flowNodeType/
  nextNodeProxyId/flowInstanceBizRelevanceList/当前节点权限与覆盖字段（提交按 formId 决定发送
  formProxyId，此前空表单代理必然发出 `flowProxyId:""`）（55b1f84）。
  ④ 转发动作接入接收人：RuntimeRunRequest 新增接收人/勾选事实，新增 load_forward 装载模式与
  桥接 `loadForward`，审批宿主 `applyForwardLoad` 首次接通；无接收人写前阻塞（55b1f84）。
  ⑤ 选择闭环严格校验：分支入口必须通过真实图边校验（是分支节点直接后继），并行/分支的
  自选节点必须已有配置人员，任一缺配置整体阻塞，绝不让目标临时节点集合覆盖已保存配置（35e77a6）。
  ⑥ runtime 详情生产接线：迁移 033 新增 `run_step_attempts.runtime_detail`，三阶段投影与请求
  轨迹随尝试行落库，详情接口透传，运行详情弹窗新增「表单运行时执行详情」区块（e3b418d）。
  ⑦ 生产接线：配置 `RUNTIME_EXECUTOR_URL` 后 Go 服务默认托管拉起本机执行服务子进程
  （就绪探测、复用已运行服务、随后端停止；`RUNTIME_EXECUTOR_MANAGED=0` 关闭，远端地址不托管），
  修正 run-runtime-exec.sh 真实验证入口说明（16268b4）。
  验证：`bash test/run-runtime-exec.sh` 10/10 通过（含真实无头 Chromium 集成与新增托管启动
  集成用例）；`go build ./...`、`go test ./test/unit/backend/... ./test/contracts/...` 全部通过；
  `pnpm --dir web run typecheck` 通过。既有环境失败如实记录：test/integration 的 F-016 系列
  MySQL 用例因本机 shell 缺少 `PLAN_DB_HOST/USER/PASSWORD` 失败，经基线提交 3baef75 复现确认
  与本分支无关。真实目标动作验证仍待用户按 real-target-checklist.md 人工执行。

- 2026-09-16 执行引擎改造自查评审修复一轮完成（状态保持 **ready_for_manual**）。评审证实四处真实缺陷并已修复：
  ① RunAction 把已编码装载信封塞进外层 Payload 二次包装，协议校验必然拒绝（探针复现「消息缺少 requestId」），
  整条 runtime 动作链路此前从未走通装载；② SID 换会话重放后写响应丢失不记写出事实，断连会误判零写入失败；
  ③ runViaRuntime 审批族动作不带任务号（audit 载荷 data.jobTaskId 是目标硬依赖）、分支选择丢键、
  发起人用户 ID 误用主实例 ID、发起模式传空模板（空表单写目标风险）、空人员条目原样下发；
  均已修复并新增定向用例（装载信封形状、重放写丢失判不确定、事实传递/无任务阻塞/发起模板/缺表单代理阻塞），
  全部经变异验证。`go build ./...`、`go test ./test/unit/backend/... ./test/contracts/...`（25 包）、
  `go test ./test/integration/runtimeexec/`（真实无头 Chromium 服务）全部通过。
  评审同时确认三处接线缺口未在本轮处理（runtime 路由是可选能力，保持停用不影响既有 Go 写路径）：
  `EnableRuntimeExecution` 无生产装配点（服务启动未接线，当前生产仍全量走 Go 写路径）；
  `BuildDetailProjection`/`UnlockFromVerifiedRun` 无生产调用方（运行详情三阶段投影与能力自动登记未接入 API）；
  real-target-checklist 缺少 run-runtime-exec.sh 承诺的「逐项命令」。三处均需人工验收裁决后另行批准实施。

- 2026-09-15 执行引擎改造（docs/s/，分支「执行引擎改造」）T001–T015 全部实施完成，状态停在
  **ready_for_manual**，等待用户按 `test/integration/runtimeexec/real-target-checklist.md` 做真实目标人工验收。
  已落地：BrowserContext 会话管理（同账号互斥/崩溃广播）；Go/runtime 消息协议（exec-engine/v1，
  双侧夹具锁定）；执行模式根桥接 `window.__runtimeBridge__`（六方法，iframe 模式共存，销毁失效）；
  Playwright 全量网络拦截（目标请求转发 Go、未登记请求本地拒绝、base64 保真、可回放轨迹）；
  Go runtime 客户端（WS 往返、SID 失效换会话重放一次、断连按写出事实二分 uncertain/failed）；
  发起/审批装载（真实 FlowDialog 自装载目标模板、操作人/发起人身份分存、双审批人隔离）；
  七个 runtime 业务动作真实入口（sumbitFlow/formMakingFormBusiness/temporaryStorage/reSubmit/confirmTranspond）
  与选择闭环（custom_choose/parallel_choose/run_node_choose 仅继续一次，choice_required 事件）；
  工具流程名称协议级恢复；动作完成检测（网络空闲稳定窗、硬超时后拒绝新写、business_failed/uncertain 口径）；
  执行器可选 runtime 路由（能力未登记即阻塞、接管后零 Go 写）；运行详情三阶段投影；
  能力登记表（只认 confirmed_success，逐组合登记，当前为空）。
  验证：`bash test/run-runtime-exec.sh` 10/10 通过（单测 36、契约 5 组、真实服务集成 6 项）；
  真实目标写验证（15 动作对照人工操作）待用户按清单逐项回填证据。
  各任务逐项中文提交见 git log（T001–T015）。

- 2026-09-15 用户反馈动作编辑弹窗"添加动作"按钮不见了。根因：提交 5f2170b 重写弹窗 footer 时误删了
  `<n-button … @click="addAction">添加动作</n-button>`，`addAction`/`canAddAction`/`AddOutline` 成为死代码，且
  F-012 结构契约测试没有对按钮的断言导致静默丢失。修复：在 footer（取消与保存动作配置之间）原样恢复按钮并加
  `data-testid="add-action"`，保留只读、无可配置动作、超 10 条上限的禁用语义；结构契约补两条断言锁死按钮存在与
  禁用条件。前端 151 项测试与结构检查全部通过。F-035 保持 `implementing`，等待用户人工验收，不自动开始下一功能。

- 2026-09-15 用户反馈路径 4222 表单值已正确（vacateType 显示"产假"）但右侧"路径关键信息"仍提示"需要处理"。根因：
  `vacateType` 是 `showLabel=false` 且 `remoteType=datasource` 的下拉，模板静态选项 label/value 完全错位（label 残留
  采购分类名），FormMaking 渲染层对这类选项直接显示 value，而 iframe 选项协调的 `fieldOptions` 优先读 `getOptionData`
  返回的原始 `remoteOptions`（label 原样），按 label 匹配"产假"失败即误报 `OPTION_PATCH_UNRESOLVED`。修复：
  `formTemplate.js` 描述符记录 `showLabel`，实时选项经 `normalizeLiveOptionLabels` 按渲染口径
  （showLabel||remote ? label : value）归一化显示名，与 `buildStaticOptions`、服务端 `staticOptionEntries` 三方同口径；
  已修复前代码复现误报、修复后消除。前端 109 项、后端 `test/unit/...` 与 `test/contracts/f035/...` 全部通过。
  F-035 保持 `implementing`，等待用户人工验收，不自动开始下一功能。

- 2026-09-15 用户裁决去掉浏览器验证环节：一键配置后的最终表单确认不再依赖浏览器 form-runtime，服务端新增静态选项绑定解析器
  （`internal/service/path_data_static_options.go`）：对模板里 `remote:false` 且 options 完整的下拉，按目标渲染口径
  （showLabel 时显示 label，否则直接显示 value）把补丁改写的显示名唯一匹配成绑定值并回填 `__virtualName`；
  找不到或同名歧义落 `STATIC_OPTION_UNRESOLVED`/`STATIC_OPTION_AMBIGUOUS` 阻断且不覆盖旧值。回放（`replayItem`）与保存
  （`SaveData`）两处接入；`workspaceDataStatus` 不再把浏览器校验证据作为 ready 必要条件；删除 `HistoryReplayRuntimeValidator`
  桥接及其全部测试假件。真实验证：计划 30 十二条路径重新一键配置 12/12 直接 `ready`，路径 4222 的 `vacateType` 与
  `vacateType__virtualName` 均为“产假”。`go test ./test/unit/... ./test/contracts/f035/...` 与漂移脚本全部通过。
  F-035 保持 `implementing`，等待用户人工验收，不自动开始下一功能。

- 2026-09-15 F-035 第五轮人工复核退回 `implementing`：路径 12 的分支提示已显示“请假类别：事假 → 产假”，但最终 `vacateType`/`__virtualName` 仍为“事假”，且 `myDutyName` 与 `global_user_basic_information.dutyName` 相互矛盾，目标没有生成当前处理人不是偶发问题。动作编辑器仍会把英文 `approve` 与固定“同意”重复展示，未校验“同意必须最后”，添加动作的删除入口不可用；节点缺口提示仍只给数量；详情已阻塞而列表仍显示运行中。F-035 第五轮任务已登记，需先修复有效表单值和请求对照，再复验目标处理人；同时完成统一阻塞状态投影、动作和配置提示、无扩展 `runtime.lastError` A/B 及 Vue 生命周期人工验证。详细指导见 `docs/auto/2026-09-15-f035-review-round5-ui-state-and-assignment-task.md`，状态保持 `implementing`。

- 2026-09-14 F-035 第四轮继续保持 `implementing`。已实现页不再“命中即阻塞”：资金往来/投资款前置保存、出版委托日期改写、已登记 NoFormFlow 走 `mixin.saveData` + `flowProxyId`；合同盖章/合规自定义组件仍写前阻塞。配置期身份替换不再新建缺失的 `global_user_basic_information`；无表单 `initiatorRange` 覆盖为当前会话。`auto_audit_info_*` 已写入语义清单 16.1。运行详情请求代次已落地；`runtime.lastError` 产品源码零调用，无扩展浏览器 A/B 仍待人工。真实路径 12 对照未完成，不得进入 `ready_for_manual`。详细指导见 `docs/auto/2026-09-14-f035-review-and-runtime-repair-task.md`。

- 2026-09-14 F-035「目标请求协议一致性、真实处理人和有效表单数据闭环」经评审退回 `implementing`（评审指出 2 Critical + 6 High：
  FormMaking 特殊业务链路缺失、无表单通用兜底、formPersonFields 未迁移、运行时旧身份、全局 customerCode、
  跨节点只查字段缺失、无写后表单核对、矩阵与构造器不一致）。第三轮已全部处理：新增目标业务生命周期登记处
  （命中特殊业务类型的全部写动作门禁阻塞）、vue_custom 页面如实标记 partial 且发起/重提阻塞、
  form_person 人员选择器字段按目标 traverseFlowNode 规则递归收集并生成、执行器在每次发起/审批前用当前会话
  实时身份覆盖登录人字段（岗位缺失阻塞）、写出口按 session.CustomerCode 注入客户码、跨节点核对升级为
  深度值比较（改写即阻塞）、新增写后表单逐字段核对（下一步门禁据此阻塞）、`ValidateBodyMatrix` 在发送前
  强制 required/forbidden/未登记端点。`go test ./test/unit/... ./test/contracts/f035/...` 全部通过，漂移脚本通过。
  自定义组件（contractBusiness、saveCostFundsBusiness 等）的具体目标写接口仍按切片边界以“命中即阻塞”保证安全，
  逐类型实现需后续切片批准。详细记录见功能文档「本切片实施现状（第三轮）」。

- 2026-09-14 F-035「目标请求协议一致性、真实处理人和有效表单数据闭环」经用户批准实施完成，停在 `ready_for_manual`。
  已落地：逐接口协议矩阵登记处 `internal/adapter/target/protocol_matrix.go`（四态字段存在性）与统一写出口信封注入
  （顶层 sid/projectId、data.customerCode，与目标 axios 拦截器同语义）；删除 `verdict` 对 `batchCode` 的无条件禁令，
  `docs/TARGET_SEMANTICS.md` 2.2 节与 F-014 同步改写为逐接口矩阵（submit/draft 顶层必带批次号，reSubmit/audit 等不带；
  批次号绝不作为幂等键）；submit/draft 固定发送 `nextAuditorList` 数组（空时 []）与审批无条件 `tracking`；
  FormMaking 只发 `formProxyId`、无表单只发 `flowProxyId`（submit/reSubmit 强制互斥），运行上下文新增 `FormProxyID`
  从模板唯一 Forms 项取得（多表单阻塞）；计划账号身份扩展岗位事实（目标人员目录 dutyId/dutyName），身份读取失败
  与岗位缺失分别落 `IDENTITY_READ_FAILED`/`IDENTITY_DUTY_MISSING` 阻断，历史账号值不得进入请求；目标节点已到达但无
  currentAuditUserInfo/待办时进入有界轮询（预算来自配置，默认 1 分钟、每 3 秒复查一次、只读），超时按 `assignment_missing` 阻塞并显示
  “目标节点未生成处理人”，不再显示“当前待办已经处理”；step.log 新增协议摘要行。内网系统按用户裁决：日志原样记录
  完整请求/响应（含 SID 与表单正文），不做脱敏，已写入 AGENTS.md。新增 test/contracts/f035/protocol 与 identity、
  test/unit/backend/form_data_by_node、执行器 assignment_poll 定向用例与 f035/drift 漂移脚本。
  `go test ./test/unit/... ./test/contracts/f035/...` 全部通过；test/integration 与真实目标相关的既有失败
  （环境缺失及 F-014/F-018/F-016 既有记录）经未修改工作树复现确认与本切片无关。
  等待人工验收：按功能文档「人工验收」9 项清单在真实目标逐项核对（路径 12 重跑对照手工 curl、身份随计划账号变化、
  真实处理人、分节点表单保留/扣留、NoFormFlow 对比、batchCode 矩阵、阻塞路径不覆盖有效数据）。
  详细实施现状见 `docs/features/F-035-target-assignment-and-effective-form-data.md`。

- 2026-09-14 F-035「目标请求协议一致性、真实处理人和有效表单数据闭环」完成新证据下的只读复核并登记为 `awaiting_approval`。
  运行 6、7 / 路径 12 的 submit 都被目标平台接受并进入“行政综合部-考勤管理”，但目标没有生成
  `currentAuditUserInfo` 和待办。对比人工成功 curl 后确认工具同时存在 FormMaking 误传 `flowProxyId`、表单身份仍是张泽华而计划账号为骆蒙恩、
  `batchCode`/空 `nextAuditorList`/`projectId` 等字段形状缺失，以及其他动作接口尚未逐接口对照的问题；因此不能再把问题归结为目标偶发异步。
  F-035 现改为先建立目标前后端协议矩阵，再修复代理 ID、身份、批次号、显式处理人、各动作读写载荷、处理人有界核验和表单最终快照。
  新增硬性要求：发起人和表单身份只能来自计划当前配置账号；分节点表单必须以目标实例最新完整数据为基线，只覆盖当前节点可编辑字段，后续节点数据不得提前发送；目标页面实际
  发送的字段必须发送，项目中阻挠该形状的旧 `batchCode` 禁令和 `omitempty` 规则必须改写，不能保留运行时猜测。
  当前只更新任务文档，不改目标平台、不新增表，等待用户批准后实施。
  详细任务见 `docs/features/F-035-target-assignment-and-effective-form-data.md`。

- 2026-09-13 F-034 评审整改完成：review 提出 7 项阻塞问题（候选重复动作、isSkip 语义缺失、部分成功返回失败、
  人员校验缺失、状态/节点顺序校验未接入编译、分支解析旧兜底、前端原始英文动作名）全部修复并重跑验证，
  新增 isSkip 三场景、BuildRequest 无写入、人员共享校验、种子多样性等定向测试。同日第二轮评审 3 项
  （手动分支已选路径不得改走其他分支、人员策略与动作提交拆分、目标 message/code 摘要落盘 network.log
  并在请求行展示原文）也已修复并补定向测试。第三轮评审 3 项阻塞（人员策略提交前统一校验、
  路径级阻塞按 stopStepNo 绑定到步骤、actionLabel 全部传稳定 action 键）与 2 项建议
  （包络测试迁移 test/、摘要按 rune 截断）同日修复。`bash test/run-f034.sh` 全部通过。

- 2026-09-13 F-034「一键配置动作多样化与运行详情可读性修复」已获批准并实施完成，停在 `ready_for_manual`：
  一键配置按目标静态动作矩阵为每个业务节点补一条额外动作（发起节点仅保存草稿、审批节点按安全等级选暂存/
  加签/移交/不同意/取回/回退，系统节点不补），逐候选完整场景编译，无安全候选时报告节点与原因；固定尾动作
  占位记录按稳定键清除且不覆盖人工动作。手动分支入口改为沿真实图边跨节点解析（空节点入口精确携带
  nodeProxyId），无选择时写前阻塞；「手动条件分支,请选择/未设置审批人」前置拒绝归为确定阻塞（stopKind=blocked）。
  运行详情弹窗重排为结果摘要/接口请求行式列表/白话耗时说明/折叠排查区，动作名统一中文白名单。
  `bash test/run-f034.sh` 全部通过；history_replay 两个既有失败经 stash 复现与本切片无关。
  等待人工验收：一键配置动作分布与多样性、保存草稿→重新提交仅一次、isSkip 跳过/阻塞/自选分型、
  1005 路径 8 分支入口携带、详情弹窗可读性与复制布局。F-033 仍为 `awaiting_approval`。

- 2026-09-13 用户提出 F-034「一键配置动作多样化与运行详情可读性修复」。只读排查确认：一键配置在节点已有动作时直接跳过，且有固定尾动作时只返回“提交/同意”，非流转动作被全部过滤；1005/路径 8 已保存手动分支选择，但运行时因入口后经过空/导航节点未把入口 `8b676...` 放入审批载荷，目标以 HTTP 200、`isSuccess=false`、`errorType=custom_choose` 返回“手动条件分支,请选择”，工具因拒绝文案未进入清单而落成 `uncertain`；动作预览名称存在英文键回退；节点详情已有 `network.log` 真实计时但首屏仍堆叠内部阶段、逐项检查和复制操作。补充复核确认一键配置还必须服从 GroupApproveManage 前端与目标后端的静态矩阵：首节点额外动作只能是保存草稿，提交为固定尾动作，草稿/驳回/撤回后的再次发起才使用重新提交；`isSkip=true` 的普通节点无人时才允许目标跳过，`run_node_choose` 无人仍是“未设置审批人”阻塞，手动分支空入口仍必须携带入口 `nodeProxyId`；当前编译器定义但未接入的状态/顺序校验和跳过判定缺少 `isSkip` 事实也已列为 F-034 修复项。F-034 计划已补充上述矩阵与测试门禁，当前为 `awaiting_approval`，批准前不改业务实现、不调用目标写接口、不启动浏览器。

- 2026-09-13 F-032 人工验收未通过并退回 `implementing`：原实现把计划状态错误当成编辑锁，运行后不能编辑计划、路径和节点配置，且任务中心文案仍是“运行记录”。本轮修复目标是：任一状态均可重复运行；新增完整计划编辑入口；路径、节点人员/动作和历史来源恢复编辑；账号/流程绑定变更把旧配置标记为受影响；导航和页面改为“任务中心/任务列表”，历史运行事实保持只读。状态聚合和最近任务结果的无 N+1 查询行为继续保留。
- 2026-09-13 F-032 修复完成并进入 `ready_for_manual`：移除状态驱动的计划、路径、节点和历史来源编辑锁；新增 `PUT /api/plans/{id}` 与“编辑计划”页面，计划列表所有状态均可“运行/编辑计划/编辑路径”；身份绑定变化原子标记路径配置为受影响，并立即失效带代次保护的流程图缓存，历史任务事实不改写；导航和运行页面统一为“任务中心/任务列表”。`bash test/run-f032.sh` 全部通过，等待人工页面验收重复运行、编辑计划与路径及任务列表入口。
- 2026-09-13 用户只读排查确认两个缺陷并登记两个独立修复切片，均停在 `awaiting_approval`，批准前不改实现代码：
  ① F-032「计划状态与最近运行结果修复」（`docs/features/F-032-plan-status-and-last-run-result.md`）：计划创建后状态固定
  `not_started` 永不流转、列表不结合 `runs` 表、接口最近运行结果写死为空，并导致运行后路径配置锁定失效；方案是状态按运行事实
  流转（未运行/运行中/已运行，永不回退）、运行事务内同步更新计划状态、列表单条 SQL 补齐最近运行信息，`docs/PRODUCT.md`
  「已完成」同步改「已运行」。
  ② F-033「运行详情原子快照与节点过渡显示修复」（`docs/features/F-033-atomic-run-snapshot-node-transition.md`）：放行后后台协程
  执行期间详情接口非原子读取 CurrentPreview/View，前端轮询覆盖详情导致画布短暂无节点高亮；方案是控制服务原子快照、过渡态
  白话说明、前端保留最后有效当前节点与一次受控快速刷新。
  推进顺序：先 F-032 后 F-033，各自独立验收。
- 2026-09-12 F-031「任务事实参数与实例日志归属修复」已登记为 `awaiting_approval`。源码核对确认：`/web/flowJobTaskLink/list` 的实例过滤必须使用顶层 `flowInstanceIdList`，`data.flowInstanceId` 会被目标忽略；`pending` 使用 `queryUserId`、`done` 使用 `data.executorId`。当前节点处理人必须取发起人已发列表的 `currentAuditUserInfo`，配置中的 `NextNodeAuditors` 只表示下一节点选人。日志方案已按页面调整为 `logs/runs/<运行记录>/paths/<路径运行>`，目录使用 `runId/pathRunId` 稳定定位，`runNo/planName/pathName` 与页面标签对应；目标实例名称只写详情和 `meta.json`，不参与目录寻址。用户批准前不改执行代码、不改数据库、不启动浏览器。详细计划见 `docs/features/F-031-task-facts-and-instance-log-naming.md`。

- 2026-09-13 F-031 评审整改完成（评审提出「一个处理人安全缺陷 + 一个页面验收缺陷」，均已修复并重跑验证）：
  P1 处理人事实缺失改为直接阻断待办动作——写前准备只认 `currentAuditUserInfo` 事实，缺失或核对不命中即
  停在当前步骤返回空快照，不再回退到计划账号/会话本人待办（此前计划账号读到自身历史待办会误放行），
  并新增危险用例 `TestMissingHandlerFactBlocksEvenWhenPlanAccountHasTask`；执行器测试假件补齐事实发现路径
  （事实读取、处理人账号解析、视角查询），既有用例由此走生产同一条路径。P2 页面禁用词整改：状态名
  「核验中」→「确认结果中」、失败分类「演员不可解析」→「处理人身份未确认」，路径配置/动作目录/场景编译/
  控制事实/断点说明/运行命令/运行详情提示/表单运行时维护中的「演员、核验、落账、门禁」全部换词；
  `web/src` 渲染文本扫描无禁用词。`go test ./test/unit/...`、`bash test/run-f031.sh`、
  `bash test/run-f030.sh` 全部通过（两处既有失败与本切片无关，见功能文档记录）。

- 2026-09-13 F-031「任务事实参数与实例日志归属修复」已按文档实施完成并停在 `ready_for_manual`：
  `/web/flowJobTaskLink/list` 收敛到唯一载荷出口（实例筛选只写协议顶层 `flowInstanceIdList`，pending 用顶层
  `queryUserId`，done 用 `data.executorId`），`FindDueFlow`/`FindDoneTaskOnNode` 迁到该出口，done 复用统一快照读取
  并遍历全部分页、按当前真实操作账号视角核对；当前处理人发现改为「已发列表 `currentAuditUserInfo` 事实优先」，
  删除按配置候选人登录与扫描的全部路径，解析结果按用户 ID 去重，缺失事实时记录字段与实例 ID 并停步；
  运行日志改为 `logs/runs/运行_<运行号>__<计划名>__run-<运行ID>/paths/<路径名>__path-run-<路径运行ID>__path-<路径ID>/`，
  与页面「运行记录 -> 路径运行」逐层对应，实例名称只进日志字段与 `meta.json`（历史日志不迁移），
  运行详情直接返回日志相对目录与实例名称。`bash test/run-f031.sh` 与 `bash test/run-f030.sh` 全部通过，
  结构契约脚本经变异验证。既有失败如实记录且与本切片无关：`test/unit/backend/history_replay` 两个用例
  （历史链路调用未批准端点）与 `TestF016RunControlResolvesRunIDToPathRun`（断点回放数量断言），
  均经 `git stash` 在改动前工作树复现。等待人工核对：连续两次运行的目录归属、`currentAuditUserInfo` 与动作详情
  处理人对照、`network.log` 字段与修复前后请求数/耗时对比。详细计划与状态记录见
  `docs/features/F-031-task-facts-and-instance-log-naming.md`。

- 2026-09-12 全局空态改为纯文字提示（用户要求所有空状态去掉图标）：共享空态组件去掉插画，只留标题、简短说明和下一步操作；组件库 `NEmpty` 的空态图标（含表格、下拉候选、级联等组件内部空态）在全局样式层统一隐藏；`AppEmptyIcon` 组件与应用层空态图标注册删除。空态与错误态边界不变，`docs/PRODUCT.md` 产品原则同步改为「正常空数据只给文字提示」。
- 2026-09-12 F-030 人工验收反馈（评审 4 项：同账号活跃路径仍会启动新路径、请求无法稳定归属步骤/尝试/阶段、阶段说明泛化且预览无进度上报、候选处理人扫描耗时未消除）已全部修复，状态退回 `implementing` 后再次停在 `ready_for_manual`：调度器两轮扫描把活跃路径账号计入忙账号；执行器把 step_id/attempt/phase 注入目标请求上下文，请求明细按日志行直接归属；阶段说明统一为带动作、节点、处理人和下一步的白话口径，预览阶段实时上报进度，页面不回显内部阶段名；候选扫描改用目标原生 queryUserId 视角窄查询（源码锚点已入语义清单第 4 节），仅命中者登录。`test/run-f030.sh` 全部通过；基准复测与 queryUserId 真实部署行为仍待人工验收确认。
- 2026-09-12 F-030 全量实施完成，状态进入 `ready_for_manual`：任务列表 memo 覆盖整个预览调用树（同账号×实例×状态只扫一次目标分页，写后核验天然全新）；候选人账号目录批量解析；调度器按计划账号同账号串行排队（同轮 Tick 每账号最多启动一条，跨运行生效）；账号锁等待 >100ms 结构化日志；请求耗时明细来自 network.log 传输层真实计时（duration 缺失显示「未知」不显示 0ms，汇总「部分未知」不把未知当 0 求和）；节点动作详情重排为摘要区/四指标/本地过程/请求明细表/折叠原文，明细带阶段（用户口径）与可复制 trace；当前步实时阶段说明条渲染 currentPhaseNote；写端点白名单补齐 approverAppend/rollBackThePreviousLevel/revocation/urgeHandleRecord/transpond/flowTracking。汇总测试入口 `test/run-f030.sh` 全部通过。量化基准复测（请求数-50%/本地等待-50%/端到端-30%）留待用户验收时同一样本对照。
- 2026-09-12 用户反馈流程整体耗时过长并要求“不影响功能”优化，同时在节点动作详情展示真实接口请求耗时、重新排版动作详情，并补充要求消除“准备运行/准备放行”等泛化描述。已登记 F-030「执行性能与节点动作耗时展示」，状态为 `awaiting_approval`：计划聚焦同一步重复只读读取、同账号无效排队、目标传输层真实耗时、节点详情分层展示和逐阶段白话说明；明确不改变节点/动作顺序、一次写约束、重试边界、写后核验、账号安全和运行结论。默认不增加数据库迁移，先消费现有 `network.log`，若日志无法稳定归属请求需另行批准最小模型变更。用户批准前不改实现代码、不启动浏览器、不进入 `implementing`。计划见 `docs/features/F-030-execution-performance-and-request-timing.md`。

- 2026-09-11 F-029 会话缓存与网络重试已完成实施与定向验证，状态进入 `ready_for_manual`：服务端读服务与执行器共享同一目标客户端/会话管理器；同账号验证、读取、写入、核验串行，不同账号互不阻塞；会话缓存增加代次校验，迟到旧 SID 不能覆盖刷新后的新 SID。`RESP401`/`AUTH_401`/`SID已失效` 只对当前账号自动重登一次，第二次失效明确停步提示外部会话竞争。完整业务拒绝保留原始 code/message 且不重试；读取网络错误、写请求未写出连接错误按边界有限重试，写出后响应丢失不重发。网络日志新增 retry、attempt、transport phase、request written 字段，内网测试环境按原样记录请求与响应，日志文件不经 API 且不提交 Git。定向 Go 单测、竞态测试、API 契约与构建检查通过；等待用户页面连续推进与人工制造 TCP timeout 验收。

- 2026-09-11 目标会话机制源码勘定（LoginServiceImpl.java）：SID 为随机 UUID 按 sid 键存 Redis，
  有效期取决于目标配置 `login.operation.timeout`，该值与本地工具的 8 小时缓存上限不是同一个配置；源码只能证明按 SID 存 Redis、登录不主动删除旧 SID，不能据此断言目标一定不存在账号级会话竞争。

- 2026-09-11 核验失败改为「停在本步可重新核验」（用户裁决：工具自己的读取错误不得把运行判成终局）：
  写请求成功发出但核验重读失败（会话失效/目标抖动）时，运行保持运行中、停在本步、不落结论尝试行；
  重新放行只重读核验结果（Reverify 模式，绝不重发写请求），核验通过即正常推进。实测运行 80 第 4 步：
  会话限制惩罚窗口内反复核验失败均停在本步、状态不终局、事实不丢；窗口过后重新放行即可继续。
  历史运行曾观察到多账号连续登录与会话失败同时出现，但现有源码和日志不足以证明是目标账号级互踢；本轮改为记录传输阶段与外部竞争提示，等待人工复核。

- 2026-09-11 会话管理修正与账号使用锁：统一会话（按账号全局缓存 SID、失效自动重登）已补上共享协调器、当前会话账号跟随和账号使用锁；用户可见文案全面去掉「演员」改为「当前处理人」。旧记录中“目标必然互踢”的表述降级为待复核假设，不再作为产品结论。

- 2026-09-10 F-028 人工验收实跑定位并修复四个缺陷（状态保持 `ready_for_manual`）：按用户裁决实现会签多人审批循环（同意后实例仍在本节点则用新发现的当前处理人原步重跑，每人一次独立七阶段独立落账）、竞签一人即走；当前处理人发现改用「发起人已发」列表的 currentAuditUserInfo（实例事实一律发起人会话读取），目录按 bizId/手机号/姓名匹配真实用户 ID。遗留：多账号快速登录环境下目标会话互相踢，核验读取仍可能撞「SID已失效」被判结果待确认（运行 77 第 4 步），会话保持策略需独立决策。

- 2026-09-10 F-028 人工验收实跑定位并修复四个缺陷（状态保持 `ready_for_manual`）：
  ①演员目录读 RESP401 改走重登+重放；②prepare 阶段失败补落失败事实行（修复画布失败节点标错、
  用户看到的「当前节点没有了」）；③同意载荷携带手动分支代选与链路自选节点人员（修复
  「手动条件分支,请选择」「未设置审批人」两类拒绝）；④目标「无处理人时跳过该节点」的运行时适配
  （新增 skipped 步骤状态，无写推进）与「指定人员」节点的代理树人员发现+会话切换。
  页面实跑：运行 67 重试+重新提交成功；运行 72 单步推进至第 9 步（跳过与固定人员审批均实测通过）。
  遗留缺口如实记录：会签节点需同节点多人审批（审核人3「满足 3 人」），路径 1 后半被此阻塞，
  需独立切片决策。语义清单第 4/9/13 条更新为勘定中，详见 `docs/TARGET_SEMANTICS.md`。

- 2026-09-10 F-028「失败动作重试」已获用户批准并完成实施，状态进入 `ready_for_manual`：
  运行画布顶部操作区与节点检视面板错误页签在服务端判定 `retryable` 时显示「重试失败动作」；
  点击后同一路径运行从失败步骤重新装填（单事务重开聚合、清空结论、释放租约、追加事件与控制事实），
  按原模式继续（人工控制等放行、单步等执行一步、自动继续且断点按事实回放）。前缀一致性校验拒绝失败后改过配置的重试；
  待对账、已停止、已取消、已完成一律拒绝。`./test/run-f028.sh` 汇总通过（定向单元、真实 MySQL 集成 0 跳过、
  实现契约、前端类型检查）；真实目标端到端写验证等待用户人工核对。

- 2026-09-10 用户提出新需求「动作执行失败以后，该动作需要一个重试按钮再次发起该动作」，已登记为 F-028
  失败动作重试并产出计划，状态 `awaiting_approval`：只允许对确定失败（确认无目标副作用）的动作重试，
  同一运行从失败步骤重新装填、按原模式继续，事实只追加不改写；结果待确认（写不确定）维持 2026-09-06
  终局裁决不提供重试。计划见 `docs/features/F-028-failed-action-retry.md`，等待用户明确批准后实施。

- 2026-09-10 F-026 人工验收反馈已修复，继续停在 `ready_for_manual`：第 4 步“同意”第一次放行被误判“当前待办已经处理”，根因是计划账号骆蒙恩读不到审核人成浩楠的待办；执行器现在在计划账号待办为空时，使用下一节点已解析候选人逐个切换只读会话并重读待办，仍保持写前实时门禁。目标实例命名同步改为“计划名-路径名-YYYY-MM-DD HH:mm:ss”，与运行日志目录层级一致；运行详情顶部同步显示计划和路径与开始时间。步骤弹窗和画布卡片改为大白话检查文案、加高卡片支持两行完整结果；流程画布不再随执行自动跳动，只保留用户点击“定位当前节点”。

- 2026-09-10 F-026「状态感知的动作循环编排」已完成自动验证，状态为 `ready_for_manual`：固定提交/同意从可配置动作下拉移除，
  由状态编译器在节点末尾生成 `system_default` 尾动作；“不同意 ×2”等离开节点的重复动作会补齐重提、逐节点同意并返回原节点的恢复链，
  连续草稿只在尾部重提一次；实时门禁变化降级为非阻断提醒。人工控制按 `ReleaseGroup` 动作组放行，恢复链内部每个物理步骤仍独立重读、落账并遵守一次写。
  `./test/run-f026.sh` 已汇总定向 Go 用例与前端类型检查；下一步等待用户按 `oyg测试1003/路径1` 做人工验收，运行记录删除属 F-027。

- 2026-09-10 F-016 新一轮人工验收反馈已修复，状态重新进入 `ready_for_manual`：计划 `oyg测试1003` 的 `路径1`
  配置“保存草稿 → 保存草稿 → 提交”，旧编译器却在第一次保存草稿后立即插入重新提交，目标正确进入“审核人1”后，
  下一编译步骤仍要求回到发起人节点，因而误报路径偏离。现连续草稿保持在发起人节点；已有草稿、驳回或撤回主实例后的
  “提交”统一编译为一次系统“重新提交”，不再发起第二主实例。回归用例已在旧实现复现并在修复后通过，
  `./test/run-f016.sh` 完整通过；计划 27、路径 4186 已保留原动作和人员配置重编译到修订 39，落盘前四步为
  “保存草稿 → 保存草稿 → 系统重新提交 → 审核人1同意”。既有运行 61 未被修改，等待用户新建运行复核。

- 2026-09-10 F-016 人工验收反馈已修复，状态重新进入 `ready_for_manual`：运行详情曾把目标结构读取限制为 500ms，而本次真实模板树读取已记录 628–661ms 成功响应，轮询因此周期性把正常读取误判为“节点运行状态可能不完整”。现将预算调整为 2 秒；首次放行前仍不显示结构降级提示，执行开始后只有实际超时或读取失败才如实提示。新增回归用例锁定读取预算下限；F-016 定向验证、真实 MySQL 集成测试、写端点白名单契约与前端构建全部通过，等待人工页面复核。

- 2026-09-10 F-025 第三轮人工验收反馈已修复，状态重新进入 `ready_for_manual`：空节点服务端原本已无人员、仅返回只读系统动作，一键配置和动作保存也不会写入；但节点侧栏把实例级动作目录合并进所有节点编辑器，导致空节点仍显示“动作配置”和可配置数量。现以服务端 `systemOnly` 语义识别系统自动节点，空节点只显示“无需配置人员和动作”，不挂载人员、动作和保存入口；前端回归契约、系统节点动作门禁、一键配置定向验证及类型检查通过，等待人工页面复核。

- 2026-09-10 F-025 第二轮人工验收反馈已修复，状态重新进入 `ready_for_manual`：节点保存成功状态原本只在父页面赋值并传入侧栏，但侧栏模板没有渲染，自动定位下一节点时还会提前清除。现改为保存收尾直接显示全局成功通知，当前节点保存、响应丢失后的服务端对账成功和全部节点保存共用明确反馈；新增前端回归契约并完成定向验证，等待人工页面复核。

- 2026-09-10 F-025 人工验收反馈已修复，状态重新进入 `ready_for_manual`：发起人节点新增“提交”动作时，浏览器提交节点修订 `1`，服务端错误按整体修订 `2` 校验并返回 `CONFIG_REVISION_CONFLICT`。按用户“第一版不做这类版本控制”的要求，节点与实例动作请求不再发送或接受浏览器修订号，服务端改为在最新配置上合并；内部并发冲突自动重读，仍保留幂等键和数据库事务保护。去掉 revision 后同一业务请求实测返回 200，发起人节点重读包含“保存草稿、提交”，同一幂等键重放仍返回同一修订；携带 revision 的旧请求明确返回 400。定向 Go、Node 测试和前端类型检查通过，等待人工页面复核。

- 2026-09-09 F-025「节点人员配置与范围内选人」已完成自动验证，状态进入 `ready_for_manual`：人员候选失效、数量不足/超限和策略不再适用均改为具体原因，并直接指出“本节点的处理人员区”；前端不再把服务端原因替换成泛化提示。批量保存会自动定位首个缺项节点，遗漏列表中的其他节点名称可点击直达。已通过 `go test ./internal/analyzer ./internal/service ./test/unit/backend/action_orchestration`、`npm --prefix web run typecheck` 和 `node --experimental-strip-types --test test/unit/frontend/path_configuration/personnel_message_test.mjs`，等待人工页面验收。

- 2026-09-09 F-025「节点人员配置与范围内选人」已获用户明确“开始”批准，状态进入 `implementing`。已核对当前实现与 V1/目标 `GroupApproveManage` 直接组件：节点人员、动作接收人和表单人员必须分域；可编辑范围默认稳定随机一人，手动只能在当前目标范围内选择；固定人员、主管/职级/扩展属性/表单人员只读显示目标运行时规则；会签数量、候选失效和目录解析失败必须阻塞或标记受影响。

- 2026-09-09 全局空态反馈修复：用户指出上轮只修改运行记录、计划列表仍显示默认叉号“无数据”。现已在应用配置层统一替换组件库空态插画，覆盖表格、下拉候选及所有现有 `NEmpty`；计划列表和运行记录使用共享业务空态，按首次无数据与筛选无结果提供对应说明和下一步操作。原先借用空态组件展示的读取失败、资源不存在和运行时不可用已改为独立错误/警告结果，正常空态与错误态不再混用。全局视觉契约纳入 F-000 与 F-021 验证。

- 2026-09-09 F-021 人工验收反馈修复：运行记录列表正常空态改为主题色轻量插画、标题、简短说明与下一步操作；首次无记录可前往测试计划，筛选无结果可查看全部记录，读取失败继续使用独立错误提示。状态保持 `ready_for_manual`，等待人工复核。

- 2026-09-08 F-019 全动作收口：15 个动作已逐项核对为 12 个目标原子端点；重新提交补齐当前公司和提交同源选人参数，任务动作在发送前读取实时任务/批次/节点，写后按动作专用目标状态确认。加签以完整代理树更新，转发辅助实例经目标查询确认并以运行事件独立记录。执行失败页面保留当前运行，用户可直接看到目标原始错误；本地参数错误明确为未发送请求。`test/run-f019.sh` 含真实目标只读集成通过，逐项真实写仍待人工验证，F-019 保持 `ready_for_manual`。

- 2026-09-08 参考前端复核：`rsh-cloud-invest-power-system` 当前本地提交为 `586ca09474c3`，已按 `GroupApproveManage` 及直接组件检查，动作端点、任务标识与写载荷语义均未变化，`TARGET_SEMANTICS.md` 证据基线已更新。`make refs-sync` 因该参考目录本地分支为 `oygdev2`、清单要求 `test` 而安全中止，未对参考源码执行任何改动。

- 2026-09-07 晨「交付缺口返工」窗口（docs/auto/2026-09-07-delivery-repair-task.md，报告
  docs/auto/2026-09-07-delivery-repair-report.md，总体未完成不写收口）：本线程关闭缺口 1（旧报告
  「已收口」撤销）、缺口 7（动作配置删除保存未生效——真实 API 复现证明后端全链路无缺口，根因是
  前端 saveCurrentNode 的重读重建草稿把实例动作编辑静默丢弃、实例容器保存成死代码，已修复并加
  服务层回归）；完成阶段 1 真值表（docs/auto/2026-09-07-target-action-matrix.md，15 动作三跳源码
  证据，重大纠偏：加签目标端点是 updateFlowProxy+整树+所有权复制，已完成与移交
  approverAppend+实时 userIds 语义的拆分，真实写验证待人工执行）；run-f019.sh 纳入动作编排回归；画布删除整卡脉冲并给节点卡片补
  步序·动作·处理人·耗时事实行。并行线程同窗完成提交载荷 nextAuditorList 与会话策略对齐 V1。
  未完成项（执行链缺口 2-6、右栏诊断缺口 9、15 动作全链路测试、十个真实样本）在报告中如实列出。
- 2026-09-07 「全自动交付验收」收口（docs/auto/2026-09-07-delivery-acceptance-report.md）：
  行为修复全部完成——移除用户侧对账、运行记录一行一次运行+二级路径页+安全删除、
  运行中自动/单步互切（安全边界生效、条件写幂等）、画布质感升级（状态点呼吸/选中态/错误摘要）、
  返回入口与内容区对齐。样本链路实测把工具写链路推进到目标业务层并沿途修复五个会话缺陷
  （AUTH_401 失效形状、探活形状毒化会话、写端点会话校验与读不同步、恢复重发、图读取短缓存），
  新语义条目记入 TARGET_SEMANTICS 1.8。十个真实样本未全部跑到终态：一键配置把「保存草稿」
  编译为第一步且绑定历史主实例被门禁必然拒绝、目标模板 oyg流程分支测试 未设置审批人，
  两项阻断与剩余顺序如实记录在验收报告第五节。
- 2026-09-06 深夜「全自动交付验收」任务书执行（docs/auto/2026-09-06-delivery-acceptance.md）：
  第一项行为修复「移除用户侧对账」已完成。产品口径改为：写结果无法确认（超时、响应丢失、服务重启）
  是终局——路径运行进入「结果待确认」并闭合，运行聚合收尾为已停止；界面只有一句大白话说明与只读记录，
  不再有任何对账、重试或人工登记入口；重启恢复后全部闭合的运行聚合在启动阶段直接收尾，不留僵尸运行。
  删除 internal/engine/reconcile、control 的对账与重建、/api/runs/{id}/reconcile 与 /recovery 端点、
  人工结论登记与对账三列展示；保留写前基准落库（迁移 029）、核验重读与 recovery.log 内部判定。
  新契约 test/contracts/f018/reconcile_readonly.sh 反向锁定「对账不得回来」；
  新集成用例锁定不确定收尾与重启收尾（真实 MySQL）。F-018 定向脚本已重写并通过。
- 2026-09-06 运行详情页按用户验收反馈做画布优先改版（用户逐条裁决）：返回入口与运行身份 Teleport 到应用顶栏、
  页签顶到左上角、操作区在右上角、剩余高度整块给流程图；画布只高亮当前步并按本地时钟播执行动画，其余节点压暗
  （失败与待对账不压暗）；右侧改为「点节点才出现」的检视面板（配置／运行信息／错误三页签，明细进详情弹窗）；
  断点区收进操作区的「断点（N）」弹出层；定位按钮改为画布内浮动常驻入口，页面全屏时也在。
  后端新增只读 `nodePlans`（按图节点 ID 归组的已配置计划动作，来自已保存编译场景，不额外读目标平台）。
  顺带修掉三个既有缺陷：侧栏用两套键空间比对导致当前步预览永不显示且节点标题回落成内部 ID、
  `NAlert` 未导入导致停止原因提示渲染不出来、画布初始定位被误判成「用户接管」导致自动跟随一进页面就停。
  验证：`pnpm check:frontend` 通过；F-016/F-017/F-018/F-020/F-021 界面结构契约与 F-004/F-006 结构脚本通过；
  新增 `test/unit/backend/run_orchestration/node_plans_test.go` 锁定键空间翻译与中文口径。

- 2026-09-06 F-015 至 F-023 全部处理完成（每项停在 `ready_for_manual`）：F-015/F-024/F-016 完成评审修复轮；
  F-017/F-018 完成 claude 只读评审修复（断点并发加锁、控制事实流水、对账读取失败改「重新对账」、
  ReadError 贯穿判定链、人工结论终局守卫、恢复动作中间态回滚等）；F-019 评审因网关 524 两次超时按规则重试；
  F-020 多路径调度与单次定时启动全新实施（迁移 030、调度 Worker、聚合收尾、按路径寻址、
  幂等启动、事件与列表增量，真实 MySQL 集成与零写多路径运行验证）；F-021 增量实施（事件流时间线、
  状态筛选、权限清单快照、日志位置复制）；F-022 套件入口与覆盖契约；F-023 五服务编排与结构契约
  （冷启动实测被本机 Docker Hub 不可达阻断，已如实记录并保留显式启用开关）。

- 2026-09-05 F-015 claude 只读评审返回 22 条意见，有效项集中修复并重新验证：预检弹窗运行模式三选一
  因未导入 `NRadio` 退化为死文本（真实写风险最高的死控件）、两处裸 input、内部节点键回显、启动失败无反馈
  全部按纲领第 12 节返工；目标结构读取失败按根因分类（新增 401 重新验证账号语义）并有界重试；
  「摘要已配置却读不到配置记录」的矛盾状态改为阻塞；勾选部分失效改为报错；锚点 `?panel=` 接通表单数据面板；
  用户取消全选不再被后台刷新覆盖；新增 API 层 404/400/401/502/503 行为测试；三份文档与纲领第 9/11 节
  同步「未验证动作只提醒不阻塞」的用户裁决。F-012 已由用户人工验收通过进入 `accepted`。
- 2026-09-05 F-012 已由用户完成人工验收并明确确认通过，状态进入 `accepted`；功能文档、ROADMAP 已同步注明。该切片不再测试、复验或修改。

- 2026-09-05 用户授权真实不确定写、要求 F-018 当场收口，已完成并停回 `ready_for_manual`。
  查清并修掉的是一件比"缺一次实验"严重得多的事：**恢复层在真实产品里原本整段不可达**——
  不确定写落账时内存现场被清空（只读对账只会回答"没有等待放行的步骤"）、待对账被按终态一律拒绝、
  迁移表与租约都不允许回到运行中、尝试主键没取回导致对账三列的 UPDATE 落在 id=0 上。
  四处缺陷各自都足以让三个恢复动作一个都执行不了，而单元用例直接喂 `Dims`，把缺口完全遮住。
  同时修掉三处记账缺陷（重放上限从未生效、`is_replay` 恒为 false、对账事件行的 run_id 填成路径运行 ID）。
- 真实证据：运行 12 取得真实不确定写（实例 6bd617f3…，trace c6ea7555b51afaee）并**第一次**在真实目标上
  跑通只读对账、五维真实读数、唯一动作校验与人工结论登记（recovery.log 首次有真实内容）。
  只读复查进一步证明那次"不确定"是工具自己的读取口径问题：按实例精确复查带了公司业务关联过滤，
  把本工具发起的实例整条排除（语义清单第 19 条，34 个证据块通过漂移检测）。修掉后运行 13 同一条发起写
  判为确定成功（实例 7bcf0c29…，trace 0715f0cdf7cc2700），F-016 遗留的"实例可见性"未决事项一并清掉。
- 如实记录：「未生效 → 重放」的真实目标证据结构性不可达，不是漏跑实验——发起类写永远得不到该结论、
  对已有实例动手的执行接线属 F-019、且测试账号手上没有任何审批待办（实测 169 条全是草稿 2/驳回 83/撤回 84）。
  该链路正确性由真实 MySQL + 真实状态机的集成用例保证（只有目标读写是假件）。
- 2026-09-05 按用户要求根治「对账现场只在内存里」：把缺的那一块事实补进库而不是让现场常驻内存——
  写之前的目标事实随尝试行落库（迁移 029，`run_step_attempts.before_facts`），
  新增 `Rehydrate` 按运行事实重建现场（游标来自 run_steps、已用重放数按 is_replay 还原、断点由控制事实回放），
  重建按需触发在对账与恢复动作上。基准缺失（历史尝试或崩溃在落账前）一律如实降级为仍无法判定，
  绝不把零值基准读成「写之前实例不存在」。同时修掉一个新引入的陈旧文案：保留现场后待对账一度还显示
  「执行一步」，现已固定为空命令集。真实验证：运行 12 重启后按运行事实重建并给出真实五维读数。

- 2026-09-05 核查线程按用户要求复核 F-017 至 F-023 七个切片的计划，已统一口径并纠正问题，
  七份文档各写入同一份「口径对齐」小节（写载荷规则、节点真实标识、迁移编号从 029 起、语义清单 17 条、
  16 个动作与 12 个写端点、控制端点当前按运行 ID 解析成第一条路径运行、实施前必须先确认既有实现）。
  纲领新增第 12 节「界面与交互质量基线」并被七份文档引用与写入完成标准：六条硬性禁止
  （裸 input、操作堆砌、整页遮罩、空白态、界面暴露内部标识、陈旧文案）加九条必须做到。
- 核查发现的状态口径不一（已如实同步）：F-017、F-018、F-019 的功能文档写 `ready_for_manual`
  且实现确已落库（`internal/engine/control/`、`internal/engine/reconcile/`、9 个动作写端点、
  迁移 027/028、三个定向入口与契约脚本齐备），但 `docs/ROADMAP.md` 仍写 `awaiting_approval`。
  路线图已按功能文档改为 `ready_for_manual` 并注明界面部分待按纲领第 12 节返工。
  三个切片的实际缺口已逐条写进各自文档「本切片现状」小节，按补齐与返工推进，不重建第二套实现。

- 当前功能：F-024 节点级字段权限与分段填写，已停在 `ready_for_manual`（用户 2026-09-05 明确「开始」批准范围）。
  交付三件事：写请求的表单数据改为「实例当前完整数据 + 本节点声明可编辑且已配置字段」（目标保存是整份覆盖，
  语义清单第 16 条）；发起只带发起人可编辑字段，只有后续节点才能编辑的字段不再提前提交；
  数据工作区可按节点切换填写视图，条件字段标出「将在哪个节点自动填写」，一个节点都填不了的决定性条件字段阻断。
  顺带修复一个阻塞性缺陷：编译场景的 nodeKey 是哈希派生的不透明键，此前被直接当作目标 nodeProxyId 使用，
  导致同意永远报「无待办」、事实重读永远判「待办已消失」（没生效的写会被判成确定成功）、对账对照失效。
- F-024 已知边界：当前测试流程（合同盖章评审）的审批节点可编辑字段都是发起人可编辑字段的子集，
  这条路线上看不到分段填写效果；全库 2732 个流程模板里 1718 个存在「审批节点独占可填字段」。
  目标后端是否拦截越权字段仍未实测；不同意动作在统一动作载荷里缺分支（F-019 既有缺口，失败安全）。
- 表单条件字段修复（提交 fc6893d）：目标条件常直接声明 FormMaking 虚拟显示字段
  （实测 `classificationId__virtualName eq 施工类`），此前运行时协调固定优先取同前缀名称字段当期望名称，
  于是历史值与绑定值"自洽"，施工分类永远停在行政综合类。现改为按补丁真正写过的那个显示名取期望值，
  两个显示名矛盾时阻断；服务端补齐反方向同步，避免"路由按新分类走、业务名称还是旧分类"。
- F-024 分类回显复验修复（提交 51c2594）：运行时会话端点此前直接发放缓存 SID，目标已经作废该会话时，
  合同分类远程选项接口返回 `RESP401`，造成右侧补丁已是「施工类」而控件仍保留「行政综合类」。现于发放前
  先探活，失效时复用只读单次重登并发放新 SID；过期缓存定向回归、相关会话测试、编译与 vet 已通过。

- F-024 分类回显再次复验（2026-09-09）：人工反馈界面仍显示「行政综合类」后，确认根因是 FormMaking `refresh()`
  只启动自动数据源而不等待返回，分类协调和宿主 `editData` 二次刷新会被迟到响应覆盖。现以 XHR/fetch 请求跟踪器等待
  所有目标接口归零并稳定后再协调，保留服务端补丁意图以覆盖迟到旧绑定值；宿主二次刷新后再做最终一次协调，且每个会话/操作
  使用独立代次，旧操作不能继续写当前表单。加载空白阶段显示「正在加载表单结构和历史数据」「正在加载表单组件和远程选项」，
  最终阶段显示「正在自动修正关键路径数据」。前端 49 项定向测试、运行时类型检查和生产构建通过；`test/run-f024.sh`
  仍在既有 F-012 人员目录端点 `/web/user/api/user/findByCompanyIdUserList` 白名单缺口处失败，未在本轮扩张修复。

- F-024 loading 复验（2026-09-09）：人工反馈加载遮罩长时间停留在「正在加载表单组件和远程选项」。首轮选项协调已收敛为最多 20 轮，
  最终校验只做 1 轮；目标请求屏障默认超时从 120 秒收紧为 45 秒，超时会关闭 loading 并返回具体接口。遮罩固定在页面顶部 56px
  以下，不覆盖顶部工具栏。前端 49 项定向测试、运行时类型检查和生产构建通过，状态保持 `ready_for_manual`。

- 当前功能：F-016 执行器最小真实闭环。补做评审返回未通过，经用户裁决按全量一轮修复：
  三类高危判定缺陷（审批伪造 200、零写入判写不确定、控制端点运行 ID/路径运行 ID 错位）与全部存活中低项已修复
  并带回归用例（提交 42428c4..c0c16b9、6edcc7f），修复轮细节见 F-016 功能文档 2026-09-05 状态记录，
  现重新停在 `ready_for_manual` 等待人工验收。F-012 仍由另一线程返工（`implementing`）；
  F-015、F-017、F-018、F-019 为 `ready_for_manual`，F-013 与 F-014 已由用户明确验收为 `accepted`。
- F-015 范围已按用户决定收敛：成功断言整体移除（断言表、领域类型、`internal/engine/assert`、两个只读端点、
  前端断言卡片与相关测试全部删除，迁移 025 删表，契约脚本反向锁定不得回归），跑到哪里算成功改由后续断点功能表达。
  当前交付只有运行前检查：五类用户能直接处理的阻塞（节点配置未完成、基础表单数据未就绪、配置里标记为阻塞的问题、
  编译场景为空、路径拓扑与真实结构不一致）、一类安全阻塞（路径配置读取失败）、一类纪律阻塞（动作尚未验证可执行），
  提醒与阻塞分区、阻塞可点击定位，只检查本次勾选的路径。
- F-015 人工复审 5 个问题已修复：路径配置读取失败原先与"配置不存在"合并、路径摘要正常时会错误放行为可运行，
  现新增独立阻塞类别；首次进入页面未套用默认勾选，现改为调用既有 `defaultRunSelection`；
  功能文档与实现严重不一致（仍把已删除的断言功能记为完成），已按当前实现整体重写并同步三份文档；
  `test/run-f015.sh` 原先没有执行任何集成测试却声称真实 MySQL 与真实目标测试通过，
  现新增 `test/integration/f015_run_readiness_mysql_test.go` 三个真实 MySQL 用例并在脚本里实跑、禁止跳过，
  同时更正"覆盖真实目标"的说法；计划存储故障原先一律返回"计划不存在"，现按 `PlanError` 种类分开映射为
  404、参数错误与可重试的存储错误。
- F-015 关键实测结论：真实目标模板一个 `end` 类型节点都没有（只有 start/manual/condition/common/empty/synergy），
  流程直接终止在没有后继的审批节点上。该结论是当初移除成功断言的实测背景之一。
- F-015 已知缺口（如实记录）：`person_not_unique` 与 `semantics_pending` 两类阻塞的服务层数据源未接，
  前者要改并行线程正在返工的 F-012 人员模块，后者的映射关系尚无定义；人员问题仍通过 `config_issue` 阻塞。
  `verifiedRunnableActions` 为空集是纲领第 9 节的要求，F-016 之前任何已配置动作的路径都会被阻塞。
- F-014 已完成：新增 `docs/TARGET_SEMANTICS.md` 作为目标平台行为结论的唯一载体，14 条标题建立，本切片只填错误语义与幂等两条；新增纯判定包 `internal/engine/verdict`（三值结论、五项输入含动作与端点、20 格矩阵、兜底一律不确定、会话失效覆盖 `AUTH_401`、`batchCode` 禁令），未接入任何现有流程；新增判定表对照测试、真实目标只读集成测试、写端点白名单与证据漂移检测脚本、`test/run-f014.sh` 汇总入口。
- F-014 关键结论修正：审批端点在乐观锁检查之前已写入 Mongo 表单数据且不受该事务约束，因此「乐观锁冲突 + 明确未变」由确定失败降级为不确定。
- F-014 实测部署事实：目标返回的 `auditWay` 是字符串编码名，说明已执行 20260828 迁移；失效会话真实响应是 HTTP 200 + `code=RESP401`，会话失效不一定伴随 HTTP 401。
- F-014 边界：纯只读，未发出任何写请求，未执行任何重复写探针，六条待实测结论均标注待 F-016 实测并配同编号探针。
- 当前状态：`ready_for_manual`（日志查看方式与日志归档方式两轮人工验收反馈均已修复并实测通过，等待重新验收）。用户 2026-09-04 明确：暂未发现问题，但不视为明确验收。
- F-013 人工验收反馈（已修复）：“日志没有按计划和执行路径归档，配置日志集中在日期目录，无法从业务对象定位。”
  现在顶层只有 `logs/application/<日期>/` 与 `logs/plans/<计划显示名>__plan-<ID>/`：
  配置阶段落 `configuration/<执行路径显示名>__path-<ID>/<日期>/`（`meta.json`、`operation.log`、`operation-error.log`
  与三个网络日志文件），执行阶段落 `runs/<执行路径显示名>__path-<ID>/<运行号>/`（`execution.log`、`execution-error.log`
  与同样三个网络日志文件），只知道计划时落 `configuration/_plan/<日期>/`。
  `logging.Scope` 增加计划与执行路径的 ID 与显示名并进日志字段，`WithScope` 改为合并语义；
  中间件只从路由取不可变 ID，显示名由 `service.LogScopeService` 从真实业务记录读取并完成归属校验，每请求解析一次；
  作用域随 context 传给目标站点客户端，`network.log`、`curl.log` 与业务错误日志因此都落进同一个计划目录；
  `application` 只保留启动停止与无法归属业务对象的系统级事件，业务日志不再重复写入根目录。
- F-013 归档实测（真实数据）：计划 2（`oyg测试002`）路径 13（`路径 1`）的配置接口日志落在
  `logs/plans/oyg测试002__plan-2/configuration/路径 1__path-13/2026-09-04/`，每行带 `plan_id`/`plan_name`/
  `execution_path_id`/`execution_path_name`；路径 13 与路径 14 互不串目录；只带计划 ID 的接口进 `_plan`；
  `application.log` 只有服务监听与停止等系统事件；code-server 可逐层点到含中文与空格的嵌套目录。
  改动前的 `logs/app-<日期>.log` 与 `logs/config/<日期>/` 已由用户永久删除，当前日志根下只有 `application/` 与 `plans/` 两棵目录树。
- F-013 代码审查两个 P1 问题已修复：后台全路径解析与一键配置 worker 原来用 `context.Background()` 起协程只传 planID，
  日志会掉进 `application` 目录，现在按任务的计划 ID 兜底并继承请求作用域，明细处理时再补上执行路径归属；
  `WriteBlock` 原来逐行加锁导致并发 `curl.log` 块互相穿插，现在整块在同一把锁内一次写入。
  顺带修掉 `StartGeneration` 解锁后复制任务结构体的数据竞争。三处修复均有用例并做过变异验证。
- F-013 复审两个 P2 已修复：后台 worker 与后台路径解析原来在持锁时解析日志作用域（计划名缺失会查库），
  数据库慢会连带阻塞取消、恢复与任务查询，现已把解析移到锁外，锁内只做任务状态判断；
  文档中"旧日志按要求未删除"的记录已更正为已由用户永久删除。
- F-013 已完成：新增 `internal/logging`（日志根、作用域注入、统一单行格式、有界写入器与行号、配置桶与运行目录路由、保留期清理）；
  目标请求日志在传输层单点接入，成功与失败分流到 `network.log` / `network-error.log`，可重放命令与完整响应写入 `curl.log`；
  API 中间件记录请求、失败响应的稳定错误码与界面同源中文提示，并恢复 panic 返回稳定中文 500；
  全局日志按天分文件（`app-<日期>.log`）并按保留期滚动删除，默认保留 7 天；
  日志查看沿用已批准的 code-server：`make logs-viewer` 起固定版本 `codercom/code-server:4.96.4`，
  19002 映射 8080，挂载本项目 `logs/` 到 `/home/coder/logs` 并直接打开该目录，无登录、可读写、
  以当前本机用户 UID/GID 运行；`make logs-viewer-stop` 只停止并删除该容器；`.gitignore` 已忽略 `/logs/`。
  本切片只给单容器启动方式，整套 Docker Compose 编排属于 F-023。
- F-013 实测：本机新构建接真实目标与真实数据库，`network.log` 出现真实请求行，`curl.log` 的命令直接重放成功，
  请求不存在的计划后 `program-error.log` 的 `user_message` 与接口响应体 `error.message` 完全一致。
- F-013 容器实测：`make logs-viewer` 起容器后，容器内实际列出并读到当天 `app-<日期>.log` 与 `config/<日期>/network.log`；容器内写入的探针文件宿主 `logs/` 立刻可见、删除后同步消失；19002 重定向到 `./?folder=/home/coder/logs`，code-server 读到的文件字节数与宿主完全一致；`make logs-viewer-stop` 后容器删除、端口释放。
  外接盘项目在 Colima 下需先把 `logs/` 挂进虚拟机（`~/.colima/default/colima.yaml` 的 `mounts` 加 `writable: true` 后 `colima restart`），否则容器只会看到一个空的可写目录；`make logs-viewer` 已内置挂载双向自检，挂载没生效时直接报错并停容器，不把 `docker inspect` 的 `RW=true` 当成挂载成功。
- F-012（上一切片）状态：`ready_for_manual`。已按用户要求改为运行时路由直接加载 `runtime-source` 宿主代码，本地只保留 postMessage 数据协议、数据回显和操作区隐藏适配；等待人工核对付款单位、请款金额、自定义组件、附件和选择弹窗。
- F-012 付款单位回显第五轮修复（2026-09-04）：定位到结构性根因——分支补丁只改名称字段，历史项目公司 ID 残留，且新公司不在用户子树的选项数据源里，运行时按名称回填 ID 结构上不可能命中。现由数据工作区在读写边界按目标公司主数据与项目公司并集同步真实 ID 与虚拟显示值，详见功能文档当日记录。阻塞：计划 8 的目标账号"骆梦恩"在目标用户中心不存在（只有"骆蒙恩"），后端登录持续失败使目标读取 503，真实页面人工验证需先修正计划账号或凭据。
- F-012 通用选项字段协调与身份替换（2026-09-04 下午）：按用户要求把修复泛化为"选项型字段补丁协调"（模板驱动，覆盖单/多选、级联、子表单，找不到或同名多条时阻断），并把目标提交约定的 `global_user_basic_information` 替换为当前计划账号身份；真实页面实测路径 580 三处一致（提示/显示/落盘均为临猗县斯能电力有限公司，ID 为项目公司真实 ID），阻塞已解除（目标服务恢复后计划 8 账号骆蒙恩登录正常）。
- F-012 单条路径配置三类问题修复（2026-09-04 晚）：统计剔除静态/联动隐藏容器内的必填字段（修复合同盖章评审表误报 22 项未填）；协调器把被补丁波及字段"远程选项重试后仍为空"升级为阻断并实时回传宿主面板（合同分类下拉空的目标测量中心服务故障由此如实可见）；数据工作区目标读取增加 3 次有界退避重试（缓解目标瞬断导致的"暂时无法读取路径配置"）。用户在改的 NodeConfigurationPanel.vue/ActionFlowDialog.vue 未纳入本轮提交。
- F-012 回显按实例绑定版本渲染（2026-09-04 深夜）：按用户明确的架构方向（渲染完全走统一工作台宿主页面，工具只做数据适配层），数据工作区在历史来源绑定时改用实例绑定的流程代理宿主配置作为 FormMaking 模板——模板改版导致的"历史数据键对不上当前模板"这类逐表映射问题从此在机制上消除；旧快照按候选键自动补齐实例标识，解析失败回落当前模板并提示。真实页面复验出差申请单整表按实例数据回显。
- F-012 登录人字段替换与关联流程附件隐藏（2026-09-05）：新增目标登录人字段约定登记表（handling*/handledBy、initiatorDepartmentId、currentDepartment/currentCompanyId/currentDepName、expense*、my* 选择器含伴生键），数据工作区读写边界统一替换为当前计划账号；prepareTemplate 隐藏 _flow 结尾的关联流程回传附件；load 命令超时放宽到 30 秒。真实页面复验合同盖章评审表：当前部门显示计划账号部门名，关联流程附件不再显示。
- F-012 角色名称读取失败修复（2026-09-05）：固定角色挂在创建平台码 999999 下而统一网关按 200001 查询导致按角色节点全部"角色名称读取失败"；目录解析改为按节点审批配置自带平台码查询并缓存隔离，新增 httptest 定向回归。
- 已完成：T01 至 T08 已按批准范围实施并完成中文原子提交；已验证 FormMaking 与 `vue_custom`/NoFormFlow 原始数据链路、分支窄补丁、真实动作目录、同一主实例场景编译、数据库重建、旧体系删除和前后端构建。
- 已完成复核修复轮：修复迁移执行器（服务可启动）、条件求值数字保真、动作门禁目录接入产品与实例动作编排、编译场景预览与拖拽排序、集成门禁真实执行；并修复复核中实测发现的 MySQL `JSON` 列改写目标数字和幂等重试按字节比较两个新缺陷。
- 已完成人工验收反馈修复：路径配置接口中的随机人员空选择由 `null` 收敛为按当前候选恢复的数组，前端保留旧响应防护；计划 2 的路径 13 已实测恢复节点配置画布。
- 已完成人工验收反馈修复：配置工作区读取与保存均清理目标 `auto_audit_info_*` 历史审批意见；路径关键字段从目标 FormMaking 报表布局提取真实可见中文标签，并兼容 `__virtualName` 虚拟字段。计划 2 的路径 13 已实测不再返回非空审批意见，“请假天数/请假类别”标签正确。
- 已完成人工验收反馈修复：恢复目标模板 `eventScript` 初始化与字段联动，只隔离提交/草稿钩子；FormMaking 模板中语义明确的查询 POST 进入只读清单，写请求与未知 POST 仍阻断；内存认证恢复 `invest-power-system-*` 键前缀和可遍历 Storage 行为。计划 4 路径 37 接口实测返回合同分类、用户、公司和字典查询 POST，F-012 全量验证与构建通过。
- 已完成人工验收反馈修复：直接挂载 FormMaking 时为业务自定义组件补齐与目标包装层一致的发起态 `extendProps`，空业务 ID 保证合同分类走可用分类查询且不读取目标业务详情覆盖历史数据；F-012 再次全量验证 19 个集成用例无跳过、前端 60 个用例通过。
- 已完成人工验收反馈修复：无表单/自定义表单补齐参考运行时页面注册和统一回显注入，缺失页面标识回落到通用业务数据页；宿主页面初始化后回填 `initForm/detail/rawData` 等同名模型，并读取页面 `getValues`；路径侧栏只显示中文字段标签，不再展示内部字段 ID。定向 Go/Node 测试 20 项、form-runtime 类型检查与生产构建通过，F-012 状态保持 `ready_for_manual`。
- 阻塞：无。F-013 先后遇到的两个本机阻塞已解除——Docker 数据盘由用户自行恢复（现 59G 已用 19G、可用 37G，全程未执行 `prune`，镜像、构建缓存与卷均保留）；Colima 未挂载外接盘项目目录的问题已通过在 `mounts` 中新增本项目 `logs/`（`writable: true`）并 `colima restart` 解决，仓库内配套增加挂载自检。MySQL 真实迁移需配置独立测试库（`PLAN_DB_*`），目标平台和浏览器行为保留人工验收边界。
- 待用户判断：迁移 `023` 把 15 个载荷列由 `JSON` 改为 `LONGTEXT`，属于授权范围内的模型变更；另有两处待用户裁决的差异见 F-012 功能文档状态记录。
- 下一步：F-016 评审修复轮已停在 `ready_for_manual`，等待用户人工验收；按用户 2026-09-05 裁决，后续切片连续实施、人工验收统一后置。F-018 至 F-023 计划已补齐并保持 `awaiting_approval`，依赖顺序为 F-017 → F-018 → F-019 → F-020 → F-021 → F-022 → F-023；未获批准不得进入 `implementing`，不得跨切片实施。
- F-016 执行器最小真实闭环：已按用户要求产出计划文档 `docs/features/F-016-executor-minimal-real-loop.md`，
  状态 `awaiting_approval`，未获明确批准前不进入 `implementing`。这是本项目第一次真实写。
  范围：运行与路径运行状态机、一步七阶段（plan/gate/control/prepare/submit/verify/settle）、
  强制单步且每步用户放行、一次尝试最多一次写请求且 submit 内部零重试、写结果不确定即停在 `待对账` 且界面不给重试入口、
  场景走完后的收尾重读并把「路径结果」与「最终目标事实」分开表述、迁移 `026` 与六张运行记录表（事实表只 INSERT）、
  `step.log` 与记录日志双向可达、
  最小启动与路径运行详情页。
  写端点白名单只有两个：`/web/flowInstanceApi/submit` 与 `/flowInstanceApi/audit`；其余九个写端点仍由运行准备阻塞。
  同时按纲领第 4.4.1 节补齐两项基础缺口：传输层区分「连接阶段未完成」与「请求已发出但响应丢失」以喂给判定包，
  以及目标网关显式绕过本机代理（实测本机代理会截走内网目标请求并返回空正文 502）。
  承接 F-015 的缺口：跑通哪个动作才登记哪个（`verifiedRunnableActions` 从空集开始），
  为登记的动作建立到语义清单条目的映射；`person_not_unique` 依赖 F-012 正在返工的人员模块，实施前需协调。
  只在用户指定的可污染真实测试账号与流程上写，账号值不入仓库。
  运行调试页按用户明确的形态定为运行画布：真实流程节点为载体，按纲领已定义的九个中文节点运行态高亮，
  当前步有持续可见指示器并自动平移到操作区中央，点击节点在固定侧栏看该节点的运行信息与错误；
  另按用户要求补齐节点耗时（含七阶段耗时）、侧栏完整请求与响应正文加一键复制可重放 curl、运行结束整图结果总览着色，
  以及一节可检查的交互质量要求（放行不绑单键快捷键、自动跟随可被接管并有「回到当前步」、
  等待中显示阶段名与重试退避剩余时间、不整页 loading、轮询不引入 WebSocket）。
  画布复用 `web/src/features/flow-graph` 现有组件新增 `runMode` 变体，不建第二套；
  用户 2026-09-04 裁决运行画布留在 F-016（不挪到 F-017），并要求每一步执行都有流畅的进行中指示器：
  按七阶段分段推进、三处读数同源、动画由本地时钟连续驱动不被轮询带跳、耗时单调不减、重试显示第几次与下次退避剩余时间、
  submit 阶段明确显示写请求已发出且停止只能在本步结束后生效、结束时收敛为最终结果不留孤儿动画、超预算无更新转为疑似无响应；
  执行中不加整页遮罩，进度由当前步节点上的指示器承载。
  断点管理、执行到下一节点、事件流时间线与 F-021 完整分析视图仍不在本切片。
- F-017 调试器（运行模式、断点与运行控制）：已按用户要求结合现状产出计划文档
  `docs/features/F-017-debugger-modes-breakpoints-control.md`，状态 `awaiting_approval`。
  它没有独立价值，全部依赖 F-016 的状态机、七阶段、运行记录与运行画布，而 F-016 于 2026-09-04 刚获批准进入 `implementing`、代码尚未落地
  （`internal/engine` 下只有 actioncatalog/scenario/verdict，迁移最新 `025`，`RunsView.vue` 仍是占位页），
  因此计划里把第一个任务定为 T00「按 F-016 实际实现校准本文件」，并写明两条硬约束：
  F-016 未获用户明确验收前 F-017 不进入 `implementing`；F-016 落地后若接口、表列或前端目录与计划假设不一致，先改文件再写代码。
  范围：三种运行模式（自动/单步/人工控制，默认单步，运行中不改模式）、五类断点（首次写默认开、路径偏离强制开不可关、
  步骤/节点/动作默认关）、断点命中统一在阶段 3 判定并按固定优先级取主因、路径偏离在 `verify` 认定事实并在下一步阶段 3 强制停下、
  三条命令（执行一步/执行到下一节点/继续运行）且可用命令集合由后端给出、暂停与停止只在阶段 3 生效且 submit 后必须走完
  verify 与 settle、控制命令条件写且幂等（重复放行只前进一步）、`run_controls` 九类控制事实 append-only 且生效断点集合由事实回放得出、
  `control.log` 写入器、迁移 `027`（若 F-016 已预留列则不新增并说明原因）、断点与控制界面落在 F-016 运行画布上不新建页面。
  写端点白名单不扩张，仍只有 `/web/flowInstanceApi/submit` 与 `/flowInstanceApi/audit`；自动运行下的连续真实写由首次写断点作为安全阀。
  承接的产品决定：成功断言已整体移除，「跑到哪里算成功」改由本切片的断点表达，所以它是用户表达期望的唯一手段而不是体验增强。
  已知边界如实记录：动作断点当前只能在 F-016 登记的发起与同意上真正命中，其余动作等 F-019；`person_not_unique` 仍依赖 F-012 返工中的人员模块，本切片不碰。
  三项待用户裁决的取舍写在计划的「未决与需要用户裁决的事项」：单步不提供执行到下一节点与继续运行、路径偏离停止后不提供放行、自动运行继续用轮询不引入 WebSocket。
  对账与安全重试（F-018）、动作全集（F-019）、多路径与定时（F-020）、运行记录分析视图（F-021）仍不在本切片。
- 2026-09-05 规划补齐：复核发现 F-018 原计划只有目标、范围和设计要点，缺少详细任务、完成标准、状态记录、测试分类与人工验收，已补成完整 `awaiting_approval` 计划。依据同一纲领继续补齐 F-019 全动作与恢复步骤执行、F-020 多路径调度与单次定时启动、F-021 运行记录分析视图、F-022 目标语义一致性套件、F-023 Docker Compose 发布编排；五份文档均保留独立门禁，不进入实施，不改变当前运行代码。
- F-015 计划产出与后续收敛的完整过程见 `docs/features/F-015-success-assertion-run-readiness.md` 的状态记录：
  初版计划包含成功断言，成功断言随后按用户决定整体移除，当前范围只有运行前检查。
- 2026-09-04 参考仓库锚定分支调整：按用户裁决把 `rsh-cloud-workflow-center` 由 `test` 改为 `master`。
  `scripts/reference-repositories.tsv` 已改；旧克隆经洁净核对（无改动、无未跟踪文件、无 stash、未领先远端）后移除，
  由 `make refs-sync` 按 `master` 干净克隆，未使用 reset 或 checkout。
  切换时 `master` `37c01d04` 相对原 `test` `0c6c7f0e` 只多一个把 test 合入 master 的合并提交、内容差异为空，
  既有语义证据不受影响，已在新 HEAD 上逐条复核通过。
  `make refs-sync` 13/13、`make refs-status` 13/13 全部「正常」；`rsh-flow-components` 按裁决保持 `test` `24f3a280`。
- 2026-09-04 用户裁决：`GroupApproveManage/CaseManage/index.vue`「我的案件」操作列不渲染是明确需求，不是缺陷。
  不恢复 `operation` 列，不作为 F-012 阻断项，也不为此修改表单渲染或补测试。
- 2026-09-04 参考仓库同步：按用户要求执行 `make refs-sync`（13/13 快进）。`rsh-cloud-workflow-center` `test` 到 `0c6c7f0e`、
  `rsh-cloud-workflow-center-api` `master` 到 `088aed79`、`rsh-framework-all` `test` 到 `84bb1973`。
  `rsh-flow-components` 规范分支维持 `test`（`24f3a280`）；不改用另一条语义基线 `master`。旧 `master` 工作树已移入
  `参考代码/.branch-backups/rsh-flow-components-master-3ffb41ba`，当前运行快照仍保留历史 `master` 基线，后续维护任务才按清单生成候选同步。
  目标平台上了「审批方式动态化」：`auditWay` 不再受 `AuditWayEnum` 约束，提交校验路由改为字符串键并支持动态注册，
  未注册或无健康实例时前置门禁静默放行，未注册 `auditWay` 由抛异常改为返回 `未发现实例` 失败包；
  另有 `flow_trigger_config_relevance.audit_way` 由数字 ordinal 迁移为 `VARCHAR(100)` 的脚本（目标环境是否已执行需实测）。
  写路径（`submit`/`audit`/`reSubmit`）未改动，F-014 方向不变，已按此调整其任务与完成标准。
  连带影响记入其它切片、本次不改动：待办列表不再带实例运行态字段、终态实例不再返回当前处理人、
  流程图详情改走无 Redis 快速查询链路需只读回归、`auditWay` 迁移前工具读到的可能仍是数字而它被用作 `vue_custom` 页面键。
- 已产出后续工作纲领 `docs/EXECUTION_PROGRAM.md`（执行器、调试器、运行记录与分层日志，F-013 至 F-023），只界定边界与顺序，不构成实施授权。F-029 按用户裁决恢复内网测试日志原样记录，SID、密码和令牌不经公开 API，日志文件不进 Git；code-server 随 Docker 交付、发布用 Docker Compose、真实写使用已指定测试账号（账号值不入仓库）；`docs/ARCHITECTURE.md` 的日志条文已同步。

## 当前决策摘要

- 数据统一支持真实 FormMaking 和 `vue_custom`/NoFormFlow 自定义表单；历史候选只按目标接口原字段匹配 `flowCode` 与表单/流程名称，所有状态可选，已完成优先，无候选只提示先在目标平台发起一次并填写业务数据。
- 目标原始表单数据深复制后直接交给复制的 `form-runtime` 完整回显；只对当前路径条件字段做可证明的最小补丁，条件字段可编辑，人工改值导致换路时必须确认；数据读取、runtime 接收或路径复验失败进入 `needs_input`，无生成器和兜底。
- 条件语义严格复刻目标 Java：`lt/gt/lte/gte/eq/neq/contains/is_update/is_not_null/boolean_value`、BigDecimal、B 包含 A、顺序 `and/or`、首个命中、最后分支兜底；目标布尔矛盾不自行修正。
- 动作覆盖目标真实用户操作；保存草稿是实例生命周期，审批暂存是任务检查点；加签只在当前活动人工待办且代理可编辑时可用，系统自动节点不提供动作。
- 用户动作按独立记录排序并编译为同一主实例的用户步骤、恢复步骤和导航步骤；不可恢复顺序保存时阻止，不用快照恢复或新建主实例。
- 清空本工具数据库全部业务数据，不保留备份；保留 `schema_migrations` 和运行时维护能力；绝不触碰目标平台数据。

详细范围、任务顺序、接口、表结构、验证和人工验收见 `docs/features/F-012-history-form-replay-action-orchestration.md`。

- 2026-09-19 夜间真实浏览器流程测试完成（任务书 docs/auto/2026-09-19-zcode-overnight-real-browser-flow-test.md，报告同目录 -report.md）：主测流程「员工请假单（智慧斯能）」12 条自动解析路径，A/B/C/D 四组合共 5 次运行、11 条路径全部真实浏览器操作执行，手动串行两路径全程成功（34/35 步）、自动串行与自动并发各 3 路径全部成功、手动并发 3 路径全部成功。夜间发现并修复 3 项工具问题（提交 56b42e3 登录协议适配 getVerifiCode flag→sign；一键配置排除加签；排除取回），并实证目标平台同账号会话互踢与密码过期弹窗。加签专项四方对照通过（目标流程图显示插入的「审核人/陈涛」节点）。产品缺口待用户裁决：加签后编译场景需自动补「插入节点同意」步骤；画布动态投影插入感知待修。F-037 保持 ready_for_manual。跨流程调休单测试发现 isPermissionCheckSubmit 裸子串误判真实草稿写（已修，改 data 顶层键级判定，提交见 git log）与 auto_audit_info 重提基线缺口（安全门禁拦截，待独立切片）。
- 2026-10-28 F-053 合同业务故障修复：线上 `oyg测试10` 第 16 步的 `contractReview/modify` 明确返回 `ERROR_99999`，旧执行器把它误归为主流程写缺失。当前已补齐合同审批初始化时序、当前状态读取、合同与附件写入串行、业务写失败/响应不明阻断和运行详情错误投影；执行器缺少合同自定义业务组件时不再回退普通审批。发布同步新增项目内受控覆盖，覆盖上下文漂移、LF/CRLF 和摘要失效均有回归门禁。`6cf2e25568ce562d38d33719d9dd3f783df2502e` 已发布到 `192.168.2.200`，两侧来源与候选摘要核对一致，六服务验活通过；当前停在 `ready_for_manual`，等待用户页面验收。
