#!/usr/bin/env node

// T002（Windows 迁移修订）：flow-source 不再复制任何本机静态目录，改为直接 git 同步参考仓库。
//
// 旧实现把原项目在 macOS 外接盘上的绝对路径整体复制成快照，导致：
//   ① 依赖只在 Mac 上存在的挂载盘路径，迁移到 Windows 后必然报「原项目不存在」；
//   ② 快照脱离版本控制，无法判断本地是否被改脏；
//   ③ 与 form-runtime 的 sync-manifest 出现两套来源口径。
// 现统一为：从固定远端 + 固定分支 clone/快进，路径全部相对工作区解析。
import childProcess from 'node:child_process'
import crypto from 'node:crypto'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const serviceRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const workspaceRoot = path.resolve(serviceRoot, '..')
const manifestPath = path.join(serviceRoot, 'flow-source-manifest.json')
const targetRoot = path.join(serviceRoot, 'flow-source')
const metadataPath = path.join(serviceRoot, 'flow-source.json')
const lockPath = path.join(serviceRoot, 'flow-source.lock.json')
const configRuntime = path.join(workspaceRoot, 'form-runtime', 'runtime-source')

// SKIP_NAMES 是与运行时无关的本机噪声，git checkout 之后按需清理，不进构建输入。
const SKIP_NAMES = new Set(['.git', 'node_modules', '.npmrc', '.claude', '.omc', '.DS_Store', '__MACOSX'])

function fail (message) {
  process.stderr.write(`错误：${message}\n`)
  process.exit(1)
}

// git 只执行固定只读参数并返回文本；调用方负责保证参数来自清单，不接受任意输入。
function git (root, args, options = {}) {
  return childProcess.execFileSync('git', ['-C', root, ...args], { encoding: 'utf8', timeout: 30000, ...options }).trim()
}
// gitRaw 与 git 相同，但不裁剪首尾空白。
// porcelain 状态行的第一列就是「暂存区状态」，为空时是一个前导空格；
// 用 git() 的 trim() 会把第一行的前导空格吃掉，导致把「未暂存的删除」误读成「已暂存的修改」。
function gitRaw (root, args) {
  return childProcess.execFileSync('git', ['-C', root, ...args], { encoding: 'utf8' })
}

// readManifest 读取本项目固定同步清单；远端、分支和目标目录都不允许由命令行或环境变量覆盖。
function readManifest () {
  try {
    return JSON.parse(fs.readFileSync(manifestPath, 'utf8'))
  } catch (error) {
    fail(`同步清单不可读：${manifestPath}（${String(error?.message ?? error)}）`)
  }
}

// readLock 读取上次同步落盘的锁文件；首次同步没有锁文件，按全量同步处理。
function readLock () {
  if (!fs.existsSync(lockPath)) return null
  try {
    return JSON.parse(fs.readFileSync(lockPath, 'utf8'))
  } catch {
    return null
  }
}

// digestDirectory 计算目录内容摘要，用于证明「同一版本的 flow-source 字节一致」。
// 元数据文件本身不参与摘要，避免自引用导致每次都判定为变化。
//
// dist/ 是 build-native-page 在本机生成的页面产物，不属于 git 同步内容，
// 也不参与摘要：它是构建结果而非源码，纳入摘要会让「同步后摘要」随构建而变。
function digestDirectory (root) {
  const hash = crypto.createHash('sha256')
  const files = []
  const visit = (directory, prefix) => {
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      if (SKIP_NAMES.has(entry.name)) continue
      const relative = prefix ? `${prefix}/${entry.name}` : entry.name
      if (relative === 'dist' || relative.startsWith('dist/')) continue
      if (entry.isDirectory()) visit(path.join(directory, entry.name), relative)
      else files.push(relative)
    }
  }
  visit(root, '')
  for (const relative of files.sort()) {
    hash.update(relative)
    hash.update('\0')
    hash.update(fs.readFileSync(path.join(root, relative)))
    hash.update('\0')
  }
  return hash.digest('hex')
}

// removeNoise 清掉 git checkout 之后仍可能存在的本机噪声目录（.DS_Store 等不参与摘要，但也无意义留盘）。
function removeNoise (root) {
  for (const name of SKIP_NAMES) {
    if (name === '.git') continue
    fs.rmSync(path.join(root, name), { recursive: true, force: true })
  }
}

// isCheckedOutAt 判断目标目录当前是否就是「清单远端 + 清单分支 + 清单 HEAD」的受控检出。
// 这是幂等判据，不使用时间戳或本地缓存。
//
// 脏判据必须只看「内容被改动」的文件，不能把「脚本主动删除的本机噪声」也算脏：
// .npmrc/.claude/.omc 是上游 tracked 文件，removeNoise 会按设计删掉它们，
// status 于是长期显示 ` D`。若把这些删除也算脏，同步永远判不出「已是最新」，
// 每次执行都会整个重新 clone（实测每次都重新下载 722 个文件）。
// 因此这里只放行 status 第一列（暂存区）为空、第二列仅为 `D` 的行，
// 任何内容修改（M）、新增或暂存改动仍然判定为脏并触发重新同步。
function isCheckedOutAt (manifest, head) {
  if (!fs.existsSync(path.join(targetRoot, '.git'))) return false
  const remote = git(targetRoot, ['remote', 'get-url', 'origin'])
  if (remote !== manifest.remote) return false
  if (git(targetRoot, ['rev-parse', 'HEAD']) !== head) return false
  if (hasContentChanges()) return false
  return git(targetRoot, ['ls-tree', '-r', '--name-only', 'HEAD']).length > 0
}

// hasContentChanges 判断工作树里是否存在「本机噪声删除」以外的任何变化。
// 必须用 gitRaw：porcelain 行的首列空白是有意义的（空格=暂存区无变化）。
function hasContentChanges () {
  const lines = gitRaw(targetRoot, ['status', '--porcelain', '--untracked-files=no'])
    .split('\n')
    .map(line => line.trimEnd())
    .filter(line => line !== '')
  return lines.some(line => {
    const index = line[0]
    const worktree = line[1]
    // 只允许「未暂存的工作树删除」这一种状态；其余一律视为有真实变化。
    return index !== ' ' || worktree !== 'D'
  })
}

// cloneAt 做一次干净克隆：先删残留目录，再 fetch 到临时目录并检出。
// 不直接 clone --branch 是为了在同一处统一处理锁定的 commit（分支 HEAD 已前移时仍可复现）。
function cloneAt (manifest, head, remoteOverride) {
  fs.rmSync(targetRoot, { recursive: true, force: true })
  fs.mkdirSync(targetRoot, { recursive: true })
  const remote = remoteOverride || manifest.remote
  git(targetRoot, ['init', '--quiet'])
  git(targetRoot, ['remote', 'add', 'origin', remote])
  git(targetRoot, ['fetch', '--quiet', '--depth', '1', 'origin', head])
  git(targetRoot, ['checkout', '--quiet', '--detach', 'FETCH_HEAD'])
  git(targetRoot, ['branch', '--quiet', '--force', manifest.branch, 'HEAD'])
  removeNoise(targetRoot)
}
// assertNotConfigRuntime 挡住把 flow-source 指向 form-runtime 运行源码的误配置：
// 两者都必须各自独立，跨用会让执行服务装载与表单运行时互相污染。
function assertNotConfigRuntime () {
  if (!fs.existsSync(configRuntime)) return
  if (path.resolve(targetRoot) === path.resolve(configRuntime)) {
    fail('flow-source 目标不能是 form-runtime/runtime-source')
  }
}

// FORMMAKING_RELATIVE 是 FormMaking 产物在仓库内的固定位置。
const FORMMAKING_RELATIVE = path.join('src', 'lib', 'vue-form-making', 'dist')
// FORMMAKING_ENTRY 是运行时必须存在的入口文件；只校验目录存在会把半截产物当成功。
const FORMMAKING_ENTRY = 'FormMaking.common.js'

// stageFormMaking 把 FormMaking 构建产物放进 clone 出来的 flow-source。
//
// 为什么必须单独补：rsh-flow-components 的 Git 只跟踪 src/lib/vue-form-making/package.json，
// dist 是构建产物且被上游忽略（`git ls-tree` 实测只剩一个 package.json）。
// 产物在 form-runtime 侧作为 preservedTargetPaths 受控保存，是工作区里唯一可信副本；
// 这里只做「复制 + 逐字节校验」，绝不自己编译，避免产物来源出现第二个口径。
function stageFormMaking () {
  const target = path.join(targetRoot, FORMMAKING_RELATIVE)
  const source = path.join(configRuntime, FORMMAKING_RELATIVE)
  const targetEntry = path.join(target, FORMMAKING_ENTRY)
  const sourceEntry = path.join(source, FORMMAKING_ENTRY)

  if (fs.existsSync(targetEntry)) {
    process.stdout.write('[OK] FormMaking 产物已在快照内\n')
    return
  }
  if (!fs.existsSync(sourceEntry)) {
    fail(`缺少 FormMaking 产物：${sourceEntry}（受控副本不存在，拒绝自行编译）`)
  }

  fs.rmSync(target, { recursive: true, force: true })
  fs.mkdirSync(path.dirname(target), { recursive: true })
  fs.cpSync(source, target, { recursive: true })

  // 逐字节校验：复制后必须与受控副本完全一致，否则装载原生 /flow 页面会用到未知产物。
  if (digestDirectory(target) !== digestDirectory(source)) {
    fail('FormMaking 产物复制后与受控副本不一致')
  }
  process.stdout.write('[OK] FormMaking 产物已从受控副本补齐\n')
}

function resolveRemoteCandidates (remote) {
  const candidates = [remote]
  // 内网 GitLab 在 Windows/Node 环境下若 SSH 密钥未加载，提供等价 HTTP 路径作为自动降级候选。
  if (remote.startsWith('git@') && remote.includes(':')) {
    const match = remote.match(/^git@([^:]+):(.+)$/)
    if (match) {
      candidates.push(`http://${match[1]}/${match[2]}`)
    }
  }
  return candidates
}

// fetchRemoteLatestHEAD 尝试探测远端最新 HEAD。
function fetchRemoteLatestHEAD (manifest) {
  const candidates = resolveRemoteCandidates(manifest.remote)
  for (const remote of candidates) {
    try {
      const output = childProcess.execFileSync('git', ['ls-remote', remote, `refs/heads/${manifest.branch}`], {
        encoding: 'utf8',
        timeout: 10000
      }).trim()
      if (output) {
        const match = output.match(/^([0-9a-f]{40})\s+/)
        if (match) return { head: match[1], remote }
      }
    } catch {
      // 继续尝试下一候选
    }
  }
  return null
}

function main () {
  const manifest = readManifest()
  assertNotConfigRuntime()

  // 自动检测并拉取远端最新代码：
  // 当开启 EXECUTOR_PULL_LATEST=1 或传入 --pull-latest 时，自动探测远端对应分支最新 HEAD；
  // 若探测成功则跟进最新 HEAD，若网络不可达则安全回落到清单锁定的版本。
  const pullLatest = process.env.EXECUTOR_PULL_LATEST === '1' || process.argv.includes('--pull-latest')
  let head = manifest.head
  let targetRemote = manifest.remote

  if (pullLatest) {
    const remoteInfo = fetchRemoteLatestHEAD(manifest)
    if (remoteInfo) {
      if (remoteInfo.head !== head) {
        process.stdout.write(`[GIT] 远端 ${manifest.branch} 分支发现新提交：${head.slice(0, 7)} -> ${remoteInfo.head.slice(0, 7)}\n`)
      }
      head = remoteInfo.head
      targetRemote = remoteInfo.remote
    } else {
      process.stdout.write(`[GIT] 远端分支不可达，安全回退至清单锁定版本 ${head.slice(0, 7)}\n`)
    }
  }

  if (!/^[0-9a-f]{40}$/.test(String(head))) fail(`清单 head 必须是完整提交号：${head}`)
  if (!manifest.remote || !manifest.branch) fail('清单必须给出 remote 与 branch')

  if (isCheckedOutAt({ ...manifest, remote: targetRemote }, head)) {
    const digest = digestDirectory(targetRoot)
    writeMetadata(manifest, head, digest, 'unchanged')
    process.stdout.write(`[GIT] flow-source 已是最新 ${head.slice(0, 7)}（未变更）\n`)
    return
  }

  // cloneAt 内部的 fetch 就是远端可达性判据：目标提交可能已被分支前移甩开，
  // 这里不能用 ls-remote 按 SHA 探测（服务端只按 ref 过滤，已前移的提交会查不到，
  // 但按 SHA 直接 fetch 依然成立）。失败时给出明确原因，绝不静默留下半同步目录。
  try {
    cloneAt(manifest, head, targetRemote)
  } catch (error) {
    // 如果使用 targetRemote 失败，尝试候选降级
    let cloned = false
    const candidates = resolveRemoteCandidates(manifest.remote)
    for (const altRemote of candidates) {
      if (altRemote === targetRemote) continue
      try {
        cloneAt(manifest, head, altRemote)
        cloned = true
        break
      } catch {
        // 继续
      }
    }
    if (!cloned) {
      fail(`从 ${manifest.remote} 同步提交 ${head} 失败：${String(error?.message ?? error)}`)
    }
  }
  const actual = git(targetRoot, ['rev-parse', 'HEAD'])
  if (actual !== head) fail(`检出提交不符：期望 ${head}，实际 ${actual}`)

  // FormMaking 产物不在 rsh-flow-components 的 Git 里（该仓库只跟踪
  // src/lib/vue-form-making/package.json，dist 是构建产物且被上游忽略），
  // 因此不能指望 clone 带出它，必须从受控副本补齐并逐字节校验。
  stageFormMaking()
  if (!fs.existsSync(path.join(targetRoot, 'src', 'api', 'index.js'))) {
    fail('同步后缺少 src/api/index.js（端点真名清单来源）')
  }

  const digest = digestDirectory(targetRoot)
  writeMetadata(manifest, head, digest, 'synced')
  process.stdout.write(`[GIT] ${manifest.remote}@${head.slice(0, 7)} -> flow-source/\n`)
}

// writeMetadata 落盘同步事实：远端、分支、HEAD、内容摘要与同步方式。
// digest 供 build-form-making 与测试核对「本地快照未被手改」。
function writeMetadata (manifest, head, digest, action) {
  const metadata = {
    repository: manifest.repository,
    remote: manifest.remote,
    branch: manifest.branch,
    head,
    subject: git(targetRoot, ['log', '-1', '--format=%s', head]),
    digest,
    action,
    syncedAt: new Date().toISOString()
  }
  fs.writeFileSync(metadataPath, `${JSON.stringify(metadata, null, 2)}\n`)
  fs.writeFileSync(lockPath, `${JSON.stringify({ head, digest }, null, 2)}\n`)
}

main()
