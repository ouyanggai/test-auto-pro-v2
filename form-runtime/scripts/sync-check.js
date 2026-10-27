#!/usr/bin/env node
'use strict'

const childProcess = require('child_process')
const crypto = require('crypto')
const fs = require('fs')
const path = require('path')
const { digestTarget, getSyncConfig } = require('./sync-config')

// sha256 比较映射文件内容。
function sha256 (file) {
  return crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex')
}

// trackedFiles 返回固定来源的 Git 跟踪路径。
function trackedFiles (sourceRoot) {
  return childProcess.execFileSync('git', ['-C', sourceRoot, 'ls-files', '-z']).toString('utf8').split('\0').filter(Boolean)
}

// pathWithin 判断路径是否位于目录映射。
function pathWithin (candidate, root) {
  return candidate === root || candidate.startsWith(`${root}/`)
}

// isPreservedTarget 判断映射目标是否属于项目明确保留的本地运行依赖。
function isPreservedTarget (target, preservedTargetPaths) {
  const normalizedTarget = target.replace(/\\/g, '/')
  return preservedTargetPaths.some(value => {
    const preserved = value.replace(/\\/g, '/')
    return pathWithin(normalizedTarget, preserved)
  })
}

// mappedTargetPath 计算映射文件在 runtime-source 中的相对路径。
function mappedTargetPath (mapping, suffix) {
  const target = mapping.target.replace(/\\/g, '/')
  return mapping.type === 'file' ? target : path.posix.join(target, suffix)
}

// syncCheck 确认候选构建实际消费的 runtime-source 与任务快照一致。
function syncCheck () {
  const config = getSyncConfig()
  const files = trackedFiles(config.sourceRoot)
  const preservedTargetPaths = config.manifest.preservedTargetPaths || []
  for (const preserved of preservedTargetPaths) {
    const normalized = preserved.replace(/^runtime-source\//, '').replace(/\\/g, '/')
    if (!fs.existsSync(path.join(config.targetRoot, normalized))) {
      throw new Error(`本地保留的运行依赖缺失: ${preserved}`)
    }
  }
  for (const mapping of config.manifest.mappings) {
    const sourceRelative = mapping.source.split(path.sep).join('/')
    const targetRelative = mapping.target.replace(/^runtime-source\//, '')
    const selected = mapping.type === 'file' ? [sourceRelative] : files.filter(file => pathWithin(file, sourceRelative))
    for (const file of selected) {
      const suffix = mapping.type === 'file' ? '' : file.slice(sourceRelative.length + 1)
      const source = path.join(config.sourceRoot, file)
      const target = path.join(config.targetRoot, targetRelative, suffix)
      // 保留目录来自本项目受控产物，不与上游同名文件比较；完整目标摘要仍会校验其实际内容。
      if (isPreservedTarget(mappedTargetPath(mapping, suffix), preservedTargetPaths)) continue
      if (!fs.existsSync(target) || sha256(source) !== sha256(target)) {
        throw new Error(`同步校验不一致: ${file} -> ${path.join(mapping.target, suffix)}`)
      }
    }
    process.stdout.write(`[SYNC_CHECK] ${mapping.target} 已与任务来源一致\n`)
  }
  const metadataPath = path.join(config.targetRoot, '.f007-source.json')
  const metadata = JSON.parse(fs.readFileSync(metadataPath, 'utf8'))
  if (metadata.head !== config.head || metadata.branch !== config.branch || metadata.remote !== config.remote) {
    throw new Error('候选来源元数据与任务快照不一致')
  }
  if (!metadata.targetDigest || metadata.targetDigest !== digestTarget(config.targetRoot)) {
    throw new Error('实际候选源码在同步后被修改')
  }
  const sourceMain = fs.readFileSync(path.join(config.sourceRoot, 'src/main.js'), 'utf8')
  const targetMain = fs.readFileSync(path.join(config.targetRoot, 'src/main.js'), 'utf8')
  const registrations = [...sourceMain.matchAll(/name:\s*['"]([^'"]+)['"]\s*,\s*component:/g)].map(match => match[1])
  if (!registrations.length || registrations.some(name => !targetMain.includes(`name: '${name}'`))) {
    throw new Error('真实 FormMaking 自定义组件注册未完整进入构建源码')
  }
  process.stdout.write(`[SYNC_CHECK] ${registrations.length} 个目标自定义组件注册已进入真实入口\n`)
}

if (require.main === module) syncCheck()

module.exports = { isPreservedTarget, mappedTargetPath, syncCheck }
