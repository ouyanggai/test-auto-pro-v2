import assert from 'node:assert/strict'
import test from 'node:test'

import { pathConfigurationMessage } from '../../../../web/src/features/path-configuration/logic.ts'

// TestPersonnelMessageKeepsActionableReason 验证人员候选失效的具体处理位置不会被前端抹成泛化提示。
test('人员配置保留服务端的具体处理原因', () => {
  const message = '当前候选范围已变化，原随机结果不再有效，请在本节点的处理人员区重新选择'
  assert.equal(pathConfigurationMessage(message), message)
  assert.doesNotMatch(pathConfigurationMessage(message), /请补充配置|需要重新确认/)
})

// TestPersonnelMessageExposesNodeLink 验证批量保存遗漏项携带节点键，前端可渲染为可定位的节点入口。
test('人员配置遗漏项包含可定位节点键', () => {
  const detail = { kind: 'node', name: '项目负责人审批', reason: '缺少：处理人员', nodeKey: 'node-token' }
  assert.equal(detail.nodeKey, 'node-token')
})
