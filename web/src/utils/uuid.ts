// createUuid 在 HTTPS 以外的局域网页面中也提供稳定的请求标识。
// randomUUID 只在安全上下文可用，因此依次退回 getRandomValues 和本地随机实现。
export function createUuid(): string {
  const secureCrypto = globalThis.crypto
  if (typeof secureCrypto?.randomUUID === 'function') return secureCrypto.randomUUID()
  if (typeof secureCrypto?.getRandomValues === 'function') {
    const bytes = new Uint8Array(16)
    secureCrypto.getRandomValues(bytes)
    bytes[6] = (bytes[6] & 0x0f) | 0x40
    bytes[8] = (bytes[8] & 0x3f) | 0x80
    const hex = [...bytes].map(byte => byte.toString(16).padStart(2, '0')).join('')
    return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
  }
  const time = Date.now().toString(16)
  const random = Math.random().toString(16).slice(2).padEnd(24, '0')
  return `${time.slice(-8)}-${random.slice(0, 4)}-4${random.slice(4, 7)}-8${random.slice(7, 10)}-${random.slice(10, 22)}`
}
