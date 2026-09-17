import assert from 'node:assert/strict'
import childProcess from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import test from 'node:test'

const PINNED_HEAD = 'bff4ef8b938db5578c3f7eab1f482a4e9388917c'
const PINNED_REMOTE = 'git@192.168.1.155:rsh-cloud/portal/rsh-flow-components.git'
const scriptPath = path.resolve('executor-service/scripts/sync-flow-source.js')
const manifestPath = path.resolve('executor-service/flow-source-manifest.json')
const formMakingScript = path.resolve('executor-service/scripts/build-form-making.js')
const metadataPath = path.resolve('executor-service/flow-source.json')
const configSnapshot = path.resolve('form-runtime/runtime-source/.f007-source.json')

test('同步脚本只按 git 同步，不依赖任何本机静态目录', () => {
  const source = fs.readFileSync(scriptPath, 'utf8')
  // 迁移修订的核心约束：不得再出现 macOS 绝对路径，也不得再整体复制本机目录。
  assert.equal(source.includes('/Volumes/'), false, '同步脚本不得依赖 macOS 绝对路径')
  assert.equal(source.includes('copiedAt'), false, '不再使用「复制快照」口径的元数据')
  assert.equal(source.includes('git archive'), false)
  // 同步必须真的走 git clone/fetch。
  assert.equal(source.includes("'fetch'"), true)
  assert.equal(source.includes("'checkout'"), true)
})

test('同步清单固定远端、分支与提交，不接受路径覆盖', () => {
  const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8')) as
    { repository: string, remote: string, branch: string, head: string }
  assert.equal(manifest.repository, 'rsh-flow-components')
  assert.equal(manifest.remote, PINNED_REMOTE)
  assert.equal(manifest.branch, 'test')
  assert.equal(manifest.head, PINNED_HEAD)
  // 清单里不得出现本机路径字段，来源只能是远端。
  assert.equal(Object.prototype.hasOwnProperty.call(manifest, 'sourceRoot'), false)
})

test('快照元数据来自 git 同步结果', () => {
  assert.equal(fs.existsSync(metadataPath), true, '缺少 executor-service/flow-source.json')
  const metadata = JSON.parse(fs.readFileSync(metadataPath, 'utf8')) as
    { head: string, branch: string, remote: string, digest: string }
  assert.equal(metadata.head, PINNED_HEAD)
  assert.equal(metadata.branch, 'test')
  assert.equal(metadata.remote, PINNED_REMOTE)
  assert.match(metadata.digest, /^[0-9a-f]{64}$/)
})

test('flow-source 确实是清单提交的 git 检出', () => {
  const target = path.resolve('executor-service/flow-source')
  assert.equal(fs.existsSync(path.join(target, '.git')), true, 'flow-source 必须是 git 检出')
  const head = childProcess.execFileSync('git', ['-C', target, 'rev-parse', 'HEAD'], { encoding: 'utf8' }).trim()
  assert.equal(head, PINNED_HEAD)
  const remote = childProcess.execFileSync('git', ['-C', target, 'remote', 'get-url', 'origin'], { encoding: 'utf8' }).trim()
  assert.equal(remote, PINNED_REMOTE)
})

test('配置侧 runtime-source 快照未被改写', () => {
  const metadata = JSON.parse(fs.readFileSync(configSnapshot, 'utf8')) as { head: string }
  assert.equal(metadata.head, 'bff4ef8b938db5578c3f7eab1f482a4e9388917c')
})

test('FormMaking 产物由受控副本补齐，不自行编译', () => {
  const source = fs.readFileSync(formMakingScript, 'utf8')
  assert.equal(source.includes('不自己编译'), true)
  assert.equal(source.includes('vue-cli-service'), false)
  // 产物不在 rsh-flow-components 的 Git 里，必须由同步脚本从受控副本补齐。
  const syncSource = fs.readFileSync(scriptPath, 'utf8')
  assert.equal(syncSource.includes('stageFormMaking'), true)
  const entry = path.resolve('executor-service/flow-source/src/lib/vue-form-making/dist/FormMaking.common.js')
  assert.equal(fs.existsSync(entry), true, '缺少 FormMaking.common.js')
})
