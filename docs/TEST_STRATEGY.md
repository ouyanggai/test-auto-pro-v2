# 测试策略

## 风险分层

- 单元测试：快速验证纯逻辑，分别放在 `test/unit/backend/` 和 `test/unit/frontend/`。
- 契约测试：验证稳定 HTTP 响应和接口边界，放在 `test/contracts/`。
- 集成测试：验证前台开发命令、端口、热重载、代理和多组件协作，放在 `test/integration/`。
- 手工验收：记录需要用户在浏览器或真实目标平台核对的步骤，放在 `test/manual/`。
- 固定样本：非敏感测试输入放在 `test/fixtures/`。

## 执行原则

- 所有测试用例和测试脚本必须保存在根目录 `test/` 并分类。
- 日常只运行当前功能相关测试，不运行无关全量测试。
- 自动验证必须实际执行；视觉与真实目标交互由用户手工验证，不自动启动浏览器。
- 测试不得依赖或写入真实凭证，不修改目标平台数据。

## F-000 验证范围

- Go build 与健康接口精确响应。
- 前端类型检查、生产构建和 `pnpm dev:frontend` 开发服务器 smoke。
- `pnpm dev:backend` 的 Air 配置、健康接口和 Go 源码变更后的热重载。
- 两个前台命令均以前台日志和 `Ctrl+C` 停止，不建立 PID 或后台进程管理。
- 应用壳主题默认值、切换与 `localStorage` 持久化源码契约，以及 `NLayoutHeader bordered`/`NLayoutSider bordered` 的 1px 分隔线、无嵌入色的统一表面、至少 `32px × 32px` 且收缩后完整可点的 Naive UI 圆形触发器和主内容滚动边界。
- 13 个参考仓库的远端、分支、HEAD 与清洁状态。
- 参考仓库脏工作树保护必须拒绝同步。
