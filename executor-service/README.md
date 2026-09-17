# executor-service（F-036 独立执行服务）

Playwright + Chromium 无头执行服务：装载参考项目原生 `/flow` 路由，拦截目标请求并转发给 Go，不做界面操作。边界见 `docs/s2/01-执行服务设计.md`。

`flow-source/` 由 `executor-service/flow-source-manifest.json` 固定远端、分支与提交，用 `sync-source` 直接 git 同步（当前 `test`/`bff4ef8`），不依赖任何本机静态目录。FormMaking 产物不在上游 Git 里，同步时从 `form-runtime/runtime-source` 的受控副本补齐并逐字节校验。原生 `/flow` 页面由 `build-native-page` 在本机真实构建到 `flow-source/dist`。

## 固定运行方式

- `playwright` 版本固定为 `1.63.0`。
- 启动参数固定在 `src/browser.ts` 的 `HEADLESS_LAUNCH_ARGS`，默认无头。
- 默认端口 `19011`，沿用 `RUNTIME_EXECUTOR_URL`。Go 不托管本进程。

## 命令

```bash
pnpm --dir executor-service install
pnpm --dir executor-service exec playwright install chromium
pnpm --dir executor-service sync-source
pnpm --dir executor-service build-native-page
pnpm --dir executor-service build-form-making
pnpm --dir executor-service typecheck
pnpm --dir executor-service probe
pnpm --dir executor-service start
pnpm --dir executor-service health
```

`sync-source` 幂等：HEAD、远端与内容都与清单一致时只报告「已是最新」，不重新下载；本地被改动时会重新检出并覆盖。

`build-native-page` 按源码摘要判定产物是否过期，未过期直接跳过；置 `EXECUTOR_FORCE_PAGE_BUILD=1` 可强制重建。它只依赖 Node，不再需要 macOS 上手工产出的 `dist.zip` 或系统 `unzip`。

`probe` 退出码为 0 表示无头 Chromium 可启动。`health` 只请求本服务 `/health`，不访问目标平台。

## 目录

- `src/`：执行进程
- `flow-source/`：按清单 git 同步的 `rsh-flow-components` 检出与页面产物（已忽略，不进版本库）
- `page/`：独立页面构建包（后续任务）
