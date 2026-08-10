# F-007 独立表单运行时

本目录是当前项目独立持有的目标表单渲染服务。主 Vue 3 应用只通过版本化 `postMessage` 和 iframe 使用它；运行时本身不复用旧工作台路由、登录页或流程提交入口。

## 目录边界

- `upstream/`：`rsh-flow-components` 固定提交的原样区，只能由受控同步清单覆盖。
- `vendor/form-making/`：目标副本实际使用的 `form-making-advanced 1.6.12` 构建产物；参考仓库没有跟踪该忽略产物，因此单独固定并校验。
- `src/`：本项目适配层，只包含 iframe 入口、版本化消息、SID 会话、目标只读请求策略、字段权限和 unsupported 识别；同步不得覆盖。
- `sync-manifest.json`：固定来源、精确 HEAD、同步映射、排除项和本地保护区。

目标自定义组件源码完整保留在 `upstream/`。当前依赖旧工作台路由、Vuex 或业务写接口的组件不会静默降级为普通输入框，而是由模板扫描明确标记 `unsupported` 并阻止保存为可执行配置。

## 本地命令

```bash
pnpm --filter test-auto-pro-v2-form-runtime typecheck
pnpm --filter test-auto-pro-v2-form-runtime build
pnpm dev:f
make form-runtime-status
make form-runtime-sync
```

`make form-runtime-sync` 只创建固定维护任务。后台按 `INSPECT → SYNC → SYNC_CHECK → BUILD → RESTART → VERIFY → COMPLETED` 执行：pnpm 先输出到隔离版本目录，构建成功后才原子切换 `web/dist/form-runtime`；切换或健康检查失败会恢复并再次验证 previous 版本。

SID 只存在于当前 iframe 会话的内存闭包，关闭工作区、切换计划/路径/账号时销毁；不写入配置数据库、Git 或浏览器长期存储。配置阶段阻断目标流程提交、草稿和已知业务写接口。
