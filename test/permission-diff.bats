#!/usr/bin/env bats

bats_require_minimum_version 1.5.0

# The backlog skills' permission check: it prints the requested allow rules no
# settings file lists verbatim, and the effective defaultMode on stderr. Each
# test gets its own CLAUDE_CONFIG_DIR and repo, so the host's settings never
# reach it.

function setup() {
  load test_helper/common
  PERMISSION_DIFF="${REPO_DIR}/.claude/skills/work-backlog/permission-diff"
  CLAUDE_CONFIG_DIR="${BATS_TEST_TMPDIR}/config"
  export CLAUDE_CONFIG_DIR
  mkdir "${CLAUDE_CONFIG_DIR}"
  make_repo
  mkdir .claude
}

# @description Write a settings file allowing the given rules.
# @arg $1 path the settings file
# @arg $@ the rules to allow, after the path
function allow() {
  local -r path="$1"
  shift
  jq --null-input '{permissions: {allow: $ARGS.positional}}' --args "$@" > "${path}"
}

@test 'exits 2 with usage when given no arguments' {
  run "${PERMISSION_DIFF}"
  assert_failure 2
  assert_output --partial 'usage: permission-diff'
}

@test 'exits 2 when CLAUDE_CONFIG_DIR is not set' {
  unset CLAUDE_CONFIG_DIR
  run "${PERMISSION_DIFF}" 'Bash(ls *)'
  assert_failure 2
  assert_output --partial 'CLAUDE_CONFIG_DIR is not set'
}

@test 'prints every rule when no settings file exists' {
  run --separate-stderr "${PERMISSION_DIFF}" 'Bash(ls *)' 'Bash(gh *)'
  assert_success
  assert_equal "${output}" "$(printf 'Bash(ls *)\nBash(gh *)')"
}

@test 'prints nothing when the user settings allow every rule' {
  allow "${CLAUDE_CONFIG_DIR}/settings.json" 'Bash(ls *)' 'Bash(gh *)'
  run --separate-stderr "${PERMISSION_DIFF}" 'Bash(ls *)' 'Bash(gh *)'
  assert_success
  assert_output ''
}

@test 'reads all four settings files' {
  allow "${CLAUDE_CONFIG_DIR}/settings.json" 'Bash(a)'
  allow "${CLAUDE_CONFIG_DIR}/settings.local.json" 'Bash(b)'
  allow "${FIXTURE_REPO}/.claude/settings.json" 'Bash(c)'
  allow "${FIXTURE_REPO}/.claude/settings.local.json" 'Bash(d)'
  run --separate-stderr "${PERMISSION_DIFF}" 'Bash(a)' 'Bash(b)' 'Bash(c)' 'Bash(d)' 'Bash(e)'
  assert_success
  assert_output 'Bash(e)'
}

@test 'finds the repo settings from a subdirectory' {
  allow "${FIXTURE_REPO}/.claude/settings.json" 'Bash(c)'
  mkdir sub
  cd sub
  run --separate-stderr "${PERMISSION_DIFF}" 'Bash(c)'
  assert_output ''
}

@test 'counts a rule present only when listed verbatim, and names the wider rule that covers it' {
  allow "${CLAUDE_CONFIG_DIR}/settings.json" 'Bash(git *)'
  run --separate-stderr "${PERMISSION_DIFF}" 'Bash(git fetch *)'
  assert_output "$(printf 'Bash(git fetch *)\n  covered by: Bash(git *)')"
}

@test 'prints no covered-by line when no wider rule covers a missing rule' {
  allow "${CLAUDE_CONFIG_DIR}/settings.json" 'Bash(git worktree list)' 'Bash(gi *)' 'Bash(gitk *)'
  run --separate-stderr "${PERMISSION_DIFF}" 'Bash(git worktree list *)' 'Edit(//x/**)'
  assert_output "$(printf 'Bash(git worktree list *)\nEdit(//x/**)')"
}

@test 'names the most specific wider rule when several cover a missing rule' {
  allow "${CLAUDE_CONFIG_DIR}/settings.json" 'Bash(git *)' 'Bash(git worktree *)'
  run --separate-stderr "${PERMISSION_DIFF}" 'Bash(git worktree list *)'
  assert_output "$(printf 'Bash(git worktree list *)\n  covered by: Bash(git worktree *)')"
}

@test 'names a wider rule from any settings file' {
  allow "${FIXTURE_REPO}/.claude/settings.local.json" 'Bash(mkdir *)'
  run --separate-stderr "${PERMISSION_DIFF}" 'Bash(mkdir --parents *)'
  assert_output "$(printf 'Bash(mkdir --parents *)\n  covered by: Bash(mkdir *)')"
}

@test 'reads the rules one per line from stdin for -' {
  allow "${CLAUDE_CONFIG_DIR}/settings.json" 'Bash(a)'
  # shellcheck disable=SC2016 # $1 expands in the inner bash, which gets the script path as its first argument
  run --separate-stderr bash -c 'printf "Bash(a)\n\nBash(b)" | "$1" -' _ "${PERMISSION_DIFF}"
  assert_success
  assert_output 'Bash(b)'
}

@test 'exits 2 when - comes with other arguments' {
  run "${PERMISSION_DIFF}" - 'Bash(a)'
  assert_failure 2
}

@test 'reports defaultMode unset when no file sets it' {
  run --separate-stderr "${PERMISSION_DIFF}" 'Bash(a)'
  assert_equal "${stderr}" 'defaultMode: unset'
}

@test 'reports the defaultMode of the most specific file that sets it' {
  jq --null-input '{permissions: {defaultMode: "acceptEdits"}}' > "${CLAUDE_CONFIG_DIR}/settings.json"
  jq --null-input '{permissions: {defaultMode: "bypassPermissions"}}' > "${FIXTURE_REPO}/.claude/settings.json"
  allow "${FIXTURE_REPO}/.claude/settings.local.json" 'Bash(a)'
  run --separate-stderr "${PERMISSION_DIFF}" 'Bash(a)'
  assert_equal "${stderr}" 'defaultMode: bypassPermissions'
}

@test 'exits 2 when a settings file is not valid JSON' {
  printf '{not json' > "${CLAUDE_CONFIG_DIR}/settings.json"
  run "${PERMISSION_DIFF}" 'Bash(a)'
  assert_failure 2
  assert_output --partial 'is not valid JSON'
}

@test 'prints the installed superpowers directory from the plugin registry' {
  mkdir --parents "${CLAUDE_CONFIG_DIR}/plugins" "${BATS_TEST_TMPDIR}/sp/7.0.0"
  jq --null-input --arg dir "${BATS_TEST_TMPDIR}/sp/7.0.0" \
    '{plugins: {"other@m": [{installPath: "/x"}], "superpowers@m": [{installPath: $dir}]}}' \
    > "${CLAUDE_CONFIG_DIR}/plugins/installed_plugins.json"
  run "${PERMISSION_DIFF}" --superpowers-dir
  assert_success
  assert_output "${BATS_TEST_TMPDIR}/sp/7.0.0"
}

@test 'exits 1 when no superpowers plugin is installed' {
  run "${PERMISSION_DIFF}" --superpowers-dir
  assert_failure 1
  assert_output --partial 'no installed superpowers plugin'
}

@test 'exits 2 when --superpowers-dir comes with other arguments' {
  run "${PERMISSION_DIFF}" --superpowers-dir 'Bash(a)'
  assert_failure 2
}
