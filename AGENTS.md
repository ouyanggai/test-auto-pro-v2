# 项目协作约束

## 开始工作前

1. 阅读 `CONTEXT.md`、当前功能文档及其直接关联的项目文档。
2. 在修改前写明本次完成标准。
3. 一次只实施一个已获用户批准的功能切片，不自动开始下一项。

## 项目文档

- `docs/PRODUCT.md`：产品行为唯一来源。
- `docs/ARCHITECTURE.md`：技术选型与代码边界唯一来源。
- `docs/ROADMAP.md`：阶段全景以及当前、后两项工作。
- `docs/PROGRESS.md`：仅记录当前状态、阻塞和下一步。
- `docs/features/`：单项功能范围、验收与人工检查点。
- `重构指南/`：只作为历史输入；与现行文档冲突时不得采用。
- `参考代码/`：目标平台行为的代码依据，不进入 Git。

## 功能状态与人工门禁

正常推进顺序为：

`preparing -> awaiting_approval -> implementing -> ready_for_manual -> accepted`

- 只有用户明确批准后才能进入 `implementing`。
- 实施与自动验证完成后停在 `ready_for_manual`。
- 只有用户明确验收后才能进入 `accepted`。
- 到达 `ready_for_manual` 后禁止自动开始下一功能。
- 用户明确反馈人工验收未通过时，当前功能允许从 `ready_for_manual` 返回 `implementing`；必须记录反馈、重新验证，并再次停在 `ready_for_manual`。

## 默认技能

后续默认只使用以下项目本地技能：

- `.agents/skills/plan-feature-slice`
- `.agents/skills/implement-feature-slice`
- `.agents/skills/diagnose-one-issue`
- `.agents/skills/review-feature-slice`

不得默认启用 Matt 全套、全局工作流、严格 TDD、issue/ticket 或 `.scratch` 体系。只有用户明确指定或系统强制时，才可使用其他技能；适用时必须说明原因并把影响限制在当前任务内。

## 实施与验证

- 使用最简单、可运行的实现，不做未获批准的架构、基础设施或数据库调整。
- 新增或修改的代码注释使用中文；同步的参考源码保持原样。
- 测试必须按类别保存在根目录 `test/`，只运行当前功能相关测试。
- 代码必须实际执行；视觉结果由用户手工在浏览器核对，不自动启动浏览器。
- 每个原子成果验证通过后提交一次；所有提交说明使用中文。
- 不提交凭证、缓存、依赖目录、构建产物、参考源码、PID 或日志。

## 参考代码边界

- 参考仓库只能通过 `make refs-sync` 安全同步；禁止用 reset、checkout 或复制脏工作树覆盖。
- `rsh-cloud-invest-power-system` 后续业务分析只查看 `GroupApproveManage` 及其直接引用的公共组件。
- V1 只可参考已验证交互经验，不是业务规则来源，不复制旧业务代码。
