# Docker 部署

发布包含 MySQL、Go 后端、Vue 主前端、配置表单运行时、独立执行服务和日志查看器六个服务。工具数据库使用独立持久化卷，首次启动自动执行现有向前迁移；本部署不导入开发机的计划与运行记录。

## 一键更新 192.168.2.200

在本机双击 `deploy/update.cmd`，或在项目根目录运行：

```powershell
python -u deploy/update.py
```

当前电脑的连接配置已写入被 Git 忽略的 `deploy/.env.update.json`，自动读取服务器密码，无需再次输入。该文件仅保存在本机，不打包、不上传、不提交。其他电脑需要自行准备同名 JSON，字段为 `host`、`user`、`password`、`directory`、`proxy_port`；当前部署目录固定为 `/opt/test-auto-pro-v2`，代理端口默认 `7897`，设为 `0` 可关闭代理。需要 Python 3、Git、Paramiko（缺少时执行 `python -m pip install paramiko`）；执行源码清单更新时自动调用现有 Node 同步脚本，因此也需要 Node 和本机 Git 访问权限。

脚本发布当前 **Git 跟踪文件的已保存内容**，包括尚未提交的改动；新增源码先 `git add`，无须先 push。每次发布都会重新读取组件基座与全过程仓库各自远端 `test` 分支的最新提交，再把本次刚取得的版本组装为同一份 `/flow` 页面源码；`--package-only` 也执行相同的重新读取和源码准备。此次 SHA 仅记入当次发布包、构建缓存与版本事实，下一次发布会再次查远端，不沿用固定 SHA。远端不可达、同步或校验失败时立即停止，不会上传旧包或继续切换服务。

本机全过程仓库目录只用于确认规范远端；发布从远端 Git 对象读取，不复制本机工作树。因此该目录保持 `oygdev2` 或存在未提交修改都不会影响发布，也不需要切换分支。`executor-service/flow-source` 是本机生成的候选目录，不提交；包内只包含已校验源码，不含 Git 元数据、凭证、依赖缓存、日志或页面构建产物。受控 FormMaking 产物仍从 `form-runtime/runtime-source` 获取，不修改其库源码。

本项目专用的全过程业务修复放在 `executor-service/flow-source-overrides/`，只在本机合成候选上应用，不写回全过程仓库。覆盖定义必须对目标源码命中唯一上下文，候选摘要和 `deploy/release-source.json` 会记录覆盖数量；来源代码发生不兼容变化时同步直接失败，不能带着旧覆盖继续发布。

上传后先在独立目录构建四个应用镜像，成功后才替换部署源码并启动六服务；数据库卷、`deploy/.env`、日志和当前配置表单维护卷保留。源码目录整体替换，不遗留已删除的旧源码。配置表单的当前维护版本继续由现有维护流水线管理，不在发布时覆盖持久化 live 卷。构建失败不切换容器；切换后的健康或验活失败会返回非零退出码并保留现场，不自动回退数据库迁移。发布锁拒绝并发更新。

本机代理运行时自动建立临时 SSH 隧道并传给构建。缺少基础镜像时临时给 Docker 配置下载代理，直接访问官方仓库，发布结束恢复原配置；代理未运行时使用服务器直连。六服务健康、HTTP、Chromium 和日志双向挂载检查全部通过才报告成功。

本机日志在 `.runtime/deploy/`，服务器日志在 `/opt/test-auto-pro-v2/logs/update-*.log`；成功版本事实在服务器 `.runtime/last-release.json`。终端完成后按任意键关闭，不自动打开验收浏览器。

## 准备与启动

在源码根目录执行：

```bash
# 直接 Compose 构建前必须先同步全过程 test 并准备执行器候选。
node executor-service/scripts/sync-flow-source.js
pnpm --dir executor-service build-native-page
cp deploy/.env.example deploy/.env
# 在 deploy/.env 中填写目标平台配置和工具库密码。
docker compose --env-file deploy/.env -f deploy/docker-compose.yml up -d --build
docker compose --env-file deploy/.env -f deploy/docker-compose.yml ps
bash test/integration/f023_deployment_smoke.sh
```

生产一键发布会自动完成源码准备；直接 Compose 构建则必须先执行上面的同步与页面构建命令。同步每次重新读取全过程远端 `test`，远端不可读时失败，不回落到上一次业务源码。页面构建会校验候选摘要，源码未变化时才复用页面产物。

`deploy/.env` 必须限制为部署账号可读（`chmod 600 deploy/.env`），不进入 Git、镜像或构建参数。后端从容器环境读取目标配置；浏览器执行服务通过 Go 转发目标请求。执行服务只在容器内网监听，发布时固定源码版本，启动时关闭远端自动同步。

| 入口 | 默认端口 |
| --- | --- |
| 主应用与 API 反代 | 19000 |
| 后端 API | 19080 |
| 配置表单运行时 | 19001 |
| 日志查看器 | 19002 |
| MySQL、独立执行服务 | 仅容器内网 |

表单维护需要宿主通过 `make refs-sync` 安全同步参考仓库，再以只读方式挂载给后端；禁止复制脏工作树。后端镜像自带 git、Node、pnpm 和锁定的配置侧构建依赖。`fr-live` 同时挂到后端的产物目录和 nginx 的 `/form-runtime/` 目录，维护切换后立即提供新版本。维护健康检查读取实际 `runtime-health.json`。

若下载需要代理，Docker 守护进程与构建过程都需要能访问代理。服务器上的 `127.0.0.1` 指服务器本机；部署完成后撤除临时隧道和代理配置，运行服务不依赖下载代理。

执行镜像的 Debian 下载地址可通过构建参数 `DEBIAN_MIRROR` 和 `DEBIAN_SECURITY_MIRROR` 设置。APT 使用签名校验、五次重试和独立构建缓存；Chromium 的无头产物单独下载，网络失败重试时保留已完成的系统依赖层。

## 运维

```bash
docker compose --env-file deploy/.env -f deploy/docker-compose.yml logs --tail 100 backend executor
docker compose --env-file deploy/.env -f deploy/docker-compose.yml restart
bash test/integration/f023_deployment_smoke.sh
# 停止服务，保留数据库、运行时卷和宿主 logs/。
docker compose --env-file deploy/.env -f deploy/docker-compose.yml down
```

备份应覆盖工具 MySQL 的一致性导出、`logs/`、`fr-live`、`fr-maintenance` 和单独保管的环境文件；恢复到同一版本后再检查迁移与六服务健康。不要使用 `down -v` 或全局 prune 清理生产数据。

验活脚本检查六服务健康、HTTP 入口、后端到执行服务的连接、无头 Chromium 启动和日志双向挂载。它不创建计划、不发送目标业务写请求、不启动人工验收浏览器。页面显示、表单编辑及真实流程运行由用户人工核对。
