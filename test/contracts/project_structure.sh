#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

required_files=(
  AGENTS.md
  CONTEXT.md
  docs/PRODUCT.md
  docs/ARCHITECTURE.md
  docs/ROADMAP.md
  docs/PROGRESS.md
  docs/REFERENCE_CODE.md
  docs/TEST_STRATEGY.md
  docs/features/TEMPLATE.md
  docs/features/F-000-project-bootstrap.md
)

for required_file in "${required_files[@]}"; do
  [[ -f "${project_root}/${required_file}" ]] || {
    printf '缺少项目文件：%s\n' "${required_file}" >&2
    exit 1
  }
done

for skill_name in plan-feature-slice implement-feature-slice diagnose-one-issue review-feature-slice; do
  skill_dir="${project_root}/.agents/skills/${skill_name}"
  [[ -f "${skill_dir}/SKILL.md" && -f "${skill_dir}/agents/openai.yaml" ]] || {
    printf '本地技能不完整：%s\n' "${skill_name}" >&2
    exit 1
  }
  ! grep -Fq '[TODO' "${skill_dir}/SKILL.md"
done

grep -Fq 'ready_for_manual' "${project_root}/AGENTS.md"
grep -Fq '只有用户明确验收' "${project_root}/AGENTS.md"
grep -Fq '纯 Vue 3、Vite、Vue Router、Pinia、Naive UI' "${project_root}/docs/ARCHITECTURE.md"
grep -Fq 'Vue Flow 与 dagre 统一延后到 F-004' "${project_root}/docs/ARCHITECTURE.md"
