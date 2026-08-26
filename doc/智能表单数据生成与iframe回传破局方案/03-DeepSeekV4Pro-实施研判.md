# DeepSeek V4 Pro：实施与验证研判

## 核心建议

DeepSeek V4 Pro 建议以“统一字段模型 + 约束传播 + 有界回溯 + 运行快照”拆成可验证切片，但交叉评审后收敛为：IR 必须从现有 `Field`、`Constraint` 和 `pathFormSolveVariable` 细化而来，不另造长期并行系统；字段依赖传播先作为候选域构造的扩展，不急于引入完整通用 CSP。

## 推荐的统一数据模型

统一字段至少包括：规范化路径、中文名称、来源（FormMaking/Vue）、值形态（scalar/array/object/json_string/file_ref）、required、只读/隐藏、静态选项、候选来源、组件 capability、校验能力、集合根和证据。

统一约束至少包括：左字段、操作符、右常量或字段引用、来源（路径/模板/依赖/组件/日期）、条件组、可解释文本、是否阻塞。`Constraint.ValueField`、`Avoid`、`DateRangeBinding` 等既有结构应先被纳入该契约，再决定是否重命名。

## 分层求解

```text
1. 读取一次规则快照、路径选择、身份和当前账号候选
2. 构造有限域：真实选项、级联完整路径、身份节点、候选对象、稳定安全默认值
3. 传播常量约束：范围交集、枚举过滤、必填与可编辑边界
4. 传播可证明字段依赖：有向无环按拓扑顺序；环或动态脚本降级为人工核对
5. 用现有 20000 次确定性搜索处理剩余变量
6. 用同一规则复验当前完整路径，返回 values、生成所有权、问题和复验结果
```

不要把 AC-3 当成第一阶段硬依赖。只有 P0 统计证明当前真实模板需要更强域传播时，才评估外部 CSP；不能因为算法名称先进就增加运行时依赖。

## FormMaking、Vue 和运行时

- FormMaking：真实 `fm-generate-form` 渲染，`getData(true)` 通过后调用 `getValues()`，必须保留虚拟字段和宿主辅助字段。
- `vue_custom`：规则目录确认入口后，真实 `HostVuePage` 加载组件，字段路径读写和 Element 表单校验必须可观测；字段声明与实例状态不一致时返回 partial/blocked。
- 自定义组件：新增组件只需登记 capability 和候选读取适配，不改求解核心；外部对象没有当前账号候选时不造假值。

## 后端与未来运行数据

当前 `test_execution_path_configs` 已保存完整 `form_values`、seed、生成/人工字段、模板版本、修订号和幂等键。先利用这些字段完成配置闭环，不为了“看起来完整”提前创建运行快照表。

未来真实执行开始时，再从已确认配置复制不可变 `RunInputSnapshot`（包含 values、renderType、templateRuleVersion、shapeDigest、来源路径和账号引用；是否包含 SID 由运行记录排障设计决定），由 `internal/adapter/target` 按 FormMaking 或 Vue 的真实提交协议编译目标请求体。编译不在 iframe，不执行未知动态协议；编译失败则运行前阻断并给出具体字段/协议问题。

## 建议切片与验证

1. P0 契约基线：字段/约束/候选矩阵、golden 表单、消息往返、请求影子报告；现有行为零变化。
2. P1 IR：区间/枚举/依赖传播、结构化冲突原因；回退输出必须同一 IR 复验。
3. P2 Vue/组件契约：值形态、宿主读写失败和外部候选隔离；FormMaking 与 Vue 各一条真实黄金样例。
4. P3 iframe/只读请求：完整 JSON、虚拟字段、旧消息拒绝、规则清单默认拒绝和 SID 泄漏断言。
5. P4 运行输入编译：只做预检和快照，不做目标提交；通过后再单独申请真实运行切片。
