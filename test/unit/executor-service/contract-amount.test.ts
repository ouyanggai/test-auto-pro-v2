import assert from 'node:assert/strict'
import fs from 'node:fs'
import vm from 'node:vm'
import test from 'node:test'

const sourceRoot = new URL('../../../executor-service/flow-source/src/', import.meta.url)
const amountSource = fs.readFileSync(new URL('utils/contractAmount.js', sourceRoot), 'utf8')
const amountApi = vm.runInNewContext(`${amountSource.replace(/^export /gm, '')}\n({ getContractAmountFields, getContractRmbAmount })`)
const getContractAmountFields = amountApi.getContractAmountFields as (value?: Record<string, any>) => Record<string, any>
const getContractRmbAmount = amountApi.getContractRmbAmount as (value?: Record<string, any>) => unknown

test('全过程合同金额真实函数按 CNY 与外币分别保留原币和人民币金额', () => {
  assert.deepEqual({ ...getContractAmountFields({ currency: 'CNY', contractSum: 1250, contractAmountRmb: 1200 }) }, {
    currencyCode: 'CNY',
    originalAmount: 1250,
    contractSum: 1250
  })
  assert.deepEqual({ ...getContractAmountFields({ currency: 'GBP', contractSum: 5000001, contractAmountRmb: 600 }) }, {
    currencyCode: 'GBP',
    originalAmount: 5000001,
    contractSum: 600
  })
})

test('全过程合同金额真实函数支持币种对象和零值，不把零当成缺失金额', () => {
  assert.deepEqual({ ...getContractAmountFields({ currency: { value: 'GBP', label: '英镑' }, contractSum: 5000001, contractAmountRmb: 600 }) }, {
    currencyCode: 'GBP',
    originalAmount: 5000001,
    contractSum: 600
  })
  assert.deepEqual({ ...getContractAmountFields({ currency: { dictValue: 'CNY' }, contractSum: 0, contractAmountRmb: 0 }) }, {
    currencyCode: 'CNY',
    originalAmount: 0,
    contractSum: 0
  })
  assert.equal(getContractRmbAmount({ currency: 'GBP', contractSum: 5000001, contractAmountRmb: 0 }), 0)
})

test('合同发起、合规组件、发起说明和审批意见实际调用本次同步的金额函数', () => {
  const callers = [
    ['components/Custom/components/Business/ContractSealReviewBusiness.vue', 'getContractAmountFields(formVal)'],
    ['components/Custom/components/Business/LegalContractDocTable.vue', 'getContractAmountFields(formVal)'],
    ['views/GroupApproveManage/Submitted/components/FlowDialog.vue', 'getContractRmbAmount(value)'],
    ['views/GroupApproveManage/components/EnterpriseExamineOpinion.vue', 'getContractRmbAmount(value)']
  ] as const
  for (const [relative, call] of callers) {
    const source = fs.readFileSync(new URL(relative, sourceRoot), 'utf8')
    assert.ok(source.includes(call), `${relative} 未调用 ${call}`)
  }
})
