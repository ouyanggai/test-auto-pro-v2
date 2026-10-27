import assert from 'node:assert/strict'
import crypto from 'node:crypto'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import test, { type TestContext } from 'node:test'

import { readBaselineSource } from '../../../executor-service/src/baseline-source.ts'

// createFixtureRoot 创建本地临时服务目录，清理前核实它仍是本测试直接创建的目录。
function createFixtureRoot (context: TestContext): string {
  const temporaryParent = fs.realpathSync(os.tmpdir())
  const root = fs.mkdtempSync(path.join(temporaryParent, 'executor-baseline-source-'))
  context.after(() => {
    const resolvedRoot = fs.realpathSync(root)
    assert.equal(path.dirname(resolvedRoot).toLowerCase(), temporaryParent.toLowerCase())
    assert.ok(path.basename(resolvedRoot).startsWith('executor-baseline-source-'))
    fs.rmSync(resolvedRoot, { recursive: true, force: true })
  })
  return root
}

// writeFixture 保存测试元数据或二进制产物，不调用构建、同步或网络接口。
function writeFixture (root: string, relativePath: string, contents: string | Buffer): void {
  const target = path.join(root, relativePath)
  fs.mkdirSync(path.dirname(target), { recursive: true })
  fs.writeFileSync(target, contents)
}

test('缺失元数据和产物明确未知，部署与 FormMaking 源码版本不猜测', context => {
  const root = createFixtureRoot(context)
  const result = readBaselineSource(root)
  assert.equal(result.deployedVersion, 'unknown')
  assert.equal(result.formMakingSourceHead, 'unknown')
  assert.ok(typeof result.capturedAt === 'string')
  assert.ok(Number.isFinite(Date.parse(result.capturedAt)))
  assert.deepEqual(result.flowSourceManifest, { state: 'unknown', mappingVersion: 'unknown' })
  assert.deepEqual(result.flowSource, {
    state: 'unknown',
    mappingVersion: 'unknown',
    mappingHash: 'unknown',
    candidateDigest: 'unknown',
    baseRepository: 'unknown',
    baseRemote: 'unknown',
    baseBranch: 'unknown',
    baseHead: 'unknown',
    baseDigest: 'unknown',
    businessRepository: 'unknown',
    businessRemote: 'unknown',
    businessBranch: 'unknown',
    businessHead: 'unknown',
    businessDigest: 'unknown',
    preparedAt: 'unknown'
  })
  assert.deepEqual(result.nativePageBuild, {
    state: 'unknown',
    inputDigest: 'unknown',
    candidateDigest: 'unknown',
    baseHead: 'unknown',
    businessHead: 'unknown',
    mappingHash: 'unknown',
    builtAt: 'unknown'
  })
  assert.deepEqual(result.formMakingArtifact, { state: 'unknown', sha256: 'unknown', bytes: 'unknown' })
})

test('损坏 JSON 与非对象 JSON 不抛错也不伪造已知字段', context => {
  const root = createFixtureRoot(context)
  writeFixture(root, 'flow-source-manifest.json', '{broken')
  writeFixture(root, 'flow-source.json', '["not-metadata"]')
  writeFixture(root, path.join('flow-source', '.native-page-build.json'), 'null')
  const result = readBaselineSource(root)
  assert.deepEqual(result.flowSourceManifest, { state: 'unknown', mappingVersion: 'unknown' })
  assert.deepEqual(result.flowSource, {
    state: 'unknown',
    mappingVersion: 'unknown',
    mappingHash: 'unknown',
    candidateDigest: 'unknown',
    baseRepository: 'unknown',
    baseRemote: 'unknown',
    baseBranch: 'unknown',
    baseHead: 'unknown',
    baseDigest: 'unknown',
    businessRepository: 'unknown',
    businessRemote: 'unknown',
    businessBranch: 'unknown',
    businessHead: 'unknown',
    businessDigest: 'unknown',
    preparedAt: 'unknown'
  })
  assert.deepEqual(result.nativePageBuild, {
    state: 'unknown',
    inputDigest: 'unknown',
    candidateDigest: 'unknown',
    baseHead: 'unknown',
    businessHead: 'unknown',
    mappingHash: 'unknown',
    builtAt: 'unknown'
  })
})

test('已有元数据只读允许字段，FormMaking 摘要来自原始字节而不是文本', context => {
  const root = createFixtureRoot(context)
  writeFixture(root, 'flow-source-manifest.json', JSON.stringify({ schemaVersion: 2, mappingVersion: 'f053-test', password: 'must-not-copy' }))
  writeFixture(root, 'flow-source.json', JSON.stringify({
    schemaVersion: 2,
    repository: 'composed-flow-source',
    mappingVersion: 'f053-test',
    mappingHash: 'mapping-hash',
    candidateDigest: 'candidate-digest',
    baseRepository: 'rsh-flow-components',
    baseRemote: 'base-remote',
    baseBranch: 'test',
    baseHead: 'base-head',
    baseDigest: 'base-digest',
    businessRepository: 'rsh-cloud-invest-power-system',
    businessRemote: 'business-remote',
    businessBranch: 'test',
    businessHead: 'business-head',
    businessDigest: 'business-digest',
    preparedAt: '2026-10-13T00:00:00.000Z',
    sid: 'must-not-copy'
  }))
  writeFixture(root, path.join('flow-source', '.native-page-build.json'), JSON.stringify({
    inputDigest: 'build-digest',
    candidateDigest: 'candidate-digest',
    baseHead: 'base-head',
    businessHead: 'business-head',
    mappingHash: 'mapping-hash',
    builtAt: '2026-10-13T01:00:00.000Z',
    deployedVersion: 'must-not-infer'
  }))
  const artifact = Buffer.from([0, 255, 13, 10, 128, 97, 98, 99])
  writeFixture(root, path.join('flow-source', 'src', 'lib', 'vue-form-making', 'dist', 'FormMaking.common.js'), artifact)
  const result = readBaselineSource(root)
  assert.deepEqual(result.flowSourceManifest, { state: 'available', mappingVersion: 'f053-test' })
  assert.deepEqual(result.flowSource, {
    state: 'available',
    mappingVersion: 'f053-test',
    mappingHash: 'mapping-hash',
    candidateDigest: 'candidate-digest',
    baseRepository: 'rsh-flow-components',
    baseRemote: 'base-remote',
    baseBranch: 'test',
    baseHead: 'base-head',
    baseDigest: 'base-digest',
    businessRepository: 'rsh-cloud-invest-power-system',
    businessRemote: 'business-remote',
    businessBranch: 'test',
    businessHead: 'business-head',
    businessDigest: 'business-digest',
    preparedAt: '2026-10-13T00:00:00.000Z'
  })
  assert.deepEqual(result.nativePageBuild, {
    state: 'available',
    inputDigest: 'build-digest',
    candidateDigest: 'candidate-digest',
    baseHead: 'base-head',
    businessHead: 'business-head',
    mappingHash: 'mapping-hash',
    builtAt: '2026-10-13T01:00:00.000Z'
  })
  assert.deepEqual(result.formMakingArtifact, { state: 'available', sha256: crypto.createHash('sha256').update(artifact).digest('hex'), bytes: artifact.length })
  assert.equal(result.deployedVersion, 'unknown')
  assert.equal(result.formMakingSourceHead, 'unknown')
  assert.ok(!JSON.stringify(result).includes('must-not-'))
  assert.deepEqual(fs.readFileSync(path.join(root, 'flow-source', 'src', 'lib', 'vue-form-making', 'dist', 'FormMaking.common.js')), artifact)
})

test('缺失或类型错误的元数据字段仍为未知，不作字符串强制转换', context => {
  const root = createFixtureRoot(context)
  writeFixture(root, 'flow-source.json', JSON.stringify({ baseHead: 123, businessBranch: 'test', candidateDigest: '', preparedAt: null }))
  const result = readBaselineSource(root)
  assert.equal(result.flowSource.state, 'available')
  assert.equal(result.flowSource.baseHead, 'unknown')
  assert.equal(result.flowSource.businessBranch, 'test')
  assert.equal(result.flowSource.candidateDigest, 'unknown')
  assert.equal(result.flowSource.preparedAt, 'unknown')
})

test('文件位置变为目录的读取错误只保留未知，不使采集失败', context => {
  const root = createFixtureRoot(context)
  fs.mkdirSync(path.join(root, 'flow-source-manifest.json'))
  fs.mkdirSync(path.join(root, 'flow-source', 'src', 'lib', 'vue-form-making', 'dist', 'FormMaking.common.js'), { recursive: true })
  const result = readBaselineSource(root)
  assert.deepEqual(result.flowSourceManifest, { state: 'unknown', mappingVersion: 'unknown' })
  assert.deepEqual(result.formMakingArtifact, { state: 'unknown', sha256: 'unknown', bytes: 'unknown' })
})
