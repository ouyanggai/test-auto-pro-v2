#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
refs_root="${REFS_ROOT:-${project_root}/参考代码}"
manifest="${REFS_MANIFEST:-${project_root}/scripts/reference-repositories.tsv}"
reference_doc="${REFS_DOC:-${project_root}/docs/REFERENCE_CODE.md}"

declare -a names=()
declare -a paths=()
declare -a remotes=()
declare -a branches=()
declare -a remote_heads=()
declare -a selected_names=()
declare -i total_count=0

fail() {
  printf '错误：%s\n' "$*" >&2
  exit 1
}

load_manifest() {
  [[ -f "${manifest}" ]] || fail "参考仓库清单不存在：${manifest}"

  while IFS=$'\t' read -r name relative_path remote branch extra; do
    [[ -z "${name}" || "${name}" == \#* ]] && continue
    [[ -z "${extra:-}" ]] || fail "清单字段过多：${name}"
    [[ -n "${relative_path}" && -n "${remote}" && -n "${branch}" ]] || fail "清单字段不完整：${name}"
    [[ "${relative_path}" != /* && "${relative_path}" != *".."* ]] || fail "清单目录不安全：${relative_path}"

    names+=("${name}")
    paths+=("${refs_root}/${relative_path}")
    remotes+=("${remote}")
    branches+=("${branch}")
  done < "${manifest}"

  total_count="${#names[@]}"
  [[ "${total_count}" -eq 13 ]] || fail "参考仓库数量必须为 13，当前是 ${total_count}"
  load_selection
}

# load_selection 只允许清单中已有且去重后的仓库名，供机房迁移等场景安全同步单个来源。
load_selection() {
  local raw="${REFS_REPOSITORIES:-}"
  local name
  [[ -z "${raw}" ]] && return 0

  IFS=',' read -ra raw_names <<<"${raw}"
  for name in "${raw_names[@]}"; do
    name="$(printf '%s' "${name}" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    [[ -n "${name}" ]] || fail "指定参考仓库不能为空"
    [[ "${#selected_names[@]}" -eq 0 ]] || [[ " ${selected_names[*]} " != *" ${name} "* ]] || fail "指定参考仓库重复：${name}"

    local found=0
    local existing
    for existing in "${names[@]}"; do
      if [[ "${existing}" == "${name}" ]]; then
        found=1
        break
      fi
    done
    [[ "${found}" -eq 1 ]] || fail "指定参考仓库不存在：${name}"
    selected_names+=("${name}")
  done

  [[ "${#selected_names[@]}" -gt 0 ]] || fail "指定参考仓库不能为空"
  filter_selected_repositories
}

# filter_selected_repositories 只缩小本次操作范围，不改变完整清单的 13 仓库校验。
filter_selected_repositories() {
  [[ "${#selected_names[@]}" -gt 0 ]] || return 0

  local kept_names=()
  local kept_paths=()
  local kept_remotes=()
  local kept_branches=()
  local index
  for index in "${!names[@]}"; do
    if [[ " ${selected_names[*]} " == *" ${names[index]} "* ]]; then
      kept_names+=("${names[index]}")
      kept_paths+=("${paths[index]}")
      kept_remotes+=("${remotes[index]}")
      kept_branches+=("${branches[index]}")
    fi
  done

  names=("${kept_names[@]}")
  paths=("${kept_paths[@]}")
  remotes=("${kept_remotes[@]}")
  branches=("${kept_branches[@]}")
}

read_remote_head() {
  local remote="$1"
  local branch="$2"
  local output

  output="$(git ls-remote --exit-code --heads "${remote}" "refs/heads/${branch}")" || return 1
  printf '%s\n' "${output%%[[:space:]]*}"
}

check_existing_repo() {
  local index="$1"
  local repo="${paths[index]}"
  local expected_remote="${remotes[index]}"
  local expected_branch="${branches[index]}"
  local actual_remote
  local actual_branch

  [[ -d "${repo}" ]] || return 0
  git -C "${repo}" rev-parse --is-inside-work-tree >/dev/null 2>&1 || fail "目录不是 Git 仓库：${repo}"

  actual_remote="$(git -C "${repo}" remote get-url origin 2>/dev/null)" || fail "仓库缺少 origin：${repo}"
  actual_branch="$(git -C "${repo}" branch --show-current)"
  [[ "${actual_branch}" == "${expected_branch}" ]] || fail "分支不符：${repo}，实际为 ${actual_branch}"

  [[ -z "$(git -C "${repo}" status --porcelain --untracked-files=all)" ]] || fail "仓库存在未提交修改，拒绝同步：${repo}"

  if [[ "${actual_remote}" != "${expected_remote}" ]]; then
    migrate_existing_remote "${repo}" "${expected_remote}" "${expected_branch}"
    actual_remote="$(git -C "${repo}" remote get-url origin)"
    [[ "${actual_remote}" == "${expected_remote}" ]] || fail "远端迁移失败：${repo}"
  fi
}

# migrate_existing_remote 只在分支正确、工作树干净且新远端包含本地历史时切换 origin。
function migrate_existing_remote() {
  local repo="$1"
  local expected_remote="$2"
  local expected_branch="$3"
  local local_head
  local remote_head

  local_head="$(git -C "${repo}" rev-parse HEAD)"
  remote_head="$(read_remote_head "${expected_remote}" "${expected_branch}")" || fail "无法读取新远端分支：${expected_remote} ${expected_branch}"
  git -C "${repo}" fetch --quiet "${expected_remote}" "refs/heads/${expected_branch}:refs/remotes/refs-sync-migration/${expected_branch}" || fail "拉取新远端信息失败：${repo}"
  git -C "${repo}" merge-base --is-ancestor "${local_head}" "${remote_head}" || fail "新远端与本地历史分叉，拒绝迁移：${repo}"
  git -C "${repo}" remote set-url origin "${expected_remote}" || fail "更新 origin 失败：${repo}"
  git -C "${repo}" fetch --quiet origin "${expected_branch}" || fail "按新远端拉取跟踪分支失败：${repo}"
  printf '安全迁移远端：%s\n' "${repo}"
}

query_all_remote_heads() {
  local index
  local head

  remote_heads=()
  for index in "${!names[@]}"; do
    printf '核对远端：%s (%s)\n' "${names[index]}" "${branches[index]}"
    head="$(read_remote_head "${remotes[index]}" "${branches[index]}")" || fail "无法读取远端分支：${remotes[index]} ${branches[index]}"
    [[ -n "${head}" ]] || fail "远端分支没有 HEAD：${names[index]}"
    remote_heads+=("${head}")
  done
}

preflight_sync() {
  local index
  local repo
  local local_head

  for index in "${!names[@]}"; do
    repo="${paths[index]}"
    check_existing_repo "${index}"
    [[ -d "${repo}" ]] || continue

    git -C "${repo}" fetch --quiet origin "${branches[index]}" || fail "拉取远端信息失败：${repo}"
    git -C "${repo}" cat-file -e "${remote_heads[index]}^{commit}" 2>/dev/null || fail "远端 HEAD 不可读取：${repo}"
    local_head="$(git -C "${repo}" rev-parse HEAD)"
    if [[ "${local_head}" != "${remote_heads[index]}" ]]; then
      git -C "${repo}" merge-base --is-ancestor "${local_head}" "${remote_heads[index]}" || fail "仓库已领先或与远端分叉，拒绝同步：${repo}"
    fi
  done
}

sync_repositories() {
  local index
  local repo
  local final_remote_head

  query_all_remote_heads
  preflight_sync

  for index in "${!names[@]}"; do
    repo="${paths[index]}"
    if [[ -d "${repo}" ]]; then
      printf '快进同步：%s\n' "${names[index]}"
      git -C "${repo}" pull --ff-only --quiet origin "${branches[index]}" || fail "无法快进同步：${repo}"
    else
      printf '干净克隆：%s\n' "${names[index]}"
      mkdir -p "$(dirname "${repo}")"
      git clone --quiet --branch "${branches[index]}" "${remotes[index]}" "${repo}" || fail "克隆失败：${repo}"
    fi

    check_existing_repo "${index}"
    final_remote_head="$(read_remote_head "${remotes[index]}" "${branches[index]}")" || fail "同步后无法核对远端：${repo}"
    [[ "$(git -C "${repo}" rev-parse HEAD)" == "${final_remote_head}" ]] || fail "同步期间远端发生变化，请重新执行：${repo}"
    remote_heads[index]="${final_remote_head}"
  done

  if [[ "${#names[@]}" -eq "${total_count}" ]]; then
    write_reference_doc
  else
    update_selected_reference_doc
    printf '指定仓库同步完成，参考清单文档保持全量同步时重写。\n'
  fi
  printf '参考仓库同步完成：%d/%d\n' "${#names[@]}" "${total_count}"
}

write_reference_doc() {
  local synced_at
  local temporary_doc
  local index
  local relative_path

  synced_at="$(date '+%Y-%m-%d %H:%M:%S %z')"
  mkdir -p "$(dirname "${reference_doc}")"
  temporary_doc="${reference_doc}.tmp.$$"

  {
    printf '# 参考代码清单\n\n'
    printf '本文件由 `make refs-sync` 生成。`参考代码/` 被当前 Git 仓库忽略；同步只允许首次干净克隆或对正确分支执行 `pull --ff-only`。\n\n'
    printf '| 仓库 | 本地目录 | 远端 | 分支 | HEAD | 同步时间 |\n'
    printf '| --- | --- | --- | --- | --- | --- |\n'
    for index in "${!names[@]}"; do
      relative_path="${paths[index]#${refs_root}/}"
      printf '| `%s` | `参考代码/%s` | `%s` | `%s` | `%s` | %s |\n' \
        "${names[index]}" "${relative_path}" "${remotes[index]}" "${branches[index]}" \
        "${remote_heads[index]}" "${synced_at}"
    done
    printf '\n## 使用边界\n\n'
    printf -- '- 目标平台真实代码和运行结果是业务规则依据。\n'
    printf -- '- `rsh-cloud-invest-power-system` 只分析 `GroupApproveManage` 及其直接引用公共组件。\n'
    printf -- '- 任何脏目录、分支不符、分叉或远端错误都会中止同步；禁止 reset 或 checkout 覆盖。\n'
  } > "${temporary_doc}"

  mv "${temporary_doc}" "${reference_doc}"
}

# update_selected_reference_doc 只刷新已选仓库记录，避免部分同步伪装为全量同步。
update_selected_reference_doc() {
  local temporary_doc
  local index
  local relative_path
  local synced_at

  [[ -f "${reference_doc}" ]] || fail "参考清单文档不存在，不能部分更新：${reference_doc}"
  temporary_doc="${reference_doc}.tmp.$$"
  synced_at="$(date '+%Y-%m-%d %H:%M:%S %z')"
  cp "${reference_doc}" "${temporary_doc}"
  for index in "${!names[@]}"; do
    relative_path="${paths[index]#${refs_root}/}"
    REFS_NAMES="${names[index]}" REFS_PATH="参考代码/${relative_path}" \
      REFS_REMOTE="${remotes[index]}" REFS_BRANCH="${branches[index]}" \
      REFS_HEAD="${remote_heads[index]}" REFS_TIME="${synced_at}" REFS_DOC="${temporary_doc}" \
      python -c 'import os, pathlib; e=os.environ; p=pathlib.Path(e["REFS_DOC"]); name=e["REFS_NAMES"]; key="| `{}` |".format(name); rows=p.read_text(encoding="utf-8").splitlines(); matches=[i for i,row in enumerate(rows) if key in row]; assert len(matches)==1, "参考清单仓库行必须且只能有一条: {}".format(key); i=matches[0]; rows[i]="| `{}` | `{}` | `{}` | `{}` | `{}` | {} |".format(name,e["REFS_PATH"],e["REFS_REMOTE"],e["REFS_BRANCH"],e["REFS_HEAD"],e["REFS_TIME"]); p.write_text("\n".join(rows)+"\n", encoding="utf-8")' \
      || { rm -f "${temporary_doc}"; fail "更新参考清单失败：${names[index]}"; }
  done
  mv "${temporary_doc}" "${reference_doc}"
}

show_status() {
  local index
  local repo
  local local_head
  local remote_head
  local has_error=0
  local state

  printf '%-42s %-12s %-12s %s\n' '仓库' '分支' '状态' 'HEAD'
  for index in "${!names[@]}"; do
    repo="${paths[index]}"
    state='正常'

    if [[ ! -d "${repo}" ]]; then
      printf '%-42s %-12s %-12s %s\n' "${names[index]}" "${branches[index]}" '未克隆' '-'
      has_error=1
      continue
    fi

    if ! git -C "${repo}" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
      printf '%-42s %-12s %-12s %s\n' "${names[index]}" "${branches[index]}" '非仓库' '-'
      has_error=1
      continue
    fi

    if [[ "$(git -C "${repo}" remote get-url origin 2>/dev/null || true)" != "${remotes[index]}" ]]; then
      state='远端不符'
    elif [[ "$(git -C "${repo}" branch --show-current)" != "${branches[index]}" ]]; then
      state='分支不符'
    elif [[ -n "$(git -C "${repo}" status --porcelain --untracked-files=all)" ]]; then
      state='工作树脏'
    else
      remote_head="$(read_remote_head "${remotes[index]}" "${branches[index]}" || true)"
      local_head="$(git -C "${repo}" rev-parse HEAD)"
      if [[ -z "${remote_head}" ]]; then
        state='远端错误'
      elif [[ "${local_head}" != "${remote_head}" ]]; then
        state='未同步'
      fi
    fi

    local_head="$(git -C "${repo}" rev-parse --short=12 HEAD 2>/dev/null || printf '-')"
    printf '%-42s %-12s %-12s %s\n' "${names[index]}" "${branches[index]}" "${state}" "${local_head}"
    [[ "${state}" == '正常' ]] || has_error=1
  done

  [[ "${has_error}" -eq 0 ]]
}

main() {
  load_manifest
  case "${1:-}" in
    sync) sync_repositories ;;
    status) show_status ;;
    *) fail '用法：reference-repos.sh sync|status' ;;
  esac
}

main "$@"
