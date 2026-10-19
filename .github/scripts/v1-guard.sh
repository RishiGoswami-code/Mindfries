#!/usr/bin/env bash
# The V1 guard (v2/PLAN.md, Part 1 §1 and task 0.2).
#
# Reads a PR's changed paths, one per line, on stdin. Fails when V2 work and
# V1 changes are mixed in one PR.
#
#   HEAD_REF  the PR's head branch, e.g. feat/v2-development
#   LABELS    the PR's labels, comma-separated
#
# Rules:
#   - .github/ and the root CLAUDE.md are allowed in any PR.
#   - A PR labelled v1-seam is a connection point: it may change V1, but must
#     not touch v2/, so each seam lands on its own.
#   - Otherwise a V2 PR (one that touches v2/, or whose branch name has a
#     "v2" segment) may change nothing outside v2/.
#   - Any other PR is V1 work and passes.
set -euo pipefail

head_ref="${HEAD_REF:-}"
labels=",${LABELS:-},"

touches_v2=false
outside=()
while IFS= read -r path; do
  path="${path%$'\r'}"
  [ -z "$path" ] && continue
  case "$path" in
    v2/*) touches_v2=true ;;
    .github/*|CLAUDE.md) ;;
    *) outside+=("$path") ;;
  esac
done

if [[ "$labels" == *",v1-seam,"* ]]; then
  if $touches_v2; then
    echo "V1 guard: a v1-seam PR must not touch v2/. Put the V2 side in its own PR."
    exit 1
  fi
  echo "V1 guard: v1-seam PR, V1 changes allowed (${#outside[@]} paths)."
  exit 0
fi

is_v2=$touches_v2
if [[ "$head_ref" =~ (^|[/_-])v2([/_-]|$) ]]; then
  is_v2=true
fi

if ! $is_v2; then
  echo "V1 guard: not a V2 PR, nothing to check."
  exit 0
fi

if [ ${#outside[@]} -gt 0 ]; then
  echo "V1 guard: this V2 PR changes files outside v2/:"
  printf '  %s\n' "${outside[@]}"
  echo "Move V2 work under v2/. A change V1 needs is a connection point:"
  echo "its own PR, labelled v1-seam, approved by the owner (v2/PLAN.md, Part 1 §5)."
  exit 1
fi

echo "V1 guard: V2 PR stays inside v2/."
