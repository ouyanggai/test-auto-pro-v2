#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
test_root="$(mktemp -d "${project_root}/.runtime/refs-dirty-test.XXXXXX")"
seed_repo="${test_root}/seed"
bare_remote="${test_root}/remote.git"
refs_root="${test_root}/refs"
manifest="${test_root}/manifest.tsv"
reference_doc="${test_root}/REFERENCE_CODE.md"
error_log="${test_root}/sync-error.log"

git init -q -b master "${seed_repo}"
printf '初始内容\n' > "${seed_repo}/README.md"
git -C "${seed_repo}" add README.md
git -C "${seed_repo}" -c user.name='测试' -c user.email='test@example.invalid' commit -q -m '初始化'
git clone -q --bare "${seed_repo}" "${bare_remote}"

for index in {1..13}; do
  printf 'repo-%02d\trepo\t%s\tmaster\n' "${index}" "${bare_remote}" >> "${manifest}"
done

REFS_ROOT="${refs_root}" REFS_MANIFEST="${manifest}" REFS_DOC="${reference_doc}" \
  "${project_root}/scripts/reference-repos.sh" sync >/dev/null

printf '未提交修改\n' >> "${refs_root}/repo/README.md"
if REFS_ROOT="${refs_root}" REFS_MANIFEST="${manifest}" REFS_DOC="${reference_doc}" \
  "${project_root}/scripts/reference-repos.sh" sync >"${error_log}" 2>&1; then
  printf '脏仓库同步应当失败\n' >&2
  exit 1
fi

grep -Fq '仓库存在未提交修改，拒绝同步' "${error_log}"
grep -Fq '未提交修改' "${refs_root}/repo/README.md"
