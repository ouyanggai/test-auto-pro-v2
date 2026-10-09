# Docker 部署

发布包含 MySQL、Go 后端、Vue 主前端、配置表单运行时、独立执行服务和日志查看器六个服务。工具数据库使用独立持久化卷，首次启动自动执行现有向前迁移；本部署不导入开发机的计划与运行记录。

## 一键更新 192.168.2.200

在本机双击 `deploy/update.cmd`，或在项目根目录运行：

```powershell
python -u deploy/update.py
```

当前电脑的连接配置已写入被 Git 忽略的 `deploy/.env.update.json`，自动读取服务器密码，无需再次输入。该文件仅保存在本机，不打包、不上传、不提交。其他电脑需要自行准备同名 JSON，字段为 `host`、`user`、`password`、`directory`、`proxy_port`；当前部署目录固定为 `/opt/test-auto-pro-v2`，代理端口默认 `7897`，设为 `0` 可关闭代理。需要 Python 3、Git、Paramiko（缺少时执行 `python -m pip install paramiko`）；执行源码清单更新时自动调用现有 Node 同步脚本，因此也需要 Node 和本机 Git 访问权限。

脚本发布当前 **Git 跟踪文件的已保存内容**，包括尚未提交的改动；新增源码先 `git add`，无须先 push。执行源码按清单固定提交从 Git 对象打包，并拒绝真实脏改动；缺少或过期快照会自动安全同步。凭证、参考仓库、依赖缓存、日志和本机构建产物不会进入包。

上传后先在独立目录构建四个应用镜像，成功后才替换部署源码并启动六服务；数据库卷、`deploy/.env`、日志和当前配置表单维护卷保留。源码目录整体替换，不遗留已删除的旧源码。配置表单的当前维护版本继续由现有维护流水线管理，不在发布时覆盖持久化 live 卷。构建失败不切换容器；切换后的健康或验活失败会返回非零退出码并保留现场，不自动回退数据库迁移。发布锁拒绝并发更新。

本机代理运行时自动建立临时 SSH 隧道并传给构建。缺少基础镜像时临时给 Docker 配置下载代理，直接访问官方仓库，发布结束恢复原配置；代理未运行时使用服务器直连。六服务健康、HTTP、Chromium 和日志双向挂载检查全部通过才报告成功。

本机日志在 `.runtime/deploy/`，服务器日志在 `/opt/test-auto-pro-v2/logs/update-*.log`；成功版本事实在服务器 `.runtime/last-release.json`。终端完成后按任意键关闭，不自动打开验收浏览器。

## 准备与启动

在源码根目录执行：

```bash
# 执行服务按清单同步自己的源码；需要配置内网 Git 的访问权限。
pnpm --dir executor-service sync-source
cp deploy/.env.example deploy/.env
# 在 deploy/.env 中填写目标平台配置和工具库密码。
docker compose --env-file deploy/.env -f deploy/docker-compose.yml up -d --build
docker compose --env-file deploy/.env -f deploy/docker-compose.yml ps
bash test/integration/f023_deployment_smoke.sh
```

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
