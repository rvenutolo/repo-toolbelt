#!/usr/bin/env bats

# The workflow agents' wait helper: it returns once a status file has content,
# and exits 124 when the time runs out. The waits are a few seconds each,
# bounded by the helper's poll interval.

function setup() {
  load test_helper/common
  WAIT_FOR="${REPO_DIR}/.claude/skills/work-backlog-parallel/wait-for"
  STATUS="${BATS_TEST_TMPDIR}/cmd.status"
}

@test 'exits 2 with usage when given no arguments' {
  run "${WAIT_FOR}"
  assert_failure 2
  assert_output --partial 'usage: wait-for <file> <seconds>'
}

@test 'exits 2 when given one argument' {
  run "${WAIT_FOR}" "${STATUS}"
  assert_failure 2
}

@test 'exits 2 when given three arguments' {
  run "${WAIT_FOR}" "${STATUS}" 1 extra
  assert_failure 2
}

@test 'exits 2 for an empty path' {
  run "${WAIT_FOR}" '' 1
  assert_failure 2
}

@test 'exits 2 for zero seconds' {
  run "${WAIT_FOR}" "${STATUS}" 0
  assert_failure 2
}

@test 'exits 2 for seconds that are not a whole number' {
  run "${WAIT_FOR}" "${STATUS}" 1.5
  assert_failure 2
  run "${WAIT_FOR}" "${STATUS}" -1
  assert_failure 2
}

@test 'reads seconds with a leading zero as decimal' {
  printf '0\n' > "${STATUS}"
  run "${WAIT_FOR}" "${STATUS}" 08
  assert_success
}

@test 'exits 2 when the path is a directory' {
  mkdir "${STATUS}"
  run "${WAIT_FOR}" "${STATUS}" 1
  assert_failure 2
}

@test 'returns at once when the file already has content' {
  printf '0\n' > "${STATUS}"
  run "${WAIT_FOR}" "${STATUS}" 1
  assert_success
  assert_output ''
}

@test 'exits 124 when the file never appears' {
  run "${WAIT_FOR}" "${STATUS}" 1
  assert_failure 124
}

@test 'exits 124 when the file stays empty' {
  : > "${STATUS}"
  run "${WAIT_FOR}" "${STATUS}" 1
  assert_failure 124
}

@test 'returns once the file is written while it waits' {
  (sleep 2 && printf '0\n' > "${STATUS}") &
  run "${WAIT_FOR}" "${STATUS}" 30
  wait
  assert_success
}
