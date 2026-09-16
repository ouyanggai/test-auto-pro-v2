#!/usr/bin/env node

// T002：把原项目 /Volumes/oygsky/bigsys/rsh-flow-components 复制为独立 flow-source。
// 后续一键升级与原项目相同：从其 sync-manifest 同步目标平台代码。
// 不复用 form-runtime/runtime-source，不改原项目工作树。
import childProcess from 'node:child_process'
import crypto from 'node:crypto'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const PINNED_HEAD = 'be355ae309479a0c37598c0ee95ba724d062e5f1'
const PINNED_BRANCH = 'test'
const DEFAULT_SOURCE = '/Volumes/oygsky/bigsys/rsh-flow-components'
const SKIP_NAMES = new Set(['.git', 'node_modules', '.npmrc', '.claude', '.omc', '.DS_Store', '__MACOSX'])

const serviceRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const targetRoot = path.join(serviceRoot, 'flow-source')
const metadataPath = path.join(serviceRoot, 'flow-source.json')
const configRuntime = path.resolve(serviceRoot, '..', 'form-runtime', 'runtime-source')

function git (root, args) {
  return childProcess.execFileSync('git', ['-C', root, ...args], { encoding: 'utf8' }).trim()
}

function fail (message) {
  process.stderr.write(`错误：${message}\n`)
  process.exit(1)
}

function digestDirectory (root) {
  const hash = crypto.createHash('sha256')
  const files = []
  const visit = (directory, prefix) => {
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      const relative = prefix ? `${prefix}/${entry.name}` : entry.name
      if (entry.isDirectory()) visit(path.join(directory, entry.name), relative)
      else files.push(relative)
    }
  }
  if (fs.existsSync(root)) visit(root, '')
  for (const relative of files.sort()) {
    hash.update(relative)
    hash.update('\0')
    hash.update(fs.readFileSync(path.join(root, relative)))
    hash.update('\0')
  }
  return hash.digest('hex')
}

function copyTree (sourceRoot) {
  fs.rmSync(targetRoot, { recursive: true, force: true })
  fs.mkdirSync(targetRoot, { recursive: true })
  fs.cpSync(sourceRoot, targetRoot, {
    recursive: true,
    filter: (source) => {
      const name = path.basename(source)
      if (SKIP_NAMES.has(name)) return false
      const relative = path.relative(sourceRoot, source)
      if (relative === 'dist' || relative.startsWith(`dist${path.sep}`)) return false
      return true
    }
  })
}

function main () {
  const sourceRoot = process.env.EXECUTOR_FLOW_SOURCE_ROOT || DEFAULT_SOURCE
  if (!fs.existsSync(path.join(sourceRoot, '.git'))) {
    fail(`原项目不存在：${sourceRoot}`)
  }
  if (fs.existsSync(configRuntime) && fs.realpathSync(sourceRoot) === fs.realpathSync(configRuntime)) {
    fail('来源不能是 form-runtime/runtime-source')
  }
  const branch = git(sourceRoot, ['rev-parse', '--abbrev-ref', 'HEAD'])
  const head = git(sourceRoot, ['rev-parse', 'HEAD'])
  if (branch !== PINNED_BRANCH) {
    fail(`原项目必须在 ${PINNED_BRANCH} 分支，实际 ${branch}`)
  }
  if (head !== PINNED_HEAD) {
    fail(`原项目 HEAD 必须是 ${PINNED_HEAD}，实际 ${head}`)
  }
  copyTree(sourceRoot)
  const formMaking = path.join(targetRoot, 'src', 'lib', 'vue-form-making', 'dist', 'FormMaking.common.js')
  if (!fs.existsSync(formMaking)) {
    fail('复制后缺少原项目 FormMaking 产物 src/lib/vue-form-making/dist/FormMaking.common.js')
  }
  const metadata = {
    repository: 'rsh-flow-components',
    sourceRoot,
    branch: PINNED_BRANCH,
    head: PINNED_HEAD,
    subject: git(sourceRoot, ['log', '-1', '--format=%s', PINNED_HEAD]),
    targetDigest: digestDirectory(targetRoot),
    copiedAt: new Date().toISOString()
  }
  fs.writeFileSync(metadataPath, `${JSON.stringify(metadata, null, 2)}\n`)
  process.stdout.write(`[COPY] ${sourceRoot}@${PINNED_HEAD.slice(0, 7)} -> flow-source/\n`)
}

main()
