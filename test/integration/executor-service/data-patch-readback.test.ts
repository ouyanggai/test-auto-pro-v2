// 数据补丁（S5 权威实现 applyNodePatch）的单元测试：
// 只写允许字段（含 __virtualName 虚拟字段），写完即回读完整值。
// optionsReady 的硬编码门禁已被证伪删除——就绪由服务端 waitForOptionsReady「等待」出来。
import assert from 'node:assert/strict'
import test from 'node:test'

import { applyNodePatch, UnsupportedFormError } from '../../../executor-service/src/data-patch.ts'

test('只写允许字段并回读完整值（含虚拟字段）', async () => {
  const written: string[] = []
  const values = await applyNodePatch({
    setData: next => {
      written.push(...Object.keys(next))
    },
    getValues: () => ({ vacateType: '2', vacateType__virtualName: '产假', reason: 'unchanged' })
  }, { vacateType: '2', vacateType__virtualName: '产假', other: 'must-not-write' }, { allowedKeys: ['vacateType'] })
  assert.deepEqual(written, ['vacateType', 'vacateType__virtualName'])
  assert.equal(values.vacateType, '2')
  assert.equal(values.reason, 'unchanged')
})

test('缺少 setData/getValues 接口 → unsupported 阻塞', async () => {
  await assert.rejects(
    () => applyNodePatch({}, { vacateType: '2' }, { allowedKeys: ['vacateType'] }),
    (error: unknown) => error instanceof UnsupportedFormError
  )
  await assert.rejects(
    () => applyNodePatch(null, { vacateType: '2' }, { allowedKeys: ['vacateType'] }),
    (error: unknown) => error instanceof UnsupportedFormError
  )
})

test('回读结果不是对象 → unsupported（数据接口不可信）', async () => {
  await assert.rejects(
    () => applyNodePatch({
      setData: () => undefined,
      getValues: () => null
    }, { vacateType: '2' }, { allowedKeys: ['vacateType'] }),
    (error: unknown) => error instanceof UnsupportedFormError
  )
})
