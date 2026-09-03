import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const read = (relative) => readFileSync(new URL(`../../../..${relative}`, import.meta.url), 'utf8')

test('T05 数据工作区按原始 values 协议连接复制 runtime', () => {
  const view = read('/web/src/views/PlanPathConfigurationView.vue')
  const frame = read('/web/src/features/path-configuration/FormRuntimeFrame.vue')
  const runtime = read('/form-runtime/src/App.vue')
  const api = read('/web/src/features/path-configuration/api.ts')

  assert.match(view, /fetchPathConfigurationData\(/)
  assert.match(view, /savePathConfigurationData\(/)
  assert.match(view, /values: captured\.values/)
  assert.match(frame, /values: props\.form\.effectiveFormData/)
  assert.match(frame, /function setValues\(/)
  assert.match(runtime, /this\.values = clonePlain\(payload\.values \|\| \{\}\)/)
  assert.doesNotMatch(runtime, /generatedValues|generatedFieldPaths|manualOverridePaths/)
  assert.doesNotMatch(frame, /setGeneratedData|generatedFieldPaths|manualOverridePaths/)
  assert.doesNotMatch(runtime, /ruleVersion/)
  assert.doesNotMatch(frame, /ruleVersion/)
  assert.doesNotMatch(read('/form-runtime/src/HostVuePage.vue'), /vueFieldBridge|page\.fields/)
  assert.doesNotMatch(api, /HistoricalDataPayload|fieldMapping|renderAdapter/)
})

test('T05 表单提示改为右侧可收起悬浮面板并给出决定路径的关键字段', () => {
  const view = read('/web/src/views/PlanPathConfigurationView.vue')
  const panel = read('/web/src/features/path-configuration/FormDataHintsPanel.vue')

  assert.match(view, /<form-data-hints-panel/)
  assert.match(view, /:key-fields="dataWorkspace\.keyFields \?\? \[\]"/)
  assert.match(panel, /决定当前路径的字段/)
  assert.match(panel, /position: absolute/)
  assert.match(panel, /open = !open/)
  assert.match(panel, /useThemeVars\(\)/)
  // 提示面板不再把内部术语直接摊在页面上。
  for (const forbidden of ['最小补丁', 'form-runtime 校验', 'needs_input']) {
    assert.equal(panel.includes(forbidden), false, `提示面板不应出现 ${forbidden}`)
  }
})

test('T05 保存换路需要确认令牌且取消不触发写入', () => {
  const view = read('/web/src/views/PlanPathConfigurationView.vue')
  const api = read('/web/src/features/path-configuration/api.ts')

  assert.match(view, /routeConfirmationOpen/)
  assert.match(view, /confirmRouteChange\(\)/)
  assert.match(view, /cancelRouteChange\(\)/)
  assert.match(view, /confirmationToken/)
  assert.match(view, /PATH_ROUTE_CONFIRMATION_REQUIRED/)
  assert.match(api, /headers: \{ 'Idempotency-Key': idempotencyKey \}/)
  assert.doesNotMatch(view, /generatePathFormData|换一组|nextFormGenerationSeed/)
  // 智能生成数据只是打开基础表单数据弹窗，真实分支补丁由服务端在读取数据工作区时完成。
  assert.match(view, /智能生成数据/)
  assert.match(view, /dataPickerOpen = true/)
  assert.match(view, /handleBaseFormDataSaved/)
})
