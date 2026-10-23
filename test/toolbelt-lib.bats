#!/usr/bin/env bats

bats_require_minimum_version 1.5.0

function setup() {
  load test_helper/common
  # shellcheck source=lib/toolbelt.bash
  source "${REPO_DIR}/lib/toolbelt.bash"
}

@test 'return_count returns a count below the cap unchanged' {
  run toolbelt::return_count 7
  assert_failure 7
}

@test 'return_count returns zero errors as success' {
  run toolbelt::return_count 0
  assert_success
}

@test 'return_count caps a count above 255 at 255' {
  run toolbelt::return_count 300
  assert_failure 255
}

@test 'return_count caps 256 at 255 rather than wrapping to success' {
  run toolbelt::return_count 256
  assert_failure 255
}
