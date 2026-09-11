# F-029 会话缓存与网络重试

- 状态：`ready_for_manual`
- 计划确认：2026-09-11（用户批准实施）
- 产品依据：`docs/PRODUCT.md`「会话与网络错误边界」
- 架构依据：`docs/ARCHITECTURE.md`「F-029 会话缓存与网络重试」

## 交付范围

1. 目标客户端与会话管理器在服务装配层只创建一份，读服务和执行器共享按账号的 SID 缓存。
2. 同账号验证、读取、写入和核验排队；缓存用会话代次阻止迟到旧登录结果覆盖新 SID。
3. `RESP401`、`AUTH_401`、`SID已失效` 只允许当前账号重登并重放一次；第二次仍失效时停步并提示外部会话竞争。
4. 完整业务拒绝透传目标 `code/message` 且不可重试。读取网络错误可有限重试；写请求只有连接未建立时重试，写出后响应丢失不重发。
5. 网络日志记录 `retry`、`retry_attempt`、`transport_phase`、`request_written`，禁止记录 SID、密码等敏感值。

## 自动验证

- `test/unit/backend/session_test.go`：同账号读操作串行、不同账号并行、刷新代次防旧 SID 覆盖、读取连接错误重试及业务错误单次返回。
- `test/unit/backend/target/business_error_test.go`：完整业务拒绝保留原始 code/message，连接阶段与响应阶段重试分类。
- `test/unit/backend/executor/step_test.go`：写请求未写出时重试，响应中断和业务拒绝不重发。
- `test/unit/backend/logging/program_and_network_log_test.go`：网络日志写入重试标记与尝试序号。
- `test/contracts/target_api_test.go`：业务拒绝公开响应不可重试。

已通过定向 Go 测试、`go test -race` 会话并发回归和构建检查；真实目标端 TCP 抖动、浏览器连续推进与外部账号竞争由人工验收。

## 人工验收

1. 连续推进一条包含读取、审批、核验的路径，确认读服务与执行器不会互相覆盖 SID。
2. 人为制造 TCP connect timeout，确认读取在有限退避后恢复，页面能区分网络重试与业务拒绝。
3. 模拟写请求已写出但响应丢失，确认不会重复提交，运行停在可核验状态。
4. 用浏览器或其他进程登录同一账号，确认工具最多自动重登一次，随后明确停步提示外部会话竞争。
