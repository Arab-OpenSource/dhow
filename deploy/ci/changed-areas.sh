#!/usr/bin/env bash
set -euo pipefail

event=${1:-}
if [[ $event != pull_request ]]; then
  printf 'code=true\nweb=true\n'
  exit 0
fi

code=false
web=false
while IFS= read -r file; do
  [[ -n $file ]] || continue
  case $file in
    docs/* | Plan/* | .claude/* | .github/ISSUE_TEMPLATE/* | .github/*.md | *.md) ;;
    apps/web/*) web=true ;;
    packages/openapi/*)
      web=true
      code=true
      ;;
    .github/workflows/ci.yml | .github/workflows/codeql.yml | deploy/ci/*)
      web=true
      code=true
      ;;
    *) code=true ;;
  esac
done < <(git diff --name-only HEAD^1 HEAD)
printf 'code=%s\nweb=%s\n' "$code" "$web"
