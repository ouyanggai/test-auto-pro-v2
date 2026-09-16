#!/usr/bin/env node

// FormMaking 产物随原项目 /Volumes/oygsky/bigsys/rsh-flow-components 一并复制。
// 本脚本只核对快照内已有 dist，不自己编译，不复用 form-runtime/runtime-source。
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const serviceRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const destFile = path.join(serviceRoot, 'flow-source', 'src', 'lib', 'vue-form-making', 'dist', 'FormMaking.common.js')

if (!fs.existsSync(destFile)) {
  process.stderr.write('错误：缺少 FormMaking 产物，先执行 pnpm --dir executor-service sync-source（从原项目复制）\n')
  process.exit(1)
}
process.stdout.write(`[OK] FormMaking 产物已在快照内：${destFile}\n`)
