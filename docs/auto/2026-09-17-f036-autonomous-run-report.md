# F-036 夜间自主施工晨报（2026-09-16 23:00 ～ 2026-09-17 02:00）

## 一、结论

**能用的**：S2 独立执行服务的 FormMaking 原生页面装载与六个动作（保存草稿、提交、审批暂存、同意、
不同意、重提）已用真实页面 + 真实目标写打通；手动分支选择闭环（choice_required → 继续一次 →
多一笔写 → 成功）已打通。每条证据都含「状态=success + 期望写端点 + 传输层 trace + 写后核验」。

**不能用的**：转发（M6，可选）未验证——无已配置路径，且实例预算（施工令 §0：≤12）已用满。
NoFormFlow/Vue 自定义页按 S8 方案 B 收窄回退 Go 直连路径（范围变更，待确认）。

**整体是否达到 DoD 五条**：达到 4.5/5——
1. 七个动作真实证据：六/七个有（转发缺）✅（按可验证范围）
2. 装载不再 500、发起精确命中绑定模板 ✅（证据 L1-initiate）
3. `bash test/run-executor-service.sh` 全绿且含真实闭环用例 ✅
4. 手动分支 choice_required → 继续一次 → 只多一笔写 ✅（014655 证据）
5. 功能文档回到 ready_for_manual、逐项结论与证据一致、晨报/核对步骤齐 ✅

## 二、逐动作证据

| 动作 | 验证台 | 实例/任务 | 写端点 | 结果 | 证据文件 |
| --- | --- | --- | --- | --- | --- |
| 装载-发起 | L1 | —（纯读） | 无写（list+findById） | ready，hostPath=[InitFlowDialog,FlowDialog]，模板精确命中 | 20260917-001606-L1-initiate.json |
| 装载-详情(view) | L1 | bc847fa1 前身（纯读） | 无写 | ready，组件链含 EnterpriseExamineDialog | 20260917-001609-L1-detail-view.json |
| 发起-坏模板阻塞 | L1 | —（零写） | 无写 | 409 blocked | 20260917-001724-L2-initiate-bad-template-blocked.json |
| 保存草稿 | L2 | 新实例（draft） | /web/flowInstanceApi/submit | success + 实例核验 | 20260917-004421-L2-save_draft.json |
| 提交 | L2 | bbfbbac5→run（待办生成） | /web/flowInstanceApi/submit | success + 实例核验 | 20260917-010000-L2-submit.json |
| 提交（自审） | L2 | 06c4a0d8→run | /web/flowInstanceApi/submit | success | 20260917-010521-L2-submit.json |
| 审批暂存 | L2 | bc847fa1@审核人1 | /web/flowInstanceApi/storageFormData | success + 暂存记录核验 | 20260917-012011-L2-storage_form_data.json |
| 同意 | L2 | bc847fa1@审核人1 | /flowInstanceApi/audit | success + taskDoneOnNode | 20260917-012818/012948-L2-approve.json |
| 不同意 | L2 | cd9c8b01@审核人1/2 | /flowInstanceApi/audit | success + 实例驳回发起人 | 20260917-013213-L2-reject.json |
| 重提 | L2 | cd9c8b01/bc847fa1（待发→run） | /web/flowInstanceApi/reSubmit | success | 20260917-011945/013156-L2-resubmit.json |
| 选择闭环（手动分支） | L2 | cd9c8b01@审核人1 | 第1笔 audit（choice_required）→ 第2笔 audit 成功 | success | 20260917-014655-L2-approve.json |
| 转发（M6 可选） | — | 未验证（预算满） | — | 未验证 | — |

计划 29/27 的 L3 全链路运行（run 126/127/128/129/130）暴露并修复了三个集成缺陷
（旧后端代码、新实例 ID 回传、发起全量表单值）；run 130 第 1 步已可正常核验推进。

## 三、代码改动与提交（按时间序）

- M0：L1/L2 真实闭环验证台、Go `flowTemplateId` 专字段与 edit/view 装载模式
- S1/S2：装载捕获改 `window.$vue` 原型、就绪判据、trace 真实候选选择、`/load` 诊断、转发体 SID 信封修复
- S2 验收：坏模板 ID 阻塞用例
- S4/S7：nativeDispatch 生产分发、完成判定端点表修正、权限校验排除、页面异常口径、`/api` 前缀写端点判定
- S5/S7：补丁页面内一次完成、宿主轮询、全量值补丁、新实例 ID 回传（businessSummary→MainInstanceRef）
- S6/M4/M5：choice_required/choice_continue 全链路、finalize 选择拒绝 supersede、Execute 结果按 requestId 关联
- S8/M7：范围收敛 FormMaking（方案 B）、native-dispatch 抽取与三旧测试重写、聚合脚本纳入真实验证台

## 四、未解决项与原因

1. **转发未验证（M6，可选）**：无已配置转发路径；实例预算 ≤12 已用满（7 草稿 + 3 提交 + 计划27 产生 2 个）。
2. **S8 方案 B 是范围变更**：runtime 不再接 NoFormFlow/Vue 自定义页（回退 Go 直连）。**需用户确认**；
   若要求方案 A 请反馈另立切片。
3. **计划 27（oyg测试1003）路径配置含两个 save_draft 动作**：与引擎「路径独占主实例」模型冲突，
   第 2 步被正确拦截停止（属计划配置问题，非引擎缺陷，建议用户调整该计划）。
4. **工具侧 cosmo**：`RUNTIME_EXEC_VERIFICATION=1` 仍在 .env.local，本轮未依赖其放行。

## 五、需要用户做什么

- [ ] 看 `docs/features/F-036-independent-executor-native-page.md` 的「次日人工核对步骤」并执行
- [ ] 确认 S8 方案 B（runtime 仅 FormMaking）的范围口径，或提出方案 A 需求
- [ ] 目标平台清理 `T-AUTO-F036-*` 测试实例（草稿 7 个 + run 3 个，清单见核对步骤 3）
- [ ] 调整计划 27「oyg测试1003」路径（去掉重复的保存草稿动作）后再跑
- [ ] 人工验收通过后将功能文档状态改为 accepted

## 六、日志与证据索引

- 证据：`docs/auto/evidence/`（M0 起点失败证据 + M1 装载 + M4/M5 动作族 + 里程碑汇总）
- 执行服务日志：`/tmp/f036-executor.log`；后端日志：`/tmp/f036-backend.log`（当夜临时路径）
- 运行日志：`logs/runs/运行_1__oyg测试1003__run-129`、`运行_2__oyg测试1003__run-130` 等
- 起点对照：`docs/auto/2026-09-16-f036-second-review-repair-task.md` §附录B（八个 P1 基线）
