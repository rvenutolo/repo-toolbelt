#!/usr/bin/env bats

# build-dist builds the repo containing its working directory, so most tests
# run it from the real repo and the rest from a fixture repo; a `go` stub on PATH stands in for the
# compiler: it creates the file named by -o and records its arguments. A
# fixture repo's formatters.toml is empty unless the test is about formatters.

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
  cd "${REPO_DIR}" || return 1
}

@test 'builds the runner, every Go lint, every bash lint and lib' {
  run "${REPO_DIR}/.ci/build-dist" "${OUT}"
  assert_success
  assert [ -x "${OUT}/bin/toolbelt" ]
  assert [ -f "${OUT}/lib/toolbelt.bash" ]
  assert [ -x "${OUT}/lints/check-workflow-hermetic/check-workflow-hermetic" ]
  assert [ -x "${OUT}/lints/check-issue-numbers/check-issue-numbers" ]
  local go_lints
  go_lints="$(yq --input-format toml '[.lint[] | select(.language == "go")] | length' "${REPO_DIR}/lints.toml")"
  run grep --count '^go build -o .*/lints/' "${CALLS}"
  assert_output "${go_lints}"
}

@test 'every lints.toml entry is present and executable' {
  run "${REPO_DIR}/.ci/build-dist" "${OUT}"
  assert_success
  local name
  while IFS= read -r name; do
    assert [ -x "${OUT}/lints/${name}/${name}" ]
  done < <(yq --input-format toml --output-format tsv '.lint[].name' "${REPO_DIR}/lints.toml")
}

@test 'the formatters dir holds exactly the formatters.toml entries' {
  run "${REPO_DIR}/.ci/build-dist" "${OUT}"
  assert_success
  assert [ -d "${OUT}/formatters" ]
  local -r want_file="${BATS_TEST_TMPDIR}/want"
  local -r got_file="${BATS_TEST_TMPDIR}/got"
  yq --input-format toml --output-format tsv '.formatter[].name' "${REPO_DIR}/formatters.toml" \
    | sort > "${want_file}"
  find "${OUT}/formatters" -mindepth 1 -maxdepth 1 -printf '%f\n' | sort > "${got_file}"
  run diff "${want_file}" "${got_file}"
  assert_success
  local name
  while IFS= read -r name; do
    assert [ -x "${OUT}/formatters/${name}/${name}" ]
  done < "${want_file}"
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

@test 'a relative out dir gets the full layout beneath it' {
  # Run from a subdirectory, so a path resolved against the repo root
  # instead of the caller's directory lands somewhere else.
  make_repo
  mkdir --parents lib sub
  printf '[[lint]]\nname = "check-x"\nlanguage = "go"\nspeed = "quick"\ndescription = "d"\n' > lints.toml
  touch formatters.toml
  cd sub
  run "${REPO_DIR}/.ci/build-dist" rel-out
  assert_success
  assert [ -x "${FIXTURE_REPO}/sub/rel-out/bin/toolbelt" ]
  assert [ -d "${FIXTURE_REPO}/sub/rel-out/lib" ]
  assert [ -x "${FIXTURE_REPO}/sub/rel-out/lints/check-x/check-x" ]
}

@test 'a lint in an unsupported language fails with its name' {
  make_repo
  mkdir lib
  printf '[[lint]]\nname = "check-x"\nlanguage = "rust"\nspeed = "quick"\ndescription = "d"\n' > lints.toml
  touch formatters.toml
  run "${REPO_DIR}/.ci/build-dist" "${OUT}"
  assert_failure 1
  assert_output --partial 'lint check-x has unsupported language rust'
}

@test 'a failing build fails the script with the compiler status and stops' {
  make_repo
  mkdir lib
  printf '[[lint]]\nname = "check-x"\nlanguage = "go"\nspeed = "quick"\ndescription = "d"\n' > lints.toml
  touch formatters.toml
  cat > "${BATS_TEST_TMPDIR}/bin/go" << 'STUB'
#!/usr/bin/env bash
printf 'go %s\n' "$*" >> "${CALLS}"
exit 3
STUB
  run "${REPO_DIR}/.ci/build-dist" "${OUT}"
  assert_failure 3
  assert_output --partial 'ERROR: line'
  run grep --count '^go ' "${CALLS}"
  assert_output 1
}

@test 'builds a bash formatter and a Go formatter at formatters/<name>/<name>' {
  make_repo
  mkdir --parents lib formatters/shell
  touch lints.toml
  printf '[[formatter]]\nname = "shell"\nlanguage = "bash"\n[[formatter]]\nname = "go"\nlanguage = "go"\n' \
    > formatters.toml
  printf '#!/usr/bin/env bash\n' > formatters/shell/shell
  chmod +x formatters/shell/shell
  touch formatters/shell/helper.bash
  run "${REPO_DIR}/.ci/build-dist" "${OUT}"
  assert_success
  assert [ -x "${OUT}/formatters/shell/shell" ]
  assert [ -f "${OUT}/formatters/shell/helper.bash" ]
  assert [ -x "${OUT}/formatters/go/go" ]
  run grep --count "^go build -o ${OUT}/formatters/go/go ./cmd/fmt-go\$" "${CALLS}"
  assert_output 1
}

@test 'a formatter in an unsupported language fails with its name' {
  make_repo
  mkdir lib
  touch lints.toml
  printf '[[formatter]]\nname = "rust"\nlanguage = "rust"\n' > formatters.toml
  run "${REPO_DIR}/.ci/build-dist" "${OUT}"
  assert_failure 1
  assert_output --partial 'formatter rust has unsupported language rust'
}
