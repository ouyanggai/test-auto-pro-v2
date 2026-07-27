#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

node - "${project_root}/web/package.json" <<'NODE'
const fs = require('node:fs')

const packageFile = process.argv[2]
const packageData = JSON.parse(fs.readFileSync(packageFile, 'utf8'))
const expected = ['vue', 'vue-router', 'pinia', 'naive-ui']
const forbidden = ['element-plus', 'vue-vben-admin', '@vue-flow/core', 'dagre', 'elkjs']

for (const dependency of expected) {
  if (!packageData.dependencies?.[dependency]) {
    throw new Error(`缺少前端依赖：${dependency}`)
  }
}
for (const dependency of forbidden) {
  if (packageData.dependencies?.[dependency] || packageData.devDependencies?.[dependency]) {
    throw new Error(`F-000 不应引入：${dependency}`)
  }
}
NODE

for route_name in plans runs settings; do
  grep -Fq "path: '/${route_name}'" "${project_root}/web/src/router/index.ts"
done

grep -Fq '流程自动化测试平台' "${project_root}/web/src/stores/app.ts"
