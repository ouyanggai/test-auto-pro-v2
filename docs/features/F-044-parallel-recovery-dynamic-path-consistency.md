# F-044 并行分支恢复与动态加签路径一致性补缺

- 状态：ready_for_manual（2026-09-28 运行 9 反馈修复与自动验证完成，等待用户重新运行同一路径验收）
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
