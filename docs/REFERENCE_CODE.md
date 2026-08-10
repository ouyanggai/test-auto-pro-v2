# 参考代码清单

本文件由 `make refs-sync` 生成。`参考代码/` 被当前 Git 仓库忽略；同步只允许首次干净克隆或对正确分支执行 `pull --ff-only`。

| 仓库 | 本地目录 | 远端 | 分支 | HEAD | 同步时间 |
| --- | --- | --- | --- | --- | --- |
| `rsh-cloud-gateway` | `参考代码/java-serve/rsh-cloud-gateway` | `http://192.168.1.155/rsh-cloud/cloud-server-full/rsh-cloud-gateway.git` | `chenqiuyu` | `79dda259f48df7b625cabf0bb5c62d0ae9759413` | 2026-07-28 11:57:48 +0800 |
| `rsh-cloud-measuring-center` | `参考代码/java-serve/rsh-cloud-measuring-center` | `git@192.168.1.155:rsh-cloud/cloud-server-full/rsh-cloud-measuring-center.git` | `master` | `da4d27ee596a507ba9e61a78de11e48d96d696b2` | 2026-07-28 11:57:48 +0800 |
| `rsh-cloud-measuring-center-api` | `参考代码/java-serve/rsh-cloud-measuring-center-api` | `git@192.168.1.155:rsh-cloud/cloud-server-full/rsh-cloud-measuring-center-api.git` | `master` | `e39c888dcf70da38533a2ac3834c0ee93774f3f1` | 2026-07-28 11:57:48 +0800 |
| `rsh-cloud-user-center` | `参考代码/java-serve/rsh-cloud-user-center` | `http://192.168.1.155/rsh-cloud/cloud-server-full/rsh-cloud-user-center.git` | `master` | `a3c6bd520e107c19a93071b7b25ef09eddb8b306` | 2026-07-28 11:57:48 +0800 |
| `rsh-cloud-user-center-api` | `参考代码/java-serve/rsh-cloud-user-center-api` | `http://192.168.1.155/rsh-cloud/cloud-server-full/rsh-cloud-user-center-api.git` | `master` | `eac9358e11d5505c0446a53b7aa789f9d19c74fd` | 2026-07-28 11:57:48 +0800 |
| `rsh-cloud-web-api` | `参考代码/java-serve/rsh-cloud-web-api` | `http://192.168.1.155/rsh-cloud/cloud-server-full/rsh-cloud-web-api.git` | `master` | `4b8f881469fa7f0e33519ae00cb4e8c5f285f251` | 2026-07-28 11:57:48 +0800 |
| `rsh-cloud-workflow-center` | `参考代码/java-serve/rsh-cloud-workflow-center` | `git@192.168.1.155:rsh-cloud/cloud-server-full/rsh-cloud-workflow-center.git` | `test` | `c86990e3cee8b8967c6cabf7bfeb419f40509c65` | 2026-07-28 11:57:48 +0800 |
| `rsh-cloud-workflow-center-api` | `参考代码/java-serve/rsh-cloud-workflow-center-api` | `git@192.168.1.155:rsh-cloud/cloud-server-full/rsh-cloud-workflow-center-api.git` | `master` | `e65b095ca31f347e087690c71003af32b1b899bb` | 2026-07-28 11:57:48 +0800 |
| `rsh-framework-all` | `参考代码/rsh-framework-all` | `git@192.168.1.155:rsh-cloud/cloud-framework/rsh-framework-all.git` | `test` | `0c31d87415e7ffd8fd21036e7c361dbee72f4cbc` | 2026-07-28 11:57:48 +0800 |
| `rsh-cloud-invest-power-system` | `参考代码/rsh-cloud-invest-power-system` | `git@192.168.1.155:rsh-cloud/cloud-web/business-system/rsh-cloud-invest-power-system.git` | `test` | `0ce1584f6829e16cdbc6df8b34f98ee4aa340710` | 2026-07-28 11:57:48 +0800 |
| `rsh-flow-components` | `参考代码/rsh-flow-components` | `git@192.168.1.155:rsh-cloud/portal/rsh-flow-components.git` | `master` | `bff4ef8b938db5578c3f7eab1f482a4e9388917c` | 2026-07-28 11:57:48 +0800 |
| `rsh-cloud-vue-form-making` | `参考代码/rsh-cloud-vue-form-making` | `http://192.168.1.155/rsh-cloud/cloud-web/cloud-system/rsh-cloud-vue-form-making.git` | `oygdev` | `f7470d1f2f9ece31cd309fdb94edf0c2318f003e` | 2026-07-28 11:57:48 +0800 |
| `rsh-cloud-saas-implementation-web` | `参考代码/rsh-cloud-saas-implementation-web` | `http://192.168.1.155/rsh-cloud/cloud-web/cloud-system/rsh-cloud-saas-implementation-web` | `test` | `71e10641df7ab82381eb6efd60ad2533f5f7d506` | 2026-07-28 11:57:48 +0800 |

## 使用边界

- 目标平台真实代码和运行结果是业务规则依据。
- `rsh-cloud-invest-power-system` 只分析 `GroupApproveManage` 及其直接引用公共组件。
- 任何脏目录、分支不符、分叉或远端错误都会中止同步；禁止 reset 或 checkout 覆盖。
- F-007 的 `form-runtime/upstream/` 固定同步 `rsh-flow-components` 的规范远端、`master` 和 HEAD `bff4ef8b938db5578c3f7eab1f482a4e9388917c`。更新先执行 `make refs-sync`/`make refs-status`，再评审并更新 `form-runtime/sync-manifest.json`；系统设置的一键维护只消费该固定快照，不接受任意来源。
- 上游原样区、本项目 iframe/SID/写阻断适配层和 FormMaking vendor 的边界记录在 `form-runtime/UPSTREAM.md`。维护流水线复用旧 V2 ADR-0016 的固定来源、候选、回退、租约和恢复语义，但以 pnpm 版本目录替代 Docker 镜像。
