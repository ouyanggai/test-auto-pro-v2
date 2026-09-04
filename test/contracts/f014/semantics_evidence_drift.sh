#!/usr/bin/env bash

# F-014 证据漂移检测：解析 docs/TARGET_SEMANTICS.md 的 evidence 块，断言引用的文件与关键字符串
# 在参考仓库当前 HEAD 下仍然存在；再固定断言本次同步已证明会变动的几处符号与数量，
# 最后输出三个相关仓库的 HEAD，便于对照证据失效范围。
#
# 参考仓库是本机克隆，内容会随 make refs-sync 变化。证据失效不是本工具的缺陷，
# 而是必须重新勘定语义清单的信号，所以这里失败即中止。

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "${project_root}"

semantics='docs/TARGET_SEMANTICS.md'
refs='参考代码'

if [ ! -f "${semantics}" ]; then
  printf '%s\n' "[F-014] 缺少语义清单：${semantics}" >&2
  exit 1
fi

printf '%s\n' '[F-014] 参考仓库 HEAD'
for repo in rsh-framework-all java-serve/rsh-cloud-workflow-center java-serve/rsh-cloud-workflow-center-api java-serve/rsh-cloud-web-api rsh-cloud-invest-power-system; do
  if [ -d "${refs}/${repo}/.git" ]; then
    printf '  %-46s %s\n' "${repo}" "$(git -C "${refs}/${repo}" rev-parse --short=12 HEAD)"
  elif [ -d "${refs}/${repo}" ]; then
    printf '  %-46s %s\n' "${repo}" '（非独立 git 目录，随上层仓库同步）'
  else
    printf '%s\n' "[F-014] 缺少参考仓库：${repo}" >&2
    exit 1
  fi
done

# 逐条校验 evidence 块。file 不存在或 contains 消失即判漂移。
python3 - "${semantics}" <<'PY'
import io, os, re, sys

text = io.open(sys.argv[1], encoding='utf-8').read()
blocks = re.findall(r'```evidence\n(.*?)```', text, re.S)
if not blocks:
    print('[F-014] 语义清单里没有任何 evidence 块', file=sys.stderr)
    raise SystemExit(1)
failures = []
for block in blocks:
    fields = dict(line.split('=', 1) for line in block.strip().split('\n') if '=' in line)
    path, needle, strength = fields.get('file', ''), fields.get('contains', ''), fields.get('strength', '')
    if not path or not needle:
        failures.append('证据块缺少 file 或 contains：%s' % block.strip().replace('\n', ' / '))
        continue
    if strength not in ('源码可证明', '源码推断、待 F-016 实测'):
        failures.append('证据块的 strength 取值不合法：%s' % strength)
    if not os.path.isfile(path):
        failures.append('证据文件已不存在：%s' % path)
        continue
    if needle not in io.open(path, encoding='utf-8', errors='replace').read():
        failures.append('证据字符串已消失：%s -> %s' % (path, needle))
for failure in failures:
    print('[F-014] %s' % failure, file=sys.stderr)
if failures:
    raise SystemExit(1)
print('[F-014] evidence 块校验通过：%d 条' % len(blocks))
PY

# 固定断言：本次参考仓库同步已证明这几处会变动，必须逐次核对。
server='参考代码/rsh-framework-all/rsh-framework-cloud-server/src/com/rsh/framework/cloud'
commons='参考代码/rsh-framework-all/rsh-framework-cloud-commons/src/com/rsh/framework/cloud/commons'

assert_contains() {
  if ! grep -qF "$2" "$1"; then
    printf '%s\n' "[F-014] 固定断言失败：$1 里找不到 $2" >&2
    exit 1
  fi
}

assert_contains "${server}/flow/FlowSubmitVerifyBaseController.java" 'REDIS_HASH_KEY = "flowSubmitVerifyMap"'
assert_contains "${server}/flow/FlowSubmitVerifyAspect.java" 'hashGetNormalizedOwner'
assert_contains '参考代码/rsh-framework-all/rsh-framework-redis/src/com/rsh/framework/redis/configure/RshRedisServer.java' 'hashGetNormalizedOwner'

handlers="$(grep -c '@ExceptionHandler' "${server}/server/GlobalExceptionHandler.java")"
if [ "${handlers}" -ne 9 ]; then
  printf '%s\n' "[F-014] GlobalExceptionHandler 的异常处理器数量由 9 变为 ${handlers}，第 1.2 节映射表需重新勘定" >&2
  exit 1
fi

audit_ways="$(grep -cE '^[[:space:]]+[A-Za-z_][A-Za-z_0-9]*\("' "${commons}/workflow/web/model/enums/AuditWayEnum.java")"
if [ "${audit_ways}" -ne 143 ]; then
  printf '%s\n' "[F-014] AuditWayEnum 枚举项数由 143 变为 ${audit_ways}，需确认动态审批方式相关结论是否受影响" >&2
  exit 1
fi

printf '%s\n' "[F-014] 固定断言通过（异常处理器 ${handlers} 个、AuditWayEnum ${audit_ways} 项）"
printf '%s\n' 'F-014 证据漂移检测通过'
