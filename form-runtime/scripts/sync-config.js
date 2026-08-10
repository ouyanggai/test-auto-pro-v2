'use strict'

const childProcess = require('child_process')
const fs = require('fs')
const path = require('path')

const runtimeRoot = path.resolve(__dirname, '..')
const workspaceRoot = path.resolve(runtimeRoot, '..')
const manifestPath = path.join(runtimeRoot, 'sync-manifest.json')

// digestTarget 计算实际构建源码的完整摘要；生成的元数据文件本身不参与，避免摘要循环。
function digestTarget (targetRoot) {
  const hash = require('crypto').createHash('sha256')
  const files = []
  const visit = (directory, prefix = '') => {
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      const relative = prefix ? `${prefix}/${entry.name}` : entry.name
      if (relative === '.f007-source.json') continue
      if (entry.isSymbolicLink()) throw new Error(`实际运行源码不得包含符号链接: ${relative}`)
      if (entry.isDirectory()) visit(path.join(directory, entry.name), relative)
      else if (entry.isFile()) files.push(relative)
    }
  }
  visit(targetRoot)
  for (const relative of files.sort()) {
    hash.update(relative)
    hash.update('\0')
    hash.update(fs.readFileSync(path.join(targetRoot, relative)))
  }
  return hash.digest('hex')
}

// canonicalPath 解析 macOS /var 与 /private/var 这类同一路径别名，同时允许候选目录尚未创建。
function canonicalPath (value) {
  const suffix = []
  let cursor = path.resolve(value)
  while (!fs.existsSync(cursor)) {
    const parent = path.dirname(cursor)
    if (parent === cursor) return path.resolve(value)
    suffix.unshift(path.basename(cursor))
    cursor = parent
  }
  return path.join(fs.realpathSync(cursor), ...suffix)
}

// readManifest 读取本项目固定同步清单；API 和命令行都不能覆盖来源仓库、远端或分支。
function readManifest () {
  return JSON.parse(fs.readFileSync(manifestPath, 'utf8'))
}

// within 判断目标是否仍位于允许的固定目录，阻止环境变量形成任意复制命令。
function within (root, target) {
  const relative = path.relative(root, target)
  return relative !== '..' && !relative.startsWith(`..${path.sep}`) && !path.isAbsolute(relative)
}

// resolveTargetRoot 只允许真实运行源码或维护任务候选源码作为同步目标。
function resolveTargetRoot (manifest, environment = process.env) {
	const liveRoot = canonicalPath(path.join(runtimeRoot, 'runtime-source'))
	const candidateRoot = canonicalPath(path.join(workspaceRoot, '.runtime', 'form-runtime-maintenance', 'workspaces'))
	const requested = environment.FORM_RUNTIME_TARGET_ROOT
		? canonicalPath(environment.FORM_RUNTIME_TARGET_ROOT)
		: liveRoot
  if (requested !== liveRoot && (!within(candidateRoot, requested) || path.basename(requested) !== 'runtime-source')) {
    throw new Error(`同步目标不在允许范围: ${requested}`)
  }
  return requested
}

// assertLiveTargetClean 保护实际构建入口中的本地修改；候选工作区由任务独占，不需要 Git 状态门禁。
function assertLiveTargetClean (targetRoot) {
  const liveRoot = canonicalPath(path.join(runtimeRoot, 'runtime-source'))
  if (targetRoot !== liveRoot) return
  const metadataPath = path.join(targetRoot, '.f007-source.json')
  if (!fs.existsSync(metadataPath)) throw new Error('实际运行源码缺少受控同步快照，拒绝覆盖')
  const metadata = JSON.parse(fs.readFileSync(metadataPath, 'utf8'))
  if (!metadata.targetDigest || digestTarget(targetRoot) !== metadata.targetDigest) {
    throw new Error('实际运行源码存在同步任务之外的修改，拒绝覆盖')
  }
}

// git 只执行固定只读参数并返回文本。
function git (root, args) {
  return childProcess.execFileSync('git', ['-C', root, ...args], { encoding: 'utf8' }).trim()
}

// inspectSource 校验固定远端、master、干净状态与任务创建时 HEAD。
function inspectSource (manifest, environment = process.env) {
  const sourceRoot = path.join(workspaceRoot, manifest.sourceRoot)
  const remote = git(sourceRoot, ['remote', 'get-url', 'origin'])
  const branch = git(sourceRoot, ['branch', '--show-current'])
  const head = git(sourceRoot, ['rev-parse', 'HEAD'])
  const dirty = git(sourceRoot, ['status', '--porcelain=v1', '--untracked-files=all'])
  if (remote !== manifest.sourceRemote) throw new Error(`来源远端不符: ${remote}`)
  if (branch !== manifest.sourceBranch) throw new Error(`来源分支必须是 ${manifest.sourceBranch}`)
  if (dirty) throw new Error('来源工作树必须干净')
  if (environment.FORM_RUNTIME_EXPECTED_HEAD && head !== environment.FORM_RUNTIME_EXPECTED_HEAD) {
    throw new Error(`来源 HEAD 已变化: current=${head} expected=${environment.FORM_RUNTIME_EXPECTED_HEAD}`)
  }
  return { sourceRoot, remote, branch, head }
}

// getSyncConfig 返回原生同步脚本共用的固定来源、目标和任务快照。
function getSyncConfig (environment = process.env) {
  const manifest = readManifest()
  const source = inspectSource(manifest, environment)
  return {
    manifest,
    workspaceRoot,
    runtimeRoot,
    targetRoot: resolveTargetRoot(manifest, environment),
    ...source
  }
}

module.exports = { assertLiveTargetClean, canonicalPath, digestTarget, getSyncConfig, inspectSource, readManifest, resolveTargetRoot, within }
