#!/usr/bin/env bats

# The devShell's shellHook, read from the flake so the test runs the hook that
# ships. It is run in a scratch repo whose .ci/activate-githooks only touches a
# marker, so a hook that runs the wrong repo's script does nothing else.

function setup_file() {
  load test_helper/common
  local system
  system="$(nix eval --extra-experimental-features 'nix-command flakes' --impure --raw --expr 'builtins.currentSystem')"
  SHELL_HOOK="$(nix eval --extra-experimental-features 'nix-command flakes' --raw \
    "${REPO_DIR}#devShells.${system}.default.shellHook")"
  export SHELL_HOOK
}

function setup() {
  load test_helper/common
  make_repo
  MARKER="${BATS_TEST_TMPDIR}/activate-githooks-ran"
  mkdir .ci
  printf '#!/usr/bin/env bash\ntouch %q\n' "${MARKER}" > .ci/activate-githooks
  chmod +x .ci/activate-githooks
}

@test 'activates the git hooks of the repo it is entered from' {
  run env --unset=IN_DEVSHELL bash -c "${SHELL_HOOK}"
  assert_success
  assert [ -e "${MARKER}" ]
}

@test 'skips activation when entered through .ci/in-devshell' {
  IN_DEVSHELL=1 run bash -c "${SHELL_HOOK}"
  assert_success
  assert_output ''
  assert [ ! -e "${MARKER}" ]
}
