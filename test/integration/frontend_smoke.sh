#!/usr/bin/env bash

set -euo pipefail

expected_health='{"status":"ok","service":"test-auto-pro","version":"dev"}'

[[ "$(curl --silent --fail http://127.0.0.1:19080/api/health)" == "${expected_health}" ]]
[[ "$(curl --silent --fail http://127.0.0.1:19000/api/health)" == "${expected_health}" ]]

for route_name in plans runs settings; do
  curl --silent --fail "http://127.0.0.1:19000/${route_name}" | grep -Fq '<title>流程自动化测试平台</title>'
done
