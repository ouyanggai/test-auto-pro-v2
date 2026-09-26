#!/usr/bin/env node

// build-native-page 把 flow-source 构建成执行服务装载用的原生 /flow 页面产物。
//
// 迁移修订：旧实现直接消费一个 macOS 上手工产出的 flow-source/dist.zip，Windows 上既没有
// 那个 zip、系统也不带 unzip，页面装载必然失败。现在改为在本机真实构建：
// 产物固定落在 flow-source/dist/（vue.config.js 的 outputDir），由 extractPageDist 直接消费。
//
// 构建属于重操作（vue-cli-service 全量打包），因此只在 dist 缺失或源码变化时才执行；
// 用例可用 EXECUTOR_FORCE_PAGE_BUILD=1 强制重建。
import childProcess from 'node:child_process'
import crypto from 'node:crypto'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const serviceRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const flowSource = path.join(serviceRoot, 'flow-source')
const distDir = path.join(flowSource, 'dist')
const stampPath = path.join(flowSource, '.native-page-build.json')
const babelConfigPath = path.join(flowSource, 'babel.config.js')

// 构建输入是这些目录/文件的内容摘要；任一变化都应触发重建。
const BUILD_INPUTS = ['src', 'public', 'package.json', 'vue.config.js', 'babel.config.js']

function fail (message) {
  process.stderr.write(`错误：${message}\n`)
  process.exit(1)
}

// digestInputs 计算构建输入摘要，用于判定「产物是否已经对应当前源码」。
function digestInputs () {
  const hash = crypto.createHash('sha256')
  const files = []
  const visit = (absolute, relative) => {
    const stat = fs.statSync(absolute)
    if (stat.isDirectory()) {
      for (const entry of fs.readdirSync(absolute, { withFileTypes: true }).sort((a, b) => a.name.localeCompare(b.name))) {
        if (entry.name === 'node_modules' || entry.name === 'dist') continue
        visit(path.join(absolute, entry.name), `${relative}/${entry.name}`)
      }
      return
    }
    files.push(relative)
  }
  for (const input of BUILD_INPUTS) {
    const absolute = path.join(flowSource, input)
    if (fs.existsSync(absolute)) visit(absolute, input)
  }
  for (const relative of files.sort()) {
    hash.update(relative)
    hash.update('\0')
    hash.update(fs.readFileSync(path.join(flowSource, relative)))
    hash.update('\0')
  }
  return hash.digest('hex')
}

// isPageFresh 判断现有 dist 是否已经对应当前源码。
function isPageFresh (digest) {
  if (!fs.existsSync(path.join(distDir, 'index.html'))) return false
  if (process.env.EXECUTOR_FORCE_PAGE_BUILD === '1') return false
  if (!fs.existsSync(stampPath)) return false
  try {
    return JSON.parse(fs.readFileSync(stampPath, 'utf8')).inputDigest === digest
  } catch {
    return false
  }
}

// ensureDependencies 安装页面构建依赖；flow-source 自带 package.json 与 lock，独立于工作区。
//
// 必须带 --legacy-peer-deps：上游 package.json 存在无法自洽的 peer 依赖
// （@wangeditor/editor-for-vue@1.0.2 要求 vue ^2.6.14，而同一 package.json 把 vue 钉在 2.6.11），
// 上游提交的 package-lock.json 本身就是按 legacy 解析生成的（node_modules/vue = 2.6.11）。
// 不加该标志 npm ci 直接 ERESOLVE 失败；加 --force 则会违背 lock 的解析结果，故取 legacy。
function ensureDependencies () {
  if (fs.existsSync(path.join(flowSource, 'node_modules', '@vue', 'cli-service'))) return
  process.stdout.write('[BUILD] 安装 flow-source 构建依赖（首次较慢）\n')
  const result = childProcess.spawnSync('npm', ['ci', '--legacy-peer-deps', '--no-audit', '--no-fund'], {
    cwd: flowSource,
    stdio: 'inherit',
    shell: process.platform === 'win32'
  })
  if (result.status !== 0) fail('flow-source 依赖安装失败')
}

// runBuild 执行真实构建：VUE_APP_FLAG=test 对应目标测试环境，与既有验收口径一致。
//
// 不走 `npm run build:test`：上游 prebuild:test 用 `SYNC_ENV=test node ...` 这种
// Unix 专有的内联环境变量写法，Windows cmd 下直接失败。那两步是把跨模块源码同步进
// flow-source 的准备步骤，需要另一套目标工程目录，本机没有也不该有；
// 这里直接调用同一个 vue-cli-service build，构建输入仍完全来自已 git 同步的源码。
function runBuild () {
  process.stdout.write('[BUILD] 构建原生 /flow 页面（vue-cli-service build）\n')
  fs.rmSync(distDir, { recursive: true, force: true })
  const cliService = path.join(flowSource, 'node_modules', '@vue', 'cli-service', 'bin', 'vue-cli-service.js')
  if (!fs.existsSync(cliService)) fail('缺少 @vue/cli-service，先安装 flow-source 依赖')
  const result = childProcess.spawnSync(process.execPath, [cliService, 'build'], {
    cwd: flowSource,
    stdio: 'inherit',
    env: { ...process.env, VUE_APP_FLAG: 'test', NODE_ENV: 'production' }
  })
  if (result.status !== 0) fail('原生页面构建失败')
  if (!fs.existsSync(path.join(distDir, 'index.html'))) {
    fail('构建结束但 dist/index.html 不存在')
  }
}

// ensureBabelConfig 兜底恢复 flow-source/babel.config.js。
// 该文件被 Git 忽略、只随远端克隆落地;内网 git 不可达导致的克隆超时会把它连同 dist 一并清空,
// 缺失时 vue-cli-service build 直接失败。内容与参考仓库 rsh-flow-components 一致(vue-cli 标准预设),
// 只在缺失时写入、已存在则原样保留,避免与克隆结果产生第二处口径。
function ensureBabelConfig () {
  if (fs.existsSync(babelConfigPath)) return
  process.stdout.write('[BUILD] 恢复缺失的 flow-source/babel.config.js\n')
  fs.writeFileSync(babelConfigPath, "module.exports = {\n  presets: ['@vue/cli-plugin-babel/preset'],\n  compact: false\n};\n")
}

function main () {
  if (!fs.existsSync(flowSource)) fail('缺少 executor-service/flow-source，先执行 pnpm --dir executor-service sync-source')
  if (!fs.existsSync(path.join(flowSource, 'package.json'))) fail('flow-source 缺少 package.json')
  // babel.config.js 是构建输入:缺失会让 digest 计算与 vue-cli 构建同时失败,先兜底恢复再算摘要。
  ensureBabelConfig()

  const digest = digestInputs()
  if (isPageFresh(digest)) {
    process.stdout.write('[OK] 原生 /flow 页面产物已是最新，跳过构建\n')
    return
  }

  ensureDependencies()
  runBuild()
  fs.writeFileSync(stampPath, `${JSON.stringify({ inputDigest: digest, builtAt: new Date().toISOString() }, null, 2)}\n`)
  process.stdout.write('[OK] 原生 /flow 页面产物已生成：flow-source/dist\n')
}

main()
