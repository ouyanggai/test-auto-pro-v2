# 技术架构

## 固定选型

- 前端：纯 Vue 3、Vite、Vue Router、Pinia、Naive UI。
- 后端：Go 单体，一个二进制进程；F-000 使用标准库 `net/http`。
- 包管理：pnpm。
- 本地端口：前端 `19000`，后端 `19080`；前端开发服务器将 `/api` 代理到后端。
- Go module：`test-auto-pro-v2`。

不采用 Vben Admin、Element Plus 或其他中后台应用基座。历史 PRD 中相反描述已被本文件取代。

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

- 应用壳只提供产品名、三个中文导航和路由内容区。
- 保持浅色、中文、桌面优先；F-001 才进行正式视觉稿与计划页 mock。
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
