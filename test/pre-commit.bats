#!/usr/bin/env bats

# A copy of the pre-commit hook runs in a fixture repo holding a copy of the
# real .ci/run-built-toolbelt and the build-dist stub test_helper/built-runner
# describes, so the whole chain runs with nothing real built or checked. The
# scripts take their repo root from their own path, so the copies sit at the
# same depth in the fixture as in the real repo. The suite runs inside the
# devShell, so the hook runs the chain directly; its re-run through
# .ci/in-devshell from outside is test/self-wrap.bats' concern.

readonly FIX_HINT='pre-commit: formatting check failed; run ./.ci/fmt and stage the result'
readonly COULD_NOT_RUN='pre-commit: the formatting check could not run; see the error above'

function setup() {
  load test_helper/common
  load test_helper/built-runner
  make_repo
  CALLS="${BATS_TEST_TMPDIR}/calls"
  export CALLS
  write_build_dist_stub
  cp -- "${REPO_DIR}/.ci/run-built-toolbelt" .ci/run-built-toolbelt
  cp -- "${REPO_DIR}/.ci/dogfood-formatters.bash" .ci/dogfood-formatters.bash
  mkdir --parents .githooks
  cp -- "${REPO_DIR}/.githooks/pre-commit" .githooks/pre-commit
}

@test 'checks the dogfooded formatters with the built runner, directly inside the devShell' {
  run "${FIXTURE_REPO}/.githooks/pre-commit"
  assert_success
  assert_output --partial 'pre-commit: formatting check passed'
  run cat "${CALLS}"
  assert_line --index 0 --regexp '^build-dist /.+/dist$'
  assert_line --index 1 'toolbelt run --hook pre-commit check-shell-formatted check-go-formatted'
  assert_line --index 2 'lints dir ok'
  assert_line --index 3 'formatters dir ok'
  assert_line --index 4 "ran in ${FIXTURE_REPO}"
  assert_equal "${#lines[@]}" 5
}

@test 'runs from a subdirectory of the checkout' {
  mkdir sub
  cd sub
  run "${FIXTURE_REPO}/.githooks/pre-commit"
  assert_success
  run grep --count '^toolbelt run --hook pre-commit ' "${CALLS}"
  assert_output 1
}

@test 'hands the runner the index git is committing' {
  GIT_INDEX_FILE="${BATS_TEST_TMPDIR}/index.lock" run "${FIXTURE_REPO}/.githooks/pre-commit"
  assert_success
  run grep '^index ' "${CALLS}"
  assert_output "index ${BATS_TEST_TMPDIR}/index.lock"
}

@test 'never rewrites: the runner is never asked to fmt' {
  run "${FIXTURE_REPO}/.githooks/pre-commit"
  assert_success
  run grep --count '^toolbelt fmt' "${CALLS}"
  assert_output 0
}

@test 'a finding fails the commit and names the fix' {
  RUNNER_STATUS=1 run "${FIXTURE_REPO}/.githooks/pre-commit"
  assert_failure 1
  assert_output --partial "${FIX_HINT}"
  refute_output --partial "${COULD_NOT_RUN}"
  refute_output --partial 'ERROR: line'
}

@test 'a runner error fails the commit and says the check could not run' {
  RUNNER_STATUS=2 run "${FIXTURE_REPO}/.githooks/pre-commit"
  assert_failure 1
  assert_output --partial "${COULD_NOT_RUN}"
  refute_output --partial "${FIX_HINT}"
  refute_output --partial 'ERROR: line'
}

@test 'leaves no scratch directory behind' {
  RUNNER_STATUS=1 run "${FIXTURE_REPO}/.githooks/pre-commit"
  local built
  built="$(built_scratch_dir)"
  assert [ -n "${built}" ]
  assert [ ! -e "${built}" ]
}

@test 'a layout that does not build fails the commit and says the check could not run' {
  BUILD_STATUS=1 run "${FIXTURE_REPO}/.githooks/pre-commit"
  assert_failure 1
  assert_output --partial "${COULD_NOT_RUN}"
  refute_output --partial "${FIX_HINT}"
  run grep --count '^toolbelt ' "${CALLS}"
  assert_output 0
}
