# Claude Code 排查任务：表单运行时 Loading 长时间不关闭

请在 `/Volumes/oygsky/AIstudy/test-auto-pro-v2` 直接排查并修复下面的问题。不要只给分析或建议，确认根因后直接修改代码、补测试、执行验证，并提交中文 commit。

## 现象

在路径配置页打开「表单数据」工作区后，页面被半透明 loading 遮罩覆盖，文案为：

> 正在加载表单组件和远程选项

遮罩会长时间停留，用户看不到或无法操作表单。用户要求：

1. Loading 位于页面顶部工具栏下方，不遮住顶部操作栏。
2. 表单加载成功、失败、超时、切换路径、切换历史数据和销毁 iframe 会话时，Loading 都必须关闭。
3. 目标接口不返回时不能无限等待；错误必须明确指出具体阶段或接口，不能只提示「请重新确认」等模糊文案。
4. 表单分类等关键路径数据必须在表单结构、历史数据、组件初始化和远程选项请求完成后再覆盖；迟到的宿主 `editData`/数据源响应不能把修正后的值覆盖回旧值。
5. 最终修正阶段应显示「正在自动修正关键路径数据」。

## 当前上下文

此前已经有 F-024 的异步分类修正和 Loading 收敛改动，不能直接回退。当前重点不是重新设计人员配置，也不是修改目标平台业务规则，而是找出 Loading 仍然卡住的真实状态链路。

优先阅读：

- `AGENTS.md`
- `CONTEXT.md`
- `docs/PRODUCT.md`
- `docs/ARCHITECTURE.md`
- `docs/features/F-024-node-field-power-staged-filling.md`
- `web/src/views/PlanPathConfigurationView.vue`
- `web/src/features/path-configuration/FormRuntimeFrame.vue`
- `form-runtime/src/App.vue`
- `form-runtime/src/runtime/formTemplate.js`
- `form-runtime/src/runtime/requestPolicy.js`
- `test/unit/frontend/form_runtime_test.mjs`

当前工作区可能有用户尚未提交的 `web/src/views/PlanPathConfigurationView.vue` 修改，必须保留并与之协作，禁止使用 `git reset`、`git checkout` 或其他方式覆盖无关改动。

## 排查要求

请先建立完整的异步调用链，再修改：

1. 从父页面 `openFormWorkspace`、iframe `load` 命令、`FormMaking.refresh()`、数据源请求、选项协调到 `ready`/`error` 消息逐段确认谁负责结束 Loading。
2. 确认 `XMLHttpRequest` 和 `fetch` 的请求计数是否一定会在成功、HTTP 错误、网络错误、Abort、同步抛错和组件销毁时归零。重点检查 `loadend` 监听是否覆盖真实请求实现，以及请求计数是否可能因重复包装、旧会话或异常路径泄漏。
3. 确认等待 Promise 是否存在永不 resolve/reject 的分支；所有等待都必须有明确的总超时、取消和会话代次判断。
4. 确认父页面的整体超时是否真的能解除遮罩，而不是只改变错误状态后被后续异步回调重新打开。
5. 确认 `load` 命令的异常、取消和 superseded 分支不会因为提前 return 遗留 `loading=true`。
6. 确认生产页面实际加载的 `form-runtime` 构建产物与源码一致；不要只修源码而漏掉同步/构建入口。
7. 保留“先等待所有表单接口，再执行分类等关键路径修正”的语义。若当前实现为了等待远程选项而重复刷新导致长时间卡住，应改为有界、可诊断的状态转换，而不是简单删除修正逻辑。

## 实现约束

- 只处理本任务涉及的 Loading、异步请求屏障、会话取消/超时、关键路径最终修正和其直接测试；不要顺手重构无关模块。
- 不改数据库、迁移、目标平台权限或写接口语义。
- 不保留旧兼容层或无限等待兜底。
- 用户可见错误必须包含阶段和具体请求路径（不得包含 SID、请求正文或凭证）。
- Loading 的位置使用稳定布局约束，不能因文案或计数变化导致页面跳动。
- 新增或修改的代码注释使用中文；新增测试放在根目录 `test/` 的对应分类下。

## 完成标准

- [ ] 找到并在交付说明中明确写出导致 Loading 卡住的实际根因。
- [ ] Loading 在成功、失败、超时、取消、旧会话回调和 iframe 销毁后均能关闭。
- [ ] Loading 位于顶部操作栏下方，不覆盖顶部操作按钮。
- [ ] 请求屏障对 XHR/fetch 的成功、失败、Abort、同步异常均能归零；超时后返回具体待处理路径。
- [ ] 分类等关键路径补丁只在初始化请求和宿主二次刷新稳定后执行；最终修正不会被迟到响应覆盖。
- [ ] 页面在等待期间显示阶段性中文文案，最终修正显示「正在自动修正关键路径数据」。
- [ ] 错误提示可定位到具体阶段/接口，不出现模糊的「重新确认」类提示。
- [ ] 相关定向测试覆盖至少：请求成功归零、请求失败归零、Abort/销毁、超时、旧会话回调、加载成功关闭 Loading、加载失败关闭 Loading、最终分类修正不被迟到响应覆盖。
- [ ] 执行并记录：

  ```bash
  node --experimental-strip-types --test test/unit/frontend/form_runtime_test.mjs
  npm --prefix form-runtime run typecheck
  npm --prefix form-runtime run build
  ```

- [ ] 如修改了 Web 端，同时执行 Web 端现有类型检查/构建入口。
- [ ] 每个原子修改使用中文 commit message；最终停在 `ready_for_manual`，不要代替用户做浏览器验收。

## 交付说明格式

完成后请只汇报以下内容：

1. 实际根因（一句话，指出具体状态或调用链）。
2. 修改的文件和关键行为变化。
3. 执行过的测试/构建命令及结果。
4. 中文 commit hash。
5. 用户需要手工验证的最短步骤：打开路径配置页 -> 表单数据 -> 等待或模拟接口异常，确认 Loading 能关闭且错误能定位。

