#!/usr/bin/env bats
# bats file_tags=e2e

# A Git LFS consumer: `git lfs install --local` sets a required clean filter,
# which every working-tree diff and status runs on a file whose stat changed.
# The image carries git-lfs, and the checkout's git dir is mounted read-only.

bats_require_minimum_version 1.5.0

# Fixture text. Nothing in a fixture is ever executed.
readonly ASSET=$'PAYLOAD_RAN marker\n'
readonly UNFORMATTED_SH=$'if true;then\necho PAYLOAD_RAN\nfi\n'
readonly FORMATTED_SH=$'if true; then\n  echo PAYLOAD_RAN\nfi\n'

function setup_file() {
  load e2e_helper
  require_image
}

# A consumer with one LFS-tracked file, committed, then touched: same content,
# a later mtime, so git must run the clean filter to know it is unchanged.
function setup() {
  load e2e_helper
  make_consumer "${BATS_TEST_TMPDIR}/repo"
  git lfs install --local > /dev/null
  git lfs track '*.bin' > /dev/null
  write_file assets/blob.bin "${ASSET}"
  commit_all 'lfs asset'
  touch --date='+1 minute' assets/blob.bin
  # diff-files compares stat data only and runs no filter.
  run git diff-files --name-only
  assert_output 'assets/blob.bin'
}

@test 'pre-commit with a touched LFS file passes and warns of nothing' {
  run --separate-stderr hook_run run --hook pre-commit check-issue-numbers
  assert_success
  assert_output --partial '== check-issue-numbers: pass =='
  refute_stderr --partial 'unstaged changes'
}

@test 'a repo-local lfs.storage does not move LFS objects onto the read-only mount' {
  git config lfs.storage lfs-local
  run --separate-stderr hook_run run --hook pre-commit check-issue-numbers
  assert_success
  refute_stderr --partial 'unstaged changes'
}

@test 'pre-commit with an edited LFS file warns of the unstaged change' {
  write_file assets/blob.bin $'PAYLOAD_RAN edited\n'
  run --separate-stderr hook_run run --hook pre-commit check-issue-numbers
  assert_success
  assert_stderr --partial 'toolbelt: 1 tracked file(s) have unstaged changes'
}

@test 'pre-push with a touched LFS file passes and warns of nothing' {
  local head parent
  head="$(git rev-parse HEAD)"
  parent="$(git rev-parse HEAD^)"
  run --separate-stderr hook_run run --hook pre-push --remote origin check-issue-numbers \
    <<< "refs/heads/main ${head} refs/heads/main ${parent}"
  assert_success
  assert_output --partial '== check-issue-numbers: pass =='
  refute_stderr --partial 'uncommitted changes'
}

@test 'every Go lint passes with a touched LFS file' {
  local -r names_file="${BATS_TEST_TMPDIR}/names"
  manifest_lints go > "${names_file}"
  local -a go_lints
  mapfile -t go_lints < "${names_file}"
  run hook_run run --hook pre-commit "${go_lints[@]}"
  assert_success
  assert_line "toolbelt: ${#go_lints[@]} passed, 0 failed, 0 errored"
}

@test 'fmt --staged with a touched LFS file formats the staged file' {
  write_file bin/staged.sh "${UNFORMATTED_SH}"
  git add bin/staged.sh
  run rw_hook_run fmt --staged shell
  assert_success
  assert_line 'bin/staged.sh: formatted'
  assert_content bin/staged.sh "${FORMATTED_SH}"
}
