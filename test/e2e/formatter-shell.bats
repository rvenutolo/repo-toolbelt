#!/usr/bin/env bats
# bats file_tags=e2e

# The shell formatter in the image: its lint, check-shell-formatted, run the
# way the hook files run it, and `fmt shell` run with the read-write mount the
# README gives.

bats_require_minimum_version 1.5.0

# Fixture text. Nothing in a fixture is ever executed: the formatter only
# parses and prints it, so each payload is an inert marker.
readonly UNFORMATTED_SH=$'if true;then\necho PAYLOAD_RAN\nfi\n'
readonly FORMATTED_SH=$'if true; then\n  echo PAYLOAD_RAN\nfi\n'
readonly FORMATTED_SH_INDENT_4=$'if true; then\n    echo PAYLOAD_RAN\nfi\n'
readonly UNFORMATTED_SCRIPT=$'#!/usr/bin/env bash\nif true;then\necho PAYLOAD_RAN\nfi\n'
readonly FORMATTED_SCRIPT=$'#!/usr/bin/env bash\nif true; then\n  echo PAYLOAD_RAN\nfi\n'
readonly UNFORMATTED_BATS=$'@test "x" {\necho PAYLOAD_RAN\n}\n'
readonly FORMATTED_BATS=$'@test "x" {\n  echo PAYLOAD_RAN\n}\n'

function setup_file() {
  load e2e_helper
  require_image
}

function setup() {
  load e2e_helper
  make_consumer "${BATS_TEST_TMPDIR}/repo"
}

# @description Write and commit one unformatted file of each kind the
#              formatter selects: a `.sh` script, a BATS file, and an
#              extensionless executable with a bash shebang.
# @noargs
function commit_unformatted() {
  write_file bin/build.sh "${UNFORMATTED_SH}"
  write_file test/unit.bats "${UNFORMATTED_BATS}"
  write_file bin/tool "${UNFORMATTED_SCRIPT}"
  chmod 0755 bin/tool
  commit_all 'unformatted'
}

@test 'check-shell-formatted passes a formatted tree' {
  run hook_run run --hook pre-commit check-shell-formatted
  assert_success
  assert_line '== check-shell-formatted: pass =='
}

@test 'check-shell-formatted finds an unformatted script, BATS file and extensionless script' {
  commit_unformatted
  run hook_run run --hook pre-commit check-shell-formatted
  assert_failure 1
  assert_line '== check-shell-formatted: fail =='
  assert_line 'bin/build.sh: not formatted'
  assert_line 'bin/tool: not formatted'
  assert_line 'test/unit.bats: not formatted'
  assert_line '3 file(s)'
  # Check mode writes nothing.
  assert_content bin/build.sh "${UNFORMATTED_SH}"
}

@test 'fmt shell rewrites the files in place, keeping owner, mode bits and inode' {
  commit_unformatted
  local inode_before mode_before
  inode_before="$(stat --format '%i' bin/tool)"
  mode_before="$(stat --format '%a' bin/build.sh)"
  run rw_hook_run fmt shell
  assert_success
  assert_line '== shell: formatted =='
  assert_line 'bin/build.sh: formatted'
  assert_line 'bin/tool: formatted'
  assert_line 'test/unit.bats: formatted'
  assert_line 'toolbelt: 3 file(s) formatted, 0 failed, 0 errored; review and stage them'
  assert_content bin/build.sh "${FORMATTED_SH}"
  assert_content bin/tool "${FORMATTED_SCRIPT}"
  assert_content test/unit.bats "${FORMATTED_BATS}"
  assert_equal "$(stat --format '%u:%g' bin/tool)" "${HOST_USER}"
  assert_equal "$(stat --format '%a' bin/tool)" '755'
  assert_equal "$(stat --format '%a' bin/build.sh)" "${mode_before}"
  assert_equal "$(stat --format '%i' bin/tool)" "${inode_before}"
}

@test 'fmt --fail-on-change shell exits 1 after a rewrite' {
  commit_unformatted
  run rw_hook_run fmt --fail-on-change shell
  assert_failure 1
  assert_line 'bin/tool: formatted'
  assert_content bin/tool "${FORMATTED_SCRIPT}"
}

@test 'fmt --staged shell rewrites only staged files and leaves a partly staged one alone' {
  write_file bin/staged.sh "${FORMATTED_SH}"
  write_file bin/partly.sh "${FORMATTED_SH}"
  write_file bin/unstaged.sh "${FORMATTED_SH}"
  commit_all 'formatted'
  write_file bin/staged.sh "${UNFORMATTED_SH}"
  write_file bin/partly.sh "${UNFORMATTED_SH}"
  git add bin/staged.sh bin/partly.sh
  write_file bin/partly.sh "${UNFORMATTED_SH}"$'# unstaged\n'
  write_file bin/unstaged.sh "${UNFORMATTED_SH}"
  run rw_hook_run fmt --staged shell
  assert_failure 1
  assert_line 'bin/partly.sh: staged and unstaged changes; format it by hand or stage it whole'
  assert_line 'bin/staged.sh: formatted'
  assert_line 'toolbelt: 1 file(s) formatted, 0 failed, 0 errored, 1 partly staged; review and stage them'
  assert_content bin/staged.sh "${FORMATTED_SH}"
  assert_content bin/partly.sh "${UNFORMATTED_SH}"$'# unstaged\n'
  assert_content bin/unstaged.sh "${UNFORMATTED_SH}"
}

@test 'fmt shell with the read-only mount exits 2 giving the read-write line' {
  commit_unformatted
  run hook_run fmt shell
  assert_failure 2
  # shellcheck disable=SC2016 # the expansions are the message's literal text
  assert_output --partial '--volume "${root}:${root}"'
  assert_content bin/tool "${UNFORMATTED_SCRIPT}"
}

@test 'an indent setting takes effect in the lint and in fmt' {
  write_file bin/build.sh "${FORMATTED_SH}"
  printf 'check-shell-formatted:\n  settings:\n    indent: 4\n' >> .repo-toolbelt.yml
  commit_all 'indent 4'
  run hook_run run --hook pre-commit check-shell-formatted
  assert_failure 1
  assert_line 'bin/build.sh: not formatted'
  run rw_hook_run fmt shell
  assert_success
  assert_line 'bin/build.sh: formatted'
  assert_content bin/build.sh "${FORMATTED_SH_INDENT_4}"
}

@test 'an excluded file is neither reported nor rewritten' {
  write_file scripts/gen.sh "${UNFORMATTED_SH}"
  printf 'check-shell-formatted:\n  exclude:\n    - scripts/**\n' >> .repo-toolbelt.yml
  commit_all 'excluded'
  run hook_run run --hook pre-commit check-shell-formatted
  assert_success
  assert_line '== check-shell-formatted: pass =='
  run rw_hook_run fmt shell
  assert_success
  assert_line '== shell: pass =='
  assert_content scripts/gen.sh "${UNFORMATTED_SH}"
}

@test 'a file under vendor is neither reported nor rewritten' {
  write_file third_party/vendor/lib.sh "${UNFORMATTED_SH}"
  commit_all 'vendored'
  run git ls-files third_party/vendor
  assert_output 'third_party/vendor/lib.sh'
  run hook_run run --hook pre-commit check-shell-formatted
  assert_success
  assert_line '== check-shell-formatted: pass =='
  run rw_hook_run fmt shell
  assert_success
  assert_line '== shell: pass =='
  assert_content third_party/vendor/lib.sh "${UNFORMATTED_SH}"
}
