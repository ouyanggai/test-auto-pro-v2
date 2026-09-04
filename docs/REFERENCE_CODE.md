# 参考代码清单

本文件由 `make refs-sync` 生成。`参考代码/` 被当前 Git 仓库忽略；同步只允许首次干净克隆或对正确分支执行 `pull --ff-only`。

| 仓库 | 本地目录 | 远端 | 分支 | HEAD | 同步时间 |
| --- | --- | --- | --- | --- | --- |
| `rsh-cloud-gateway` | `参考代码/java-serve/rsh-cloud-gateway` | `http://192.168.1.155/rsh-cloud/cloud-server-full/rsh-cloud-gateway.git` | `chenqiuyu` | `79dda259f48df7b625cabf0bb5c62d0ae9759413` | 2026-09-04 13:34:09 +0800 |
| `rsh-cloud-measuring-center` | `参考代码/java-serve/rsh-cloud-measuring-center` | `git@192.168.1.155:rsh-cloud/cloud-server-full/rsh-cloud-measuring-center.git` | `master` | `3b8720466e7cebf047800daf7548c54189e6c6b1` | 2026-09-04 13:34:09 +0800 |
| `rsh-cloud-measuring-center-api` | `参考代码/java-serve/rsh-cloud-measuring-center-api` | `git@192.168.1.155:rsh-cloud/cloud-server-full/rsh-cloud-measuring-center-api.git` | `master` | `93bd5e77c367a0702159635398b6ca5a072450ab` | 2026-09-04 13:34:09 +0800 |
| `rsh-cloud-user-center` | `参考代码/java-serve/rsh-cloud-user-center` | `http://192.168.1.155/rsh-cloud/cloud-server-full/rsh-cloud-user-center.git` | `master` | `1d9775b7c570ba76b402c0e34220673c5f74dfb9` | 2026-09-04 13:34:09 +0800 |
| `rsh-cloud-user-center-api` | `参考代码/java-serve/rsh-cloud-user-center-api` | `http://192.168.1.155/rsh-cloud/cloud-server-full/rsh-cloud-user-center-api.git` | `master` | `059bb75ca055ebb760903dd883e64f812f87b889` | 2026-09-04 13:34:09 +0800 |
| `rsh-cloud-web-api` | `参考代码/java-serve/rsh-cloud-web-api` | `http://192.168.1.155/rsh-cloud/cloud-server-full/rsh-cloud-web-api.git` | `master` | `16410b5e731565f64ffabf1e2720616127d6f2de` | 2026-09-04 13:34:09 +0800 |
| `rsh-cloud-workflow-center` | `参考代码/java-serve/rsh-cloud-workflow-center` | `git@192.168.1.155:rsh-cloud/cloud-server-full/rsh-cloud-workflow-center.git` | `master` | `37c01d04eb10ad8720d637cd8690b1440b541374` | 2026-09-04 13:34:09 +0800 |
| `rsh-cloud-workflow-center-api` | `参考代码/java-serve/rsh-cloud-workflow-center-api` | `git@192.168.1.155:rsh-cloud/cloud-server-full/rsh-cloud-workflow-center-api.git` | `master` | `088aed79ad0b7d3cca09f38a0325724a8d685cd7` | 2026-09-04 13:34:09 +0800 |
| `rsh-framework-all` | `参考代码/rsh-framework-all` | `git@192.168.1.155:rsh-cloud/cloud-framework/rsh-framework-all.git` | `test` | `84bb19736a8abf78a7ad71b2f714f0e2aec35758` | 2026-09-04 13:34:09 +0800 |
| `rsh-cloud-invest-power-system` | `参考代码/rsh-cloud-invest-power-system` | `git@192.168.1.155:rsh-cloud/cloud-web/business-system/rsh-cloud-invest-power-system.git` | `test` | `8a00cb9995dfc030d3aa88fe3d2bcc94aabce76f` | 2026-09-04 13:34:09 +0800 |
| `rsh-flow-components` | `参考代码/rsh-flow-components` | `git@192.168.1.155:rsh-cloud/portal/rsh-flow-components.git` | `test` | `24f3a280cd3942cf26644bd278c84c7aa3a8b116` | 2026-09-04 13:34:09 +0800 |
| `rsh-cloud-vue-form-making` | `参考代码/rsh-cloud-vue-form-making` | `http://192.168.1.155/rsh-cloud/cloud-web/cloud-system/rsh-cloud-vue-form-making.git` | `oygdev` | `695b2783e226606548eb86aa4ab84ca7bca99140` | 2026-09-04 13:34:09 +0800 |
| `rsh-cloud-saas-implementation-web` | `参考代码/rsh-cloud-saas-implementation-web` | `http://192.168.1.155/rsh-cloud/cloud-web/cloud-system/rsh-cloud-saas-implementation-web` | `test` | `89947a923e7d204c7e11ba75192183397370fde4` | 2026-09-04 13:34:09 +0800 |

## 使用边界

- 目标平台真实代码和运行结果是业务规则依据。
- `rsh-cloud-invest-power-system` 只分析 `GroupApproveManage` 及其直接引用公共组件。
- 任何脏目录、分支不符、分叉或远端错误都会中止同步；禁止 reset 或 checkout 覆盖。
