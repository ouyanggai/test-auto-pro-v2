#!/usr/bin/env node

// FormMaking 产物由 sync-flow-source.js 从 form-runtime/runtime-source 的受控副本补齐并逐字节校验。
// 本脚本只核对快照内已有 dist，不自己编译，不复用 form-runtime/runtime-source 作为构建输入。
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const serviceRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const destFile = path.join(serviceRoot, 'flow-source', 'src', 'lib', 'vue-form-making', 'dist', 'FormMaking.common.js')

if (!fs.existsSync(destFile)) {
  process.stderr.write('错误：缺少 FormMaking 产物，先执行 pnpm --dir executor-service sync-source（按 git 同步并从受控副本补齐）\n')
  process.exit(1)
}
process.stdout.write(`[OK] FormMaking 产物已在快照内：${destFile}\n`)
