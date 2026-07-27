#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
temporary_root="$(mktemp -d)"
trap 'rm -rf "${temporary_root}"' EXIT

source_file="${temporary_root}/v1-config.yaml"
output_file="${temporary_root}/target.env"
fixture_key="$(printf 'f%015d' "$$")"
fixture_password="fixture-password-$$"
fixture_code="fixture-code-$$"

{
  printf 'target:\n'
  printf '  apiGateway: https://target.example.invalid/gateway\n'
  printf '  platformCode: fixture-platform\n'
  printf '  customerCode: fixture-customer\n'
  printf '  loginAesKey: %s\n' "${fixture_key}"
} >"${source_file}"

cd "${project_root}"
TARGET_LOGIN_PASSWORD="${fixture_password}" \
TARGET_LOGIN_CODE="${fixture_code}" \
go run ./cmd/sync-v1-target-config -source "${source_file}" -output "${output_file}" >/dev/null

test -f "${output_file}"
test "$(stat -f '%Lp' "${output_file}")" = "600"
for name in TARGET_API_GATEWAY TARGET_LOGIN_PASSWORD TARGET_LOGIN_AES_KEY TARGET_LOGIN_CODE TARGET_PLATFORM_CODE TARGET_CUSTOMER_CODE; do
  grep -q "^${name}=" "${output_file}"
done

printf 'F-002 本机配置同步与私有权限验证通过\n'
