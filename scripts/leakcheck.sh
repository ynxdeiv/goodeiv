#!/usr/bin/env bash
set -euo pipefail

root="$(git rev-parse --show-toplevel)"
terms="$root/.local/forbidden-terms.txt"

[ -f "$terms" ] || exit 0

mode="${1:-files}"

if [ "$mode" = "message" ]; then
  if grep -inwFf "$terms" "$2"; then
    echo "leakcheck: commit message contains a forbidden term" >&2
    exit 1
  fi
  exit 0
fi

cd "$root"
files=()
while IFS= read -r -d '' path; do
  [ -f "$path" ] && [ ! -L "$path" ] && files+=("$path")
done < <(git ls-files -z --cached --others --exclude-standard)

if [ "${#files[@]}" -gt 0 ] && grep -inwIFf "$terms" -- "${files[@]}"; then
  echo "leakcheck: forbidden terms found in versionable files" >&2
  exit 1
fi

if git ls-files -z --cached --others --exclude-standard | tr '\0' '\n' | grep -iwFf "$terms"; then
  echo "leakcheck: file path contains a forbidden term" >&2
  exit 1
fi

branch="$(git symbolic-ref --quiet --short HEAD || true)"
if [ -n "$branch" ] && printf '%s\n' "$branch" | grep -iwFqf "$terms"; then
  echo "leakcheck: branch name contains a forbidden term" >&2
  exit 1
fi
