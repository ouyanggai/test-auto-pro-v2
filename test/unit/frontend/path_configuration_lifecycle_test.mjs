import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const configurationView = readFileSync(new URL('../../../web/src/views/PlanPathConfigurationView.vue', import.meta.url), 'utf8')
const runtimeFrame = readFileSync(new URL('../../../web/src/features/path-configuration/FormRuntimeFrame.vue', import.meta.url), 'utf8')
const runtimeProtocol = readFileSync(new URL('../../../web/src/features/path-configuration/runtimeProtocol.ts', import.meta.url), 'utf8')
const configurationAPI = readFileSync(new URL('../../../web/src/features/path-configuration/api.ts', import.meta.url), 'utf8')
const configurationRetry = readFileSync(new URL('../../../web/src/features/path-configuration/retry.ts', import.meta.url), 'utf8')

test('表单工作区离开时只失效宿主会话，不重复销毁 iframe', () => {
  assert.match(configurationView, /function invalidateRuntimeSession\(\) \{[\s\S]*runtimeSessionController\?\.abort\(\)[\s\S]*formOperationController\?\.abort\(\)[\s\S]*runtimeSession\.value = null/)
  assert.doesNotMatch(configurationView, /formFrame\.value\?\.destroyRuntime\(\)/)
  assert.match(configurationView, /function returnToNodes\(\) \{\s*invalidateRuntimeSession\(\)\s*workspace\.value = 'nodes'/)
  assert.match(configurationView, /onBeforeUnmount\(\(\) => \{[\s\S]*invalidateRuntimeSession\(\)/)
})

test('表单异步操作只允许当前 iframe 和当前会话代次回写', () => {
  assert.match(configurationView, /function isActiveFormOperation\(epoch: number, frame: FormRuntimeExpose \| null\): boolean/)
  assert.equal((configurationView.match(/isActiveFormOperation\(epoch, frame\)/g) || []).length >= 7, true)
  assert.match(configurationView, /function handleRuntimeError\(message: string\) \{\s*if \(workspace\.value === 'form' && runtimeSession\.value\) formError\.value = message/)
  assert.match(configurationView, /fetchPathFormRuntimeSession\(planID\.value, pathID\.value, controller\.signal\)/)
  assert.match(configurationView, /generatePathFormData\([\s\S]*controller\.signal\)/)
  assert.match(configurationView, /savePathFormData\([\s\S]*controller\.signal\)/)
  assert.match(configurationAPI, /signal\?: AbortSignal,[\s\S]*\): Promise<PathConfigSaveResult>/)
})

test('iframe teardown 幂等且旧加载不会终止新会话', () => {
  assert.match(runtimeFrame, /let runtimeActive = false\s*let runtimeGeneration = 0/)
  assert.match(runtimeFrame, /let iframeBootPending = true/)
  assert.match(runtimeFrame, /function resetRuntime\(notifyFrame: boolean\) \{\s*if \(!runtimeActive && pending\.size === 0\) return/)
  assert.match(runtimeFrame, /runtimeGeneration \+= 1\s*runtimeActive = false/)
  assert.match(runtimeFrame, /const generation = \+\+runtimeGeneration/)
  assert.match(runtimeFrame, /generation !== runtimeGeneration/)
  assert.match(runtimeFrame, /const disposition = classifyRuntimeMessage\(message,[\s\S]*bootPending: iframeBootPending/)
  assert.match(runtimeFrame, /if \(disposition === 'boot'\) \{\s*iframeBootPending = false/)
  assert.match(runtimeProtocol, /context\.disposed \|\| context\.runtimeActive \|\| !context\.bootPending \? 'ignore' : 'boot'/)
  assert.match(runtimeFrame, /onBeforeUnmount\(\(\) => \{\s*destroyRuntime\(\)\s*disposed = true/)
})

test('条件绑定 API 将 null 响应归一为可渲染数组和布尔值', async () => {
  const originalFetch = globalThis.fetch
  const responses = [
    { form: { conditionBindings: [{ key: 'hint', fields: null, selected: null, locked: null, needsReview: null }], conditionReviews: null, fieldRules: null } },
    { conditionBindings: [{ key: 'generated', fields: null, selected: null, locked: null, needsReview: null }], conditionReviews: null, fieldRules: null },
  ]
  globalThis.fetch = async () => new Response(JSON.stringify({ success: true, data: responses.shift() }), { status: 200, headers: { 'Content-Type': 'application/json' } })
  try {
    const api = await import('../../../web/src/features/path-configuration/api.ts')
    const configuration = await api.fetchPathConfiguration('1', '1', new AbortController().signal)
    const binding = configuration.form.conditionBindings[0]
    assert.deepEqual(binding.fields, [])
    assert.equal(binding.selected, false)
    assert.equal(binding.locked, false)
    assert.equal(binding.needsReview, false)
    assert.deepEqual(configuration.form.conditionReviews, [])
    assert.deepEqual(configuration.form.fieldRules, [])

    const generated = await api.generatePathFormData('1', '1', 1, {}, [], false)
    assert.deepEqual(generated.conditionBindings[0].fields, [])
    assert.equal(generated.conditionBindings[0].selected, false)
  } finally {
    globalThis.fetch = originalFetch
  }
})

test('路径配置页导入实际使用的 NSpace 组件并保护条件提示长度访问', () => {
  assert.match(configurationView, /import \{[^}]*NSpace[^}]*\} from 'naive-ui'/s)
  assert.match(configurationView, /Array\.isArray\(binding\.fields\) && binding\.fields\.length/)
})

test('首读初始化只对可重试失败重试并保持 loading 到真实初始化完成', async () => {
  const { retryPathLoad } = await import('../../../web/src/features/path-configuration/retry.ts')
  const signal = new AbortController().signal
  let attempts = 0
  const transient = Object.assign(new Error('服务暂不可用'), { retryable: true })
  const result = await retryPathLoad(async () => {
    attempts += 1
    if (attempts < 3) throw transient
    return 'ready'
  }, signal, [0, 0])
  assert.equal(result, 'ready')
  assert.equal(attempts, 3)

  let businessAttempts = 0
  await assert.rejects(() => retryPathLoad(async () => {
    businessAttempts += 1
    throw new Error('当前已保存路径与真实流程不一致，请先编辑路径')
  }, signal, [0]), /真实流程不一致/)
  assert.equal(businessAttempts, 1)
  assert.match(configurationView, /const \[storedPlan, storedGraph, storedPaths\] = await retryPathLoad\([\s\S]*fetchFlowGraph\(planID\.value, signal\)/)
  assert.match(configurationView, /pageLoading\.value = true[\s\S]*retryPathLoad/)
  assert.match(configurationRetry, /defaultRetryDelays = \[500, 1200\]/)
  assert.match(configurationRetry, /retryable.*=== true/)
})

test('表单生成、恢复和保存使用相互独立的忙碌状态', () => {
  assert.match(configurationView, /const formGenerating = ref\(false\)/)
  assert.match(configurationView, /const formSaving = ref\(false\)/)
  assert.match(configurationView, /const formRestoring = ref\(false\)/)
  assert.match(configurationView, /formGenerating\.value = true[\s\S]*formGenerationKind\.value = nextGroup \? 'next' : 'smart'/)
  assert.match(configurationView, /formRestoring\.value = true/)
  assert.match(configurationView, /:loading="formGenerating && formGenerationKind === 'smart'"/)
  assert.match(configurationView, /:loading="formGenerating && formGenerationKind === 'next'"/)
  assert.match(configurationView, /:loading="formSaving"[^>]*@click="saveFormData"/)
  assert.doesNotMatch(configurationView, /formSaving\.value = true[\s\S]*generatePathFormData\(/)
})

test('表单生成共享二十秒期限并原位回填真实 iframe', () => {
  assert.match(configurationView, /const formGenerationOperationTimeout = 20_000/)
  assert.match(configurationView, /beginFormGenerationDeadline\(controller, \(\) => stage\)/)
  assert.match(configurationView, /frame\.getValues\(controller\.signal\)/)
  assert.match(configurationView, /generatePathFormData\([\s\S]*controller\.signal\)/)
  assert.match(configurationView, /frame\.setGeneratedData\([\s\S]*generated\.fieldRules, controller\.signal\)/)
  assert.match(configurationView, /window\.clearTimeout\(deadline\)[\s\S]*formGenerating\.value = false/)
  assert.match(configurationView, /当前表单值已保留/)
  assert.doesNotMatch(configurationView, /reloadRuntime/)
  assert.match(runtimeFrame, /function setGeneratedData\([\s\S]*postCommand\('setData'/)
  assert.match(runtimeFrame, /signal\?\.addEventListener\('abort'/)
})

test('表单反馈使用可关闭悬浮通知且不占运行时布局', () => {
  assert.match(configurationView, /useNotification/)
  assert.match(configurationView, /function dismissFormNotice\(\)[\s\S]*formNotice\?\.destroy\(\)/)
  assert.match(configurationView, /function showFormNotice\(\)[\s\S]*notification\.(error|warning|success)/)
  assert.match(configurationView, /watch\(\[workspace, formError, formSavedSuccessfully, runtimeBlockingReasons, formErrorDetails\], showFormNotice/)
  assert.match(configurationView, /function invalidateRuntimeSession\(\) \{[\s\S]*dismissFormNotice\(\)/)
  assert.doesNotMatch(configurationView, /path-configuration-page__form-feedback/)
  const app = readFileSync(new URL('../../../web/src/App.vue', import.meta.url), 'utf8')
  assert.match(app, /<n-notification-provider>/)
})

test('表单运行时固定浅色且条件提示使用标准折叠容器', () => {
  const runtimeApp = readFileSync(new URL('../../../form-runtime/src/App.vue', import.meta.url), 'utf8')
  const runtimeIndex = readFileSync(new URL('../../../form-runtime/public/index.html', import.meta.url), 'utf8')
  const runtimeFrame = readFileSync(new URL('../../../web/src/features/path-configuration/FormRuntimeFrame.vue', import.meta.url), 'utf8')
  assert.match(runtimeApp, /color-scheme: light/)
  assert.doesNotMatch(runtimeApp, /prefers-color-scheme: dark/)
  assert.match(runtimeIndex, /meta name="color-scheme" content="light"/)
  assert.match(runtimeFrame, /background: #fff/)
  assert.match(configurationView, /<section v-if="configuration\.form\.conditionBindings\.length" class="path-configuration-page__form-hints"/)
  assert.match(configurationView, /<n-collapse :default-expanded-names=/)
  assert.doesNotMatch(configurationView, /box-shadow: 0 8px 24px/)
  assert.match(configurationView, /path-configuration-page__form-hints \{[\s\S]*color: #262626[\s\S]*color-scheme: light/)
  assert.match(configurationView, /--n-title-text-color: rgba\(0, 0, 0, 0\.9\) !important/)
})
