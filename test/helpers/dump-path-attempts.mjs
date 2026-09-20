// 只读诊断：打印一次路径运行里每个步骤尝试的结论原文（不修改任何状态）。
// 用法：node test/helpers/dump-path-attempts.mjs <runId> <pathRunId>
const [runId, pathRunId] = process.argv.slice(2)
if (!runId || !pathRunId) {
  console.error('用法：node test/helpers/dump-path-attempts.mjs <runId> <pathRunId>')
  process.exit(2)
}
const envelope = await (await fetch(`http://127.0.0.1:19080/api/runs/${runId}?pathRunId=${pathRunId}`)).json()
const data = envelope.data || {}
console.log(`path=${data.pathName} status=${data.pathRunStatusName} failure=${data.failureClassName} steps=${(data.steps || []).length}`)
for (const step of data.steps || []) {
  console.log(`\n--- 第 ${step.stepNo} 步 action=${step.action} node=${step.nodeKey} actor=${step.actorName} status=${step.statusName} ---`)
  for (const attempt of step.attempts || []) {
    console.log(`  尝试 ${attempt.attemptNo}: ${attempt.verdictName}`)
    console.log(`    原因: ${attempt.reason}`)
    console.log(`    依据: ${attempt.basis}`)
  }
}
