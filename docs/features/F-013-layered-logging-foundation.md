# F-013 分层日志与追踪底座

- 状态：ready_for_manual
- 产品依据：`docs/PRODUCT.md` 的产品原则第 2 条（工具问题与目标平台问题必须分开说明）与「明确不做」中的“独立技术状态与 JSON 配置页面”
- 架构依据：`docs/ARCHITECTURE.md` 的包边界与目标适配层条文（已按内网裁决同步日志条文）
- 纲领依据：`docs/EXECUTION_PROGRAM.md` 第 6 节全部，第 9 节 F-013 行
- 计划确认时间：2026-09-04（用户在 F-012 之后明确要求开始实施 F-013）
- 前置条件：F-012 已由用户明确验收

## 目标

让本工具的每一次目标平台请求和每一次程序错误都留下可定位、可关联、可重放的记录，并且用户能用已经认可的 code-server 方式直接打开这些记录。

本切片不执行任何目标写操作，也不创建运行记录。它是后续执行器与调试器全部切片的验收依据：先有观测，再有执行。

## 为什么先做这一项

- 当前后端只有 `cmd/server/main.go` 里的几处 `log.Printf`，`internal/` 下没有任何日志。配置阶段出问题只能靠复现和猜。
- F-012 遗留的两个集成用例失败（`TestFormRuntimeFixedSourceSnapshot`、`TestPathConfigurationSnapshotReadsTemplateDefaultsAndProxyValues`）正是这种情况：没有请求日志就看不到目标当时返回了什么。本切片交付后这类问题可直接查。
- 后续每个切片的人工验收都要求“能看到证据”。证据设施必须先存在。

## 单一用户结果

用户在浏览器里做完一组配置操作后，打开 code-server 就能看到：这段时间工具向目标平台发了哪些请求、每个请求的完整请求与响应、失败的请求单独成文件、可以直接复制重放的 curl 命令，以及工具自身在同一时刻报的错和界面上看到的那句中文提示是同一件事。

## 范围

### 包含

- 新增 `internal/logging` 包：日志根解析、日志作用域、统一单行格式、写入器（返回写入行号）、容量轮转、按日期清理。
- 日志根下只有两棵树：`logs/application/` 放系统级事件，`logs/plans/` 放全部能归属到计划的业务日志。
- 应用程序日志 `logs/application/<YYYY-MM-DD>/application.log` 与 `application-error.log`：只放启动停止、
  服务监听、保留期清理这类系统事件和无法归属业务对象的基础设施异常，不作为业务日志的汇总文件。
- 配置阶段业务日志 `logs/plans/<计划显示名>__plan-<计划ID>/configuration/<执行路径显示名>__path-<路径ID>/<YYYY-MM-DD>/`，
  内含 `meta.json`、`operation.log`、`operation-error.log`、`network.log`、`network-error.log`、`curl.log`。
- 执行阶段业务日志 `logs/plans/<计划显示名>__plan-<计划ID>/runs/<执行路径显示名>__path-<路径ID>/<运行号>/`，
  内含 `meta.json`、`execution.log`、`execution-error.log` 与同样三个网络日志文件；本切片只实现路由，不创建运行记录。
- 已知计划但还不知道执行路径的计划级操作进 `configuration/_plan/<YYYY-MM-DD>/`。
- 中间件从路由取计划与执行路径的不可变 ID，显示名由 `service.LogScopeService` 从真实业务记录读取并完成归属校验，
  每次请求只解析一次；作用域随 `context` 传给目标站点客户端，网络日志与可重放命令因此落在同一个计划目录。
- 目标请求日志在 `internal/adapter/target` 的 `Client.call` 单点接入，覆盖当前全部只读请求。
- `curl.log` 写入可直接复制重放的完整命令，含真实会话值与完整请求响应正文（内网裁决，见 `docs/EXECUTION_PROGRAM.md` 第 6.5 节）。
- API 中间件：请求日志、失败响应日志（记录实际返回给用户的稳定错误码与中文文案）、panic 恢复并落程序错误日志。
- 程序错误日志字段：`error_class`、`error_chain`、`source`、`stack`（仅 panic）、`run_terminated`、`user_message`。
- `make logs-viewer`：一条命令起固定版本 code-server（`codercom/code-server:4.96.4`），本机 19002 映射容器 8080，挂载本项目 `logs/` 到 `/home/coder/logs` 并直接打开该目录，内网使用不设登录，挂载目录可读写，容器以当前本机用户 UID/GID 运行。`make logs-viewer-stop` 只停止并删除该容器。
- `make logs-viewer` 启动后做一次挂载双向自检：宿主写入的探针容器必须能读到，容器写入的内容宿主也必须能读到；任一方向不通就停掉容器并明确报错，不允许把一个空的可写目录当成挂载成功。
- `.gitignore` 增加 `/logs/`。

### 不包含

- 任何目标写请求。本切片的写端点白名单为空，测试断言实际发出的写请求集合为空集（延续 F-012 的零写入断言，改为白名单形式）。
- `step.log`、`control.log`、`recovery.log` 三个文件的写入器。它们分别属于 F-016、F-017、F-018，本切片不预先实现。
- 运行记录表、运行相关 API、`RunsView.vue` 的任何改动。
- Docker Compose 整套编排。属于 F-023，本切片只给单个 code-server 容器启动方式。
- 系统设置页里的日志配置项。保留期与容量本切片只用环境变量控制，`docs/EXECUTION_PROGRAM.md` 第 6.6 节提到的“可在系统设置里调整”留到有实际需要时再单独立项。
- 脱敏过滤器、日志级别开关矩阵、正文摘要化。用户已明确不做。
- Loki、Promtail、Grafana 一类外部可观测栈。

## 设计要点

### 日志根与目录

- 日志根为 `<workspaceRoot>/logs`。`workspaceRoot` 复用 `cmd/server/main.go` 已解析的值，不重新搜索仓库标记（V1 的 `findRepositoryRoot` 在 V2 没必要）。
- 可用 `TEST_AUTO_PRO_LOG_ROOT` 覆盖，供测试使用临时目录。
- 目录分段全部经过路径清洗，不接受目标返回的原始名称直接落盘。

### 统一行格式

单行 `key=value`，空格分隔，值内空格替换为下划线，空值写 `-`。前两列固定：

```
time=2026-09-03 18:56:31 level=error ...
```

关联键：`request_id` 必填；`run_id`、`path_run_id`、`step_id`、`attempt`、`phase` 在配置阶段写 `-`，由 `context.Context` 携带的作用域自动注入，不靠调用方逐处传参；`trace_id`、`curl_trace_id` 由请求日志生成。

多行内容只允许出现在 `curl.log` 和程序错误日志的 `stack`，必须用块包裹：

```
--- begin curl trace_id=<id> ---
...
--- end curl trace_id=<id> ---
```

### 写入器约束

- 写入返回该记录所在的行号。后续切片要把行号存进运行记录，用来做日志深链，所以签名现在就定下来。
- 单文件上限默认 8 MiB，超出轮转为 `.1`，最多保留 3 个。沿用 `formruntimemaintenance` 已验证的有界日志思路。
- 同一文件的并发写入必须串行，不允许出现半行交错。
- 写日志失败绝不影响主流程，只降级为一次标准错误输出。

### 错误分类

`error_class` 取值固定为八类：`tool_bug`、`tool_config`、`tool_storage`、`tool_dependency`、`target_contract`、`target_runtime`、`network`、`unknown`。

产品原则要求工具问题与目标平台问题分开说明，暂时无法判断时按工具问题处理，因此 `unknown` 必须同时写 `level=error` 并在说明里标注按工具问题跟进。目标侧四类错误直接由 `internal/adapter/target` 现有的 `ErrorLoginRejected`、`ErrorSessionExpired`、`ErrorResponseInvalid`、`ErrorTimeout`、`ErrorUnavailable` 映射，不新造判断。

### 界面与日志同源

失败响应统一经过 `internal/api` 的 `writeFailure`，中间件捕获实际写出的状态码、稳定错误码和中文文案，把同一句话写进 `user_message`。这样用户报“页面提示 XXX”时可以直接 grep 到那一行。

不改 `writeFailure` 的签名，79 个调用点保持不动。

### 接入点

只有三处，这是本切片能小的原因：

| 位置 | 改动 |
| --- | --- |
| `internal/adapter/target/client.go` 的 `Client.call` | 目标请求的唯一出口，接入网络日志与 curl 日志 |
| `internal/api` 新增一个中间件 | 请求日志、失败日志、panic 恢复 |
| `cmd/server/main.go` | 构造日志器、启动清理循环、包装现有 handler |

不新增 `NewHandlerWith...` 构造函数。现有构造链已经有 11 个重载，本切片在 `main.go` 里包装返回的 handler，不再加长这条链。

## 详细执行任务

### T01：日志包骨架与写入器

新增 `internal/logging`：日志根解析、`Scope` 与 `context` 注入（合并语义）、统一行格式化、值清洗、写入器（返回行号、有界轮转、并发串行）、目录路由（应用程序、配置阶段、执行阶段三个分支）与 `meta.json` 归属说明。

完成判据：单元测试覆盖行号返回、轮转、并发无交错、目录段清洗（保留中文括号横线空格、拦下路径穿越）、空值占位、作用域合并不丢 `RequestID`、三个目录分支的路由结果，以及同名不同 ID 的计划与执行路径不共用目录。

### T02：程序日志与程序错误日志

按作用域选择落点：配置阶段写 `operation.log` 与 `operation-error.log`，执行阶段写 `execution.log` 与 `execution-error.log`，无业务归属时才写 `application.log` 与 `application-error.log`，同一条日志只落一处目录。实现 `error_class` 映射、错误链展开、`source` 定位、panic 栈有界截断、`run_terminated` 与 `user_message`。

完成判据：单元测试覆盖八类分类与错误链展开；panic 用例产生带 `stack` 块的记录且截断生效。

### T03：目标请求日志与可重放 curl

在 `Client.call` 单点记录：`time`、`level`、`duration_s`、`request_id`、`trace_id`、`curl_trace_id`、`method`、`endpoint`、`request_class`、`status_code`、`result`、`outcome_kind`、`error_type`、目标实例与任务标识（原样）。成功与运行提示进 `network.log`，失败与最终错误进 `network-error.log`，请求块进 `curl.log`。

完成判据：集成测试用不可达的目标地址触发失败，断言两个文件各出现对应记录且 `trace_id` 与 `curl_trace_id` 双向可查；断言 `curl.log` 里的命令与实际发出的请求逐字一致（含会话值），可直接重放。

### T04：API 中间件与组装

新增请求中间件：生成 `request_id`、记录请求与响应、捕获失败响应的稳定错误码与中文文案、recover panic 并返回稳定的中文 500 响应。在 `cmd/server/main.go` 包装现有 handler，与既有 `gzipResponses` 组合。

完成判据：集成测试断言一次失败请求在该计划目录 `operation-error.log` 里的 `user_message` 与 API 响应体的 `error.message` 完全一致，且这条业务异常不出现在 `application-error.log`；panic 处理器返回 500 且不泄漏栈到响应体，栈只进日志；分别访问两个计划与执行路径的配置接口后日志不串目录。

### T05：保留期清理与容量轮转

`logs/application/<日期>/` 与 `logs/plans/<计划>/configuration/<执行路径>/<日期>/` 按目录名日期清理，`logs/plans/<计划>/runs/<执行路径>/<运行号>/` 按最后修改时间清理，并收掉因此空掉的父目录，默认保留 7 天，`TEST_AUTO_PRO_LOG_RETENTION_DAYS` 可覆盖。清理只删过期目录，绝不触碰数据库运行事实，也不删当天目录。`.gitignore` 增加 `/logs/`。

完成判据：单元测试构造过期与当天目录，断言只删过期项；`git status` 在产生日志后保持干净。

### T06：code-server 启动方式与全范围验证

新增 `make logs-viewer`（固定镜像 `codercom/code-server:4.96.4`、挂载 `logs/`、端口 19002、无登录、可读写、以当前本机用户 UID/GID 运行）与 `make logs-viewer-stop`（只停止并删除该容器）。Docker 未启动时两个目标都直接报错退出，不提供任何替代方案。容器内对挂载目录的读写权限需实测确认。启动后追加一次挂载双向自检，因为 `docker inspect` 里的 `RW=true` 只说明声明了可写，并不证明宿主目录真的映射进容器：Colima、Rancher Desktop 一类虚拟机方案还需要先把项目 `logs/` 目录挂进虚拟机，否则容器里只会出现一个空的可写目录。新增 `test/run-f013.sh` 聚合本切片测试。

完成判据：`go build ./...` 通过；`test/run-f013.sh` 全量通过；`make logs-viewer` 实际起得来，`http://127.0.0.1:19002` 能打开 code-server，文件树能按计划 → 配置或运行 → 执行路径 → 日期或运行号逐层找到日志，容器内能读取日志且挂载目录可写（容器内新建的文件宿主 `logs/` 能立刻看到），挂载自检在挂载未生效时能拦下并报错，`make logs-viewer-stop` 能正常停止。

## 自动验证

- `go build ./...`
- `go vet ./...`
- `go test -race ./test/unit/backend/logging/...`
- `go test ./test/integration/...` 中本切片新增的日志用例
- `test/run-f013.sh` 聚合上述项
- 写端点白名单断言：本切片白名单为空，断言运行测试过程中实际发出的目标写端点集合为空集

测试归档位置：`test/unit/backend/logging/`、`test/integration/`、人工步骤 `test/manual/`。

## 人工验收

1. 启动后端与前端，在浏览器里依次做：验证账号、打开流程图、进入一条路径的节点配置、打开历史业务数据工作区。
2. 执行 `make logs-viewer`。使用 Colima 等虚拟机方案时，需先在 `~/.colima/default/colima.yaml` 的 `mounts` 中加入本项目 `logs/` 目录并设为 `writable: true`，再 `colima restart`；挂载没生效时该命令会直接报错并停掉容器。浏览器打开 http://127.0.0.1:19002 ，确认 code-server 直接停在 `/home/coder/logs`，
   顶层只有 `application/` 与 `plans/` 两棵新目录树，并能按计划 → `configuration` → 执行路径 → 日期逐层点到日志；
   核对完成后执行 `make logs-viewer-stop`。
3. 打开 `logs/plans/<计划显示名>__plan-<ID>/configuration/<执行路径显示名>__path-<ID>/<今天>/network.log`，
   确认上一步的每个操作都有对应请求行，每行都带 `plan_id`、`plan_name`、`execution_path_id`、`execution_path_name`，
   字段可读、中文可读、没有乱码；同目录的 `meta.json` 与目录名指向同一个计划和执行路径。
4. 打开 `logs/application/<今天>/application.log`，确认里面只有服务启动、监听、停止这类系统事件，
   没有计划配置、表单读取、节点配置等业务操作日志。跨计划接口（如计划列表）没有计划 ID，其请求行按规则留在这里。
5. 打开同目录的 `curl.log`，复制任意一条命令到终端执行，确认能拿到与当时一致的响应。
6. 把目标平台地址临时改成一个不可达地址，重复一次账号验证。确认：界面给出中文错误提示；
   该计划目录的 `network-error.log` 出现对应失败行；同目录 `operation-error.log` 出现一行 `error_class=network`，
   其 `user_message` 与界面上那句提示完全一致，且这条业务异常没有写进 `application-error.log`。
7. 确认 `git status` 干净，`logs/` 没有进入待提交列表。

## 完成标准

- [x] 行为已实现并实际运行。
- [x] 当前范围测试、`go vet` 与构建已通过。
- [x] 已检查同一范围内的相似问题（其余目标请求出口、其余错误响应出口）。
- [x] 所有新增具名函数与方法有中文注释，导出符号注释以符号名开头。
- [x] 文档状态已更新为 `ready_for_manual`。
- [x] 已列出用户手工核对步骤。

## 回退边界

日志设施失效不得阻塞主流程：日志根不可写、磁盘满、轮转失败时降级为一次标准错误输出并继续服务。不因为写不了日志就让配置操作失败。

## 状态记录

- 2026-09-04：人工验收未通过，状态从 `ready_for_manual` 退回 `implementing`。
  未通过原因：日志查看方式被错误地实现成自建 Node 网页（`logs-viewer/` + `pnpm dev:l`），
  与本文件"包含"条目以及 `docs/EXECUTION_PROGRAM.md` 第 6.7 节已批准的裁决（沿用 code-server、不另造日志页面）冲突。
  产品明确不做独立技术状态页，日志查看必须复用用户已经认可的 code-server。
- 2026-09-04：容器验证被本机 Docker 磁盘占满阻塞，状态保持 `implementing`。
  本机 Docker 运行时是 colima（已由本次实施启动，profile `default`），其数据盘
  `/dev/vdb1` 挂载在 `/var/lib/containerd`，59G 已用 58G、可用 0，使用率 100%。
  表现：`docker pull codercom/code-server:4.96.4` 报
  `failed to extract layer ... no space left on device`；改用本机已有的
  `codercom/code-server:4.126.0` 同参数试启动，容器也在启动阶段退出，
  容器日志为 `error ENOSPC: no space left on device, mkdir '/home/coder/.config'`。
  已确认这不是 Makefile 目标的问题：Docker 未启动时两个目标都按预期直接报错退出。
  释放空间需要删除本机既有镜像或构建缓存（`docker system df`：镜像 117 个共 60.64GB，
  构建缓存 43.29GB），属于用户数据，未获明确授权前不执行。
  待用户释放空间或明确授权清理后，按 T06 完成判据重新验证并把状态推进到 `ready_for_manual`。
- 2026-09-04：Docker 磁盘已由用户自行恢复（`/dev/vdb1` 59G 已用 19G、可用 37G、35%），未执行任何 `prune`，用户镜像、构建缓存与卷全部保留。磁盘阻塞解除后暴露出真正的挂载阻塞：
  colima 只挂载了 `/Volumes/oygsky/bigsys`（只读），本项目在外接盘 `/Volumes/oygsky/AIstudy/test-auto-pro-v2`，  虚拟机里根本看不到该路径，所以 `docker run -v` 只是在虚拟机内新建了一个空的可写目录，  容器内 `/home/coder/logs` 是空目录。`docker inspect` 的 `RW=true` 在这种情况下依然为真，不能作为挂载成功的判据。
- 2026-09-04：本机基础设施变更（仓库外，已先向用户说明）。在 `~/.colima/default/colima.yaml` 的 `mounts` 中新增
  `- location: /Volumes/oygsky/AIstudy/test-auto-pro-v2/logs` 且 `writable: true`，保留原有 bigsys 只读挂载，  改动前备份为 `colima.yaml.bak-20260904`，随后 `colima restart` 使挂载生效（重启前运行中的容器为 0，无工作负载受影响）。  只挂载 `logs/` 一个目录，项目源码不进入虚拟机。仓库内配套修复：`make logs-viewer` 增加挂载双向自检。
- 2026-09-04：T06 容器验证实测通过，状态推进到 `ready_for_manual`。证据：
  - 虚拟机内 `/Volumes/oygsky/AIstudy/test-auto-pro-v2/logs` 为 `virtiofs (rw)`；宿主 501:20 与虚拟机 501 属主一致，容器内以 501 读写正常。
  - 容器内 `ls /home/coder/logs` 实际列出 `app-2026-09-04.log`、`app-error-2026-09-04.log`、`config/`，并读出 `config/2026-09-04/network.log` 的真实请求行。
  - 容器内写入探针文件，宿主 `logs/` 立刻可见同一内容；随后删除，宿主同步消失。
  - `http://127.0.0.1:19002/` 返回 302 到 `./?folder=/home/coder/logs`，code-server 资源接口读取两个日志文件的字节数与宿主完全一致（1460 与 1174），即文件树读到的就是挂载进来的真实文件。
  - 反向验证：用 `--tmpfs` 造出"空的可写目录"这一失败形态，挂载自检能识别并拦下，不会误判为成功。
  - `make logs-viewer-stop` 后容器被删除、19002 端口释放；`./test/run-f013.sh` 全量通过；`git status` 只有 `Makefile` 一处改动。
- 2026-09-04：修复日志查看方式。删除 `logs-viewer/` 自建网页及其全部入口，不保留兼容层；
  恢复 `make logs-viewer` 与 `make logs-viewer-stop`，只提供单个 code-server 容器启动方式，
  Docker 未启动时明确报错退出。日志采集、错误分类与 curl 重放未改动。
  全局日志按天分文件与默认保留 7 天两项按用户要求保留。
- 2026-09-04：T01 至 T06 实施完成，`test/run-f013.sh` 全量通过，状态进入 `ready_for_manual`。
  实际运行证据（本机 19013 端口起新构建，接真实目标与真实数据库）：
  - `logs/config/<当天>/network.log` 出现真实目标请求行，含 `trace_id`、`request_class=read`、
    `status_code=200`、`duration_s`、`outcome_kind=business_success`。
  - `logs/config/<当天>/curl.log` 的命令直接复制到终端执行，返回与当时一致的登录成功响应。
  - 请求一个不存在的计划后，`program-error.log` 出现 `error_class=tool_config`、
    `error_code=PLAN_NOT_FOUND`、`user_message=计划不存在`，与接口响应体的 `error.message` 完全一致。
  - `logs/` 已被 `.gitignore` 忽略，`git status` 保持干净。
- 实施期间发现并修正的偏差：`source` 字段原按固定调用层数定位，内联后会漂移到 `testing.go`；
  改为用 `runtime.CallersFrames` 逐帧过滤日志包自身与运行时帧后定位。
- 写端点白名单检查只扫描 `internal/adapter/target`：`internal/engine/actioncatalog` 里的
  `targetOperation` 是动作目录的说明元数据，描述未来执行时会调用哪个接口，不构成一次请求。

- 2026-09-04：人工验收未通过，状态从 `ready_for_manual` 退回 `implementing`。
  未通过原因：“日志没有按计划和执行路径归档，配置日志集中在日期目录，无法从业务对象定位。”
  具体表现：所有配置阶段日志都落在 `logs/config/<日期>/`；`internal/api/request_logging.go` 只注入了 `RequestID`；
  `logging.Scope` 虽然有 `PlanName`、`PathName`、`RunSeq`，但没有从真实业务请求接入，`logs/runs` 路由只在合成测试里生效；
  根目录 `app-<日期>.log` 又把所有内容聚合一遍，用户无法从计划和执行路径快速定位日志。
  用户同时给出新的目录规则：顶层只有 `application` 与 `plans` 两棵树，业务日志按
  计划 → 配置/运行 → 执行路径 → 日期或运行号逐层归档，配置阶段用 `operation.log`，执行阶段用 `execution.log`，
  `application` 只保留启动停止与无法归属业务对象的系统级事件，且不得与业务日志重复写入。

- 2026-09-04：按人工验收反馈重构日志归档，状态回到 `ready_for_manual`。落地内容：
  顶层只保留 `logs/application/` 与 `logs/plans/`，删除所有计划共用的 `logs/config/<日期>` 路由，不留兼容层；
  `logging.Scope` 扩展出 `PlanID`、`PlanName`、`ExecutionPathID`、`ExecutionPathName` 并进日志字段；
  `WithScope` 改为合并语义；中间件只从路由取不可变 ID，显示名由 `service.LogScopeService` 从真实业务记录读取
  （按计划 ID 读执行路径顺带完成归属校验，每请求解析一次并带 30 秒缓存）；作用域随 `context` 传给目标站点客户端，
  网络日志与可重放命令因此与业务日志落在同一个计划目录；配置阶段改用 `operation.log`，执行阶段用 `execution.log`；
  已归属的业务日志不再重复写进根目录聚合文件，业务异常也不改写进 `application-error.log`。
  真实数据实测证据（本机 19013 端口接真实目标与真实数据库，计划 2 名为 `oyg测试002`，路径 13 名为 `路径 1`）：
  - 访问 `/api/plans/2/execution-paths/13/configuration` 后，`operation.log`、`network.log`、`curl.log` 与 `meta.json`
    全部落在 `logs/plans/oyg测试002__plan-2/configuration/路径 1__path-13/2026-09-04/`，每行都带
    `plan_id=2 plan_name=oyg测试002 execution_path_id=13 execution_path_name=路径_1`。
  - 同时访问路径 13 与路径 14 的配置接口，两个目录互不出现对方的 `execution_path_id`，没有串目录。
  - 只带计划 ID 的 `/api/plans/2/flow-graph` 与业务数据候选接口进 `configuration/_plan/2026-09-04/`。
  - `logs/application/2026-09-04/application.log` 里只有服务开始监听、服务已停止和无计划 ID 的跨计划接口请求行，
    没有计划配置、表单读取、节点配置这类业务操作日志。
  - `meta.json` 的 `planId`、`planName`、`executionPathId`、`executionPathName` 与目录名一致，运行目录另有 `runId` 与 `startedAt`。
  - code-server 内可逐层点开 计划 → `configuration` → `执行路径 1__path-13` → `2026-09-04`，
    资源接口读取含中文与空格的嵌套路径，字节数与宿主完全一致；`make logs-viewer-stop` 与再次 `make logs-viewer` 均正常。
  - `./test/run-f013.sh` 全量通过，`git status` 无 `logs/` 产物。
  说明：改动前产生的 `logs/app-<日期>.log` 与 `logs/config/<日期>/` 仍在磁盘上（按要求不删除既有日志），
  新代码已不再写入也不再清理这两处，需要清空顶层时由用户自行删除。

- 2026-09-04：代码审查发现两个 P1 问题，均已修复并实测，状态保持 `ready_for_manual`。
  1. 后台任务丢失计划日志作用域：`internal/service/execution_path.go` 的后台全路径解析与
     `internal/service/history_replay.go` 的一键配置 worker 都用 `context.Background()` 起协程，
     只传 `planID`，因此拿不到 `plan_id`、`plan_name` 与 `request_id`，它们发出的目标请求日志会掉进
     `logs/application/<日期>/`。修复：新增 `service.backgroundLogScope`（任务自己的计划 ID 兜底、
     请求作用域优先、计划名缺失时再从计划记录补一次），`PathGenerationJob` 保存发起时的作用域并注入后台 context；
     `startWorker` 接收请求 context 只为取作用域和补计划名，worker 生命周期仍与请求解绑；
     `replayItem` 读到执行路径后把路径归属接回 `ctx`，每条明细都落进该路径自己的目录。
  2. 多行日志块并发写入交错：`WriteBlock` 原来逐行调用 `WriteLine`，各自加解锁，
     两个请求同时写 `curl.log` 时块与块互相穿插。修复：单行与块统一走 `append`，
     在同一把锁内完成轮转判断与一次写入，行号语义不变。
  修复过程中还发现并修掉 `StartGeneration` 的数据竞争：返回值原来在解锁后才复制整个任务结构体，
  而后台协程已经在改写 `Status` 与 `UpdatedAt`，`go test -race` 可复现；改为在锁内取快照。
  新增用例：并发多行块不穿插（40 个五行块）、后台路径解析与一键配置 worker 的作用域继承与计划名回补，
  三处都做过变异验证——去掉修复后对应用例失败。
  实测：一键配置计划 2 路径 13 后，后台 worker 的目标请求日志落在
  `logs/plans/oyg测试002__plan-2/configuration/路径 1__path-13/2026-09-04/network.log`，
  带完整归属字段；后台全路径解析落在同一计划的 `configuration/_plan/2026-09-04/`；
  `application` 目录没有混入任何目标请求。`go test -race ./test/unit/...` 与 `./test/run-f013.sh` 全量通过。

正常状态按 `preparing -> awaiting_approval -> implementing -> ready_for_manual -> accepted` 推进。当前为 `ready_for_manual`：T01 至 T06 已实施，日志查看方式与日志归档方式两轮反馈均已修复并实测通过，等待用户按上面「人工验收」七步确认后再推进到 `accepted`。
