# executor-service（F-036 独立执行服务）

Playwright + Chromium 无头执行服务：装载参考项目原生 `/flow` 路由，拦截目标请求并转发给 Go，不做界面操作。边界见 `docs/s2/01-执行服务设计.md`。

组件快照复制自 `/Volumes/oygsky/bigsys/rsh-flow-components`（`test`/`be355ae`，含 FormMaking 产物）。后续一键升级与原项目同一套 `sync-manifest`。



## 固定运行方式

- `playwright` 版本固定为 `1.63.0`。
- 启动参数固定在 `src/browser.ts` 的 `HEADLESS_LAUNCH_ARGS`，默认无头。
- 默认端口 `19011`，沿用 `RUNTIME_EXECUTOR_URL`。Go 不托管本进程。

## 命令

```bash
pnpm --dir executor-service install
pnpm --dir executor-service exec playwright install chromium
pnpm --dir executor-service sync-source
pnpm --dir executor-service build-form-making
pnpm --dir executor-service typecheck
pnpm --dir executor-service probe
pnpm --dir executor-service start
pnpm --dir executor-service health
```


`probe` 退出码为 0 表示无头 Chromium 可启动。`health` 只请求本服务 `/health`，不访问目标平台。

## 目录

- `src/`：执行进程
- `flow-source/`：独立 `rsh-flow-components` 快照（T002）
- `page/`：独立页面构建包（后续任务）
