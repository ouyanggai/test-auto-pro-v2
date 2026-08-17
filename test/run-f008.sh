#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

"${project_root}/test/integration/f008_frontend_structure.sh"
(cd "${project_root}" && go test ./test/contracts ./test/unit/backend)
node --no-warnings --experimental-strip-types --test "${project_root}/test/unit/frontend/path_configuration_test.mjs"
(cd "${project_root}/web" && pnpm exec vue-tsc --noEmit && pnpm build)

echo 'F-008 定向验证通过'
