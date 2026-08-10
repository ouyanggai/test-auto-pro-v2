# rsh-flow-components 副本边界

- 来源仓库：`参考代码/rsh-flow-components`
- 来源提交：`bff4ef8b938db5578c3f7eab1f482a4e9388917c`
- FormMaking 运行包：来源副本同步规则对应的 `form-making-advanced 1.6.12`，保存在 `src/lib/vue-form-making/dist/`。

本目录保留来源组件代码以便逐项建立独立适配，但生产入口已替换为 F-007 表单运行时桥接。旧工作台路由、Vuex、认证页和流程提交页没有进入运行时入口；iframe 仅通过版本化 `postMessage` 接收主应用会话。

当前会话 SID 只保存在 iframe 请求策略闭包中，用于目标表单组件的必要只读请求。销毁工作区后立即恢复网络对象并清空 SID；不会写入 localStorage、sessionStorage、配置数据库或日志。所有目标流程提交、草稿和已知业务写端点在运行时层继续阻断。
