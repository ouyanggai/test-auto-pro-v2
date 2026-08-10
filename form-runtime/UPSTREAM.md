# rsh-flow-components 来源与同步边界

- 来源仓库：`参考代码/rsh-flow-components`
- 规范远端：`git@192.168.1.155:rsh-cloud/portal/rsh-flow-components.git`
- 规范分支：`master`
- 当前落库基线：`bff4ef8b938db5578c3f7eab1f482a4e9388917c`
- 实际运行源码：`form-runtime/runtime-source/`
- 本地适配层：`form-runtime/src/`、`build/`、根构建配置和同步适配脚本
- 目标 FormMaking：`runtime-source/src/lib/vue-form-making/dist/`

参考仓库只能先由 `make refs-sync`/`make refs-status` 保证规范远端、`master` 和干净工作树。维护 API 不接受任意路径、分支、HEAD 或命令；每个任务把创建时的当前 HEAD 持久化，Worker 在同步前复核同一快照。参考仓库后续安全快进不需要修改代码中的历史 HEAD。

同步清单覆盖完整 tracked `src/`、`public/`、原生 `scripts/`/`sync-manifest.json` 和构建资产，排除 `.git`、`.npmrc`、依赖、构建产物和凭证。同步目标的完整摘要会保护未知修改；本地 iframe/SID/写阻断适配位于清单保护区，候选同步不能覆盖。

`runtime-source/` 本身保留上游原生 35 映射与 sync-check 资产，便于追踪 `rsh-flow-components` 的生成来源；当前项目的维护流水线只从已核实的 `rsh-flow-components` 参考仓库同步，不直接改写或跨过 `make refs-sync` 操作参考代码。
