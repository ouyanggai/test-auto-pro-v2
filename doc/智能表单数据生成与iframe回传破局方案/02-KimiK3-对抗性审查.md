# Kimi K3：对抗性系统审查

## 先纠正三处容易写错的事实

1. `getData(true)` 不在 `form-runtime/src/App.vue` 中直接实现，而在 `form-runtime/src/runtime/formTemplate.js` 的 `captureFormValues` 中封装；方案引用必须指向真实职责。
2. SID 不会“完全不出 iframe”：`requestPolicy.js` 会把 SID 放入发往目标网关的请求头/请求体。正确边界是只进入当前授权的目标请求通道，不进入持久层、日志、主应用长期状态或浏览器长期存储。
3. `protocol.js` 负责消息结构/版本/会话校验，origin/source 校验由 `FormRuntimeFrame.vue` 与运行时消息处理共同完成；不能把两层责任混成一个模块。

## 生成成功但流程走不通的反例

1. FormMaking `required` 通过，但正则或自定义 validator 未进入目录，`getData(true)` 仍失败。
2. 条件字段被生成器满足，但隐藏/禁用字段的业务校验依赖宿主脚本，提交时失败。
3. `custome-info-select`、项目、流程对象等值实际是 JSON 字符串，误存为对象会导致组件回显或目标接口解析失败。
4. 自定义组件写入 `__condition`、`__formPersonId` 等虚拟字段，若只保存可见字段，后续条件或人员接口失效。
5. 组件候选来自当前发起账号；错误使用目录分析账号候选会越权或生成目标账号不可见的对象。
6. 选择了级联叶子值而非完整路径数组，页面可能显示但目标条件比较不成立。
7. 模板已 stale，旧值仍能回显；若不阻断生成/保存，规则变更后路径会悄悄走错。
8. `vue_custom` 字段被写入错误的宿主实例层级，`HostVuePage` 静默找不到字段，回传 JSON 缺键却被当作成功。
9. iframe 销毁后旧 `requestId`、旧 `sessionId` 响应迟到，污染新路径或新账号的表单状态。
10. `requestPolicy.js` 以词汇判断写请求，`commit/confirm/handle/deal/process/finish/audit/sign` 等端点可能漏判；反过来未知只读 POST 也可能被误阻断。
11. “换一组”覆盖了用户人工修改，用户看到的值与最后保存的值不一致。
12. 两个标签页或重复点击使用不同修订号，后写请求覆盖先写结果；缺少幂等和修订核对会出现刷新后状态漂移。
13. iframe 回传了完整 FormMaking 值，但未来执行直接把它当目标接口请求体，遗漏 `formDataMongoVo.data`、身份字段或 Java DTO 映射。
14. `vue_custom` 的动态提交脚本无法静态证明，却被名称猜测成可执行，最终宿主提交协议不匹配。

## 每个反例对应的防线

- 目录把 `required/pattern/validator/动态脚本` 分成可证明、需人工核对、阻断三态；保存必须经过运行时校验和后端同规则复验。
- 组件 capability 固定值形态与虚拟字段；候选缓存键包含账号、流程、模板和规则版本。
- 级联、集合、表格行使用结构化路径；回传只接受 `getValues()` 完整对象。
- stale 规则阻止生成、保存和批量准备，但保留旧值供用户查看。
- `HostVuePage` 对声明字段未读写到时返回结构化 partial/needs_attention，不静默吞掉。
- 消息必须核对 origin、source、版本、会话和请求号；销毁后拒绝迟到响应。
- 只读请求先影子观测再转为按目录清单默认拒绝；清单缺失时保留旧启发式但产生 issue。
- 后端以修订号、幂等键、规则版本和完整路径复验为最终权威，未来运行另做目标协议编译。

## 最小优先级

P0：契约事实矩阵、请求覆盖率、golden message、现有行为回归。

P1：现有 `Field`/`Constraint` 的细化 IR；回退结果必须再次经过同一 IR 复验，不能形成双真相。

P2：显式化 `vue_custom` 值形态、实例读写失败上报和自定义组件六要素。

P3：完善 iframe payload/状态回传与请求清单协商，不为非破坏性变化盲目升 v2。

P4：真实运行前的不可变 RunInputSnapshot 与目标适配转换器；当前不执行任何目标写请求。
