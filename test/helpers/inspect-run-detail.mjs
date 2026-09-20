// 读取一次路径运行的步骤事实并打印（只读诊断）。用于夜间自动排查，不改任何状态。
// 用法：node test/helpers/inspect-run-detail.mjs <runId> [pathRunId] [--index N]
const [runId, pathRunId] = process.argv.slice(2)
if (!runId) {
  console.error('用法：node test/helpers/inspect-run-detail.mjs <runId> [pathRunId]')
  process.exit(2)
}
const url = `http://127.0.0.1:19080/api/runs/${runId}${pathRunId ? `?pathRunId=${pathRunId}` : ''}`
const envelope = await (await fetch(url)).json()
const data = envelope.data

console.log(`run=${data.runId} path=${data.pathName} status=${data.pathRunStatusName} result=${data.resultName} failure=${data.failureClassName}`)
console.log(`currentStepNo=${data.currentStepNo} instance=${data.instanceId}`)
console.log(`pathDeviation=${JSON.stringify(data.pathDeviation)}`)

const steps = data.steps || []
console.log(`steps=${steps.length}`)
console.log('step keys =', Object.keys(steps[0] || {}).join(','))

const indexArg = process.argv.indexOf('--index')
const targets = indexArg >= 0 ? [steps[Number(process.argv[indexArg + 1])]] : steps.slice(-3)
for (const step of targets) {
  if (!step) continue
  const summary = {}
  for (const [key, value] of Object.entries(step)) {
    if (Array.isArray(value)) summary[key] = `[${value.length} 项]`
    else if (value && typeof value === 'object') summary[key] = '[对象]'
    else summary[key] = value
  }
  console.log('--- step ---')
  console.log(JSON.stringify(summary, null, 1))
  for (const attempt of step.attempts || []) {
    console.log('  attempt:', JSON.stringify({
      attemptNo: attempt.attemptNo, verdictName: attempt.verdictName, reason: attempt.reason,
      basis: attempt.basis, traceId: attempt.traceId, durationMs: attempt.durationMs
    }))
  }
}
