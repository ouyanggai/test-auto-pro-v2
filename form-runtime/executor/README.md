# form-runtime executor（执行引擎改造）

Playwright + Chromium 无头执行服务：只装载本项目真实 form-runtime 页面、注入桥接、拦截目标请求并转发给 Go，
不做界面操作（不找控件、不点击、不输入坐标）。边界见 `docs/s/01-执行引擎设计.md`。

## 固定运行方式

- `playwright` 版本固定为 `1.63.0`（对应 Chromium 版本随该包固定，见 `package.json`）。
- 启动参数固定在 `src/browser.ts` 的 `HEADLESS_LAUNCH_ARGS`，一律无头模式。
- 构建产物：先执行 `pnpm --dir form-runtime build`（产物在 `web/dist/form-runtime`，publicPath 固定 `/form-runtime/`）。

## 安装与验证命令

```bash
pnpm --dir form-runtime/executor install
pnpm --dir form-runtime/executor exec playwright install chromium
pnpm --dir form-runtime/executor typecheck
pnpm --dir form-runtime/executor probe
```

`probe` 退出码为 0 表示：无头 Chromium 可启动、固定路由 `/form-runtime/index.html#/test-auto-form` 可加载、
真实 runtime 版本可从页面读取。

## 服务

```bash
pnpm --dir form-runtime/executor start   # 默认 127.0.0.1:19011，GET /healthz
```
