#!/usr/bin/env bats

# A stub `go` stands in for the toolchain: it prints the report in
# STUB_REPORT (or exits non-zero when STUB_FAIL is set), so no scan runs.

function setup() {
  load test_helper/common
  make_repo
  mkdir "${BATS_TEST_TMPDIR}/bin"
  cat > "${BATS_TEST_TMPDIR}/bin/go" << 'STUB'
#!/usr/bin/env bash
if [[ -n "${STUB_FAIL:-}" ]]; then
  exit 3
fi
cat "${STUB_REPORT}"
STUB
  chmod +x "${BATS_TEST_TMPDIR}/bin/go"
  PATH="${BATS_TEST_TMPDIR}/bin:${PATH}"
  STUB_REPORT="${BATS_TEST_TMPDIR}/report.json"
  export STUB_REPORT
}

# @description Write a govulncheck-style report with one finding per OSV id
#              given; an id prefixed with `import:` is reported without a
#              called function.
function write_report() {
  printf '{"config":{"scanner_name":"govulncheck"}}\n' > "${STUB_REPORT}"
  local spec osv function_json
  for spec in "$@"; do
    osv="${spec#import:}"
    function_json=',"function":"Run"'
    if [[ "${spec}" == import:* ]]; then
      function_json=''
    fi
    printf '{"finding":{"osv":"%s","fixed_version":"v1.2.3","trace":[{"module":"example.com/dep","version":"v1.0.0"%s}]}}\n' \
      "${osv}" "${function_json}" >> "${STUB_REPORT}"
  done
}

@test 'passes when the report has no findings' {
  write_report
  run "${REPO_DIR}/.ci/vuln"
  assert_success
}

@test 'fails on a reachable vulnerability and names it' {
  write_report GO-2099-0001
  run "${REPO_DIR}/.ci/vuln"
  assert_failure 1
  assert_output --partial 'GO-2099-0001 in example.com/dep@v1.0.0, fixed in v1.2.3'
}

@test 'ignores a vulnerability that is only imported' {
  write_report import:GO-2099-0002
  run "${REPO_DIR}/.ci/vuln"
  assert_success
}

@test 'passes a reachable vulnerability listed in the exceptions file' {
  write_report GO-2099-0001
  printf -- '- GO-2099-0001: no fix released; review by 2099-01-01\n' > SECURITY-EXCEPTIONS.md
  run "${REPO_DIR}/.ci/vuln"
  assert_success
  assert_output --partial 'excepted: GO-2099-0001'
}

@test 'fails on a reachable vulnerability the exceptions file does not list' {
  write_report GO-2099-0001 GO-2099-0003
  printf -- '- GO-2099-0001: no fix released; review by 2099-01-01\n' > SECURITY-EXCEPTIONS.md
  run "${REPO_DIR}/.ci/vuln"
  assert_failure 1
  assert_output --partial 'FAIL: GO-2099-0003'
  refute_output --partial 'FAIL: GO-2099-0001'
}

@test 'does not treat an id mentioned in prose as an exception' {
  write_report GO-2099-0001
  printf 'GO-2099-0001 is mentioned here but not excepted\n' > SECURITY-EXCEPTIONS.md
  run "${REPO_DIR}/.ci/vuln"
  assert_failure 1
}

@test 'fails when the scan does not complete' {
  write_report
  STUB_FAIL=1 run "${REPO_DIR}/.ci/vuln"
  assert_failure 1
  assert_output --partial 'govulncheck did not complete'
}

@test 'fails when the output is not a govulncheck report' {
  printf '{"finding":{"osv":"GO-2099-0001"}}\n' > "${STUB_REPORT}"
  run "${REPO_DIR}/.ci/vuln"
  assert_failure 1
  assert_output --partial "could not read govulncheck's report"
}
