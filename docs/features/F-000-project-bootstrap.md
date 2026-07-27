# F-000 项目初始化

- 状态：ready_for_manual
- 产品依据：`docs/PRODUCT.md` 的“人工验收原则”
- 架构依据：`docs/ARCHITECTURE.md`
- 计划确认时间：2026-07-27

## 目标

建立能安全读取参考源码、运行最小前后端、执行分类验证并在人工门禁处停止的新项目地基。

## 范围

包含：

- Git 与忽略规则、持久化项目文档、四个项目本地技能。
- 13 个参考仓库的干净克隆、安全同步、状态报告与清单生成。
- Go 标准库健康接口和 Vue 3 最小中文应用壳。
- 根目录 pnpm workspace、两个前台热更新终端、前台启动命令和 F-000 分类测试。

不包含：

- 数据库、Docker、微服务和部署编排。
- Vue Flow、dagre、ELK.js、FormMaking。
- 正式视觉稿、计划页业务 mock、目标平台调用和旧 V1/V2 业务代码。

## 完成标准

- [x] Git 已初始化，忽略项覆盖参考源码、凭证、依赖、构建和运行产物。
- [x] 项目文档职责单一，现行技术冲突已解决。
- [x] 四个本地技能包含 `SKILL.md` 与 `agents/openai.yaml` 并通过格式校验。
- [x] 13 个参考仓库来自远端干净克隆，分支与远端 HEAD 已核对。
- [x] `make refs-sync` 和 `make refs-status` 可用，脏仓库保护已验证。
- [x] 健康接口精确返回约定 JSON，后端监听 19080。
- [x] 前端使用约定技术栈，监听 19000，三个中文路由与 `/api` 代理可用。
- [x] 根目录 `pnpm dev:frontend` 与 `pnpm dev:backend` 均使用前台热更新方式，且不保留后台 PID 管理。
- [x] Go build、前端类型检查与构建、F-000 当前测试全部通过。
- [x] 状态更新为 `ready_for_manual` 并停止，不开始 F-001。

## 状态记录

- 2026-07-27：`preparing`
- 2026-07-27：`awaiting_approval`
- 2026-07-27：用户确认计划，进入 `implementing`
- 2026-07-27：实现与自动验证完成，进入 `ready_for_manual`
- 2026-07-27：用户反馈后台 PID 启动管理方式不通过，按人工验收反馈从 `ready_for_manual` 返回 `implementing`。
- 2026-07-27：已改为根目录 pnpm 两前台终端，Air 热重载和 Vite 访问实测通过，重新进入 `ready_for_manual`。

## 人工验收

1. 在一个终端执行 `pnpm dev:backend`，保持运行并观察 Air 日志。
2. 在另一个终端执行 `pnpm dev:frontend`，打开 `http://127.0.0.1:19000`。
3. 核对产品名与“测试计划、运行记录、系统设置”三个中文入口，并依次点击确认路由和占位内容。
4. 打开 `http://127.0.0.1:19080/api/health`，核对固定 JSON。
5. 修改任意项目 Go 源码并保存，核对后端终端显示重编译和重新运行。
6. 在两个终端分别按 `Ctrl+C` 停止前端和后端。
