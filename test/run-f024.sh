#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${project_root}"

# 集成用例通过 config.LoadPlanDBConfig 读取本机忽略配置，go test 的工作目录是包目录，
# 相对路径读不到项目根的 .env.local，这里显式指向绝对路径避免真实数据库用例被静默跳过。
plan_db_env_file="${TEST_AUTO_PRO_PLAN_DB_ENV_FILE:-${project_root}/.env.local}"
if [[ ! -f "${plan_db_env_file}" ]]; then
  printf '%s\n' "[F-024] 缺少计划数据库本机配置：${plan_db_env_file}" >&2
  exit 1
fi
export TEST_AUTO_PRO_PLAN_DB_ENV_FILE="${plan_db_env_file}"
export TEST_AUTO_PRO_TARGET_ENV_FILE="${TEST_AUTO_PRO_TARGET_ENV_FILE:-${project_root}/.env.local}"

printf '%s\n' '[F-024] 编译与静态检查'
go build ./...
go vet ./internal/... ./test/unit/backend/executor ./test/unit/backend/history_replay
test -z "$(gofmt -l internal cmd test)"

printf '%s\n' '[F-024] 节点权限与写载荷构造用例（含竞态检测）'
go test -race -count=1 -run 'TestF024' ./test/unit/backend/executor ./test/unit/backend/history_replay

printf '%s\n' '[F-024] 必需用例逐条核对'
unit_log="$(mktemp -t f024-unit)"
trap 'rm -f "${unit_log}"' EXIT
if ! go test -count=1 -v -run 'TestF024' ./test/unit/backend/executor ./test/unit/backend/history_replay 2>&1 | tee "${unit_log}"; then
  exit 1
fi
if grep -Eq -- '^[[:space:]]*--- SKIP' "${unit_log}"; then
  printf '%s\n' '[F-024] 存在跳过用例，判定为失败' >&2
  exit 1
fi
for required in \
  TestF024SubmitDropsFieldsOnlyLaterNodesCanEdit \
  TestF024ApproveMergesInstanceDataAndOverlaysOnlyNodeEditable \
  TestF024CompanionKeysFollowTheirOwnControl \
  TestF024ApproveUsesRealTargetNodeID \
  TestF024MissingTargetNodeIDBlocksStep \
  TestF024KeyFieldFillHintsPointAtTheNodeThatCanFillIt \
  TestF024UnfillableDecisiveConditionFieldBlocks \
  TestF024NodeFormViewsFollowTargetDeclaration \
  TestF024FieldPowerCoversTargetConventions \
  TestF024SaveRestoresFieldsOutsideCurrentView \
  TestF024NoFieldPowerDeclarationDegradesInsteadOfBlocking \
  TestF024InitiatorWithoutDeclarationIsExplained; do
  if ! grep -Eq -- "^[[:space:]]*--- PASS: ${required}" "${unit_log}"; then
    printf '[F-024] 缺少必需用例的通过记录：%s\n' "${required}" >&2
    exit 1
  fi
done

printf '%s\n' '[F-024] 写载荷不再直接透传历史快照'
if grep -n 'FormData:.*runCtx.EffectiveFormData' internal/engine/step/gate.go; then
  printf '%s\n' '[F-024] 写请求不得直接透传整份历史表单数据，必须经 BuildNodeFormData 按节点权限构造' >&2
  exit 1
fi
grep -qF 'BuildNodeFormData' internal/engine/step/executor.go
printf '%s\n' '[F-024] 目标节点标识不再用工具侧不透明键'
if grep -nE 'FindDueTaskID\(ctx, session, [^,]+, step\.NodeKey\)|NodeProxyID = step\.NodeKey' internal/engine/step/*.go; then
  printf '%s\n' '[F-024] 发给目标的节点标识必须是真实标识，不能用编译场景的不透明键' >&2
  exit 1
fi

printf '%s\n' '[F-024] 权限判据保持通用：不得按具体表单或字段名写死'
if grep -rnE '"(classificationId|contractSum|closeResult|accountantOpinion|paymentId)"|合同盖章|应诉案件' \
  internal/formdata/fieldpower internal/engine/step/formdata.go internal/service/path_node_field_power.go; then
  printf '%s\n' '[F-024] 节点权限与写载荷构造只允许依据目标平台级约定，不得出现具体表单或字段名' >&2
  exit 1
fi

printf '%s\n' '[F-024] 表单装载命令不设超时'
# 装载要等目标数据源与选项协调（分钟级），一旦沿用 15 秒默认超时就会两次超时后把会话置为未就绪，
# 出现"数据已正确显示却提示响应超时、保存说运行时未就绪"。这里反向锁定必须显式传 0。
python3 - <<'PYCHECK'
import io, re, sys
text = io.open('web/src/features/path-configuration/FormRuntimeFrame.vue', encoding='utf-8').read()
start = text.find("postCommand('load'")
if start < 0:
    print('[F-024] 找不到装载命令调用', file=sys.stderr); raise SystemExit(1)
segment = text[start:start + 2000]
end = segment.find('\n    if (disposed')
if end < 0:
    print('[F-024] 装载命令调用形状变了，需重新核对超时参数', file=sys.stderr); raise SystemExit(1)
if 'undefined, 0)' not in segment[:end]:
    print('[F-024] 装载命令必须显式不设超时（undefined, 0）', file=sys.stderr); raise SystemExit(1)
PYCHECK

printf '%s\n' '[F-024] 语义清单证据未漂移'
./test/contracts/f014/semantics_evidence_drift.sh >/dev/null

printf '%s\n' '[F-024] F-012 与执行器回归'
go test -count=1 ./test/unit/... ./test/integration/f012

printf '%s\n' '[F-024] 前端类型检查与构建'
pnpm --dir web typecheck
pnpm --dir web build
node --no-warnings --test test/unit/frontend/form_runtime_test.mjs

git diff --check
printf '%s\n' 'F-024 定向验证完成'
