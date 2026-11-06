#!/usr/bin/env bash

set -euo pipefail

export TERM=${TERM:-xterm-256color}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHECK="${SCRIPT_DIR}/submodules"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

FAILURES=0

pass() { printf 'ok - %s\n' "$1"; }
fail_test() { printf 'not ok - %s\n' "$1"; FAILURES=$((FAILURES + 1)); }
assert_contains() { grep -q "$2" <<<"$1"; }

new_fixture() {
  local dir="$WORK/$1"
  mkdir -p "$dir"
  git init --bare -b main "$dir/origin.git" >/dev/null
  git init -q -b main "$dir/seed"
  git -C "$dir/seed" config user.email test@example.com
  git -C "$dir/seed" config user.name Test
  echo one >"$dir/seed/file"
  git -C "$dir/seed" add file
  git -C "$dir/seed" commit -qm "initial"
  git -C "$dir/seed" remote add origin "$dir/origin.git"
  git -C "$dir/seed" push -q origin main
  git init -q -b main "$dir/parent"
  git -C "$dir/parent" config user.email test@example.com
  git -C "$dir/parent" config user.name Test
  git -C "$dir/parent" -c protocol.file.allow=always submodule add -q "$dir/origin.git" sub
  git -C "$dir/parent" commit -qm "add submodule"
  git -C "$dir/parent" tag v0.0.0
  git -C "$dir/parent" update-ref refs/remotes/origin/main HEAD
  echo "$dir"
}

test_main_mode_passes_when_synced() {
  local dir
  dir=$(new_fixture synced)
  if (cd "$dir/parent" && "$CHECK" check --mode main) >/dev/null; then
    pass "main mode passes when pin equals remote main"
  else
    fail_test "main mode passes when pin equals remote main"
  fi
}

test_main_mode_fails_when_pin_missing_on_remote() {
  local dir
  dir=$(new_fixture missing)
  echo two >"$dir/parent/sub/two"
  git -C "$dir/parent/sub" add two
  git -C "$dir/parent/sub" -c user.email=t@e -c user.name=T commit -qm "unpushed"
  git -C "$dir/parent" add sub
  git -C "$dir/parent" commit -qm "bad pin"
  local out status
  out=$(cd "$dir/parent" && "$CHECK" check --mode main 2>&1) && status=0 || status=$?
  if [ "$status" -ne 0 ] && assert_contains "$out" "is not the remote main tip"; then
    pass "main mode rejects a pin missing on the remote"
  else
    fail_test "main mode rejects a pin missing on the remote"
  fi
}

test_main_mode_fails_when_pin_is_on_unmerged_branch() {
  local dir
  dir=$(new_fixture unmerged)
  git -C "$dir/seed" checkout -qb feature
  echo two >"$dir/seed/two"
  git -C "$dir/seed" add two
  git -C "$dir/seed" commit -qm "feature"
  git -C "$dir/seed" push -q origin feature
  git -C "$dir/parent/sub" fetch -q origin
  git -C "$dir/parent/sub" checkout -q origin/feature
  git -C "$dir/parent" add sub
  git -C "$dir/parent" commit -qm "pin feature"
  local out status
  out=$(cd "$dir/parent" && "$CHECK" check --mode main 2>&1) && status=0 || status=$?
  if [ "$status" -ne 0 ] && assert_contains "$out" "is not the remote main tip"; then
    pass "main mode rejects an unmerged feature pin"
  else
    fail_test "main mode rejects an unmerged feature pin"
  fi
}

test_status_reports_synced_and_drift() {
  local dir out
  dir=$(new_fixture status)
  out=$(cd "$dir/parent" && "$CHECK" status 2>&1) || {
    fail_test "status reports synced state"
    return
  }
  if ! assert_contains "$out" "synced"; then
    fail_test "status reports synced state"
    return
  fi
  pass "status reports synced state"

  echo two >"$dir/seed/two"
  git -C "$dir/seed" add two
  git -C "$dir/seed" commit -qm "advance"
  git -C "$dir/seed" push -q origin main
  out=$(cd "$dir/parent" && "$CHECK" status 2>&1) || true
  if assert_contains "$out" "drift"; then
    pass "status reports drift"
  else
    fail_test "status reports drift"
  fi
}

test_unreachable_remote_fails() {
  local dir out status
  dir=$(new_fixture unreachable)
  git -C "$dir/parent" config -f .gitmodules submodule.sub.url "$dir/does-not-exist.git"
  out=$(cd "$dir/parent" && "$CHECK" check --mode main 2>&1) && status=0 || status=$?
  if [ "$status" -ne 0 ] && assert_contains "$out" "cannot read remote"; then
    pass "unreachable remote fails with a named error"
  else
    fail_test "unreachable remote fails with a named error"
  fi
}

for t in \
  test_main_mode_passes_when_synced \
  test_main_mode_fails_when_pin_missing_on_remote \
  test_main_mode_fails_when_pin_is_on_unmerged_branch \
  test_status_reports_synced_and_drift \
  test_unreachable_remote_fails; do
  "$t"
done

[ "$FAILURES" -eq 0 ]
