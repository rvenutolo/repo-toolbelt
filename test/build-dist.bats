#!/usr/bin/env bats

# build-dist runs from the real repo; a `go` stub on PATH stands in for the
# compiler: it creates the file named by -o and records its arguments.

function setup() {
  load test_helper/common
  OUT="${BATS_TEST_TMPDIR}/out"
  CALLS="${BATS_TEST_TMPDIR}/calls"
  export CALLS
  mkdir "${BATS_TEST_TMPDIR}/bin"
  cat > "${BATS_TEST_TMPDIR}/bin/go" << 'STUB'
#!/usr/bin/env bash
printf 'go %s\n' "$*" >> "${CALLS}"
mkdir --parents "$(dirname "$3")"
printf '#!/bin/sh\n' > "$3"
chmod +x "$3"
STUB
  chmod +x "${BATS_TEST_TMPDIR}/bin/go"
  PATH="${BATS_TEST_TMPDIR}/bin:${PATH}"
}

@test 'builds the runner, every Go lint, every bash lint and lib' {
  run "${REPO_DIR}/.ci/build-dist" "${OUT}"
  assert_success
  assert [ -x "${OUT}/bin/toolbelt" ]
  assert [ -f "${OUT}/lib/toolbelt.bash" ]
  assert [ -x "${OUT}/lints/check-workflow-hermetic/check-workflow-hermetic" ]
  assert [ -x "${OUT}/lints/check-issue-numbers/check-issue-numbers" ]
  run grep --count '^go build -o .*/lints/' "${CALLS}"
  assert_output 4
}

@test 'every lints.toml entry is present and executable' {
  run "${REPO_DIR}/.ci/build-dist" "${OUT}"
  assert_success
  local name
  while IFS= read -r name; do
    assert [ -x "${OUT}/lints/${name}/${name}" ]
  done < <(yq --input-format toml --output-format tsv '.lint[].name' "${REPO_DIR}/lints.toml")
}

@test 'accepts an existing empty out dir' {
  mkdir "${OUT}"
  run "${REPO_DIR}/.ci/build-dist" "${OUT}"
  assert_success
}

@test 'a non-empty out dir fails and nothing is written' {
  mkdir "${OUT}"
  touch "${OUT}/keep"
  run "${REPO_DIR}/.ci/build-dist" "${OUT}"
  assert_failure 1
  assert_output --partial 'is not empty'
  run ls --almost-all "${OUT}"
  assert_output 'keep'
  assert [ ! -e "${CALLS}" ]
}

@test 'a wrong argument count prints usage and fails' {
  run "${REPO_DIR}/.ci/build-dist"
  assert_failure 1
  assert_output --partial 'usage:'
}

@test 'a failing build fails the script' {
  printf '#!/bin/sh\nexit 3\n' > "${BATS_TEST_TMPDIR}/bin/go"
  run "${REPO_DIR}/.ci/build-dist" "${OUT}"
  assert_failure
}
