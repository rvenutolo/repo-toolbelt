#!/usr/bin/env bats
# bats file_tags=e2e

# The hook files' mount shape: same-path read-only mounts of the checkout and
# its git common dir, the caller's IDs, no network, stdin attached.

bats_require_minimum_version 1.5.0

function setup_file() {
  load e2e_helper
  require_image
}

function setup() {
  load e2e_helper
}

# Files a test hands to another user ID are handed back, so bats can delete
# its temp dir.
function teardown() {
  if [[ -n "${FOREIGN_DIR:-}" ]]; then
    root_run "${FOREIGN_DIR}" /bin/chown --recursive "${HOST_USER}" "${FOREIGN_DIR}"
  fi
}

# @description Copy the current fixture repo into a new directory owned by
#              uid:gid 4242:4242, which has no passwd entry in the image, and
#              export its path as FOREIGN_DIR.
# @noargs
function make_foreign_copy() {
  local -r parent="${BATS_TEST_TMPDIR}/foreign"
  mkdir "${parent}"
  cp --recursive -- "${PWD}" "${parent}/repo"
  FOREIGN_DIR="${parent}/repo"
  export FOREIGN_DIR
  root_run "${FOREIGN_DIR}" /bin/chown --recursive 4242:4242 "${FOREIGN_DIR}"
}

@test 'runs as a user ID with no passwd entry' {
  make_consumer "${BATS_TEST_TMPDIR}/repo"
  make_foreign_copy
  run shape_run "${FOREIGN_DIR}" "${FOREIGN_DIR}/.git" 4242:4242 run --hook pre-commit check-issue-numbers
  assert_success
  assert_output --partial '== check-issue-numbers: pass =='
}

@test 'a checkout owned by another user exits 2 with the ownership message' {
  make_consumer "${BATS_TEST_TMPDIR}/repo"
  make_foreign_copy
  run shape_run "${FOREIGN_DIR}" "${FOREIGN_DIR}/.git" "${HOST_USER}" run --hook pre-commit check-issue-numbers
  assert_failure 2
  assert_output --partial 'the checkout must be owned by the user running the container'
}

@test 'a checkout path with a space and a dollar sign' {
  make_consumer "${BATS_TEST_TMPDIR}/odd path \$HOME/repo"
  run hook_run run --hook pre-commit check-issue-numbers
  assert_success
  assert_output --partial '== check-issue-numbers: pass =='
}

@test 'a linked worktree' {
  make_consumer "${BATS_TEST_TMPDIR}/repo"
  git worktree add --quiet -b side "${BATS_TEST_TMPDIR}/worktree"
  cd "${BATS_TEST_TMPDIR}/worktree" || return 1
  run hook_run run --hook pre-commit check-issue-numbers check-workflow-hermetic
  assert_failure 1
  assert_output --partial '== check-issue-numbers: pass =='
  # The bash lint reads the worktree's files; finding the fixture's step
  # proves it saw the worktree and not an empty tree.
  assert_output --partial '== check-workflow-hermetic: fail =='
}

@test 'the checkout is mounted read-only' {
  make_consumer "${BATS_TEST_TMPDIR}/repo"
  # shellcheck disable=SC2034 # read by shape_run in e2e_helper.bash
  DOCKER_EXTRA=(--entrypoint /bin/touch)
  run hook_run "${PWD}/WRITTEN_MARKER"
  assert_failure
  assert_output --partial 'Read-only file system'
  assert [ ! -e "${PWD}/WRITTEN_MARKER" ]
}

@test 'the container has no network but loopback' {
  make_consumer "${BATS_TEST_TMPDIR}/repo"
  # shellcheck disable=SC2034 # read by shape_run in e2e_helper.bash
  DOCKER_EXTRA=(--entrypoint /bin/ls)
  run hook_run /sys/class/net
  assert_success
  assert_output 'lo'
}

@test 'pre-push stdin and --remote reach the runner' {
  make_consumer "${BATS_TEST_TMPDIR}/repo"
  local -r zeros='0000000000000000000000000000000000000000'
  local head
  head="$(git rev-parse HEAD)"
  # A new branch is compared with the remote's HEAD, which this remote lacks,
  # so the runner can only fail naming the remote it was given, for the ref
  # it read from stdin.
  git remote add upstream-x 'https://example.invalid/repo.git'
  run hook_run run --hook pre-push --remote upstream-x check-issue-numbers \
    <<< "refs/heads/feature ${head} refs/heads/feature ${zeros}"
  assert_failure 2
  assert_output --partial 'git remote set-head upstream-x --auto'
  assert_output --partial 'refs/heads/feature'
}

@test 'pre-push judges each ref read from stdin' {
  make_consumer "${BATS_TEST_TMPDIR}/repo"
  local -r zeros='0000000000000000000000000000000000000000'
  local first second
  first="$(git rev-parse HEAD)"
  printf 'more\n' > more.txt
  commit_all 'second'
  second="$(git rev-parse HEAD)"
  git remote add upstream-x 'https://example.invalid/repo.git'
  git update-ref refs/remotes/upstream-x/main "${first}"
  git symbolic-ref refs/remotes/upstream-x/HEAD refs/remotes/upstream-x/main
  run hook_run run --hook pre-push --remote upstream-x check-issue-numbers \
    <<< "refs/heads/main ${second} refs/heads/main ${first}
refs/heads/feature ${second} refs/heads/feature ${zeros}"
  assert_success
  assert_line '== check-issue-numbers (refs/heads/main): pass =='
  assert_line '== check-issue-numbers (refs/heads/feature): pass =='
}
