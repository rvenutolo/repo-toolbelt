#!/usr/bin/env bats

# A stub commitlint prints a marker and the arguments it received and exits
# with COMMITLINT_EXIT, so no real lint runs. The suite runs inside the
# devShell, so the hook runs commitlint directly; its re-run through
# .ci/in-devshell from outside is test/self-wrap.bats' concern.

function setup() {
  load test_helper/common
  make_repo
  STUB_BIN="${BATS_TEST_TMPDIR}/bin"
  mkdir "${STUB_BIN}" .ci
  # A wrapper in the fixture that must never be used: inside the devShell the
  # hook has no reason to enter it again.
  printf '#!/usr/bin/env bash\nprintf "devshell-ran %%s\\n" "$*"\n' > .ci/in-devshell
  chmod +x .ci/in-devshell
  # shellcheck disable=SC2016 # the stub's own expansions must stay literal in the file it writes
  printf '#!/usr/bin/env bash\nprintf "path-commitlint-ran %%s\\n" "$*"\nexit "${COMMITLINT_EXIT:-0}"\n' \
    > "${STUB_BIN}/commitlint"
  chmod +x "${STUB_BIN}/commitlint"
  PATH="${STUB_BIN}:/usr/bin:/bin"
}

@test 'runs the commitlint on PATH directly' {
  run "${REPO_DIR}/.githooks/commit-msg" 'msg-file'
  assert_success
  assert_output 'path-commitlint-ran --edit msg-file'
}

@test 'fails when commitlint rejects the message' {
  COMMITLINT_EXIT=1 run "${REPO_DIR}/.githooks/commit-msg" 'msg-file'
  assert_failure 1
  assert_output --partial 'path-commitlint-ran --edit msg-file'
}

@test 'inside the devShell never hands commitlint to the wrapper' {
  rm -- "${STUB_BIN}/commitlint"
  run "${REPO_DIR}/.githooks/commit-msg" 'msg-file'
  assert_failure
  refute_output --partial 'devshell-ran'
}
