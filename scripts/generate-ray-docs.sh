#!/usr/bin/env bash
# Generate docs/ray/** (Jekyll pages) from ray/** README files.
# Run before `jekyll build`; keeps ray/ as the single source of truth.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="$ROOT/ray"
DST="$ROOT/docs/ray"

# category dir -> (nav title, nav_order under the Ray top-level page)
CATEGORIES=(
  "1-user-guide|Ray 用户指南|1"
  "2-advanced|Ray 高级实践|2"
  "3-e2e-examples|Ray 端到端示例|3"
)

category_title() {
  local dir="$1"
  for entry in "${CATEGORIES[@]}"; do
    [[ "$dir" == "${entry%%|*}" ]] && echo "${entry#*|}" && return
  done
  echo ""
}

# Extract the first H1 heading text (fallback: dir name); skip placeholder files.
page_title() {
  local file="$1" fallback="$2"
  local first
  first="$(grep -m 1 -v '^[[:space:]]*$' "$file" 2>/dev/null || true)"
  if [[ -z "$first" || "$first" == "# TODO" ]]; then
    echo ""
    return
  fi
  if [[ "$first" == \#* ]]; then
    echo "${first#\# }"
  else
    echo "$fallback"
  fi
}

# nav_order: numeric dir prefix * 10, or empty when no prefix.
dir_prefix() {
  local dir="$1"
  [[ "$dir" =~ ^([0-9]+)- ]] && echo "${BASH_REMATCH[1]}" || true
}

# Escape {{ }} so Go template snippets render literally instead of being parsed as Liquid.
escape_liquid() {
  perl -pe 's/(\{\{.*?\}\})/{% raw %}${1}{% endraw %}/g'
}

# Generate pages for one guide dir; recurse into nested dirs (e.g. nixl/).
# Only the Chinese version (README_ZH.md) is published, as index.md.
generate_dir() {
  local src_dir="$1" dst_dir="$2" parent_title="$3" prefix="$4"
  mkdir -p "$dst_dir"

  for subdir in "$src_dir"/*/; do
    [[ -d "$subdir" ]] || continue
    local name="${subdir%/}"
    name="${name##*/}"
    local sub_prefix="$prefix"
    local p
    p="$(dir_prefix "$name")"
    [[ -n "$p" ]] && sub_prefix="${sub_prefix:+${sub_prefix},}$((p * 10))"

    local zh_title=""
    [[ -f "$subdir/README_ZH.md" ]] && zh_title="$(page_title "$subdir/README_ZH.md" "$name")"

    if [[ -n "$zh_title" ]]; then
      mkdir -p "$dst_dir/$name"
      cat > "$dst_dir/$name/index.md" <<EOF
---
title: "$zh_title"
layout: default
parent: "$parent_title"
${sub_prefix:+nav_order: $sub_prefix}
---

$(cat "$subdir/README_ZH.md" | escape_liquid)
EOF
      echo "generated: docs/ray/${dst_dir#$DST/}/$name/index.md"
    fi

    local sub_parent="$parent_title"
    [[ -n "$zh_title" ]] && sub_parent="$zh_title"
    generate_dir "$subdir" "$dst_dir/$name" "$sub_parent" "$sub_prefix"
  done
}

rm -rf "$DST"

for entry in "${CATEGORIES[@]}"; do
  cat_dir="${entry%%|*}"
  rest="${entry#*|}"
  cat_title="${rest%%|*}"
  cat_order="${rest##*|}"
  src_cat="$SRC/$cat_dir"
  [[ -d "$src_cat" ]] || continue

  mkdir -p "$DST/$cat_dir"
  cat > "$DST/$cat_dir/index.md" <<EOF
---
title: "$cat_title"
layout: default
parent: "Ray"
nav_order: $cat_order
has_children: true
---

# $cat_title

本部分收录在 ACK 上使用 Ray 的实践文档。
EOF
  echo "generated: docs/ray/$cat_dir/index.md"

  generate_dir "$src_cat" "$DST/$cat_dir" "$cat_title" ""
done
