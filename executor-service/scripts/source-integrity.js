#!/usr/bin/env node

import crypto from 'node:crypto'
import fs from 'node:fs'
import path from 'node:path'

// 构建输入不包含 Git 元数据、依赖、页面产物和构建戳；这些内容不能改变源码版本事实。
export const INTEGRITY_SKIP_NAMES = new Set([
  '.git',
  'node_modules',
  '.native-page-build.json',
  '.npmrc',
  '.claude',
  '.omc',
  '.DS_Store',
  '__MACOSX'
])

// assertSafeRelativePath 拒绝绝对路径和目录穿越，保证清单路径只能落在候选目录内。
export function assertSafeRelativePath (relative, label = '路径') {
  const normalized = String(relative).replaceAll('\\', '/')
  const parsed = path.posix.normalize(normalized)
  if (!normalized || parsed === '.' || parsed.startsWith('/') || /^[a-z]:/i.test(parsed) ||
      normalized.split('/').includes('..') || normalized.includes('\0')) {
    throw new Error(`${label}不安全：${relative}`)
  }
  return parsed
}

// digestDirectory 对候选源码的路径和原始字节做稳定摘要，保证同一输入在不同平台得到同一事实。
export function digestDirectory (root, options = {}) {
  const skipNames = options.skipNames || INTEGRITY_SKIP_NAMES
  const files = []
  const visit = (directory, prefix) => {
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      const skipRootBuildOutput = prefix === '' && entry.name === 'dist'
      if (skipRootBuildOutput || skipNames.has(entry.name)) continue
      const relative = prefix ? `${prefix}/${entry.name}` : entry.name
      const absolute = path.join(directory, entry.name)
      if (entry.isSymbolicLink()) throw new Error(`候选源码不能包含符号链接：${relative}`)
      if (entry.isDirectory()) visit(absolute, relative)
      else files.push(relative)
    }
  }
  visit(root, '')
  const hash = crypto.createHash('sha256')
  for (const relative of files.sort()) {
    hash.update(relative)
    hash.update('\0')
    hash.update(fs.readFileSync(path.join(root, relative)))
    hash.update('\0')
  }
  return hash.digest('hex')
}

// digestFiles 对业务映射实际纳入候选的源文件做摘要，避免只记录 Git SHA 而漏掉映射边界变化。
export function digestFiles (root, relativeFiles) {
  const hash = crypto.createHash('sha256')
  for (const relative of [...relativeFiles].sort()) {
    const safe = assertSafeRelativePath(relative, '源文件')
    hash.update(safe)
    hash.update('\0')
    hash.update(fs.readFileSync(path.join(root, safe)))
    hash.update('\0')
  }
  return hash.digest('hex')
}

// canonicalJson 生成映射摘要使用的稳定 JSON，避免格式空白影响版本证明。
export function canonicalJson (value) {
  return JSON.stringify(value)
}
