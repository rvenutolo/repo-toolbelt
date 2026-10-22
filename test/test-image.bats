#!/usr/bin/env bats

# test-image is driven with stand-ins for sysctl, nix, docker and bats on
# PATH: each records its arguments, so nothing is built, no container starts
# and the e2e suite does not run. The real image is exercised by the e2e suite
# itself.

bats_require_minimum_version 1.5.0

function setup() {
  load test_helper/common
  CALLS="${BATS_TEST_TMPDIR}/calls"
  export CALLS
  local -r bin="${BATS_TEST_TMPDIR}/bin"
  mkdir "${bin}"
  # What each stand-in answers with, set per test.
  export IP_FORWARD=1
  export LOAD_REPORT='Loaded image: ghcr.io/rvenutolo/repo-toolbelt:0.0.0-abc1234'
  export BATS_STATUS=0
  # nix prints a stand-in image stream: a script that writes a marker, so the
  # docker stand-in can show it received the stream on stdin.
  STREAM="${BATS_TEST_TMPDIR}/stream"
  printf '#!/usr/bin/env bash\nprintf "IMAGE_STREAM\\n"\n' > "${STREAM}"
  chmod +x "${STREAM}"
  export STREAM
  cat > "${bin}/sysctl" << 'STUB'
#!/usr/bin/env bash
printf 'sysctl %s\n' "$*" >> "${CALLS}"
printf '%s\n' "${IP_FORWARD}"
STUB
  cat > "${bin}/nix" << 'STUB'
#!/usr/bin/env bash
printf 'nix %s\n' "$*" >> "${CALLS}"
printf '%s\n' "${STREAM}"
STUB
  cat > "${bin}/docker" << 'STUB'
#!/usr/bin/env bash
printf 'docker %s\n' "$*" >> "${CALLS}"
if [[ "$1" == 'load' ]]; then
  printf 'docker stdin %s\n' "$(cat)" >> "${CALLS}"
  printf '%s\n' "${LOAD_REPORT}"
fi
STUB
  cat > "${bin}/bats" << 'STUB'
#!/usr/bin/env bash
printf 'bats %s image=%s version=%s cwd=%s\n' "$*" "${TOOLBELT_TEST_IMAGE}" "${TOOLBELT_TEST_VERSION}" "${PWD}" >> "${CALLS}"
exit "${BATS_STATUS}"
STUB
  chmod +x "${bin}"/*
  PATH="${bin}:${PATH}"
  # Run from a subdirectory: the script must still build and test the repo.
  cd "${REPO_DIR}/test" || return 1
}

@test 'builds, loads and tags the image, then runs the e2e suite against it' {
  run "${REPO_DIR}/.ci/test-image"
  assert_success
  run cat "${CALLS}"
  assert_line --index 0 'sysctl --values net.ipv4.ip_forward'
  assert_line --index 1 'nix build --no-link --print-out-paths .#image'
  assert_line --index 2 'docker load'
  assert_line --index 3 'docker stdin IMAGE_STREAM'
  assert_line --index 4 'docker tag ghcr.io/rvenutolo/repo-toolbelt:0.0.0-abc1234 repo-toolbelt:test'
  assert_line --index 5 "bats test/e2e image=repo-toolbelt:test version=0.0.0-abc1234 cwd=${REPO_DIR}"
}

@test 'arguments go to bats in place of the whole suite' {
  run "${REPO_DIR}/.ci/test-image" --filter 'linked' test/e2e/mount.bats
  assert_success
  run grep '^bats ' "${CALLS}"
  assert_output --partial 'bats --filter linked test/e2e/mount.bats image=repo-toolbelt:test'
}

@test 'IP forwarding off fails before anything is built or run' {
  IP_FORWARD=0
  run "${REPO_DIR}/.ci/test-image"
  assert_failure 1
  assert_output --partial 'net.ipv4.ip_forward is 0, not 1'
  run cat "${CALLS}"
  assert_output 'sysctl --values net.ipv4.ip_forward'
}

@test 'a docker load that names no image fails before the suite runs' {
  LOAD_REPORT='something else'
  run "${REPO_DIR}/.ci/test-image"
  assert_failure 1
  assert_output --partial 'docker load named no image'
  run grep --count '^bats \|^docker tag ' "${CALLS}"
  assert_output '0'
}

@test "the suite's failure is the script's exit status" {
  BATS_STATUS=3
  run "${REPO_DIR}/.ci/test-image"
  assert_failure 3
}
