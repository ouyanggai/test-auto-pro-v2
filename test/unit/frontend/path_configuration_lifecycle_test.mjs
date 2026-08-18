import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const configurationView = readFileSync(new URL('../../../web/src/views/PlanPathConfigurationView.vue', import.meta.url), 'utf8')
const runtimeFrame = readFileSync(new URL('../../../web/src/features/path-configuration/FormRuntimeFrame.vue', import.meta.url), 'utf8')
const configurationAPI = readFileSync(new URL('../../../web/src/features/path-configuration/api.ts', import.meta.url), 'utf8')

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
  assert.match(runtimeFrame, /if \(disposed \|\| runtimeActive \|\| !iframeBootPending\) return\s*iframeBootPending = false/)
  assert.match(runtimeFrame, /onBeforeUnmount\(\(\) => \{\s*destroyRuntime\(\)\s*disposed = true/)
})
