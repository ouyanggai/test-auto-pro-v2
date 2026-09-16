import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import test from 'node:test'

const PINNED_HEAD = 'be355ae309479a0c37598c0ee95ba724d062e5f1'
const ORIGINAL_ROOT = '/Volumes/oygsky/bigsys/rsh-flow-components'
const scriptPath = path.resolve('executor-service/scripts/sync-flow-source.js')
const formMakingScript = path.resolve('executor-service/scripts/build-form-making.js')
const metadataPath = path.resolve('executor-service/flow-source.json')
const configSnapshot = path.resolve('form-runtime/runtime-source/.f007-source.json')

test('同步脚本复制原项目并钉死 be355ae', () => {
  const source = fs.readFileSync(scriptPath, 'utf8')
  assert.equal(source.includes(ORIGINAL_ROOT), true)
  assert.equal(source.includes(PINNED_HEAD), true)
  assert.equal(source.includes('form-runtime/runtime-source'), true)
  assert.equal(source.includes('git archive'), false)
})

test('快照元数据来自原项目 be355ae', () => {
  assert.equal(fs.existsSync(metadataPath), true, '缺少 executor-service/flow-source.json')
  const metadata = JSON.parse(fs.readFileSync(metadataPath, 'utf8')) as { head: string, branch: string, sourceRoot: string }
  assert.equal(metadata.head, PINNED_HEAD)
  assert.equal(metadata.branch, 'test')
  assert.equal(metadata.sourceRoot, ORIGINAL_ROOT)
})

test('配置侧 runtime-source 快照未被改写', () => {
  const metadata = JSON.parse(fs.readFileSync(configSnapshot, 'utf8')) as { head: string }
  assert.equal(metadata.head, 'bff4ef8b938db5578c3f7eab1f482a4e9388917c')
})

test('FormMaking 随原项目复制，不自行编译', () => {
  const source = fs.readFileSync(formMakingScript, 'utf8')
  assert.equal(source.includes('不自己编译'), true)
  assert.equal(source.includes('vue-cli-service'), false)
})
