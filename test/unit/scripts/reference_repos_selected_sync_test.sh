#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
test_root="$(mktemp -d "${project_root}/.runtime/reference-selected-test.XXXXXX")"
seed_repo="${test_root}/seed"
old_remote="${test_root}/old.git"
new_remote="${test_root}/new.git"
refs_root="${test_root}/refs"
manifest="${test_root}/manifest.tsv"
reference_doc="${test_root}/REFERENCE_CODE.md"
repo="${refs_root}/repo-01"
# Git for Windows 会把本地克隆远端规范化为 Windows 盘符路径，清单必须记录同一文本形态。
old_manifest_remote="$(cygpath -m "${old_remote}")"
new_manifest_remote="$(cygpath -m "${new_remote}")"

git init -q -b master "${seed_repo}"
printf '第一版\n' > "${seed_repo}/README.md"
git -C "${seed_repo}" add README.md
git -C "${seed_repo}" -c user.name='测试' -c user.email='test@example.invalid' commit -q -m '初始化'
git clone -q --bare "${seed_repo}" "${old_remote}"

for index in {1..13}; do
  printf 'repo-%02d\trepo-%02d\t%s\tmaster\n' "${index}" "${index}" "${old_manifest_remote}" >> "${manifest}"
done

{
  printf '# 参考代码清单\n\n'
  printf '| 仓库 | 本地目录 | 远端 | 分支 | HEAD | 同步时间 |\n'
  printf '| --- | --- | --- | --- | --- | --- |\n'
  for index in {1..13}; do
    printf '| `repo-%02d` | `参考代码/repo-%02d` | `%s` | `master` | `seed-head-%02d` | 2026-01-01 00:00:00 +0800 |\n' \
      "${index}" "${index}" "${old_manifest_remote}" "${index}"
  done
} > "${reference_doc}"

REFS_REPOSITORIES='repo-01' REFS_ROOT="${refs_root}" REFS_MANIFEST="${manifest}" REFS_DOC="${reference_doc}" \
  "${project_root}/scripts/reference-repos.sh" sync >/dev/null

test -d "${repo}"
test ! -e "${refs_root}/repo-02"
grep -Fq '| `repo-01` | `参考代码/repo-01` |' "${reference_doc}"
grep -Fq '| `repo-02` | `参考代码/repo-02` |' "${reference_doc}"
[[ "$(git -C "${repo}" remote get-url origin)" == "${old_manifest_remote}" ]]

git clone -q --bare "${seed_repo}" "${new_remote}"
printf '第二版\n' > "${seed_repo}/README.md"
git -C "${seed_repo}" add README.md
git -C "${seed_repo}" -c user.name='测试' -c user.email='test@example.invalid' commit -q -m '更新'
git -C "${seed_repo}" push -q "${new_remote}" master:master
sed -i "s|${old_manifest_remote}|${new_manifest_remote}|" "${manifest}"

REFS_REPOSITORIES='repo-01' REFS_ROOT="${refs_root}" REFS_MANIFEST="${manifest}" REFS_DOC="${reference_doc}" \
  "${project_root}/scripts/reference-repos.sh" sync >/dev/null

[[ "$(git -C "${repo}" remote get-url origin)" == "${new_manifest_remote}" ]]
grep -Fq '第二版' "${repo}/README.md"
grep -Fq '| `repo-01` | `参考代码/repo-01` |' "${reference_doc}"
grep -Fq 'seed-head-02' "${reference_doc}"
if grep -Fq 'seed-head-01' "${reference_doc}"; then
  printf '参考清单中的已同步仓库 HEAD 没有刷新\n' >&2
  exit 1
fi

printf '指定参考仓库同步与安全远端迁移测试通过\n'
