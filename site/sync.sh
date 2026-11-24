#!/usr/bin/env bash
# Generate Starlight pages from ../docs. Pages are not committed (see .gitignore).
set -euo pipefail
cd "$(dirname "$0")"

ref="${SITE_REF:-main}"
repo=https://github.com/werbot/werbot
out=src/content/docs
rm -f "$out"/*.md

for f in ../docs/*.md; do
  name=$(basename "$f" .md)
  title=$(sed -n 's/^# //p' "$f" | head -n1 | sed 's/"/\\"/g')
  {
    printf -- '---\ntitle: "%s"\neditUrl: %s/blob/main/docs/%s.md\n---\n' "$title" "$repo" "$name"
    tail -n +2 "$f" | sed -E \
      -e "s#\]\(\.\./([^)]+)\)#](${repo}/blob/${ref}/\1)#g" \
      -e 's#\]\(([a-z0-9-]+)\.md(\#[^)]*)?\)#](/\1/\2)#g'
  } > "$out/$name.md"
done
