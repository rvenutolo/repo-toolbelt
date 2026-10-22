#!/usr/bin/env bats

function setup() {
  load test_helper/common
}

@test 'exits 1 with usage when given no command' {
  run env --unset=IN_DEVSHELL "${REPO_DIR}/.ci/in-devshell"
  assert_failure 1
  assert_output --partial 'usage: in-devshell <command>'
}

@test 'runs the command directly when IN_DEVSHELL is set' {
  IN_DEVSHELL=1 run "${REPO_DIR}/.ci/in-devshell" echo MARKER
  assert_success
  assert_output 'MARKER'
}

@test 'exits 1 outside a git repo' {
  cd "${BATS_TEST_TMPDIR}"
  run env --unset=IN_DEVSHELL "${REPO_DIR}/.ci/in-devshell" true
  assert_failure 1
  assert_output --partial 'not a git repo'
}

@test 'enters the devShell through nix develop' {
  make_repo
  mkdir "${BATS_TEST_TMPDIR}/bin"
  cat > "${BATS_TEST_TMPDIR}/bin/nix" << 'STUB'
#!/usr/bin/env bash
printf 'nix-ran %s\n' "$*"
printf 'IN_DEVSHELL=%s\n' "${IN_DEVSHELL}"
STUB
  chmod +x "${BATS_TEST_TMPDIR}/bin/nix"
  PATH="${BATS_TEST_TMPDIR}/bin:${PATH}"

  run env --unset=IN_DEVSHELL "${REPO_DIR}/.ci/in-devshell" echo hi

  assert_success
  local -ra keep_vars=(
    HOME TERM CI IN_DEVSHELL NIX_SSL_CERT_FILE
    GITHUB_ACTIONS GITHUB_OUTPUT GITHUB_ENV GITHUB_STEP_SUMMARY GITHUB_WORKSPACE
    GITHUB_REPOSITORY GITHUB_REF GITHUB_REF_NAME GITHUB_SHA GITHUB_EVENT_NAME GITHUB_TOKEN
    GH_TOKEN GH_REPO RUNNER_OS RUNNER_TEMP
    GOFLAGS GOPATH GOCACHE GOMODCACHE XDG_CACHE_HOME DOCKER_HOST
  )
  local keeps=''
  local keep_var
  for keep_var in "${keep_vars[@]}"; do
    keeps+=" --keep ${keep_var}"
  done
  assert_line --index 0 \
    "nix-ran develop ${FIXTURE_REPO} --ignore-environment${keeps} --command echo hi"
  assert_line --index 1 'IN_DEVSHELL=1'
}
