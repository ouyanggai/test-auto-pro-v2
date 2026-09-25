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

test('当前用户字段由初始化事件填好 → 补丁不用历史发起人值覆盖（缺陷②）', async () => {
  const written: string[] = []
  await applyNodePatch({
    setData: next => { written.push(...Object.keys(next)) },
    // 初始化后：applyUser 已被表单按注入身份（user-plan）填好；reason 业务字段仍为空待补。
    getValues: () => ({
      applyUser: '{"id":"user-plan","name":"欧阳改"}',
      applyUser__condition: '欧阳改',
      reason: ''
    })
  }, {
    applyUser: '{"id":"user-hist","name":"黄成礼"}',
    applyUser__condition: '黄成礼',
    reason: '进入机房检修'
  }, { allowedKeys: ['applyUser', 'reason'], identity: { userId: 'user-plan', companyId: 'company-plan' } })
  assert.ok(!written.includes('applyUser'), '当前用户字段不应被历史发起人值覆盖')
  assert.ok(!written.includes('applyUser__condition'), '当前用户字段伴生键不应被覆盖')
  assert.ok(written.includes('reason'), '业务字段仍需应用历史/路径修正值')
})

test('无身份提示时不跳过任何字段（回落原覆盖行为）', async () => {
  const written: string[] = []
  await applyNodePatch({
    setData: next => { written.push(...Object.keys(next)) },
    getValues: () => ({ applyUser: '{"id":"user-plan"}', reason: '' })
  }, { applyUser: '{"id":"user-hist"}', reason: 'x' }, { allowedKeys: ['applyUser', 'reason'] })
  assert.ok(written.includes('applyUser'), '无身份提示应回落为按补丁覆盖')
})

test('补丁本身包含当前用户完整信息时保留补丁写入（不被误跳过）', async () => {
  const written: Record<string, unknown> = {}
  await applyNodePatch({
    setData: next => { Object.assign(written, next) },
    // 初始化后：页面取出的 company 为瘦对象（name 为空）
    getValues: () => ({
      company: '{"id":"company-plan","name":""}',
      reason: ''
    })
  }, {
    // 补丁提供了当前用户带有完整公司名称的有效对象
    company: '{"id":"company-plan","name":"深圳市润世华智数技术有限公司"}',
    reason: '巡检'
  }, { allowedKeys: ['company', 'reason'], identity: { userId: 'user-plan', companyId: 'company-plan' } })
  assert.ok(written.company, '补丁本身是当前用户时应保留')
  assert.equal(written.company, '{"id":"company-plan","name":"深圳市润世华智数技术有限公司"}')
})
