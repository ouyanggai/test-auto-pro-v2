# F-007 独立表单运行时

本目录由完整 `rsh-flow-components` 可运行源码驱动。主 Vue 3 应用只通过 iframe 与版本化 `postMessage` 使用它；真实 FormMaking、自定义组件、Vuex、router、axios 和目标表单样式均从 `runtime-source/` 进入实际 dev/build 链路。

## 目录边界

- `runtime-source/`：受控同步后的实际构建源码，不是闲置快照。它包含完整 tracked `rsh-flow-components`、原生 `scripts/sync*.js`/清单，以及参考仓库未跟踪但目标运行必需的 `src/lib/vue-form-making/dist/`。
- `src/`：本项目保护的 iframe 配置适配层，只负责版本化消息、SID 内存会话、目标只读请求、字段权限和 unsupported 边界；同步不得覆盖。
- `scripts/` 与 `sync-manifest.json`：当前项目维护流水线适配。来源仓库、远端和 `test` 固定；该分支对应当前目标测试环境，HEAD 在创建任务时记录，Worker 执行前复核，不永久锁死历史提交。
- `build/`、`babel.config.js`、`vue.config.js`：让本地适配入口以完整 `runtime-source/` 为真实依赖完成 Vue CLI dev/build，并输出可经 HTTP 核验的源码快照。

目标自定义组件保持真实注册。仅当模板引用的组件不在实际入口注册表中，或依赖配置阶段禁止执行的业务写钩子时，才明确标记 `unsupported`；不得降级为普通输入框。

## 本地命令

```bash
pnpm --filter test-auto-pro-v2-form-runtime typecheck
pnpm --filter test-auto-pro-v2-form-runtime build
pnpm dev:f
make form-runtime-status
make form-runtime-sync
```

`pnpm dev:f` 同时持有主前端与 19001 表单服务。`make form-runtime-sync` 只创建固定维护任务；后台按 `INSPECT → SYNC → SYNC_CHECK → BUILD → RESTART → VERIFY → COMPLETED` 执行。同步先写隔离候选，候选构建成功后才同时切换实际 dev 输入与生产 `/form-runtime/` 产物；运行中的 HTTP 快照未切换、候选失败或健康失败都会恢复并再次验证 previous。

SID 只存在于当前 iframe 会话的内存认证与请求策略，关闭工作区、切换计划/路径/账号时销毁；不写入配置数据库、Git或浏览器长期存储。配置阶段阻断目标流程提交、草稿和已知业务写接口。
