# 参考代码清单

本文件由 `make refs-sync` 生成。`参考代码/` 被当前 Git 仓库忽略；同步只允许首次干净克隆或对正确分支执行 `pull --ff-only`。

| 仓库 | 本地目录 | 远端 | 分支 | HEAD | 同步时间 |
| --- | --- | --- | --- | --- | --- |
| `rsh-cloud-gateway` | `参考代码/java-serve/rsh-cloud-gateway` | `http://192.168.1.155/rsh-cloud/cloud-server-full/rsh-cloud-gateway.git` | `chenqiuyu` | `79dda259f48df7b625cabf0bb5c62d0ae9759413` | 2026-07-27 12:43:04 +0800 |
| `rsh-cloud-measuring-center` | `参考代码/java-serve/rsh-cloud-measuring-center` | `git@192.168.1.155:rsh-cloud/cloud-server-full/rsh-cloud-measuring-center.git` | `master` | `da4d27ee596a507ba9e61a78de11e48d96d696b2` | 2026-07-27 12:43:04 +0800 |
| `rsh-cloud-measuring-center-api` | `参考代码/java-serve/rsh-cloud-measuring-center-api` | `git@192.168.1.155:rsh-cloud/cloud-server-full/rsh-cloud-measuring-center-api.git` | `master` | `e39c888dcf70da38533a2ac3834c0ee93774f3f1` | 2026-07-27 12:43:04 +0800 |
| `rsh-cloud-user-center` | `参考代码/java-serve/rsh-cloud-user-center` | `http://192.168.1.155/rsh-cloud/cloud-server-full/rsh-cloud-user-center.git` | `master` | `a3c6bd520e107c19a93071b7b25ef09eddb8b306` | 2026-07-27 12:43:04 +0800 |
| `rsh-cloud-user-center-api` | `参考代码/java-serve/rsh-cloud-user-center-api` | `http://192.168.1.155/rsh-cloud/cloud-server-full/rsh-cloud-user-center-api.git` | `master` | `eac9358e11d5505c0446a53b7aa789f9d19c74fd` | 2026-07-27 12:43:04 +0800 |
| `rsh-cloud-web-api` | `参考代码/java-serve/rsh-cloud-web-api` | `http://192.168.1.155/rsh-cloud/cloud-server-full/rsh-cloud-web-api.git` | `master` | `4b8f881469fa7f0e33519ae00cb4e8c5f285f251` | 2026-07-27 12:43:04 +0800 |
| `rsh-cloud-workflow-center` | `参考代码/java-serve/rsh-cloud-workflow-center` | `git@192.168.1.155:rsh-cloud/cloud-server-full/rsh-cloud-workflow-center.git` | `test` | `c86990e3cee8b8967c6cabf7bfeb419f40509c65` | 2026-07-27 12:43:04 +0800 |
| `rsh-cloud-workflow-center-api` | `参考代码/java-serve/rsh-cloud-workflow-center-api` | `git@192.168.1.155:rsh-cloud/cloud-server-full/rsh-cloud-workflow-center-api.git` | `master` | `e65b095ca31f347e087690c71003af32b1b899bb` | 2026-07-27 12:43:04 +0800 |
| `rsh-framework-all` | `参考代码/rsh-framework-all` | `git@192.168.1.155:rsh-cloud/cloud-framework/rsh-framework-all.git` | `test` | `0c31d87415e7ffd8fd21036e7c361dbee72f4cbc` | 2026-07-27 12:43:04 +0800 |
| `rsh-cloud-invest-power-system` | `参考代码/rsh-cloud-invest-power-system` | `git@192.168.1.155:rsh-cloud/cloud-web/business-system/rsh-cloud-invest-power-system.git` | `test` | `824a7092176412261185e3986c17a277f6ec7dea` | 2026-07-27 12:43:04 +0800 |
| `rsh-flow-components` | `参考代码/rsh-flow-components` | `git@192.168.1.155:rsh-cloud/portal/rsh-flow-components.git` | `master` | `bff4ef8b938db5578c3f7eab1f482a4e9388917c` | 2026-07-27 12:43:04 +0800 |
| `rsh-cloud-vue-form-making` | `参考代码/rsh-cloud-vue-form-making` | `http://192.168.1.155/rsh-cloud/cloud-web/cloud-system/rsh-cloud-vue-form-making.git` | `oygdev` | `f7470d1f2f9ece31cd309fdb94edf0c2318f003e` | 2026-07-27 12:43:04 +0800 |
| `rsh-cloud-saas-implementation-web` | `参考代码/rsh-cloud-saas-implementation-web` | `http://192.168.1.155/rsh-cloud/cloud-web/cloud-system/rsh-cloud-saas-implementation-web` | `test` | `93698f2a18cfc3f3346c7ee7150518d980512f0c` | 2026-07-27 12:43:04 +0800 |

## 使用边界

- 目标平台真实代码和运行结果是业务规则依据。
- `rsh-cloud-invest-power-system` 只分析 `GroupApproveManage` 及其直接引用公共组件。
- 任何脏目录、分支不符、分叉或远端错误都会中止同步；禁止 reset 或 checkout 覆盖。
