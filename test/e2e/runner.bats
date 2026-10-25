#!/usr/bin/env bats
# bats file_tags=e2e

# The image's entrypoint and every lint it ships, run the way a hook file runs
# them.

bats_require_minimum_version 1.5.0

function setup_file() {
  load e2e_helper
  require_image
}

function setup() {
  load e2e_helper
}

@test 'no arguments prints usage and exits 2' {
  run --separate-stderr docker run --rm --network=none "${TOOLBELT_TEST_IMAGE}"
  assert_failure 2
  assert_stderr --partial 'Usage:'
}

@test '--version prints the version the image was built as' {
  run docker run --rm --network=none "${TOOLBELT_TEST_IMAGE}" --version
  assert_success
  assert_output --partial "${TOOLBELT_TEST_VERSION}"
}

@test 'list names every lint in the manifest' {
  run docker run --rm --network=none "${TOOLBELT_TEST_IMAGE}" list
  assert_success
  local -r names_file="${BATS_TEST_TMPDIR}/names"
  manifest_lints '' > "${names_file}"
  local -a names
  mapfile -t names < "${names_file}"
  assert [ "${#names[@]}" -gt 0 ]
  local name
  for name in "${names[@]}"; do
    assert_line --partial "${name}"
  done
}

@test 'fmt with an unknown formatter is a usage error naming it' {
  run docker run --rm --network=none "${TOOLBELT_TEST_IMAGE}" fmt nope
  assert_failure 2
  assert_output --partial 'unknown formatter "nope"'
}

@test 'check-workflow-hermetic finds an unwrapped run step' {
  make_consumer "${BATS_TEST_TMPDIR}/repo"
  run hook_run run --hook pre-commit check-workflow-hermetic
  assert_failure 1
  assert_output --partial '== check-workflow-hermetic: fail =='
  assert_output --partial '.github/workflows/ci.yml'
  assert_output --partial 'unwrapped'
}

@test 'every Go lint runs and passes on the fixture' {
  make_consumer "${BATS_TEST_TMPDIR}/repo"
  local -r names_file="${BATS_TEST_TMPDIR}/names"
  manifest_lints go > "${names_file}"
  local -a go_lints
  mapfile -t go_lints < "${names_file}"
  assert [ "${#go_lints[@]}" -gt 0 ]
  run hook_run run --hook pre-commit "${go_lints[@]}"
  assert_success
  local name
  for name in "${go_lints[@]}"; do
    assert_line "== ${name}: pass =="
  done
  assert_line "toolbelt: ${#go_lints[@]} passed, 0 failed, 0 errored"
}
