#!/usr/bin/env bash

set -euo pipefail

server_base="${TEST_AUTO_PRO_SERVER_BASE_URL:-http://127.0.0.1:19080}"

case "${1:-}" in
  status)
    curl --fail --silent --show-error "${server_base}/api/form-runtime-maintenance/source"
    printf '\n'
    curl --fail --silent --show-error "${server_base}/api/form-runtime-maintenance/jobs/latest" || true
    printf '\n'
    ;;
  sync)
    curl --fail --silent --show-error --request POST --header 'Content-Length: 0' "${server_base}/api/form-runtime-maintenance/jobs"
    printf '\n'
    ;;
  *)
    echo '用法：form-runtime-maintenance.sh status|sync' >&2
    exit 1
    ;;
esac
