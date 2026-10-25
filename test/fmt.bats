#!/usr/bin/env bats

# .ci/fmt runs in a fixture repo holding a copy of the real
# .ci/run-built-toolbelt and the build-dist stub test_helper/built-runner
# describes. A stub `nix` comes first on PATH; it records its arguments and
# the directory it ran in, and exits with NIX_STATUS (0 when unset). Nothing
# real is built, formatted or rewritten.

function setup() {
  load test_helper/common
  load test_helper/built-runner
  make_repo
  CALLS="${BATS_TEST_TMPDIR}/calls"
  export CALLS
  write_build_dist_stub
  cp -- "${REPO_DIR}/.ci/run-built-toolbelt" .ci/run-built-toolbelt
  mkdir "${BATS_TEST_TMPDIR}/bin"
  cat > "${BATS_TEST_TMPDIR}/bin/nix" << 'STUB'
#!/usr/bin/env bash
printf 'nix %s\n' "$*" >> "${CALLS}"
printf 'nix ran in %s\n' "$(pwd)" >> "${CALLS}"
exit "${NIX_STATUS:-0}"
STUB
  chmod +x "${BATS_TEST_TMPDIR}/bin/nix"
  # The stub must shadow the real nix for this test only.
  PATH="${BATS_TEST_TMPDIR}/bin:${PATH}"
}

@test 'runs nix fmt at the repo root, then the built runner fmt with the dogfooded formatters' {
  mkdir sub
  cd sub
  run "${REPO_DIR}/.ci/fmt"
  assert_success
  run cat "${CALLS}"
  assert_line --index 0 'nix fmt'
  assert_line --index 1 "nix ran in ${FIXTURE_REPO}"
  assert_line --index 2 --regexp '^build-dist /.+/dist$'
  assert_line --index 3 'toolbelt fmt shell'
  assert_line --index 4 'lints dir ok'
  assert_line --index 5 'formatters dir ok'
  assert_line --index 6 "ran in ${FIXTURE_REPO}"
  assert_equal "${#lines[@]}" 7
}

@test 'exits 0 when the runner does and leaves no scratch directory behind' {
  run "${REPO_DIR}/.ci/fmt"
  assert_success
  local built
  built="$(built_scratch_dir)"
  assert [ -n "${built}" ]
  assert [ ! -e "${built}" ]
}

@test 'exits with the runner status when fmt fails' {
  local status_code
  for status_code in 1 2; do
    : > "${CALLS}"
    RUNNER_STATUS="${status_code}" run "${REPO_DIR}/.ci/fmt"
    assert_failure "${status_code}"
  done
}

@test 'a failing nix fmt fails without building or running the runner' {
  NIX_STATUS=1 run "${REPO_DIR}/.ci/fmt"
  assert_failure 1
  run grep --count '^build-dist \|^toolbelt ' "${CALLS}"
  assert_output 0
}

@test 'a layout that does not build fails' {
  BUILD_STATUS=1 run "${REPO_DIR}/.ci/fmt"
  assert_failure 1
  run grep --count '^toolbelt ' "${CALLS}"
  assert_output 0
}

@test 'exits 1 with usage when given an argument, running nothing' {
  run "${REPO_DIR}/.ci/fmt" shell
  assert_failure 1
  assert_output 'fmt: expected no arguments, got 1'
  assert [ ! -e "${CALLS}" ]
}
