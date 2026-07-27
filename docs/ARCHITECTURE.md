# 技术架构

## 固定选型

- 前端：纯 Vue 3、Vite、Vue Router、Pinia、Naive UI。
- 后端：Go 单体，一个二进制进程；F-000 使用标准库 `net/http`。
- 包管理：pnpm。
- 本地端口：前端 `19000`，后端 `19080`；前端开发服务器将 `/api` 代理到后端。
- Go module：`test-auto-pro-v2`。

不采用 Vben Admin、Element Plus 或其他中后台应用基座。历史 PRD 中相反描述已被本文件取代。

## 本地开发方式

- 根目录 `package.json` 仅提供全栈开发与检查命令；Go 依赖仍只由 `go.mod` 管理。
- 根目录 `pnpm-workspace.yaml` 只包含 `web`，从根目录执行 `pnpm install` 只生成一个 `pnpm-lock.yaml`。
- 前端在一个前台终端执行 `pnpm dev:frontend`，Vite 监听 `127.0.0.1:19000` 并提供热更新。
- 后端在另一个前台终端执行 `pnpm dev:backend`，实际命令为 `go tool air -c .air.toml`。Air 固定为 Go 1.25 tool dependency，构建 `cmd/server` 到被忽略的 `.runtime/`。
- 两个前台进程的日志留在各自终端，用户通过 `Ctrl+C` 停止；不使用后台守护、PID 文件、日志文件、`concurrently` 或后台重启命令。
- `.air.toml` 只监听项目 Go 源码，排除 `.git`、`.runtime`、根目录 `node_modules`、`web`、`参考代码`、`test`、`docs` 和历史资料。

## 系统边界

```text
浏览器 -> Vue 单页应用 -> Go HTTP API -> 目标平台适配层 -> 目标平台
                              |
                              -> 工具侧存储（后续功能）
```

- `cmd/server`：唯一服务入口。
- `internal/api`：HTTP 路由、参数校验和响应。
- `internal/engine`：后续执行状态机与调度。
- `internal/adapter/target`：后续唯一可直接调用目标平台的区域。
- `internal/analyzer`、`internal/builder`：后续表单与人员分析、数据生成。
- `internal/repository`：后续存储访问，不包含业务判断。
- `internal/model`：领域类型。
- `internal/config`：配置加载。
- `web`：单个 Vue 应用。

## 前端边界

- 应用壳只提供产品名、三个中文导航、主题切换和路由内容区；不在 F-000 实现计划列表、创建页或其他业务页面。
- 应用壳使用 Naive UI 文档站式的扁平布局：顶栏横跨全宽，侧栏从顶栏下方开始，主内容占用剩余空间；使用组件库语义色和肉眼可见的 1px 分隔线，不使用卡片套层、渐变、阴影或装饰图形。
- 侧栏使用 `NLayoutSider` 的 `collapse-mode="width"`、`show-trigger="arrow-circle"` 与 `collapsed-width="0"`；收缩时不展示无依据图标，圆形触发器保留恢复入口。
- 浏览器根节点固定为 `100dvh` 且不滚动；顶栏固定，侧栏可收缩，主内容区独立 `overflow-y: auto`，侧栏仅在菜单超出时自身滚动。Flex/Grid 子项使用最小尺寸约束，避免撑出 viewport。
- 通过 `NConfigProvider`、`NGlobalStyle` 与 `darkTheme` 实现深浅主题；默认浅色，主题值保存到 `localStorage`，所有表面、文字、边线和选中态使用 Naive UI 语义变量随主题切换。
- 保持中文、桌面优先；F-001 才进行测试计划静态 mock。
- Vue Flow 与 dagre 统一延后到 F-004；F-000 不安装也不预留画布抽象。
- 只有真实复杂流程证明 dagre 不可读时，才另行批准评估 ELK.js。

## 数据与部署演进

- MySQL 延后到 F-003。
- MongoDB 与 Redis 延后到真实运行阶段，并以实际需要为前提。
- 当前不引入 Docker、微服务、独立 worker 或部署编排。

## 参考源码边界

- `参考代码/` 完整克隆 13 个 canonical 仓库，但被当前仓库忽略。
- 同步只能首次 clone 或在分支正确、工作树干净、未分叉时执行 `pull --ff-only`。
- `rsh-cloud-invest-power-system` 的业务分析只允许查看 `GroupApproveManage` 及其直接引用公共组件。
- 不复制旧项目脏工作树，不修改同步源码。
