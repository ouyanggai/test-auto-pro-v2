const fs = require('fs')

// RuntimeHealthPlugin 只在编译成功后写出当前实际源码快照，HTTP 健康检查据此确认服务已经消费候选源码。
class RuntimeHealthPlugin {
  constructor (metadataPath) {
    this.metadataPath = metadataPath
  }

  apply (compiler) {
    compiler.hooks.emit.tapAsync('RuntimeHealthPlugin', (compilation, done) => {
      try {
        const source = JSON.parse(fs.readFileSync(this.metadataPath, 'utf8'))
        const payload = JSON.stringify({
          service: 'rsh-flow-components',
          sourceRepository: source.repository,
          sourceBranch: source.branch,
          sourceHead: source.head,
          sourceDigest: source.digest,
          buildHash: compilation.hash || ''
        })
        compilation.assets['runtime-health.json'] = {
          source: () => payload,
          size: () => Buffer.byteLength(payload)
        }
        done()
      } catch (error) {
        done(error)
      }
    })
  }
}

module.exports = RuntimeHealthPlugin
