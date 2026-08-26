# F-011 智能表单统一契约基线

- 状态：ready_for_manual
- 批准时间：2026-08-26
- 方案依据：`doc/智能表单数据生成与iframe回传破局方案/05-统一可执行总体方案.md`
- 切片依据：`doc/智能表单数据生成与iframe回传破局方案/06-实施切片与验收清单.md` 的 P0
- 关联功能：F-009、F-010、F-010R

## 本轮范围

本切片只建立“什么是当前正确行为”的可执行基线，不实现统一 IR，不替换现有生成器和有界求解器，不切换请求默认拒绝策略，也不新增目标平台写操作。

- 固定一份 FormMaking 与 `vue_custom` 字段契约样例，覆盖标准字段、级联完整路径、子表单行、JSON 字符串自定义组件、当前账号候选和身份字段。
- 固定 `eq/neq/gt/gte/lt/lte/contains/in` 八个现有操作符的接受与拒绝语义。
- 固定路径无解时的问题基线，至少保留字段、约束原因和阻断等级。
- 冻结 `f007-form-runtime/v1` 的七种命令及 boot、state、result、error 响应，验证完整 values、虚拟字段、旧版本、旧会话、迟到请求和一次性消费边界。
- 对现有 `requestPolicy.js` 增加不改变判定结果的影子观察，只记录方法、无查询串路径、渲染/组件上下文、判定和原因；不记录 SID、请求正文或业务响应。
- 使用受控清单量化当前启发式策略缺口，未达到切换门槛前继续保持现有策略。

## 完成标准

- [x] FormMaking、`vue_custom`、JSON 字符串组件、级联/集合与冲突样例可重复回放。
- [x] 八个现有操作符均有正反例，F-009 求解相关回归保持通过。
- [x] iframe 全命令和响应 golden 通过，旧 origin/source/session/request 不能污染当前表单。
- [x] `validateAndGetValues` 保持嵌套数组、JSON 字符串和虚拟字段。
- [x] 影子记录不含 SID、查询参数、正文和业务响应。
- [x] 当前受控请求清单报告明确列出覆盖率和漏判，未提前切换默认拒绝。
- [x] F-009、F-010 相关自动门禁通过。
- [x] 代码已实际执行，状态停在 `ready_for_manual`。

## 实施结果

### 字段与约束基线

- `test/fixtures/f011_smart_form_contract_baseline.json` 保存无敏感信息的两类表单样例。
- `test/unit/backend/f011_contract_baseline_test.go` 验证字段解析、值形态、生成、同规则复验、八操作符和路径冲突问题。
- 测试路径快照补齐无凭证运行时会话，使 F-009 服务级求解回放实际经过当前生成入口。

### iframe 与请求基线

- `test/fixtures/f011_iframe_protocol_golden.json` 冻结 v1 消息及受控请求清单。
- `web/src/features/path-configuration/runtimeProtocol.ts` 把父页现有消息判定收敛成可执行纯函数；真实 origin/source 仍在 iframe 监听器中核对。
- `form-runtime/src/runtime/protocol.js` 增加运行时响应结构校验。
- `form-runtime/src/runtime/requestPolicy.js` 在保持现有放行结果的前提下返回判定原因，并支持有界内存影子观察。
- 当前受控清单含 3 个只读和 2 个写请求：2 个只读命中、1 个只读未命中、1 个写请求被阻断、1 个写请求因只读后缀被误判。该样例证明当前词表尚不满足切换条件，只作为 P1/P3 输入，不能代表全部真实模板覆盖率。

## 明确未做

- 不新增 `ConstraintIR`、依赖传播、区间求交或通用 CSP。
- 不改变生成、保存、批量准备和目标请求的公开 API。
- 不把清单为空的模板直接切成默认拒绝。
- 不记录或持久化 SID、目标请求正文、响应正文和源码全文。
- 不开始 P1、P2、P3、P4。

## 验证与人工门禁

自动验证入口为 `test/run-f011.sh`。人工核对见 `test/manual/F-011.md`。人工确认前不得进入 `accepted`，也不得自动开始 P1。
