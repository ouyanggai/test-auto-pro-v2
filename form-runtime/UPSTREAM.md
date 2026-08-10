# rsh-flow-components 来源记录

- 来源仓库：`参考代码/rsh-flow-components`
- 规范远端：`git@192.168.1.155:rsh-cloud/portal/rsh-flow-components.git`
- 规范分支：`master`
- 当前基线：`bff4ef8b938db5578c3f7eab1f482a4e9388917c`
- 上游原样区：`form-runtime/upstream/`
- 本地适配层：`form-runtime/src/`
- 目标 FormMaking 运行包：`form-runtime/vendor/form-making/`

同步前必须先由 `make refs-sync`/`make refs-status` 保证参考仓库远端、分支、HEAD 和工作树正确。维护任务 API 不接受任意路径、分支或命令；清单精确排除参考仓库中含凭证的 `.npmrc`、`.git`、`node_modules` 和构建产物。

当前同步直接镜像已经独立化的 `rsh-flow-components` 仓库，因此旧 V2 从目标业务应用抽取 35 个路径的清单不再适用；其“显式映射、精确校验、本地适配保护”语义由当前 `sync-manifest.json` 保留，且覆盖完整 tracked `src/`，不会遗漏目标组件源码。

更新基线时必须同时评审参考仓库 HEAD、同步清单、upstream 差异、FormMaking vendor 来源和本地适配兼容性；不得只修改提交号绕过 `SYNC_CHECK`。
