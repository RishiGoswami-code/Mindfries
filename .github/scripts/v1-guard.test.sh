#!/usr/bin/env bash
# Tests for v1-guard.sh. Run: bash .github/scripts/v1-guard.test.sh
set -uo pipefail

guard="$(dirname "$0")/v1-guard.sh"
failed=0

# expect <pass|fail> <name> <head_ref> <labels> <paths…>
expect() {
  local want="$1" name="$2" ref="$3" labels="$4"
  shift 4
  if printf '%s\n' "$@" | HEAD_REF="$ref" LABELS="$labels" bash "$guard" >/dev/null; then
    got=pass
  else
    got=fail
  fi
  if [ "$got" = "$want" ]; then
    echo "ok   $name"
  else
    echo "FAIL $name: want $want, got $got"
    failed=1
  fi
}

expect pass "clean V2 PR"                 feat/v2-cli ""       v2/cli/main.go v2/README.md
expect pass "V2 PR with .github and CLAUDE.md" feat/v2-cli "" v2/cli/main.go .github/workflows/ci.yml CLAUDE.md
expect fail "V2 PR touching candidate/"   feat/v2-cli ""       v2/cli/main.go candidate/frontend/src/app/page.tsx
expect fail "V2 branch touching only V1"  feat/candidate-v2 "" candidate/backend/main.go
expect fail "V2 PR touching supabase/"    fix/x ""             v2/db/0001.sql supabase/migrations/0028_engine.sql
expect pass "v1-seam PR"                  feat/v2-seam "v1-seam" supabase/migrations/0028_engine.sql
expect fail "v1-seam PR touching v2/"     feat/v2-seam "bug,v1-seam" supabase/migrations/0028_engine.sql v2/db/0001.sql
expect pass "plain V1 PR"                 fix/dashboard ""     candidate/frontend/src/app/page.tsx
expect pass "branch name merely contains v2" fix/kv2store "" candidate/backend/main.go
expect pass "empty diff"                  feat/v2-cli ""

exit $failed
