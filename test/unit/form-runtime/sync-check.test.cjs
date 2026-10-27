'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')
const { isPreservedTarget, mappedTargetPath } = require('../../../form-runtime/scripts/sync-check.js')

test('项目保留的 FormMaking 产物不要求与上游同名文件一致', () => {
  const manifest = require('../../../form-runtime/sync-manifest.json')
  const mapping = manifest.mappings.find(item => item.source === 'src')
  const mappedPath = mappedTargetPath(mapping, 'lib/vue-form-making/dist/FormMaking.common.js')

  assert.equal(mappedPath, 'runtime-source/src/lib/vue-form-making/dist/FormMaking.common.js')
  assert.equal(isPreservedTarget(mappedPath, manifest.preservedTargetPaths), true)
})

test('保留目录之外的 FormMaking 源码仍逐文件比对上游', () => {
  const manifest = require('../../../form-runtime/sync-manifest.json')
  const mapping = manifest.mappings.find(item => item.source === 'src')
  const mappedPath = mappedTargetPath(mapping, 'lib/vue-form-making/src/index.js')

  assert.equal(isPreservedTarget(mappedPath, manifest.preservedTargetPaths), false)
})

test('相邻目录不会误匹配受控产物保留边界', () => {
  const preserved = ['runtime-source/src/lib/vue-form-making/dist']

  assert.equal(isPreservedTarget('runtime-source/src/lib/vue-form-making/dist-old/file.js', preserved), false)
})
