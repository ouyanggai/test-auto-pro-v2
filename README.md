# 流程自动化测试平台

当前已完成应用壳、测试计划静态页面，以及 F-002 真实账号登录和三类目标平台只读候选接入。F-002 正等待用户使用现场账号与流程人工验收；当前仍不保存计划、不配置路径、不修改目标平台。

## 本地开发

首次安装前端依赖：

```bash
pnpm install
```

使用两个前台终端：

```bash
# 终端一：后端 Go 热更新
pnpm dev:b
```

```bash
# 终端二：前端 Vite 热更新
pnpm dev:f
```

两条命令都在当前终端输出日志，按 `Ctrl+C` 停止。后端 Air 固定在 `go.mod` 的 Go 1.25 tool dependency 中，通过 `go tool air` 运行，无需全局安装。前端地址为 `http://127.0.0.1:19000`，健康接口为 `http://127.0.0.1:19080/api/health`。

应用壳默认使用浅色主题；顶栏右侧主题入口可切换深浅主题，选择会保存在当前浏览器。

## 目标平台本机配置

当前工作区已经从 V1 配置安全生成被 Git 忽略的 `.env.local`。后端启动时自动读取，日常开发无需填写或导出 `TARGET_*`；进程环境中的同名变量仍可覆盖本机文件，便于后续切换环境。

V1 配置变化时，可在维护时执行一次以下命令刷新本机文件；命令不会回显配置值，正常启动不依赖 V1 仓库持续存在：

```bash
go run ./cmd/sync-v1-target-config -source /path/to/V1/config.yaml
```

首次在另一台机器生成时，统一登录配置必须通过安全的进程环境传给同步命令，生成后不再需要每次提供。变量名和用途见 `.env.example`；真实连接、密码、AES key、code 和验收账号不得写入仓库或前端。

缺少必需目标配置时后端仍可启动，health 保持正常；目标平台 API 会返回 `TARGET_CONFIG_MISSING`。

## 当前验证

```bash
pnpm check:backend
pnpm check:frontend
pnpm test:f000
./test/run-f002.sh
```

参考代码同步与状态核对使用：

```bash
make refs-sync
make refs-status
```
