#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
temporary_root="$(mktemp -d)"
trap 'rm -rf "${temporary_root}"' EXIT

source_file="${temporary_root}/v1-config.yaml"
output_file="${temporary_root}/local.env"
fixture_password="fixture-plan-password-$$"

{
  printf 'runnerDb:\n'
  printf '  host: db.example.invalid\n'
  printf '  port: 3307\n'
  printf '  user: runner-user\n'
  printf '  password: %s\n' "${fixture_password}"
  printf '  name: v1_runner_database\n'
} >"${source_file}"
printf 'TARGET_API_GATEWAY=https://target.example.invalid/gateway\nTARGET_LOGIN_PASSWORD=preserved-target-secret\n' >"${output_file}"
chmod 600 "${output_file}"

cd "${project_root}"
go run ./cmd/sync-v1-plan-db-config -source "${source_file}" -output "${output_file}" >/dev/null

test "$(stat -f '%Lp' "${output_file}")" = "600"
for name in PLAN_DB_HOST PLAN_DB_PORT PLAN_DB_USER PLAN_DB_PASSWORD PLAN_DB_NAME; do
  grep -q "^${name}=" "${output_file}"
done
grep -q '^PLAN_DB_NAME=test_auto_pro_v2$' "${output_file}"
grep -q '^TARGET_API_GATEWAY=' "${output_file}"
grep -q '^TARGET_LOGIN_PASSWORD=' "${output_file}"

if go run ./cmd/sync-v1-plan-db-config -source "${source_file}" -output "${output_file}" -database v1_runner_database >/dev/null 2>&1; then
  printf '同步工具错误地允许复用 V1 runnerDb 原数据库\n' >&2
  exit 1
fi

printf 'F-003 本机计划数据库配置同步与私有权限验证通过\n'
