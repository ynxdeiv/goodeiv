#!/usr/bin/env bash
set -uo pipefail

file="$(jq -r '.tool_input.file_path // empty')"
case "$file" in
  *.go) ;;
  *) exit 0 ;;
esac
[ -f "$file" ] || exit 0

cd "${CLAUDE_PROJECT_DIR:-.}"

gofmt -w "$file"
if ! out="$(go run ./internal/tools/nocomments "$file" 2>&1)"; then
  echo "$out" >&2
  echo "Only doc comments on exported API are allowed (AGENTS.md §2)." >&2
  exit 2
fi
