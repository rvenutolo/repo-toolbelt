#!/usr/bin/env bats

bats_require_minimum_version 1.5.0

function setup() {
  load test_helper/common
  REPORT="${BATS_TEST_TMPDIR}/cobertura.xml"
}

# @description Write a Cobertura report whose root element carries the given
#              attributes.
# @arg $1 attributes the root element's attribute text
function write_report() {
  local -r attributes="$1"
  printf '<?xml version="1.0" ?>\n<coverage %s>\n\t<packages/>\n</coverage>\n' "${attributes}" > "${REPORT}"
}

@test 'passes when the report covers at least one line' {
  write_report 'line-rate="0.500" lines-covered="3" lines-valid="6"'
  run --separate-stderr "${REPO_DIR}/.ci/check-kcov-hits" "${REPORT}"
  assert_success
  assert_output ''
  assert_stderr ''
}

@test 'fails when the report covers no line and names the problem' {
  write_report 'line-rate="0.000" lines-covered="0" lines-valid="759"'
  run --separate-stderr "${REPO_DIR}/.ci/check-kcov-hits" "${REPORT}"
  assert_failure 1
  assert_stderr "check-kcov-hits: ${REPORT}: kcov recorded no covered lines, so it is not tracing the bash under test"
}

@test 'fails when the report has no lines-covered count' {
  write_report 'line-rate="0.000"'
  run --separate-stderr "${REPO_DIR}/.ci/check-kcov-hits" "${REPORT}"
  assert_failure 1
  assert_stderr "check-kcov-hits: ${REPORT}: no lines-covered count on the coverage element"
}

@test 'fails when the lines-covered count is not a number' {
  write_report 'lines-covered="many"'
  run --separate-stderr "${REPO_DIR}/.ci/check-kcov-hits" "${REPORT}"
  assert_failure 1
  assert_stderr "check-kcov-hits: ${REPORT}: no lines-covered count on the coverage element"
}

@test 'fails when the report is not XML' {
  printf 'not a report <\n' > "${REPORT}"
  run --separate-stderr "${REPO_DIR}/.ci/check-kcov-hits" "${REPORT}"
  assert_failure 1
  assert_stderr --partial "check-kcov-hits: ${REPORT}: cannot read the report"
}

@test 'fails when the report does not exist' {
  run --separate-stderr "${REPO_DIR}/.ci/check-kcov-hits" "${REPORT}"
  assert_failure 1
  assert_stderr "check-kcov-hits: ${REPORT}: no such report"
}

@test 'fails with usage when no report is named' {
  run --separate-stderr "${REPO_DIR}/.ci/check-kcov-hits"
  assert_failure 1
  assert_stderr 'usage: check-kcov-hits <cobertura.xml>'
}

@test 'fails with usage when more than one report is named' {
  write_report 'lines-covered="3"'
  run --separate-stderr "${REPO_DIR}/.ci/check-kcov-hits" "${REPORT}" "${REPORT}"
  assert_failure 1
  assert_stderr 'usage: check-kcov-hits <cobertura.xml>'
}
