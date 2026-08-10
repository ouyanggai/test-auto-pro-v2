#!/usr/bin/env node
'use strict'

const childProcess = require('child_process')
const crypto = require('crypto')
const fs = require('fs')
const os = require('os')
const path = require('path')
const { assertLiveTargetClean, digestTarget, getSyncConfig } = require('./sync-config')

// ensureDir 创建同步目标父目录。
function ensureDir (directory) {
  fs.mkdirSync(directory, { recursive: true })
}

// copyFile 复制已跟踪源码并保留权限。
function copyFile (source, target) {
  ensureDir(path.dirname(target))
  fs.copyFileSync(source, target)
  fs.chmodSync(target, fs.statSync(source).mode)
}

// trackedFiles 只读取 Git 已跟踪文件，避免把 .npmrc、凭证或本机缓存带进运行服务。
function trackedFiles (sourceRoot) {
  const output = childProcess.execFileSync('git', ['-C', sourceRoot, 'ls-files', '-z'])
  return output.toString('utf8').split('\0').filter(Boolean).map(value => value.split(path.sep).join('/'))
}

// pathWithin 判断 tracked 文件是否属于清单映射。
function pathWithin (candidate, root) {
  return candidate === root || candidate.startsWith(`${root}/`)
}

// preservePaths 在精确镜像前备份 FormMaking 运行依赖，避免外层仓库的 dist 忽略规则删除真实引擎。
function preservePaths (targetRoot, manifest) {
  const temporaryRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'f007-runtime-preserve-'))
  const preserved = []
  for (const relative of manifest.preservedTargetPaths || []) {
    const normalized = relative.replace(/^runtime-source\//, '')
    const source = path.join(targetRoot, normalized)
    if (!fs.existsSync(source)) continue
    const backup = path.join(temporaryRoot, normalized)
    ensureDir(path.dirname(backup))
    fs.cpSync(source, backup, { recursive: true })
    preserved.push({ normalized, backup })
  }
  return { temporaryRoot, preserved }
}

// restorePaths 恢复清单声明的本地运行依赖。
function restorePaths (targetRoot, backup) {
  for (const item of backup.preserved) {
    const target = path.join(targetRoot, item.normalized)
    fs.rmSync(target, { recursive: true, force: true })
    ensureDir(path.dirname(target))
    fs.cpSync(item.backup, target, { recursive: true })
  }
  fs.rmSync(backup.temporaryRoot, { recursive: true, force: true })
}

// digestMappings 计算真实候选构建输入摘要，健康检查以此确认服务已经消费本次源码。
function digestMappings (sourceRoot, mappings, files) {
  const hash = crypto.createHash('sha256')
  const selected = files.filter(file => mappings.some(mapping => pathWithin(file, mapping.source))).sort()
  for (const relative of selected) {
    hash.update(relative)
    hash.update('\0')
    hash.update(fs.readFileSync(path.join(sourceRoot, relative)))
  }
  return hash.digest('hex')
}

// sync 把固定参考仓库完整可运行资产镜像到实际候选源码，不触碰本地 iframe 适配层。
function sync () {
  const config = getSyncConfig()
  assertLiveTargetClean(config.targetRoot)
  const files = trackedFiles(config.sourceRoot)
  const backup = preservePaths(config.targetRoot, config.manifest)
  try {
    for (const mapping of config.manifest.mappings) {
      const sourceRelative = mapping.source.split(path.sep).join('/')
      const targetRelative = mapping.target.replace(/^runtime-source\//, '')
      const target = path.join(config.targetRoot, targetRelative)
      fs.rmSync(target, { recursive: true, force: true })
      if (mapping.type === 'file') {
        if (!files.includes(sourceRelative)) throw new Error(`同步来源缺少已跟踪文件: ${sourceRelative}`)
        copyFile(path.join(config.sourceRoot, sourceRelative), target)
      } else {
        const children = files.filter(file => pathWithin(file, sourceRelative))
        if (!children.length) throw new Error(`同步来源缺少已跟踪目录: ${sourceRelative}`)
        for (const file of children) {
          copyFile(path.join(config.sourceRoot, file), path.join(target, file.slice(sourceRelative.length + 1)))
        }
      }
      process.stdout.write(`[SYNC] ${mapping.source} -> ${mapping.target}\n`)
    }
  } finally {
    restorePaths(config.targetRoot, backup)
  }
  const metadata = {
    repository: config.manifest.repository,
    remote: config.remote,
    branch: config.branch,
    head: config.head,
    digest: digestMappings(config.sourceRoot, config.manifest.mappings, files),
    targetDigest: digestTarget(config.targetRoot)
  }
  fs.writeFileSync(path.join(config.targetRoot, '.f007-source.json'), `${JSON.stringify(metadata, null, 2)}\n`)
  process.stdout.write(`[SYNC] 来源快照 ${metadata.head} 已写入实际构建源码\n`)
}

sync()
