import assert from 'node:assert/strict'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { performance } from 'node:perf_hooks'

import { ExecutionBaselineRecorder, type BaselineRecord } from '../../../executor-service/src/execution-baseline.ts'

const samplesPerMode = 100
const operationsPerSample = 200
const warmupOperations = 2000

// percentile 只对同一模式的真实样本排序取位，不将模拟CPU时间称为目标平台提速。
function percentile(values: number[], fraction: number): number {
  const ordered = [...values].sort((left, right) => left - right)
  return ordered[Math.max(0, Math.ceil(ordered.length * fraction) - 1)]!
}

// makeMode 构造同一业务调用和内存编码sink：启用时模拟JSON编码成本，避免日志洪水。
function makeMode(enabled: boolean) {
  const counts = { calls: 0, requestCalls: 0, formCalls: 0, encodedRecords: 0, encodedCharacters: 0 }
  const returned = { data: { instanceId: 'local-i1', person: 'local-person', person__virtualName: '本地人员', amount: '9007199254740993' } }
  const data = { component: 'FlowDialog', method: 'sumbitFlow', requestBytes: 256, source: 'local-cpu-fixture' }
  const sink = (record: BaselineRecord) => {
    counts.encodedRecords++
    counts.encodedCharacters += JSON.stringify(record).length
  }
  const recorder = new ExecutionBaselineRecorder({ runId: 'local-run', pathRunId: 'local-path', sid: 'fake-sid', platformCode: 'local-platform', projectId: 'local-project' }, { enabled, sink })
  const originalCall = () => {
    counts.calls++
    // 这些仅是内存出口计数器，没有网络、页面或目标平台副作用。
    counts.requestCalls++
    counts.formCalls++
    return returned
  }
  return { enabled, counts, recorder, originalCall, returned, data, samples: [] as number[], memorySamples: [] as ReturnType<typeof process.memoryUsage>[] }
}

// runBatch 执行实际invoke和相同假件，业务返回对象必须保持，墙钟只用于测量。
function runBatch(mode: ReturnType<typeof makeMode>, operations: number): number {
  let result: unknown
  const startedAt = performance.now()
  for (let index = 0; index < operations; index++) result = mode.recorder.invoke('native_call', mode.originalCall, mode.data)
  const elapsedMs = performance.now() - startedAt
  assert.strictEqual(result, mode.returned)
  return elapsedMs
}

const off = makeMode(false)
const on = makeMode(true)
for (let index = 0; index < warmupOperations / operationsPerSample; index++) {
  runBatch(off, operationsPerSample)
  runBatch(on, operationsPerSample)
}
const memoryBefore = process.memoryUsage()
for (let index = 0; index < samplesPerMode; index++) {
  // 交替先后顺序，减少预热、GC和后台负载总是落在同一模式上的偏差。
  for (const mode of index % 2 === 0 ? [off, on] : [on, off]) {
    mode.samples.push(runBatch(mode, operationsPerSample))
    mode.memorySamples.push(process.memoryUsage())
  }
}
const expectedCalls = warmupOperations + samplesPerMode * operationsPerSample
for (const mode of [off, on]) {
  assert.equal(mode.counts.calls, expectedCalls)
  assert.equal(mode.counts.requestCalls, expectedCalls)
  assert.equal(mode.counts.formCalls, expectedCalls)
  assert.equal(mode.recorder.stats().activeSpans, 0)
}
assert.equal(off.counts.encodedRecords, 0)
assert.ok(on.counts.encodedRecords > 0)

// summarizeMode 报告批次区间与内存采样，不把GC后的堆差额冒充累计分配。
function summarizeMode(mode: ReturnType<typeof makeMode>) {
  return {
    enabled: mode.enabled,
    samples: mode.samples.length,
    operationsPerSample,
    batchMs: { p50: percentile(mode.samples, 0.5), p95: percentile(mode.samples, 0.95), max: Math.max(...mode.samples) },
    counts: mode.counts,
    recorder: mode.recorder.stats(),
    sampledMemory: {
      heapUsedMin: Math.min(...mode.memorySamples.map(sample => sample.heapUsed)),
      heapUsedMax: Math.max(...mode.memorySamples.map(sample => sample.heapUsed)),
      rssMax: Math.max(...mode.memorySamples.map(sample => sample.rss))
    }
  }
}

// benchmarkFileSink 在独立临时文件中对照模拟正文的JSONL编码和同步写出，不使用生产stdout。
function benchmarkFileSink() {
  const ioSamplesPerMode = 100
  const ioOperationsPerSample = 2
  const ioWarmupOperations = 20
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'f045-baseline-sink-'))
  const ownFiles: string[] = []
  const openDescriptors = new Set<number>()
  const measurements: unknown[] = []
  try {
    for (const fixtureBodyBytes of [256, 64 * 1024]) {
      const body = 'x'.repeat(fixtureBodyBytes)
      const response = { httpStatus: 200, body, bodyEncoding: 'utf8' }
      const modes = [false, true].map(enabled => {
        const file = path.join(directory, `${fixtureBodyBytes}-${enabled ? 'enabled' : 'disabled'}.jsonl`)
        ownFiles.push(file)
        const fd = fs.openSync(file, 'wx')
        openDescriptors.add(fd)
        const counts = { calls: 0, encodedRecords: 0, writtenBytes: 0 }
        const recorder = new ExecutionBaselineRecorder({ runId: 'local-run', pathRunId: 'local-path', sid: 'fake-sid', projectId: 'local-project' }, {
          enabled,
          sink: record => {
            const encoded = Buffer.from(JSON.stringify(record) + '\n', 'utf8')
            counts.encodedRecords++
            // writeSync可能部分写出；循环仅写本条模拟记录，不新增任何业务调用。
            let offset = 0
            while (offset < encoded.length) {
              const written = fs.writeSync(fd, encoded, offset, encoded.length - offset)
              if (written <= 0) throw new Error('本地基准文件未能继续写出')
              offset += written
              counts.writtenBytes += written
            }
          }
        })
        const originalCall = () => { counts.calls++; return response }
        return { enabled, file, fd, counts, recorder, originalCall, samples: [] as number[], memory: [] as ReturnType<typeof process.memoryUsage>[] }
      })
      try {
        // runIOBatch 保持开关记录下调用同一假件，只改变本地日志sink工作量。
        const runIOBatch = (mode: typeof modes[number], operations: number) => {
          const startedAt = performance.now()
          for (let index = 0; index < operations; index++) {
            const span = mode.recorder.begin('target_response', { request: { body, fixtureBodyBytes } })
            const originalResult = mode.recorder.invoke('forward_invocation', mode.originalCall)
            assert.strictEqual(originalResult, response)
            mode.recorder.end(span, 'returned', { response: originalResult, fixtureBodyBytes, boundary: 'full_response_available' })
          }
          return performance.now() - startedAt
        }
        for (const mode of modes) runIOBatch(mode, ioWarmupOperations)
        const memoryBefore = process.memoryUsage()
        for (let sample = 0; sample < ioSamplesPerMode; sample++) {
          for (const mode of sample % 2 === 0 ? modes : [...modes].reverse()) {
            mode.samples.push(runIOBatch(mode, ioOperationsPerSample))
            mode.memory.push(process.memoryUsage())
          }
        }
        const expectedCalls = ioWarmupOperations + ioSamplesPerMode * ioOperationsPerSample
        for (const mode of modes) {
          assert.equal(mode.counts.calls, expectedCalls)
          assert.equal(mode.recorder.stats().activeSpans, 0)
          assert.equal(mode.recorder.stats().sinkFailures, 0)
          assert.equal(fs.fstatSync(mode.fd).size, mode.counts.writtenBytes)
          if (!mode.enabled) assert.equal(mode.counts.writtenBytes, 0)
        }
        measurements.push({
          fixtureBodyBytes,
          warmupOperationsPerMode: ioWarmupOperations,
          samplesPerMode: ioSamplesPerMode,
          operationsPerSample: ioOperationsPerSample,
          modes: modes.map(mode => ({
            enabled: mode.enabled,
            batchMs: { p50: percentile(mode.samples, 0.5), p95: percentile(mode.samples, 0.95), max: Math.max(...mode.samples) },
            counts: mode.counts,
            recorder: mode.recorder.stats(),
            sampledMemory: {
              heapUsedMin: Math.min(...mode.memory.map(sample => sample.heapUsed)),
              heapUsedMax: Math.max(...mode.memory.map(sample => sample.heapUsed)),
              rssMax: Math.max(...mode.memory.map(sample => sample.rss))
            }
          })),
          processMemoryBefore: memoryBefore,
          processMemoryAfter: process.memoryUsage()
        })
      } finally {
        for (const mode of modes) {
          fs.closeSync(mode.fd)
          openDescriptors.delete(mode.fd)
        }
      }
    }
  } finally {
    // 只逐个删本次创建的文件，再删自己的空目录，禁止递归清理或扩大到临时目录根。
    for (const fd of openDescriptors) fs.closeSync(fd)
    const resolvedDirectory = path.resolve(directory)
    for (const file of ownFiles) {
      if (path.dirname(path.resolve(file)) !== resolvedDirectory) throw new Error('基准清理路径越出本次目录')
      if (fs.existsSync(file)) fs.unlinkSync(file)
    }
    fs.rmdirSync(resolvedDirectory)
  }
  return measurements
}

const fileSinkMeasurements = benchmarkFileSink()

process.stdout.write(JSON.stringify({
  type: 'execution_baseline_local_benchmark',
  version: 1,
  scope: '本地CPU内存假件；不是目标平台性能、真实页面或业务提速证明',
  warmupOperationsPerMode: warmupOperations,
  modes: [summarizeMode(off), summarizeMode(on)],
  localFileSink: {
    scope: '模拟正文→JSON.stringify→writeSync本地临时JSONL文件；非HTTP/页面/生产stdout实验',
    measurements: fileSinkMeasurements
  },
  processMemoryBefore: memoryBefore,
  processMemoryAfter: process.memoryUsage(),
  limits: [
    'modes的CPU对照只计JSON.stringify日志编码，没有stdout逐条输出或磁盘IO',
    'localFileSink单独包含同步文件写出，不含fsync耐久落盘，也未测生产console/stdout重定向成本',
    'memory为同一进程交替执行时的采样，不是各模式独占内存或累计分配',
    '未手动强制GC；heap差额和尾延迟会受GC及主机负载影响',
    '仅断言业务调用次数/返回与资源收口，不断言不稳定的墙钟提速阈值'
  ]
}) + '\n')
