#!/usr/bin/env node

// F-053：把独立的 /flow 基座与全过程 test 业务代码合成为一次发布候选。
// 目标业务仓库只读取远端 Git 对象，不复制用户工作树，也不修改用户分支。
import childProcess from 'node:child_process'
import crypto from 'node:crypto'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import {
  assertSafeRelativePath,
  canonicalJson,
  digestDirectory,
  digestFiles
} from './source-integrity.js'

const defaultServiceRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const serviceRoot = path.resolve(process.env.EXECUTOR_SOURCE_ROOT || defaultServiceRoot)
const workspaceRoot = path.resolve(process.env.EXECUTOR_WORKSPACE_ROOT || path.join(serviceRoot, '..'))
const manifestPath = path.join(serviceRoot, 'flow-source-manifest.json')
const targetRoot = path.join(serviceRoot, 'flow-source')
const metadataPath = path.join(serviceRoot, 'flow-source.json')
const formMakingRoot = path.join(workspaceRoot, 'form-runtime', 'runtime-source')
const generatedTargetName = 'flow-source'
const sensitiveSuffixes = new Set([
  '.key',
  '.pem',
  '.log',
  '.crt',
  '.cer',
  '.p12',
  '.pfx',
  '.jks',
  '.der'
])

function fail (message) {
  process.stderr.write(`错误：${message}\n`)
  process.exitCode = 1
  throw new Error(message)
}

// runGit 在隔离临时仓库内执行固定 Git 参数，任何失败都中断候选准备。
function runGit (root, args) {
  try {
    return childProcess.execFileSync('git', ['-C', root, ...args], {
      encoding: 'utf8',
      timeout: 120000,
      stdio: ['ignore', 'pipe', 'pipe']
    })
  } catch (error) {
    const detail = error?.stderr ? String(error.stderr).trim() : String(error?.message ?? error)
    throw new Error(`Git ${args.join(' ')} 失败：${detail}`)
  }
}

// readJson 只读取本项目清单或本机状态文件，不接受命令行覆盖来源。
function readJson (filePath, label) {
  try {
    return JSON.parse(fs.readFileSync(filePath, 'utf8'))
  } catch (error) {
    throw new Error(`${label}不可读：${filePath}（${String(error?.message ?? error)}）`)
  }
}

// readManifest 校验双来源和受控业务映射；提交号必须在每次准备时从远端解析。
function readManifest () {
  const manifest = readJson(manifestPath, '执行器来源清单')
  if (manifest.schemaVersion !== 2 || !manifest.base || !manifest.business || !Array.isArray(manifest.mappings)) {
    throw new Error('执行器来源清单必须使用 F-053 双来源格式')
  }
  for (const source of [manifest.base, manifest.business]) {
    if (!source.repository || !source.remote || !source.branch) throw new Error('来源清单缺少 repository、remote 或 branch')
    if (Object.prototype.hasOwnProperty.call(source, 'head')) {
      throw new Error(`来源清单不能固定 ${source.repository} 的 head；每次准备都从远端分支读取最新提交`)
    }
  }
  if (!manifest.mappingVersion) throw new Error('来源清单缺少 mappingVersion')
  if (manifest.localOverrides !== undefined && !Array.isArray(manifest.localOverrides)) {
    throw new Error('来源清单 localOverrides 必须是数组')
  }
  return manifest
}

// assertGeneratedPath 只允许删除本脚本在 serviceRoot 下创建的临时目录或候选目录。
function assertGeneratedPath (target, expectedName) {
  const resolvedRoot = path.resolve(serviceRoot)
  const resolvedTarget = path.resolve(target)
  if (path.dirname(resolvedTarget) !== resolvedRoot || path.basename(resolvedTarget) !== expectedName) {
    throw new Error(`拒绝操作工作区外目录：${resolvedTarget}`)
  }
}

// removeGeneratedPath 删除已核实的候选路径，避免错误变量导致递归删除越界。
function removeGeneratedPath (target, expectedName) {
  assertGeneratedPath(target, expectedName)
  fs.rmSync(target, { recursive: true, force: true })
}

// resolveRemoteCandidates 为基座 SSH 远端提供等价 HTTP 候选；全过程远端保持清单原值。
function resolveRemoteCandidates (remote) {
  const candidates = [remote]
  if (remote.startsWith('git@') && remote.includes(':')) {
    const match = remote.match(/^git@([^:]+):(.+)$/)
    if (match) candidates.push(`http://${match[1]}/${match[2]}`)
  }
  return candidates
}

// resolveBranchHead 只从远端分支读取最新完整 SHA；不可达或分支不存在都必须失败。
function resolveBranchHead (source, allowFallbackCandidates) {
  const candidates = allowFallbackCandidates ? resolveRemoteCandidates(source.remote) : [source.remote]
  let lastError = ''
  for (const remote of candidates) {
    try {
      const output = childProcess.execFileSync('git', ['ls-remote', remote, `refs/heads/${source.branch}`], {
        encoding: 'utf8',
        timeout: 30000,
        stdio: ['ignore', 'pipe', 'pipe']
      }).trim()
      const match = output.match(/^([0-9a-f]{40})\s+refs\/heads\/\S+$/m)
      if (match) return { ...source, fetchRemote: remote, head: match[1] }
      lastError = `远端没有 refs/heads/${source.branch}`
    } catch (error) {
      lastError = String(error?.stderr || error?.message || error).trim()
    }
  }
  throw new Error(`${source.repository} ${source.remote} 不可达或分支不存在：${lastError}`)
}

// cloneAt 在 serviceRoot 下创建一次隔离 Git 对象仓库，成功后才允许读取源码。
function cloneAt (source, name) {
  const cloneName = `${name}-${process.pid}`
  const cloneRoot = path.join(serviceRoot, cloneName)
  assertGeneratedPath(cloneRoot, cloneName)
  removeGeneratedPath(cloneRoot, cloneName)
  fs.mkdirSync(cloneRoot, { recursive: true })
  try {
    runGit(cloneRoot, ['init', '--quiet'])
    runGit(cloneRoot, ['remote', 'add', 'origin', source.fetchRemote || source.remote])
    runGit(cloneRoot, ['fetch', '--quiet', '--depth', '1', 'origin', source.head])
    runGit(cloneRoot, ['checkout', '--quiet', '--detach', 'FETCH_HEAD'])
    return cloneRoot
  } catch (error) {
    removeGeneratedPath(cloneRoot, cloneName)
    throw error
  }
}

// listTrackedEntries 读取 Git 树并拒绝符号链接和子模块，保证候选只包含普通文件字节。
function listTrackedEntries (repositoryRoot) {
  const raw = runGit(repositoryRoot, ['ls-tree', '-r', '-z', '--full-tree', 'HEAD'])
  const entries = []
  for (const record of raw.split('\0').filter(Boolean)) {
    const separator = record.indexOf('\t')
    if (separator < 0) throw new Error(`Git 树记录格式非法：${record}`)
    const header = record.slice(0, separator)
    const relative = assertSafeRelativePath(record.slice(separator + 1), 'Git 树路径')
    const mode = header.split(' ')[0]
    if (mode === '120000' || mode === '160000') {
      throw new Error(`Git 来源不能包含符号链接或子模块：${relative}`)
    }
    if (mode !== '100644' && mode !== '100755') throw new Error(`Git 来源文件模式不受支持：${relative}`)
    entries.push({ relative, mode })
  }
  return entries
}

// isExcluded 判断映射相对路径是否命中受保护的运行时文件或清单排除项。
function isExcluded (relative, patterns = []) {
  return patterns.some(pattern => {
    const normalized = pattern.replaceAll('\\', '/').replace(/\/+$/, '')
    return normalized.endsWith('/**')
      ? relative === normalized.slice(0, -3) || relative.startsWith(`${normalized.slice(0, -3)}/`)
      : relative === normalized
  })
}

// isSensitivePath 阻止业务映射把凭证、私钥和日志带入执行器候选。
function isSensitivePath (relative) {
  const normalized = relative.replaceAll('\\', '/')
  const name = path.posix.basename(normalized)
  return name.startsWith('.env') || sensitiveSuffixes.has(path.posix.extname(name).toLowerCase())
}

// copyFileFromRepository 逐字节复制隔离仓库文件，并再次核对目标不是符号链接。
function copyFileFromRepository (repositoryRoot, sourceRelative, candidateRoot, destinationRelative) {
  const source = path.join(repositoryRoot, sourceRelative)
  const destination = path.join(candidateRoot, destinationRelative)
  const sourceStat = fs.lstatSync(source)
  if (!sourceStat.isFile()) throw new Error(`Git 来源不是普通文件：${sourceRelative}`)
  fs.mkdirSync(path.dirname(destination), { recursive: true })
  fs.copyFileSync(source, destination)
}

// candidateFilesUnder 列出候选目录内的普通文件，供映射清理和冲突核对使用。
function candidateFilesUnder (candidateRoot, relativeRoot) {
  const root = path.join(candidateRoot, relativeRoot)
  if (!fs.existsSync(root)) return []
  const files = []
  const visit = (directory, prefix) => {
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      const relative = prefix ? `${prefix}/${entry.name}` : entry.name
      const absolute = path.join(directory, entry.name)
      if (entry.isSymbolicLink()) throw new Error(`候选源码不能包含符号链接：${relative}`)
      if (entry.isDirectory()) visit(absolute, relative)
      else files.push(relative)
    }
  }
  visit(root, '')
  return files
}

// collectMappingFiles 将业务 Git 树映射到候选路径，同时拒绝缺失源、重复目标和受保护冲突。
function collectMappingFiles (repositoryRoot, manifest) {
  const entries = listTrackedEntries(repositoryRoot)
  const entrySet = new Set(entries.map(entry => entry.relative))
  const destinations = new Map()
  const selected = []
  for (const mapping of manifest.mappings) {
    const source = assertSafeRelativePath(mapping.source, '业务映射源')
    const destination = assertSafeRelativePath(mapping.destination, '业务映射目标')
    if (!['file', 'directory'].includes(mapping.kind)) throw new Error(`业务映射类型不受支持：${mapping.kind}`)
    const candidates = mapping.kind === 'file'
      ? [source]
      : entries.filter(entry => entry.relative === source || entry.relative.startsWith(`${source}/`)).map(entry => entry.relative)
    if (!candidates.length) throw new Error(`业务映射源不存在：${source}`)
    let selectedFromSource = 0
    for (const sourceRelative of candidates) {
      const mappingRelative = mapping.kind === 'file'
        ? path.posix.basename(sourceRelative)
        : sourceRelative.slice(source.length + 1)
      if (isExcluded(mappingRelative, mapping.exclude)) continue
      if (isSensitivePath(sourceRelative)) {
        throw new Error(`业务映射命中敏感文件，拒绝纳入候选：${sourceRelative}`)
      }
      const destinationRelative = mapping.kind === 'file'
        ? destination
        : `${destination}/${mappingRelative}`
      assertSafeRelativePath(destinationRelative, '业务映射目标')
      if (isExcluded(destinationRelative, manifest.protected)) {
        throw new Error(`业务映射覆盖了执行器保护路径：${destinationRelative}`)
      }
      if (!entrySet.has(sourceRelative)) throw new Error(`业务映射源文件不存在：${sourceRelative}`)
      if (destinations.has(destinationRelative)) {
        throw new Error(`业务映射目标冲突：${destinationRelative}`)
      }
      destinations.set(destinationRelative, sourceRelative)
      selected.push({ sourceRelative, destinationRelative })
      selectedFromSource += 1
    }
    if (!selectedFromSource) throw new Error(`业务映射没有纳入任何文件：${source}`)
  }
  if (!selected.length) throw new Error('业务映射没有纳入任何文件')
  return selected
}

// copyBaseRepository 把基座 Git 树复制到候选，不把全过程完整应用替换为 /flow 基座。
function copyBaseRepository (repositoryRoot, candidateRoot) {
  for (const entry of listTrackedEntries(repositoryRoot)) {
    const parts = entry.relative.split('/')
    if (parts[0] === 'dist' || parts.includes('node_modules') ||
        parts.includes('.npmrc') || parts.includes('.claude') || parts.includes('.omc') ||
        parts.includes('.DS_Store') || parts.includes('__MACOSX') || isSensitivePath(entry.relative)) continue
    copyFileFromRepository(repositoryRoot, entry.relative, candidateRoot, entry.relative)
  }
}

// applyBusinessMappings 以全过程 test 为业务主体，只保留清单保护的执行器适配路径。
function applyBusinessMappings (businessRoot, candidateRoot, manifest, selected) {
  const preservedFiles = new Set()
  const preservedDirectories = new Set()
  for (const pattern of manifest.protected) {
    const normalized = pattern.replaceAll('\\', '/').replace(/\/+$/, '')
    if (normalized.endsWith('/**')) preservedDirectories.add(normalized.slice(0, -3))
    else preservedFiles.add(normalized)
  }
  for (const mappingRoot of ['src', 'public']) {
    for (const relative of candidateFilesUnder(candidateRoot, mappingRoot)) {
      const destinationRelative = `${mappingRoot}/${relative}`
    const isPreserved = preservedFiles.has(destinationRelative) ||
      [...preservedDirectories].some(directory =>
        destinationRelative === directory || destinationRelative.startsWith(`${directory}/`))
      if (!isPreserved) fs.rmSync(path.join(candidateRoot, destinationRelative), { force: true })
    }
  }
  for (const item of selected) {
    if (isExcluded(item.destinationRelative, manifest.protected)) continue
    copyFileFromRepository(businessRoot, item.sourceRelative, candidateRoot, item.destinationRelative)
  }
}

// resolveServiceRelativePath 只允许读取本项目 serviceRoot 下的受控覆盖文件。
function resolveServiceRelativePath (relative, label) {
  const safe = assertSafeRelativePath(relative, label)
  const resolved = path.resolve(serviceRoot, safe)
  const rootPrefix = `${path.resolve(serviceRoot)}${path.sep}`
  if (!resolved.startsWith(rootPrefix)) throw new Error(`${label}越出执行器目录：${relative}`)
  return resolved
}

// readLocalOverride 读取严格的文本替换覆盖定义，拒绝目录、符号链接和空操作。
function readLocalOverride (definition) {
  if (!definition || typeof definition !== 'object') throw new Error('localOverrides 含有非法覆盖定义')
  const relative = definition.file
  const filePath = resolveServiceRelativePath(relative, '业务覆盖文件')
  const stat = fs.lstatSync(filePath)
  if (!stat.isFile() || stat.isSymbolicLink()) throw new Error(`业务覆盖文件不是普通文件：${relative}`)
  const override = readJson(filePath, `业务覆盖文件 ${relative}`)
  if (override.schemaVersion !== 1 || !override.target || !Array.isArray(override.operations) || override.operations.length === 0) {
    throw new Error(`业务覆盖文件格式非法：${relative}`)
  }
  for (const operation of override.operations) {
    if (!operation || typeof operation.find !== 'string' || operation.find.length === 0 ||
        typeof operation.replace !== 'string' || operation.count !== 1) {
      throw new Error(`业务覆盖操作必须包含唯一 find/replace/count=1：${relative}`)
    }
  }
  return { relative, filePath, override }
}

// resolveOverrideOperation 允许覆盖定义使用 LF，而来源文件使用 CRLF，同时保持唯一匹配门禁。
function resolveOverrideOperation (source, operation, relative, targetRelative, index) {
  const candidates = [{ find: operation.find, replace: operation.replace }]
  if (operation.find.includes('\n') && !operation.find.includes('\r\n')) {
    candidates.push({
      find: operation.find.replace(/\n/g, '\r\n'),
      replace: operation.replace.replace(/\n/g, '\r\n')
    })
  }
  for (const candidate of candidates) {
    const matches = source.split(candidate.find).length - 1
    if (matches === operation.count) return candidate
    if (matches > 0) {
      throw new Error(`业务覆盖上下文不匹配：${relative} 第 ${index + 1} 个操作在 ${targetRelative} 命中 ${matches} 次，期望 ${operation.count} 次`)
    }
  }
  throw new Error(`业务覆盖上下文不匹配：${relative} 第 ${index + 1} 个操作在 ${targetRelative} 命中 0 次，期望 ${operation.count} 次`)
}

// applyLocalOverrides 只在映射完成后对候选做上下文唯一替换；任一上下文漂移都拒绝发布。
function applyLocalOverrides (candidateRoot, manifest) {
  const applied = []
  for (const definition of manifest.localOverrides || []) {
    const { relative, filePath, override } = readLocalOverride(definition)
    const targetRelative = assertSafeRelativePath(override.target, '业务覆盖目标')
    const targetPath = path.join(candidateRoot, targetRelative)
    if (!fs.existsSync(targetPath)) throw new Error(`业务覆盖目标不存在：${targetRelative}`)
    if (isExcluded(targetRelative, manifest.protected)) {
      throw new Error(`业务覆盖不能修改执行器保护路径：${targetRelative}`)
    }
    let source = fs.readFileSync(targetPath, 'utf8')
    for (const [index, operation] of override.operations.entries()) {
      const resolved = resolveOverrideOperation(source, operation, relative, targetRelative, index)
      source = source.replace(resolved.find, resolved.replace)
    }
    fs.writeFileSync(targetPath, source)
    applied.push({ file: relative, target: targetRelative, operations: override.operations.length })
    void filePath
  }
  return applied
}

// removeEmptyDirectories 清掉来源映射删除后留下的空目录，不影响候选之外的文件。
function removeEmptyDirectories (root) {
  const visit = directory => {
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      if (entry.isDirectory() && !entry.isSymbolicLink()) visit(path.join(directory, entry.name))
    }
    if (directory !== root && fs.readdirSync(directory).length === 0) fs.rmdirSync(directory)
  }
  if (fs.existsSync(root)) visit(root)
}

// stageFormMaking 只从受控 runtime-source 补齐 FormMaking 产物，不修改库源码或自行编译。
function stageFormMaking (candidateRoot) {
  const source = path.join(formMakingRoot, 'src', 'lib', 'vue-form-making', 'dist')
  const destination = path.join(candidateRoot, 'src', 'lib', 'vue-form-making', 'dist')
  const entry = path.join(source, 'FormMaking.common.js')
  if (!fs.existsSync(entry)) throw new Error(`缺少受控 FormMaking 产物：${entry}`)
  fs.rmSync(destination, { recursive: true, force: true })
  fs.mkdirSync(path.dirname(destination), { recursive: true })
  fs.cpSync(source, destination, { recursive: true })
  const artifactSkipNames = new Set(['.git', 'node_modules', '.npmrc', '.claude', '.omc', '.DS_Store', '__MACOSX'])
  if (digestDirectory(source, { skipNames: artifactSkipNames }) !== digestDirectory(destination, { skipNames: artifactSkipNames })) {
    throw new Error('FormMaking 产物复制后与受控副本不一致')
  }
}

// mappingHash 固定映射、保护边界和覆盖文件内容，覆盖变化必须让候选与构建缓存失效。
function mappingHash (manifest) {
  const overrides = (manifest.localOverrides || []).map(definition => {
    const { relative, filePath, override } = readLocalOverride(definition)
    return {
      file: relative,
      content: fs.readFileSync(filePath, 'utf8'),
      definition: override
    }
  })
  return crypto.createHash('sha256').update(canonicalJson({
    mappingVersion: manifest.mappingVersion,
    mappings: manifest.mappings,
    protected: manifest.protected,
    localOverrides: overrides
  })).digest('hex')
}

// readCurrentMetadata 读取已有合成候选的版本事实，损坏时由调用方阻止继续发布。
function readCurrentMetadata () {
  if (!fs.existsSync(metadataPath)) return null
  return readJson(metadataPath, '执行器合成源码元数据')
}

// verifyPreparedSource 验证候选字节、当次双来源提交事实、映射摘要和 FormMaking 产物。
export function verifyPreparedSource () {
  const manifest = readManifest()
  const metadata = readCurrentMetadata()
  if (!metadata || metadata.schemaVersion !== 2 || metadata.repository !== 'composed-flow-source') {
    throw new Error('缺少 F-053 合成源码元数据')
  }
  if (!fs.existsSync(targetRoot)) throw new Error('缺少 executor-service/flow-source 合成候选')
  const actualDigest = digestDirectory(targetRoot)
  if (actualDigest !== metadata.candidateDigest) {
    throw new Error(`合成候选摘要不一致：元数据 ${metadata.candidateDigest}，实际 ${actualDigest}`)
  }
  const expectedMappingHash = mappingHash(manifest)
  if (expectedMappingHash !== metadata.mappingHash) {
    throw new Error(`业务映射摘要不一致：元数据 ${metadata.mappingHash}，清单 ${expectedMappingHash}`)
  }
  if (!/^[0-9a-f]{40}$/.test(String(metadata.businessHead))) {
    throw new Error(`合成候选全过程提交号非法：${metadata.businessHead}`)
  }
  if (!/^[0-9a-f]{40}$/.test(String(metadata.baseHead))) {
    throw new Error(`合成候选基座提交号非法：${metadata.baseHead}`)
  }
  if (metadata.baseBranch !== manifest.base.branch ||
      metadata.baseRepository !== manifest.base.repository || metadata.baseRemote !== manifest.base.remote ||
      metadata.businessBranch !== manifest.business.branch ||
      metadata.businessRepository !== manifest.business.repository || metadata.businessRemote !== manifest.business.remote) {
    throw new Error('合成候选来源与当前来源清单不一致')
  }
  const formMaking = path.join(targetRoot, 'src', 'lib', 'vue-form-making', 'dist', 'FormMaking.common.js')
  if (!fs.existsSync(formMaking)) throw new Error('合成候选缺少 FormMaking.common.js')
  const appliedOverrides = applyLocalOverridesForVerification(targetRoot, manifest)
  if (appliedOverrides.length !== Number(metadata.localOverrideCount || 0)) {
    throw new Error(`合成源码覆盖数量不一致：元数据 ${metadata.localOverrideCount || 0}，清单 ${appliedOverrides.length}`)
  }
  return { manifest, metadata, candidateDigest: actualDigest }
}

// applyLocalOverridesForVerification 校验覆盖定义与候选最终字节一致，不重复写入候选。
function applyLocalOverridesForVerification (candidateRoot, manifest) {
  const applied = []
  for (const definition of manifest.localOverrides || []) {
    const { relative, override } = readLocalOverride(definition)
    const targetRelative = assertSafeRelativePath(override.target, '业务覆盖目标')
    const targetPath = path.join(candidateRoot, targetRelative)
    if (!fs.existsSync(targetPath)) throw new Error(`业务覆盖目标不存在：${targetRelative}`)
    for (const operation of override.operations) {
      const source = fs.readFileSync(targetPath, 'utf8')
      const replacementMatches = [
        operation.replace,
        operation.replace.includes('\n') && !operation.replace.includes('\r\n')
          ? operation.replace.replace(/\n/g, '\r\n')
          : null
      ].filter(Boolean).some(replacement => source.includes(replacement))
      if (!replacementMatches) {
        throw new Error(`合成候选未包含业务覆盖结果：${relative} -> ${targetRelative}`)
      }
    }
    applied.push(relative)
  }
  return applied
}

// writeMetadata 落盘基座、全过程、映射和候选摘要，供打包、构建与健康检查逐级对账。
function writeMetadata (manifest, base, business, candidateDigest, businessDigestValue, selectedCount, localOverrideCount) {
  const data = {
    schemaVersion: 2,
    repository: 'composed-flow-source',
    mappingVersion: manifest.mappingVersion,
    mappingHash: mappingHash(manifest),
    candidateDigest,
    baseRepository: base.repository,
    baseRemote: base.remote,
    baseBranch: base.branch,
    baseHead: base.head,
    baseDigest: base.digest,
    businessRepository: business.repository,
    businessRemote: business.remote,
    businessBranch: business.branch,
    businessHead: business.head,
    businessDigest: businessDigestValue,
    businessFileCount: selectedCount,
    localOverrideCount,
    preparedAt: new Date().toISOString()
  }
  fs.writeFileSync(metadataPath, `${JSON.stringify(data, null, 2)}\n`)
  return data
}

// prepareSource 每次从两个远端 test 解析最新 HEAD，仅将本次 SHA 用于一致构建和追溯。
export function prepareSource () {
  const manifest = readManifest()
  const base = resolveBranchHead(manifest.base, true)
  const business = resolveBranchHead(manifest.business, false)
  const mapHash = mappingHash(manifest)
  const current = readCurrentMetadata()
  if (current?.schemaVersion === 2 && current.baseHead === base.head &&
      current.businessHead === business.head && current.mappingHash === mapHash &&
      fs.existsSync(targetRoot)) {
    const verified = verifyPreparedSource()
    process.stdout.write(`[GIT] 两侧 test 均为最新：基座 ${base.head}，全过程 ${business.head}\n`)
    return verified.metadata
  }

  let baseRepo = null
  let businessRepo = null
  let candidateName = null
  let candidateRoot = null
  try {
    baseRepo = cloneAt(base, 'flow-source-base')
    businessRepo = cloneAt(business, 'flow-source-business')
    candidateName = `${generatedTargetName}.sync-${process.pid}`
    candidateRoot = path.join(serviceRoot, candidateName)
    assertGeneratedPath(candidateRoot, candidateName)
    removeGeneratedPath(candidateRoot, candidateName)
    fs.mkdirSync(candidateRoot, { recursive: true })
    copyBaseRepository(baseRepo, candidateRoot)
    const baseDigest = digestDirectory(candidateRoot)
    const selected = collectMappingFiles(businessRepo, manifest)
    applyBusinessMappings(businessRepo, candidateRoot, manifest, selected)
    const appliedOverrides = applyLocalOverrides(candidateRoot, manifest)
    removeEmptyDirectories(path.join(candidateRoot, 'src'))
    removeEmptyDirectories(path.join(candidateRoot, 'public'))
    stageFormMaking(candidateRoot)
    const businessDigestValue = digestFiles(businessRepo, selected.map(item => item.sourceRelative))
    const candidateDigest = digestDirectory(candidateRoot)
    const metadata = {
      schemaVersion: 2,
      repository: 'composed-flow-source',
      mappingVersion: manifest.mappingVersion,
      mappingHash: mapHash,
      candidateDigest,
      baseRepository: base.repository,
      baseRemote: base.remote,
      baseBranch: base.branch,
      baseHead: base.head,
      baseDigest,
      businessRepository: business.repository,
      businessRemote: business.remote,
      businessBranch: business.branch,
      businessHead: business.head,
      businessDigest: businessDigestValue,
      businessFileCount: selected.length,
      localOverrideCount: appliedOverrides.length,
      preparedAt: new Date().toISOString()
    }

    const backupName = `${generatedTargetName}.old-${process.pid}`
    const backupRoot = path.join(serviceRoot, backupName)
    assertGeneratedPath(backupRoot, backupName)
    removeGeneratedPath(backupRoot, backupName)
    if (fs.existsSync(targetRoot)) {
      assertGeneratedPath(targetRoot, generatedTargetName)
      fs.renameSync(targetRoot, backupRoot)
    }
    try {
      fs.renameSync(candidateRoot, targetRoot)
      if (fs.existsSync(backupRoot)) removeGeneratedPath(backupRoot, backupName)
    } catch (error) {
      if (!fs.existsSync(targetRoot) && fs.existsSync(backupRoot)) fs.renameSync(backupRoot, targetRoot)
      throw error
    }
    writeMetadata(manifest, { ...base, digest: baseDigest }, business, candidateDigest, businessDigestValue, selected.length, appliedOverrides.length)
    process.stdout.write(`[GIT] 合成完成：基座 ${base.head} + 全过程 test ${business.head}，纳入 ${metadata.businessFileCount} 个业务文件\n`)
    return metadata
  } finally {
    if (baseRepo && fs.existsSync(baseRepo)) removeGeneratedPath(baseRepo, `flow-source-base-${process.pid}`)
    if (businessRepo && fs.existsSync(businessRepo)) removeGeneratedPath(businessRepo, `flow-source-business-${process.pid}`)
    if (candidateRoot && candidateName && fs.existsSync(candidateRoot)) removeGeneratedPath(candidateRoot, candidateName)
  }
}

function main () {
  try {
    if (process.argv.includes('--verify-prepared')) {
      verifyPreparedSource()
      process.stdout.write('[OK] F-053 合成源码摘要校验通过\n')
      return
    }
    prepareSource()
  } catch (error) {
    fail(String(error?.message ?? error))
  }
}

if (process.argv[1] && path.resolve(process.argv[1]) === path.resolve(fileURLToPath(import.meta.url))) {
  main()
}
