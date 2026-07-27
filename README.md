# 流程自动化测试平台

F-000 提供最小可运行骨架。正式页面视觉、测试计划 mock 和目标平台调用不在本阶段范围内。

## 本地开发

首次安装前端依赖：

```bash
pnpm install
```

使用两个前台终端：

```bash
# 终端一：后端 Go 热更新
pnpm dev:backend
```

```bash
# 终端二：前端 Vite 热更新
pnpm dev:frontend
```

两条命令都在当前终端输出日志，按 `Ctrl+C` 停止。后端 Air 固定在 `go.mod` 的 Go 1.25 tool dependency 中，通过 `go tool air` 运行，无需全局安装。前端地址为 `http://127.0.0.1:19000`，健康接口为 `http://127.0.0.1:19080/api/health`。

应用壳默认使用浅色主题；左上角“深色”入口可切换主题，选择会保存在当前浏览器。

## 当前验证

```bash
pnpm check:backend
pnpm check:frontend
pnpm test:f000
```

参考代码同步与状态核对使用：

```bash
make refs-sync
make refs-status
```
