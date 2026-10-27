import assert from 'node:assert/strict'
import childProcess from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import test, { type TestContext } from 'node:test'

const root = path.resolve('.')
const scriptPath = path.join(root, 'executor-service/scripts/sync-flow-source.js')
const manifestPath = path.join(root, 'executor-service/flow-source-manifest.json')
const metadataPath = path.join(root, 'executor-service/flow-source.json')
const candidateRoot = path.join(root, 'executor-service/flow-source')

function readJson (filePath: string): Record<string, any> {
  return JSON.parse(fs.readFileSync(filePath, 'utf8')) as Record<string, any>
}

function git (cwd: string, ...args: string[]): string {
  return childProcess.execFileSync('git', ['-C', cwd, ...args], {
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe']
  }).trim()
}

function write (rootPath: string, relative: string, contents: string): void {
  const target = path.join(rootPath, relative)
  fs.mkdirSync(path.dirname(target), { recursive: true })
  fs.writeFileSync(target, contents)
}

function createRepository (temporaryRoot: string, name: string, files: Record<string, string>): { remote: string, worktree: string } {
  const seed = path.join(temporaryRoot, `${name}-seed`)
  const remote = path.join(temporaryRoot, `${name}.git`)
  fs.mkdirSync(seed, { recursive: true })
  git(seed, 'init', '-q')
  git(seed, 'config', 'user.name', 'F-053 测试')
  git(seed, 'config', 'user.email', 'f053@example.invalid')
  for (const [relative, contents] of Object.entries(files)) write(seed, relative, contents)
  git(seed, 'add', '.')
  git(seed, 'commit', '-qm', '初始化来源')
  git(seed, 'branch', '-M', 'test')
  fs.mkdirSync(remote, { recursive: true })
  childProcess.execFileSync('git', ['init', '--bare', '-q', remote])
  git(seed, 'remote', 'add', 'origin', remote)
  git(seed, 'push', '-q', 'origin', 'test')
  git(seed, 'branch', 'oygdev2')
  git(seed, 'push', '-q', 'origin', 'oygdev2')

  const worktree = path.join(temporaryRoot, `${name}-worktree`)
  childProcess.execFileSync('git', ['clone', '-q', '--branch', 'oygdev2', remote, worktree])
  return { remote, worktree }
}

function runSync (serviceRoot: string, workspaceRoot: string, args: string[] = []): string {
  return childProcess.execFileSync(process.execPath, [scriptPath, ...args], {
    cwd: root,
    encoding: 'utf8',
    env: {
      ...process.env,
      EXECUTOR_SOURCE_ROOT: serviceRoot,
      EXECUTOR_WORKSPACE_ROOT: workspaceRoot
    },
    stdio: ['ignore', 'pipe', 'pipe']
  })
}

function runSyncFailure (serviceRoot: string, workspaceRoot: string, args: string[] = []): string {
  try {
    runSync(serviceRoot, workspaceRoot, args)
    assert.fail('同步应该失败')
  } catch (error: any) {
    return `${String(error?.stdout ?? '')}\n${String(error?.stderr ?? '')}\n${String(error?.message ?? '')}`
  }
}

function createFixture (context: TestContext): {
  temporaryRoot: string
  serviceRoot: string
  workspaceRoot: string
  baseUpdateWorktree: string
  business: { remote: string, worktree: string }
  updateWorktree: string
} {
  const temporaryRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'f053-flow-source-'))
  context.after(() => fs.rmSync(temporaryRoot, { recursive: true, force: true }))

  const workspaceRoot = path.join(temporaryRoot, 'workspace')
  const serviceRoot = path.join(workspaceRoot, 'executor-service')
  fs.mkdirSync(serviceRoot, { recursive: true })
  write(workspaceRoot, 'form-runtime/runtime-source/src/lib/vue-form-making/dist/FormMaking.common.js', '受控 FormMaking 产物')

  const base = createRepository(temporaryRoot, 'base', {
    'package.json': '{"name":"base"}',
    'src/main.js': '执行器基座入口',
    'src/views/FlowDetail.vue': '执行器路由适配详情页',
    'src/views/FlowInitiate.vue': '执行器路由适配发起页',
    'src/router/index.js': '执行器路由基座占位',
    'src/components/InitFlowDialog/index.js': '执行器适配层',
    'src/components/base-only.js': '基座专用',
    'src/lib/vue-form-making/package.json': '{"main":"dist/FormMaking.common.js"}',
    'src/utils/auth.js': '基座鉴权适配',
    'src/utils/backendRequestRouting.js': '执行器请求路由适配',
    'src/utils/portalFlowAuth.js': '执行器门户身份适配',
    'src/config/env.js': '基座配置适配'
  })
  const business = createRepository(temporaryRoot, 'business', {
    'src/views/FlowDetail.vue': '全过程业务页不应覆盖路由适配页',
    'src/views/GroupApproveManage/contract.js': 'test 分支合同评审页面',
    'src/components/Business.js': 'test 分支业务组件',
    'src/utils/contractAmount.js': 'export function getContractAmountFields() { return { contractSum: 0 } }',
    'src/assets/contract.svg': '<svg />',
    'src/api/index.js': 'test 分支接口'
  })
  const baseUpdateWorktree = path.join(temporaryRoot, 'base-update')
  childProcess.execFileSync('git', ['clone', '-q', '--branch', 'test', base.remote, baseUpdateWorktree])
  const manifest = {
    schemaVersion: 2,
    mappingVersion: 'f053-test',
    base: {
      repository: 'rsh-flow-components',
      remote: base.remote,
      branch: 'test'
    },
    business: {
      repository: 'rsh-cloud-invest-power-system',
      remote: business.remote,
      branch: 'test'
    },
    mappings: [
      {
        source: 'src/views',
        destination: 'src/views',
        kind: 'directory',
        exclude: ['FlowDetail.vue', 'FlowInitiate.vue']
      },
      { source: 'src/components', destination: 'src/components', kind: 'directory', exclude: ['InitFlowDialog/**'] },
      { source: 'src/utils', destination: 'src/utils', kind: 'directory', exclude: ['auth.js'] },
      { source: 'src/assets', destination: 'src/assets', kind: 'directory' },
      { source: 'src/api/index.js', destination: 'src/api/index.js', kind: 'file' }
    ],
    protected: [
      'src/components/InitFlowDialog/**',
      'src/lib/vue-form-making/**',
      'src/utils/auth.js',
      'src/utils/backendRequestRouting.js',
      'src/utils/portalFlowAuth.js',
      'src/config/**',
      'src/main.js',
      'src/router/**',
      'src/views/FlowDetail.vue',
      'src/views/FlowInitiate.vue'
    ]
  }
  write(serviceRoot, 'flow-source-manifest.json', JSON.stringify(manifest, null, 2))

  const updateWorktree = path.join(temporaryRoot, 'business-update')
  childProcess.execFileSync('git', ['clone', '-q', '--branch', 'test', business.remote, updateWorktree])
  return { temporaryRoot, serviceRoot, workspaceRoot, baseUpdateWorktree, business, updateWorktree }
}

test('双来源清单不固定 SHA，业务 test 动态读取且执行器适配层保持保护', () => {
  const manifest = readJson(manifestPath)
  assert.equal(manifest.schemaVersion, 2)
  assert.equal(manifest.base.repository, 'rsh-flow-components')
  assert.equal(manifest.base.branch, 'test')
  assert.equal(Object.prototype.hasOwnProperty.call(manifest.base, 'head'), false)
  assert.ok(manifest.protected.includes('src/lib/vue-form-making/**'))
  assert.ok(manifest.protected.includes('src/utils/backendRequestRouting.js'))
  assert.ok(manifest.protected.includes('src/utils/portalFlowAuth.js'))
  assert.equal(manifest.business.repository, 'rsh-cloud-invest-power-system')
  assert.equal(manifest.business.branch, 'test')
  assert.ok(manifest.business.remote)
  assert.equal(Object.prototype.hasOwnProperty.call(manifest.business, 'head'), false)
})

test('真实候选包含最新业务文件、适配层和受控 FormMaking 产物', () => {
  const metadata = readJson(metadataPath)
  assert.equal(metadata.repository, 'composed-flow-source')
  assert.equal(metadata.baseRepository, 'rsh-flow-components')
  assert.equal(metadata.businessRepository, 'rsh-cloud-invest-power-system')
  assert.equal(metadata.businessBranch, 'test')
  assert.match(metadata.businessHead, /^[0-9a-f]{40}$/)
  assert.match(metadata.candidateDigest, /^[0-9a-f]{64}$/)
  assert.equal(fs.existsSync(path.join(candidateRoot, '.git')), false)
  assert.equal(fs.existsSync(path.join(candidateRoot, 'src/utils/contractAmount.js')), true)
  assert.equal(fs.existsSync(path.join(candidateRoot, 'src/lib/vue-form-making/dist/FormMaking.common.js')), true)
  assert.equal(fs.existsSync(path.join(candidateRoot, 'src/components/InitFlowDialog/index.js')), true)
  assert.equal(fs.existsSync(path.join(candidateRoot, 'src/lib/vue-form-making/package.json')), true)
  assert.equal(fs.existsSync(path.join(candidateRoot, 'src/utils/backendRequestRouting.js')), true)
  assert.equal(fs.existsSync(path.join(candidateRoot, 'src/utils/portalFlowAuth.js')), true)
  assert.equal(fs.existsSync(path.join(candidateRoot, 'src/views/FlowDetail.vue')), true)
  assert.equal(fs.existsSync(path.join(candidateRoot, 'src/views/FlowInitiate.vue')), true)
  assert.equal(fs.readFileSync(path.join(candidateRoot, 'src/components/Custom/components/Business/ContractSealReviewBusiness.vue'), 'utf8').includes('getContractAmountFields'), true)
  assert.equal(fs.readFileSync(path.join(candidateRoot, 'src/views/GroupApproveManage/Submitted/components/FlowDialog.vue'), 'utf8').includes('getContractRmbAmount'), true)
})

test('临时远端只消费 test，不读取 oygdev2 工作树，远端前移后下一次准备取得新提交', context => {
  const fixture = createFixture(context)
  write(fixture.business.worktree, 'src/views/GroupApproveManage/contract.js', 'oygdev2 工作树未提交改动')
  const beforeBranch = git(fixture.business.worktree, 'branch', '--show-current')
  const beforeStatus = git(fixture.business.worktree, 'status', '--short')
  const firstBaseHead = git(fixture.baseUpdateWorktree, 'rev-parse', 'HEAD')
  const firstBusinessHead = git(fixture.updateWorktree, 'rev-parse', 'HEAD')

  runSync(fixture.serviceRoot, fixture.workspaceRoot)
  const first = readJson(path.join(fixture.serviceRoot, 'flow-source.json'))
  assert.equal(first.baseHead, firstBaseHead)
  assert.equal(first.businessHead, firstBusinessHead)
  assert.equal(fs.readFileSync(path.join(fixture.serviceRoot, 'flow-source/src/views/FlowDetail.vue'), 'utf8'), '执行器路由适配详情页')
  assert.equal(fs.readFileSync(path.join(fixture.serviceRoot, 'flow-source/src/views/FlowInitiate.vue'), 'utf8'), '执行器路由适配发起页')
  assert.equal(beforeBranch, 'oygdev2')
  assert.match(beforeStatus, /contract\.js/)
  assert.equal(fs.readFileSync(path.join(fixture.serviceRoot, 'flow-source/src/views/GroupApproveManage/contract.js'), 'utf8'), 'test 分支合同评审页面')
  assert.equal(fs.readFileSync(path.join(fixture.serviceRoot, 'flow-source/src/components/InitFlowDialog/index.js'), 'utf8'), '执行器适配层')

  write(fixture.updateWorktree, 'src/views/GroupApproveManage/contract.js', 'test 分支第二版')
  git(fixture.updateWorktree, 'add', '.')
  git(fixture.updateWorktree, 'commit', '-qm', '更新 test 业务')
  git(fixture.updateWorktree, 'push', '-q', 'origin', 'test')
  write(fixture.baseUpdateWorktree, 'src/router/index.js', '新机房 test 最新路由')
  git(fixture.baseUpdateWorktree, 'add', '.')
  git(fixture.baseUpdateWorktree, 'commit', '-qm', '更新 test 基座')
  git(fixture.baseUpdateWorktree, 'push', '-q', 'origin', 'test')
  runSync(fixture.serviceRoot, fixture.workspaceRoot)
  const second = readJson(path.join(fixture.serviceRoot, 'flow-source.json'))
  assert.notEqual(second.businessHead, first.businessHead)
  assert.notEqual(second.baseHead, first.baseHead)
  assert.equal(second.baseHead, git(fixture.baseUpdateWorktree, 'rev-parse', 'HEAD'))
  assert.equal(second.businessHead, git(fixture.updateWorktree, 'rev-parse', 'HEAD'))
  assert.equal(fs.readFileSync(path.join(fixture.serviceRoot, 'flow-source/src/views/GroupApproveManage/contract.js'), 'utf8'), 'test 分支第二版')
  assert.equal(fs.readFileSync(path.join(fixture.serviceRoot, 'flow-source/src/router/index.js'), 'utf8'), '新机房 test 最新路由')
  assert.equal(git(fixture.business.worktree, 'branch', '--show-current'), 'oygdev2')
  assert.match(git(fixture.business.worktree, 'status', '--short'), /contract\.js/)
})

test('候选目录被清理后，相同远端来源也会重新生成候选', context => {
  const fixture = createFixture(context)
  runSync(fixture.serviceRoot, fixture.workspaceRoot)
  const first = readJson(path.join(fixture.serviceRoot, 'flow-source.json'))

  fs.rmSync(path.join(fixture.serviceRoot, 'flow-source'), { recursive: true, force: true })
  runSync(fixture.serviceRoot, fixture.workspaceRoot)

  const rebuilt = readJson(path.join(fixture.serviceRoot, 'flow-source.json'))
  assert.equal(fs.existsSync(path.join(fixture.serviceRoot, 'flow-source/src/utils/contractAmount.js')), true)
  assert.equal(rebuilt.businessHead, first.businessHead)
  assert.equal(rebuilt.candidateDigest, first.candidateDigest)
})

test('来源清单残留固定 head 时拒绝准备', context => {
  const fixture = createFixture(context)
  const manifestPath = path.join(fixture.serviceRoot, 'flow-source-manifest.json')
  const manifest = readJson(manifestPath)
  manifest.base.head = 'a'.repeat(40)
  write(fixture.serviceRoot, 'flow-source-manifest.json', JSON.stringify(manifest, null, 2))

  assert.match(runSyncFailure(fixture.serviceRoot, fixture.workspaceRoot), /不能固定 rsh-flow-components 的 head/)
  assert.equal(fs.existsSync(path.join(fixture.serviceRoot, 'flow-source.json')), false)
})

test('候选被改动或业务远端不可读时明确失败，不沿用旧候选', context => {
  const fixture = createFixture(context)
  runSync(fixture.serviceRoot, fixture.workspaceRoot)
  const candidateFile = path.join(fixture.serviceRoot, 'flow-source/src/views/GroupApproveManage/contract.js')
  fs.writeFileSync(candidateFile, '被篡改')
  assert.match(runSyncFailure(fixture.serviceRoot, fixture.workspaceRoot, ['--verify-prepared']), /合成候选摘要不一致/)

  const metadataBefore = fs.readFileSync(path.join(fixture.serviceRoot, 'flow-source.json'), 'utf8')
  const manifest = readJson(path.join(fixture.serviceRoot, 'flow-source-manifest.json'))
  manifest.business.remote = path.join(fixture.temporaryRoot, 'missing-business.git')
  write(fixture.serviceRoot, 'flow-source-manifest.json', JSON.stringify(manifest, null, 2))
  assert.match(runSyncFailure(fixture.serviceRoot, fixture.workspaceRoot), /不可达或分支不存在/)
  assert.equal(fs.readFileSync(path.join(fixture.serviceRoot, 'flow-source.json'), 'utf8'), metadataBefore)

  manifest.base.remote = path.join(fixture.temporaryRoot, 'missing-base.git')
  write(fixture.serviceRoot, 'flow-source-manifest.json', JSON.stringify(manifest, null, 2))
  assert.match(runSyncFailure(fixture.serviceRoot, fixture.workspaceRoot), /不可达或分支不存在/)
  assert.equal(fs.readFileSync(path.join(fixture.serviceRoot, 'flow-source.json'), 'utf8'), metadataBefore)
})

test('敏感业务文件进入映射时拒绝候选', context => {
  const fixture = createFixture(context)
  write(fixture.updateWorktree, 'src/components/.env.test', '凭证')
  git(fixture.updateWorktree, 'add', '.')
  git(fixture.updateWorktree, 'commit', '-qm', '加入错误敏感文件')
  git(fixture.updateWorktree, 'push', '-q', 'origin', 'test')
  assert.match(runSyncFailure(fixture.serviceRoot, fixture.workspaceRoot), /敏感文件/)
})

test('同步脚本不复制 macOS 路径或完整 Git 元数据', () => {
  const source = fs.readFileSync(scriptPath, 'utf8')
  assert.equal(source.includes('/Volumes/'), false)
  assert.equal(source.includes('git archive'), false)
  assert.equal(source.includes("'fetch'"), true)
  assert.equal(source.includes("'checkout'"), true)
})
