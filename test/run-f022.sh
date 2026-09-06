#!/usr/bin/env bash

# F-022 目标语义一致性套件汇总入口：
# 静态证据 → 漂移检测 → 语义单元测试 → 只读目标对照 → 契约与白名单 →（显式）受控写。
# 受控写套件默认不执行：只有显式设置 F022_CONTROLLED_WRITE=1 且本机配置提供
# 测试账号与流程时才允许，任何写请求都受 11 个端点白名单约束。

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${project_root}"

export TEST_AUTO_PRO_PLAN_DB_ENV_FILE="${TEST_AUTO_PRO_PLAN_DB_ENV_FILE:-${project_root}/.env.local}"
export TEST_AUTO_PRO_TARGET_ENV_FILE="${TEST_AUTO_PRO_TARGET_ENV_FILE:-${project_root}/.env.local}"
if [ -f "${project_root}/.env.local" ]; then
  set -a
  # shellcheck disable=SC1091
  . "${project_root}/.env.local"
  set +a
fi

printf '%s\n' '[F-022] 编译与静态检查'
go build ./...
go vet ./test/unit/backend/target_semantics ./test/integration
test -z "$(gofmt -l internal cmd test)"

printf '%s\n' '[F-022] 语义覆盖契约（19 条条目、34 个证据块、11 个写端点）'
./test/contracts/f022/semantic_coverage.sh

printf '%s\n' '[F-022] 证据漂移检测（参考仓库 HEAD 与符号存在性）'
./test/contracts/f014/semantics_evidence_drift.sh

printf '%s\n' '[F-022] 语义判定与幂等约束单元测试（含竞态检测）'
go test -race -count=1 ./test/unit/backend/target_semantics/...

printf '%s\n' '[F-022] 只读目标对照测试（错误语义与判定矩阵）'
readonly_log="$(mktemp -t f022-readonly)"
trap 'rm -f "${readonly_log}"' EXIT
if ! go test -count=1 -v -run 'TestF014' ./test/integration 2>&1 | tee "${readonly_log}"; then
  printf '%s\n' '[F-022] 只读对照测试失败：目标环境不可用与语义偏离必须分开排查' >&2
  exit 1
fi
if grep -Eq -- '^[[:space:]]*--- SKIP' "${readonly_log}"; then
  printf '%s\n' '[F-022] 只读对照测试存在跳过用例，判定为失败（缺配置必须明确失败）' >&2
  exit 1
fi

printf '%s\n' '[F-022] 写端点白名单契约（合并口径 = 各切片白名单的并集）'
./test/contracts/f014/target_write_whitelist.sh
./test/contracts/f016/target_write_whitelist.sh

if [ "${F022_CONTROLLED_WRITE:-0}" != "1" ]; then
  printf '%s\n' '[F-022] 受控写对照套件未执行（默认关闭）：需要 F022_CONTROLLED_WRITE=1 与指定测试账号/流程，逐动作按白名单执行并记录证据'
else
  printf '%s\n' '[F-022] 受控写对照套件：按 F-019 动作白名单逐动作执行（当前仓库尚未提供可污染流程矩阵，保持如实记录不伪造通过）'
  printf '%s\n' '[F-022] 受控写前提核对：账号来自本机忽略配置，流程与路径由人工指定；结果必须写回 docs/TARGET_SEMANTICS.md 证据块'
fi

printf '%s\n' '[F-022] 语义一致性套件汇总完成'
