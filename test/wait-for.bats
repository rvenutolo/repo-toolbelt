#!/usr/bin/env bats

# The workflow agents' wait helper: it returns once a status file has content,
# and exits 124 when the time runs out. With a hang limit it also keeps the
# first wait's time in <file>.start and exits 3 once that limit has passed
# since then with no status. The waits are a few seconds each, bounded by the
# helper's poll interval.

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

@test 'exits 2 when given four arguments' {
  run "${WAIT_FOR}" "${STATUS}" 1 60 extra
  assert_failure 2
}

@test 'exits 2 for a hang limit that is not a positive whole number' {
  run "${WAIT_FOR}" "${STATUS}" 1 extra
  assert_failure 2
  run "${WAIT_FOR}" "${STATUS}" 1 0
  assert_failure 2
  run "${WAIT_FOR}" "${STATUS}" 1 -5
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

@test 'records the first wait time in <file>.start when given a hang limit' {
  local -r before="$(date +%s)"
  run "${WAIT_FOR}" "${STATUS}" 1 600
  assert_failure 124
  assert [ -f "${STATUS}.start" ]
  local -r start="$(cat "${STATUS}.start")"
  assert [ "${start}" -ge "${before}" ]
  assert [ "${start}" -le "$(date +%s)" ]
}

@test 'keeps an existing start time on later waits' {
  printf '%s\n' "$(($(date +%s) - 30))" > "${STATUS}.start"
  local -r start="$(cat "${STATUS}.start")"
  run "${WAIT_FOR}" "${STATUS}" 1 600
  assert_failure 124
  assert_equal "$(cat "${STATUS}.start")" "${start}"
}

@test 'exits 3 at once when the hang limit has passed since the start time' {
  printf '%s\n' "$(($(date +%s) - 120))" > "${STATUS}.start"
  SECONDS=0
  run "${WAIT_FOR}" "${STATUS}" 60 100
  assert_failure 3
  assert [ "${SECONDS}" -lt 30 ]
}

@test 'exits 3 once the hang limit passes during a wait' {
  printf '%s\n' "$(($(date +%s) - 98))" > "${STATUS}.start"
  run "${WAIT_FOR}" "${STATUS}" 60 100
  assert_failure 3
}

@test 'returns the status rather than 3 when the file has content after the hang limit' {
  printf '%s\n' "$(($(date +%s) - 120))" > "${STATUS}.start"
  printf '0\n' > "${STATUS}"
  run "${WAIT_FOR}" "${STATUS}" 1 100
  assert_success
}

@test 'exits 2 when the start file does not hold a whole number' {
  printf 'soon\n' > "${STATUS}.start"
  run "${WAIT_FOR}" "${STATUS}" 1 100
  assert_failure 2
}

@test 'exits 2 when an existing start file is at least twice the hang limit old' {
  printf '%s\n' "$(($(date +%s) - 200))" > "${STATUS}.start"
  SECONDS=0
  run "${WAIT_FOR}" "${STATUS}" 60 100
  assert_failure 2
  assert_output --partial 'stale start file'
  assert [ "${SECONDS}" -lt 30 ]
}

@test 'a start file it writes itself is never stale' {
  run "${WAIT_FOR}" "${STATUS}" 1 1
  assert_failure 3
}

@test 'a status name with no start file of its own starts fresh beside an old start file of another name' {
  printf '%s\n' "$(($(date +%s) - 3600))" > "${BATS_TEST_TMPDIR}/other.status.start"
  run "${WAIT_FOR}" "${STATUS}" 1 100
  assert_failure 124
  assert [ "$(($(date +%s) - $(cat "${STATUS}.start")))" -lt 30 ]
}

@test 'exits 1 when a hang limit is given and the status file directory does not exist' {
  run "${WAIT_FOR}" "${BATS_TEST_TMPDIR}/missing/cmd.status" 1 100
  assert_failure 1
}

@test 'without a hang limit it writes no start file' {
  run "${WAIT_FOR}" "${STATUS}" 1
  assert_failure 124
  assert [ ! -e "${STATUS}.start" ]
}
