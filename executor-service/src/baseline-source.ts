import crypto from 'node:crypto'
import fs from 'node:fs'
import path from 'node:path'

const UNKNOWN = 'unknown'

// unknownMetadata 保留每个待采集字段的未知值，不能用缺失元数据推断部署版本。
function unknownMetadata (fields: readonly string[]): Record<string, unknown> {
  return Object.fromEntries([['state', UNKNOWN], ...fields.map(field => [field, UNKNOWN])])
}

// readMetadata 只读取指定的本机 JSON；损坏、缺失和非对象内容均不影响服务启动。
function readMetadata (serviceRoot: string, relativePath: string, fields: readonly string[]): Record<string, unknown> {
  const result = unknownMetadata(fields)
  try {
    const parsed: unknown = JSON.parse(fs.readFileSync(path.join(serviceRoot, relativePath), 'utf8'))
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return result
    const metadata = parsed as Record<string, unknown>
    result.state = 'available'
    for (const field of fields) {
      // 只复制本轮允许的字符串字段；清单或构建戳可读不等于线上正在使用该版本。
      const value = metadata[field]
      if (typeof value === 'string' && value.trim() !== '') result[field] = value
    }
  } catch {
    // 采集失败只表示没有取得这份元数据，不触发同步、重建或任何业务回退。
  }
  return result
}

// readFormMakingArtifact 按当前文件的原始字节计算摘要，不猜测它对应的源码提交。
function readFormMakingArtifact (serviceRoot: string): Record<string, unknown> {
  const result: Record<string, unknown> = { state: UNKNOWN, sha256: UNKNOWN, bytes: UNKNOWN }
  try {
    const bytes = fs.readFileSync(path.join(serviceRoot, 'flow-source', 'src', 'lib', 'vue-form-making', 'dist', 'FormMaking.common.js'))
    result.state = 'available'
    result.sha256 = crypto.createHash('sha256').update(bytes).digest('hex')
    result.bytes = bytes.length
  } catch {
    // 文件不存在或无法读取时保留未知，不能用元数据替代真实产物摘要。
  }
  return result
}

// readBaselineSource 采集执行服务根目录已有的版本线索；调用方仅在启用基线记录时启动采集一次。
// 本函数不读取环境凭证、不执行命令，参考源码、构建戳与真实部署的对应关系始终保持未确认。
export function readBaselineSource (serviceRoot: string): Record<string, unknown> {
  return {
    capturedAt: new Date().toISOString(),
    deployedVersion: UNKNOWN,
    formMakingSourceHead: UNKNOWN,
    flowSourceManifest: readMetadata(serviceRoot, 'flow-source-manifest.json', ['mappingVersion']),
    flowSource: readMetadata(serviceRoot, 'flow-source.json', [
      'mappingVersion',
      'mappingHash',
      'candidateDigest',
      'baseRepository',
      'baseRemote',
      'baseBranch',
      'baseHead',
      'baseDigest',
      'businessRepository',
      'businessRemote',
      'businessBranch',
      'businessHead',
      'businessDigest',
      'preparedAt'
    ]),
    nativePageBuild: readMetadata(serviceRoot, path.join('flow-source', '.native-page-build.json'), [
      'inputDigest',
      'candidateDigest',
      'baseHead',
      'businessHead',
      'mappingHash',
      'builtAt'
    ]),
    formMakingArtifact: readFormMakingArtifact(serviceRoot)
  }
}
