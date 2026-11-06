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
  git -C "$dir/parent/sub" config user.email test@example.com
  git -C "$dir/parent/sub" config user.name Test
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
  git -C "$dir/parent/sub" remote set-url origin "$dir/does-not-exist.git"
  out=$(cd "$dir/parent" && "$CHECK" check --mode main 2>&1) && status=0 || status=$?
  if [ "$status" -ne 0 ] && assert_contains "$out" "cannot read remote"; then
    pass "unreachable remote fails with a named error"
  else
    fail_test "unreachable remote fails with a named error"
  fi
}

test_branch_mode_passes_for_branch_tip() {
  local dir
  dir=$(new_fixture branchpass)
  git -C "$dir/seed" checkout -qb feature
  echo two >"$dir/seed/two"
  git -C "$dir/seed" add two
  git -C "$dir/seed" commit -qm feature
  git -C "$dir/seed" push -q origin feature
  git -C "$dir/parent/sub" fetch -q origin
  git -C "$dir/parent/sub" checkout -q origin/feature
  git -C "$dir/parent" add sub
  git -C "$dir/parent" commit -qm pin
  if (cd "$dir/parent" && "$CHECK" check --mode branch --branch feature) >/dev/null; then
    pass "branch mode accepts the same-named branch tip"
  else
    fail_test "branch mode accepts the same-named branch tip"
  fi
}

test_branch_mode_fails_when_pin_is_behind() {
  local dir out status
  dir=$(new_fixture branchbehind)
  git -C "$dir/seed" checkout -qb feature
  echo two >"$dir/seed/two"
  git -C "$dir/seed" add two
  git -C "$dir/seed" commit -qm one
  git -C "$dir/seed" push -q origin feature
  git -C "$dir/parent/sub" fetch -q origin
  git -C "$dir/parent/sub" checkout -q feature
  git -C "$dir/parent" add sub
  git -C "$dir/parent" commit -qm pin
  echo three >"$dir/seed/three"
  git -C "$dir/seed" add three
  git -C "$dir/seed" commit -qm two
  git -C "$dir/seed" push -q origin feature
  out=$(cd "$dir/parent" && "$CHECK" check --mode branch --branch feature 2>&1) && status=0 || status=$?
  if [ "$status" -ne 0 ] && assert_contains "$out" "is not the tip of remote branch 'feature'"; then
    pass "branch mode rejects a stale pin"
  else
    fail_test "branch mode rejects a stale pin"
  fi
}

test_branch_mode_ignores_unchanged_module() {
  local dir
  dir=$(new_fixture branchskip)
  if (cd "$dir/parent" && "$CHECK" check --mode branch --branch missing-branch) >/dev/null; then
    pass "branch mode skips unchanged submodules"
  else
    fail_test "branch mode skips unchanged submodules"
  fi
}

test_branch_mode_requires_base_ref() {
  local dir out status
  dir=$(new_fixture nobase)
  git -C "$dir/parent" update-ref -d refs/remotes/origin/main 2>/dev/null || true
  out=$(cd "$dir/parent" && "$CHECK" check --mode branch --branch feature 2>&1) && status=0 || status=$?
  if [ "$status" -ne 0 ] && assert_contains "$out" "base ref"; then
    pass "branch mode names a missing base ref"
  else
    fail_test "branch mode names a missing base ref"
  fi
}

test_sync_merges_pushes_and_updates_pin() {
  local dir old_pin
  dir=$(new_fixture syncok)
  git -C "$dir/seed" checkout -qb feature
  echo two >"$dir/seed/two"
  git -C "$dir/seed" add two
  git -C "$dir/seed" commit -qm one
  git -C "$dir/seed" push -q origin feature
  git -C "$dir/parent/sub" fetch -q origin
  git -C "$dir/parent/sub" checkout -q feature
  git -C "$dir/parent" add sub
  git -C "$dir/parent" commit -qm pin
  old_pin=$(git -C "$dir/parent" rev-parse :sub)
  echo three >"$dir/seed/three"
  git -C "$dir/seed" add three
  git -C "$dir/seed" commit -qm two
  git -C "$dir/seed" push -q origin feature
  (cd "$dir/parent" && "$CHECK" sync --branch feature) >/dev/null || {
    fail_test "sync merges, pushes and stages the new pin"
    return
  }
  local main_sha indexed
  main_sha=$(git -C "$dir/origin.git" rev-parse refs/heads/main)
  indexed=$(git -C "$dir/parent" rev-parse :sub)
  if [ "$main_sha" = "$indexed" ] && [ "$indexed" != "$old_pin" ]; then
    pass "sync merges, pushes and stages the new pin"
  else
    fail_test "sync merges, pushes and stages the new pin"
  fi
}

test_sync_aborts_on_dirty_submodule() {
  local dir out status
  dir=$(new_fixture syncdirty)
  echo dirty >"$dir/parent/sub/dirty"
  out=$(cd "$dir/parent" && "$CHECK" sync --branch feature 2>&1) && status=0 || status=$?
  if [ "$status" -ne 0 ] && assert_contains "$out" "uncommitted changes"; then
    pass "sync refuses a dirty submodule"
  else
    fail_test "sync refuses a dirty submodule"
  fi
}

test_sync_requires_initialized_submodule() {
  local dir out status
  dir=$(new_fixture syncuninit)
  git -C "$dir/parent" submodule deinit -f -- sub >/dev/null 2>&1
  rm -rf "$dir/parent/sub"
  out=$(cd "$dir/parent" && "$CHECK" sync --branch feature 2>&1) && status=0 || status=$?
  if [ "$status" -ne 0 ] && assert_contains "$out" "not initialized"; then
    pass "sync names an uninitialized submodule"
  else
    fail_test "sync names an uninitialized submodule"
  fi
}

test_sync_commit_creates_parent_commit() {
  local dir subject
  dir=$(new_fixture synccommit)
  git -C "$dir/seed" checkout -qb feature
  echo two >"$dir/seed/two"
  git -C "$dir/seed" add two
  git -C "$dir/seed" commit -qm one
  git -C "$dir/seed" push -q origin feature
  git -C "$dir/parent/sub" fetch -q origin
  git -C "$dir/parent/sub" checkout -q feature
  git -C "$dir/parent" add sub
  git -C "$dir/parent" commit -qm pin
  echo three >"$dir/seed/three"
  git -C "$dir/seed" add three
  git -C "$dir/seed" commit -qm two
  git -C "$dir/seed" push -q origin feature
  (cd "$dir/parent" && "$CHECK" sync --branch feature --commit) >/dev/null || {
    fail_test "--commit creates a parent pin commit"
    return
  }
  subject=$(git -C "$dir/parent" log -1 --format=%s)
  if assert_contains "$subject" "sync submodule pins"; then
    pass "--commit creates a parent pin commit"
  else
    fail_test "--commit creates a parent pin commit"
  fi
}

test_pre_push_fails_on_unpushed_commit() {
  local dir line out status
  dir=$(new_fixture prepush)
  echo two >"$dir/parent/sub/two"
  git -C "$dir/parent/sub" add two
  git -C "$dir/parent/sub" -c user.email=t@e -c user.name=T commit -qm local
  line="refs/heads/main $(git -C "$dir/parent" rev-parse HEAD) refs/heads/main 0000000000000000000000000000000000000000"
  out=$(cd "$dir/parent" && printf '%s\n' "$line" | "$CHECK" check --pre-push 2>&1) && status=0 || status=$?
  if [ "$status" -ne 0 ] && assert_contains "$out" "unpushed commit"; then
    pass "pre-push rejects unpushed submodule commits"
  else
    fail_test "pre-push rejects unpushed submodule commits"
  fi
}

test_pre_push_passes_after_push() {
  local dir line
  dir=$(new_fixture prepushok)
  echo two >"$dir/parent/sub/two"
  git -C "$dir/parent/sub" add two
  git -C "$dir/parent/sub" -c user.email=t@e -c user.name=T commit -qm local
  git -C "$dir/parent/sub" push -q origin HEAD:main
  git -C "$dir/parent/sub" fetch -q origin
  git -C "$dir/parent" add sub
  git -C "$dir/parent" commit -qm pin
  line="refs/heads/main $(git -C "$dir/parent" rev-parse HEAD) refs/heads/main 0000000000000000000000000000000000000000"
  if (cd "$dir/parent" && printf '%s\n' "$line" | "$CHECK" check --pre-push) >/dev/null; then
    pass "pre-push passes when the module is pushed and merged"
  else
    fail_test "pre-push passes when the module is pushed and merged"
  fi
}

test_pre_push_handles_detached_submodule() {
  local dir line
  dir=$(new_fixture prepushdetached)
  git -C "$dir/parent/sub" checkout -q --detach
  line="refs/heads/main $(git -C "$dir/parent" rev-parse HEAD) refs/heads/main 0000000000000000000000000000000000000000"
  if (cd "$dir/parent" && printf '%s\n' "$line" | "$CHECK" check --pre-push) >/dev/null; then
    pass "pre-push handles a detached submodule"
  else
    fail_test "pre-push handles a detached submodule"
  fi
}

test_runs_without_tags_or_term() {
  local dir
  dir=$(new_fixture notags)
  git -C "$dir/parent" tag -d v0.0.0 >/dev/null
  if (cd "$dir/parent" && env -u TERM "$CHECK" check --mode main) >/dev/null; then
    pass "check runs without tags or TERM"
  else
    fail_test "check runs without tags or TERM"
  fi
}

test_make_targets() {
  local root out
  root="$(cd "$SCRIPT_DIR/.." && pwd)"
  if ! out=$(make -C "$root" -n submodules-check 2>&1); then
    fail_test "make submodules-check target exists"
    return
  fi
  if ! assert_contains "$out" "scripts/submodules check --mode main"; then
    fail_test "make submodules-check runs the main check"
    return
  fi
  if ! out=$(make -C "$root" -n submodules-sync 2>&1) ||
    ! assert_contains "$out" "scripts/submodules sync"; then
    fail_test "make submodules-sync target exists"
    return
  fi
  pass "make submodule targets exist"
}

test_missing_url_fails() {
  local dir out status
  dir=$(new_fixture nourl)
  git -C "$dir/parent" config -f .gitmodules --unset submodule.sub.url
  out=$(cd "$dir/parent" && "$CHECK" check --mode main 2>&1) && status=0 || status=$?
  if [ "$status" -ne 0 ] && assert_contains "$out" "missing URL"; then
    pass "missing .gitmodules URL fails the check"
  else
    fail_test "missing .gitmodules URL fails the check"
  fi
}

test_pre_push_checks_pushed_ref_not_head() {
  local dir line bad_sha out status
  dir=$(new_fixture prepushref)
  git -C "$dir/seed" checkout -qb feature
  echo two >"$dir/seed/two"
  git -C "$dir/seed" add two
  git -C "$dir/seed" commit -qm feature
  git -C "$dir/seed" push -q origin feature
  git -C "$dir/parent" checkout -qb bad
  git -C "$dir/parent/sub" fetch -q origin
  git -C "$dir/parent/sub" checkout -q origin/feature
  git -C "$dir/parent" add sub
  git -C "$dir/parent" commit -qm "bad pin"
  bad_sha=$(git -C "$dir/parent" rev-parse HEAD)
  git -C "$dir/parent/sub" checkout -q main
  git -C "$dir/parent" checkout -q main
  line="refs/heads/bad ${bad_sha} refs/heads/bad 0000000000000000000000000000000000000000"
  out=$(cd "$dir/parent" && printf '%s\n' "$line" | "$CHECK" check --pre-push 2>&1) && status=0 || status=$?
  if [ "$status" -ne 0 ]; then
    pass "pre-push checks the pushed ref, not HEAD"
  else
    fail_test "pre-push checks the pushed ref, not HEAD"
  fi
}

test_branch_mode_accepts_synced_merge_commit() {
  local dir
  dir=$(new_fixture mergesync)
  git -C "$dir/seed" checkout -qb feature
  echo two >"$dir/seed/two"
  git -C "$dir/seed" add two
  git -C "$dir/seed" commit -qm one
  git -C "$dir/seed" push -q origin feature
  git -C "$dir/parent/sub" fetch -q origin
  git -C "$dir/parent/sub" checkout -q feature
  git -C "$dir/parent" add sub
  git -C "$dir/parent" commit -qm pin

  git -C "$dir/seed" checkout -q main
  echo main >"$dir/seed/main"
  git -C "$dir/seed" add main
  git -C "$dir/seed" commit -qm main
  git -C "$dir/seed" push -q origin main
  git -C "$dir/seed" checkout -q feature
  echo three >"$dir/seed/three"
  git -C "$dir/seed" add three
  git -C "$dir/seed" commit -qm two
  git -C "$dir/seed" push -q origin feature

  (cd "$dir/parent" && "$CHECK" sync --branch feature --commit) >/dev/null || {
    fail_test "branch mode accepts a synced merge commit"
    return
  }
  if (cd "$dir/parent" && "$CHECK" check --mode branch --branch feature) >/dev/null; then
    pass "branch mode accepts a synced merge commit"
  else
    fail_test "branch mode accepts a synced merge commit"
  fi
}

test_sync_commit_ignores_unrelated_staged_files() {
  local dir
  dir=$(new_fixture syncstage)
  git -C "$dir/seed" checkout -qb feature
  echo two >"$dir/seed/two"
  git -C "$dir/seed" add two
  git -C "$dir/seed" commit -qm one
  git -C "$dir/seed" push -q origin feature
  git -C "$dir/parent/sub" fetch -q origin
  git -C "$dir/parent/sub" checkout -q feature
  git -C "$dir/parent" add sub
  git -C "$dir/parent" commit -qm pin
  echo three >"$dir/seed/three"
  git -C "$dir/seed" add three
  git -C "$dir/seed" commit -qm two
  git -C "$dir/seed" push -q origin feature

  echo unrelated >"$dir/parent/staged.txt"
  git -C "$dir/parent" add staged.txt
  (cd "$dir/parent" && "$CHECK" sync --branch feature --commit) >/dev/null || {
    fail_test "sync commit ignores unrelated staged files"
    return
  }
  if git -C "$dir/parent" show --name-only --format= HEAD | grep -qx "staged.txt"; then
    fail_test "sync commit ignores unrelated staged files"
    return
  fi
  if git -C "$dir/parent" diff --cached --name-only | grep -qx "staged.txt"; then
    pass "sync commit ignores unrelated staged files"
  else
    fail_test "sync commit ignores unrelated staged files"
  fi
}

test_check_uses_gitmodules_url_when_uninitialized() {
  local dir
  dir=$(new_fixture uninitialized)
  git -C "$dir/parent" submodule deinit -f -- sub >/dev/null 2>&1
  rm -rf "$dir/parent/sub"
  mkdir -p "$dir/parent/sub"
  git -C "$dir/parent" remote add origin "$dir/parent-does-not-exist.git"
  if (cd "$dir/parent" && "$CHECK" check --mode main) >/dev/null; then
    pass "check uses the .gitmodules URL for an uninitialized module"
  else
    fail_test "check uses the .gitmodules URL for an uninitialized module"
  fi
}

for t in \
  test_main_mode_passes_when_synced \
  test_main_mode_fails_when_pin_missing_on_remote \
  test_main_mode_fails_when_pin_is_on_unmerged_branch \
  test_status_reports_synced_and_drift \
  test_unreachable_remote_fails \
  test_branch_mode_passes_for_branch_tip \
  test_branch_mode_fails_when_pin_is_behind \
  test_branch_mode_ignores_unchanged_module \
  test_branch_mode_requires_base_ref \
  test_sync_merges_pushes_and_updates_pin \
  test_sync_aborts_on_dirty_submodule \
  test_sync_requires_initialized_submodule \
  test_sync_commit_creates_parent_commit \
  test_pre_push_fails_on_unpushed_commit \
  test_pre_push_passes_after_push \
  test_pre_push_handles_detached_submodule \
  test_runs_without_tags_or_term \
  test_make_targets \
  test_missing_url_fails \
  test_check_uses_gitmodules_url_when_uninitialized \
  test_pre_push_checks_pushed_ref_not_head \
  test_branch_mode_accepts_synced_merge_commit \
  test_sync_commit_ignores_unrelated_staged_files; do
  "$t"
done

[ "$FAILURES" -eq 0 ]
