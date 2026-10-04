#!/usr/bin/env bash
#
# generate-release-notes.sh —— 生成 GitHub Release 正文（release notes）。
#
# 由 .github/workflows/release-build.yml 的 release job 调用，也可本地预览：
#   GITHUB_REF_NAME=v1.2.3 GITHUB_REPOSITORY=KingPrimes/NyxBot-Go \
#     bash .github/scripts/generate-release-notes.sh > release-notes.md
#
# 输出（Markdown）分节：
#   1. 发布信息      版本 / 提交 / 内嵌前端 commit / 支持平台 / Docker 镜像标签
#   2. 发布重点      可选：build/release-notes/<tag>.md 存在时原样引入（人工补充“这次改了什么值得注意”）
#   3. 本次变更明细  按约定式提交类型（feat/fix/...）分组的逐条改动，带 PR 与 commit 链接
#   4. 参与贡献      去重后的贡献者（GitHub 登录名）与各自提交数，并标出首次参与者
#   5. 下载与校验    产物文件名 / 大小 / SHA256
#   6. 完整变更日志  与上一版本的 compare 链接
#
# 环境变量（全部可选，CI 里由 workflow 传入）：
#   GITHUB_REF_NAME       目标 tag；缺省取仓库中最近的 v* tag
#   GITHUB_REPOSITORY     owner/repo；缺省从 origin 推断
#   VERSION               版本号（build/metadata/app.json 或 tag）；缺省从 tag 推导
#   PRODUCT_NAME          产品名，用于标题
#   EXECUTABLE_NAME       可执行文件名，用于产物列出
#   WEBUI_REPOSITORY      内嵌前端仓库，默认 KingPrimes/NyxBot-WebUI
#   WEBUI_COMMIT          内嵌前端的 commit sha
#   WEBUI_SUBJECT         内嵌前端的 commit 标题
#   ASSETS_DIR            产物目录，默认 release-assets（内含 SHA256SUMS.txt）
#   GH_TOKEN              可选；用于把提交邮箱映射成 GitHub 登录名，缺失时回退为提交作者名
#
# 退出码：恒为 0——发布说明生成失败不应该阻断发版；缺数据时对应小节退化为占位文案，
#  并配合 workflow 里“正文为空则回退最小正文”的兜底。异常信息写 stderr，日志里可见。

set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$REPO_ROOT"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# ---------------------------------------------------------------------------
# 0. 参数与环境
# ---------------------------------------------------------------------------

TAG="${1:-${GITHUB_REF_NAME:-}}"
if [[ -z "$TAG" ]]; then
  TAG="$(git describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null || true)"
fi
if [[ -z "$TAG" ]]; then
  TAG="$(git rev-parse --short HEAD)"
  echo "warn: 未指定 tag，回退为 HEAD ($TAG)" >&2
fi

REPO_SLUG="${GITHUB_REPOSITORY:-}"
if [[ -z "$REPO_SLUG" ]]; then
  _url="$(git config --get remote.origin.url || true)"
  _url="${_url%.git}"
  REPO_SLUG="${_url##*github.com[:/]}"
fi
if [[ "$REPO_SLUG" != */* ]]; then
  echo "warn: 无法从 origin 推断 owner/repo，链接将不可用" >&2
  REPO_SLUG="unknown/unknown"
fi
REPO_URL="https://github.com/${REPO_SLUG}"

# 目标提交：优先用 tag 解析（GITHUB_SHA 在 tag 触发时同样是该提交）
TARGET_SHA="$(git rev-parse "${TAG}^{commit}" 2>/dev/null || git rev-parse HEAD)"

# 上一个版本 tag：取「本 tag 的父提交」可达的最近 v* tag；没有则视为首个版本
PREV_TAG=""
if git rev-parse -q --verify "refs/tags/${TAG}" >/dev/null 2>&1; then
  PREV_TAG="$(git describe --tags --abbrev=0 --match 'v[0-9]*' "${TAG}^" 2>/dev/null || true)"
fi
if [[ "$PREV_TAG" == "$TAG" ]]; then
  PREV_TAG=""
fi

if [[ -n "$PREV_TAG" ]]; then
  RANGE="${PREV_TAG}..${TAG}"
else
  RANGE="${TAG}"
fi

# 版本号：CI 传入的 build/metadata/app.json 值优先，其次从 tag 去掉 v 前缀
VERSION="${VERSION:-${TAG#v}}"
PRODUCT_NAME="${PRODUCT_NAME:-NyxBot-Go}"
ASSETS_DIR="${ASSETS_DIR:-release-assets}"
WEBUI_REPOSITORY="${WEBUI_REPOSITORY:-KingPrimes/NyxBot-WebUI}"

BUILD_TIME="$(date -u '+%Y-%m-%d %H:%M UTC')"

# ---------------------------------------------------------------------------
# 1. 收集提交（不看 merge commit：squash 合并的 PR 标题已在被合并提交里）
# ---------------------------------------------------------------------------

# 注意：--pretty=format: 只在记录之间插换行，最后一条不带换行；这里补一个换行，
# 否则 while read 循环会丢掉末行（区间里最老的那个提交）——配合循环末尾的 || [[ -n $sha ]]
{
  git log --no-merges --pretty=format:'%h%x09%s' "$RANGE" 2>/dev/null
  printf '\n'
} > "$WORK/commits.tsv"
COMMIT_TOTAL="$(grep -c . "$WORK/commits.tsv" 2>/dev/null || true)"
COMMIT_TOTAL="${COMMIT_TOTAL:-0}"

GROUP_DIR="$WORK/groups"
mkdir -p "$GROUP_DIR"
declare -A GROUP_COUNT=()

# 约定式提交类型 → 分组键（未知类型统一落到 other）
classify() {
  case "$1" in
    feat|fix|perf|refactor|docs|test|build|ci|chore|style|revert) printf '%s' "$1" ;;
    *) printf 'other' ;;
  esac
}

group_title() {
  case "$1" in
    feat)     printf '✨ 新功能' ;;
    fix)      printf '🐛 缺陷修复' ;;
    perf)     printf '⚡ 性能优化' ;;
    refactor) printf '♻️ 重构' ;;
    docs)     printf '📝 文档' ;;
    test)     printf '✅ 测试' ;;
    build)    printf '📦 构建与依赖' ;;
    ci)       printf '🔧 CI 与发布' ;;
    chore)    printf '🧹 杂项维护' ;;
    style)    printf '🎨 代码格式' ;;
    revert)   printf '⏪ 回滚' ;;
    *)        printf '🔀 其他改动' ;;
  esac
}

while IFS=$'\t' read -r sha subject || [[ -n "$sha" ]]; do
  [[ -z "$subject" ]] && continue
  type="other"
  scope=""
  body="$subject"
  if [[ "$subject" =~ ^([a-zA-Z]+)(\(([^()]*)\))?!?:[[:space:]]*(.*)$ ]]; then
    type="$(classify "${BASH_REMATCH[1],,}")"
    scope="${BASH_REMATCH[3]}"
    body="${BASH_REMATCH[4]}"
  fi
  # (#123) → PR 链接；反引号会吞掉后面内容，统一转义
  body_link="$(printf '%s' "$body" | sed -E "s@\(#([0-9]+)\)@([#\1](${REPO_URL}/pull/\1))@g")"
  body_link="${body_link//\`/\\\`}"
  if [[ -n "$scope" ]]; then
    printf -- '- **%s** %s — [`%s`](%s/commit/%s)\n' "$scope" "$body_link" "$sha" "$REPO_URL" "$sha" >> "$GROUP_DIR/$type"
  else
    printf -- '- %s — [`%s`](%s/commit/%s)\n' "$body_link" "$sha" "$REPO_URL" "$sha" >> "$GROUP_DIR/$type"
  fi
  GROUP_COUNT["$type"]=$(( ${GROUP_COUNT["$type"]:-0} + 1 ))
done < "$WORK/commits.tsv"

# 变更规模（首个版本没有可比基线，略过）
SHORTSTAT=""
if [[ -n "$PREV_TAG" ]]; then
  SHORTSTAT="$(git diff --shortstat "$PREV_TAG" "$TARGET_SHA" 2>/dev/null || true)"
  SHORTSTAT="${SHORTSTAT# }"
fi

# ---------------------------------------------------------------------------
# 2. 收集贡献者：邮箱 → GitHub 登录名（尽力而为），再按登录名/邮箱去重
# ---------------------------------------------------------------------------

git log --no-merges --pretty=format:'%aN%x09%aE' "$RANGE" > "$WORK/authors.tsv" 2>/dev/null || : > "$WORK/authors.tsv"
: > "$WORK/logins.tsv"
if command -v gh >/dev/null 2>&1 && [[ -n "${GH_TOKEN:-${GITHUB_TOKEN:-}}" ]]; then
  # 只取该 tag 上已关联 GitHub 账号的提交；失败（无网络/无权限/非 GitHub）时静默回退
  gh api --paginate "repos/${REPO_SLUG}/commits?sha=${TARGET_SHA}&per_page=100" \
    --jq '.[] | select(.author.login != null) | [(.commit.author.email // ""), .author.login, (.commit.author.name // "")] | @tsv' \
    > "$WORK/logins.tsv" 2>/dev/null || : > "$WORK/logins.tsv"
fi

# 聚合：key = GitHub 登录名（已知时）否则作者名；输出 提交数 / 人机标记 / 显示名 / 姓名 / 邮箱集合
# 注意：登录名映射表用 BEGIN + getline 读入，不能靠 NR==FNR —— 该文件可能为空，
# 空的首个输入文件会让 NR==FNR 一直成立，把作者记录全部当成映射表吃掉。
awk -F'\t' -v logins="$WORK/logins.tsv" '
  function isbot(name, email) {
    return (name ~ /\[bot\]$/) || (name ~ /^[Dd]ependabot/) || (email ~ /\[bot\]/) || (email ~ /bot@/)
  }
  BEGIN {
    while ((getline line < logins) > 0) {
      split(line, f, "\t")
      if (f[1] != "" && f[2] != "") login[tolower(f[1])] = f[2]
      # 同一账号往往只有部分提交邮箱被 GitHub 关联（其余提交的 author 为 null，取不到登录名），
      # 所以再按提交里显示的作者名兜底映射一次，否则同一个人会被拆成「@登录名」和「作者名」两行
      if (f[2] != "" && f[3] != "") byname[tolower(f[3])] = f[2]
    }
    close(logins)
  }
  {
    name = $1; email = tolower($2)
    if (email == "") next
    lgn = ""
    if (email in login) lgn = login[email]
    else if (!isbot(name, email) && (tolower(name) in byname)) lgn = byname[tolower(name)]
    if (lgn != "") key = "login:" lgn
    else if (name != "") key = "name:" tolower(name)
    else key = "email:" email
    if (!(key in cnt)) {
      order[++n] = key
      disp[key] = (lgn != "") ? "@" lgn : name
      names[key] = name
      emails[key] = email
      flag[key] = isbot(name, email) ? "bot" : "human"
    } else if (index(emails[key] ",", email ",") == 0) {
      emails[key] = emails[key] "," email
    }
    cnt[key]++
  }
  END {
    for (i = 1; i <= n; i++) {
      k = order[i]
      printf "%d\t%s\t%s\t%s\t%s\n", cnt[k], flag[k], disp[k], names[k], emails[k]
    }
  }
' "$WORK/authors.tsv" | sort -t$'\t' -k1,1rn > "$WORK/contributors.tsv"

CONTRIBUTOR_TOTAL="$(awk -F'\t' '$2=="human"' "$WORK/contributors.tsv" | wc -l | tr -d ' ')"
CONTRIBUTOR_COMMITS="$(awk -F'\t' '$2=="human" {s+=$1} END {print s+0}' "$WORK/contributors.tsv")"

# 首次参与者：该版本之前从未在本仓库提交过（按邮箱精确比对，避免 --author 的正则误伤）
: > "$WORK/newcomers.tsv"
: > "$WORK/prev-authors.txt"
if [[ -n "$PREV_TAG" ]]; then
  git log --format='%aE' "$PREV_TAG" > "$WORK/prev-authors.txt" 2>/dev/null || :
  awk -F'\t' -v prev="$WORK/prev-authors.txt" '
    BEGIN {
      while ((getline e < prev) > 0) { if (e != "") seen[tolower(e)] = 1 }
      close(prev)
    }
    $2 == "human" {
      isnew = 1
      n = split($5, list, ",")
      for (i = 1; i <= n; i++) { if (tolower(list[i]) in seen) { isnew = 0; break } }
      if (isnew) printf "%s\t%s\n", $3, $1
    }
  ' "$WORK/contributors.tsv" > "$WORK/newcomers.tsv"
fi

# ---------------------------------------------------------------------------
# 3. 产物清单（文件名 / 大小 / SHA256）
# ---------------------------------------------------------------------------

: > "$WORK/assets.md"
if [[ -d "$ASSETS_DIR" ]]; then
  for f in "$ASSETS_DIR"/*; do
    [[ -f "$f" ]] || continue
    name="$(basename "$f")"
    [[ "$name" == "SHA256SUMS.txt" ]] && continue
    size="$(du -h "$f" 2>/dev/null | cut -f1)"
    # 兼容 GNU sha256sum 的二进制模式输出（Windows/MSYS 下为 "<hash> *<name>"）
    hash="$(awk -v n="$name" '{ f = $2; sub(/^\*/, "", f); if (f == n) { print $1; exit } }' \
      "$ASSETS_DIR/SHA256SUMS.txt" 2>/dev/null)"
    printf '| `%s` | %s | `%s` |\n' "$name" "${size:-?}" "${hash:-未生成}" >> "$WORK/assets.md"
  done
fi

# ---------------------------------------------------------------------------
# 4. 输出正文
# ---------------------------------------------------------------------------

printf '# %s %s\n\n' "$PRODUCT_NAME" "$TAG"
printf '> NyxBot 的 Go/Gin 后端；前端（[%s](https://github.com/%s)）已 `go:embed` 内嵌进二进制，下载即可运行。\n\n' \
  "$WEBUI_REPOSITORY" "$WEBUI_REPOSITORY"

printf '## 📌 发布信息\n\n'
printf '| 项目 | 值 |\n| --- | --- |\n'
printf '| 版本 | `%s` |\n' "$VERSION"
printf '| 提交 | [`%s`](%s/commit/%s) |\n' "$(printf '%s' "$TARGET_SHA" | cut -c1-10)" "$REPO_URL" "$TARGET_SHA"
if [[ -n "${WEBUI_COMMIT:-}" ]]; then
  printf '| 内嵌前端 | [%s@`%s`](https://github.com/%s/commit/%s) — %s |\n' \
    "$WEBUI_REPOSITORY" "$(printf '%s' "$WEBUI_COMMIT" | cut -c1-10)" "$WEBUI_REPOSITORY" "$WEBUI_COMMIT" \
    "$(printf '%s' "${WEBUI_SUBJECT:-}" | tr '|' '/')"
fi
printf '| 构建时间 | %s |\n' "$BUILD_TIME"
printf '| 支持平台 | linux/amd64、linux/arm64、darwin/amd64、darwin/arm64、windows/amd64 |\n'
printf '| Docker | `kingprimes/nyxbot-go:%s`（另有 `:latest` 与 semver 短标签）· `ghcr.io/%s/nyxbot-go:%s` |\n' \
  "$TAG" "${REPO_SLUG%%/*}" "$TAG"
printf '\n'

# 可选的人工“发布重点”
MANUAL_NOTES="$REPO_ROOT/build/release-notes/${TAG}.md"
if [[ -f "$MANUAL_NOTES" ]]; then
  printf '## 🎯 发布重点\n\n'
  cat "$MANUAL_NOTES"
  printf '\n'
fi

printf '## 🧾 本次变更明细\n\n'
if [[ -n "$PREV_TAG" ]]; then
  printf '对比基准：`%s` → `%s`，共 **%s** 个提交' "$PREV_TAG" "$TAG" "$COMMIT_TOTAL"
  [[ -n "$SHORTSTAT" ]] && printf '（%s）' "$SHORTSTAT"
  printf '。\n\n'
else
  printf '本版本为首个发布，收录全部 **%s** 个提交。\n\n' "$COMMIT_TOTAL"
fi

GROUP_ORDER=(feat fix perf refactor docs test build ci chore style revert other)
HAS_DETAIL=0
for type in "${GROUP_ORDER[@]}"; do
  if [[ -s "$GROUP_DIR/$type" ]]; then
    HAS_DETAIL=1
    printf '### %s (%s) — %s 项\n\n' "$(group_title "$type")" "$type" "${GROUP_COUNT["$type"]:-0}"
    cat "$GROUP_DIR/$type"
    printf '\n'
  fi
done
if [[ "$HAS_DETAIL" == "0" ]]; then
  printf '_未从提交记录中解析出变更明细，请查看下方完整变更日志。_\n\n'
fi

printf '## 👥 参与贡献\n\n'
if [[ "${CONTRIBUTOR_TOTAL:-0}" -gt 0 ]]; then
  printf '本版本共有 **%s** 位贡献者、**%s** 次提交（同一账号的多个提交邮箱已合并，不含合并提交）：\n\n' \
    "$CONTRIBUTOR_TOTAL" "$CONTRIBUTOR_COMMITS"
  printf '| 贡献者 | 提交数 |\n| --- | --- |\n'
  awk -F'\t' '$2=="human" { printf "| %s | %s |\n", $3, $1 }' "$WORK/contributors.tsv"
  printf '\n'
  if [[ -s "$WORK/newcomers.tsv" ]]; then
    printf '**首次参与本项目的贡献者**：'
    awk -F'\t' '{ n++; printf "%s%s（%s 次提交）", (n>1 ? "、" : ""), $1, $2 }' "$WORK/newcomers.tsv"
    printf ' 🎉\n\n'
  fi
  if [[ -n "$(awk -F'\t' '$2=="bot"' "$WORK/contributors.tsv")" ]]; then
    printf '自动化工具：'
    awk -F'\t' '$2=="bot" { n++; printf "%s%s（%s 次提交）", (n>1 ? "、" : ""), $4, $1 }' "$WORK/contributors.tsv"
    printf '\n\n'
  fi
else
  printf '本版本未解析出提交作者信息。\n\n'
fi

printf '## 📦 下载与校验\n\n'
printf '| 文件 | 大小 | SHA256 |\n| --- | --- | --- |\n'
cat "$WORK/assets.md"
printf '\n'
printf -- '- 校验：`sha256sum -c SHA256SUMS.txt`（macOS：`shasum -a 256 -c SHA256SUMS.txt`）。\n'
printf -- '- Linux / macOS 二进制下载后需自行 `chmod +x`（GitHub Release 不保留可执行位）。\n'
printf -- '- Windows 版为 `%s.exe`，已注入图标与版本资源。\n\n' "${EXECUTABLE_NAME:-NyxBot}"

printf '## 🚀 升级方式\n\n'
printf -- '- 替换可执行文件即可；`config.yaml`、`data/` 与 `resources/` 均在运行期自动生成/内嵌，无需手工迁移。\n'
printf -- '- 旧版本 `config.yaml` 缺失的新配置项会在启动时按注释补写，日志中会以 WARN 列出。\n\n'

printf '## 🔗 完整变更日志\n\n'
if [[ -n "$PREV_TAG" ]]; then
  printf '%s/compare/%s...%s\n' "$REPO_URL" "$PREV_TAG" "$TAG"
else
  printf '%s/commits/%s\n' "$REPO_URL" "$TAG"
fi

exit 0
