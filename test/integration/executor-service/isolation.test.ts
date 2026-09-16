import assert from 'node:assert/strict'
import { spawn } from 'node:child_process'
import crypto from 'node:crypto'
import fs from 'node:fs'
import path from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../..')
const executorMeta = path.join(root, 'executor-service', 'flow-source.json')
const formMeta = path.join(root, 'form-runtime', 'runtime-source', '.f007-source.json')
const formVueConfig = path.join(root, 'form-runtime', 'vue.config.js')
const formApp = path.join(root, 'form-runtime', 'src', 'App.vue')
const formProtocol = path.join(root, 'form-runtime', 'src', 'runtime', 'protocol.js')
const formFrame = path.join(root, 'web', 'src', 'features', 'path-configuration', 'FormRuntimeFrame.vue')

function sha (file: string): string {
  return crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex')
}

function readJSON (file: string): Record<string, unknown> {
  return JSON.parse(fs.readFileSync(file, 'utf8')) as Record<string, unknown>
}

test('执行服务停止后配置协议仍支持 getValues', async () => {
  const protocol = fs.readFileSync(formProtocol, 'utf8')
  const app = fs.readFileSync(formApp, 'utf8')
  const frame = fs.readFileSync(formFrame, 'utf8')
  assert.match(protocol, /f007-form-runtime\/v1/)
  assert.match(protocol, /getValues/)
  assert.match(app, /command\.type === 'getValues'/)
  assert.match(frame, /defineExpose\(\{[^}]*getValues/)
  assert.doesNotMatch(app, /executor-service/)
  assert.doesNotMatch(protocol, /exec-engine/)

  const check = spawn('node', ['--experimental-strip-types', 'src/health-check.ts'], {
    cwd: path.join(root, 'executor-service'),
    env: { ...process.env, EXECUTOR_PORT: '1', RUNTIME_EXECUTOR_URL: 'http://127.0.0.1:1' }
  })
  const [code] = await Promise.all([
    new Promise<number>(resolve => check.on('close', value => resolve(value ?? 1)))
  ])
  assert.notEqual(code, 0)
})

test('改一侧快照文件不改变另一侧 HEAD 元数据', () => {
  const dir = fs.mkdtempSync(path.join(root, '.runtime', 'iso-'))
  const execCopy = path.join(dir, 'flow-source.json')
  const formCopy = path.join(dir, 'form-source.json')
  fs.copyFileSync(executorMeta, execCopy)
  fs.copyFileSync(formMeta, formCopy)
  const formBefore = sha(formCopy)
  const execBefore = sha(execCopy)
  const exec = readJSON(execCopy)
  exec.head = 'deadbeefdeadbeefdeadbeefdeadbeefdeadbeef'
  fs.writeFileSync(execCopy, JSON.stringify(exec, null, 2))
  assert.equal(sha(formCopy), formBefore)
  assert.notEqual(sha(execCopy), execBefore)
  fs.rmSync(dir, { recursive: true, force: true })
})



test('两套构建产物目录互不覆盖', () => {
  const vueConfig = fs.readFileSync(formVueConfig, 'utf8')
  assert.match(vueConfig, /web\/dist\/form-runtime/)
  assert.match(vueConfig, /publicPath:\s*'\/form-runtime\/'/)
  const ignore = fs.readFileSync(path.join(root, 'executor-service', '.gitignore'), 'utf8')
  assert.match(ignore, /\/page\/dist\//)
  assert.match(ignore, /\/flow-source\/dist\//)
  assert.doesNotMatch(ignore, /web\/dist\/form-runtime/)
  const formRuntimeSrc = fs.readFileSync(path.join(root, 'form-runtime', 'src', 'main.js'), 'utf8')
  assert.doesNotMatch(formRuntimeSrc, /executor-service/)
  assert.doesNotMatch(formRuntimeSrc, /\/flow\/initiate/)
})
