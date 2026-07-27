---
name: plan-feature-slice
description: 将用户提出的产品或工程需求收敛为一个可独立验收的功能切片。用于实施前明确范围、完成标准、验证方式、人工检查点，并将功能从 preparing 推进到 awaiting_approval；不用于直接写代码或批量规划后续任务。
---

# 规划功能切片

## 读取上下文

1. 读取 `AGENTS.md`、`CONTEXT.md`、`docs/PRODUCT.md`、`docs/ARCHITECTURE.md`、`docs/ROADMAP.md` 和当前进度。
2. 只在需要核实历史原因时读取 `重构指南/`。
3. 只在需要核实目标平台行为时读取 `参考代码/`；投资系统仅查看 `GroupApproveManage` 及其直接引用公共组件。

## 收敛计划

1. 定义单一用户结果，避免把多个功能绑在一起。
2. 写清包含与不包含的范围，禁止预判式引入基础设施或抽象。
3. 列出可执行的完成标准、分类测试和用户手工核对步骤。
4. 检查产品行为只进入 `docs/PRODUCT.md`，技术决定只进入 `docs/ARCHITECTURE.md`。
5. 创建或更新 `docs/features/F-XXX-*.md`，状态从 `preparing` 推进到 `awaiting_approval`。
6. 更新 `docs/ROADMAP.md` 的当前与后两项，以及 `docs/PROGRESS.md` 的当前状态和下一步。

## 停止条件

- 等待用户明确批准，不写实现代码。
- 不生成 issue、ticket、`.scratch` 或大批后续任务。
- 未获批准时不得进入 `implementing`。
