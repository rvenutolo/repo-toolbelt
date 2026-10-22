#!/usr/bin/env bats

# Stubs stand in for commitlint and .ci/in-devshell; each prints a marker and
# the arguments it received, so no real lint or devShell runs.

function setup() {
  load test_helper/common
  make_repo
  STUB_BIN="${BATS_TEST_TMPDIR}/bin"
  mkdir "${STUB_BIN}" .ci
  printf '#!/usr/bin/env bash\nprintf "devshell-ran %%s\\n" "$*"\n' > .ci/in-devshell
  chmod +x .ci/in-devshell
  # Only the stub dir and the system dirs: the devShell's commitlint is absent.
  PATH="${STUB_BIN}:/usr/bin:/bin"
}

@test 'runs the commitlint on PATH directly' {
  printf '#!/usr/bin/env bash\nprintf "path-commitlint-ran %%s\\n" "$*"\n' > "${STUB_BIN}/commitlint"
  chmod +x "${STUB_BIN}/commitlint"
  run "${REPO_DIR}/.githooks/commit-msg" 'msg-file'
  assert_success
  assert_output 'path-commitlint-ran --edit msg-file'
}

@test 'falls back to .ci/in-devshell when commitlint is not on PATH' {
  run "${REPO_DIR}/.githooks/commit-msg" 'msg-file'
  assert_success
  assert_output 'devshell-ran commitlint --edit msg-file'
}

@test 'fails when the fallback commitlint rejects the message' {
  printf '#!/usr/bin/env bash\nexit 1\n' > .ci/in-devshell
  run "${REPO_DIR}/.githooks/commit-msg" 'msg-file'
  assert_failure 1
}
