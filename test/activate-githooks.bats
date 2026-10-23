#!/usr/bin/env bats

function setup() {
  load test_helper/common
  make_repo
}

@test 'sets core.hooksPath to .githooks' {
  run "${REPO_DIR}/.ci/activate-githooks"
  assert_success
  run git config --local --get core.hooksPath
  assert_success
  assert_output '.githooks'
}

@test 'is silent when already set' {
  "${REPO_DIR}/.ci/activate-githooks" 2> '/dev/null'
  run "${REPO_DIR}/.ci/activate-githooks"
  assert_success
  assert_output ''
}

@test 'exits 1 when given an argument' {
  run "${REPO_DIR}/.ci/activate-githooks" unexpected
  assert_failure 1
  assert_output --partial 'expected no arguments, got 1'
}

@test 'fails outside a git repository' {
  cd "${BATS_TEST_TMPDIR}"
  run "${REPO_DIR}/.ci/activate-githooks"
  assert_failure 1
  assert_output --partial 'not a git repo'
}

@test 'repoints a foreign core.hooksPath' {
  git config --local core.hooksPath 'other-hooks'
  run "${REPO_DIR}/.ci/activate-githooks"
  assert_success
  assert_output --partial 'repointed core.hooksPath from other-hooks to .githooks'
  run git config --local --get core.hooksPath
  assert_output '.githooks'
}

@test 'fails when git cannot read core.hooksPath' {
  local -r real_git="$(command -v git)"
  mkdir "${BATS_TEST_TMPDIR}/bin"
  # shellcheck disable=SC2016 # the stand-in's own expansions must stay literal in the file it writes
  printf '#!/usr/bin/env bash\nif [[ "$*" == *"config --local --get"* ]]; then\n  exit 3\nfi\nexec %q "$@"\n' \
    "${real_git}" > "${BATS_TEST_TMPDIR}/bin/git"
  chmod +x "${BATS_TEST_TMPDIR}/bin/git"
  PATH="${BATS_TEST_TMPDIR}/bin:${PATH}" run "${REPO_DIR}/.ci/activate-githooks"
  assert_failure 3
  assert_output --partial 'could not read core.hooksPath (git exit 3)'
}
