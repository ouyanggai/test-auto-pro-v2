import assert from 'node:assert/strict'
import test from 'node:test'

import { applyNodePatch, UnsupportedFormError } from '../../../executor-service/src/data-patch.ts'

test('补丁后绑定值、显示值和虚拟字段一致', async () => {
  const store: Record<string, unknown> = { vacateType: '1', vacateType__virtualName: '事假', other: 'keep' }
  const values = await applyNodePatch({
    async setData (next) {
      Object.assign(store, next)
    },
    async getValues () {
      return { ...store }
    }
  }, { vacateType: '2', vacateType__virtualName: '产假', other: 'must-not-write' }, { allowedKeys: ['vacateType'], optionsReady: true })
  assert.equal(values.vacateType, '2')
  assert.equal(values.vacateType__virtualName, '产假')
  assert.equal(values.other, 'keep')
})

test('未完成选项加载不得执行写动作', async () => {
  let wrote = false
  await assert.rejects(
    () => applyNodePatch({
      async setData () { wrote = true },
      async getValues () { return {} }
    }, { vacateType: '2' }, { allowedKeys: ['vacateType'], optionsReady: false }),
    (error: unknown) => error instanceof UnsupportedFormError && error.code === 'unsupported'
  )
  assert.equal(wrote, false)
})

test('无数据接口的页面不写目标', async () => {
  await assert.rejects(
    () => applyNodePatch({}, { vacateType: '2' }, { allowedKeys: ['vacateType'], optionsReady: true }),
    (error: unknown) => error instanceof UnsupportedFormError
  )
})
