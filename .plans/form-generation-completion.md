# 表单智能生成完整能力实现计划

## 目标
补齐表单智能生成的核心欠缺能力，达成以下目标：
1. 支持全部自定义组件的智能生成（外部对象候选管理）
2. Vue 页面保存协议完整验证
3. 规则完整性自检与分级阻断
4. 增量同步自动化触发
5. 批量任务体验优化

## 当前完成度：78%
目标完成度：98%

---

## 阶段一：P0 核心阻断项（目标：85%）

### 任务 1.1：组件候选池管理系统
**优先级**：🔴 P0（最高）
**完成度影响**：+7%
**预计工期**：2天

#### 实现内容

1. **定义候选提供者接口**
   - 文件：`internal/adapter/target/component_candidates.go`
   - 接口：`ComponentCandidateProvider`
   - 方法：
     - `GetMaterialCandidates(ctx, account, flowCode string) ([]MaterialCandidate, error)`
     - `GetProjectCandidates(ctx, account, flowCode string) ([]ProjectCandidate, error)`
     - `GetOrderCandidates(ctx, account, flowCode string) ([]OrderCandidate, error)`
     - `GetFlowListCandidates(ctx, account, flowCode string) ([]FlowListCandidate, error)`

2. **实现候选缓存层**
   - 文件：`internal/service/component_candidate_cache.go`
   - 功能：
     - 按规则版本+账号+组件类型缓存
     - 15分钟 TTL
     - 内存 LRU（最多1000条）
     - 可选 Redis 后端

3. **集成到生成器**
   - 修改文件：`internal/formdata/generator.go`
   - 在 `GenerateInput` 中填充 `ComponentCandidates`
   - 按字段路径映射到具体组件候选

4. **目标适配器实现**
   - 文件：`internal/adapter/target/impl/component_candidates.go`
   - 实现真实 API 调用（材料/项目/订单）
   - 权限过滤逻辑

5. **测试覆盖**
   - 单元测试：候选获取、缓存逻辑
   - 集成测试：端到端生成流程
   - 测试文件：`test/unit/backend/component_candidate_test.go`

#### 验收标准
- [ ] 材料选择组件可生成真实候选值
- [ ] 项目组件按权限过滤
- [ ] 订单组件生成有效对象引用
- [ ] 候选缓存命中率 > 80%
- [ ] 无候选时标记 `partial` 而非 `blocked`

---

### 任务 1.2：Vue 页面保存协议完善
**优先级**：🔴 P0
**完成度影响**：+5%
**预计工期**：1.5天

#### 实现内容

1. **Java 字段错误路径识别**
   - 修改文件：`internal/service/template_catalog.go` 中的 `javaPageRule`
   - 识别模式：
     - `data.errors[].field`（字段级错误）
     - `data.fieldErrors`（字段错误映射）
     - `data.validationErrors`（校验错误列表）
   - 更新 `JavaPageRule.FieldErrors` 字段

2. **Vue 错误回显逻辑分析**
   - 扩展 `vueSubmitRule` 函数
   - 识别 `this.$refs.form.setFieldError` 调用
   - 识别 `this.errors[field]` 赋值
   - 标记已证明的错误路径

3. **保存复验字段映射**
   - 新增函数：`validateVueSubmitResponse`
   - 交叉验证 Vue Submit 与 Java Response
   - 移除"字段级错误路径未证明"问题（如已证明）

4. **测试覆盖**
   - 单元测试：Java 响应解析
   - 集成测试：Vue 保存复验
   - 测试文件：`test/unit/backend/vue_submit_validation_test.go`

#### 验收标准
- [ ] Java 控制器字段错误路径识别率 > 90%
- [ ] Vue 页面错误回显逻辑识别率 > 80%
- [ ] 保存成功后 `Issues` 不包含字段错误提示
- [ ] 保存失败可定位到具体字段

---

## 阶段二：P1 体验提升（目标：92%）

### 任务 2.1：规则完整性自检分级
**优先级**：🟡 P1
**完成度影响**：+4%
**预计工期**：1天

#### 实现内容

1. **定义规则完整性模型**
   - 文件：`internal/model/rule_completeness.go`
   - 结构：
     ```go
     type RuleCompleteness struct {
         Blocking []string // 阻断生成
         Warning  []string // 需处理但不阻断
         Info     []string // 仅供参考
         Readiness string  // ready/partial/blocked
     }
     ```

2. **问题分级逻辑**
   - 修改文件：`internal/service/template_catalog.go`
   - Blocking：
     - "页面入口尚未识别"
     - "表单渲染协议尚未识别"
     - "未知自定义组件"（未注册）
   - Warning：
     - "动态脚本需要人工核对"
     - "数据源无可用记录"
     - "字段级错误路径未证明"
   - Info：
     - "宿主页面缺少渲染标记"

3. **生成前置检查**
   - 修改文件：`internal/service/path_config_workspace.go`
   - 在智能生成前检查 `Readiness`
   - `blocked` 直接返回错误
   - `partial` 允许生成但标记原因

4. **设置页问题聚合**
   - 修改 API：`GET /api/template-catalog/summary`
   - 返回分级问题统计
   - 前端展示 Blocking/Warning/Info 分组

#### 验收标准
- [ ] 阻断问题无法发起智能生成
- [ ] 警告问题可生成但显示提示
- [ ] 设置页清晰展示分级问题
- [ ] 问题修复后自动更新就绪状态

---

### 任务 2.2：Vue 页面表单渲染
**优先级**：🟡 P1
**完成度影响**：+3%
**预计工期**：2天

#### 实现内容

1. **规则到动态表单转换器**
   - 文件：`internal/service/vue_form_renderer.go`
   - 功能：
     - `VueCustomFieldRule` → Element 表单项配置
     - 支持字段类型：input/number/date/select/checkbox
     - 应用 required/readonly/disabled/hidden 规则
     - 绑定静态选项

2. **动态表单渲染 API**
   - 端点：`GET /api/workspace/:id/vue-form-config`
   - 返回：Element Form JSON 配置
   - 前端使用 `el-form` + `el-form-item` 动态渲染

3. **字段联动支持**
   - 根据 `Dependencies` 生成 watch 配置
   - 简单赋值联动（assignment 类型）
   - 复杂联动提示"需真实页面确认"

4. **保存协议代理**
   - 端点：`POST /api/workspace/:id/vue-form-submit`
   - 代理到真实保存接口
   - 拦截响应，提取字段错误
   - 返回标准化错误格式

#### 验收标准
- [ ] 配置式 Vue 页面可动态渲染
- [ ] 必填/只读/隐藏规则生效
- [ ] 静态选项正确显示
- [ ] 简单字段联动可用
- [ ] 保存错误可定位到字段

---

## 阶段三：P2 运维增强（目标：98%）

### 任务 3.1：增量同步自动化触发
**优先级**：🟢 P2
**完成度影响**：+3%
**预计工期**：1.5天

#### 实现内容

1. **定时检查目标模板**
   - 文件：`internal/service/template_catalog_scheduler.go`
   - 功能：
     - 每小时检查模板 `UpdateDate`
     - 对比本地 `SourceVersion`
     - 发现变化自动排队增量分析

2. **Git commit 触发钩子**
   - 文件：`.git/hooks/post-commit`（或 CI/CD）
   - 检查变更文件：
     - `form-runtime/runtime-source/src/**/*.{js,vue}`
     - `参考代码/java-serve/**/*.java`
   - 触发增量分析 API

3. **WebHook 通知支持**
   - 端点：`POST /api/template-catalog/webhook`
   - 接收外部系统通知（目标平台模板更新）
   - 验证签名，触发增量分析

4. **过期规则标记**
   - 新增字段：`TemplateRuleCatalogItem.Stale bool`
   - 检测到变化但未重新分析时标记
   - 工作区使用过期规则时显示警告

#### 验收标准
- [ ] 目标模板更新后 1 小时内触发增量分析
- [ ] Git commit 触发本地规则更新
- [ ] WebHook 签名验证通过
- [ ] 过期规则在工作区显示警告

---

### 任务 3.2：批量任务体验优化
**优先级**：🟢 P2
**完成度影响**：+3%
**预计工期**：1天

#### 实现内容

1. **实时进度推送**
   - 实现 WebSocket 端点：`/ws/preparation/:jobId`
   - 每完成一批推送进度
   - 包含：已处理/总数/当前路径/预计剩余时间

2. **候选池预加载**
   - 修改 `loadPathPreparationAssets`
   - 预加载所有自定义组件候选
   - 填充 `ComponentCandidates` 到 assets

3. **失败原因聚合**
   - 新增 API：`GET /api/preparation/:jobId/failures-summary`
   - 按原因分组统计
   - 前端显示 Top 5 失败原因

4. **断点续传优化**
   - 记录当前批次进度
   - 取消后可从当前批次恢复
   - 避免重复处理已完成项

#### 验收标准
- [ ] 批量任务进度实时显示
- [ ] 候选池预加载命中率 > 95%
- [ ] 失败原因聚合清晰
- [ ] 断点续传无重复处理

---

## 实施顺序

```
Week 1:
├─ Day 1-2: 任务 1.1（组件候选池）
├─ Day 3-4: 任务 1.2（Vue 保存协议）
└─ Day 5:   任务 2.1（规则完整性自检）

Week 2:
├─ Day 1-2: 任务 2.2（Vue 表单渲染）
├─ Day 3-4: 任务 3.1（增量同步自动化）
└─ Day 5:   任务 3.2（批量任务优化）
```

## 关键里程碑

- **M1（P0 完成）**：Day 4 - 支持全部自定义组件生成 + Vue 保存验证
- **M2（P1 完成）**：Day 7 - 规则自检 + Vue 表单渲染
- **M3（P2 完成）**：Day 10 - 增量自动化 + 批量优化

## 风险与应对

1. **目标平台 API 变更**
   - 风险：候选接口结构不稳定
   - 应对：适配器模式隔离，版本检测

2. **Vue 页面复杂度超预期**
   - 风险：动态逻辑无法静态分析
   - 应对：标记"需真实页面确认"，不强制覆盖

3. **批量任务性能瓶颈**
   - 风险：候选预加载耗时过长
   - 应对：分批预加载，异步缓存

## 验收标准（总体）

### 功能验收
- [ ] FormMaking 标准表单生成率 > 98%
- [ ] FormMaking + 自定义组件生成率 > 95%
- [ ] Vue 配置式页面生成率 > 90%
- [ ] Vue 组件式页面生成率 > 85%
- [ ] 批量任务成功率 > 90%

### 性能验收
- [ ] 单条智能生成响应 < 2s
- [ ] 批量任务 100 条路径 < 5min
- [ ] 规则目录增量分析 < 3min
- [ ] 候选缓存命中率 > 80%

### 稳定性验收
- [ ] 规则分析失败率 < 5%
- [ ] 批量任务中断可恢复
- [ ] 过期规则自动检测
- [ ] 单条失败不影响其他

## 后续优化方向

1. **AI 辅助生成**
   - 使用 LLM 理解复杂 Vue 逻辑
   - 生成字段联动规则
   - 推荐最优路径

2. **可视化规则编辑器**
   - 图形化编辑 Vue 页面规则
   - 拖拽式字段映射
   - 实时预览生成效果

3. **跨流程规则复用**
   - 提取通用字段规则
   - 相似页面规则推荐
   - 规则模板库

---

## 附录：文件清单

### 新增文件
```
internal/adapter/target/component_candidates.go
internal/adapter/target/impl/component_candidates.go
internal/service/component_candidate_cache.go
internal/service/vue_form_renderer.go
internal/service/template_catalog_scheduler.go
internal/model/rule_completeness.go
test/unit/backend/component_candidate_test.go
test/unit/backend/vue_submit_validation_test.go
test/integration/component_candidate_integration_test.go
```

### 修改文件
```
internal/formdata/generator.go
internal/service/template_catalog.go
internal/service/path_config_workspace.go
internal/service/path_preparation.go
internal/api/workspace.go
internal/api/template_catalog.go
```

### 测试文件
```
test/unit/backend/f010_template_catalog_test.go
test/unit/backend/form_data_generator_test.go
test/integration/f010_template_catalog_mysql_test.go
test/integration/form_generation_e2e_test.go
```

---

**计划制定完成时间**：2026-08-20
**计划执行开始时间**：待批准
**预计完成时间**：2026-09-03（10个工作日）
