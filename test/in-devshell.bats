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
  local -r keeps='--keep HOME --keep TERM --keep CI --keep IN_DEVSHELL --keep NIX_SSL_CERT_FILE'
  local -r github_keeps='--keep GITHUB_ACTIONS --keep GITHUB_OUTPUT --keep GITHUB_ENV --keep GITHUB_STEP_SUMMARY --keep GITHUB_WORKSPACE --keep GITHUB_REPOSITORY --keep GITHUB_REF --keep GITHUB_REF_NAME --keep GITHUB_SHA --keep GITHUB_EVENT_NAME --keep GITHUB_TOKEN'
  local -r other_keeps='--keep GH_TOKEN --keep GH_REPO --keep RUNNER_OS --keep RUNNER_TEMP --keep GOFLAGS --keep GOPATH --keep GOCACHE --keep GOMODCACHE --keep XDG_CACHE_HOME --keep DOCKER_HOST'
  assert_line --index 0 "nix-ran develop ${FIXTURE_REPO} --ignore-environment ${keeps} ${github_keeps} ${other_keeps} --command echo hi"
  assert_line --index 1 'IN_DEVSHELL=1'
}
