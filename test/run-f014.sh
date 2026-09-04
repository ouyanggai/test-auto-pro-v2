#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${project_root}"

# 只读集成用例要连真实目标，配置与测试账号都只放本机被 Git 忽略的 .env.local。
# 这里显式导出，避免 go test 的工作目录读不到项目根的配置而让用例静默降级。
export TEST_AUTO_PRO_TARGET_ENV_FILE="${TEST_AUTO_PRO_TARGET_ENV_FILE:-${project_root}/.env.local}"
export TEST_AUTO_PRO_PLAN_DB_ENV_FILE="${TEST_AUTO_PRO_PLAN_DB_ENV_FILE:-${project_root}/.env.local}"
if [ -f "${project_root}/.env.local" ]; then
  set -a
  # shellcheck disable=SC1091
  . "${project_root}/.env.local"
  set +a
fi

printf '%s\n' '[F-014] 编译与静态检查'
go build ./...
go vet ./internal/engine/verdict/... ./test/unit/backend/target_semantics/... ./test/integration

printf '%s\n' '[F-014] 三值判定单元测试（含竞态检测）'
go test -race -count=1 ./test/unit/backend/target_semantics/...

printf '%s\n' '[F-014] 真实目标只读集成测试'
integration_log="$(mktemp -t f014-integration)"
trap 'rm -f "${integration_log}"' EXIT
if ! go test -count=1 -v -run 'TestF014Readonly' ./test/integration 2>&1 | tee "${integration_log}"; then
  exit 1
fi
if grep -Eq -- '^[[:space:]]*--- SKIP' "${integration_log}"; then
  printf '%s\n' '[F-014] 只读集成测试存在跳过用例，判定为失败' >&2
  exit 1
fi
for required in TestF014ReadonlySuccessAndFailureShapes TestF014ReadonlyTimeoutClassifiesAsUncertain TestF014ReadonlyContractRegression; do
  if ! grep -Eq -- "^[[:space:]]*--- PASS: ${required}" "${integration_log}"; then
    printf '%s\n' "[F-014] 缺少必需的只读集成用例通过记录：${required}" >&2
    exit 1
  fi
done

printf '%s\n' '[F-014] 写端点白名单为空'
./test/contracts/f014/target_write_whitelist.sh

printf '%s\n' '[F-014] 证据漂移检测'
./test/contracts/f014/semantics_evidence_drift.sh

printf '%s\n' '[F-014] 探针清单完整性与样本来源标注'
python3 - <<'PY'
import io, json, os, re, sys

semantics = io.open('docs/TARGET_SEMANTICS.md', encoding='utf-8').read()
failures = []

# 每条「源码推断、待 F-016 实测」结论都必须有编号，且编号必须在探针清单里再出现一次。
pending = re.findall(r'^\|\s*(P\d+)\s*\|[^\n]*源码推断、待 F-016 实测[^\n]*\|', semantics, re.M)
if not pending:
    failures.append('语义清单里找不到任何带编号的待实测结论')
for probe in pending:
    rows = re.findall(r'^\|\s*%s\s*\|' % probe, semantics, re.M)
    if len(rows) < 2:
        failures.append('待实测结论 %s 没有对应的探针条目' % probe)

# 探针条目必须写清前置条件、最小步骤、期望观察点与由 F-016 哪一步覆盖，即整行至少五列。
for probe in set(pending):
    for row in re.findall(r'^\|\s*%s\s*\|([^\n]*)$' % probe, semantics, re.M):
        columns = [column.strip() for column in row.split('|') if column.strip()]
        if len(columns) >= 4 and all(columns):
            break
    else:
        failures.append('探针 %s 缺少前置条件、最小步骤、期望观察点或覆盖步骤' % probe)

# 每个按源码证据构造的样本都必须带来源位置与待 F-016 复核标注。
source_dir = os.path.join('test', 'fixtures', 'f014', 'from-source')
names = [name for name in sorted(os.listdir(source_dir)) if name.endswith('.json')]
if not names:
    failures.append('源码构造样本目录为空')
for name in names:
    sample = json.load(io.open(os.path.join(source_dir, name), encoding='utf-8'))
    if not sample.get('source', '').strip():
        failures.append('%s 缺少 source 来源标注' % name)
    if 'F-016' not in sample.get('note', ''):
        failures.append('%s 的 note 缺少待 F-016 复核或替换标注' % name)

for failure in failures:
    print('[F-014] %s' % failure, file=sys.stderr)
if failures:
    raise SystemExit(1)
print('[F-014] 待实测结论 %d 条均有探针，源码构造样本 %d 个均带来源标注' % (len(pending), len(names)))
PY

printf '%s\n' '[F-014] 日志目录未进入版本库'
if ! git check-ignore -q logs/app.log; then
  printf '%s\n' '[F-014] logs/ 必须被 .gitignore 忽略' >&2
  exit 1
fi
git status --porcelain logs 2>/dev/null | grep -q . && {
  printf '%s\n' '[F-014] logs/ 出现在待提交列表' >&2
  exit 1
}

git diff --check
printf '%s\n' 'F-014 定向验证完成'
