import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const picker = readFileSync('web/src/features/history-replay/BaseFormDataPicker.vue', 'utf8')
const api = readFileSync('web/src/features/history-replay/api.ts', 'utf8')
const types = readFileSync('web/src/features/history-replay/types.ts', 'utf8')
const planPathsView = readFileSync('web/src/views/PlanPathsView.vue', 'utf8')
const pathConfigurationView = readFileSync('web/src/views/PlanPathConfigurationView.vue', 'utf8')

test('基础表单数据弹窗展示计划统一/单独指定、完整性和明确空态', () => {
  assert.match(picker, /选择基础表单数据/)
  assert.match(picker, /沿用计划统一数据/)
  assert.match(picker, /本路径单独指定/)
  assert.match(picker, /candidate\.integrityNotice/)
  assert.match(picker, /candidate\.completeness/)
  assert.match(picker, /candidate\.initiator/)
  assert.match(picker, /candidate\.companyName/)
  assert.match(picker, /candidate\.createdAt/)
  assert.match(picker, /请先在目标平台发起一次该流程并填写业务数据，再回来刷新/)
})

test('选择请求只回传不透明键、模式、修订号和幂等键', () => {
  assert.match(api, /history-data\/candidates/)
  assert.match(api, /history-data\/default/)
  assert.match(api, /configuration\/data\/source/)
  assert.match(api, /Idempotency-Key/)
  assert.match(api, /JSON\.stringify\(\{ candidateKey, revision \}\)/)
  assert.match(api, /JSON\.stringify\(\{ mode, candidateKey: mode === 'override' \? candidateKey : undefined, revision \}\)/)
  for (const forbidden of ['targetInstanceId', 'flowProxyId', 'formProxyId', 'snapshotId', 'rawFormData', 'sourceAccount', 'HistoricalDataPayload']) {
    assert.equal(api.includes(forbidden), false, `选择 API 不应包含 ${forbidden}`)
    assert.equal(types.includes(forbidden), false, `浏览器类型不应包含 ${forbidden}`)
  }
})

test('一键配置与智能生成数据共用同一个基础表单数据弹窗', () => {
  assert.match(planPathsView, /BaseFormDataPicker/)
  assert.match(planPathsView, /一键配置/)
  assert.match(planPathsView, /scope="default"/)
  assert.match(planPathsView, /@click="dataPickerOpen = true"/)
  assert.match(planPathsView, /@saved="startSelectedPreparation"/)
  assert.match(pathConfigurationView, /BaseFormDataPicker/)
  assert.match(pathConfigurationView, /智能生成数据/)
  assert.match(pathConfigurationView, /scope="path"/)
  assert.match(pathConfigurationView, /@saved="handleBaseFormDataSaved"/)
})

test('界面不再出现历史来源与历史回放这类内部术语，也没有旧生成入口', () => {
  for (const [name, source] of [['基础表单数据弹窗', picker], ['计划详情', planPathsView], ['路径配置页', pathConfigurationView]]) {
    for (const forbidden of ['历史来源', '历史回放', '历史快照', '换一组', '随机种子', '示例值']) {
      assert.equal(source.includes(forbidden), false, `${name}不应出现 ${forbidden}`)
    }
  }
})

test('基础表单数据弹窗提供服务端搜索并对搜索空态给出不同指引', () => {
  assert.match(picker, /placeholder="搜索单据名称、发起人或公司"/)
  assert.match(picker, /query: search\.value\.trim\(\)/)
  assert.match(picker, /function applySearch/)
  assert.match(picker, /setTimeout\(\(\) => void loadCandidates\(1\), 300\)/)
  assert.match(picker, /没有匹配的业务数据，换个关键词再试/)
})

test('基础表单数据弹窗不使用 naive 内部变量着色，避免祖先主题色渗漏', () => {
  assert.equal(picker.includes('var(--n-color'), false, '弹窗不应直接引用 naive 内部 --n-color 变量')
  assert.match(picker, /useThemeVars\(\)/)
  assert.match(picker, /--picker-active-color/)
})
