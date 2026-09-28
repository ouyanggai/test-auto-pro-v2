# F-044 并行分支恢复与动态加签路径一致性补缺

- 状态：ready_for_manual（2026-09-28 运行 13 路径 5 动态加签后表单人员解析修复与自动验证完成，等待用户重新运行验收）
- 依据：F-039 夜间审查修复、F-037 运行画布与加签重绑定、运行 7 路径 1/2/12 的失败证据。

## 单一用户结果

用户配置包含并行网关的路径时，各分支按真实图关系独立推进；分支内回退只回到本分支真实前驱，不把兄弟分支或分叉前主线误编进恢复链。不同意、撤回和回退仍沿主实例恢复链继续，不创建第二主实例。加签重建代理树后，动态节点和原路径的结构映射使用同一套真实边关系，合法的插入节点/并行网关状态不再误报路径不一致；无法证明时停在查看/停止，不猜测放行。

## 本切片范围

包含：

1. 场景编译输入携带当前路径真实图边，并将边转换到稳定语义节点键。
2. 回退恢复链按真实入边寻找人工前驱，沿唯一可证明的本支后继回到原节点；并行兄弟不再作为线性前驱。
3. 回退/不同意/撤回恢复链的分支导航与动作顺序回归测试，保证仍复用同一主实例。
4. 加签后动态代理树投影与路径一致性测试，覆盖并行网关返回当前节点、插入同类型节点和合法反向恢复。
5. 运行详情重新编译场景时使用同一套图边，避免保存预览与实际运行场景各自猜路径。

不包含：

- 不修改目标平台源码、权限、流程配置或目标业务规则。
- 不新增数据库表、迁移、旧接口兼容层或第二套执行器。
- 不把目标平台待办异步、取回后 pending 缺失等尚未有工具侧证据的问题伪装成已修复。
- 不替用户执行浏览器或真实目标写入；自动验证完成后停在 `ready_for_manual`。

## 完成标准

- [x] 场景编译器接收路径边并建立稳定的入边/出边索引；无边旧数据保持原有线性行为。
- [x] 并行分支回退恢复链只包含本分支前驱和本分支中间节点，不包含兄弟分支节点。
- [x] 本分支已有人工前驱时回退回该前驱；没有可证明前驱时保存/运行前阻塞而不是猜测。
- [x] 不同意、撤回恢复链仍从主线起点重提并按真实路径重走，不创建第二主实例。
- [x] 加签后的并行网关当前节点和锚点映射通过合法结构校验；无关节点、重复映射和缺锚点继续阻塞。
- [x] 新增场景编译、代理路径和动态投影回归测试，相关 Go 测试、vet、构建和差异检查通过。
- [x] 文档状态进入 `ready_for_manual`，提供第二轮手工运行顺序；不标记 `accepted`。

## 自动验证

2026-09-28 运行 8 反馈：路径 1/4/6/10 在实例私有代理换 ID 后仍按旧节点 ID 查处理人；路径 2 把未激活的并行兄弟分支误套模板 `isSkip`；路径 3 把目标清洗的人员选择器伴生字段误判为正文丢失；路径 9 把同一恢复组的后续审批节点排除在合法前进锚点外。本轮回到 `implementing` 修复并重新验证，完成后再次停在 `ready_for_manual`。

2026-09-28 修复完成：`currentAuditUserInfo` 保留节点名称，编译期节点 ID 失效时只允许唯一同名动态节点重绑定；门禁命中的实时节点 ID 贯穿 Go 写前任务刷新、runtime `nodeProxyId` 和写后核验。未激活的并行兄弟分支按真实图关系零写入跳过；人员选择器主字段仍存在时允许目标清洗伴生字段；同一恢复触发组的后续恢复同意纳入合法前进锚点。新增预览、Go 写前执行、runtime 请求、并行跳过、正文保护和恢复链回归测试。

2026-09-28 运行 9 人工验收未通过，按用户反馈由 `ready_for_manual` 返回 `implementing`：路径 1/3/4/6/9/10 的活动并行待办被最小步骤序号掩盖；路径 2 的较短支线先入队，使“公司顾问”排到裴国英所在合规支线末端之前；路径 5 的加签插入节点导致对齐递归错一层，重排后的并行分支没有对齐，且重绑失败只停止放行、未持久收尾。

本轮修复后重新停在 `ready_for_manual`：可达图按真实入边稳定拓扑排序，所有并行支线先于共同后继；门禁逐一核对本节点及活动上游待办，汇合前阻塞且不发写请求；加签从插入点以下再次对齐分支并按结构路径重绑；无法重绑时在确认写成功后落 `stopped` 终态并退出循环。新增较短/嵌套支线、多个同时活动待办、插入后并行重排回归测试。

运行 9 路径 5 的既有“运行中”记录没有被本次代码修改回写：本机未运行后端，且本进程缺少计划库连接配置，不能安全操作旧记录；服务正常运行后应通过该路径的停止入口关闭。未连接目标平台、未发送真实写请求、未启动浏览器，所有路径的真实通过仍待用户重跑确认。

2026-09-28 15:19 运行 11 人工验收未通过，按用户反馈从 `ready_for_manual` 返回 `implementing`：除第 11 条外多条路径启动时被“动作回到前序节点但没有回退或取回恢复步骤”阻断；第 11 条已进入并行段，但单游标显示和仅重走目标支线的恢复链遗漏了兄弟分支。用户明确要求所有并行分支同时为活动待办、全部完成才进入共同后继；不同意、撤回以及分支首节点退回分叉前主线后已完成的兄弟分支也要重审。

本轮修复并重新停在 `ready_for_manual`：旧广度序动作在当前真实图上按拓扑节点顺序归一，保留同节点动作顺序；手工按节点保存和一键配置候选共用该编译规则，无法编译的结构问题立即提示。运行前检查重新编译当前动作与图，结构错误在创建路径运行前提示。不同意、撤回或退回分叉前时重走此前已完成的所有相关支线，分支内部回退不重做兄弟支线；兄弟分支单独活动不再被误认作本支已经跳过。运行详情把目标当前全部待办投影为活动节点，同时仍以逐步写入、逐步核验保持同一主实例的安全边界；画布同时高亮不意味着对同一实例无序并发发送写请求。共同后继由拓扑顺序及全部活动上游待办门禁共同约束。新增旧配置重编译、恢复重审、多待办高亮和并行跳过判据回归。

修正后对真实计划「欧阳改测试2」（plan 67）的只读运行预检：路径 1-6、9、10 均为 runnable；路径 7、8 不再有动作结构阻断，但历史数据路由值不匹配且嵌套分支尚未选择，仍为不可运行；路径 11 以后尚未配置。该结论仅为配置和当前图的只读预检，不能代替目标审批执行的人工验收。

2026-09-28 16:13 运行 12 路径 5 人工验收未通过：`updateFlowProxy` 已返回成功，写后实例事实的当前节点为 `1c79b06167f74a059c9526e6c89aad13`，且该节点属于新代理树；但接口响应的 `currentNodeProxyId` 为 `c7fe70ec16cf4cdfb5cb5dc8c8e3642c`，控制层仍拿这个接口游标与加签锚点映射节点硬比较，误报“节点结构无法唯一重绑定”并停止。日志同时证明加签业务结果已确认成功，后续节点未执行。

本轮修复后重新停在 `ready_for_manual`：加签写后以实例事实重读得到的完整 `CurrentNodes` 作为代理重绑定后的权威活动节点集合，并验证集合中的每个节点都属于新代理树；接口响应的单一 `currentNodeProxyId` 只保留为诊断字段，不再覆盖或否决合法实例事实。非加签的代理重建仍保留原有当前节点校验；新代理树、活动节点集合或结构映射确实无法证明时继续停止。新增并行加签写后节点事实回归测试。

本轮验证：

```text
go test ./internal/engine/step ./internal/engine/control ./test/unit/backend/executor ./test/unit/backend/control
go vet ./internal/engine/step ./internal/engine/control ./test/unit/backend/executor ./test/unit/backend/control
go build ./...
```

上述检查通过；后端 `19080` 健康且已热重载，未自动重跑目标平台写请求。下一步由用户重新运行路径 5，确认加签成功后能继续到插入审批节点和后续节点；若新代理树或写后活动节点确实不在树中，仍应安全停机。

本轮验证：

```text
go test ./internal/analyzer ./internal/engine/scenario ./internal/engine/step ./internal/engine/control ./internal/service ./internal/adapter/target ./test/unit/backend/action_orchestration ./test/unit/backend/executor ./test/unit/backend/run_orchestration ./test/unit/backend/run_readiness
node test/unit/frontend/flow_graph_test.mjs
pnpm --dir web run typecheck
pnpm --dir web run build
```

上述检查通过；未连接目标平台、未发送真实写请求、未启动浏览器，因此不能代替用户对本次全部路径的真实运行验收。

2026-09-28 运行 13 路径 5 人工验收未通过，按用户反馈由 `ready_for_manual` 返回 `implementing`：动态加签插入节点同意后，下一节点「财资部-会计」为 `form_person`，目标拒绝“表单人员数据获取失败”。日志确认实例当前表单仍保留 `accountantUserName` 的真实人员 JSON、`__formPersonId` 与 `__condition`，失败发生在审批页将表单人员字段组装为目标请求时。

本轮修复后重新停在 `ready_for_manual`：审批页同时兼容 `form_person` 声明伴生键和直接声明主字段；只要已有值是人员 JSON 就转换为真实用户 ID，空值才回退读取主字段，不替换真实人员、不扩大动态加签节点编辑权限。Go 目标适配层同步支持直接声明主字段，新增回归测试；后端定向测试、`go vet`、runtime Vue 生产构建和 `git diff --check` 通过。完整 F-035 合同生命周期套件仍有一个与本轮无关的既有失败（未实现合同盖章必须阻塞），未连接目标平台、未发送新的真实写请求、未启动浏览器。

本轮人工检查：重新运行同一计划，确认不再在调度启动时报前序回跳；进入并行段时所有当前待办一起高亮、短支线先完成后共同后继仍等待；不同意、撤回、首节点退回分叉前再继续时各已完成支线重新审核，分支内回退仅重走本支。若目标事实只返回一个兄弟分支而本支是否完成未知，工具应阻断并说明无法证明，不应静默跳过。

本轮实际验证：

```text
go test ./test/unit/backend ./test/unit/backend/executor ./test/unit/backend/control ./test/unit/backend/action_orchestration ./test/unit/backend/target ./test/unit/backend/run_orchestration
go test ./internal/analyzer ./internal/engine/scenario ./internal/engine/step ./internal/engine/control ./internal/service ./internal/adapter/target
go vet ./internal/analyzer ./internal/engine/scenario ./internal/engine/step ./internal/engine/control ./internal/service ./internal/adapter/target
go build ./...
git diff --check
```

上述检查通过；依赖计划库的 `TestF016StopControl` 因缺少 `PLAN_DB_HOST/USER/PASSWORD` 跳过，不能作为本轮终态落库的实测证据。

本轮自动验证结果：

```text
go test ./internal/engine/scenario ./internal/engine/step ./internal/service
go test ./test/unit/backend/action_orchestration ./test/unit/backend/executor ./test/unit/backend/target
go vet ./internal/engine/scenario ./internal/engine/step ./internal/service
go build ./...
git diff --check
```

以上检查全部通过；未连接目标平台、未发送真实写请求、未启动浏览器。

只运行当前切片相关检查：

```text
go test ./internal/engine/scenario ./internal/engine/step ./internal/service
go test ./test/unit/backend/action_orchestration ./test/unit/backend/executor ./test/unit/backend/target
go vet ./internal/engine/scenario ./internal/engine/step ./internal/service
go build ./...
git diff --check
```

## 手工验收顺序

1. 使用第二轮测试中的并行流程，先执行分支 A，再执行分支 B，确认两条分支都完成后才进入共同后继节点。
2. 在分支 B 内执行回退，确认恢复步骤只回到 B 分支前驱，不出现 A 分支节点。
3. 在并行分支执行不同意或撤回，确认恢复从主线重新提交并继续原主实例，不新增实例。
4. 在并行节点执行加签，刷新运行画布，确认插入节点只出现一次，实际处理人来自新待办，完成后能回到原路径后继。
5. 若目标返回的当前节点、代理树或边无法唯一映射，确认界面只保留查看/停止，不提供继续放行。
